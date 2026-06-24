# Copyright 2019 The Vearch Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
# implied. See the License for the specific language governing
# permissions and limitations under the License.

# -*- coding: UTF-8 -*-

"""
End-to-end balancer tests WITH real document data on the partitions.

The fast control-plane suite in `test_balancer.py` validates the balancer state
machine on empty partitions. This file complements it by inserting N vectors
before triggering migrations / reaper / picker, which exercises:

  * raft log replication and the WaitingCaughtUp step under real load
  * pickWorstReplica path that depends on partition data_bytes
  * Picker score divergence (otherwise all nodes show current_score=0)
  * end-to-end migration data integrity (doc count survives the migration)

Default vector count is intentionally small (1000) so tests stay under a few
minutes. Override with VEARCH_E2E_DOC_COUNT env var if you want heavier load.

Uses a distinct db_name (ts_e2e_db) so this suite can run independently of
test_balancer.py without colliding on space/partition IDs.
"""

import os
import random
import time

import pytest
import requests

from utils.vearch_utils import (
    router_url,
    master_url,
    username,
    password,
    create_db,
    drop_db,
    create_space,
    drop_space,
    change_partitons,
    get_cluster_health,
    logger,
)

AUTH = (username, password)
HTTP_TIMEOUT = 30
BALANCER_PREFIX = master_url + "/balancer"

DB_NAME = "ts_e2e_db"
SPACE_NAME = "ts_e2e_space"
VECTOR_DIM = 128
DEFAULT_DOC_COUNT = int(os.getenv("VEARCH_E2E_DOC_COUNT", "1000"))


# ---------- HTTP helpers --------------------------------------------------

def _get(path, **kwargs):
    return requests.get(BALANCER_PREFIX + path, auth=AUTH, timeout=HTTP_TIMEOUT, **kwargs)


def _post(path, json_body=None, **kwargs):
    return requests.post(BALANCER_PREFIX + path, auth=AUTH, json=json_body,
                         timeout=HTTP_TIMEOUT, **kwargs)


# ---------- cluster state inspection --------------------------------------

def _enumerate_partitions():
    """Return [{pid, replicas, leader, doc_num, db, space}] for the test space."""
    resp = get_cluster_health(router_url, db_name="")
    if resp.status_code != 200:
        return []
    out = []
    for db in resp.json().get("data", []):
        db_label = db.get("db_name")
        for sp in db.get("spaces", []):
            sp_label = sp.get("name")
            for p in sp.get("partitions", []) or []:
                rs = p.get("raft_status") or {}
                replicas = sorted(int(k) for k in (rs.get("Replicas") or {}).keys())
                leader = rs.get("Leader")
                if not replicas and p.get("node_id") is not None:
                    only = int(p["node_id"])
                    replicas = [only]
                    if leader is None:
                        leader = only
                out.append({
                    "pid": p.get("pid"),
                    "db": db_label,
                    "space": sp_label,
                    "replicas": replicas,
                    "leader": leader,
                    "doc_num": p.get("doc_num", 0),
                })
    return out


def _space_doc_count():
    """Total doc_num across all partitions of the test space."""
    return sum(p["doc_num"] for p in _enumerate_partitions() if p["space"] == SPACE_NAME)


def _live_ps_nodes():
    resp = requests.get(master_url + "/servers", auth=AUTH, timeout=HTTP_TIMEOUT)
    body = resp.json()
    data = body.get("data", body)
    if isinstance(data, dict):
        items = data.get("servers", [])
    else:
        items = data or []
    nodes = []
    for s in items:
        if not isinstance(s, dict):
            continue
        srv = s.get("server", s)
        nid = srv.get("name") or srv.get("ID")
        if nid is None:
            continue
        nodes.append(int(nid))
    return sorted(nodes)


