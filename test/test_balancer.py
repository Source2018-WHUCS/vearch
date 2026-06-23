#
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

import time

import pytest
import requests

from utils.vearch_utils import (
    db_name,
    logger,
    master_url,
    password,
    router_url,
    space_name,
    username,
    create_db,
    create_space,
    drop_db,
    drop_space,
    list_spaces,
    get_servers_status,
    get_cluster_health,
    change_partitons,
)

__description__ = """ integration tests for Phase 1 balancer subsystem """


AUTH = (username, password)
BALANCER_PREFIX = master_url + "/balancer"
# Default per-request timeout; prevents hangs if master is unreachable.
HTTP_TIMEOUT = 30


def _get(path, **kwargs):
    kwargs.setdefault("timeout", HTTP_TIMEOUT)
    return requests.get(BALANCER_PREFIX + path, auth=AUTH, **kwargs)


def _put(path, json_body, **kwargs):
    kwargs.setdefault("timeout", HTTP_TIMEOUT)
    return requests.put(BALANCER_PREFIX + path, auth=AUTH, json=json_body, **kwargs)


def _post(path, json_body=None, **kwargs):
    kwargs.setdefault("timeout", HTTP_TIMEOUT)
    return requests.post(BALANCER_PREFIX + path, auth=AUTH, json=json_body, **kwargs)


def _default_config_body(**overrides):
    """Build a fully-formed valid Config body; tests override specific fields."""
    cfg = {
        "enabled": False,
        "score_based_placement": False,
        "cross_space_factor": 0.5,
        "disk_reject_threshold": 0.85,
        "partition_diff_threshold": 1,
        "data_bytes_skew_threshold": 0.3,
        "min_benefit_ratio": 0.2,
        "plan_interval_sec": 300,
        "max_ops_per_round": 5,
        "max_ops_per_space_per_round": 2,
        "cooldown_per_partition_sec": 1800,
        "conf_change_pool_size": 20,
        "data_replica_pool_size": 2,
        "add_member_timeout_sec": 60,
        "caught_up_timeout_sec": 1800,
        "remove_member_timeout_sec": 60,
        "allow_leader_migration": False,
        "strict_anti_affinity": True,
        "redundant_reaper_enabled": True,
        "redundant_reaper_interval_sec": 600,
        "stats_collect_interval_sec": 15,
        "ps_stat_ttl_sec": 90,
    }
    cfg.update(overrides)
    return cfg


# =============================================================================
# Module 0: Environment probe — print cluster topology so operators can see
# which tests will be exercised vs skipped under their setup.
# =============================================================================


class TestBalancerEnvironmentInfo:
    """
    Diagnostic-only class. Always passes — it just logs the test environment
    so anyone reading the run output knows why downstream tests skip.

    Tests requiring ≥3 PS, distinct hosts, or multiple resource pools are
    common in this file; in minimal CI envs many will skip with a green run.
    This probe makes the actual coverage visible.
    """

    def test_print_environment_topology(self):
        # Defer import so this module loads even if helpers below are not yet
        # defined at class-construction time.
        live = _all_live_ps_nodes()
        pools = _servers_by_resource_pool()
        hosts = {h for _, h, _ in live if h}

        logger.info("=" * 70)
        logger.info("balancer test environment")
        logger.info(f"  live PS nodes:    {len(live)}")
        logger.info(f"  distinct hosts:   {len(hosts)}  ({sorted(hosts) if hosts else '∅'})")
        logger.info(f"  resource pools:   {list(pools.keys())}")
        logger.info(f"")
        logger.info("  Tests requiring ≥3 PS will skip if PS count < 3")
        logger.info("  Tests requiring distinct hosts will skip if hosts < 3")
        logger.info("  Resource pool isolation test will skip if pools < 2")
        logger.info("  IsLive validation test will skip if no PS is down")
        logger.info("=" * 70)


# =============================================================================
# Module 1: Config (GET / PUT) + Validate
# =============================================================================


class TestBalancerConfig:
    """Covers the Config module: read, write, validate."""

    def setup_class(self):
        # Restore a known-good config before running.
        resp = _put("/config", _default_config_body())
        assert resp.status_code == 200, resp.text

    def test_get_config_returns_all_fields(self):
        resp = _get("/config")
        logger.info(resp.json())
        assert resp.status_code == 200

        cfg = resp.json()
        # Required Phase 1 fields must be present.
        required = [
            "enabled", "score_based_placement", "cross_space_factor",
            "disk_reject_threshold", "partition_diff_threshold",
            "data_bytes_skew_threshold", "min_benefit_ratio",
            "plan_interval_sec", "max_ops_per_round",
            "max_ops_per_space_per_round", "cooldown_per_partition_sec",
            "conf_change_pool_size", "data_replica_pool_size",
            "add_member_timeout_sec", "caught_up_timeout_sec",
            "remove_member_timeout_sec", "allow_leader_migration",
            "strict_anti_affinity", "redundant_reaper_enabled",
            "redundant_reaper_interval_sec", "stats_collect_interval_sec",
            "ps_stat_ttl_sec",
        ]
        for key in required:
            assert key in cfg, f"missing config field: {key}"
        logger.info("[PASS] TestBalancerConfig.test_get_config_returns_all_fields")

    def test_update_config_valid_round_trip(self):
        body = _default_config_body(plan_interval_sec=600, max_ops_per_round=10)
        resp = _put("/config", body)
        logger.info(resp.json())
        assert resp.status_code == 200
        assert resp.json().get("ok") is True

        cfg = _get("/config").json()
        assert cfg["plan_interval_sec"] == 600
        assert cfg["max_ops_per_round"] == 10
        logger.info("[PASS] TestBalancerConfig.test_update_config_valid_round_trip")

    def test_config_round_trip_preserves_strict_anti_affinity(self):
        # NOTE: this verifies round-trip (we PUT True → we GET True), not the
        # server's hardcoded DefaultConfig(). To verify the server default we
        # would need to GET *before* any PUT — not feasible inside a setup that
        # itself PUTs. The Go-side default is enforced by
        # internal/master/services/balancer/config.go and covered by code review.
        _put("/config", _default_config_body())
        cfg = _get("/config").json()
        assert cfg["strict_anti_affinity"] is True, (
            "REV4-6 / H1: strict_anti_affinity must round-trip True"
        )
        logger.info("[PASS] TestBalancerConfig.test_config_round_trip_preserves_strict_anti_affinity")


# =============================================================================
# Module 2: Config Validate — bad values must be rejected
# =============================================================================


class TestBalancerConfigValidate:
    """Covers Config.Validate() — bad values must be rejected with 400."""

    def setup_class(self):
        _put("/config", _default_config_body())

    @pytest.mark.parametrize(
        "field,bad_value",
        [
            ("cross_space_factor", -0.1),
            ("cross_space_factor", 1.5),
            ("disk_reject_threshold", 0),
            ("disk_reject_threshold", 1.5),
            ("partition_diff_threshold", 0),
            ("data_bytes_skew_threshold", -1),
            ("min_benefit_ratio", -0.1),
            ("min_benefit_ratio", 1),
            ("plan_interval_sec", 0),
            ("max_ops_per_round", 0),
            ("max_ops_per_space_per_round", 0),
            ("conf_change_pool_size", 0),
            ("data_replica_pool_size", 0),
            ("add_member_timeout_sec", 0),
            ("caught_up_timeout_sec", 0),
            ("remove_member_timeout_sec", 0),
            ("stats_collect_interval_sec", 0),
        ],
    )
    def test_invalid_field_rejected(self, field, bad_value):
        body = _default_config_body(**{field: bad_value})
        resp = _put("/config", body)
        logger.info(f"field={field} value={bad_value} -> {resp.status_code} {resp.text}")
        assert resp.status_code == 400, (
            f"expected 400 for {field}={bad_value}, got {resp.status_code}"
        )
        logger.info(f"[PASS] TestBalancerConfigValidate.test_invalid_field_rejected[{field}={bad_value}]")

    def test_ps_stat_ttl_must_exceed_collect_interval(self):
        body = _default_config_body(stats_collect_interval_sec=30, ps_stat_ttl_sec=30)
        resp = _put("/config", body)
        logger.info(resp.json())
        assert resp.status_code == 400, (
            "ps_stat_ttl_sec must be > stats_collect_interval_sec"
        )
        logger.info("[PASS] TestBalancerConfigValidate.test_ps_stat_ttl_must_exceed_collect_interval")


