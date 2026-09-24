#!/usr/bin/env bash
# Starts the Hivex suite: the harness office + the cognee and headroom containers.
set -u
cd "$(dirname "$0")"
mkdir -p logs

echo "[1/2] Starting containers (cognee + headroom)..."
(cd 9router && docker compose start cognee headroom > /dev/null 2>&1 && docker compose stop hiveapi-gateway > /dev/null 2>&1)

echo "[2/2] Starting the Hivex office..."
if [ ! -x ./hivex ]; then
  echo "Building the office binary (cmd/hivex)..."
  go build -o hivex ./cmd/hivex || exit 1
fi
# Rotate a bloated engine log before appending (keep 3 generations).
if [ -f logs/harness.log ] && [ "$(stat -f%z logs/harness.log 2>/dev/null || echo 0)" -gt 5242880 ]; then
  mv logs/harness.log.2 logs/harness.log.3 2>/dev/null
  mv logs/harness.log.1 logs/harness.log.2 2>/dev/null
  mv logs/harness.log logs/harness.log.1
fi
(nohup ./hivex --broker-port 7910 --web-port 7911 > logs/harness.log 2>&1 & echo $! > logs/harness.pid)

sleep 6
echo ""
echo "The suite is running. Open this in your browser:"
echo "  Hivex office : http://localhost:7911"
echo ""
echo "Keep this window open while you work. To stop, run: ./stop-suite.sh"
echo "If something looks wrong, the 'logs' folder inside the project explains why."
