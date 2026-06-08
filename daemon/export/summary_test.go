package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kathbigra/activity-monitor/storage"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newTestExporter(t *testing.T) *SummaryExporter {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	out := filepath.Join(t.TempDir(), "summary.json")
	return NewSummaryExporter(db, out, "v1.1.0")
}

func insertSession(t *testing.T, db *storage.Database, appName string, start time.Time, durationSeconds int) {
	t.Helper()
	s := storage.Session{
		StartTime:       start,
		EndTime:         start.Add(time.Duration(durationSeconds) * time.Second),
		DurationSeconds: durationSeconds,
		AppName:         appName,
	}
	// storage.Database.WriteSession is exported
	if err := db.WriteSession(s); err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

func assertEntry(t *testing.T, entries []storage.ChartEntry, idx int, wantLabel string, wantMinutes int, wantHighlight bool) {
	t.Helper()
	if idx >= len(entries) {
		t.Fatalf("entries[%d] out of range (len=%d)", idx, len(entries))
	}
	e := entries[idx]
	if e.Label != wantLabel {
		t.Errorf("entries[%d].Label = %q, want %q", idx, e.Label, wantLabel)
	}
	if e.Minutes != wantMinutes {
		t.Errorf("entries[%d].Minutes = %d, want %d", idx, e.Minutes, wantMinutes)
	}
	if e.IsHighlight != wantHighlight {
		t.Errorf("entries[%d].IsHighlight = %v, want %v", idx, e.IsHighlight, wantHighlight)
	}
}

// ── buildRollingWeekChart ─────────────────────────────────────────────────────

func TestBuildRollingWeekChart_AlwaysSevenBars(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 9, 15, 0, 0, 0, loc) // Monday
	from := today.AddDate(0, 0, -6)

	entries, err := e.buildRollingWeekChart(from, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 7 {
		t.Fatalf("want 7 entries, got %d", len(entries))
	}
}

func TestBuildRollingWeekChart_LabelsReflectActualDays(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	// today = Wednesday Jun 4; from = Thu May 29
	today := time.Date(2025, 6, 4, 15, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -6)

	entries, err := e.buildRollingWeekChart(from, today)
	if err != nil {
		t.Fatal(err)
	}

	wantLabels := []string{"Thu", "Fri", "Sat", "Sun", "Mon", "Tue", "Wed"}
	for i, want := range wantLabels {
		if entries[i].Label != want {
			t.Errorf("entries[%d].Label = %q, want %q", i, entries[i].Label, want)
		}
	}
}

func TestBuildRollingWeekChart_TodayHighlighted(t *testing.T) {
	tests := []struct {
		name    string
		today   time.Time
		wantIdx int
	}{
		{"Monday as today", time.Date(2025, 6, 9, 10, 0, 0, 0, time.Local), 6},
		{"Wednesday as today", time.Date(2025, 6, 4, 10, 0, 0, 0, time.Local), 6},
		{"Sunday as today", time.Date(2025, 6, 8, 10, 0, 0, 0, time.Local), 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExporter(t)
			from := tt.today.AddDate(0, 0, -6)
			entries, err := e.buildRollingWeekChart(from, tt.today)
			if err != nil {
				t.Fatal(err)
			}
			for i, entry := range entries {
				wantHighlight := i == tt.wantIdx
				if entry.IsHighlight != wantHighlight {
					t.Errorf("entries[%d].IsHighlight = %v, want %v (today idx=%d)",
						i, entry.IsHighlight, wantHighlight, tt.wantIdx)
				}
			}
		})
	}
}

func TestBuildRollingWeekChart_DataMappedToCorrectDay(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	// from uses midnight (how Export() computes it); to uses current time of day
	// (how Export() passes now), so sessions during the day are included.
	from := time.Date(2025, 5, 29, 0, 0, 0, 0, loc) // Thursday May 29 midnight
	now := time.Date(2025, 6, 4, 15, 0, 0, 0, loc)  // Wednesday Jun 4 15:00

	// 90m on the first day (Thu May 29) and 120m on today (Wed Jun 4)
	insertSession(t, e.db, "App", time.Date(2025, 5, 29, 10, 0, 0, 0, loc), 90*60)
	insertSession(t, e.db, "App", time.Date(2025, 6, 4, 10, 0, 0, 0, loc), 120*60)

	entries, err := e.buildRollingWeekChart(from, now)
	if err != nil {
		t.Fatal(err)
	}

	assertEntry(t, entries, 0, "Thu", 90, false)  // May 29
	assertEntry(t, entries, 1, "Fri", 0, false)
	assertEntry(t, entries, 6, "Wed", 120, true)  // Jun 4 (today)
}

