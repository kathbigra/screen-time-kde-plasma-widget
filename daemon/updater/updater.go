package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repo     = "kathbigra/screen-time-kde-plasma-widget"
	widgetID = "com.github.kathbigra.activitymonitor"
)

// apiURLVar is a variable so tests can redirect it to a local mock server.
var apiURLVar = "https://api.github.com/repos/" + repo + "/releases/latest"

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// SelfUpdate downloads the latest release, replaces the daemon binary and widget
// files, then restarts the systemd service.
func SelfUpdate() error {
	rel, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}

	arch := runtime.GOARCH // "amd64" or "arm64"
	suffix := fmt.Sprintf("linux-%s", arch)
	wantName := fmt.Sprintf("screen-time-%s-%s.tar.gz", rel.TagName, suffix)

	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == wantName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no asset found for %s", wantName)
	}

	tmpDir, err := os.MkdirTemp("", "screen-time-update-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, wantName)
	if err := download(downloadURL, tarPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	if err := extractAndInstall(tarPath, rel.TagName, suffix, tmpDir); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	// Restart the service — the new binary is already in place.
	_ = exec.Command("systemctl", "--user", "restart", "screen-time").Run()
	return nil
}

func fetchLatestRelease() (*release, error) {
	req, err := http.NewRequest("GET", apiURLVar, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func download(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractAndInstall(tarPath, tag, suffix, tmpDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	topDir := fmt.Sprintf("screen-time-%s-%s", tag, suffix)
	tr := tar.NewReader(gz)

	binInstalled := false
	widgetDest := widgetInstallPath()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		rel := strings.TrimPrefix(hdr.Name, topDir+"/")
		if rel == hdr.Name || rel == "" {
			continue
		}

		switch {
		case rel == "bin/screen-time":
			binPath := filepath.Join(tmpDir, "screen-time.new")
			if err := writeExecutable(tr, binPath); err != nil {
				return err
			}
			execPath, err := os.Executable()
			if err != nil {
				return err
			}
			if err := os.Rename(binPath, execPath); err != nil {
				return err
			}
			binInstalled = true

		case strings.HasPrefix(rel, "widget/"):
			if widgetDest == "" {
				continue
			}
			dest := filepath.Join(widgetDest, strings.TrimPrefix(rel, "widget/"))
			if hdr.Typeflag == tar.TypeDir {
				if err := os.MkdirAll(dest, 0755); err != nil {
					return err
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					return err
				}
				if err := writeFile(tr, dest, hdr.FileInfo().Mode()); err != nil {
					return err
				}
			}
		}
	}

	if !binInstalled {
		return fmt.Errorf("binary not found in archive")
	}
	return nil
}

func writeExecutable(r io.Reader, dest string) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func writeFile(r io.Reader, dest string, mode os.FileMode) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func widgetInstallPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "plasma", "plasmoids", widgetID)
}
