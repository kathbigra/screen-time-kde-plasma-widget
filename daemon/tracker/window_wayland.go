package tracker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

// WindowInfo holds information about an active window.
type WindowInfo struct {
	AppName     string
	WindowTitle string
	RawClass    string
}

const dbusName = "com.kathbigra.ActivityMonitor"
const dbusPath = "/com/kathbigra/ActivityMonitor"
const dbusIface = "com.kathbigra.ActivityMonitor"

// kwinScript hooks workspace.windowActivated and calls back to the daemon via D-Bus.
const kwinScript = `
workspace.windowActivated.connect(function(client) {
    var cls   = client ? (client.resourceClass || "") : "";
    var title = client ? (client.caption       || "") : "";
    callDBus("com.kathbigra.ActivityMonitor", "/com/kathbigra/ActivityMonitor",
             "com.kathbigra.ActivityMonitor", "NotifyWindow", cls, title);
});

// Report the current active window immediately on load.
var cur = workspace.activeWindow;
if (cur) {
    callDBus("com.kathbigra.ActivityMonitor", "/com/kathbigra/ActivityMonitor",
             "com.kathbigra.ActivityMonitor", "NotifyWindow",
             cur.resourceClass || "", cur.caption || "");
}
`

// WaylandDetector receives active-window notifications from a KWin script via D-Bus.
type WaylandDetector struct {
	mu   sync.Mutex
	last WindowInfo
	conn *dbus.Conn
}

func NewWaylandDetector() (*WaylandDetector, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect session bus: %w", err)
	}

	d := &WaylandDetector{conn: conn}

	if err := conn.ExportMethodTable(map[string]interface{}{
		"NotifyWindow": d.NotifyWindow,
	}, dbus.ObjectPath(dbusPath), dbusIface); err != nil {
		return nil, fmt.Errorf("export D-Bus methods: %w", err)
	}

	reply, err := conn.RequestName(dbusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, fmt.Errorf("request D-Bus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, fmt.Errorf("D-Bus name %s already taken", dbusName)
	}

	if err := loadKWinScript(); err != nil {
		return nil, fmt.Errorf("load kwin script: %w", err)
	}

	return d, nil
}

// NotifyWindow is called by the KWin script over D-Bus on each window activation.
func (d *WaylandDetector) NotifyWindow(resourceClass, caption string) *dbus.Error {
	d.mu.Lock()
	d.last = WindowInfo{
		RawClass:    strings.ToLower(resourceClass),
		WindowTitle: caption,
	}
	d.mu.Unlock()
	return nil
}

func (d *WaylandDetector) ActiveWindow() (WindowInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.last, nil
}

func loadKWinScript() error {
	scriptPath, err := writeKWinScript()
	if err != nil {
		return err
	}

	pluginName := "activity-monitor-watcher"

	// Unload any previous instance so we always get a fresh script.
	exec.Command("qdbus6", "org.kde.KWin", "/Scripting",
		"org.kde.kwin.Scripting.unloadScript", pluginName).Run()

	out, err := exec.Command("qdbus6", "org.kde.KWin", "/Scripting",
		"org.kde.kwin.Scripting.loadScript", scriptPath, pluginName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("loadScript: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if err := exec.Command("qdbus6", "org.kde.KWin", "/Scripting",
		"org.kde.kwin.Scripting.start").Run(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	return nil
}

func writeKWinScript() (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "activity-monitor")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "kwin-watcher.js")
	if err := os.WriteFile(path, []byte(kwinScript), 0644); err != nil {
		return "", err
	}
	return path, nil
}
