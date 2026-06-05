// Package notifier sends Telegram notifications when monitored sites transition
// between up and down states. It uses the Telegram Bot HTTP API.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/sitemon/internal/config"
)

const defaultTelegramAPI = "https://api.telegram.org"

// State tracks the last known up/down status of a site to detect transitions.
// Exported for external testing.
type State struct {
	WasUp   bool
	Checked bool // false on first notification to avoid spurious alerts
}

// state is the internal type with unexported fields for use within the package.
type state struct {
	wasUp   bool
	checked bool
}

// Notifier sends Telegram messages when site availability changes.
type Notifier struct {
	botToken   string
	chatID     string
	apiBaseURL string
	client     *http.Client
	logger     *slog.Logger
	mu         sync.Mutex
	prevStates map[string]*state
}

// New creates a new Notifier. If botToken or chatID is empty, the notifier
// will be a no-op (useful for development or when Telegram is not configured).
func New(tg config.Telegram, logger *slog.Logger, transport *http.Transport) *Notifier {
	if transport == nil {
		transport = &http.Transport{
			Proxy: nil,
		}
	}
	return &Notifier{
		botToken:   tg.BotToken,
		chatID:     tg.ChatID,
		apiBaseURL: defaultTelegramAPI,
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		logger:     logger,
		prevStates: make(map[string]*state),
	}
}

// NewTest creates a Notifier with custom fields for testing.
// Exposed for external testing only.
func NewTest(botToken, chatID, apiBaseURL string, client *http.Client, logger *slog.Logger) *Notifier {
	return &Notifier{
		botToken:   botToken,
		chatID:     chatID,
		apiBaseURL: apiBaseURL,
		client:     client,
		logger:     logger,
		prevStates: make(map[string]*state),
	}
}

// Enabled returns true if the notifier has valid Telegram credentials.
func (n *Notifier) Enabled() bool {
	return n.botToken != "" && n.chatID != ""
}

// Notify processes a health check result and sends a Telegram notification
// if the site transitioned from up to down or from down to up.
func (n *Notifier) Notify(ctx context.Context, siteName string, isUp bool, statusCode int, responseTime time.Duration) {
	if !n.Enabled() {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	s, exists := n.prevStates[siteName]
	if !exists {
		// First time seeing this site -- record state but don't notify.
		n.prevStates[siteName] = &state{wasUp: isUp, checked: true}
		return
	}

	if s.wasUp == isUp {
		// No state change.
		return
	}

	// State changed -- send notification.
	s.wasUp = isUp
	s.checked = true

	var msg string
	if isUp {
		msg = fmt.Sprintf(
			"SITE RECOVERED: %s is back up! (status: %d, response: %s)",
			siteName, statusCode, responseTime,
		)
	} else {
		msg = fmt.Sprintf(
			"SITE DOWN: %s is unreachable! (status: %d, response: %s)",
			siteName, statusCode, responseTime,
		)
	}

	if err := n.sendMessage(ctx, msg); err != nil {
		n.logger.Error("failed to send Telegram notification",
			"site", siteName,
			"error", err,
		)
	} else {
		n.logger.Info("Telegram notification sent",
			"site", siteName,
			"up", isUp,
		)
	}
}

// sendMessage sends a text message to the configured Telegram chat.
func (n *Notifier) sendMessage(ctx context.Context, text string) error {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", n.apiBaseURL, n.botToken)

	payload := map[string]string{
		"chat_id": n.chatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	// Basic validation: check the "ok" field in the response.
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}

	return nil
}

// EscapeMarkdown escapes special characters for Telegram MarkdownV2 format.
// Exposed for external testing.
func EscapeMarkdown(s string) string {
	return escapeMarkdown(s)
}

// escapeMarkdown escapes special characters for Telegram MarkdownV2 format.
// Not used in the default plain-text messages, but available if needed.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}
