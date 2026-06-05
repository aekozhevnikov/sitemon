// Package config handles loading and validating configuration from YAML files
// and environment variables. Environment variables take precedence over file values.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the top-level application configuration.
type Config struct {
	CheckInterval time.Duration `yaml:"check_interval"`
	Timeout       time.Duration `yaml:"timeout"`
	Sites         []Site        `yaml:"sites"`
	Telegram      Telegram      `yaml:"telegram"`
	Server        Server        `yaml:"server"`
	Storage       Storage       `yaml:"storage"`
}

// Site represents a single monitored endpoint.
type Site struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	ExpectedStatus int    `yaml:"expected_status"`
}

// Telegram holds the Telegram bot configuration for sending notifications.
type Telegram struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// Server holds the HTTP server configuration for the dashboard.
type Server struct {
	Addr string `yaml:"addr"`
}

// Storage holds the SQLite database configuration.
type Storage struct {
	Path string `yaml:"path"`
}

// Load reads the configuration from the given YAML file path and applies
// environment variable overrides. It validates the resulting configuration
// and returns an error if any required fields are missing or invalid.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	applyEnvOverrides(&cfg)
	setDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides overrides configuration values with environment variables
// when they are set.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SITEMON_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CheckInterval = d
		}
	}
	if v := os.Getenv("SITEMON_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}
	if v := os.Getenv("SITEMON_TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Telegram.BotToken = v
	}
	if v := os.Getenv("SITEMON_TELEGRAM_CHAT_ID"); v != "" {
		cfg.Telegram.ChatID = v
	}
	if v := os.Getenv("SITEMON_SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("SITEMON_STORAGE_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	// SITES env var format: "Name1|URL1|Status1,Name2|URL2|Status2"
	if v := os.Getenv("SITEMON_SITES"); v != "" {
		sites := parseSitesFromEnv(v)
		if len(sites) > 0 {
			cfg.Sites = sites
		}
	}
}

// ParseSitesFromEnv parses a comma-separated list of "name|url|status" site
// definitions from an environment variable value. Exposed for external testing.
func ParseSitesFromEnv(s string) []Site {
	return parseSitesFromEnv(s)
}

// parseSitesFromEnv parses a comma-separated list of "name|url|status" site
// definitions from an environment variable value.
func parseSitesFromEnv(s string) []Site {
	parts := strings.Split(s, ",")
	sites := make([]Site, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.SplitN(p, "|", 3)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		url := strings.TrimSpace(fields[1])
		status := 200
		if len(fields) == 3 {
			if n, err := strconv.Atoi(strings.TrimSpace(fields[2])); err == nil {
				status = n
			}
		}
		sites = append(sites, Site{
			Name:           name,
			URL:            url,
			ExpectedStatus: status,
		})
	}
	return sites
}

// setDefaults applies default values for any zero-valued fields that need them.
func setDefaults(cfg *Config) {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = "./sitemon.db"
	}
}

// validate checks that all required configuration fields are present.
func validate(cfg *Config) error {
	if len(cfg.Sites) == 0 {
		return fmt.Errorf("at least one site must be configured")
	}
	for i, s := range cfg.Sites {
		if s.Name == "" {
			return fmt.Errorf("site %d: name is required", i)
		}
		if s.URL == "" {
			return fmt.Errorf("site %d: url is required", i)
		}
	}
	return nil
}
