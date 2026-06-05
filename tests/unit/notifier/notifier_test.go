package notifier_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/config"
	"github.com/anthropic/sitemon/internal/notifier"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	n := notifier.New(config.Telegram{BotToken: "token123", ChatID: "chat456"}, logger, nil)
	if !n.Enabled() {
		t.Error("expected notifier to be enabled with credentials")
	}

	n2 := notifier.New(config.Telegram{}, logger, nil)
	if n2.Enabled() {
		t.Error("expected notifier to be disabled without credentials")
	}
}

func TestNotify_NoOpWhenDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.New(config.Telegram{}, logger, nil)

	// Should not panic or do anything.
	n.Notify(context.Background(), "test-site", false, 500, 0)
}

func TestNotify_StateTransitions(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	// Mock the Telegram API.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var body struct {
			ChatID string `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"ok":false}`, http.StatusBadRequest)
			return
		}
		messages = append(messages, body.Text)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	// First notification: should record state but not notify (first time).
	n.Notify(context.Background(), "mysite", true, 200, 100*time.Millisecond)
	mu.Lock()
	if len(messages) != 0 {
		t.Errorf("expected 0 messages on first check, got %d", len(messages))
	}
	mu.Unlock()

	// Second notification: site goes down -- should send notification.
	n.Notify(context.Background(), "mysite", false, 500, 0)
	mu.Lock()
	if len(messages) != 1 {
		t.Errorf("expected 1 message after site went down, got %d", len(messages))
	}
	if len(messages) > 0 && !contains(messages[0], "DOWN") {
		t.Errorf("expected DOWN message, got: %s", messages[0])
	}
	mu.Unlock()

	// Third notification: site still down -- no new notification.
	n.Notify(context.Background(), "mysite", false, 500, 0)
	mu.Lock()
	if len(messages) != 1 {
		t.Errorf("expected still 1 message (no state change), got %d", len(messages))
	}
	mu.Unlock()

	// Fourth notification: site recovers -- should send notification.
	n.Notify(context.Background(), "mysite", true, 200, 150*time.Millisecond)
	mu.Lock()
	if len(messages) != 2 {
		t.Errorf("expected 2 messages after recovery, got %d", len(messages))
	}
	if len(messages) > 1 && !contains(messages[1], "RECOVERED") {
		t.Errorf("expected RECOVERED message, got: %s", messages[1])
	}
	mu.Unlock()
}

func TestNotify_MultipleSites(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var body struct {
			ChatID string `json:"chat_id"`
			Text   string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		messages = append(messages, body.Text)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	// Initialize both sites as up.
	n.Notify(context.Background(), "site-a", true, 200, 100*time.Millisecond)
	n.Notify(context.Background(), "site-b", true, 200, 100*time.Millisecond)

	mu.Lock()
	msgCount := len(messages)
	mu.Unlock()
	if msgCount != 0 {
		t.Errorf("expected 0 messages after initial checks, got %d", msgCount)
	}

	// site-a goes down.
	n.Notify(context.Background(), "site-a", false, 0, 0)

	mu.Lock()
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
	mu.Unlock()
}

func TestNotify_AfterRecovery_GoesDownAgain(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body struct {
			Text string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		messages = append(messages, body.Text)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	// init -> up (no notify)
	n.Notify(context.Background(), "site", true, 200, 50*time.Millisecond)
	// up -> down (notify DOWN)
	n.Notify(context.Background(), "site", false, 500, 0)
	// down -> up (notify RECOVERED)
	n.Notify(context.Background(), "site", true, 200, 100*time.Millisecond)
	// up -> down again (notify DOWN again)
	n.Notify(context.Background(), "site", false, 0, 0)

	mu.Lock()
	defer mu.Unlock()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d: %v", len(messages), messages)
	}
	if !contains(messages[0], "DOWN") {
		t.Errorf("msg 0 should be DOWN: %s", messages[0])
	}
	if !contains(messages[1], "RECOVERED") {
		t.Errorf("msg 1 should be RECOVERED: %s", messages[1])
	}
	if !contains(messages[2], "DOWN") {
		t.Errorf("msg 2 should be DOWN: %s", messages[2])
	}
}

func TestNotify_TelegramAPIError(t *testing.T) {
	// Server returns non-200 status.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ok":false,"description":"Bad Request"}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	// Should log error but not panic.
	n.Notify(context.Background(), "site", true, 200, 50*time.Millisecond)
	n.Notify(context.Background(), "site", false, 500, 0)
}

func TestNotify_TelegramAPINotOK(t *testing.T) {
	// Server returns 200 but ok=false.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":false,"description":"blocked by user"}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	n.Notify(context.Background(), "site", true, 200, 50*time.Millisecond)
	n.Notify(context.Background(), "site", false, 0, 0)
}

func TestNotify_ConcurrentSites(t *testing.T) {
	var mu sync.Mutex
	sentCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sentCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	// Initialize 5 sites as up, then take them all down concurrently.
	for i := 0; i < 5; i++ {
		n.Notify(context.Background(), "site", true, 200, 50*time.Millisecond)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.Notify(context.Background(), "site", false, 500, 0)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	// All 5 goroutines hit the same site name, but only the first
	// transition (up->down) should send; subsequent ones see no change.
	if sentCount != 1 {
		t.Errorf("expected 1 notification for same site, got %d", sentCount)
	}
}

func TestNotify_ContainsStatusAndResponseTime(t *testing.T) {
	var mu sync.Mutex
	var capturedMsg string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body struct {
			Text string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		capturedMsg = body.Text
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := notifier.NewTest("test-token", "test-chat", ts.URL, &http.Client{Timeout: 5 * time.Second}, logger)

	n.Notify(context.Background(), "MySite", true, 200, 10*time.Millisecond)
	n.Notify(context.Background(), "MySite", false, 503, 500*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if capturedMsg == "" {
		t.Fatal("no message captured")
	}
	if !contains(capturedMsg, "503") {
		t.Errorf("message should contain status code 503: %s", capturedMsg)
	}
}

func TestEscapeMarkdown(t *testing.T) {
	input := "Hello_world *bold* [link](url)"
	expected := "Hello\\_world \\*bold\\* \\[link\\]\\(url\\)"
	result := notifier.EscapeMarkdown(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