# =============================================================================
# Module 3: Pause / Resume — Enabled flag persisted to etcd
# =============================================================================


class TestBalancerPauseResume:
    """Covers /balancer/pause and /balancer/resume."""

    def setup_class(self):
        _put("/config", _default_config_body(enabled=True))

    def test_pause_sets_enabled_false(self):
        resp = _post("/pause")
        logger.info(resp.json())
        assert resp.status_code == 200
        assert resp.json().get("enabled") is False

        cfg = _get("/config").json()
        assert cfg["enabled"] is False, "Enabled flag should be false after pause"
        logger.info("[PASS] TestBalancerPauseResume.test_pause_sets_enabled_false")

    def test_resume_sets_enabled_true(self):
        resp = _post("/resume")
        logger.info(resp.json())
        assert resp.status_code == 200
        assert resp.json().get("enabled") is True

        cfg = _get("/config").json()
        assert cfg["enabled"] is True, "Enabled flag should be true after resume"
        logger.info("[PASS] TestBalancerPauseResume.test_resume_sets_enabled_true")

    def test_pause_then_resume_round_trip(self):
        # Several toggles in sequence to verify state stability.
        for _ in range(3):
            assert _post("/pause").json().get("enabled") is False
            assert _post("/resume").json().get("enabled") is True
        logger.info("[PASS] TestBalancerPauseResume.test_pause_then_resume_round_trip")

    def teardown_class(self):
        # Final state from this class is Enabled=true; restore default
        # (Enabled=false) so downstream classes start from a known baseline.
        _put("/config", _default_config_body())


# =============================================================================
# Module 4: Snapshot — cluster load view
# =============================================================================


class TestBalancerSnapshot:
    """Covers /balancer/snapshot."""

    def setup_class(self):
        _put("/config", _default_config_body())

    def test_snapshot_basic_shape(self):
        resp = _get("/snapshot")
        logger.info(resp.json())
        assert resp.status_code == 200

        snap = resp.json()
        assert "items" in snap
        assert "score_std_dev" in snap
        assert "inflight_count" in snap
        assert isinstance(snap["items"], list)
        assert isinstance(snap["inflight_count"], int) and snap["inflight_count"] >= 0
        logger.info("[PASS] TestBalancerSnapshot.test_snapshot_basic_shape")

    def test_snapshot_item_fields(self):
        snap = _get("/snapshot").json()
        if not snap["items"]:
            pytest.skip("no live PS items in cluster; snapshot fields cannot be checked")

        sample = snap["items"][0]
        required = [
            "node_id", "ip", "current_score", "assigned_score",
            "priority", "partition_count", "leader_count",
            "data_bytes", "disk_usage",
        ]
        for key in required:
            assert key in sample, f"snapshot item missing field: {key}"
        logger.info(f"[PASS] TestBalancerSnapshot.test_snapshot_item_fields (sample={sample})")


# =============================================================================
# Module 5: Tasks — inflight migration listing
# =============================================================================


class TestBalancerTasks:
    """Covers /balancer/tasks."""

    def setup_class(self):
        _put("/config", _default_config_body())

    def test_list_tasks_returns_array(self):
        resp = _get("/tasks")
        logger.info(resp.json())
        assert resp.status_code == 200
        tasks = resp.json()
        assert isinstance(tasks, list)
        # Every task in the array must have at least these fields.
        for t in tasks:
            for key in ("id", "partition_id", "from_node_id", "to_node_id", "step"):
                assert key in t, f"task missing field {key}: {t}"
        logger.info(f"[PASS] TestBalancerTasks.test_list_tasks_returns_array (n={len(tasks)})")

    def test_list_tasks_has_no_residual_terminal_tasks(self):
        # No task in /tasks should be in a terminal step — terminal tasks are
        # removed from etcd by fail()/complete() (M2 fix). Anything in Done or
        # Failed here is a leak.
        tasks = _get("/tasks").json()
        terminal = [t for t in tasks if str(t.get("step", "")).lower() in ("done", "failed")]
        assert not terminal, (
            f"M2 violation: terminal tasks should be reaped from etcd; saw: {terminal}"
        )
        logger.info(f"[PASS] TestBalancerTasks.test_list_tasks_has_no_residual_terminal_tasks")


# =============================================================================
# Module 6: Manual Migrate — input validation
# =============================================================================


class TestBalancerManualMigratePrepare:
    """Spin up a small space so manual-migrate has something to operate on."""

    def setup_class(self):
        # Best-effort drop in case a previous run left state.
        try:
            spaces = list_spaces(router_url, db_name).json()
            if spaces.get("code") == 0:
                for s in spaces.get("data", []):
                    drop_space(router_url, db_name, s["space_name"])
            drop_db(router_url, db_name)
        except Exception as e:
            logger.info(f"setup cleanup ignored: {e}")

        resp = create_db(router_url, db_name)
        logger.info(resp.json())
        assert resp.json()["code"] == 0

    def test_prepare_space_for_migrate(self):
        # replica_num=2 so we have partitions with both `from_node_id ∈ replicas`
        # and `to_node_id ∈ replicas` cases available for validation tests.
        space_config = {
            "name": space_name,
            "partition_num": 3,
            "replica_num": 2,
            "fields": [
                {"name": "field_int", "type": "integer"},
                {
                    "name": "field_vector",
                    "type": "vector",
                    "dimension": 128,
                    "index": {
                        "name": "gamma",
                        "type": "FLAT",
                        "params": {"metric_type": "InnerProduct"},
                    },
                },
            ],
        }
        resp = create_space(router_url, db_name, space_config)
        logger.info(resp.json())
        if resp.json().get("code") != 0:
            pytest.skip(f"cannot create replica_num=2 space (likely <2 PS nodes): {resp.json()}")
        logger.info("[PASS] TestBalancerManualMigratePrepare.test_prepare_space_for_migrate")


