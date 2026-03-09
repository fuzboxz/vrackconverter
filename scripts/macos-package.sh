#!/bin/bash
# Package macOS app bundle for vRackConverter

set -e

BINARY_NAME="${1:-vrackconverter}"
VERSION="${2:-dev}"
APP="${BINARY_NAME}.app"
CONTENTS="$APP/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
ICONSET="$RESOURCES/AppIcon.iconset"

echo "Packaging $APP..."

# Remove old app
rm -rf "$APP"
mkdir -p "$MACOS" "$RESOURCES" "$ICONSET"

# Copy binary
cp "$BINARY_NAME" "$MACOS/$BINARY_NAME"

# Generate icon sizes
sips -z 16 16 logo.png --out "$ICONSET/icon_16x16.png" >/dev/null 2>&1
sips -z 32 32 logo.png --out "$ICONSET/icon_16x16@2x.png" >/dev/null 2>&1
sips -z 32 32 logo.png --out "$ICONSET/icon_32x32.png" >/dev/null 2>&1
sips -z 64 64 logo.png --out "$ICONSET/icon_32x32@2x.png" >/dev/null 2>&1
sips -z 128 128 logo.png --out "$ICONSET/icon_128x128.png" >/dev/null 2>&1
sips -z 256 256 logo.png --out "$ICONSET/icon_128x128@2x.png" >/dev/null 2>&1
sips -z 256 256 logo.png --out "$ICONSET/icon_256x256.png" >/dev/null 2>&1
sips -z 512 512 logo.png --out "$ICONSET/icon_256x256@2x.png" >/dev/null 2>&1
sips -z 512 512 logo.png --out "$ICONSET/icon_512x512.png" >/dev/null 2>&1
sips -z 1024 1024 logo.png --out "$ICONSET/icon_512x512@2x.png" >/dev/null 2>&1

# Create icns from iconset
iconutil -c icns "$ICONSET"
rm -rf "$ICONSET"

# Create Info.plist
cat > "$CONTENTS/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${BINARY_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>com.vrackconverter.app</string>
    <key>CFBundleName</key>
    <string>vRackConverter</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

echo "Package complete: $APP"
