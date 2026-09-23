#!/usr/bin/env bash
# Stops everything started by start-suite.sh.
cd "$(dirname "$0")"
for f in logs/gateway.pid logs/harness.pid; do
  if [ -f "$f" ]; then
    kill "$(cat "$f")" 2>/dev/null && echo "Stopped: $(cat "$f")"
    rm -f "$f"
  fi
done
pkill -f "next dev" 2>/dev/null
pkill -f "hivebot --broker-port 7910" 2>/dev/null
echo "The suite is stopped."