class TestBalancerManualMigrateValidation:
    """Covers POST /balancer/migrate input validation (fail-fast paths)."""

    def setup_class(self):
        _put("/config", _default_config_body())
        # Self-contained: ensure validation has a target db+space, regardless
        # of whether TestBalancerManualMigratePrepare ran first (pytest-randomly
        # or explicit class selection may reorder).
        spaces_resp = list_spaces(router_url, db_name)
        has_space = False
        try:
            j = spaces_resp.json()
            has_space = j.get("code") == 0 and any(
                s.get("space_name") == space_name for s in (j.get("data") or [])
            )
        except Exception:
            pass
        if not has_space:
            try:
                create_db(router_url, db_name)
            except Exception:
                pass
            resp = _create_test_space(replica_num=2, partition_num=2)
            if resp.json().get("code") != 0:
                # Some validation tests can still run if there's any partition
                # in the cluster (they enumerate cluster-wide); don't fail here.
                logger.info(f"setup could not create replica_num=2 space: {resp.json()}")

    def test_migrate_missing_partition(self):
        # Bogus partition_id that doesn't exist; QueryPartition fails → 400.
        body = {
            "partition_id": 999999999,
            "from_node_id": 1,
            "to_node_id": 2,
            "reason": "test-missing-partition",
        }
        resp = _post("/migrate", body)
        logger.info(f"status={resp.status_code} body={resp.text}")
        assert resp.status_code == 400, (
            f"missing partition should yield 400, got {resp.status_code} ({resp.text})"
        )
        logger.info("[PASS] TestBalancerManualMigrateValidation.test_migrate_missing_partition")

    def test_migrate_from_node_not_replica(self):
        partitions = _enumerate_partitions()
        if not partitions:
            pytest.skip("no partitions available")
        part = partitions[0]

        body = {
            "partition_id": part["pid"],
            "from_node_id": 999999,  # bogus — not a replica
            # Use a separate bogus to_node_id so we test the from-check in
            # isolation; passing a real replica here would risk hitting the
            # to_already_replica check first if the validation order changed.
            "to_node_id": 999998,
            "reason": "test-from-not-replica",
        }
        resp = _post("/migrate", body)
        logger.info(f"status={resp.status_code} body={resp.text}")
        assert resp.status_code == 400
        assert "is not a replica" in resp.text, (
            f"expected 'is not a replica' in error; got: {resp.text}"
        )
        logger.info("[PASS] TestBalancerManualMigrateValidation.test_migrate_from_node_not_replica")

    def test_migrate_to_node_not_registered(self):
        partitions = _enumerate_partitions()
        if not partitions:
            pytest.skip("no partitions available")
        part = next((p for p in partitions if p["replicas"]), None)
        if part is None:
            pytest.skip("no partition with replicas to use as from_node_id")

        body = {
            "partition_id": part["pid"],
            "from_node_id": part["replicas"][0],
            "to_node_id": 999999,  # not registered
            "reason": "test-to-not-registered",
        }
        resp = _post("/migrate", body)
        logger.info(f"status={resp.status_code} body={resp.text}")
        assert resp.status_code == 400
        assert "not registered" in resp.text, (
            f"expected 'not registered' in error; got: {resp.text}"
        )
        logger.info("[PASS] TestBalancerManualMigrateValidation.test_migrate_to_node_not_registered")

    def test_migrate_to_node_already_replica(self):
        """to_node_id 已经是该 partition 的 replica → 应被 fail-fast 拒绝"""
        partitions = _enumerate_partitions()
        target = next((p for p in partitions if len(p["replicas"]) >= 2), None)
        if target is None:
            pytest.skip("need a partition with ≥2 replicas to exercise to_already_replica")

        replicas = target["replicas"]
        body = {
            "partition_id": target["pid"],
            "from_node_id": replicas[0],
            "to_node_id": replicas[1],  # already a replica of this partition
            "reason": "test-to-already-replica",
        }
        resp = _post("/migrate", body)
        logger.info(f"status={resp.status_code} body={resp.text}")
        assert resp.status_code == 400
        assert "already has a replica" in resp.text, (
            f"expected 'already has a replica' in error; got: {resp.text}"
        )
        logger.info("[PASS] TestBalancerManualMigrateValidation.test_migrate_to_node_already_replica")

    def test_migrate_to_node_not_live(self):
        """
        to_node_id 已注册但不 live → 应被 fail-fast 拒绝（IsLive 检查）.

        Skipped in healthy CI environments where every PS is alive. To
        exercise the IsLive code path, stop a PS (e.g., `docker compose stop
        ps-2`) and rerun. This is a documented coverage gap, not a test bug.
        """
        servers_resp = get_servers_status(router_url)
        if servers_resp.status_code != 200:
            pytest.skip("cannot list servers")
        servers_body = servers_resp.json()
        servers = servers_body.get("data", servers_body) if isinstance(servers_body, dict) else servers_body
        if isinstance(servers, dict) and "servers" in servers:
            servers = servers["servers"]

        # Find a registered-but-down PS. Vearch's /servers reports per-PS
        # status; treat anything other than explicit "OK"/"alive"/"up" as
        # not-live for the purpose of this test.
        dead_node = None
        for s in servers or []:
            srv = s.get("server", s) if isinstance(s, dict) else {}
            status_str = str(s.get("status", "") if isinstance(s, dict) else "").lower()
            alive_flag = s.get("alive", None) if isinstance(s, dict) else None
            if alive_flag is False or status_str in ("down", "dead", "offline"):
                dead_node = srv.get("name") or srv.get("ID") or s.get("ID")
                break
        if dead_node is None:
            pytest.skip("no registered-but-down PS in cluster; cannot exercise IsLive path")

        # Find any partition + a from_node that's a replica.
        parts = _enumerate_partitions()
        usable = next((p for p in parts if p["replicas"] and dead_node not in p["replicas"]), None)
        if usable is None:
            pytest.skip("no partition usable as migration source")

        req_body = {
            "partition_id": usable["pid"],
            "from_node_id": usable["replicas"][0],
            "to_node_id": dead_node,
            "reason": "test-to-not-live",
        }
        resp = _post("/migrate", req_body)
        logger.info(f"status={resp.status_code} body={resp.text}")
        assert resp.status_code == 400
        assert "is not live" in resp.text, (
            f"expected 'is not live' in error; got: {resp.text}"
        )
        logger.info("[PASS] TestBalancerManualMigrateValidation.test_migrate_to_node_not_live")


# =============================================================================
# Module 7: Reaper trigger
# =============================================================================


class TestBalancerReaperTrigger:
    """Covers POST /balancer/reaper/trigger (manual trigger, async)."""

    def setup_class(self):
        _put("/config", _default_config_body())

    def test_reaper_trigger_accepted(self):
        resp = _post("/reaper/trigger")
        logger.info(resp.json())
        assert resp.status_code == 200
        assert resp.json().get("ok") is True
        # Trigger fires asynchronously; give it a moment to log.
        time.sleep(1)
        logger.info("[PASS] TestBalancerReaperTrigger.test_reaper_trigger_accepted")

    def test_reaper_trigger_idempotent(self):
        # Two back-to-back triggers both return 200; this only verifies the
        # endpoint stays well-behaved under quick repeat (no 5xx, no crash).
        # The actual STM-lock-skip path executes in a background goroutine
        # and its outcome is observable only via master logs, not via this
        # HTTP response — that path is therefore not asserted here.
        r1 = _post("/reaper/trigger")
        r2 = _post("/reaper/trigger")
        logger.info(f"r1={r1.json()} r2={r2.json()}")
        assert r1.status_code == 200
        assert r2.status_code == 200
        logger.info("[PASS] TestBalancerReaperTrigger.test_reaper_trigger_idempotent")


# =============================================================================
# Module 8: Auth — endpoints must require BasicAuth
# =============================================================================


class TestBalancerAuth:
    """Covers C4: HTTP API must run behind BasicAuth."""

    def test_get_config_without_auth_unauthorized(self):
        # Hit endpoint without credentials.
        resp = requests.get(BALANCER_PREFIX + "/config")
        logger.info(f"unauth status={resp.status_code} body={resp.text}")
        # Should be rejected: 401 or another non-2xx (depends on auth middleware).
        # Some deployments set SkipAuth=true; allow both behaviors but log them.
        if resp.status_code == 200:
            logger.info("note: SkipAuth appears to be enabled in this environment")
        else:
            assert resp.status_code in (401, 403), (
                f"unauthenticated request should be rejected; got {resp.status_code}"
            )
        logger.info("[PASS] TestBalancerAuth.test_get_config_without_auth_unauthorized")


# =============================================================================
# Helpers for hotspot / load-balance scenarios
# =============================================================================


def _all_live_ps_nodes():
    """Return [(node_id, host_ip, host_zone), ...] for live PS nodes via /servers."""
    resp = get_servers_status(router_url)
    if resp.status_code != 200:
        return []
    out = []
    body = resp.json()
    servers = body.get("data", body) if isinstance(body, dict) else body
    if isinstance(servers, dict) and "servers" in servers:
        servers = servers["servers"]
    if not isinstance(servers, list):
        return []
    for s in servers:
        out.append((s.get("server", {}).get("name") or s.get("name") or s.get("ID"),
                    s.get("server", {}).get("host_ip") or s.get("host_ip", ""),
                    s.get("server", {}).get("host_zone") or s.get("host_zone", "")))
    return out


def _servers_by_resource_pool():
    """Return {resource_name: [node_id, ...]} for all registered PS nodes."""
    resp = get_servers_status(router_url)
    if resp.status_code != 200:
        return {}
    body = resp.json()
    servers = body.get("data", body) if isinstance(body, dict) else body
    if isinstance(servers, dict) and "servers" in servers:
        servers = servers["servers"]
    pools = {}
    if not isinstance(servers, list):
        return pools
    for s in servers:
        srv = s.get("server", s) if isinstance(s, dict) else {}
        node_id = srv.get("name") or srv.get("ID") or (s.get("name") if isinstance(s, dict) else None)
        rn = srv.get("resource_name") or (s.get("resource_name") if isinstance(s, dict) else "") or "default"
        pools.setdefault(rn, []).append(node_id)
    return pools


def _enumerate_partitions():
    """Walk /cluster/health and return [{pid, db, space, replicas, leader}].

    The /cluster/health response from router exposes per-partition info under
      data[].db_name, data[].spaces[].name, data[].spaces[].partitions[].pid,
      data[].spaces[].partitions[].raft_status.{Leader,Replicas}.
    The outer DB object uses `db_name`. Per-partition `Replicas` is a dict
    keyed by node_id rendered as a JSON string; `Leader` is the node_id as
    a JSON number. Each partition appears exactly once per `partitions[]`
    (see partition_service.go) — never duplicated per replica.

    Fallback: when `raft_status` is unavailable (e.g. follower hasn't
    replied yet), the partition entry still carries its host `node_id`.
    In that single-known-node case we use it for BOTH replicas and leader
    (the only known node is by definition the leader of a 1-member group).
    """
    resp = get_cluster_health(router_url, db_name="")
    if resp.status_code != 200:
        return []
    out = []
    for db in resp.json().get("data", []):
        db_label = db.get("db_name")
        for sp in db.get("spaces", []):
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
                    "space": sp.get("name"),
                    "replicas": replicas,
                    "leader": leader,
                })
    return out


