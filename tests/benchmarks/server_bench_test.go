package benchmarks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/server"
	"github.com/anthropic/sitemon/internal/storage"
)

func BenchmarkHandleAPIStatus_Cached(b *testing.B) {
	dir := b.TempDir()
	dbPath := filepath.Join(dir, "bench.db")
	ctx := context.Background()

	store, err := storage.New(ctx, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 10; i++ {
		result := &storage.CheckResult{
			SiteName:     fmt.Sprintf("Site%d", i),
			URL:          fmt.Sprintf("https://site%d.example.com", i),
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			CheckedAt:    time.Now(),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			b.Fatal(err)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func BenchmarkHandleAPIStatus_100Sites_Cached(b *testing.B) {
	dir := b.TempDir()
	dbPath := filepath.Join(dir, "bench.db")
	ctx := context.Background()

	store, err := storage.New(ctx, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 100; i++ {
		result := &storage.CheckResult{
			SiteName:     fmt.Sprintf("Site%d", i),
			URL:          fmt.Sprintf("https://site%d.example.com", i),
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			CheckedAt:    time.Now(),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			b.Fatal(err)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New("127.0.0.1:0", store, logger)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}
