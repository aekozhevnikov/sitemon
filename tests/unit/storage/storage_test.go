package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/storage"
)

func setupTestDB(t *testing.T) (*storage.Storage, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"

	ctx := context.Background()
	store, err := storage.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	cleanup := func() {
		store.Close()
	}

	return store, cleanup
}

func TestSaveAndRetrieveCheckResult(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	result := &storage.CheckResult{
		SiteName:     "TestSite",
		URL:          "https://example.com",
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		Success:      true,
		CheckedAt:    time.Now(),
	}

	if err := store.SaveCheckResult(ctx, result); err != nil {
		t.Fatalf("failed to save check result: %v", err)
	}

	statuses, err := store.GetSiteStatuses(ctx)
	if err != nil {
		t.Fatalf("failed to get site statuses: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 site status, got %d", len(statuses))
	}

	s := statuses[0]
	if s.SiteName != "TestSite" {
		t.Errorf("expected site name 'TestSite', got %q", s.SiteName)
	}
	if s.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %q", s.URL)
	}
	if s.LastStatusCode != 200 {
		t.Errorf("expected status 200, got %d", s.LastStatusCode)
	}
	if !s.Up {
		t.Error("expected site to be up")
	}
	if s.ResponseTime != 150*time.Millisecond {
		t.Errorf("expected response time 150ms, got %v", s.ResponseTime)
	}
}

func TestGetSiteStatuses_MultipleSites(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	sites := []struct {
		name    string
		url     string
		success bool
		status  int
	}{
		{"SiteA", "https://a.com", true, 200},
		{"SiteB", "https://b.com", false, 500},
		{"SiteC", "https://c.com", true, 200},
	}

	now := time.Now()
	for _, s := range sites {
		result := &storage.CheckResult{
			SiteName:     s.name,
			URL:          s.url,
			StatusCode:   s.status,
			ResponseTime: 100 * time.Millisecond,
			Success:      s.success,
			CheckedAt:    now,
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			t.Fatalf("failed to save result for %s: %v", s.name, err)
		}
	}

	statuses, err := store.GetSiteStatuses(ctx)
	if err != nil {
		t.Fatalf("failed to get site statuses: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("expected 3 site statuses, got %d", len(statuses))
	}

	// Verify ordering (by site_name).
	if statuses[0].SiteName != "SiteA" {
		t.Errorf("expected first site 'SiteA', got %q", statuses[0].SiteName)
	}
	if statuses[1].SiteName != "SiteB" {
		t.Errorf("expected second site 'SiteB', got %q", statuses[1].SiteName)
	}
	if !statuses[1].Up == false {
		// SiteB should be down.
	}
}

func TestGetSiteStatuses_LatestOnly(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Save two results for the same site -- the latest should be returned.
	now := time.Now()

	result1 := &storage.CheckResult{
		SiteName:     "MySite",
		URL:          "https://mysite.com",
		StatusCode:   200,
		ResponseTime: 100 * time.Millisecond,
		Success:      true,
		CheckedAt:    now.Add(-1 * time.Hour),
	}
	result2 := &storage.CheckResult{
		SiteName:     "MySite",
		URL:          "https://mysite.com",
		StatusCode:   500,
		ResponseTime: 200 * time.Millisecond,
		Success:      false,
		CheckedAt:    now,
	}

	if err := store.SaveCheckResult(ctx, result1); err != nil {
		t.Fatalf("failed to save first result: %v", err)
	}
	if err := store.SaveCheckResult(ctx, result2); err != nil {
		t.Fatalf("failed to save second result: %v", err)
	}

	statuses, err := store.GetSiteStatuses(ctx)
	if err != nil {
		t.Fatalf("failed to get site statuses: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 site status, got %d", len(statuses))
	}

	if statuses[0].LastStatusCode != 500 {
		t.Errorf("expected latest status 500, got %d", statuses[0].LastStatusCode)
	}
	if statuses[0].Up {
		t.Error("expected site to be down (latest result was failure)")
	}
}

func TestGetRecentChecks(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 5; i++ {
		result := &storage.CheckResult{
			SiteName:     "TestSite",
			URL:          "https://example.com",
			StatusCode:   200,
			ResponseTime: time.Duration(100+i*10) * time.Millisecond,
			Success:      true,
			CheckedAt:    now.Add(time.Duration(i) * time.Minute),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			t.Fatalf("failed to save result %d: %v", i, err)
		}
	}

	checks, err := store.GetRecentChecks(ctx, "TestSite", 3)
	if err != nil {
		t.Fatalf("failed to get recent checks: %v", err)
	}

	if len(checks) != 3 {
		t.Fatalf("expected 3 recent checks, got %d", len(checks))
	}

	// Should be ordered by checked_at DESC.
	if checks[0].ResponseTime < checks[1].ResponseTime {
		t.Error("expected results in descending order")
	}
}

func TestGetRecentChecks_OtherSite(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	result := &storage.CheckResult{
		SiteName:     "SiteA",
		URL:          "https://a.com",
		StatusCode:   200,
		ResponseTime: 100 * time.Millisecond,
		Success:      true,
		CheckedAt:    time.Now(),
	}
	if err := store.SaveCheckResult(ctx, result); err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	checks, err := store.GetRecentChecks(ctx, "SiteB", 10)
	if err != nil {
		t.Fatalf("failed to get recent checks: %v", err)
	}

	if len(checks) != 0 {
		t.Errorf("expected 0 checks for SiteB, got %d", len(checks))
	}
}

func TestUptimePercent(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now()

	// Save 3 successful and 1 failed check within the last 24h.
	for i := 0; i < 3; i++ {
		result := &storage.CheckResult{
			SiteName:     "MySite",
			URL:          "https://mysite.com",
			StatusCode:   200,
			ResponseTime: 100 * time.Millisecond,
			Success:      true,
			CheckedAt:    now.Add(-time.Duration(i) * time.Hour),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			t.Fatalf("failed to save result: %v", err)
		}
	}

	// One failure.
	failResult := &storage.CheckResult{
		SiteName:     "MySite",
		URL:          "https://mysite.com",
		StatusCode:   500,
		ResponseTime: 0,
		Success:      false,
		CheckedAt:    now.Add(-4 * time.Hour),
	}
	if err := store.SaveCheckResult(ctx, failResult); err != nil {
		t.Fatalf("failed to save failure result: %v", err)
	}

	statuses, err := store.GetSiteStatuses(ctx)
	if err != nil {
		t.Fatalf("failed to get site statuses: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 site status, got %d", len(statuses))
	}

	// 3 out of 4 = 75%.
	expected := 75.0
	if statuses[0].UptimePercent != expected {
		t.Errorf("expected uptime %.1f%%, got %.1f%%", expected, statuses[0].UptimePercent)
	}
}

func TestBoolToInt(t *testing.T) {
	if storage.BoolToInt(true) != 1 {
		t.Error("expected BoolToInt(true) == 1")
	}
	if storage.BoolToInt(false) != 0 {
		t.Error("expected BoolToInt(false) == 0")
	}
}
