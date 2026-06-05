package benchmarks

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/storage"
)

func BenchmarkSaveCheckResult_Single(b *testing.B) {
	dir := b.TempDir()
	dbPath := filepath.Join(dir, "bench.db")
	ctx := context.Background()

	store, err := storage.New(ctx, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	result := &storage.CheckResult{
		SiteName:     "BenchSite",
		URL:          "https://example.com",
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		Success:      true,
		CheckedAt:    time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.SaveCheckResult(ctx, result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSaveCheckResult_DifferentSites(b *testing.B) {
	dir := b.TempDir()
	dbPath := filepath.Join(dir, "bench.db")
	ctx := context.Background()

	store, err := storage.New(ctx, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &storage.CheckResult{
			SiteName:     fmt.Sprintf("Site%d", i%100),
			URL:          fmt.Sprintf("https://site%d.example.com", i%100),
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			CheckedAt:    time.Now(),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetSiteStatuses_After100(b *testing.B) {
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
			SiteName:     fmt.Sprintf("Site%d", i%10),
			URL:          fmt.Sprintf("https://site%d.example.com", i%10),
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			CheckedAt:    time.Now(),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.GetSiteStatuses(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetSiteStatuses_After10000(b *testing.B) {
	dir := b.TempDir()
	dbPath := filepath.Join(dir, "bench.db")
	ctx := context.Background()

	store, err := storage.New(ctx, dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 10000; i++ {
		result := &storage.CheckResult{
			SiteName:     fmt.Sprintf("Site%d", i%50),
			URL:          fmt.Sprintf("https://site%d.example.com", i%50),
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
			CheckedAt:    time.Now(),
		}
		if err := store.SaveCheckResult(ctx, result); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.GetSiteStatuses(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
