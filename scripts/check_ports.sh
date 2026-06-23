#!/usr/bin/env bash
# Scan the 32 ports the single-host cluster needs. For any port that's
# already in use, allocate a free replacement (orig + N×10000) and rewrite
# the affected TOML config files in place.
#
# Usage:
#   bash scripts/check_ports.sh             # scan + auto-fix (backups in .bak)
#   bash scripts/check_ports.sh --dry-run   # only report; do not modify files
#   bash scripts/check_ports.sh --restore   # restore from .bak backups
#
# Notes:
#   - Works on Linux. Uses `ss` (preferred) or `/dev/tcp` fallback.
#   - Each port is unique across the 32, so a global `sed s/<old>/<new>/`
#     per file is safe.
#   - Master/router ports live in all 3 TOML files (because every process
#     loads [[masters]] for cluster discovery). PS ports live in 1 file each.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CFG_MAIN="$REPO_ROOT/config/config_single_host.toml"
CFG_PS2="$REPO_ROOT/config/config_single_host_ps2.toml"
CFG_PS3="$REPO_ROOT/config/config_single_host_ps3.toml"

# Map a scope key (main/ps2/ps3) to its TOML file path.
# Use a function rather than `declare -A` to stay compatible with bash 3.x
# (still default on macOS); the target Linux servers usually have bash 4+
# but bash 3 compat costs nothing.
file_for_key() {
    case "$1" in
        main) echo "$CFG_MAIN" ;;
        ps2)  echo "$CFG_PS2" ;;
        ps3)  echo "$CFG_PS3" ;;
        *)    return 1 ;;
    esac
}

# Catalog: "role|port|scope"
# scope is comma-separated list of file keys (main / ps2 / ps3).
# Master + router ports appear in all 3 files; each ps appears in its own.
# All ports are the +10000-shifted defaults from config/config_single_host*.toml.
# Single-master topology — see config_single_host.toml header for why.
CATALOG=(
    "m1.api|18817|main,ps2,ps3"
    "m1.etcd|12378|main,ps2,ps3"
    "m1.etcd_peer|12390|main,ps2,ps3"
    "m1.etcd_client|12370|main,ps2,ps3"
    "m1.pprof|16062|main,ps2,ps3"
    "m1.monitor|18828|main,ps2,ps3"
    "router.http|19001|main,ps2,ps3"
    "router.pprof|16061|main,ps2,ps3"
    "ps1.rpc|18081|main"
    "ps1.raft_heartbeat|18898|main"
    "ps1.raft_replicate|18899|main"
    "ps1.pprof|16060|main"
    "ps2.rpc|18082|ps2"
    "ps2.raft_heartbeat|18900|ps2"
    "ps2.raft_replicate|18901|ps2"
    "ps2.pprof|16063|ps2"
    "ps3.rpc|18083|ps3"
    "ps3.raft_heartbeat|18902|ps3"
    "ps3.raft_replicate|18903|ps3"
    "ps3.pprof|16064|ps3"
)

DRY_RUN=0
RESTORE=0

usage() {
    grep -E '^# Usage' -A 4 "$0" | sed 's/^# //; s/^#//'
    exit 1
}

case "${1:-}" in
    -h|--help) usage ;;
    --dry-run) DRY_RUN=1 ;;
    --restore) RESTORE=1 ;;
    "")        ;;
    *)         echo "unknown option: $1"; usage ;;
esac

# ---- restore mode ----------------------------------------------------------
if [[ $RESTORE -eq 1 ]]; then
    n=0
    for f in "$CFG_MAIN" "$CFG_PS2" "$CFG_PS3"; do
        if [[ -f "$f.bak" ]]; then
            mv "$f.bak" "$f"
            echo "[restored] $f"
            n=$((n+1))
        fi
    done
    [[ $n -eq 0 ]] && echo "no .bak files found; nothing to restore"
    exit 0
fi

