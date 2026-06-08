package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kathbigra/activity-monitor/export"
	"github.com/kathbigra/activity-monitor/storage"
	"github.com/kathbigra/activity-monitor/tracker"
	"github.com/kathbigra/activity-monitor/updater"
)

var version = "dev"

func main() {
	dataDir := dataDirectory()
	dbPath := filepath.Join(dataDir, "data.db")
	summaryPath := filepath.Join(dataDir, "summary.json")
	updateSignal := filepath.Join(dataDir, "do_update")

	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	exporter := export.NewSummaryExporter(db, summaryPath, version)
	wt, err := tracker.NewWindowTracker(db, exporter)
	if err != nil {
		log.Fatalf("init tracker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	go pollUpdateSignal(ctx, updateSignal)

	log.Printf("screen-time daemon started v%s (db: %s)", version, dbPath)
	wt.Start(ctx)
	log.Println("screen-time daemon stopped")
}

func pollUpdateSignal(ctx context.Context, signalPath string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := os.Stat(signalPath); err != nil {
				continue
			}
			os.Remove(signalPath)
			log.Println("update signal received, self-updating...")
			if err := updater.SelfUpdate(); err != nil {
				log.Printf("self-update failed: %v", err)
			}
			// systemctl restart will terminate this process.
		}
	}
}

func dataDirectory() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "activity-monitor")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}
	return filepath.Join(home, ".local", "share", "activity-monitor")
}
