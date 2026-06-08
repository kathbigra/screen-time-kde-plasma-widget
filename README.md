# Screen Time

A privacy-first KDE Plasma 6 widget that tracks how much time you spend in each application — no cloud, no telemetry, everything stays on your machine.

## Screenshots

| Last 24 Hours | Today | This Week |
|---|---|---|
| ![Last 24 Hours](screenshots/last-24-hours.png) | ![Today](screenshots/today.png) | ![This Week](screenshots/this-week.png) |

## Features

- Per-app usage breakdown with icons
- Bar chart with Y-axis time markers
- Five time filters: Last 24 Hours, Today, This Week (last 7 days), This Month (last 4 weeks), Last 3 Months
- Resizable desktop widget — shows more apps as you make it taller
- Data stored locally in SQLite, purged after 3 months
- Wayland native (X11 fallback supported)
- Optional update notifications — no network activity unless you enable it

## Install

### From a release (no Go required)

Download the tarball for your architecture from the [latest release](https://github.com/kathbigra/screen-time-kde-plasma-widget/releases/latest), extract, and run:

```bash
tar -xzf screen-time-*-linux-amd64.tar.gz
cd screen-time-*-linux-amd64
bash install/install.sh
```

### From source

```bash
git clone https://github.com/kathbigra/screen-time-kde-plasma-widget.git
cd screen-time-kde-plasma-widget
bash install/install.sh
```

Requires `go` 1.22+.

Then right-click your desktop → **Add Widgets** → search for **Screen Time**.

## Prerequisites

- KDE Plasma 6
- `qdbus6` — usually ships with KDE (`qdbus-qt6` on openSUSE)
- `plasma5support6` — KDE QML module (`plasma5support6` on openSUSE)

## Updates

When a new version is available, the widget shows an update banner. You can:

- Click **Update** to be given the one-liner install command (or auto-update if you enabled it in settings)
- Click **Dismiss** to hide the banner until the next version

Update checking is opt-out — disable it in the widget's settings page. No data leaves your machine as part of the check; it is a plain request to the GitHub releases API with no identifiers.

## Uninstall

```bash
systemctl --user disable --now screen-time
rm ~/.local/bin/screen-time
rm ~/.config/systemd/user/screen-time.service
rm -rf ~/.local/share/plasma/plasmoids/com.github.kathbigra.activitymonitor
rm -rf ~/.local/share/icons/hicolor/256x256/apps/com.github.kathbigra.activitymonitor.png
rm -rf ~/.local/share/activity-monitor   # removes all stored data
```

## License

Source-available. You may read and study this code. You may not copy, reproduce, or reuse it in your own projects without explicit written permission. See [LICENSE](LICENSE) for full terms.
