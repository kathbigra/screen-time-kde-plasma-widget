package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func Open(path string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite is single-writer

	if _, err := db.Exec(createSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) WriteSession(s Session) error {
	_, err := d.db.Exec(
		`INSERT INTO sessions (start_time, end_time, duration_seconds, app_name, window_title, raw_class)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.StartTime.Unix(),
		s.EndTime.Unix(),
		s.DurationSeconds,
		s.AppName,
		s.WindowTitle,
		s.RawClass,
	)
	return err
}

func (d *Database) QuerySummary(from, to time.Time) ([]AppSummary, error) {
	rows, err := d.db.Query(`
		SELECT app_name, SUM(duration_seconds) / 60 AS minutes
		FROM sessions
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY app_name
		HAVING SUM(duration_seconds) >= 60
		ORDER BY SUM(duration_seconds) DESC
		LIMIT 10`,
		from.Unix(), to.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AppSummary
	for rows.Next() {
		var a AppSummary
		if err := rows.Scan(&a.AppName, &a.Minutes); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (d *Database) DailyTotals(from, to time.Time) (map[string]int, error) {
	rows, err := d.db.Query(`
		SELECT date(start_time, 'unixepoch', 'localtime') AS day,
		       SUM(duration_seconds) / 60 AS minutes
		FROM sessions
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY day
		ORDER BY day ASC`,
		from.Unix(), to.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var day string
		var minutes int
		if err := rows.Scan(&day, &minutes); err != nil {
			return nil, err
		}
		result[day] = minutes
	}
	return result, rows.Err()
}

func (d *Database) HourlyTotals(from, to time.Time) (map[string]int, error) {
	rows, err := d.db.Query(`
		SELECT strftime('%Y-%m-%d %H', start_time, 'unixepoch', 'localtime') AS hour_key,
		       SUM(duration_seconds) / 60 AS minutes
		FROM sessions
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY hour_key`,
		from.Unix(), to.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var key string
		var minutes int
		if err := rows.Scan(&key, &minutes); err != nil {
			return nil, err
		}
		result[key] = minutes
	}
	return result, rows.Err()
}

func (d *Database) WeeklyTotals(from, to time.Time) (map[string]int, error) {
	dailyMap, err := d.DailyTotals(from, to)
	if err != nil {
		return nil, err
	}
	loc := from.Location()
	weekMap := make(map[string]int)
	for dayStr, minutes := range dailyMap {
		day, err := time.ParseInLocation("2006-01-02", dayStr, loc)
		if err != nil {
			continue
		}
		monday := isoMonday(day)
		weekMap[monday.Format("2006-01-02")] += minutes
	}
	return weekMap, nil
}

func isoMonday(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	d := t.AddDate(0, 0, -(weekday - 1))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
}

func (d *Database) PurgeOlderThan(cutoff time.Time) error {
	_, err := d.db.Exec(`DELETE FROM sessions WHERE start_time < ?`, cutoff.Unix())
	return err
}

func (d *Database) BootTime() (time.Time, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return time.Time{}, fmt.Errorf("read /proc/uptime: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return time.Time{}, fmt.Errorf("empty /proc/uptime")
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse uptime: %w", err)
	}
	return time.Now().Add(-time.Duration(secs * float64(time.Second))), nil
}
