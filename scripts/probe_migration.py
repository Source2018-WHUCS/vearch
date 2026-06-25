#!/usr/bin/env python3
"""
Probe what happens during a balancer migration with real document data.

Stand-alone — does NOT use pytest. Use this to diagnose why a migration
stalls at WaitingCaughtUp by capturing the leader's raft view of the new
replica every few seconds. Leaves the cluster state intact on exit so you
can keep poking at it.

Usage:

    MASTER_URL=http://127.0.0.1:28817 \
    ROUTER_URL=http://127.0.0.1:19001 \
    PASSWORD=...  python3 scripts/probe_migration.py

Environment variables:
    MASTER_URL          (default http://127.0.0.1:28817)
    ROUTER_URL          (default http://127.0.0.1:19001)
    USERNAME            (default root)
    PASSWORD            (required)
    PROBE_DB            (default ts_probe_db)
    PROBE_SPACE         (default ts_probe_space)
    PROBE_DOC_COUNT     (default 1000)
    PROBE_POLL_SEC      (default 3)
    PROBE_MAX_SEC       (default 600)
    PROBE_SKIP_SETUP    (default false; set to "true" to reuse existing space)
    PROBE_KEEP_STATE    (default true; set to "false" to drop space on exit)
"""

import json
import os
import random
import sys
import time

import requests

MASTER_URL = os.environ.get("MASTER_URL", "http://127.0.0.1:28817").rstrip("/")
ROUTER_URL = os.environ.get("ROUTER_URL", "http://127.0.0.1:19001").rstrip("/")
USERNAME = os.environ.get("USERNAME", "root")
PASSWORD = os.environ.get("PASSWORD")
if not PASSWORD:
    sys.stderr.write("PASSWORD env var is required\n")
    sys.exit(2)

AUTH = (USERNAME, PASSWORD)
DB_NAME = os.environ.get("PROBE_DB", "ts_probe_db")
SPACE_NAME = os.environ.get("PROBE_SPACE", "ts_probe_space")
DOC_COUNT = int(os.environ.get("PROBE_DOC_COUNT", "1000"))
POLL_SEC = float(os.environ.get("PROBE_POLL_SEC", "3"))
MAX_SEC = float(os.environ.get("PROBE_MAX_SEC", "600"))
SKIP_SETUP = os.environ.get("PROBE_SKIP_SETUP", "false").lower() == "true"
KEEP_STATE = os.environ.get("PROBE_KEEP_STATE", "true").lower() == "true"
VECTOR_DIM = 128
HTTP_TIMEOUT = 60

BALANCER = MASTER_URL + "/balancer"


def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)


def http_get(url):
    return requests.get(url, auth=AUTH, timeout=HTTP_TIMEOUT)


def http_post(url, payload):
    return requests.post(url, auth=AUTH, json=payload, timeout=HTTP_TIMEOUT)


def http_put(url, payload):
    return requests.put(url, auth=AUTH, json=payload, timeout=HTTP_TIMEOUT)


def http_delete(url):
    return requests.delete(url, auth=AUTH, timeout=HTTP_TIMEOUT)


# ------------------ cluster state ------------------

def live_ps_nodes():
    r = http_get(MASTER_URL + "/servers")
    body = r.json()
    data = body.get("data", body)
    items = data.get("servers", []) if isinstance(data, dict) else (data or [])
    out = []
    for s in items:
        if not isinstance(s, dict):
            continue
        srv = s.get("server", s)
        nid = srv.get("name") or srv.get("ID")
        if nid is None:
            continue
        out.append(int(nid))
    return sorted(out)


def cluster_health():
    return http_get(ROUTER_URL + "/cluster/health?detail=true").json()


