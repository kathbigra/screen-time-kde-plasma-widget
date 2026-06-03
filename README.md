# Screen Time

A privacy-first KDE Plasma 6 widget that tracks and displays screen time per application — no cloud, no telemetry, all data stays on your machine.

## Features

- Tracks active window focus via KWin scripting (Wayland) or xprop (X11)
- Bar chart of daily screen time
- Ranked list of top apps by time
- Five filter periods: Last 24 Hours, Today, This Week, This Month, Last 3 Months
- Stores up to 3 months of data in a local SQLite database

## Prerequisites

- KDE Plasma 6 on Wayland (X11 fallback supported)
- `go` 1.22+ — only needed at install time to build the daemon binary
- `qdbus6` — usually installed with KDE (`qdbus-qt6` package on openSUSE)
- `plasma5support6` — KDE QML module (`plasma5support6` package on openSUSE)

## Install

```bash
git clone https://github.com/kathbigra/screen-time-widget.git
cd screen-time-widget
bash install/install.sh
```

The script will:
1. Build the Go daemon binary → `~/.local/bin/screen-time`
2. Install and enable the systemd user service
3. Copy the Plasma widget to `~/.local/share/plasma/plasmoids/`

Then right-click your desktop → **Add Widgets** → search for **Screen Time**.

## Uninstall

```bash
systemctl --user disable --now screen-time
rm ~/.local/bin/screen-time
rm ~/.config/systemd/user/screen-time.service
rm -rf ~/.local/share/plasma/plasmoids/com.github.kathbigra.activitymonitor
rm -rf ~/.local/share/screen-time   # removes all stored data
```

---

## Verifying the Full Stack

Use these steps to pinpoint exactly where something is broken, from daemon to widget.

### Step 1 — Is the daemon running?

```bash
systemctl --user status screen-time
```

Expected: `Active: active (running)`. If not running:

```bash
# Start it
systemctl --user start screen-time

# Check why it failed
journalctl --user -u screen-time -n 50
```

### Step 2 — Is the daemon using Wayland (not X11)?

```bash
# Check daemon logs — you should see NO "xprop" errors
journalctl --user -u screen-time -n 20

# Verify the daemon's environment has Wayland vars
cat /proc/$(systemctl --user show screen-time --property=MainPID --value)/environ \
  | tr '\0' '\n' | grep -E "XDG_SESSION|WAYLAND"
```

Expected: `XDG_SESSION_TYPE=wayland` and `WAYLAND_DISPLAY=wayland-0` in the output.
If missing: the service file is not setting them. Check `~/.config/systemd/user/screen-time.service` has:

```ini
Environment=XDG_SESSION_TYPE=wayland
Environment=WAYLAND_DISPLAY=wayland-0
```

Then reload:

```bash
systemctl --user daemon-reload && systemctl --user restart screen-time
```

### Step 3 — Is the KWin script loaded? (Wayland only)

The daemon loads a KWin script that fires a D-Bus callback on each window activation. Verify it loaded:

```bash
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.isScriptLoaded screen-time-watcher
```

Expected: `true`. If `false` or an error, the daemon failed to load the script — check `journalctl --user -u screen-time -n 30` for a `load kwin script:` error.

### Step 4 — Is the daemon capturing window activity?

Open a few applications and switch between them, then check the database for recent sessions:

```bash
sqlite3 ~/.local/share/screen-time/data.db \
  "SELECT app_name, datetime(start_time, 'unixepoch', 'localtime'), duration_seconds
   FROM sessions ORDER BY start_time DESC LIMIT 10;"
```

Expected: rows appearing with recent timestamps and recognisable app names (e.g. Konsole, Firefox).
If no rows, window detection is not working — check Step 2 and Step 3.

Check today's total:

```bash
sqlite3 ~/.local/share/screen-time/data.db \
  "SELECT count(*) sessions, sum(duration_seconds)/60 total_minutes
   FROM sessions WHERE start_time > strftime('%s', 'now', 'start of day');"
```

### Step 5 — Is `summary.json` being written and updated?

