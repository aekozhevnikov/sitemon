package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/sitemon/internal/config"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

// --- Load: success paths ---

func TestLoad_ValidConfig(t *testing.T) {
	path := writeTempConfig(t, `
check_interval: 15s
timeout: 5s
sites:
  - name: "Test"
    url: "https://example.com"
    expected_status: 200
server:
  addr: ":9090"
storage:
  path: "./test.db"
telegram:
  bot_token: "123456:ABC"
  chat_id: "-100123"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CheckInterval != 15*time.Second {
		t.Errorf("check_interval: got %v, want 15s", cfg.CheckInterval)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("timeout: got %v, want 5s", cfg.Timeout)
	}
	if cfg.Server.Addr != ":9090" {
		t.Errorf("server.addr: got %q, want :9090", cfg.Server.Addr)
	}
	if cfg.Storage.Path != "./test.db" {
		t.Errorf("storage.path: got %q, want ./test.db", cfg.Storage.Path)
	}
	if cfg.Telegram.BotToken != "123456:ABC" {
		t.Errorf("telegram.bot_token: got %q", cfg.Telegram.BotToken)
	}
	if cfg.Telegram.ChatID != "-100123" {
		t.Errorf("telegram.chat_id: got %q", cfg.Telegram.ChatID)
	}
	if len(cfg.Sites) != 1 {
		t.Fatalf("sites count: got %d, want 1", len(cfg.Sites))
	}
	if cfg.Sites[0].Name != "Test" {
		t.Errorf("site name: got %q", cfg.Sites[0].Name)
	}
	if cfg.Sites[0].URL != "https://example.com" {
		t.Errorf("site url: got %q", cfg.Sites[0].URL)
	}
	if cfg.Sites[0].ExpectedStatus != 200 {
		t.Errorf("site expected_status: got %d", cfg.Sites[0].ExpectedStatus)
	}
}

func TestLoad_MultipleSites(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Alpha"
    url: "https://alpha.com"
    expected_status: 200
  - name: "Beta"
    url: "https://beta.com"
    expected_status: 301
  - name: "Gamma"
    url: "https://gamma.com"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sites) != 3 {
		t.Fatalf("sites count: got %d, want 3", len(cfg.Sites))
	}
	if cfg.Sites[1].ExpectedStatus != 301 {
		t.Errorf("beta expected_status: got %d", cfg.Sites[1].ExpectedStatus)
	}
	// Gamma should default to 200 (zero value in YAML).
	if cfg.Sites[2].ExpectedStatus != 0 {
		t.Errorf("gamma expected_status: got %d, want 0", cfg.Sites[2].ExpectedStatus)
	}
}

// --- Load: defaults ---

func TestLoad_Defaults(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Only"
    url: "https://only.com"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("default check_interval: got %v, want 30s", cfg.CheckInterval)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("default timeout: got %v, want 10s", cfg.Timeout)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("default server.addr: got %q, want :8080", cfg.Server.Addr)
	}
	if cfg.Storage.Path != "./sitemon.db" {
		t.Errorf("default storage.path: got %q", cfg.Storage.Path)
	}
}

// --- Load: file errors ---

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !os.IsNotExist(err) {
		// The error wraps the os.IsNotExist error, so check the message.
		if got := err.Error(); !contains(got, "reading config file") {
			t.Errorf("expected 'reading config file' in error, got: %v", got)
		}
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempConfig(t, "this is: not: valid: yaml: [}")
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// --- Load: validation errors ---

func TestLoad_NoSites(t *testing.T) {
	path := writeTempConfig(t, `
check_interval: 10s
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for empty sites, got nil")
	}
}

func TestLoad_SiteMissingName(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: ""
    url: "https://example.com"
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for missing site name, got nil")
	}
}

func TestLoad_SiteMissingURL(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "NoURL"
    url: ""
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for missing site URL, got nil")
	}
}

// --- Environment variable overrides ---

func TestLoad_EnvOverride_CheckInterval(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_CHECK_INTERVAL", "45s")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CheckInterval != 45*time.Second {
		t.Errorf("check_interval: got %v, want 45s", cfg.CheckInterval)
	}
}

func TestLoad_EnvOverride_Timeout(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_TIMEOUT", "20s")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout != 20*time.Second {
		t.Errorf("timeout: got %v, want 20s", cfg.Timeout)
	}
}

func TestLoad_EnvOverride_Telegram(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_TELEGRAM_BOT_TOKEN", "env-token")
	t.Setenv("SITEMON_TELEGRAM_CHAT_ID", "env-chat")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Telegram.BotToken != "env-token" {
		t.Errorf("bot_token: got %q", cfg.Telegram.BotToken)
	}
	if cfg.Telegram.ChatID != "env-chat" {
		t.Errorf("chat_id: got %q", cfg.Telegram.ChatID)
	}
}

func TestLoad_EnvOverride_ServerAddr(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_SERVER_ADDR", ":5050")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Addr != ":5050" {
		t.Errorf("server.addr: got %q", cfg.Server.Addr)
	}
}

func TestLoad_EnvOverride_StoragePath(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_STORAGE_PATH", "/tmp/custom.db")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.Path != "/tmp/custom.db" {
		t.Errorf("storage.path: got %q", cfg.Storage.Path)
	}
}