def list_partitions():
    """Returns list of {pid, db, space, replicas, leader, raft_replicas_map, doc_num}."""
    out = []
    body = cluster_health()
    for db in body.get("data", []) or []:
        db_label = db.get("db_name")
        for sp in db.get("spaces", []) or []:
            sp_label = sp.get("name")
            for p in sp.get("partitions", []) or []:
                rs = p.get("raft_status") or {}
                replicas_map = rs.get("Replicas") or {}
                replicas = sorted(int(k) for k in replicas_map.keys())
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
                    "raft_replicas_map": replicas_map,
                    "doc_num": p.get("doc_num", 0),
                })
    return out


def space_doc_count(space=SPACE_NAME):
    return sum(p["doc_num"] for p in list_partitions() if p["space"] == space)


# ------------------ space + data ------------------

def drop_space():
    try:
        http_delete(f"{ROUTER_URL}/dbs/{DB_NAME}/spaces/{SPACE_NAME}")
    except Exception:
        pass


def drop_db():
    try:
        http_delete(f"{ROUTER_URL}/dbs/{DB_NAME}")
    except Exception:
        pass


def create_db():
    r = http_post(f"{ROUTER_URL}/dbs/{DB_NAME}", {"name": DB_NAME})
    if r.status_code != 200:
        raise RuntimeError(f"create_db: HTTP {r.status_code} {r.text[:200]}")


def create_space(replica_num=2, partition_num=3):
    payload = {
        "name": SPACE_NAME,
        "partition_num": partition_num,
        "replica_num": replica_num,
        "fields": [
            {"name": "field_int", "type": "integer"},
            {
                "name": "field_vector",
                "type": "vector",
                "dimension": VECTOR_DIM,
                "index": {
                    "name": "gamma",
                    "type": "FLAT",
                    "params": {"metric_type": "InnerProduct"},
                },
            },
        ],
    }
    r = http_post(f"{ROUTER_URL}/dbs/{DB_NAME}/spaces", payload)
    if r.status_code != 200:
        raise RuntimeError(f"create_space: HTTP {r.status_code} {r.text[:200]}")


def insert_docs(n, batch=100):
    rng = random.Random(42)
    url = ROUTER_URL + "/document/upsert?timeout=60000"
    for start in range(0, n, batch):
        docs = []
        for i in range(start, min(start + batch, n)):
            docs.append({
                "_id": str(i),
                "field_int": i,
                "field_vector": [rng.random() for _ in range(VECTOR_DIM)],
            })
        body = {"db_name": DB_NAME, "space_name": SPACE_NAME, "documents": docs}
        r = requests.post(url, auth=AUTH, json=body, timeout=120)
        if r.status_code != 200:
            raise RuntimeError(f"upsert {start}: HTTP {r.status_code} {r.text[:200]}")


def wait_doc_count(expected, timeout=90):
    deadline = time.time() + timeout
    last = -1
    while time.time() < deadline:
        last = space_doc_count()
        if last == expected:
            return last
        time.sleep(2)
    return last


# ------------------ migration ------------------

def submit_migrate(pid, from_id, to_id, reason="probe"):
    r = http_post(BALANCER + "/migrate", {
        "partition_id": pid,
        "from_node_id": from_id,
        "to_node_id": to_id,
        "reason": reason,
    })
    if r.status_code != 200:
        raise RuntimeError(f"migrate: HTTP {r.status_code} {r.text[:300]}")
    return r.json()


def get_task(task_id):
    tasks = http_get(BALANCER + "/tasks").json()
    if not isinstance(tasks, list):
        return None
    return next((t for t in tasks if t.get("id") == task_id), None)


# ------------------ main probe loop ------------------

STEP_NAMES = {0: "Pending", 1: "AddingMember", 2: "WaitingCaughtUp",
              3: "RemovingMember", 4: "Done", 5: "Failed"}


