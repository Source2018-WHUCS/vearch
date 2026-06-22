#!/usr/bin/env bash
# One-shot setup: check ports -> auto-rewrite config -> start cluster -> wait
# for health -> print exported env vars so the test suite picks up the right
# master/router URLs (in case ports were reallocated).
#
# Usage:
#   bash scripts/setup_and_start.sh                       # check + start
#   bash scripts/setup_and_start.sh --with-tests          # also run pytest at the end
#   bash scripts/setup_and_start.sh --with-tests --auto-stop
#                                                         # run pytest, then stop cluster regardless
#   bash scripts/setup_and_start.sh --with-tests --pass-threshold 90
#                                                         # exit non-zero if pytest pass rate < 90%
#   bash scripts/setup_and_start.sh --clean               # wipe $RUN_ROOT before start (data + logs)
#   bash scripts/setup_and_start.sh --dry-run             # only port scan, no changes / start
#   bash scripts/setup_and_start.sh --stop                # stop cluster + restore config
#   bash scripts/setup_and_start.sh --stop --clean        # stop and wipe data dirs
#
# Flags:
#   --with-tests          run test/test_balancer.py after the cluster is healthy
#   --auto-stop           after --with-tests finishes, stop the cluster (default: leave running)
#   --pass-threshold N    when --with-tests is set, require pass rate >= N% (default: 100)
#                         pass rate = passed / (passed + failed + errors); skipped excluded
#   --clean               remove $RUN_ROOT (default /tmp/vearch-single) before starting,
#                         so etcd data dirs don't carry stale member URLs from a previous
#                         run that used different ports. ALWAYS use this after editing
#                         config ports or after check_ports.sh reallocates anything.
#   --dry-run             only scan ports; do not modify or start anything
#   --stop                stop the cluster and restore configs from .bak
#
# Requires:
#   - $VEARCH_BIN (default ./build/bin/vearch) compiled
#   - bash, sed, awk, curl, jq

# Self-re-exec under bash if invoked with `sh script.sh` (dash on Debian/Ubuntu
# doesn't recognise process substitution `< <(...)` and would die at line ~195).
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CFG_MAIN="$REPO_ROOT/config/config_single_host.toml"
CHECK_PORTS="$REPO_ROOT/scripts/check_ports.sh"
START_CLUSTER="$REPO_ROOT/scripts/start_single_host_cluster.sh"
TEST_FILE="$REPO_ROOT/test/test_balancer.py"
RUN_ROOT="${VEARCH_RUN_ROOT:-/tmp/vearch-single}"

WITH_TESTS=0
DRY_RUN=0
STOP_ONLY=0
AUTO_STOP=0
CLEAN=0
PASS_THRESHOLD=100

usage() {
    grep -E '^# (Usage|Flags|  )' "$0" | sed 's/^# //; s/^#//'
    exit 1
}

# argparse: support both `--flag value` and `--flag=value`
while [[ $# -gt 0 ]]; do
    case "$1" in
        --with-tests)         WITH_TESTS=1; shift ;;
        --auto-stop)          AUTO_STOP=1; shift ;;
        --clean)              CLEAN=1; shift ;;
        --dry-run)            DRY_RUN=1; shift ;;
        --stop)               STOP_ONLY=1; shift ;;
        --pass-threshold)     PASS_THRESHOLD="$2"; shift 2 ;;
        --pass-threshold=*)   PASS_THRESHOLD="${1#*=}"; shift ;;
        -h|--help)            usage ;;
        "")                   shift ;;
        *)                    echo "unknown arg: $1"; usage ;;
    esac
done

if ! [[ "$PASS_THRESHOLD" =~ ^[0-9]+$ ]] || (( PASS_THRESHOLD < 0 || PASS_THRESHOLD > 100 )); then
    echo "ERROR: --pass-threshold must be an integer in [0,100]; got '$PASS_THRESHOLD'" >&2
    exit 1
fi

# ---- helpers --------------------------------------------------------------

