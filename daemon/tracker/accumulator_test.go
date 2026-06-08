package tracker

import (
	"testing"
	"time"

	"github.com/kathbigra/activity-monitor/storage"
)

func newTestDB(t *testing.T) *storage.Database {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ── Record ────────────────────────────────────────────────────────────────────

func TestAccumulator_RecordAddsToExisting(t *testing.T) {
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}

	a.Record("Firefox", 5*time.Second)
	a.Record("Firefox", 5*time.Second)
	a.Record("VSCode", 10*time.Second)

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.data["Firefox"] != 10*time.Second {
		t.Errorf("Firefox = %v, want 10s", a.data["Firefox"])
	}
	if a.data["VSCode"] != 10*time.Second {
		t.Errorf("VSCode = %v, want 10s", a.data["VSCode"])
	}
}

func TestAccumulator_RecordNewApp(t *testing.T) {
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}
	a.Record("NewApp", 30*time.Second)

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.data["NewApp"] != 30*time.Second {
		t.Errorf("NewApp = %v, want 30s", a.data["NewApp"])
	}
}

// ── FlushTo ───────────────────────────────────────────────────────────────────

func TestAccumulator_FlushTo_WritesSessionsToDB(t *testing.T) {
	db := newTestDB(t)
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}

	a.Record("Firefox", 2*time.Minute)
	a.Record("VSCode", 5*time.Minute)

	if err := a.FlushTo(db); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	apps, err := db.QuerySummary(now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("want 2 apps after flush, got %d", len(apps))
	}
}

func TestAccumulator_FlushTo_ClearsDataAfterFlush(t *testing.T) {
	db := newTestDB(t)
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}

	a.Record("Firefox", 2*time.Minute)
	if err := a.FlushTo(db); err != nil {
		t.Fatal(err)
	}

	a.mu.Lock()
	remaining := len(a.data)
	a.mu.Unlock()

	if remaining != 0 {
		t.Errorf("data map should be empty after flush, has %d entries", remaining)
	}
}

func TestAccumulator_FlushTo_DoubleFlushDoesNotDuplicate(t *testing.T) {
	db := newTestDB(t)
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}

	a.Record("Firefox", 2*time.Minute)
	if err := a.FlushTo(db); err != nil {
		t.Fatal(err)
	}
	// Second flush with empty accumulator — nothing new to write.
	if err := a.FlushTo(db); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	apps, err := db.QuerySummary(now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 app (no duplicates), got %d", len(apps))
	}
}

func TestAccumulator_FlushTo_SkipsSubSecondDurations(t *testing.T) {
	db := newTestDB(t)
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}

	a.Record("TinyApp", 500*time.Millisecond) // below 1s threshold
	a.Record("RealApp", 2*time.Minute)

	if err := a.FlushTo(db); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	apps, err := db.QuerySummary(now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, app := range apps {
		if app.AppName == "TinyApp" {
			t.Error("TinyApp should have been skipped (duration < 1s)")
		}
	}
}

func TestAccumulator_FlushTo_EmptyAccumulatorIsNoop(t *testing.T) {
	db := newTestDB(t)
	a := &MemoryAccumulator{data: make(map[string]time.Duration)}

	if err := a.FlushTo(db); err != nil {
		t.Fatalf("flush of empty accumulator should not error: %v", err)
	}
}