func TestBuildRollingWeekChart_ZeroForDaysWithNoData(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 4, 15, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -6)

	entries, err := e.buildRollingWeekChart(from, today)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Minutes != 0 {
			t.Errorf("expected 0 minutes for all days, got %d for %s", entry.Minutes, entry.Label)
		}
	}
}

// ── buildRollingMonthChart ────────────────────────────────────────────────────

func TestBuildRollingMonthChart_AlwaysFourBuckets(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 9, 15, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -27)

	entries, err := e.buildRollingMonthChart(from, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("want 4 entries, got %d", len(entries))
	}
}

func TestBuildRollingMonthChart_LastBucketAlwaysHighlighted(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 9, 15, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -27)

	entries, err := e.buildRollingMonthChart(from, today)
	if err != nil {
		t.Fatal(err)
	}
	for i, entry := range entries {
		wantHighlight := i == 3
		if entry.IsHighlight != wantHighlight {
			t.Errorf("entries[%d].IsHighlight = %v, want %v", i, entry.IsHighlight, wantHighlight)
		}
	}
}

func TestBuildRollingMonthChart_BucketLabelsAreWeekStarts(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 9, 15, 0, 0, 0, loc) // Jun 9
	from := today.AddDate(0, 0, -27)                  // May 13

	entries, err := e.buildRollingMonthChart(from, today)
	if err != nil {
		t.Fatal(err)
	}

	// from = Jun 9 − 27 days = May 13; buckets: May 13, May 20, May 27, Jun 3
	wantLabels := []string{"May 13", "May 20", "May 27", "Jun 3"}
	for i, want := range wantLabels {
		if entries[i].Label != want {
			t.Errorf("entries[%d].Label = %q, want %q", i, entries[i].Label, want)
		}
	}
}

func TestBuildRollingMonthChart_DataAggregatedIntoWeeklyBuckets(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 9, 15, 0, 0, 0, loc) // Jun 9
	from := today.AddDate(0, 0, -27)                  // May 13

	// 60m spread across bucket 1 (May 13–19)
	insertSession(t, e.db, "App", time.Date(2025, 5, 14, 10, 0, 0, 0, loc), 1800) // 30m
	insertSession(t, e.db, "App", time.Date(2025, 5, 16, 10, 0, 0, 0, loc), 1800) // 30m

	// 90m in bucket 4 (Jun 3–9)
	insertSession(t, e.db, "App", time.Date(2025, 6, 5, 10, 0, 0, 0, loc), 5400) // 90m

	entries, err := e.buildRollingMonthChart(from, today)
	if err != nil {
		t.Fatal(err)
	}

	assertEntry(t, entries, 0, "May 13", 60, false)
	assertEntry(t, entries, 1, "May 20", 0, false)
	assertEntry(t, entries, 2, "May 27", 0, false)
	assertEntry(t, entries, 3, "Jun 3", 90, true)
}

func TestBuildRollingMonthChart_DataOnBucketBoundary(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	today := time.Date(2025, 6, 9, 15, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -27) // May 13

	// Session exactly on May 20 (start of bucket 2)
	insertSession(t, e.db, "App", time.Date(2025, 5, 20, 10, 0, 0, 0, loc), 3600) // 60m

	entries, err := e.buildRollingMonthChart(from, today)
	if err != nil {
		t.Fatal(err)
	}

	assertEntry(t, entries, 0, "May 13", 0, false)
	assertEntry(t, entries, 1, "May 20", 60, false)
}

// ── buildHourlyChart ──────────────────────────────────────────────────────────

func TestBuildHourlyChart_AlwaysTwentyFourBars(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 4, 14, 30, 0, 0, loc)
	from := now.Add(-24 * time.Hour)

	entries, err := e.buildHourlyChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 24 {
		t.Fatalf("want 24 entries, got %d", len(entries))
	}
}

func TestBuildHourlyChart_LastBarIsCurrentHour(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 4, 14, 30, 0, 0, loc)
	from := now.Add(-24 * time.Hour)

	entries, err := e.buildHourlyChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if !entries[23].IsHighlight {
		t.Error("last entry should be highlighted (current hour)")
	}
	if entries[23].Label != "14:00" {
		t.Errorf("last entry label = %q, want 14:00", entries[23].Label)
	}
}

func TestBuildHourlyChart_OnlyLastBarHighlighted(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 4, 14, 30, 0, 0, loc)
	from := now.Add(-24 * time.Hour)

	entries, err := e.buildHourlyChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	for i, entry := range entries {
		if i < 23 && entry.IsHighlight {
			t.Errorf("entries[%d] should not be highlighted", i)
		}
	}
}

// ── buildTodayHourlyChart ─────────────────────────────────────────────────────