# Print "m1 m2 m3" api_port values (in order) from the [[masters]] blocks
# of the main config file.
read_master_api_ports() {
    awk '
        /^\[\[masters\]\]/ { in_block=1; next }
        /^\[/ && !/^\[\[masters\]\]/ { in_block=0 }
        in_block && /api_port[[:space:]]*=/ {
            gsub(/[^0-9]/, "", $NF); print $NF
        }
    ' "$CFG_MAIN"
}

# Read [router].port
read_router_port() {
    awk '
        /^\[router\]/ { in_block=1; next }
        /^\[/ && !/^\[router\]/ { in_block=0 }
        in_block && /^[[:space:]]*port[[:space:]]*=/ {
            gsub(/[^0-9]/, "", $NF); print $NF; exit
        }
    ' "$CFG_MAIN"
}

# Dump diagnostic info for one role: pid liveness, nohup stdout/stderr capture,
# and — most importantly — the per-role application log dir (vearch's
# [global].log = "./logs" resolves to <run_dir>/logs/).
dump_role_log() {
    local role="$1"
    local run_dir="$RUN_ROOT/$role"
    echo "----- $role  (run_dir=$run_dir) -----"

    # pid liveness
    local pid_file="$run_dir/vearch.pid"
    if [[ -f "$pid_file" ]]; then
        local pid; pid=$(cat "$pid_file" 2>/dev/null || echo "?")
        if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
            echo "  pid $pid: ALIVE"
        else
            echo "  pid $pid: DEAD (process exited)"
        fi
    else
        echo "  (no pid file — start_single_host_cluster.sh never recorded one)"
    fi

    # nohup-captured stdout/stderr (often only has Go init lines)
    local nohup_log="$run_dir/vearch.log"
    if [[ -f "$nohup_log" ]]; then
        echo "  --- $nohup_log (last 30 lines) ---"
        tail -n 30 "$nohup_log" 2>/dev/null | sed 's/^/    /'
    else
        echo "  (no nohup log at $nohup_log)"
    fi

    # vearch's real application log dir — [global].log = "./logs"
    if [[ -d "$run_dir/logs" ]]; then
        # Dump every .log file (FATAL/ERROR/WARN/INFO/ETCD/...). Empty ones
        # are informative too — they tell us the process never reached that
        # log level. List by mtime desc so FATAL (if it exists) comes first.
        local logs
        logs=$(find "$run_dir/logs" -maxdepth 3 -type f -name '*.log' 2>/dev/null \
               | xargs -r ls -t 2>/dev/null)
        if [[ -n "$logs" ]]; then
            while IFS= read -r f; do
                [[ -z "$f" ]] && continue
                local size; size=$(stat -c%s "$f" 2>/dev/null || stat -f%z "$f" 2>/dev/null || echo "?")
                echo "  --- $f (size=$size bytes, last 40 lines) ---"
                if [[ "$size" = "0" ]]; then
                    echo "    (empty)"
                else
                    tail -n 40 "$f" 2>/dev/null | sed 's/^/    /'
                fi
            done <<< "$logs"
        else
            echo "  (no *.log files under $run_dir/logs yet)"
        fi
    else
        echo "  (no $run_dir/logs/ directory — process may have died before log init)"
    fi
    echo
}

# Dump logs for a space-separated list of roles + a system-wide vearch ps.
dump_logs() {
    echo
    echo "============================================================"
    echo " diagnostic snapshot"
    echo "============================================================"
    echo "--- system processes (ps -ef | grep vearch) ---"
    ps -ef | grep -E '(^|[^a-zA-Z])vearch( |$)' | grep -v grep | sed 's/^/  /' || echo "  (no vearch processes found)"
    echo
    for r in "$@"; do
        dump_role_log "$r"
    done
}

# Wait until master m1's HTTP API responds healthy.
wait_for_master() {
    local port="$1"
    local deadline=$(( $(date +%s) + 120 ))
    while (( $(date +%s) < deadline )); do
        if curl -fs -u root:secret "http://127.0.0.1:$port/cluster/health" >/dev/null 2>&1; then
            return 0
        fi
        sleep 3
    done
    return 1
}

# Wait until at least 3 PS nodes have registered with the master.
wait_for_ps_registration() {
    local port="$1"
    local deadline=$(( $(date +%s) + 60 ))
    while (( $(date +%s) < deadline )); do
        # Response shape may be .data.servers or .data — accept either.
        local n
        n=$(curl -fs -u root:secret "http://127.0.0.1:$port/servers" 2>/dev/null \
            | jq -r '(.data.servers // .data // []) | length' 2>/dev/null || echo 0)
        if [[ "$n" -ge 3 ]]; then
            echo "$n"
            return 0
        fi
        sleep 3
    done
    return 1
}

# Parse pytest output for "N passed, M failed, K skipped, L error" counts.
# Echoes "passed failed errors skipped" space-separated; defaults to 0.
parse_pytest_summary() {
    local log="$1"
    # Pick the last summary line. pytest prints e.g.:
    #   ======== 48 passed, 2 failed, 5 skipped in 12.34s ========
    local line
    line=$(grep -E '^=+ .*(passed|failed|error|skipped).* in [0-9]' "$log" | tail -1 || true)
    if [[ -z "$line" ]]; then
        echo "0 0 0 0"
        return
    fi
    local p f e s
    p=$(echo "$line" | grep -oE '[0-9]+ passed' | grep -oE '^[0-9]+' || echo 0)
    f=$(echo "$line" | grep -oE '[0-9]+ failed' | grep -oE '^[0-9]+' || echo 0)
    e=$(echo "$line" | grep -oE '[0-9]+ error' | grep -oE '^[0-9]+' || echo 0)
    s=$(echo "$line" | grep -oE '[0-9]+ skipped' | grep -oE '^[0-9]+' || echo 0)
    echo "${p:-0} ${f:-0} ${e:-0} ${s:-0}"
}

# ---- stop mode ------------------------------------------------------------

if [[ $STOP_ONLY -eq 1 ]]; then
    echo "==> stopping cluster"
    bash "$START_CLUSTER" stop || true
    echo "==> restoring config from .bak"
    bash "$CHECK_PORTS" --restore || true
    if [[ $CLEAN -eq 1 ]]; then
        echo "==> --clean: wiping $RUN_ROOT"
        rm -rf "$RUN_ROOT"
    fi
    echo "done"
    exit 0
fi

# ---- step 0: optional clean of run root ----------------------------------

if [[ $CLEAN -eq 1 ]] && [[ $DRY_RUN -eq 0 ]]; then
    echo "==> --clean: wiping $RUN_ROOT (etcd data dirs, logs, pid files)"
    # Be slightly defensive — only nuke the exact dir we manage.
    case "$RUN_ROOT" in
        /tmp/vearch-single|/tmp/vearch-single/|/var/lib/vearch-single|/var/lib/vearch-single/)
            rm -rf "$RUN_ROOT"
            ;;
        *)
            # Custom RUN_ROOT — only remove sub-dirs we know we created
            for sub in m1 m2 m3 router ps1 ps2 ps3; do
                rm -rf "$RUN_ROOT/$sub"
            done
            ;;
    esac