```bash
# Check it exists and when it was last written
stat ~/.local/share/screen-time/summary.json

# View today's data from it
cat ~/.local/share/screen-time/summary.json \
  | python3 -m json.tool | grep -A 6 '"today"'
```

`summary.json` is regenerated every 10 seconds. If it's not updating, or `today.total_minutes` doesn't match the DB, check daemon logs for `export:` errors.

### Step 6 — Is the widget loading `summary.json`?

```bash
# Check plasmashell logs for widget-specific messages
journalctl --user -b \
  | grep "plasmashell\[$(pgrep plasmashell | head -1)\]" \
  | grep -i "screen-time\|kathbigra\|error\|warn" \
  | tail -20
```

If you see `error when loading applet "com.github.kathbigra.activitymonitor"` — the widget package isn't installed. Run `install.sh` or copy the widget directory manually:

```bash
cp -r widget/ ~/.local/share/plasma/plasmoids/com.github.kathbigra.activitymonitor/
```

If you see `empty stdout` — the `cat` command isn't finding the file. Verify the path:

```bash
ls -la ~/.local/share/screen-time/summary.json
```

### Step 7 — Widget shows `—` instead of time

The widget loaded but `summaryData` is null — the DataSource read failed. Check Step 6 logs.
Also verify the `plasma5support` QML module is installed:

```bash
ls /usr/lib64/qt6/qml/org/kde/plasma/plasma5support/
```

If the directory doesn't exist, install the package:

```bash
sudo zypper install plasma5support6
```

### Step 8 — Widget compact view shows time but popup is blank

The full representation popup opened but rendered nothing. This was caused by `anchors.fill: parent` on the `ColumnLayout` in `main.qml` — the popup parent has no fixed size in Plasma 6, so the layout collapses. The fix is `Layout.minimumWidth/preferredWidth/minimumHeight/preferredHeight`.

Check that the installed `main.qml` has the Layout properties:

```bash
grep "Layout.minimumWidth\|Layout.preferredWidth" \
  ~/.local/share/plasma/plasmoids/com.github.kathbigra.activitymonitor/contents/ui/main.qml
```

If not present, re-run the install or copy from source:

```bash
cp widget/contents/ui/main.qml \
  ~/.local/share/plasma/plasmoids/com.github.kathbigra.activitymonitor/contents/ui/main.qml
```

Then restart plasmashell:

```bash
pkill plasmashell; sleep 1; nohup plasmashell > /dev/null 2>&1 &
```

---

## Troubleshooting Quick Reference

| Symptom | Likely cause | Check |
|---|---|---|
| Daemon not running | Binary missing or service not enabled | Step 1 |
| `xprop` errors in daemon log | Wrong session type (X11 detector used on Wayland) | Step 2 |
| No sessions in DB | KWin script not loaded; no window switches detected | Step 3, 4 |
| `summary.json` not updating | Daemon export error | Step 5 |
| Widget shows blank (no `—`) | Widget package not installed or QML load error | Step 6 |
| Widget shows `—` | `summary.json` not loading; plasma5support missing | Step 7 |
| Popup blank, compact shows time | `anchors.fill` layout bug in `main.qml` | Step 8 |

---

## Data Location

All data is stored in `~/.local/share/screen-time/`:
- `data.db` — SQLite database (authoritative store)
- `summary.json` — pre-computed summaries read by the widget (regenerated every 10 seconds)
- `kwin-watcher.js` — KWin script auto-written by the daemon on startup

## Architecture

```
Go daemon (systemd --user)
  KWin script → D-Bus callback → window detection on each focus change
  SQLite       → flush every 10s, purge data > 3 months daily
  summary.json → regenerated every 10 seconds

QML widget (Plasma 6)
  Reads summary.json every 10 seconds via plasma5support DataSource
  Pure display layer — no SQL, no computation
```

## License

Source-available. You may read and study this code. You may not copy, reproduce, or reuse it in your own projects without explicit written permission. See [LICENSE](LICENSE) for full terms.
