// Package storage provides a SQLite-backed persistence layer for site health
// check results. It uses the pure-Go modernc.org/sqlite driver (no CGO required).
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// CheckResult represents a single health check outcome stored in the database.
type CheckResult struct {
	ID           int64
	SiteName     string
	URL          string
	StatusCode   int
	ResponseTime time.Duration
	Success      bool
	CheckedAt    time.Time
}

// SiteStatus represents the current aggregated status of a monitored site.
type SiteStatus struct {
	SiteName       string
	URL            string
	LastStatusCode int
	ResponseTime   time.Duration
	Up             bool
	LastCheck      time.Time
	UptimePercent  float64
}

// Storage provides methods to persist and query health check data.
type Storage struct {
	db *sql.DB
}

// New creates a new Storage instance, opening the SQLite database at the given
// path and running schema migrations.
func New(ctx context.Context, dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	s := &Storage{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// migrate creates the required database tables if they do not already exist.
func (s *Storage) migrate(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS check_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_name TEXT NOT NULL,
		url TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		response_time_ms INTEGER NOT NULL,
		success INTEGER NOT NULL,
		checked_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_check_results_site_name ON check_results(site_name);
	CREATE INDEX IF NOT EXISTS idx_check_results_checked_at ON check_results(checked_at);
	`
	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	_, _ = s.db.ExecContext(ctx, "PRAGMA journal_mode=WAL")
	_, _ = s.db.ExecContext(ctx, "PRAGMA synchronous=NORMAL")

	return nil
}

// SaveCheckResult persists a single health check result to the database.
func (s *Storage) SaveCheckResult(ctx context.Context, result *CheckResult) error {
	query := `
		INSERT INTO check_results (site_name, url, status_code, response_time_ms, success, checked_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		result.SiteName,
		result.URL,
		result.StatusCode,
		result.ResponseTime.Milliseconds(),
		boolToInt(result.Success),
		result.CheckedAt,
	)
	if err != nil {
		return fmt.Errorf("saving check result: %w", err)
	}
	return nil
}

// GetSiteStatuses returns the current aggregated status for all sites that have
// been checked at least once, including 24-hour uptime percentages.
func (s *Storage) GetSiteStatuses(ctx context.Context) ([]SiteStatus, error) {
	since := time.Now().Add(-24 * time.Hour)

	query := `
		WITH latest AS (
			SELECT site_name, url, status_code, response_time_ms, success, checked_at,
				ROW_NUMBER() OVER (PARTITION BY site_name ORDER BY checked_at DESC) AS rn
			FROM check_results
		),
		uptime AS (
			SELECT site_name,
				CAST(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS REAL) * 100.0 / COUNT(*) AS uptime_pct
			FROM check_results
			WHERE checked_at >= ?
			GROUP BY site_name
		)
		SELECT l.site_name, l.url, l.status_code, l.response_time_ms, l.success, l.checked_at,
		       COALESCE(u.uptime_pct, 0)
		FROM latest l
		LEFT JOIN uptime u ON l.site_name = u.site_name
		WHERE l.rn = 1
		ORDER BY l.site_name
	`

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("querying site statuses: %w", err)
	}
	defer rows.Close()

	var statuses []SiteStatus
	for rows.Next() {
		var st SiteStatus
		var success int
		var respTimeMs int64
		if err := rows.Scan(
			&st.SiteName, &st.URL, &st.LastStatusCode, &respTimeMs, &success, &st.LastCheck, &st.UptimePercent,
		); err != nil {
			return nil, fmt.Errorf("scanning site status: %w", err)
		}
		st.ResponseTime = time.Duration(respTimeMs) * time.Millisecond
		st.Up = success == 1
		statuses = append(statuses, st)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating site statuses: %w", err)
	}

	return statuses, nil
}

// GetRecentChecks returns the most recent check results for a given site,
// limited to the specified count.
func (s *Storage) GetRecentChecks(ctx context.Context, siteName string, limit int) ([]CheckResult, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, site_name, url, status_code, response_time_ms, success, checked_at
		FROM check_results
		WHERE site_name = ?
		ORDER BY checked_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, siteName, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent checks: %w", err)
	}
	defer rows.Close()

	var results []CheckResult
	for rows.Next() {
		var r CheckResult
		var success int
		var respTimeMs int64
		if err := rows.Scan(&r.ID, &r.SiteName, &r.URL, &r.StatusCode, &respTimeMs, &success, &r.CheckedAt); err != nil {
			return nil, fmt.Errorf("scanning check result: %w", err)
		}
		r.ResponseTime = time.Duration(respTimeMs) * time.Millisecond
		r.Success = success == 1
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating check results: %w", err)
	}

	return results, nil
}

// Close closes the underlying database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}

// BoolToInt converts a bool to an int (1 for true, 0 for false) for SQLite storage.
// Exposed for external testing.
func BoolToInt(b bool) int {
	return boolToInt(b)
}

// boolToInt converts a bool to an int (1 for true, 0 for false) for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