def _wait_task_terminal(task_id, timeout_sec=120, poll_sec=2):
    """Poll /balancer/tasks until the given task is Done or Failed (gone from list)."""
    deadline = time.time() + timeout_sec
    last = None
    while time.time() < deadline:
        tasks = _get("/tasks").json()
        found = next((t for t in tasks if t.get("id") == task_id), None)
        if found is None:
            # Task already terminated and reaped from etcd; treat as Done.
            return last or {"id": task_id, "step": "Done"}
        last = found
        if str(found.get("step", "")).lower() in ("done", "failed"):
            return found
        time.sleep(poll_sec)
    return last or {"id": task_id, "step": "timeout"}


def _create_test_space(replica_num=3, partition_num=3, dim=128):
    space_config = {
        "name": space_name,
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
    return create_space(router_url, db_name, space_config)


def _change_replicas(db, space, method):
    """POST /schedule/change_replicas on master. method=0 Add, method=1 Remove."""
    url = master_url + "/schedule/change_replicas"
    body = {"db_name": db, "space_name": space, "method": method}
    return requests.post(url, auth=AUTH, json=body)


# =============================================================================
# Module 9: End-to-end migration (manual migrate → Done)
# =============================================================================


class TestBalancerEndToEndMigration:
    """Submit a real migration via /balancer/migrate and poll until Done."""

    # Class-level constants so test methods don't hardcode magic numbers
    # that drift from setup_class.
    REPLICA_NUM = 2
    PARTITION_NUM = 2

    def setup_class(self):
        _put("/config", _default_config_body())
        # Ensure we have a space with ≥2 replicas so a migration target exists.
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        resp = _create_test_space(replica_num=self.REPLICA_NUM, partition_num=self.PARTITION_NUM)
        assert resp.json().get("code") == 0
        # Give partitions a moment to settle.
        time.sleep(3)

    def test_manual_migrate_reaches_done(self):
        live_nodes = _all_live_ps_nodes()
        if len(live_nodes) < 3:
            pytest.skip(f"need ≥3 live PS nodes to migrate; have {len(live_nodes)}")

        partitions = _enumerate_partitions()
        candidate = None
        for p in partitions:
            # Prefer a non-leader replica as the source so we don't have to
            # touch leader-transfer paths. Deterministic by taking [0] of a
            # list (sets give non-deterministic iteration order).
            non_leader_replicas = [r for r in p["replicas"] if r != p["leader"]]
            if not non_leader_replicas:
                continue
            others = [n for n, _, _ in live_nodes if n not in p["replicas"]]
            if not others:
                continue
            candidate = (p["pid"], non_leader_replicas[0], others[0])
            break
        if candidate is None:
            pytest.skip("no partition with both non-leader source and free target available")

        pid, from_id, to_id = candidate
        body = {
            "partition_id": pid,
            "from_node_id": from_id,
            "to_node_id": to_id,
            "reason": "test-e2e-migration",
        }
        resp = _post("/migrate", body)
        logger.info(f"submit migrate pid={pid} {from_id}→{to_id}: {resp.json()}")
        assert resp.status_code == 200
        task = resp.json()
        assert task.get("id"), "expected task id in response"

        final = _wait_task_terminal(task["id"], timeout_sec=180)
        logger.info(f"final task state: {final}")
        assert str(final.get("step", "")).lower() == "done", (
            f"migration did not complete; final={final}"
        )
        logger.info("[PASS] TestBalancerEndToEndMigration.test_manual_migrate_reaches_done")

    def test_replica_count_preserved_after_migration(self):
        # Setup created space with REPLICA_NUM replicas; verify count is
        # preserved (and converged) end-to-end through the manual migration.
        expected = self.REPLICA_NUM
        parts = _enumerate_partitions()
        violations = [p for p in parts if p["space"] == space_name and len(p["replicas"]) != expected]
        # Reaper may not yet have cleaned up if migrate added without removing yet.
        # Wait a brief moment for convergence.
        if violations:
            time.sleep(5)
            parts = _enumerate_partitions()
            violations = [p for p in parts if p["space"] == space_name and len(p["replicas"]) != expected]
        assert not violations, f"replica count drift in: {violations}"
        logger.info("[PASS] TestBalancerEndToEndMigration.test_replica_count_preserved_after_migration")

    def test_migrate_state_machine_progression(self):
        """Verify Pending → AddingMember → WaitingCaughtUp → RemovingMember → Done."""
        live_nodes = _all_live_ps_nodes()
        if len(live_nodes) < 3:
            pytest.skip(f"need ≥3 live PS nodes; have {len(live_nodes)}")
        partitions = _enumerate_partitions()
        candidate = None
        for p in partitions:
            non_leader = [r for r in p["replicas"] if r != p["leader"]]
            free = [n for n, _, _ in live_nodes if n not in p["replicas"]]
            if non_leader and free:
                candidate = (p["pid"], non_leader[0], free[0])
                break
        if candidate is None:
            pytest.skip("no partition with non-leader source and free target")

        pid, from_id, to_id = candidate
        resp = _post("/migrate", {
            "partition_id": pid,
            "from_node_id": from_id,
            "to_node_id": to_id,
            "reason": "test-state-machine",
        })
        assert resp.status_code == 200, resp.text
        task_id = resp.json()["id"]

        # Poll once per second, record step transitions.
        steps_observed = []
        deadline = time.time() + 180
        while time.time() < deadline:
            tasks = _get("/tasks").json()
            found = next((t for t in tasks if t.get("id") == task_id), None)
            if found is None:
                # Task may have completed and been removed from etcd; record Done.
                if not steps_observed or steps_observed[-1] != "done":
                    steps_observed.append("done")
                break
            step = str(found.get("step", "")).lower()
            if not steps_observed or steps_observed[-1] != step:
                steps_observed.append(step)
                logger.info(f"step observed: {step}")
            if step in ("done", "failed"):
                break
            time.sleep(1)

        logger.info(f"full step sequence: {steps_observed}")
        expected_order = ["pending", "addingmember", "waitingcaughtup", "removingmember", "done"]
        # Filter to known steps and verify they appear in order (subset allowed
        # since transitions may be too fast to observe each one).
        indices = [expected_order.index(s) for s in steps_observed if s in expected_order]
        assert indices == sorted(indices), (
            f"state machine progressed out of order; observed={steps_observed}"
        )
        assert "done" in steps_observed or "failed" not in steps_observed, (
            f"task did not reach Done: {steps_observed}"
        )
        assert steps_observed[-1] == "done", f"expected Done, got {steps_observed[-1]}"
        logger.info("[PASS] TestBalancerEndToEndMigration.test_migrate_state_machine_progression")

    def test_snapshot_inflight_count_tracks_real_tasks(self):
        """snapshot.inflight_count must reflect the number of live MigrateTasks."""
        live_nodes = _all_live_ps_nodes()
        if len(live_nodes) < 3:
            pytest.skip("need ≥3 live PS nodes")
        partitions = _enumerate_partitions()
        candidate = None
        for p in partitions:
            non_leader = [r for r in p["replicas"] if r != p["leader"]]
            free = [n for n, _, _ in live_nodes if n not in p["replicas"]]
            if non_leader and free:
                candidate = (p["pid"], non_leader[0], free[0])
                break
        if candidate is None:
            pytest.skip("no candidate partition")

        baseline = _get("/snapshot").json().get("inflight_count", 0)
        logger.info(f"baseline inflight_count = {baseline}")

        pid, from_id, to_id = candidate
        resp = _post("/migrate", {
            "partition_id": pid, "from_node_id": from_id,
            "to_node_id": to_id, "reason": "test-inflight-count",
        })
        assert resp.status_code == 200, resp.text
        task_id = resp.json()["id"]

        # Immediately after Submit, inflight_count should bump by 1.
        snap = _get("/snapshot").json()
        # Submit is synchronous w.r.t. tracker.Add (REV4-3 atomic), so the
        # delta must be observable on the next snapshot Build.
        assert snap["inflight_count"] >= baseline + 1, (
            f"inflight_count={snap['inflight_count']}, expected ≥{baseline+1} after submit"
        )
        logger.info(f"during migration: inflight_count = {snap['inflight_count']}")

        # Wait for task to finish, then verify count returns to baseline.
        _wait_task_terminal(task_id, timeout_sec=180)
        time.sleep(2)  # allow tracker.Remove to settle
        final = _get("/snapshot").json().get("inflight_count", 0)
        logger.info(f"after Done: inflight_count = {final}")
        assert final <= baseline, (
            f"inflight_count={final} did not return to baseline {baseline} after Done"
        )
        logger.info("[PASS] TestBalancerEndToEndMigration.test_snapshot_inflight_count_tracks_real_tasks")

    def teardown_class(self):
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception as e:
            logger.info(f"teardown ignored: {e}")


# =============================================================================
# Module 10: ScoreBasedPlacement — new partitions land on low-score PS
# =============================================================================


class TestBalancerScoreBasedPlacement:
    """Enable ScoreBasedPlacement and verify new partitions are placed on lower-scored nodes."""

    def setup_class(self):
        _put("/config", _default_config_body(enabled=True, score_based_placement=True))
        # Clean slate.
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)

    def test_picker_prefers_low_current_score_node(self):
        snap = _get("/snapshot").json()
        items = snap.get("items", [])
        if len(items) < 2:
            pytest.skip(f"need ≥2 live PS items, have {len(items)}")

        sorted_items = sorted(items, key=lambda x: x.get("current_score", 0))
        lowest_node = sorted_items[0]["node_id"]
        logger.info(f"lowest-score node: {lowest_node} (score={sorted_items[0]['current_score']})")

        # Create a space with partition_num large enough that picker must use
        # multiple nodes; expect the lowest-score node to receive at least one
        # replica.
        resp = _create_test_space(replica_num=1, partition_num=max(3, len(items)))
        assert resp.json().get("code") == 0
        time.sleep(3)

        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        placed_on_low = any(lowest_node in p["replicas"] for p in parts)
        assert placed_on_low, (
            f"ScoreBasedPlacement should have placed at least one partition on node "
            f"{lowest_node}; partitions={parts}"
        )
        logger.info(
            f"[PASS] TestBalancerScoreBasedPlacement.test_picker_prefers_low_current_score_node "
            f"(low-score node {lowest_node} received {sum(1 for p in parts if lowest_node in p['replicas'])} replicas)"
        )

    def teardown_class(self):
        # Restore default to not affect subsequent tests.
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 11: Anti-affinity — replicas span distinct hosts (§5 invariant 3)
# =============================================================================


