#!/usr/bin/env bash
# Run test/test_balancer.py against a remote, already-deployed Vearch cluster.
#
# Default target: JD test-s3 cluster (K8s service style — no explicit port, :80)
#   master: http://vearch.example.com
#   router: http://vearch-router.example.com
#   user  = root
#
# SECURITY NOTE
#   This script includes the default password for the test-s3 cluster as a
#   convenience for the team. DO NOT use it for prod credentials. Override via
#   the PASSWORD env var, and rotate the test-s3 password if leaked.
#
# Usage:
#   bash scripts/run_tests_remote.sh                         # default: test-s3, full suite
#   bash scripts/run_tests_remote.sh --test test_cluster_master.py
#                                                            # different test file
#   bash scripts/run_tests_remote.sh -- -k TestBalancerConfig
#                                                            # extra pytest args (after `--`)
#   MASTER_URL=http://other ROUTER_URL=http://other PASSWORD=xxx \
#     bash scripts/run_tests_remote.sh                       # point elsewhere
#
# Flags:
#   --test FILE          test file under test/ to run (default test_balancer.py)
#   --no-probe           skip the pre-flight health probe (rare)
#   --                   end of own flags; everything after passed to pytest
#
# Requires:
#   bash, curl, python3 (with pytest + requests installed)

set -euo pipefail

# Self-re-exec under bash if dash/sh invoked us (process substitution etc.)
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEST_DIR="$REPO_ROOT/test"

# Cluster defaults (test-s3). Env vars override.
DEFAULT_MASTER_URL="http://vearch.example.com"
DEFAULT_ROUTER_URL="http://vearch-router.example.com"
DEFAULT_PASSWORD="REDACTED_PASSWORD"

MASTER_URL="${MASTER_URL:-$DEFAULT_MASTER_URL}"
ROUTER_URL="${ROUTER_URL:-$DEFAULT_ROUTER_URL}"
PASSWORD="${PASSWORD:-$DEFAULT_PASSWORD}"
USERNAME="root"

TEST_FILE="test_balancer.py"
PROBE=1
PYTEST_EXTRA=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --test)         TEST_FILE="$2"; shift 2 ;;
        --no-probe)     PROBE=0; shift ;;
        --)             shift; PYTEST_EXTRA=("$@"); break ;;
        -h|--help)
            grep -E '^# (Default|Usage|Flags|SECURITY|  )' "$0" | sed 's/^# //; s/^#//'
            exit 0
            ;;
        *)              echo "unknown arg: $1" >&2; exit 1 ;;
    esac
done

if [[ ! -f "$TEST_DIR/$TEST_FILE" ]]; then
    echo "ERROR: $TEST_DIR/$TEST_FILE not found" >&2
    exit 2
fi

echo "============================================================"
echo " vearch remote test runner"
echo "============================================================"
echo "  master:   $MASTER_URL"
echo "  router:   $ROUTER_URL"
echo "  user:     $USERNAME"
echo "  password: ${PASSWORD:0:3}***  (length=${#PASSWORD})"
echo "  test:     $TEST_DIR/$TEST_FILE"
echo

# ---- pre-flight probe -----------------------------------------------------
if (( PROBE == 1 )); then
    echo "==> probing master /cluster/health ..."
    set +e
    http=$(curl -s -o /tmp/_vearch_probe.$$ -w "%{http_code}" \
             --connect-timeout 10 --max-time 20 \
             -u "$USERNAME:$PASSWORD" \
             "$MASTER_URL/cluster/health")
    rc=$?
    set -e
    body=$(cat /tmp/_vearch_probe.$$ 2>/dev/null || echo "")
    rm -f /tmp/_vearch_probe.$$
    if (( rc != 0 )); then
        echo "ERROR: curl failed (rc=$rc) — check VPN / DNS / network reachability" >&2
        echo "  hint: try the following manually:" >&2
        echo "    curl -v $MASTER_URL/cluster/health" >&2
        echo "    getent hosts $(echo "$MASTER_URL" | sed -E 's,https?://([^/:]+).*,\1,')" >&2
        exit 3
    fi
    if [[ "$http" != "200" ]]; then
        echo "ERROR: master returned HTTP $http (expected 200)" >&2
        echo "       body: ${body:0:300}" >&2
        if [[ "$http" == "401" ]]; then
            echo "       hint: PASSWORD likely wrong — check the cluster admin password" >&2
        fi
        exit 4
    fi
    echo "==> master healthy"
    echo "==> probing router /cluster/health ..."
    set +e
    rhttp=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 10 --max-time 20 \
              -u "$USERNAME:$PASSWORD" \
              "$ROUTER_URL/cluster/health?detail=false")
    set -e
    if [[ "$rhttp" != "200" ]]; then
        echo "WARN: router probe returned HTTP $rhttp; tests using router may fail" >&2
    else
        echo "==> router healthy"
    fi
fi

# ---- pytest -------------------------------------------------------------
export MASTER_URL ROUTER_URL PASSWORD

echo
echo "============================================================"
echo " running tests"
echo "============================================================"
cd "$TEST_DIR"
# Default pytest flags; user-supplied extras (after --) append.
# ${arr[@]+"${arr[@]}"} guards against `set -u` complaining on an empty array
# (bash < 4.4 treats empty-array access as "unbound variable").
python3 -m pytest "$TEST_FILE" -v --log-cli-level=INFO ${PYTEST_EXTRA[@]+"${PYTEST_EXTRA[@]}"}