func TestLoad_EnvOverride_Sites(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Old"
    url: "https://old.com"
`)
	t.Setenv("SITEMON_SITES", "NewSite|https://new.com|301")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sites) != 1 || cfg.Sites[0].Name != "NewSite" {
		t.Errorf("expected one site NewSite, got %+v", cfg.Sites)
	}
	if cfg.Sites[0].ExpectedStatus != 301 {
		t.Errorf("expected status 301, got %d", cfg.Sites[0].ExpectedStatus)
	}
}

func TestLoad_EnvInvalidDuration_Ignored(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_CHECK_INTERVAL", "not-a-duration")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid env value -> falls back to default.
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("check_interval: got %v, want 30s (default)", cfg.CheckInterval)
	}
}

// --- ParseSitesFromEnv ---

func TestParseSitesFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantSite config.Site
	}{
		{
			name:    "single site with status",
			input:   "Alpha|https://alpha.com|200",
			wantLen: 1,
			wantSite: config.Site{Name: "Alpha", URL: "https://alpha.com", ExpectedStatus: 200},
		},
		{
			name:    "single site without status defaults to 200",
			input:   "Beta|https://beta.com",
			wantLen: 1,
			wantSite: config.Site{Name: "Beta", URL: "https://beta.com", ExpectedStatus: 200},
		},
		{
			name:    "multiple sites",
			input:   "A|https://a.com|200,B|https://b.com|301",
			wantLen: 2,
		},
		{
			name:    "spaces are trimmed",
			input:   "  Spaced  |  https://spaced.com  | 404 ",
			wantLen: 1,
			wantSite: config.Site{Name: "Spaced", URL: "https://spaced.com", ExpectedStatus: 404},
		},
		{
			name:    "empty input",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "invalid status falls back to 200",
			input:   "Bad|https://bad.com|abc",
			wantLen: 1,
			wantSite: config.Site{Name: "Bad", URL: "https://bad.com", ExpectedStatus: 200},
		},
		{
			name:    "entry with only name is skipped",
			input:   "OnlyName",
			wantLen: 0,
		},
		{
			name:    "entry with empty name after trim is still parsed",
			input:   "|https://empty-name.com",
			wantLen: 1,
			wantSite: config.Site{Name: "", URL: "https://empty-name.com", ExpectedStatus: 200},
		},
		{
			name:    "trailing comma is ignored",
			input:   "A|https://a.com,",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sites := config.ParseSitesFromEnv(tt.input)
			if len(sites) != tt.wantLen {
				t.Errorf("got %d sites, want %d: %+v", len(sites), tt.wantLen, sites)
				return
			}
			if tt.wantLen > 0 && tt.wantSite != (config.Site{}) {
				if sites[0] != tt.wantSite {
					t.Errorf("site[0] = %+v, want %+v", sites[0], tt.wantSite)
				}
			}
		})
	}
}

// --- setDefaults (indirectly via Load) ---

func TestSetDefaults_AllZero(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Z"
    url: "https://z.com"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("default check_interval: got %v", cfg.CheckInterval)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("default timeout: got %v", cfg.Timeout)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("default server.addr: got %q", cfg.Server.Addr)
	}
	if cfg.Storage.Path != "./sitemon.db" {
		t.Errorf("default storage.path: got %q", cfg.Storage.Path)
	}
}

// --- validate (indirectly via Load) ---

func TestValidate_NoSites(t *testing.T) {
	path := writeTempConfig(t, `
check_interval: 5s
`)
	_, err := config.Load(path)
	if err == nil || !contains(err.Error(), "at least one site") {
		t.Errorf("expected 'at least one site' error, got: %v", err)
	}
}

func TestValidate_EmptySiteName(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: ""
    url: "https://example.com"
`)
	_, err := config.Load(path)
	if err == nil || !contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required' error, got: %v", err)
	}
}

func TestValidate_EmptySiteURL(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "NoURL"
    url: ""
`)
	_, err := config.Load(path)
	if err == nil || !contains(err.Error(), "url is required") {
		t.Errorf("expected 'url is required' error, got: %v", err)
	}
}

// --- Edge cases ---

func TestLoad_InvalidDurationInFile(t *testing.T) {
	path := writeTempConfig(t, `
check_interval: "not-a-duration"
sites:
  - name: "Test"
    url: "https://example.com"
`)
	// YAML will parse "not-a-duration" as string, Go duration parsing will fail.
	// The yaml.v3 library will return an unmarshalling error.
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid duration in YAML")
	}
}

func TestLoad_PartialConfigWithEnvOverride(t *testing.T) {
	path := writeTempConfig(t, `
sites:
  - name: "Test"
    url: "https://example.com"
`)
	t.Setenv("SITEMON_TIMEOUT", "3s")
	t.Setenv("SITEMON_CHECK_INTERVAL", "1m")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Errorf("timeout: got %v", cfg.Timeout)
	}
	if cfg.CheckInterval != 1*time.Minute {
		t.Errorf("check_interval: got %v", cfg.CheckInterval)
	}
	// Defaults for unset fields.
	if cfg.Server.Addr != ":8080" {
		t.Errorf("default server.addr: got %q", cfg.Server.Addr)
	}
}

// --- Helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
