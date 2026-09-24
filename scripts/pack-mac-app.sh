#!/usr/bin/env bash
# Packs the Hivex office into an (unsigned, ad-hoc signed) macOS app.
# Usage: scripts/pack-mac-app.sh   → dist/Hive.app
set -euo pipefail
cd "$(dirname "$0")/.."

APP="dist/Hive.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

# Single self-contained binary: web/dist is embedded at compile time.
go build -o "$APP/Contents/MacOS/Hive" ./cmd/hivex

# Launcher: keeps the office in the foreground (Dock shows it running),
# opens the office once the web server is up.
cat > "$APP/Contents/MacOS/Hivex" <<'LAUNCH'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
( sleep 6; open "http://127.0.0.1:7891" ) &
exec "$DIR/Hive" --broker-port 7890 --web-port 7891
LAUNCH
chmod +x "$APP/Contents/MacOS/Hivex"

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>              <string>Hivex</string>
  <key>CFBundleDisplayName</key>       <string>Hivex</string>
  <key>CFBundleIdentifier</key>        <string>llc.northlatch.hivex</string>
  <key>CFBundleExecutable</key>        <string>Hivex</string>
  <key>CFBundlePackageType</key>       <string>APPL</string>
  <key>CFBundleShortVersionString</key> <string>1.0.0</string>
  <key>LSMinimumSystemVersion</key>    <string>13.0</string>
  <key>NSHighResolutionCapable</key>   <true/>
</dict>
</plist>
PLIST

# Ad-hoc signature: no Apple Developer account required; lets the app run
# on this machine. Nested binaries must be signed before the bundle.
codesign --force --sign - "$APP/Contents/MacOS/Hive" >/dev/null 2>&1 || true
codesign --force --sign - "$APP" >/dev/null 2>&1 || true
codesign --verify --deep "$APP" && echo "OK $APP (ad-hoc signed)"
