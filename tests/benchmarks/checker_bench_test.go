package benchmarks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/checker"
	"github.com/anthropic/sitemon/internal/config"
)

func makeSites(n int, url string) []config.Site {
	sites := make([]config.Site, n)
	for i := 0; i < n; i++ {
		sites[i] = config.Site{
			Name:           fmt.Sprintf("Site%d", i),
			URL:            url,
			ExpectedStatus: 200,
		}
	}
	return sites
}

func BenchmarkCheckAll_Small_10(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sites := makeSites(10, ts.URL)
	c := checker.New(sites, 5*time.Second, 30*time.Second, logger, nil)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make(chan checker.Result, len(sites)*2)
		c.CheckAll(ctx, results)
		close(results)
		for range results {
		}
	}
}

func BenchmarkCheckAll_Medium_100(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sites := makeSites(100, ts.URL)
	c := checker.New(sites, 5*time.Second, 30*time.Second, logger, nil)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make(chan checker.Result, len(sites)*2)
		c.CheckAll(ctx, results)
		close(results)
		for range results {
		}
	}
}

func BenchmarkCheckAll_Large_500(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sites := makeSites(500, ts.URL)
	c := checker.New(sites, 5*time.Second, 30*time.Second, logger, nil)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make(chan checker.Result, len(sites)*2)
		c.CheckAll(ctx, results)
		close(results)
		for range results {
		}
	}
}
