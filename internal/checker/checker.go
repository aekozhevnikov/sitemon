// Package checker provides concurrent HTTP health checking for configured sites.
// It sends results through a channel for consumers to process and store.
package checker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/anthropic/sitemon/internal/config"
	"github.com/anthropic/sitemon/internal/storage"
)

// Result represents the outcome of a single health check for a site.
type Result struct {
	SiteName     string
	URL          string
	StatusCode   int
	ResponseTime time.Duration
	Success      bool
	Error        error
	CheckedAt    time.Time
}

// Checker performs periodic HTTP health checks against a list of sites.
type Checker struct {
	sites    []config.Site
	timeout  time.Duration
	interval time.Duration
	client   *http.Client
	logger   *slog.Logger
}

// New creates a new Checker with the given configuration.
func New(sites []config.Site, timeout, interval time.Duration, logger *slog.Logger, transport *http.Transport) *Checker {
	if transport == nil {
		transport = &http.Transport{
			Proxy:             nil,
			DisableKeepAlives: false,
		}
	}
	return &Checker{
		sites:    sites,
		timeout:  timeout,
		interval: interval,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: transport,
		},
		logger: logger,
	}
}

// Start begins the periodic health checking loop. It checks all sites
// concurrently at each interval tick. Results are sent to the returned
// channel. The loop exits when the context is cancelled.
func (c *Checker) Start(ctx context.Context) <-chan Result {
	results := make(chan Result, len(c.sites)*2)

	go func() {
		defer close(results)

		// Run an immediate check on startup.
		c.checkAll(ctx, results)

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				c.logger.Info("checker stopping", "reason", ctx.Err())
				return
			case <-ticker.C:
				c.checkAll(ctx, results)
			}
		}
	}()

	return results
}

// CheckAll performs a health check for every configured site concurrently
// and sends each result to the provided channel. Exposed for external testing.
func (c *Checker) CheckAll(ctx context.Context, results chan<- Result) {
	c.checkAll(ctx, results)
}

// checkAll performs a health check for every configured site concurrently
// and sends each result to the provided channel.
func (c *Checker) checkAll(ctx context.Context, results chan<- Result) {
	var wg sync.WaitGroup
	for _, site := range c.sites {
		wg.Add(1)
		go func(s config.Site) {
			defer wg.Done()
			result := c.checkSite(ctx, s)
			select {
			case results <- result:
			case <-ctx.Done():
			}
		}(site)
	}
	wg.Wait()
}

// CheckSite performs a single HTTP GET request against the site and returns
// the result. It considers the check successful if the response status code
// matches the site's expected status. Exposed for external testing.
func (c *Checker) CheckSite(ctx context.Context, site config.Site) Result {
	return c.checkSite(ctx, site)
}

// checkSite performs a single HTTP GET request against the site and returns
// the result. It considers the check successful if the response status code
// matches the site's expected status.
func (c *Checker) checkSite(ctx context.Context, site config.Site) Result {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, site.URL, nil)
	if err != nil {
		return Result{
			SiteName:  site.Name,
			URL:       site.URL,
			Error:     fmt.Errorf("creating request: %w", err),
			CheckedAt: start,
		}
	}

	// Set a generic User-Agent to avoid being blocked by some services.
	req.Header.Set("User-Agent", "sitemon/1.0")

	resp, err := c.client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		c.logger.Warn("health check failed",
			"site", site.Name,
			"url", site.URL,
			"error", err,
		)
		return Result{
			SiteName:     site.Name,
			URL:          site.URL,
			ResponseTime: elapsed,
			Error:        fmt.Errorf("executing request: %w", err),
			CheckedAt:    start,
		}
	}
	defer resp.Body.Close()

	// Drain the body to allow connection reuse.
	io.Copy(io.Discard, resp.Body)

	success := resp.StatusCode == site.ExpectedStatus

	c.logger.Debug("health check completed",
		"site", site.Name,
		"status", resp.StatusCode,
		"expected", site.ExpectedStatus,
		"success", success,
		"response_time", elapsed,
	)

	return Result{
		SiteName:     site.Name,
		URL:          site.URL,
		StatusCode:   resp.StatusCode,
		ResponseTime: elapsed,
		Success:      success,
		CheckedAt:    start,
	}
}

// ToStorageResult converts a checker Result to a storage CheckResult.
func ToStorageResult(r *Result) *storage.CheckResult {
	return &storage.CheckResult{
		SiteName:     r.SiteName,
		URL:          r.URL,
		StatusCode:   r.StatusCode,
		ResponseTime: r.ResponseTime,
		Success:      r.Success,
		CheckedAt:    r.CheckedAt,
	}
}