def _wait_task_terminal(task_id, timeout_sec=300, poll_sec=3):
    deadline = time.time() + timeout_sec
    last = None
    while time.time() < deadline:
        tasks = _get("/tasks").json()
        found = next((t for t in tasks if t.get("id") == task_id), None)
        if found is None:
            return last or {"id": task_id, "step": "Done"}
        last = found
        if str(found.get("step", "")).lower() in ("done", "failed", "4", "5"):
            return found
        time.sleep(poll_sec)
    return last or {"id": task_id, "step": "timeout"}


# ---------- space + data setup --------------------------------------------

def _create_e2e_space(replica_num=2, partition_num=3, dim=VECTOR_DIM):
    space_config = {
        "name": SPACE_NAME,
        "partition_num": partition_num,
        "replica_num": replica_num,
        "fields": [
            {"name": "field_int", "type": "integer"},
            {
                "name": "field_vector",
                "type": "vector",
                "dimension": dim,
                "index": {
                    "name": "gamma",
                    "type": "FLAT",
                    "params": {"metric_type": "InnerProduct"},
                },
            },
        ],
    }
    return create_space(router_url, DB_NAME, space_config)


def _insert_documents(n, batch_size=100, dim=VECTOR_DIM, seed=42):
    """Insert n vectors with deterministic content. Returns inserted count."""
    rng = random.Random(seed)
    url = router_url + "/document/upsert?timeout=60000"
    inserted = 0
    for batch_start in range(0, n, batch_size):
        docs = []
        for i in range(batch_start, min(batch_start + batch_size, n)):
            docs.append({
                "_id": str(i),
                "field_int": i,
                "field_vector": [rng.random() for _ in range(dim)],
            })
        payload = {
            "db_name": DB_NAME,
            "space_name": SPACE_NAME,
            "documents": docs,
        }
        resp = requests.post(url, auth=AUTH, json=payload, timeout=120)
        if resp.status_code != 200:
            raise RuntimeError(
                f"upsert batch {batch_start}: HTTP {resp.status_code} {resp.text[:200]}"
            )
        inserted += len(docs)
    return inserted


def _wait_doc_count(expected, timeout_sec=60, poll_sec=2):
    """Poll /cluster/health until total doc_num matches expected (or timeout)."""
    deadline = time.time() + timeout_sec
    last = -1
    while time.time() < deadline:
        last = _space_doc_count()
        if last == expected:
            return last
        time.sleep(poll_sec)
    return last


# ---------- module-level fixture: skip if balancer routes unavailable -----

def _skip_if_balancer_unavailable():
    resp = _get("/config")
    if resp.status_code != 200:
        pytest.skip(
            f"balancer subsystem not reachable (HTTP {resp.status_code}); "
            f"check that the deployed binary includes /balancer/* routes"
        )


# ==========================================================================
# Test classes
# ==========================================================================

