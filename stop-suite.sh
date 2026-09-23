#!/usr/bin/env bash
# Stops everything started by start-suite.sh (office + cognee + headroom).
cd "$(dirname "$0")"
for f in logs/gateway.pid logs/harness.pid; do
  if [ -f "$f" ]; then
    kill "$(cat "$f")" 2>/dev/null && echo "Stopped: $(cat "$f")"
    rm -f "$f"
  fi
done
pkill -f "hivebot --broker-port 7910" 2>/dev/null
(cd 9router && docker compose stop cognee headroom hiveapi-gateway 2>/dev/null)
echo "The suite is stopped."
