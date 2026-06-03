package storage

import "time"

const createSchema = `
CREATE TABLE IF NOT EXISTS sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    start_time       INTEGER NOT NULL,
    end_time         INTEGER NOT NULL,
    duration_seconds INTEGER NOT NULL,
    app_name         TEXT NOT NULL,
    window_title     TEXT,
    raw_class        TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_start ON sessions(start_time);
CREATE INDEX IF NOT EXISTS idx_sessions_app   ON sessions(app_name);
`

type Session struct {
	StartTime       time.Time
	EndTime         time.Time
	DurationSeconds int
	AppName         string
	WindowTitle     string
	RawClass        string
}

type AppSummary struct {
	AppName string `json:"name"`
	Minutes int    `json:"minutes"`
}

type ChartEntry struct {
	Label       string `json:"label"`
	Minutes     int    `json:"minutes"`
	IsHighlight bool   `json:"is_highlight"`
}