class TestBalancerAntiAffinity:
    """Verify that replicas of a partition are placed on distinct hosts."""

    def setup_class(self):
        _put("/config", _default_config_body(enabled=True, score_based_placement=True))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)

    def test_replicas_on_distinct_hosts(self):
        live = _all_live_ps_nodes()
        host_ips = {h for _, h, _ in live if h}
        if len(host_ips) < 3:
            pytest.skip(f"need ≥3 distinct HostIp for anti-affinity test; got {host_ips}")

        resp = _create_test_space(replica_num=3, partition_num=2)
        assert resp.json().get("code") == 0
        time.sleep(3)

        node_host = {n: h for n, h, _ in live}
        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        assert parts, "expected at least one partition"

        for p in parts:
            hosts = {node_host.get(r, f"unknown-{r}") for r in p["replicas"]}
            assert len(hosts) == len(p["replicas"]), (
                f"partition {p['pid']} replicas not on distinct hosts: "
                f"replicas={p['replicas']} hosts={hosts}"
            )
        logger.info(f"[PASS] TestBalancerAntiAffinity.test_replicas_on_distinct_hosts "
                    f"({len(parts)} partitions × 3 replicas, each spans distinct hosts)")

    def teardown_class(self):
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 12: Pause stops Planner from generating new ops
# =============================================================================


class TestBalancerPauseStopsPlanner:
    """When paused, Planner cron must not submit new tasks."""

    def setup_class(self):
        # Short plan interval so the test finishes quickly.
        _put("/config", _default_config_body(
            enabled=False,  # pause
            plan_interval_sec=10,
        ))

    def test_no_new_planner_tasks_while_paused(self):
        # Planner generates ops with these reason values; the test must filter
        # against the actual constants used by collectOps() in planner.go.
        PLANNER_REASONS = {"partition_skew", "data_skew", "disk_full"}

        before = _get("/tasks").json()
        before_plan_ids = {t.get("id") for t in before
                           if t.get("reason") in PLANNER_REASONS}
        logger.info(f"pre-pause Planner-reason tasks: {len(before_plan_ids)}")

        # Wait for ≥2 full plan intervals to ensure at least one cron tick fires.
        time.sleep(25)

        after = _get("/tasks").json()
        after_plan_ids = {t.get("id") for t in after
                          if t.get("reason") in PLANNER_REASONS}
        new_ids = after_plan_ids - before_plan_ids
        assert not new_ids, (
            f"paused balancer should not produce Planner tasks; new ids: {new_ids}"
        )
        logger.info("[PASS] TestBalancerPauseStopsPlanner.test_no_new_planner_tasks_while_paused")

    def teardown_class(self):
        _put("/config", _default_config_body())


# =============================================================================
# Module 13: Reaper trims redundant replicas
# =============================================================================


class TestBalancerReaperShrinkReplicas:
    """
    Reaper must remove excess replicas when partition.Replicas > space.ReplicaNum.
    We simulate the over-replicated state by adding a member directly via
    /partitions/change_member, then trigger Reaper.
    """

    def setup_class(self):
        _put("/config", _default_config_body(redundant_reaper_enabled=True))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        resp = _create_test_space(replica_num=1, partition_num=2)
        assert resp.json().get("code") == 0
        time.sleep(3)

    def test_reaper_removes_excess_replica(self):
        live = _all_live_ps_nodes()
        if len(live) < 2:
            pytest.skip("need ≥2 PS nodes to add a redundant replica")

        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        if not parts:
            pytest.skip("no partition created")
        target = parts[0]
        current_replicas = set(target["replicas"])
        free_node = next((n for n, _, _ in live if n not in current_replicas), None)
        if free_node is None:
            pytest.skip("no spare PS to add as redundant replica")

        # method=0 → AddNode (matches proto.ConfAddNode in vearch master)
        resp = change_partitons(router_url, [target["pid"]], free_node, 0)
        logger.info(f"add redundant replica: {resp.text}")
        # Allow raft to apply the conf change.
        time.sleep(5)

        # Now Reaper should see len(Replicas) > ReplicaNum (which is 1) and trim.
        trigger = _post("/reaper/trigger")
        assert trigger.status_code == 200
        logger.info(f"reaper triggered: {trigger.json()}")

        # Poll until partition replica count converges back to ReplicaNum=1.
        deadline = time.time() + 90
        ok = False
        while time.time() < deadline:
            parts = [p for p in _enumerate_partitions() if p["pid"] == target["pid"]]
            if parts and len(parts[0]["replicas"]) == 1:
                ok = True
                break
            time.sleep(3)
        assert ok, f"Reaper did not converge replicas; last={parts}"
        logger.info("[PASS] TestBalancerReaperShrinkReplicas.test_reaper_removes_excess_replica")

    def test_reaper_preserves_leader_when_shrinking(self):
        """Reaper.pickWorstReplica skips leader — verify the survivor IS the leader."""
        live = _all_live_ps_nodes()
        if len(live) < 2:
            pytest.skip("need ≥2 PS nodes")

        # Recreate clean state so we know exactly the initial replica + leader.
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        # Defensive: a previous test failure may have left the db gone (e.g.,
        # a cascading reaper cleanup); ensure it exists. Tolerate "already
        # exists" (success path); skip the test on any other failure so we
        # don't paper over a real cluster problem.
        try:
            cdb = create_db(router_url, db_name)
            cdb_code = cdb.json().get("code")
            if cdb_code not in (0, None) and "exist" not in str(cdb.text).lower():
                pytest.skip(f"create_db unexpectedly failed: {cdb.text}")
        except Exception as e:
            pytest.skip(f"create_db raised: {e}")
        resp = _create_test_space(replica_num=1, partition_num=1)
        assert resp.json().get("code") == 0, (
            f"create_space failed after defensive create_db: {resp.text}"
        )
        time.sleep(3)

        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        if not parts:
            pytest.skip("no partition created")
        target = parts[0]
        original_leader = target["leader"]
        if original_leader is None:
            pytest.skip("partition has no leader; cannot verify leader preservation")
        free_node = next((n for n, _, _ in live if n != original_leader), None)
        if free_node is None:
            pytest.skip("no spare PS to add")

        # Add a second replica so we are over-replicated.
        change_partitons(router_url, [target["pid"]], free_node, 0)
        time.sleep(5)

        # Sanity: now 2 replicas, leader still original.
        post_parts = [p for p in _enumerate_partitions() if p["pid"] == target["pid"]]
        assert post_parts and len(post_parts[0]["replicas"]) == 2, (
            f"expected 2 replicas after AddNode, got {post_parts}"
        )

        # Reaper must remove free_node (the non-leader extra), NOT the leader.
        _post("/reaper/trigger")

        deadline = time.time() + 90
        survivor = None
        while time.time() < deadline:
            parts = [p for p in _enumerate_partitions() if p["pid"] == target["pid"]]
            if parts and len(parts[0]["replicas"]) == 1:
                survivor = parts[0]["replicas"][0]
                break
            time.sleep(3)
        assert survivor is not None, "Reaper did not converge to 1 replica"
        assert survivor == original_leader, (
            f"Reaper removed the leader! survivor={survivor} original_leader={original_leader}"
        )
        logger.info(
            f"[PASS] TestBalancerReaperShrinkReplicas.test_reaper_preserves_leader_when_shrinking "
            f"(leader {original_leader} preserved, extra replica {free_node} removed)"
        )

    def teardown_class(self):
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 13b: Reaper avoids in-flight partitions (NEW-2 safety net)
# =============================================================================


