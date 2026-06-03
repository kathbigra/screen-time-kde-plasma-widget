#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

WIDGET_ID="com.github.kathbigra.activitymonitor"
WIDGET_DEST="$HOME/.local/share/plasma/plasmoids/$WIDGET_ID"
BIN_DIR="$HOME/.local/bin"
SYSTEMD_DIR="$HOME/.config/systemd/user"

echo "==> Building daemon..."
(cd "$REPO_ROOT/daemon" && go build -o "$BIN_DIR/screen-time" .)
echo "    Binary: $BIN_DIR/screen-time"

echo "==> Installing systemd service..."
mkdir -p "$SYSTEMD_DIR"
cp "$SCRIPT_DIR/screen-time.service" "$SYSTEMD_DIR/screen-time.service"

systemctl --user daemon-reload
systemctl --user enable --now screen-time
echo "    Service enabled and started."

echo "==> Installing Plasma widget..."
mkdir -p "$WIDGET_DEST"
cp -r "$REPO_ROOT/widget/." "$WIDGET_DEST/"
echo "    Widget installed to: $WIDGET_DEST"

echo "==> Installing icon..."
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
mkdir -p "$ICON_DIR"
cp "$REPO_ROOT/widget/icon.png" "$ICON_DIR/com.github.kathbigra.activitymonitor.png"
kbuildsycoca6 --noincremental 2>/dev/null || true
echo "    Icon installed."

echo ""
echo "Done."
echo "Right-click your desktop → Add Widgets → search for 'Screen Time'."