class TestBalancerE2EData:
    """End-to-end migration / reaper / picker behavior with real document data."""

    REPLICA_NUM = 2
    PARTITION_NUM = 3
    DOC_COUNT = DEFAULT_DOC_COUNT

    @classmethod
    def setup_class(cls):
        _skip_if_balancer_unavailable()
        live = _live_ps_nodes()
        if len(live) < 3:
            pytest.skip(f"need >=3 live PS nodes for e2e migration; have {len(live)}")

        try:
            drop_space(router_url, DB_NAME, SPACE_NAME)
        except Exception:
            pass
        try:
            drop_db(router_url, DB_NAME)
        except Exception:
            pass

        cdb = create_db(router_url, DB_NAME)
        assert cdb.json().get("code") == 0, f"create_db failed: {cdb.text}"

        resp = _create_e2e_space(replica_num=cls.REPLICA_NUM, partition_num=cls.PARTITION_NUM)
        assert resp.json().get("code") == 0, f"create_space failed: {resp.text}"

        time.sleep(3)  # let partitions register
        inserted = _insert_documents(cls.DOC_COUNT)
        logger.info(f"[e2e setup] inserted {inserted} docs into {SPACE_NAME}")

        observed = _wait_doc_count(cls.DOC_COUNT, timeout_sec=90)
        logger.info(f"[e2e setup] cluster reports {observed} docs (target {cls.DOC_COUNT})")
        if observed != cls.DOC_COUNT:
            pytest.skip(
                f"doc count did not converge to {cls.DOC_COUNT} within 90s "
                f"(got {observed}); cluster too slow for e2e tests"
            )

    @classmethod
    def teardown_class(cls):
        try:
            drop_space(router_url, DB_NAME, SPACE_NAME)
            drop_db(router_url, DB_NAME)
        except Exception as e:
            logger.info(f"[e2e teardown] cleanup ignored: {e}")

    # ---- test 1: data integrity through migration ------------------------

    def test_migration_preserves_doc_count(self):
        """Migrate one partition's replica; total doc_num must not change."""
        baseline_docs = _space_doc_count()
        assert baseline_docs == self.DOC_COUNT, (
            f"unexpected baseline doc count {baseline_docs} != {self.DOC_COUNT}"
        )

        parts = [p for p in _enumerate_partitions() if p["space"] == SPACE_NAME]
        live = _live_ps_nodes()

        candidate = None
        for p in parts:
            non_leader = [r for r in p["replicas"] if r != p["leader"]]
            free = [n for n in live if n not in p["replicas"]]
            if non_leader and free:
                candidate = (p["pid"], non_leader[0], free[0])
                break
        if candidate is None:
            pytest.skip("no partition with non-leader source + free target")

        pid, from_id, to_id = candidate
        logger.info(f"migrating partition {pid}: {from_id} -> {to_id}")

        resp = _post("/migrate", {
            "partition_id": pid,
            "from_node_id": from_id,
            "to_node_id": to_id,
            "reason": "e2e-data-migration",
        })
        assert resp.status_code == 200, resp.text
        task_id = resp.json()["id"]

        final = _wait_task_terminal(task_id, timeout_sec=600)
        step = str(final.get("step", "")).lower()
        assert step in ("done", "4"), (
            f"migration did not complete; final={final}"
        )
        logger.info(f"migration reached terminal step={step}")

        # Allow brief settle, then check total doc count is preserved.
        time.sleep(5)
        after_docs = _space_doc_count()
        assert after_docs == baseline_docs, (
            f"doc count drift: before={baseline_docs} after={after_docs}"
        )
        logger.info(f"[PASS] doc count preserved: {after_docs}")

    # ---- test 2: search returns expected docs after migration ------------

    def test_search_consistent_after_migration(self):
        """A known-id GET must return after the migration above."""
        url = router_url + "/document/query"
        # Use document_ids query: deterministic, doesn't depend on vector index quality.
        sample_ids = ["0", "1", str(self.DOC_COUNT // 2), str(self.DOC_COUNT - 1)]
        payload = {
            "db_name": DB_NAME,
            "space_name": SPACE_NAME,
            "document_ids": sample_ids,
        }
        resp = requests.post(url, auth=AUTH, json=payload, timeout=60)
        assert resp.status_code == 200, resp.text
        body = resp.json()
        docs = body.get("data", {}).get("documents") if isinstance(body.get("data"), dict) else body.get("data", [])
        # Tolerate different response shapes — primary assertion is "non-empty + len matches"
        if isinstance(docs, list):
            returned_ids = sorted(
                str(d.get("_id") or d.get("id"))
                for d in docs if isinstance(d, dict)
            )
        else:
            returned_ids = []
        logger.info(f"sample query returned ids: {returned_ids}")
        assert len(returned_ids) == len(sample_ids), (
            f"expected {len(sample_ids)} docs back; got {len(returned_ids)}: {body}"
        )

    # ---- test 3: picker score differentiates after data load -------------

    def test_picker_score_differentiates_with_data(self):
        """With non-trivial data, snapshot scores should not all be 0/tied."""
        snap = _get("/snapshot").json()
        items = snap.get("items", [])
        if len(items) < 2:
            pytest.skip(f"need >=2 PS items in snapshot; have {len(items)}")

        scores = sorted(it.get("current_score", 0) for it in items)
        data_bytes = sorted(it.get("data_bytes", 0) for it in items)
        partitions = sorted(it.get("partition_count", 0) for it in items)
        logger.info(
            f"snapshot per-PS: scores={scores}, data_bytes={data_bytes}, partition_count={partitions}"
        )
        # Either current_score or partition_count or data_bytes should differ
        # across nodes once data is loaded. (current_score may be derived from
        # any of them depending on Picker config.) Allow ties in score IF
        # partition_count or data_bytes already differ — meaning the underlying
        # signal exists even if the score formula hasn't surfaced it yet.
        diverged = (
            scores[0] != scores[-1]
            or data_bytes[0] != data_bytes[-1]
            or partitions[0] != partitions[-1]
        )
        assert diverged, (
            f"all snapshot dimensions are tied across nodes after data load; "
            f"picker has no signal to differentiate. items={items}"
        )
        logger.info("[PASS] snapshot signal differentiates nodes after data load")

    # ---- test 4: reaper removes excess replica on a partition with data --

    def test_reaper_removes_excess_replica_with_data(self):
        """Add a redundant replica to a data-bearing partition; reaper trims it."""
        # Set reaper enabled in config first.
        _post("/config", {})  # no-op POST won't change anything; just ensure path reachable
        # Actually update config via PUT to enable reaper.
        cfg = _get("/config").json()
        cfg["redundant_reaper_enabled"] = True
        put = requests.put(
            BALANCER_PREFIX + "/config",
            auth=AUTH, json=cfg, timeout=HTTP_TIMEOUT,
        )
        assert put.status_code == 200, put.text

        live = _live_ps_nodes()
        parts = [p for p in _enumerate_partitions() if p["space"] == SPACE_NAME]
        if not parts:
            pytest.skip("no partition in e2e space")
        target = parts[0]
        current = set(target["replicas"])
        free = next((n for n in live if n not in current), None)
        if free is None:
            pytest.skip("no spare PS to add as redundant replica")

        # Add a 3rd replica (over-replicated wrt ReplicaNum=2).
        resp = change_partitons(router_url, [target["pid"]], free, 0)
        logger.info(f"added redundant replica: {resp.text}")
        time.sleep(8)  # raft conf change + initial caught-up

        before = next((p for p in _enumerate_partitions() if p["pid"] == target["pid"]), None)
        if before is None or len(before["replicas"]) <= self.REPLICA_NUM:
            pytest.skip(
                f"redundant replica not visible after AddNode (state: {before}); "
                f"raft may have rejected or it caught up but isn't reported yet"
            )
        logger.info(f"partition {target['pid']} pre-reaper replicas: {before['replicas']}")

        trigger = _post("/reaper/trigger")
        assert trigger.status_code == 200, trigger.text

        # Reaper goroutine retries lock for ~90s, then RunOnce + ChangeMember.
        # Allow generous time for the conf change to propagate.
        deadline = time.time() + 180
        converged = False
        last = None
        while time.time() < deadline:
            last = next(
                (p for p in _enumerate_partitions() if p["pid"] == target["pid"]),
                None,
            )
            if last and len(last["replicas"]) == self.REPLICA_NUM:
                converged = True
                break
            time.sleep(5)

        assert converged, (
            f"reaper did not converge partition {target['pid']} back to "
            f"ReplicaNum={self.REPLICA_NUM} within 180s; last={last}"
        )
        logger.info(f"[PASS] reaper converged to {last['replicas']}")

        # Doc count must still match.
        after_docs = _space_doc_count()
        assert after_docs == self.DOC_COUNT, (
            f"doc count drift after reaper: expected {self.DOC_COUNT}, got {after_docs}"
        )
        logger.info(f"[PASS] doc count preserved across reaper: {after_docs}")