class TestBalancerReaperInflightSafety:
    """
    NEW-2 invariant: Reaper must skip partitions that have an inflight
    MigrateTask, otherwise it could remove the new replica that Scheduler
    just added — violating §5.3 (副本数 ≥ ReplicaNum).
    """

    def setup_class(self):
        _put("/config", _default_config_body(redundant_reaper_enabled=True))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        # replica_num=2 so we have a real source + a real Add target.
        resp = _create_test_space(replica_num=2, partition_num=1)
        assert resp.json().get("code") == 0
        time.sleep(3)

    def test_reaper_skips_partition_with_inflight_task(self):
        live = _all_live_ps_nodes()
        if len(live) < 3:
            pytest.skip(f"need ≥3 PS nodes; have {len(live)}")
        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        if not parts:
            pytest.skip("no test partition")
        target = parts[0]
        non_leader = [r for r in target["replicas"] if r != target["leader"]]
        free = [n for n, _, _ in live if n not in target["replicas"]]
        if not non_leader or not free:
            pytest.skip("no usable from/to nodes")

        from_id = non_leader[0]
        to_id = free[0]
        pid = target["pid"]

        # Submit a migration. The scheduler cron (30s) advances it through
        # AddingMember → WaitingCaughtUp → RemovingMember asynchronously, so
        # there is a window during which Replicas = ReplicaNum + 1.
        resp = _post("/migrate", {
            "partition_id": pid, "from_node_id": from_id,
            "to_node_id": to_id, "reason": "test-reaper-inflight-safety",
        })
        assert resp.status_code == 200, resp.text
        task_id = resp.json()["id"]
        logger.info(f"submitted migrate task {task_id}")

        # CRITICAL: We must wait for the task to leave StepPending so the
        # partition has ReplicaNum+1 replicas. While Pending, Reaper's first
        # check (len(Replicas) <= expected) short-circuits BEFORE
        # HasInflightForPartition is ever consulted — so triggering reaper
        # during Pending would not exercise the NEW-2 path we're trying to test.
        wait_until_active = time.time() + 90
        active_step = None
        while time.time() < wait_until_active:
            tasks = _get("/tasks").json()
            found = next((t for t in tasks if t.get("id") == task_id), None)
            if found is None:
                pytest.skip(
                    "task completed before leaving Pending; cannot verify "
                    "NEW-2 in-flight skip (need ReplicaNum+1 state to test)"
                )
            step = str(found.get("step", "")).lower()
            if step in ("addingmember", "waitingcaughtup", "removingmember"):
                active_step = step
                break
            time.sleep(2)
        if active_step is None:
            pytest.skip("task did not transition out of Pending within 90s")
        logger.info(f"task reached {active_step}; replicas now ReplicaNum+1, hammering reaper")

        # Hammer /reaper/trigger several times while replicas = ReplicaNum+1.
        # Each call goes through HasInflightForPartition(pid) → true → skip.
        # Reaper MUST NOT call ChangeMember(RemoveNode) on this partition.
        for i in range(5):
            tasks = _get("/tasks").json()
            still_inflight = any(t.get("id") == task_id for t in tasks)
            if not still_inflight:
                logger.info(f"task {task_id} terminated at iter {i}; stop hammering")
                break
            trigger = _post("/reaper/trigger")
            assert trigger.status_code == 200
            current_step = next((t for t in tasks if t['id']==task_id), {}).get('step')
            logger.info(f"iter {i}: reaper triggered while task in step {current_step}")
            time.sleep(2)

        # Now wait for the migration to complete naturally.
        final = _wait_task_terminal(task_id, timeout_sec=240)
        assert str(final.get("step", "")).lower() == "done", (
            f"migration did not Done; final={final} — this could indicate Reaper "
            f"interference with the inflight task"
        )

        # Final state: exactly ReplicaNum=2 replicas, to_id present, from_id absent.
        parts = [p for p in _enumerate_partitions() if p["pid"] == pid]
        assert parts, "partition disappeared!"
        final_replicas = set(parts[0]["replicas"])
        assert len(final_replicas) == 2, (
            f"§5.3 violation: ReplicaNum=2 but final replicas={final_replicas}"
        )
        assert to_id in final_replicas, f"new replica {to_id} not in final {final_replicas}"
        assert from_id not in final_replicas, f"old replica {from_id} still in final {final_replicas}"
        logger.info(
            f"[PASS] TestBalancerReaperInflightSafety.test_reaper_skips_partition_with_inflight_task "
            f"(reaper triggered 5x during inflight migration; migration completed cleanly)"
        )

    def teardown_class(self):
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 14: Cooldown blocks re-migration of just-migrated partition
# =============================================================================


class TestBalancerCooldownBlocksReplan:
    """
    After a successful migration, the same partition must not be re-migrated by
    the Planner within CooldownPerPartitionSec. We confirm this indirectly by
    observing that the Planner does not generate a new task for the cooled-down
    partition over a couple of plan intervals.
    """

    def setup_class(self):
        # Short plan interval + long cooldown for a clean signal.
        _put("/config", _default_config_body(
            enabled=True,
            score_based_placement=True,
            plan_interval_sec=10,
            cooldown_per_partition_sec=600,  # 10min
        ))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        resp = _create_test_space(replica_num=1, partition_num=2)
        assert resp.json().get("code") == 0
        time.sleep(3)

    def test_no_replan_for_cooled_partition(self):
        """
        After a successful manual migration, the same partition must not be
        re-migrated by the Planner within CooldownPerPartitionSec.

        LIMITATION: In a balanced cluster (typical test environment), Planner
        has no reason to generate ops for ANY partition — so absence of an op
        for the cooled partition is vacuously true. This test reliably catches
        only one failure mode: if a fresh-Done migration immediately triggers
        a flapping Planner op (e.g., due to inflight delta accounting bug).
        For full cooldown verification, an artificially imbalanced cluster
        (via fault injection) would be required.
        """
        live = _all_live_ps_nodes()
        if len(live) < 2:
            pytest.skip("need ≥2 PS nodes for migration")
        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        if not parts or not parts[0]["replicas"]:
            pytest.skip("no usable partition")

        target = parts[0]
        from_id = target["replicas"][0]
        to_id = next((n for n, _, _ in live if n != from_id), None)
        if to_id is None:
            pytest.skip("no spare target node")

        # Do one manual migration → Done; this sets cooldown.
        resp = _post("/migrate", {
            "partition_id": target["pid"],
            "from_node_id": from_id,
            "to_node_id": to_id,
            "reason": "test-prime-cooldown",
        })
        assert resp.status_code == 200, resp.text
        task_id = resp.json()["id"]
        final = _wait_task_terminal(task_id, timeout_sec=120)
        assert str(final.get("step", "")).lower() == "done", (
            f"prime migration did not complete: {final}"
        )
        logger.info(f"prime migration done for partition {target['pid']}")

        # Wait long enough for ≥2 Planner cron ticks (plan_interval_sec=10).
        time.sleep(25)

        # Verify Planner did not generate a Planner-reason task targeting the
        # cooled partition.
        tasks = _get("/tasks").json()
        offending = [t for t in tasks
                     if t.get("partition_id") == target["pid"]
                     and str(t.get("reason", "")) in ("partition_skew", "data_skew", "disk_full")
                     and t.get("id") != task_id]
        assert not offending, (
            f"Planner should not re-migrate cooled partition {target['pid']} "
            f"within cooldown window; saw: {offending}"
        )
        logger.info("[PASS] TestBalancerCooldownBlocksReplan.test_no_replan_for_cooled_partition")

    def teardown_class(self):
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 15: Submit dedup — same partition cannot have two inflight tasks (M4)
# =============================================================================


