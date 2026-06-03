package tracker

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AppNameResolver resolves a WindowInfo to a human-readable app name.
// It uses a cascade: .desktop Name → WM class → "Other".
type AppNameResolver struct {
	mu    sync.Mutex
	cache map[string]string // lower(wmClass) -> display name
	built bool
}

func (r *AppNameResolver) Resolve(info WindowInfo) string {
	if info.RawClass == "" && info.WindowTitle == "" {
		return "Other"
	}

	r.mu.Lock()
	if !r.built {
		r.buildCache()
		r.built = true
	}
	r.mu.Unlock()

	class := strings.ToLower(info.RawClass)

	r.mu.Lock()
	name, ok := r.cache[class]
	r.mu.Unlock()

	if ok {
		return name
	}

	if info.RawClass != "" {
		return titleCase(info.RawClass)
	}
	return "Other"
}

func (r *AppNameResolver) buildCache() {
	r.cache = make(map[string]string)

	dirs := []string{"/usr/share/applications"}
	if home := os.Getenv("HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, ".local/share/applications"))
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				continue
			}
			name, wmClass := parseDesktopFile(filepath.Join(dir, entry.Name()))
			if name == "" {
				continue
			}
			if wmClass != "" {
				r.cache[strings.ToLower(wmClass)] = name
			}
			// Also index by filename stem (e.g. "firefox" from "firefox.desktop")
			stem := strings.TrimSuffix(entry.Name(), ".desktop")
			if _, exists := r.cache[stem]; !exists {
				r.cache[stem] = name
			}
		}
	}
}

func parseDesktopFile(path string) (name, wmClass string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	inDesktopEntry := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "[Desktop Entry]" {
			inDesktopEntry = true
			continue
		}
		if strings.HasPrefix(line, "[") && line != "[Desktop Entry]" {
			if inDesktopEntry {
				break // left the [Desktop Entry] section
			}
		}
		if !inDesktopEntry {
			continue
		}
		if strings.HasPrefix(line, "Name=") && name == "" {
			name = strings.TrimPrefix(line, "Name=")
		}
		if strings.HasPrefix(line, "StartupWMClass=") {
			wmClass = strings.TrimPrefix(line, "StartupWMClass=")
		}
	}
	return
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
