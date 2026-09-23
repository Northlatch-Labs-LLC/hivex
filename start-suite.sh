#!/usr/bin/env bash
# Starts the full Hivex suite. Wait for the two links, then open them in your browser.
set -u
cd "$(dirname "$0")"
mkdir -p logs

echo "[1/2] Starting the customer website + portal..."
(cd 9router && nohup npm run dev > ../logs/gateway.log 2>&1 & echo $! > ../logs/gateway.pid)

echo "[2/2] Starting the Hivex Harness office..."
(nohup ./hivebot --broker-port 7910 --web-port 7911 > logs/harness.log 2>&1 & echo $! > logs/harness.pid)

sleep 6
echo ""
echo "The suite is running. Open these links in your browser:"
echo "  Customer site + portal : http://localhost:20127/landing"
echo "  Harness office         : http://localhost:7911"
echo ""
echo "Keep this window open while you work. To stop the suite, run: ./stop-suite.sh"
echo "If something looks wrong, the 'logs' folder inside the project explains why."