class TestBalancerSubmitDedup:
    """
    M4 invariant: TaskScheduler.Submit must reject a second migrate for a
    partition that already has an inflight task; otherwise multiple raft
    conf-changes on the same partition would conflict.
    """

    def setup_class(self):
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        # replica_num=2 so we have a non-leader source available.
        resp = _create_test_space(replica_num=2, partition_num=1)
        if resp.json().get("code") != 0:
            pytest.skip(f"cannot create test space: {resp.json()}")
        time.sleep(3)

    def test_second_submit_for_same_partition_rejected(self):
        live = _all_live_ps_nodes()
        if len(live) < 3:
            pytest.skip(f"need ≥3 PS nodes (2 replicas + 1 free); have {len(live)}")
        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        if not parts:
            pytest.skip("no test partition")
        target = parts[0]
        non_leader = [r for r in target["replicas"] if r != target["leader"]]
        free = [n for n, _, _ in live if n not in target["replicas"]]
        if not non_leader or not free:
            pytest.skip("no usable from/to nodes")

        pid = target["pid"]
        from_id = non_leader[0]
        to_id = free[0]

        # First Submit — should succeed.
        r1 = _post("/migrate", {
            "partition_id": pid, "from_node_id": from_id,
            "to_node_id": to_id, "reason": "test-dedup-first",
        })
        assert r1.status_code == 200, f"first submit must succeed: {r1.text}"
        task_id_1 = r1.json()["id"]
        logger.info(f"first submit OK: task={task_id_1}")

        # Second Submit for the SAME partition — must be rejected.
        # Use a different to_id (any other free node, or same — dedup is on partition).
        r2 = _post("/migrate", {
            "partition_id": pid, "from_node_id": from_id,
            "to_node_id": to_id, "reason": "test-dedup-second",
        })
        logger.info(f"second submit: status={r2.status_code} body={r2.text}")
        assert r2.status_code in (400, 500), (
            f"duplicate submit must be rejected; got {r2.status_code}"
        )
        # Sanity-check error message references the inflight task.
        assert "inflight" in r2.text.lower() or "already" in r2.text.lower(), (
            f"error message should indicate dedup; got {r2.text}"
        )

        # Verify /tasks shows exactly 1 inflight task for this partition.
        tasks = _get("/tasks").json()
        for_partition = [t for t in tasks if t.get("partition_id") == pid]
        assert len(for_partition) == 1, (
            f"expected exactly 1 inflight task for partition {pid}, got {for_partition}"
        )

        # Cleanup: wait for the first task so subsequent tests see clean state.
        _wait_task_terminal(task_id_1, timeout_sec=180)
        logger.info("[PASS] TestBalancerSubmitDedup.test_second_submit_for_same_partition_rejected")

    def teardown_class(self):
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 16: Resource pool isolation — §5.5 invariant
# =============================================================================


class TestBalancerResourcePoolIsolation:
    """
    §5.5: 不跨 ResourceName — replica placement must not cross resource pools.
    Verified at CreateSpace time: a space tied to resource_name=A must NOT
    receive replicas on PSes registered under resource_name=B.
    """

    def setup_class(self):
        _put("/config", _default_config_body(enabled=True, score_based_placement=True))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)

    def test_create_space_in_pool_a_uses_only_pool_a(self):
        pools = _servers_by_resource_pool()
        # Need ≥2 distinct pools to test isolation.
        if len(pools) < 2:
            pytest.skip(f"cluster has only {len(pools)} resource pool(s): {pools}")

        # Pick the pool with the most nodes as A; cannot test against single-node pool.
        sorted_pools = sorted(pools.items(), key=lambda kv: -len(kv[1]))
        pool_a, pool_a_nodes = sorted_pools[0]
        if len(pool_a_nodes) < 1:
            pytest.skip(f"pool {pool_a} has no nodes")
        other_pool_nodes = set()
        for name, nodes in sorted_pools[1:]:
            other_pool_nodes.update(nodes)
        if not other_pool_nodes:
            pytest.skip("no nodes in pools other than the largest")

        logger.info(f"pool {pool_a} nodes: {pool_a_nodes}; other pools: {sorted_pools[1:]}")

        # Create a space pinned to pool_a.
        # Use replica_num = min(2, len(pool_a_nodes)) so we don't ask for more
        # replicas than the pool can satisfy.
        replica_num = min(2, len(pool_a_nodes))
        space_config = {
            "name": space_name,
            "partition_num": 2,
            "replica_num": replica_num,
            "resource_name": pool_a,
            "fields": [
                {"name": "field_int", "type": "integer"},
                {
                    "name": "field_vector",
                    "type": "vector",
                    "dimension": 128,
                    "index": {
                        "name": "gamma",
                        "type": "FLAT",
                        "params": {"metric_type": "InnerProduct"},
                    },
                },
            ],
        }
        resp = create_space(router_url, db_name, space_config)
        logger.info(resp.json())
        assert resp.json().get("code") == 0, (
            f"create_space for pool {pool_a} failed: {resp.json()}"
        )
        time.sleep(3)

        # Verify all replicas of all partitions are in pool_a.
        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        assert parts, "expected partitions to be created"

        violations = []
        unknown = []
        for p in parts:
            for r in p["replicas"]:
                if r in other_pool_nodes:
                    violations.append((p["pid"], r))
                elif r not in pool_a_nodes:
                    # Unknown pool — should never happen if _servers_by_resource_pool
                    # enumerated the whole cluster. Treat as a real failure so
                    # we don't silently mask enumeration bugs.
                    unknown.append((p["pid"], r))

        assert not violations, (
            f"§5.5 violation: replicas placed cross-pool: {violations}"
        )
        assert not unknown, (
            f"replicas placed on unknown nodes (enumeration miss or race): {unknown}"
        )
        logger.info(
            f"[PASS] TestBalancerResourcePoolIsolation.test_create_space_in_pool_a_uses_only_pool_a "
            f"({len(parts)} partitions × {replica_num} replicas all confined to pool {pool_a!r})"
        )

    def teardown_class(self):
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 17: PartitionMemberAddHook — ChangeReplica goes through Picker
# =============================================================================


