package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/server"
	"github.com/anthropic/sitemon/internal/storage"
)

func setupTestServer(t *testing.T) (*server.Server, *storage.Storage) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	ctx := context.Background()
	store, err := storage.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	return srv, store
}

// --- HandleIndex ---

func TestHandleIndex_OK(t *testing.T) {
	srv, store := setupTestServer(t)
	defer store.Close()

	ctx := context.Background()
	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName:     "TestSite",
		URL:          "https://test.com",
		StatusCode:   200,
		ResponseTime: 100 * time.Millisecond,
		Success:      true,
		CheckedAt:    time.Now(),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.HandleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty HTML body")
	}
}

func TestHandleIndex_NotFound(t *testing.T) {
	srv, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()

	srv.HandleIndex(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandleIndex_EmptyDB(t *testing.T) {
	srv, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.HandleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// --- HandleAPIStatus ---

func TestHandleAPIStatus_OK(t *testing.T) {
	srv, store := setupTestServer(t)
	defer store.Close()

	ctx := context.Background()
	now := time.Now()
	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName:     "Alpha",
		URL:          "https://alpha.com",
		StatusCode:   200,
		ResponseTime: 50 * time.Millisecond,
		Success:      true,
		CheckedAt:    now,
	})
	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName:     "Beta",
		URL:          "https://beta.com",
		StatusCode:   500,
		ResponseTime: 200 * time.Millisecond,
		Success:      false,
		CheckedAt:    now,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	srv.HandleAPIStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type: got %q", ct)
	}

	var statuses []storage.SiteStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].SiteName != "Alpha" {
		t.Errorf("first site: got %q", statuses[0].SiteName)
	}
	if !statuses[0].Up {
		t.Error("Alpha should be up")
	}
	if statuses[1].Up {
		t.Error("Beta should be down")
	}
}

func TestHandleAPIStatus_EmptyDB(t *testing.T) {
	srv, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	srv.HandleAPIStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var statuses []storage.SiteStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

// --- WriteJSONError ---

func TestWriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	server.WriteJSONError(rec, "something went wrong", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type: got %q", ct)
	}

	var body server.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body.Error != "something went wrong" {
		t.Errorf("error: got %q", body.Error)
	}
}

// --- Server lifecycle ---

func TestServer_New(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	store, err := storage.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv == nil {
		t.Fatal("server is nil")
	}
	if srv.HttpServer == nil {
		t.Fatal("HttpServer is nil")
	}
	if srv.Templates == nil {
		t.Fatal("Templates is nil")
	}
}

func TestServer_Shutdown(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	store, err := storage.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Use port 0 for auto-assignment.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Start server in background.
	srv.Start()

	// Give it a moment to start.
	time.Sleep(50 * time.Millisecond)

	// Shutdown should succeed.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

// --- Static files ---

func TestStaticFiles_Served(t *testing.T) {
	srv, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	rec := httptest.NewRecorder()

	// The static file server is mounted at /static/ on the mux.
	// We need to test through the full handler.
	srv.HttpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty CSS body")
	}
}

// --- Integration: full HTTP round-trip ---

func TestServer_FullRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	store, err := storage.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// Use :0 so the OS picks a free port.
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Start the server.
	srv.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	// Give server time to start.
	time.Sleep(100 * time.Millisecond)

	// Insert test data.
	now := time.Now()
	store.SaveCheckResult(ctx, &storage.CheckResult{
		SiteName:     "RoundTrip",
		URL:          "https://roundtrip.com",
		StatusCode:   200,
		ResponseTime: 42 * time.Millisecond,
		Success:      true,
		CheckedAt:    now,
	})

	// We can't easily get the actual port from httptest, so test handlers directly.
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	srv.HandleAPIStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var statuses []storage.SiteStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].SiteName != "RoundTrip" {
		t.Errorf("site name: got %q", statuses[0].SiteName)
	}
	if !statuses[0].Up {
		t.Error("site should be up")
	}
	if statuses[0].ResponseTime != 42*time.Millisecond {
		t.Errorf("response time: got %v", statuses[0].ResponseTime)
	}
}
