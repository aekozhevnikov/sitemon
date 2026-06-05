package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/checker"
	"github.com/anthropic/sitemon/internal/config"
	"github.com/anthropic/sitemon/internal/server"
	"github.com/anthropic/sitemon/internal/storage"
)

// setupServer creates a server backed by httptest for reliable testing.
func setupServer(t *testing.T) (*httptest.Server, *server.Server, *storage.Storage, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	store, err := storage.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	// Use httptest to get a real listening server.
	ts := httptest.NewServer(srv)
	cleanup := func() {
		ts.Close()
		store.Close()
	}
	return ts, srv, store, cleanup
}

func httpGet(t *testing.T, url string) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// ─── Full pipeline ──────────────────────────────────────────────────────

func TestIntegration_FullPipeline(t *testing.T) {
	ts, _, store, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()
	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName:     "PipelineSite",
		URL:          "https://pipeline.com",
		StatusCode:   200,
		ResponseTime: 42 * time.Millisecond,
		Success:      true,
		CheckedAt:    time.Now(),
	})

	resp := httpGet(t, ts.URL+"/api/status")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var statuses []storage.SiteStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].SiteName != "PipelineSite" {
		t.Errorf("site name: got %q", statuses[0].SiteName)
	}
	if !statuses[0].Up {
		t.Error("site should be up")
	}
}

// ─── Site down → up transition ──────────────────────────────────────────

func TestIntegration_SiteDownToUp(t *testing.T) {
	ts, srv, store, cleanup := setupServer(t)
	defer cleanup()
	ctx := context.Background()

	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName: "TransitionSite", URL: "https://transition.com",
		StatusCode: 503, ResponseTime: 0, Success: false, CheckedAt: time.Now(),
	})

	resp := httpGet(t, ts.URL+"/api/status")
	var statuses []storage.SiteStatus
	json.NewDecoder(resp.Body).Decode(&statuses)
	resp.Body.Close()

	if len(statuses) != 1 || statuses[0].Up {
		t.Error("site should be down in phase 1")
	}

	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName: "TransitionSite", URL: "https://transition.com",
		StatusCode: 200, ResponseTime: 100 * time.Millisecond, Success: true, CheckedAt: time.Now(),
	})

	// Invalidate cache to get fresh data (cache TTL is 5s by default).
	srv.InvalidateCache()

	resp2 := httpGet(t, ts.URL+"/api/status")
	var statuses2 []storage.SiteStatus
	json.NewDecoder(resp2.Body).Decode(&statuses2)
	resp2.Body.Close()

	if len(statuses2) != 1 || !statuses2[0].Up {
		t.Error("site should be up in phase 2")
	}
}

// ─── Multiple sites ─────────────────────────────────────────────────────

func TestIntegration_MultipleSites(t *testing.T) {
	ts, _, store, cleanup := setupServer(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now()

	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName: "Healthy", URL: "https://healthy.com",
		StatusCode: 200, ResponseTime: 50 * time.Millisecond, Success: true, CheckedAt: now,
	})
	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName: "Sick", URL: "https://sick.com",
		StatusCode: 500, ResponseTime: 200 * time.Millisecond, Success: false, CheckedAt: now,
	})

	resp := httpGet(t, ts.URL+"/api/status")
	defer resp.Body.Close()

	var statuses []storage.SiteStatus
	json.NewDecoder(resp.Body).Decode(&statuses)
	if len(statuses) != 2 {
		t.Fatalf("expected 2, got %d", len(statuses))
	}
	if statuses[0].SiteName != "Healthy" || !statuses[0].Up {
		t.Error("Healthy should be up")
	}
	if statuses[1].SiteName != "Sick" || statuses[1].Up {
		t.Error("Sick should be down")
	}
}

// ─── HTML dashboard ─────────────────────────────────────────────────────

func TestIntegration_HTMLDashboard(t *testing.T) {
	ts, _, store, cleanup := setupServer(t)
	defer cleanup()
	ctx := context.Background()

	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName: "DashSite", URL: "https://dash.com",
		StatusCode: 200, ResponseTime: 100 * time.Millisecond, Success: true, CheckedAt: time.Now(),
	})

	resp := httpGet(t, ts.URL+"/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty HTML body")
	}
}

// ─── Static files ────────────────────────────────────────────────────────

func TestIntegration_StaticFiles(t *testing.T) {
	ts, _, _, cleanup := setupServer(t)
	defer cleanup()

	resp := httpGet(t, ts.URL+"/static/style.css")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty CSS body")
	}
}

// ─── 404 handling ───────────────────────────────────────────────────────

func TestIntegration_404(t *testing.T) {
	ts, _, _, cleanup := setupServer(t)
	defer cleanup()

	resp := httpGet(t, ts.URL+"/nonexistent")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.StatusCode)
	}
}

// ─── Config + checker end-to-end ────────────────────────────────────────

func TestIntegration_ConfigToChecker(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(fmt.Sprintf(`
check_interval: 10s
timeout: 5s
sites:
  - name: "E2ESite"
    url: "%s"
    expected_status: 200
server:
  addr: ":9999"
storage:
  path: "./e2e.db"
`, ts.URL)), 0644)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ckr := checker.New(cfg.Sites, cfg.Timeout, cfg.CheckInterval, logger, nil)
	results := ckr.Start(context.Background())

	select {
	case r := <-results:
		if !r.Success {
			t.Error("expected success")
		}
		if r.StatusCode != 200 {
			t.Errorf("status: got %d", r.StatusCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

// ─── Empty DB returns empty JSON ────────────────────────────────────────

func TestIntegration_EmptyDB(t *testing.T) {
	ts, _, _, cleanup := setupServer(t)
	defer cleanup()

	resp := httpGet(t, ts.URL+"/api/status")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}

	var statuses []storage.SiteStatus
	json.NewDecoder(resp.Body).Decode(&statuses)
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}