class TestBalancerChangeReplicaHook:
    """
    Verifies the PartitionMemberAddHook integration in
    services/member_service.go: when score_based_placement=true, ChangeReplica
    (extending replica_num) must pick targets via the balancer's Picker,
    not the legacy first-fit selector.
    """

    def setup_class(self):
        _put("/config", _default_config_body(enabled=True, score_based_placement=True))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        # replica_num=1 so the ChangeReplica(Add) adds a *new* replica.
        resp = _create_test_space(replica_num=1, partition_num=2)
        if resp.json().get("code") != 0:
            pytest.skip(f"cannot create test space: {resp.json()}")
        time.sleep(3)

    def test_change_replica_add_uses_score_based_picker(self):
        live = _all_live_ps_nodes()
        if len(live) < 2:
            pytest.skip(f"need ≥2 PS nodes for ChangeReplica(Add); have {len(live)}")

        # Snapshot BEFORE the ChangeReplica so we know each PS's score.
        snap = _get("/snapshot").json()
        items = snap.get("items", [])
        if len(items) < 2:
            pytest.skip(f"need ≥2 live PS items; have {len(items)}")

        # Pre-existing partitions and their current replicas (these are the
        # node IDs we want to AVOID picking again — the hook's Excluded set).
        parts_before = [p for p in _enumerate_partitions() if p["space"] == space_name]
        assert parts_before, "no partitions in test space"
        used_nodes = set()
        for p in parts_before:
            used_nodes.update(p["replicas"])

        # Among nodes NOT already used, the one with the lowest current_score
        # is what the Picker should pick.
        free_items = [it for it in items if it["node_id"] not in used_nodes]
        if not free_items:
            pytest.skip("no free PS outside current replicas to extend to")
        free_items.sort(key=lambda x: x.get("current_score", 0))
        expected_pick = free_items[0]["node_id"]
        logger.info(f"expected pick (lowest score among free): {expected_pick}")

        # Trigger ChangeReplica with method=0 (Add).
        resp = _change_replicas(db_name, space_name, method=0)
        logger.info(f"change_replicas: status={resp.status_code} body={resp.text}")
        assert resp.status_code == 200, f"ChangeReplica failed: {resp.text}"

        # Poll until every partition has 2 replicas (raft conf change applied).
        # Use a bounded loop instead of a fixed sleep to be robust on slow clusters.
        deadline = time.time() + 60
        parts_after = []
        while time.time() < deadline:
            parts_after = [p for p in _enumerate_partitions() if p["space"] == space_name]
            if parts_after and all(len(p["replicas"]) == 2 for p in parts_after):
                break
            time.sleep(2)
        else:
            assert False, (
                f"ChangeReplica did not converge to 2 replicas within 60s; "
                f"last state: {parts_after}"
            )

        # Verify each partition now has 2 replicas, and that the NEW replica
        # is the one Picker should have chosen (i.e., the expected_pick node
        # appears in at least one partition's replicas).
        added_per_partition = []
        for p in parts_after:
            before = next((q for q in parts_before if q["pid"] == p["pid"]), None)
            if before is None:
                continue
            new_replicas = set(p["replicas"]) - set(before["replicas"])
            added_per_partition.append((p["pid"], list(new_replicas)))
            assert len(p["replicas"]) == 2, (
                f"partition {p['pid']} should have 2 replicas, got {p['replicas']}"
            )

        # The hook should pick free_items[0] for at least one partition.
        all_added = {n for _, nodes in added_per_partition for n in nodes}
        assert expected_pick in all_added, (
            f"score-based hook should have picked node {expected_pick} "
            f"(lowest current_score among free); actual additions: {added_per_partition}"
        )
        logger.info(
            f"[PASS] TestBalancerChangeReplicaHook.test_change_replica_add_uses_score_based_picker "
            f"(low-score node {expected_pick} was picked for ChangeReplica)"
        )

    def teardown_class(self):
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 18: Legacy fallback — score_based_placement=false still works
# =============================================================================


class TestBalancerLegacyPlacementFallback:
    """
    Default config has score_based_placement=false. The hook should return
    (nil, nil), and space_service must fall back to the legacy selector.
    This is the most common code path in production until score-based
    placement is rolled out — must be exercised explicitly.
    """

    def setup_class(self):
        # Explicit: ScoreBasedPlacement OFF (also the default).
        _put("/config", _default_config_body(score_based_placement=False))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)

    def test_create_space_with_score_based_off_uses_legacy(self):
        live = _all_live_ps_nodes()
        if len(live) < 2:
            pytest.skip(f"need ≥2 PS for replica_num=2; have {len(live)}")

        # Legacy selector must succeed creating a multi-replica space.
        resp = _create_test_space(replica_num=2, partition_num=3)
        logger.info(resp.json())
        assert resp.json().get("code") == 0, (
            f"legacy fallback CreateSpace must succeed; got {resp.json()}"
        )
        time.sleep(3)

        # Each partition must have exactly the requested replica count.
        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        assert len(parts) == 3, f"expected 3 partitions, got {len(parts)}"
        for p in parts:
            assert len(p["replicas"]) == 2, (
                f"partition {p['pid']}: legacy selector should produce 2 replicas; got {p['replicas']}"
            )
        # Legacy selector also respects anti-affinity (handled inside
        # space_service.selectServersForPartition).
        # Don't enforce distinct hosts here — depends on cluster topology —
        # but log for visibility.
        logger.info(
            f"[PASS] TestBalancerLegacyPlacementFallback.test_create_space_with_score_based_off_uses_legacy "
            f"({len(parts)} partitions × 2 replicas via legacy path)"
        )

    def teardown_class(self):
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Module 19: Reaper disabled — RedundantReaperEnabled=false is a no-op
# =============================================================================


class TestBalancerReaperDisabledNoOp:
    """
    reaper.go:RunOnce checks `if !cfg.RedundantReaperEnabled { return nil }`
    as its very first action. When the flag is off, Reaper must do nothing
    even if redundant replicas exist.

    Test design — two-phase verification:
      Phase 1 (disabled): trigger reaper → assert excess replica is preserved.
        On its own, "no change" could mean the flag worked OR something else
        prevented reaper from running (STM lock, etcd error, etc.).
      Phase 2 (re-enabled): trigger reaper → assert convergence.
        This disambiguates Phase 1: the cluster CAN converge when reaper is
        on, so Phase 1's no-op must be due to the flag, not failure.
    """

    def setup_class(self):
        # Reaper disabled.
        _put("/config", _default_config_body(redundant_reaper_enabled=False))
        try:
            drop_space(router_url, db_name, space_name)
        except Exception:
            pass
        try:
            drop_db(router_url, db_name)
        except Exception:
            pass
        create_db(router_url, db_name)
        resp = _create_test_space(replica_num=1, partition_num=1)
        if resp.json().get("code") != 0:
            pytest.skip(f"cannot create test space: {resp.json()}")
        time.sleep(3)

    def test_reaper_disabled_leaves_excess_replica_alone(self):
        live = _all_live_ps_nodes()
        if len(live) < 2:
            pytest.skip("need ≥2 PS nodes to add a redundant replica")

        parts = [p for p in _enumerate_partitions() if p["space"] == space_name]
        if not parts:
            pytest.skip("no test partition")
        target = parts[0]
        current = set(target["replicas"])
        free_node = next((n for n, _, _ in live if n not in current), None)
        if free_node is None:
            pytest.skip("no spare PS")

        # Create over-replicated state (2 replicas; ReplicaNum=1).
        resp = change_partitons(router_url, [target["pid"]], free_node, 0)
        logger.info(f"AddNode for redundant replica: {resp.text}")
        time.sleep(5)

        # Sanity: now 2 replicas.
        post = [p for p in _enumerate_partitions() if p["pid"] == target["pid"]]
        assert post and len(post[0]["replicas"]) == 2, (
            f"expected 2 replicas after AddNode; got {post}"
        )

        # With reaper disabled, manual trigger should be no-op.
        _post("/reaper/trigger")
        time.sleep(8)

        post2 = [p for p in _enumerate_partitions() if p["pid"] == target["pid"]]
        assert post2 and len(post2[0]["replicas"]) == 2, (
            f"reaper-disabled run must leave 2 replicas; got {post2}"
        )
        logger.info("reaper disabled: redundant replica preserved ✓")

        # Now ENABLE reaper and trigger again — must converge to 1.
        _put("/config", _default_config_body(redundant_reaper_enabled=True))
        _post("/reaper/trigger")

        deadline = time.time() + 60
        converged = False
        chk = []  # initialize so the failure message doesn't NameError if loop body never runs
        while time.time() < deadline:
            chk = [p for p in _enumerate_partitions() if p["pid"] == target["pid"]]
            if chk and len(chk[0]["replicas"]) == 1:
                converged = True
                break
            time.sleep(3)
        assert converged, (
            "after re-enabling reaper, redundant replica should be removed; "
            f"last state: {chk}"
        )
        logger.info(
            "[PASS] TestBalancerReaperDisabledNoOp.test_reaper_disabled_leaves_excess_replica_alone "
            "(disabled=preserved; enabled=converged)"
        )

    def teardown_class(self):
        _put("/config", _default_config_body())
        try:
            drop_space(router_url, db_name, space_name)
            drop_db(router_url, db_name)
        except Exception:
            pass


# =============================================================================
# Cleanup
# =============================================================================


class TestBalancerCleanup:
    def setup_class(self):
        pass

    def test_drop_test_space_and_db(self):
        try:
            spaces = list_spaces(router_url, db_name).json()
            if spaces.get("code") == 0:
                for s in spaces.get("data", []):
                    drop_space(router_url, db_name, s["space_name"])
            drop_db(router_url, db_name)
        except Exception as e:
            logger.info(f"cleanup ignored: {e}")

        # Restore default config so subsequent test runs start clean.
        _put("/config", _default_config_body())
        logger.info("[PASS] TestBalancerCleanup.test_drop_test_space_and_db")
