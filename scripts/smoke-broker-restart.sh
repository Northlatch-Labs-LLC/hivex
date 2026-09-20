#!/usr/bin/env bash
# smoke-broker-restart.sh — boot hivex, mutate state via the real HTTP
# API, kill the process, reboot, and verify the mutation survived. This
# is the binary-level canary for Broker state persistence: a serialization
# or path-resolution regression that every Go test still passes would
# fail here because the process actually starts over.
#
# Runs entirely under a disposable sandbox:
#   - HIVEX_RUNTIME_HOME → per-run tempdir (onboarded.json, broker-state.json land here)
#   - HIVEX_BROKER_TOKEN_FILE → tempdir sibling (doesn't collide with
#     any live hivex using /tmp/hivex-broker-token)
#   - Alternate broker+web ports (27890/27891 default; override with
#     PORT=<N> for web port, broker port = PORT-1)
#
# Usage:
#   scripts/smoke-broker-restart.sh [path-to-hivex-binary]
#   PORT=37891 scripts/smoke-broker-restart.sh ./hivex
#
# Exits 0 on pass, non-zero on any boot failure or missing mutation.

set -euo pipefail

BIN="${1:-$PWD/hivex}"
if [ ! -x "$BIN" ]; then
  echo "[smoke] hivex binary not executable at: $BIN" >&2
  echo "[smoke]   build with: go build -o hivex ./cmd/hivex" >&2
  exit 2
fi

web_port="${PORT:-27891}"
broker_port="$((web_port - 1))"

sandbox="$(mktemp -d -t hivex-smoke-XXXXXX)"
export HIVEX_RUNTIME_HOME="$sandbox/runtime"
export HIVEX_BROKER_TOKEN_FILE="$sandbox/broker-token"
mkdir -p "$HIVEX_RUNTIME_HOME/.hivex"

echo "[smoke] sandbox=$sandbox"
echo "[smoke] broker=$broker_port web=$web_port"

# Pre-seed onboarded.json so hivex boots into shell mode rather than the
# wizard. Otherwise the /channels endpoint is gated behind onboarding.
cat > "$HIVEX_RUNTIME_HOME/.hivex/onboarded.json" <<JSON
{"version":1,"completed_at":"2026-01-01T00:00:00Z","company_name":"smoke-test"}
JSON

pid=""
kill_hivex() {
  local p="$1"
  [ -n "$p" ] || return 0
  kill -0 "$p" 2>/dev/null || return 0
  kill -TERM "$p" 2>/dev/null || true
  for _ in $(seq 1 50); do
    kill -0 "$p" 2>/dev/null || return 0
    sleep 0.1
  done
  kill -KILL "$p" 2>/dev/null || true
  wait "$p" 2>/dev/null || true
}

cleanup() {
  kill_hivex "${pid:-}"
  rm -rf "$sandbox"
}
trap cleanup EXIT

start_hivex() {
  local label="$1"
  echo "[smoke] starting hivex ($label)"
  "$BIN" --no-open --broker-port "$broker_port" --web-port "$web_port" --no-hive \
    </dev/null > "$sandbox/hivex-$label.log" 2>&1 &
  pid=$!
  for _ in $(seq 1 30); do
    if curl -sf "http://127.0.0.1:$web_port/onboarding/state" -o /dev/null; then
      echo "[smoke] hivex ready ($label, pid=$pid)"
      return 0
    fi
    sleep 1
  done
  echo "[smoke] hivex failed to become ready ($label)" >&2
  cat "$sandbox/hivex-$label.log" >&2
  exit 1
}

stop_hivex() {
  kill_hivex "${pid:-}"
  pid=""
  # Wait for the port to free up so the reboot can rebind. /dev/tcp is a
  # bash-only virtual device — if you ever switch the shebang to /bin/sh
  # this loop no-ops silently.
  for _ in $(seq 1 10); do
    if ! (exec 3<>/dev/tcp/127.0.0.1/"$web_port") 2>/dev/null; then break; fi
    sleep 1
  done
}

# ── Phase 1: boot, mutate, stop ─────────────────────────────────────────
start_hivex first
token="$(cat "$HIVEX_BROKER_TOKEN_FILE")"
if [ -z "$token" ]; then
  echo "[smoke] empty broker token" >&2
  exit 1
fi

echo "[smoke] POST /channels create smoke-channel"
status=$(curl -sS -o "$sandbox/post-resp.json" -w '%{http_code}' \
  -X POST "http://127.0.0.1:$broker_port/channels" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"action":"create","slug":"smoke-channel","name":"Smoke","description":"canary","created_by":"cos"}')
if [ "$status" != "200" ]; then
  echo "[smoke] POST /channels failed: status=$status body=$(cat "$sandbox/post-resp.json")" >&2
  exit 1
fi

state_file="$HIVEX_RUNTIME_HOME/.hivex/team/broker-state.json"
if [ ! -f "$state_file" ]; then
  echo "[smoke] state file missing after mutation: $state_file" >&2
  exit 1
fi
if ! grep -q '"smoke-channel"' "$state_file"; then
  echo "[smoke] state file does not contain smoke-channel:" >&2
  head -c 2000 "$state_file" >&2
  exit 1
fi

stop_hivex

# ── Phase 2: reboot, verify survival ────────────────────────────────────
start_hivex second
token="$(cat "$HIVEX_BROKER_TOKEN_FILE")"
resp=$(curl -sSf "http://127.0.0.1:$broker_port/channels" \
  -H "Authorization: Bearer $token")
if ! printf '%s' "$resp" | grep -q '"smoke-channel"'; then
  echo "[smoke] mutation lost across restart; GET /channels body:" >&2
  printf '%s\n' "$resp" | head -c 2000 >&2
  exit 1
fi

echo "[smoke] PASS — smoke-channel survived restart"
