// Package server provides an HTTP web dashboard for viewing site monitoring
// status. It serves both the HTML dashboard and a JSON API.
package server

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/anthropic/sitemon/internal/storage"
	"github.com/anthropic/sitemon/web"
)

type statusCache struct {
	mu        sync.RWMutex
	data      []storage.SiteStatus
	updatedAt time.Time
}

// Server wraps an HTTP server with dependencies needed by handlers.
type Server struct {
	HttpServer *http.Server
	Storage    *storage.Storage
	Logger     *slog.Logger
	Templates  *template.Template
	cache      *statusCache
	cacheTTL   time.Duration
}

// New creates a new Server with the given address, storage, and logger.
// It sets up all HTTP routes and parses embedded templates.
func New(addr string, store *storage.Storage, logger *slog.Logger) (*Server, error) {
	mux := http.NewServeMux()

	s := &Server{
		Storage:  store,
		Logger:   logger,
		cache:    &statusCache{},
		cacheTTL: 5 * time.Second,
	}

	tmpl, err := template.ParseFS(web.TemplatesFS, "templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	s.Templates = tmpl

	// Routes.
	mux.HandleFunc("/", s.HandleIndex)
	mux.HandleFunc("/api/status", s.HandleAPIStatus)
	mux.Handle("/static/", http.FileServer(http.FS(web.StaticFS)))

	s.HttpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Start begins listening for HTTP requests in a goroutine. It returns
// immediately; use Shutdown to stop the server gracefully.
func (s *Server) Start() {
	go func() {
		s.Logger.Info("web dashboard starting", "addr", s.HttpServer.Addr)
		if err := s.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.Logger.Error("web server error", "error", err)
		}
	}()
}

// ServeHTTP implements http.Handler for testing purposes.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.HttpServer.Handler.ServeHTTP(w, r)
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.HttpServer.Shutdown(ctx)
}

// InvalidateCache clears the status cache, forcing the next request to fetch fresh data.
func (s *Server) InvalidateCache() {
	s.cache.mu.Lock()
	s.cache.updatedAt = time.Time{}
	s.cache.mu.Unlock()
}

func (s *Server) getCachedStatuses(ctx context.Context) ([]storage.SiteStatus, error) {
	s.cache.mu.RLock()
	if time.Since(s.cache.updatedAt) < s.cacheTTL {
		data := s.cache.data
		s.cache.mu.RUnlock()
		return data, nil
	}
	s.cache.mu.RUnlock()

	statuses, err := s.Storage.GetSiteStatuses(ctx)
	if err != nil {
		return nil, err
	}

	s.cache.mu.Lock()
	s.cache.data = statuses
	s.cache.updatedAt = time.Now()
	s.cache.mu.Unlock()

	return statuses, nil
}
