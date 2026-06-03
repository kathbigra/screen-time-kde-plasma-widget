package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kathbigra/activity-monitor/export"
	"github.com/kathbigra/activity-monitor/storage"
	"github.com/kathbigra/activity-monitor/tracker"
)

func main() {
	dataDir := dataDirectory()
	dbPath := filepath.Join(dataDir, "data.db")
	summaryPath := filepath.Join(dataDir, "summary.json")

	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	exporter := export.NewSummaryExporter(db, summaryPath)
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

	log.Printf("screen-time daemon started (db: %s)", dbPath)
	wt.Start(ctx)
	log.Println("screen-time daemon stopped")
}

func dataDirectory() string {
	// Respect $XDG_DATA_HOME if set, otherwise default to ~/.local/share
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "activity-monitor")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}
	return filepath.Join(home, ".local", "share", "activity-monitor")
}
