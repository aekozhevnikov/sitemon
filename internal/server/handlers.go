package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropic/sitemon/internal/storage"
)

// HandleIndex renders the main dashboard HTML template.
func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	s.handleIndex(w, r)
}

// handleIndex renders the main dashboard HTML template.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	statuses, err := s.Storage.GetSiteStatuses(r.Context())
	if err != nil {
		s.Logger.Error("failed to get site statuses", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Sites     []storage.SiteStatus
		CheckedAt time.Time
	}{
		Sites:     statuses,
		CheckedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.Templates.ExecuteTemplate(w, "index.html", data); err != nil {
		s.Logger.Error("failed to render template", "error", err)
	}
}

// HandleAPIStatus returns a JSON summary of all site statuses.
func (s *Server) HandleAPIStatus(w http.ResponseWriter, r *http.Request) {
	s.handleAPIStatus(w, r)
}

// handleAPIStatus returns a JSON summary of all site statuses.
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	statuses, err := s.getCachedStatuses(r.Context())
	if err != nil {
		s.Logger.Error("failed to get site statuses for API", "error", err)
		WriteJSONError(w, "failed to fetch statuses", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		s.Logger.Error("failed to encode API response", "error", err)
	}
}

// APIError is a simple JSON error response structure.
type APIError struct {
	Error string `json:"error"`
}

// WriteJSONError writes a JSON-encoded error response with the given status code.
func WriteJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}

// writeJSONError writes a JSON-encoded error response with the given status code.
func writeJSONError(w http.ResponseWriter, msg string, status int) {
	WriteJSONError(w, msg, status)
}
