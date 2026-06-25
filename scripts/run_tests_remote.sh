#!/usr/bin/env bash
# Run test/test_balancer.py against a remote, already-deployed Vearch cluster.
#
# Required env vars:
#   MASTER_URL   e.g. http://vearch-master:8817
#   ROUTER_URL   e.g. http://vearch-router:9001
#   PASSWORD     cluster admin password (matches [global].signkey in config.toml)
#   USERNAME     optional, defaults to "root"
#
# Usage:
#   MASTER_URL=... ROUTER_URL=... PASSWORD=... \
#       bash scripts/run_tests_remote.sh                       # full suite
#   MASTER_URL=... ROUTER_URL=... PASSWORD=... \
#       bash scripts/run_tests_remote.sh --test test_balancer_e2e_data.py
#   MASTER_URL=... ROUTER_URL=... PASSWORD=... \
#       bash scripts/run_tests_remote.sh -- -k TestBalancerConfig
#                                                              # extra pytest args (after `--`)
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

# Required env vars: MASTER_URL, ROUTER_URL, PASSWORD.
# We deliberately do NOT carry default credentials in this script — they
# would otherwise persist in git history of any public fork.

if [[ -z "${MASTER_URL:-}" ]] || [[ -z "${ROUTER_URL:-}" ]] || [[ -z "${PASSWORD:-}" ]]; then
    cat >&2 <<'USAGE'
ERROR: MASTER_URL, ROUTER_URL, and PASSWORD env vars are all required.

Example:
    MASTER_URL=http://your-master:8817 \
    ROUTER_URL=http://your-router:9001 \
    PASSWORD=your-cluster-password \
        bash scripts/run_tests_remote.sh
USAGE
    exit 2
fi
USERNAME="${USERNAME:-root}"

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
