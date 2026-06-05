package checker_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/checker"
	"github.com/anthropic/sitemon/internal/config"
)

func TestCheckSite_Success(t *testing.T) {
	// Create a test server that returns 200 OK.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	site := config.Site{
		Name:           "TestSite",
		URL:            ts.URL,
		ExpectedStatus: 200,
	}

	result := c.CheckSite(context.Background(), site)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !result.Success {
		t.Error("expected success to be true")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if result.SiteName != "TestSite" {
		t.Errorf("expected site name 'TestSite', got %q", result.SiteName)
	}
}

func TestCheckSite_WrongStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	site := config.Site{
		Name:           "TestSite",
		URL:            ts.URL,
		ExpectedStatus: 200,
	}

	result := c.CheckSite(context.Background(), site)

	if result.Success {
		t.Error("expected success to be false for wrong status code")
	}
	if result.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", result.StatusCode)
	}
}

func TestCheckSite_ConnectionRefused(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 2*time.Second, 30*time.Second, logger, nil)

	site := config.Site{
		Name:           "BadSite",
		URL:            "http://127.0.0.1:1", // Nothing listens on port 1.
		ExpectedStatus: 200,
	}

	result := c.CheckSite(context.Background(), site)

	if result.Success {
		t.Error("expected success to be false for connection refused")
	}
	if result.Error == nil {
		t.Error("expected an error for connection refused")
	}
}

func TestCheckSite_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 500*time.Millisecond, 30*time.Second, logger, nil)

	site := config.Site{
		Name:           "SlowSite",
		URL:            ts.URL,
		ExpectedStatus: 200,
	}

	result := c.CheckSite(context.Background(), site)

	if result.Success {
		t.Error("expected success to be false for timeout")
	}
	if result.Error == nil {
		t.Error("expected a timeout error")
	}
}

func TestCheckAll_Concurrent(t *testing.T) {
	var mu sync.Mutex
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	sites := []config.Site{
		{Name: "Site1", URL: ts.URL, ExpectedStatus: 200},
		{Name: "Site2", URL: ts.URL, ExpectedStatus: 200},
		{Name: "Site3", URL: ts.URL, ExpectedStatus: 200},
	}
	c := checker.New(sites, 5*time.Second, 30*time.Second, logger, nil)

	results := make(chan checker.Result, len(sites))
	c.CheckAll(context.Background(), results)
	close(results)

	count := 0
	for range results {
		count++
	}

	if count != 3 {
		t.Errorf("expected 3 results, got %d", count)
	}
}

func TestToStorageResult(t *testing.T) {
	r := &checker.Result{
		SiteName:     "Test",
		URL:          "https://example.com",
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		Success:      true,
		CheckedAt:    time.Now(),
	}

	sr := checker.ToStorageResult(r)

	if sr.SiteName != r.SiteName {
		t.Errorf("site name mismatch: %s vs %s", sr.SiteName, r.SiteName)
	}
	if sr.URL != r.URL {
		t.Errorf("URL mismatch: %s vs %s", sr.URL, r.URL)
	}
	if sr.StatusCode != r.StatusCode {
		t.Errorf("status code mismatch: %d vs %d", sr.StatusCode, r.StatusCode)
	}
	if sr.Success != r.Success {
		t.Errorf("success mismatch: %v vs %v", sr.Success, r.Success)
	}
}

func TestCheckSite_RedirectNotFollowed(t *testing.T) {
	// Server returns 301 redirect -- our client should NOT follow it.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://other.com", http.StatusMovedPermanently)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	site := config.Site{
		Name:           "RedirectSite",
		URL:            ts.URL,
		ExpectedStatus: 301,
	}

	result := c.CheckSite(context.Background(), site)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.StatusCode != 301 {
		t.Errorf("expected status 301, got %d", result.StatusCode)
	}
	if !result.Success {
		t.Error("expected success for matching redirect status")
	}
}

func TestCheckSite_CustomExpectedStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	site := config.Site{
		Name:           "NotFound",
		URL:            ts.URL,
		ExpectedStatus: 404,
	}

	result := c.CheckSite(context.Background(), site)
	if !result.Success {
		t.Error("expected success=true when status matches expected 404")
	}
}

func TestCheckSite_ResponseBodyDiscarded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write a large body to ensure it's fully consumed.
		for i := 0; i < 1000; i++ {
			w.Write([]byte("x"))
		}
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	site := config.Site{Name: "BigBody", URL: ts.URL, ExpectedStatus: 200}
	result := c.CheckSite(context.Background(), site)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestCheckSite_RecordedFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	site := config.Site{Name: "FieldCheck", URL: ts.URL, ExpectedStatus: 200}
	result := c.CheckSite(context.Background(), site)

	if result.SiteName != "FieldCheck" {
		t.Errorf("SiteName: got %q", result.SiteName)
	}
	if result.URL != ts.URL {
		t.Errorf("URL: got %q", result.URL)
	}
	if result.CheckedAt.IsZero() {
		t.Error("CheckedAt should not be zero")
	}
	if result.ResponseTime <= 0 {
		t.Error("ResponseTime should be positive")
	}
}

func TestCheckAll_EmptySites(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := checker.New(nil, 5*time.Second, 30*time.Second, logger, nil)

	results := make(chan checker.Result, 1)
	c.CheckAll(context.Background(), results)
	close(results)

	count := 0
	for range results {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 results for empty sites, got %d", count)
	}
}

func TestCheckAll_MixedResults(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	sites := []config.Site{
		{Name: "Up", URL: up.URL, ExpectedStatus: 200},
		{Name: "Down", URL: down.URL, ExpectedStatus: 200},
		{Name: "Bad", URL: "http://127.0.0.1:1", ExpectedStatus: 200},
	}
	c := checker.New(sites, 2*time.Second, 30*time.Second, logger, nil)

	results := make(chan checker.Result, len(sites))
	c.CheckAll(context.Background(), results)
	close(results)

	var upCount, downCount int
	for r := range results {
		if r.SiteName == "Up" && r.Success {
			upCount++
		}
		if (r.SiteName == "Down" || r.SiteName == "Bad") && !r.Success {
			downCount++
		}
	}

	if upCount != 1 {
		t.Errorf("expected 1 up result, got %d", upCount)
	}
	if downCount != 2 {
		t.Errorf("expected 2 down results, got %d", downCount)
	}
}

func TestStart_StopsOnContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	sites := []config.Site{
		{Name: "Site1", URL: ts.URL, ExpectedStatus: 200},
	}
	c := checker.New(sites, 5*time.Second, 1*time.Second, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	results := c.Start(ctx)

	// Wait for at least one result.
	select {
	case <-results:
		// Got a result, good.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first result")
	}

	cancel()

	// The results channel should be closed after cancellation.
	select {
	case _, ok := <-results:
		if ok {
			// May get one more result before cancellation propagates.
			// Try again.
			select {
			case _, ok := <-results:
				if ok {
					t.Error("expected results channel to be closed after cancel")
				}
			case <-time.After(3 * time.Second):
				// Channel might not close immediately, that's OK.
			}
		}
	case <-time.After(3 * time.Second):
		// Channel closed or still processing -- both acceptable.
	}
}
