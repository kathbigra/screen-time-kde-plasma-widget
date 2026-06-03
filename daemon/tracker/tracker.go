package tracker

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kathbigra/activity-monitor/export"
	"github.com/kathbigra/activity-monitor/storage"
)

const (
	pollInterval   = 5 * time.Second
	flushInterval  = 10 * time.Second
	exportInterval = 10 * time.Second
	purgeInterval = 24 * time.Hour
	purgeAge      = 90 * 24 * time.Hour // 3 months
)

type windowDetector interface {
	ActiveWindow() (WindowInfo, error)
}

// WindowTracker polls for the active window and accumulates usage time.
type WindowTracker struct {
	detector    windowDetector
	resolver    *AppNameResolver
	accumulator *MemoryAccumulator
	db          *storage.Database
	exporter    *export.SummaryExporter
}

func NewWindowTracker(db *storage.Database, exporter *export.SummaryExporter) (*WindowTracker, error) {
	var detector windowDetector
	if isWayland() {
		d, err := NewWaylandDetector()
		if err != nil {
			return nil, fmt.Errorf("wayland detector: %w", err)
		}
		detector = d
	} else {
		detector = &X11Detector{}
	}

	return &WindowTracker{
		detector:    detector,
		resolver:    &AppNameResolver{},
		accumulator: &MemoryAccumulator{data: make(map[string]time.Duration)},
		db:          db,
		exporter:    exporter,
	}, nil
}

func (t *WindowTracker) Start(ctx context.Context) {
	pollTicker   := time.NewTicker(pollInterval)
	flushTicker  := time.NewTicker(flushInterval)
	exportTicker := time.NewTicker(exportInterval)
	purgeTicker  := time.NewTicker(purgeInterval)
	defer func() {
		pollTicker.Stop()
		flushTicker.Stop()
		exportTicker.Stop()
		purgeTicker.Stop()
	}()

	// Export once immediately so the widget has data as soon as the daemon starts.
	if err := t.exporter.Export(); err != nil {
		log.Printf("initial export: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			if err := t.accumulator.FlushTo(t.db); err != nil {
				log.Printf("shutdown flush: %v", err)
			}
			return

		case <-pollTicker.C:
			window, err := t.detector.ActiveWindow()
			if err != nil {
				log.Printf("detect window: %v", err)
				continue
			}
			if window.RawClass == "" && window.WindowTitle == "" {
				continue // no focused window
			}
			appName := t.resolver.Resolve(window)
			t.accumulator.Record(appName, pollInterval)

		case <-flushTicker.C:
			if err := t.accumulator.FlushTo(t.db); err != nil {
				log.Printf("flush: %v", err)
			}

		case <-exportTicker.C:
			if err := t.exporter.Export(); err != nil {
				log.Printf("export: %v", err)
			}

		case <-purgeTicker.C:
			cutoff := time.Now().Add(-purgeAge)
			if err := t.db.PurgeOlderThan(cutoff); err != nil {
				log.Printf("purge: %v", err)
			}
		}
	}
}

// MemoryAccumulator batches per-app durations before writing them to the DB.
type MemoryAccumulator struct {
	mu   sync.Mutex
	data map[string]time.Duration
}

func (a *MemoryAccumulator) Record(appName string, d time.Duration) {
	a.mu.Lock()
	a.data[appName] += d
	a.mu.Unlock()
}

func (a *MemoryAccumulator) FlushTo(db *storage.Database) error {
	a.mu.Lock()
	snapshot := a.data
	a.data = make(map[string]time.Duration)
	a.mu.Unlock()

	now := time.Now()
	for appName, duration := range snapshot {
		if duration < time.Second {
			continue
		}
		s := storage.Session{
			StartTime:       now.Add(-duration),
			EndTime:         now,
			DurationSeconds: int(duration.Seconds()),
			AppName:         appName,
		}
		if err := db.WriteSession(s); err != nil {
			return err
		}
	}
	return nil
}

func isWayland() bool {
	return os.Getenv("XDG_SESSION_TYPE") == "wayland"
}