# ---- port-in-use predicate ------------------------------------------------
port_in_use() {
    local p="$1"
    # Prefer `ss` (modern, fast). Fallback to /dev/tcp.
    if command -v ss >/dev/null 2>&1; then
        ss -tnlH 2>/dev/null | awk '{print $4}' | grep -qE ":${p}$" && return 0
    else
        (timeout 1 bash -c "echo > /dev/tcp/127.0.0.1/$p" 2>/dev/null) && return 0
    fi
    return 1
}

# Track in-use ports (system) + ports we've already allocated this run
USED_PORTS=$'\n'

mark_in_use() {
    USED_PORTS="${USED_PORTS}$1"$'\n'
}

already_marked() {
    case "$USED_PORTS" in
        *$'\n'"$1"$'\n'*) return 0 ;;
        *) return 1 ;;
    esac
}

find_replacement() {
    # Try orig+10000, orig+20000, ...; cap at port 65000.
    local orig="$1"
    local stride=10000
    local cand
    while (( orig + stride <= 65000 )); do
        cand=$(( orig + stride ))
        if ! port_in_use "$cand" && ! already_marked "$cand"; then
            echo "$cand"
            return 0
        fi
        stride=$((stride + 10000))
    done
    # Last-resort scan from 30000 upward.
    for cand in $(seq 30000 65000); do
        if ! port_in_use "$cand" && ! already_marked "$cand"; then
            echo "$cand"
            return 0
        fi
    done
    return 1
}

# ---- backup --------------------------------------------------------------
make_backups() {
    [[ $DRY_RUN -eq 1 ]] && return 0
    for f in "$CFG_MAIN" "$CFG_PS2" "$CFG_PS3"; do
        if [[ ! -f "$f.bak" ]]; then
            cp -p "$f" "$f.bak"
        fi
    done
}

# ---- main loop -----------------------------------------------------------
echo "============================================================"
echo " single-host cluster port scan"
[[ $DRY_RUN -eq 1 ]] && echo " mode: --dry-run (no files will be modified)"
echo "============================================================"
printf "%-22s %-7s %-10s %s\n" "ROLE" "PORT" "STATE" "ACTION"

backups_made=0
changes_made=0

for entry in "${CATALOG[@]}"; do
    IFS='|' read -r role port scope <<< "$entry"

    if port_in_use "$port"; then
        # Find a replacement.
        new=$(find_replacement "$port") || {
            printf "%-22s %-7s %-10s no free replacement up to 65000\n" \
                "$role" "$port" "OCCUPIED"
            exit 2
        }
        mark_in_use "$new"

        if [[ $DRY_RUN -eq 1 ]]; then
            printf "%-22s %-7s %-10s would rewrite to %s in {%s}\n" \
                "$role" "$port" "OCCUPIED" "$new" "$scope"
        else
            if [[ $backups_made -eq 0 ]]; then
                make_backups
                backups_made=1
            fi
            IFS=',' read -ra files <<< "$scope"
            for fkey in "${files[@]}"; do
                f=$(file_for_key "$fkey")
                # Each port value is unique in the catalog → safe global sed.
                # Use word-boundary-ish matching: port preceded by '= ' and
                # followed by end-of-line / whitespace, to avoid changing
                # e.g. a substring '8817' inside another number.
                sed -i -E "s/(= *)${port}( *)\$/\1${new}\2/" "$f"
            done
            printf "%-22s %-7s %-10s rewrote to %s in {%s}\n" \
                "$role" "$port" "OCCUPIED" "$new" "$scope"
            changes_made=$((changes_made+1))
        fi
    else
        mark_in_use "$port"
        printf "%-22s %-7s %-10s ok\n" "$role" "$port" "free"
    fi
done

echo "============================================================"
if [[ $DRY_RUN -eq 1 ]]; then
    echo " dry-run complete. rerun without --dry-run to apply."
else
    if [[ $changes_made -eq 0 ]]; then
        echo " all 32 ports are free. no config changes needed."
    else
        echo " $changes_made port(s) reallocated. backups saved as *.bak"
        echo " (run '$0 --restore' to undo)"
    fi
fi
echo "============================================================"
