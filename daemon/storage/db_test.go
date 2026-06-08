package storage

import (
	"testing"
	"time"
)

func newTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func insertSession(t *testing.T, db *Database, s Session) {
	t.Helper()
	if err := db.WriteSession(s); err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

func makeSession(appName string, start time.Time, durationSeconds int) Session {
	return Session{
		StartTime:       start,
		EndTime:         start.Add(time.Duration(durationSeconds) * time.Second),
		DurationSeconds: durationSeconds,
		AppName:         appName,
	}
}

// ── QuerySummary ─────────────────────────────────────────────────────────────

func TestQuerySummary_ReturnsTopApps(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local
	base := time.Date(2025, 6, 4, 10, 0, 0, 0, loc)

	insertSession(t, db, makeSession("Firefox", base, 3600))   // 60m
	insertSession(t, db, makeSession("VSCode", base, 1800))    // 30m
	insertSession(t, db, makeSession("Terminal", base, 600))   // 10m

	apps, err := db.QuerySummary(base.Add(-time.Hour), base.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 3 {
		t.Fatalf("want 3 apps, got %d", len(apps))
	}
	// Should be sorted descending by usage.
	if apps[0].AppName != "Firefox" {
		t.Errorf("want Firefox first, got %s", apps[0].AppName)
	}
	if apps[0].Minutes != 60 {
		t.Errorf("Firefox minutes = %d, want 60", apps[0].Minutes)
	}
}

func TestQuerySummary_ExcludesSessionsUnderOneMinute(t *testing.T) {
	db := newTestDB(t)
	base := time.Date(2025, 6, 4, 10, 0, 0, 0, time.Local)

	insertSession(t, db, makeSession("ShortApp", base, 59))  // 59s — below threshold
	insertSession(t, db, makeSession("LongApp", base, 60))   // 60s — exactly at threshold

	apps, err := db.QuerySummary(base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 app, got %d: %v", len(apps), apps)
	}
	if apps[0].AppName != "LongApp" {
		t.Errorf("want LongApp, got %s", apps[0].AppName)
	}
}

func TestQuerySummary_LimitsTen(t *testing.T) {
	db := newTestDB(t)
	base := time.Date(2025, 6, 4, 10, 0, 0, 0, time.Local)

	for i := 0; i < 15; i++ {
		name := "App" + string(rune('A'+i))
		insertSession(t, db, makeSession(name, base, (15-i)*60))
	}

	apps, err := db.QuerySummary(base.Add(-time.Minute), base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 10 {
		t.Fatalf("want 10 apps (limit), got %d", len(apps))
	}
}

func TestQuerySummary_RespectsTimeRange(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local
	base := time.Date(2025, 6, 4, 10, 0, 0, 0, loc)

	insertSession(t, db, makeSession("InRange", base, 300))
	insertSession(t, db, makeSession("OutOfRange", base.Add(-2*time.Hour), 300))

	apps, err := db.QuerySummary(base.Add(-time.Hour), base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].AppName != "InRange" {
		t.Errorf("want only InRange, got %v", apps)
	}
}

// ── DailyTotals ──────────────────────────────────────────────────────────────

func TestDailyTotals_AggregatesByDay(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local

	day1 := time.Date(2025, 6, 3, 10, 0, 0, 0, loc)
	day2 := time.Date(2025, 6, 4, 10, 0, 0, 0, loc)

	insertSession(t, db, makeSession("App", day1, 1800)) // 30m on day1
	insertSession(t, db, makeSession("App", day1, 600))  // 10m on day1
	insertSession(t, db, makeSession("App", day2, 3600)) // 60m on day2

	from := time.Date(2025, 6, 3, 0, 0, 0, 0, loc)
	to := time.Date(2025, 6, 5, 0, 0, 0, 0, loc)

	daily, err := db.DailyTotals(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if daily["2025-06-03"] != 40 {
		t.Errorf("day1 = %d, want 40", daily["2025-06-03"])
	}
	if daily["2025-06-04"] != 60 {
		t.Errorf("day2 = %d, want 60", daily["2025-06-04"])
	}
}

func TestDailyTotals_EmptyRange(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, loc)
	to := time.Date(2025, 6, 7, 0, 0, 0, 0, loc)

	daily, err := db.DailyTotals(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 0 {
		t.Errorf("want empty map, got %v", daily)
	}
}

// ── HourlyTotals ─────────────────────────────────────────────────────────────

func TestHourlyTotals_AggregatesByHour(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local

	h9 := time.Date(2025, 6, 4, 9, 0, 0, 0, loc)
	h10 := time.Date(2025, 6, 4, 10, 0, 0, 0, loc)

	insertSession(t, db, makeSession("App", h9, 1800))  // 30m at 09:xx
	insertSession(t, db, makeSession("App", h10, 3600)) // 60m at 10:xx

	from := time.Date(2025, 6, 4, 0, 0, 0, 0, loc)
	to := time.Date(2025, 6, 4, 23, 59, 59, 0, loc)

	hourly, err := db.HourlyTotals(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if hourly["2025-06-04 09"] != 30 {
		t.Errorf("09:00 = %d, want 30", hourly["2025-06-04 09"])
	}
	if hourly["2025-06-04 10"] != 60 {
		t.Errorf("10:00 = %d, want 60", hourly["2025-06-04 10"])
	}
}

func TestHourlyTotals_MultipleSessionsSameHour(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local
	h10 := time.Date(2025, 6, 4, 10, 0, 0, 0, loc)

	insertSession(t, db, makeSession("App", h10, 600))           // 10m
	insertSession(t, db, makeSession("App", h10.Add(20*time.Minute), 600)) // 10m

	from := time.Date(2025, 6, 4, 0, 0, 0, 0, loc)
	to := time.Date(2025, 6, 4, 23, 59, 59, 0, loc)

	hourly, err := db.HourlyTotals(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if hourly["2025-06-04 10"] != 20 {
		t.Errorf("10:00 = %d, want 20", hourly["2025-06-04 10"])
	}
}

// ── WeeklyTotals ─────────────────────────────────────────────────────────────

func TestWeeklyTotals_GroupsByISOMonday(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local

	// Week of Mon Jun 2: Tue Jun 3 and Thu Jun 5
	tue := time.Date(2025, 6, 3, 10, 0, 0, 0, loc)
	thu := time.Date(2025, 6, 5, 10, 0, 0, 0, loc)
	// Next week: Mon Jun 9
	mon2 := time.Date(2025, 6, 9, 10, 0, 0, 0, loc)

	insertSession(t, db, makeSession("App", tue, 1800)) // 30m
	insertSession(t, db, makeSession("App", thu, 1800)) // 30m
	insertSession(t, db, makeSession("App", mon2, 3600)) // 60m

	from := time.Date(2025, 6, 2, 0, 0, 0, 0, loc)
	to := time.Date(2025, 6, 10, 0, 0, 0, 0, loc)

	weekly, err := db.WeeklyTotals(from, to)
	if err != nil {
		t.Fatal(err)
	}

	// ISO Monday of first week is Jun 2.
	if weekly["2025-06-02"] != 60 {
		t.Errorf("week of Jun 2 = %d, want 60", weekly["2025-06-02"])
	}
	// ISO Monday of second week is Jun 9.
	if weekly["2025-06-09"] != 60 {
		t.Errorf("week of Jun 9 = %d, want 60", weekly["2025-06-09"])
	}
}

func TestWeeklyTotals_SundayBelongsToCorrectWeek(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local

	// ISO week: Mon Jun 2 – Sun Jun 8
	sun := time.Date(2025, 6, 8, 10, 0, 0, 0, loc)
	insertSession(t, db, makeSession("App", sun, 3600)) // 60m on Sunday

	from := time.Date(2025, 6, 2, 0, 0, 0, 0, loc)
	to := time.Date(2025, 6, 9, 0, 0, 0, 0, loc)

	weekly, err := db.WeeklyTotals(from, to)
	if err != nil {
		t.Fatal(err)
	}

	// Sunday Jun 8 should belong to the week starting Mon Jun 2, not Jun 9.
	if weekly["2025-06-02"] != 60 {
		t.Errorf("week of Jun 2 = %d, want 60 (Sunday should belong to its Monday)", weekly["2025-06-02"])
	}
	if weekly["2025-06-09"] != 0 {
		t.Errorf("week of Jun 9 = %d, want 0", weekly["2025-06-09"])
	}
}

// ── PurgeOlderThan ────────────────────────────────────────────────────────────

func TestPurgeOlderThan_RemovesOldKeepsNew(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local

	old := time.Date(2025, 1, 1, 10, 0, 0, 0, loc)
	recent := time.Date(2025, 6, 1, 10, 0, 0, 0, loc)
	cutoff := time.Date(2025, 3, 1, 0, 0, 0, 0, loc)

	insertSession(t, db, makeSession("OldApp", old, 3600))
	insertSession(t, db, makeSession("NewApp", recent, 3600))

	if err := db.PurgeOlderThan(cutoff); err != nil {
		t.Fatal(err)
	}

	apps, err := db.QuerySummary(old.Add(-time.Hour), recent.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].AppName != "NewApp" {
		t.Errorf("want only NewApp after purge, got %v", apps)
	}
}

func TestPurgeOlderThan_NothingToDelete(t *testing.T) {
	db := newTestDB(t)
	loc := time.Local
	recent := time.Date(2025, 6, 1, 10, 0, 0, 0, loc)
	insertSession(t, db, makeSession("App", recent, 3600))

	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, loc)
	if err := db.PurgeOlderThan(cutoff); err != nil {
		t.Fatal(err)
	}

	apps, err := db.QuerySummary(recent.Add(-time.Minute), recent.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Errorf("want 1 app, got %d", len(apps))
	}
}