def format_partition_state(p, target_id):
    if p is None:
        return "(partition not visible in cluster/health)"
    leader = p.get("leader")
    rmap = p.get("raft_replicas_map") or {}
    leader_match = None
    target_match = None
    target_present = False
    for nid, info in rmap.items():
        info = info or {}
        if str(nid) == str(leader):
            leader_match = info.get("Match")
        if str(nid) == str(target_id):
            target_present = True
            target_match = info.get("Match")
    pct = ""
    if leader_match and target_match:
        try:
            pct = f"  pct={(int(target_match) / int(leader_match)) * 100:.1f}%"
        except Exception:
            pass
    return (
        f"replicas={p['replicas']}  leader={leader}  doc_num={p['doc_num']}  "
        f"target_in_raft={target_present}  leader.Match={leader_match}  "
        f"target.Match={target_match}{pct}"
    )


def main():
    log(f"MASTER_URL={MASTER_URL}  ROUTER_URL={ROUTER_URL}")
    log(f"DB={DB_NAME}  SPACE={SPACE_NAME}  DOC_COUNT={DOC_COUNT}  KEEP_STATE={KEEP_STATE}")

    if not SKIP_SETUP:
        log("== setup: drop + recreate space ==")
        drop_space()
        drop_db()
        time.sleep(1)
        create_db()
        create_space(replica_num=2, partition_num=3)
        time.sleep(3)
        log(f"== inserting {DOC_COUNT} docs ==")
        insert_docs(DOC_COUNT)
        observed = wait_doc_count(DOC_COUNT)
        log(f"doc count observed = {observed}/{DOC_COUNT}")
        if observed != DOC_COUNT:
            log("WARN: doc count did not converge; continuing anyway")
    else:
        log("== SKIP_SETUP=true; reusing existing space ==")

    parts = [p for p in list_partitions() if p["space"] == SPACE_NAME]
    live = live_ps_nodes()
    log(f"live PS nodes: {live}")
    for p in parts:
        log(f"  partition {p['pid']}: replicas={p['replicas']} leader={p['leader']} doc_num={p['doc_num']}")

    candidate = None
    for p in parts:
        non_leader = [r for r in p["replicas"] if r != p["leader"]]
        free = [n for n in live if n not in p["replicas"]]
        if non_leader and free:
            candidate = (p["pid"], non_leader[0], free[0])
            break
    if candidate is None:
        log("FAIL: no partition with non-leader source + free target")
        return 2
    pid, from_id, to_id = candidate
    log(f"=== submitting migrate: partition={pid}  {from_id} -> {to_id} ===")
    task = submit_migrate(pid, from_id, to_id, reason="probe")
    task_id = task["id"]
    log(f"task_id={task_id}")

    deadline = time.time() + MAX_SEC
    last_step = None
    while time.time() < deadline:
        t = get_task(task_id)
        if t is None:
            log(f"task gone from /balancer/tasks (probably terminal); exiting")
            break
        step_n = t.get("step")
        step = STEP_NAMES.get(step_n, str(step_n))
        if step_n != last_step:
            log(f"-- step transition: {STEP_NAMES.get(last_step, last_step)} -> {step}")
            last_step = step_n

        ps = [p for p in list_partitions() if p["pid"] == pid]
        pstate = ps[0] if ps else None
        log(f"step={step}  last_tick={t.get('last_tick_time')}  "
            f"step_enter={t.get('step_enter_time')}  "
            f"locked_leader={t.get('locked_leader_id')}")
        log(f"  partition {pid}: {format_partition_state(pstate, to_id)}")

        if step_n in (4, 5):  # Done or Failed
            log(f"=== terminal: step={step} ===")
            break
        time.sleep(POLL_SEC)
    else:
        log(f"=== probe window {MAX_SEC}s expired; task NOT terminal ===")

    final = get_task(task_id)
    log(f"final task state: {json.dumps(final, sort_keys=True) if final else 'None (terminal)'}")

    if not KEEP_STATE:
        log("== cleanup: dropping space + db ==")
        drop_space()
        drop_db()
    else:
        log("== KEEP_STATE=true; leaving cluster intact ==")
    return 0


if __name__ == "__main__":
    sys.exit(main())
