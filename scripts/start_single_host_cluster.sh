#!/usr/bin/env bash
# Bring up a 3-master + 1-router + 3-PS Vearch cluster on a single host.
# Each process runs in its own working dir under VEARCH_RUN_ROOT so that
# the relative [global].data path in each TOML resolves to isolated disk.
#
# Usage:
#   bash scripts/start_single_host_cluster.sh           # start
#   bash scripts/start_single_host_cluster.sh stop      # stop all
#   bash scripts/start_single_host_cluster.sh status    # check ports/PIDs
#
# Requires:
#   - VEARCH_BIN env var pointing to compiled vearch binary
#     (default: ./build/bin/vearch — assumes you ran `cd build && bash build.sh`)
#   - LD_LIBRARY_PATH set so gamma shared libs are visible (the script sets
#     it from ./build/gamma_build if VEARCH_GAMMA_LIB is unset)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VEARCH_BIN="${VEARCH_BIN:-$REPO_ROOT/build/bin/vearch}"
VEARCH_GAMMA_LIB="${VEARCH_GAMMA_LIB:-$REPO_ROOT/build/gamma_build}"
RUN_ROOT="${VEARCH_RUN_ROOT:-/tmp/vearch-single}"

CFG_MAIN="$REPO_ROOT/config/config_single_host.toml"
CFG_PS2="$REPO_ROOT/config/config_single_host_ps2.toml"
CFG_PS3="$REPO_ROOT/config/config_single_host_ps3.toml"

LD_LIBRARY_PATH="$VEARCH_GAMMA_LIB:${LD_LIBRARY_PATH:-}"
export LD_LIBRARY_PATH

# Read a single port value from the TOML for display purposes only. The
# actual port used by each process comes from -conf, not this map.
# `section` is the bare section name like "router" or "ps"; we build the
# regex internally to avoid shell/awk backslash-escaping confusion.
toml_port() {
    local file="$1" section="$2" key="$3"
    awk -v sec="$section" -v key="$key" '
        BEGIN { pat = "^[[]" sec "[]]" }
        $0 ~ pat       { in_block=1; next }
        /^\[/          { in_block=0 }
        in_block && $0 ~ ("^[[:space:]]*"key"[[:space:]]*=") {
            gsub(/[^0-9]/, "", $NF); print $NF; exit
        }
    ' "$file"
}

resolve_role_port() {
    local role="$1"
    case "$role" in
        router)   toml_port "$CFG_MAIN" "router" "port" ;;
        ps1)      toml_port "$CFG_MAIN" "ps"     "rpc_port" ;;
        ps2)      toml_port "$CFG_PS2"  "ps"     "rpc_port" ;;
        ps3)      toml_port "$CFG_PS3"  "ps"     "rpc_port" ;;
    esac
}

# Master api_port lives in [[masters]] blocks; read by name.
master_api_port() {
    local name="$1"
    awk -v name="$name" '
        /^\[\[masters\]\]/      { in_block=1; this=""; next }
        /^\[/ && !/^\[\[masters/ { in_block=0 }
        in_block && /name[[:space:]]*=/ {
            v=$NF; gsub(/[ \t"]/, "", v); this=v
        }
        in_block && this==name && /api_port[[:space:]]*=/ {
            gsub(/[^0-9]/, "", $NF); print $NF; exit
        }
    ' "$CFG_MAIN"
}

# (role, config_file, extra_flags)
# Single-master topology: see config_single_host.toml header for the bug
# in vearch config.go:316 that forces us to use only 1 master.
declare -a INSTANCES=(
    "m1     $CFG_MAIN  -master=m1   master"
    "router $CFG_MAIN               router"
    "ps1    $CFG_MAIN               ps"
    "ps2    $CFG_PS2                ps"
    "ps3    $CFG_PS3                ps"
)

usage() {
    echo "Usage: $0 [start|stop|status]"
    exit 1
}

ensure_binary() {
    if [[ ! -x "$VEARCH_BIN" ]]; then
        echo "ERROR: vearch binary not found / not executable: $VEARCH_BIN" >&2
        echo "       set VEARCH_BIN env var or run 'cd build && bash build.sh' first." >&2
        exit 1
    fi
}

start_all() {
    ensure_binary
    mkdir -p "$RUN_ROOT"

    for line in "${INSTANCES[@]}"; do
        # shellcheck disable=SC2206
        parts=($line)
        name="${parts[0]}"
        conf="${parts[1]}"
        rest_flags=("${parts[@]:2}")

        run_dir="$RUN_ROOT/$name"
        mkdir -p "$run_dir"
        cd "$run_dir"

        pid_file="$run_dir/vearch.pid"
        if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
            echo "[skip] $name already running (pid=$(cat "$pid_file"))"
            continue
        fi

        log_file="$run_dir/vearch.log"
        local port
        case "$name" in
            m1)       port=$(master_api_port "$name") ;;
            *)        port=$(resolve_role_port "$name") ;;
        esac
        echo "[start] $name -> $run_dir (port ${port:-?})"
        nohup "$VEARCH_BIN" -conf "$conf" "${rest_flags[@]}" \
            > "$log_file" 2>&1 &
        echo $! > "$pid_file"

        # master needs to be up before ps/router connect — pace the start
        case "$name" in
            m1) sleep 3 ;;
            router|ps1|ps2|ps3) sleep 1 ;;
        esac
    done

    echo
    echo "Cluster boot kicked off. Logs under: $RUN_ROOT/<name>/{vearch.log,logs/}"
    local m1_port; m1_port=$(master_api_port m1)
    echo "Wait ~30-60s for master quorum + PS registration, then verify:"
    echo "  curl -s -u root:secret http://127.0.0.1:${m1_port:-8817}/cluster/health | jq"
    echo "  curl -s -u root:secret http://127.0.0.1:${m1_port:-8817}/servers       | jq '.data.servers // .data | length'"
}

stop_all() {
    for line in "${INSTANCES[@]}"; do
        # shellcheck disable=SC2206
        parts=($line)
        name="${parts[0]}"
        pid_file="$RUN_ROOT/$name/vearch.pid"
        if [[ -f "$pid_file" ]]; then
            pid="$(cat "$pid_file")"
            if kill -0 "$pid" 2>/dev/null; then
                echo "[stop ] $name (pid=$pid)"
                kill "$pid" 2>/dev/null || true
            fi
            rm -f "$pid_file"
        fi
    done
    echo "all stopped"
}

status() {
    printf "%-8s %-8s %-7s %s\n" "ROLE" "PID" "PORT" "STATE"
    for line in "${INSTANCES[@]}"; do
        # shellcheck disable=SC2206
        parts=($line)
        name="${parts[0]}"
        pid_file="$RUN_ROOT/$name/vearch.pid"
        local port
        case "$name" in
            m1)       port=$(master_api_port "$name") ;;
            *)        port=$(resolve_role_port "$name") ;;
        esac
        port="${port:-?}"
        if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
            printf "%-8s %-8s %-7s %s\n" "$name" "$(cat "$pid_file")" "$port" "running"
        else
            printf "%-8s %-8s %-7s %s\n" "$name" "-" "$port" "stopped"
        fi
    done
}

case "${1:-start}" in
    start)  start_all ;;
    stop)   stop_all ;;
    status) status ;;
    *)      usage ;;
esac
