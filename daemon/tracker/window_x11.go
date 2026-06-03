package tracker

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// X11Detector detects the active window on X11 via xprop.
type X11Detector struct{}

var (
	reActiveWinID = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	// WM_CLASS(STRING) = "instance", "class"  — capture the second value (class)
	reWMClass = regexp.MustCompile(`WM_CLASS\(\w+\)\s*=\s*"[^"]*",\s*"([^"]*)"`)
	reWMName  = regexp.MustCompile(`_NET_WM_NAME\(\w+\)\s*=\s*"([^"]*)"`)
)

func (d *X11Detector) ActiveWindow() (WindowInfo, error) {
	rootOut, err := exec.Command("xprop", "-root", "_NET_ACTIVE_WINDOW").Output()
	if err != nil {
		return WindowInfo{}, fmt.Errorf("xprop _NET_ACTIVE_WINDOW: %w", err)
	}

	wid := reActiveWinID.FindString(string(rootOut))
	if wid == "" || wid == "0x0" {
		return WindowInfo{}, nil
	}

	propOut, err := exec.Command("xprop", "-id", wid, "WM_CLASS", "_NET_WM_NAME").Output()
	if err != nil {
		return WindowInfo{}, fmt.Errorf("xprop -id %s: %w", wid, err)
	}

	props := string(propOut)
	info := WindowInfo{}

	if m := reWMClass.FindStringSubmatch(props); m != nil {
		info.RawClass = strings.ToLower(m[1])
	}
	if m := reWMName.FindStringSubmatch(props); m != nil {
		info.WindowTitle = m[1]
	}

	return info, nil
}