fi

# ---- step 1: check ports + (auto-rewrite or dry-run) ---------------------

echo "============================================================"
echo " step 1/4  port scan"
echo "============================================================"
if [[ $DRY_RUN -eq 1 ]]; then
    bash "$CHECK_PORTS" --dry-run
    echo
    echo "dry-run done. nothing was modified or started."
    exit 0
fi
bash "$CHECK_PORTS"

# ---- step 2: figure out the actual ports the cluster will use ------------

# Read master api_ports as a newline-split array. Avoid `mapfile + < <(...)`
# so the script doesn't depend on process substitution (failing under `sh`).
_old_IFS=$IFS
IFS=$'\n'
MASTER_PORTS=( $(read_master_api_ports) )
IFS=$_old_IFS
ROUTER_PORT="$(read_router_port)"

if [[ ${#MASTER_PORTS[@]} -lt 3 ]] || [[ -z "${ROUTER_PORT:-}" ]]; then
    echo "ERROR: could not parse master/router ports from $CFG_MAIN" >&2
    exit 2
fi

M1_API="${MASTER_PORTS[0]}"
M2_API="${MASTER_PORTS[1]}"
M3_API="${MASTER_PORTS[2]}"

echo
echo "==> resolved ports after check_ports.sh:"
printf "   m1.api      %s\n   m2.api      %s\n   m3.api      %s\n   router.http %s\n" \
    "$M1_API" "$M2_API" "$M3_API" "$ROUTER_PORT"

# ---- step 3: start cluster + wait for health -----------------------------

echo
echo "============================================================"
echo " step 2/4  starting 7 processes"
echo "============================================================"
bash "$START_CLUSTER" start

echo
echo "============================================================"
echo " step 3/4  waiting for master quorum (max 120s)"
echo "============================================================"
if ! wait_for_master "$M1_API"; then
    echo "ERROR: master m1 (port $M1_API) did not become healthy within 120s" >&2
    dump_logs m1 m2 m3
    echo "cluster left running; inspect the logs above, then run:" >&2
    echo "  bash $0 --stop" >&2
    exit 3
fi
echo "master quorum ready on port $M1_API"

echo
echo "==> waiting for PS registration (max 60s)"
if ps_count=$(wait_for_ps_registration "$M1_API"); then
    echo "$ps_count PS nodes registered"
else
    echo "WARN: <3 PS nodes registered within 60s; tests requiring >=3 PS will skip" >&2
    dump_logs ps1 ps2 ps3
fi

# ---- step 4: print summary + (optional) run tests ------------------------

echo
echo "============================================================"
echo " step 4/4  cluster ready"
echo "============================================================"
bash "$START_CLUSTER" status

cat <<EOF

To run the balancer test suite against this cluster:

    export MASTER_URL=http://127.0.0.1:$M1_API
    export ROUTER_URL=http://127.0.0.1:$ROUTER_PORT
    cd $REPO_ROOT/test
    python3 -m pytest test_balancer.py -v --log-cli-level=INFO

To stop:
    bash $0 --stop

To investigate per-process logs:
    tail -f $RUN_ROOT/<role>/vearch.log

EOF

# ---- optional: run tests inline ------------------------------------------

if [[ $WITH_TESTS -eq 1 ]]; then
    if [[ ! -f "$TEST_FILE" ]]; then
        echo "ERROR: $TEST_FILE not found; cannot run tests" >&2
        exit 4
    fi
    if ! command -v python3 >/dev/null 2>&1; then
        echo "ERROR: python3 not on PATH" >&2
        exit 5
    fi
    echo "============================================================"
    echo " running balancer tests"
    echo "============================================================"
    export MASTER_URL="http://127.0.0.1:$M1_API"
    export ROUTER_URL="http://127.0.0.1:$ROUTER_PORT"

    PYTEST_LOG="$(mktemp -t vearch-pytest.XXXXXX)"
    # Don't let pytest's non-zero exit kill us before we parse the summary.
    set +e
    ( cd "$REPO_ROOT/test" && python3 -m pytest test_balancer.py -v --log-cli-level=INFO ) \
        2>&1 | tee "$PYTEST_LOG"
    PYTEST_RC=${PIPESTATUS[0]}
    set -e

    # Parse via a temp string; avoid `read < <(...)` so the line parses cleanly
    # under any shell (the self-re-exec at top should already guarantee bash,
    # but defence in depth).
    _summary="$(parse_pytest_summary "$PYTEST_LOG")"
    PASSED="${_summary%% *}"; _rest="${_summary#* }"
    FAILED="${_rest%% *}";    _rest="${_rest#* }"
    ERRORS="${_rest%% *}";    SKIPPED="${_rest#* }"
    : "${PASSED:=0}" "${FAILED:=0}" "${ERRORS:=0}" "${SKIPPED:=0}"
    TOTAL=$(( PASSED + FAILED + ERRORS ))

    echo
    echo "============================================================"
    echo " test result summary"
    echo "============================================================"
    printf "  passed:    %d\n" "$PASSED"
    printf "  failed:    %d\n" "$FAILED"
    printf "  errors:    %d\n" "$ERRORS"
    printf "  skipped:   %d\n" "$SKIPPED"
    printf "  pytest rc: %d\n" "$PYTEST_RC"

    if (( TOTAL == 0 )); then
        PASS_RATE=0
        echo "  pass rate: n/a (no passed/failed/error tests recorded)"
    else
        # Integer percent. Round half-up via (2x+denom)/(2*denom) trick.
        PASS_RATE=$(( (PASSED * 200 + TOTAL) / (2 * TOTAL) ))
        printf "  pass rate: %d%% (threshold: %d%%)\n" "$PASS_RATE" "$PASS_THRESHOLD"
    fi
    echo

    THRESHOLD_OK=1
    if (( TOTAL == 0 )); then
        # If pytest collected nothing and exited non-zero, treat as failure.
        if (( PYTEST_RC != 0 )); then THRESHOLD_OK=0; fi
    else
        if (( PASS_RATE < PASS_THRESHOLD )); then THRESHOLD_OK=0; fi
    fi

    rm -f "$PYTEST_LOG"

    if (( AUTO_STOP == 1 )); then
        echo "==> --auto-stop set; stopping cluster"
        bash "$0" --stop || true
    else
        if (( THRESHOLD_OK == 0 )); then
            echo "Cluster left running so you can investigate the failures." >&2
            echo "Logs:  $RUN_ROOT/<role>/vearch.log" >&2
            echo "Stop:  bash $0 --stop" >&2
        fi
    fi

    if (( THRESHOLD_OK == 0 )); then
        exit 6
    fi
fi
