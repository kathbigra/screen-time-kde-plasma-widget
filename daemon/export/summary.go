package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kathbigra/activity-monitor/storage"
)

type Summary struct {
	GeneratedAt string                `json:"generated_at"`
	Filters     map[string]FilterData `json:"filters"`
}

type FilterData struct {
	TotalMinutes int                  `json:"total_minutes"`
	Apps         []storage.AppSummary `json:"apps"`
	Chart        []storage.ChartEntry `json:"chart"`
}

type SummaryExporter struct {
	db         *storage.Database
	outputPath string
}

func NewSummaryExporter(db *storage.Database, outputPath string) *SummaryExporter {
	return &SummaryExporter{db: db, outputPath: outputPath}
}

func (e *SummaryExporter) Export() error {
	now := time.Now()
	loc := now.Location()

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := today.AddDate(0, 0, -(weekday - 1))

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	currentMonday := weekStart
	last12WeekStart := currentMonday.AddDate(0, 0, -11*7)

	type filterSpec struct {
		key       string
		from      time.Time
		chartFunc func(from, to time.Time) ([]storage.ChartEntry, error)
	}

	specs := []filterSpec{
		{"last_24_hours", now.Add(-24 * time.Hour), e.buildHourlyChart},
		{"today", today, e.buildTodayHourlyChart},
		{"this_week", weekStart, e.buildWeekDailyChart},
		{"this_month", monthStart, e.buildMonthWeeklyChart},
		{"last_3_months", last12WeekStart, e.buildLast12WeeksChart},
	}

	summary := Summary{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Filters:     make(map[string]FilterData, len(specs)),
	}

	for _, spec := range specs {
		chart, err := spec.chartFunc(spec.from, now)
		if err != nil {
			return fmt.Errorf("build chart %s: %w", spec.key, err)
		}
		apps, err := e.db.QuerySummary(spec.from, now)
		if err != nil {
			return fmt.Errorf("QuerySummary %s: %w", spec.key, err)
		}
		if apps == nil {
			apps = []storage.AppSummary{}
		}
		total := 0
		for _, entry := range chart {
			total += entry.Minutes
		}
		summary.Filters[spec.key] = FilterData{
			TotalMinutes: total,
			Apps:         apps,
			Chart:        chart,
		}
	}

	return e.writeAtomic(summary)
}

// buildHourlyChart builds exactly 24 hourly bars ending at the current hour.
func (e *SummaryExporter) buildHourlyChart(from, to time.Time) ([]storage.ChartEntry, error) {
	hourMap, err := e.db.HourlyTotals(from, to)
	if err != nil {
		return nil, err
	}

	toHour := time.Date(to.Year(), to.Month(), to.Day(), to.Hour(), 0, 0, 0, to.Location())
	fromHour := toHour.Add(-23 * time.Hour)

	entries := make([]storage.ChartEntry, 24)
	for i := 0; i < 24; i++ {
		h := fromHour.Add(time.Duration(i) * time.Hour)
		entries[i] = storage.ChartEntry{
			Label:       fmt.Sprintf("%02d:00", h.Hour()),
			Minutes:     hourMap[h.Format("2006-01-02 15")],
			IsHighlight: i == 23,
		}
	}
	return entries, nil
}

// buildTodayHourlyChart builds exactly 24 hourly bars for today (00:00–23:00).
func (e *SummaryExporter) buildTodayHourlyChart(from, to time.Time) ([]storage.ChartEntry, error) {
	hourMap, err := e.db.HourlyTotals(from, to)
	if err != nil {
		return nil, err
	}

	currentHour := to.Hour()
	entries := make([]storage.ChartEntry, 24)
	for h := 0; h < 24; h++ {
		t := time.Date(from.Year(), from.Month(), from.Day(), h, 0, 0, 0, from.Location())
		entries[h] = storage.ChartEntry{
			Label:       fmt.Sprintf("%02d:00", h),
			Minutes:     hourMap[t.Format("2006-01-02 15")],
			IsHighlight: h == currentHour,
		}
	}
	return entries, nil
}

// buildWeekDailyChart builds exactly 7 daily bars (Mon–Sun), filling missing days with 0.
func (e *SummaryExporter) buildWeekDailyChart(from, to time.Time) ([]storage.ChartEntry, error) {
	weekEnd := from.AddDate(0, 0, 7)
	dailyMap, err := e.db.DailyTotals(from, weekEnd)
	if err != nil {
		return nil, err
	}

	todayStr := to.Format("2006-01-02")
	dayNames := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	entries := make([]storage.ChartEntry, 7)
	for i := 0; i < 7; i++ {
		day := from.AddDate(0, 0, i)
		dayStr := day.Format("2006-01-02")
		entries[i] = storage.ChartEntry{
			Label:       dayNames[i],
			Minutes:     dailyMap[dayStr],
			IsHighlight: dayStr == todayStr,
		}
	}
	return entries, nil
}

// buildMonthWeeklyChart builds weekly bars clipped to the current month.
func (e *SummaryExporter) buildMonthWeeklyChart(from, to time.Time) ([]storage.ChartEntry, error) {
	monthEnd := time.Date(from.Year(), from.Month()+1, 1, 0, 0, 0, 0, from.Location())

	weekMap, err := e.db.WeeklyTotals(from, monthEnd)
	if err != nil {
		return nil, err
	}

	currentMonday := isoMonday(to)
	firstWeekMonday := isoMonday(from)

	var entries []storage.ChartEntry
	for weekMon := firstWeekMonday; weekMon.Before(monthEnd); weekMon = weekMon.AddDate(0, 0, 7) {
		clippedStart := weekMon
		if clippedStart.Before(from) {
			clippedStart = from
		}
		entries = append(entries, storage.ChartEntry{
			Label:       clippedStart.Format("Jan 2"),
			Minutes:     weekMap[weekMon.Format("2006-01-02")],
			IsHighlight: weekMon.Equal(currentMonday),
		})
	}
	return entries, nil
}

// buildLast12WeeksChart builds 12 weekly bars going back from the current week.
func (e *SummaryExporter) buildLast12WeeksChart(from, to time.Time) ([]storage.ChartEntry, error) {
	weekMap, err := e.db.WeeklyTotals(from, to)
	if err != nil {
		return nil, err
	}

	currentMonday := isoMonday(to)
	entries := make([]storage.ChartEntry, 12)
	for i := 0; i < 12; i++ {
		weekMon := from.AddDate(0, 0, i*7)
		entries[i] = storage.ChartEntry{
			Label:       weekMon.Format("Jan 2"),
			Minutes:     weekMap[weekMon.Format("2006-01-02")],
			IsHighlight: weekMon.Equal(currentMonday),
		}
	}
	return entries, nil
}

func isoMonday(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	d := t.AddDate(0, 0, -(weekday - 1))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
}

func (e *SummaryExporter) writeAtomic(summary Summary) error {
	if err := os.MkdirAll(filepath.Dir(e.outputPath), 0700); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	tmpPath := e.outputPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmpPath, e.outputPath)
}
