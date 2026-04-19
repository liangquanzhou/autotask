#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LOGO_DIR="$ROOT/assets/logo"
ICONSET="$LOGO_DIR/AppIcon.iconset"
UI_RESOURCES="$ROOT/ui/Resources"

mkdir -p "$ICONSET" "$UI_RESOURCES"

magick "$LOGO_DIR/autotask-logo-clock-check.png" -resize 1024x1024 "$LOGO_DIR/autotask-icon-1024.png"

while read -r size scale name; do
  px=$((size * scale))
  magick "$LOGO_DIR/autotask-logo-clock-check.png" -resize "${px}x${px}" "$ICONSET/$name"
done <<'EOF'
16 1 icon_16x16.png
16 2 icon_16x16@2x.png
32 1 icon_32x32.png
32 2 icon_32x32@2x.png
128 1 icon_128x128.png
128 2 icon_128x128@2x.png
256 1 icon_256x256.png
256 2 icon_256x256@2x.png
512 1 icon_512x512.png
512 2 icon_512x512@2x.png
EOF

iconutil -c icns "$ICONSET" -o "$LOGO_DIR/AutotaskMenu.icns"
cp "$LOGO_DIR/AutotaskMenu.icns" "$UI_RESOURCES/AutotaskMenu.icns"

magick "$LOGO_DIR/autotask-logo-clock-check.png" -alpha set -fuzz 18% -transparent white -resize 18x18 PNG32:"$UI_RESOURCES/menubar-icon.png"
magick "$LOGO_DIR/autotask-logo-clock-check.png" -alpha set -fuzz 18% -transparent white -resize 36x36 PNG32:"$UI_RESOURCES/menubar-icon@2x.png"

rsvg-convert -w 18 -h 18 "$LOGO_DIR/source/autotask-menubar-template.svg" -o "$UI_RESOURCES/menubar-template.png"
rsvg-convert -w 36 -h 36 "$LOGO_DIR/source/autotask-menubar-template.svg" -o "$UI_RESOURCES/menubar-template@2x.png"

echo "Wrote:"
echo "  $LOGO_DIR/autotask-icon-1024.png"
echo "  $LOGO_DIR/AutotaskMenu.icns"
echo "  $UI_RESOURCES/AutotaskMenu.icns"
echo "  $UI_RESOURCES/menubar-icon.png"
echo "  $UI_RESOURCES/menubar-icon@2x.png"
echo "  $UI_RESOURCES/menubar-template.png"
echo "  $UI_RESOURCES/menubar-template@2x.png"