func TestBuildTodayHourlyChart_AlwaysTwentyFourBars(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 4, 14, 30, 0, 0, loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	entries, err := e.buildTodayHourlyChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 24 {
		t.Fatalf("want 24 entries, got %d", len(entries))
	}
}

func TestBuildTodayHourlyChart_CurrentHourHighlighted(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 4, 9, 45, 0, 0, loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	entries, err := e.buildTodayHourlyChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if !entries[9].IsHighlight {
		t.Error("hour 9 should be highlighted")
	}
	for i, entry := range entries {
		if i != 9 && entry.IsHighlight {
			t.Errorf("entries[%d] should not be highlighted", i)
		}
	}
}

func TestBuildTodayHourlyChart_LabelsAreZeroPadded(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 4, 14, 0, 0, 0, loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	entries, err := e.buildTodayHourlyChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Label != "00:00" {
		t.Errorf("entries[0].Label = %q, want 00:00", entries[0].Label)
	}
	if entries[9].Label != "09:00" {
		t.Errorf("entries[9].Label = %q, want 09:00", entries[9].Label)
	}
	if entries[14].Label != "14:00" {
		t.Errorf("entries[14].Label = %q, want 14:00", entries[14].Label)
	}
}

// ── buildLast12WeeksChart ─────────────────────────────────────────────────────

func TestBuildLast12WeeksChart_AlwaysTwelveBars(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 9, 10, 0, 0, 0, loc)
	currentMonday := isoMonday(now)
	from := currentMonday.AddDate(0, 0, -11*7)

	entries, err := e.buildLast12WeeksChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 12 {
		t.Fatalf("want 12 entries, got %d", len(entries))
	}
}

func TestBuildLast12WeeksChart_CurrentWeekHighlighted(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 9, 10, 0, 0, 0, loc)
	currentMonday := isoMonday(now)
	from := currentMonday.AddDate(0, 0, -11*7)

	entries, err := e.buildLast12WeeksChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if !entries[11].IsHighlight {
		t.Error("last entry (current week) should be highlighted")
	}
	for i := 0; i < 11; i++ {
		if entries[i].IsHighlight {
			t.Errorf("entries[%d] should not be highlighted", i)
		}
	}
}

func TestBuildLast12WeeksChart_FirstLabelIsElevenWeeksAgo(t *testing.T) {
	e := newTestExporter(t)
	loc := time.Local
	now := time.Date(2025, 6, 9, 10, 0, 0, 0, loc) // Monday Jun 9
	currentMonday := isoMonday(now)                  // Jun 9
	from := currentMonday.AddDate(0, 0, -11*7)       // Jun 9 − 77 days = Mar 24

	entries, err := e.buildLast12WeeksChart(from, now)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Label != "Mar 24" {
		t.Errorf("entries[0].Label = %q, want Mar 24", entries[0].Label)
	}
	if entries[11].Label != "Jun 9" {
		t.Errorf("entries[11].Label = %q, want Jun 9", entries[11].Label)
	}
}

// ── Export (integration) ──────────────────────────────────────────────────────

func TestExport_AllFilterKeysPresent(t *testing.T) {
	e := newTestExporter(t)
	if err := e.Export(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(e.outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{"last_24_hours", "today", "this_week", "this_month", "last_3_months"}
	for _, key := range wantKeys {
		if _, ok := summary.Filters[key]; !ok {
			t.Errorf("missing filter key %q in exported summary", key)
		}
	}
}

func TestExport_VersionWrittenToOutput(t *testing.T) {
	e := newTestExporter(t)
	if err := e.Export(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(e.outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}

	if summary.Version != "v1.1.0" {
		t.Errorf("Version = %q, want v1.1.0", summary.Version)
	}
}

func TestExport_GeneratedAtIsSet(t *testing.T) {
	e := newTestExporter(t)
	if err := e.Export(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(e.outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}

	if summary.GeneratedAt == "" {
		t.Error("GeneratedAt should not be empty")
	}
}

func TestExport_EmptyAppsNotNil(t *testing.T) {
	e := newTestExporter(t)
	if err := e.Export(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(e.outputPath)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure apps arrays are [] not null in JSON.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	filters := raw["filters"].(map[string]interface{})
	for key, f := range filters {
		fd := f.(map[string]interface{})
		apps := fd["apps"]
		if apps == nil {
			t.Errorf("filter %q: apps is null, want []", key)
		}
	}
}

func TestExport_WritesAtomically(t *testing.T) {
	e := newTestExporter(t)

	// Run two exports concurrently — should not corrupt the output.
	done := make(chan error, 2)
	go func() { done <- e.Export() }()
	go func() { done <- e.Export() }()

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent export: %v", err)
		}
	}

	data, err := os.ReadFile(e.outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Errorf("output corrupted by concurrent writes: %v", err)
	}
}
