# sitemon — Site Monitor with Telegram Alerts

A lightweight service monitoring tool written in Go that periodically checks HTTP endpoints, stores results in SQLite, sends Telegram notifications on status changes, and provides a web dashboard.

## Features

- **Periodic HTTP health checks** — Concurrent checks for all configured sites
- **Response time tracking** — Measures and stores response times
- **Telegram notifications** — Alerts when sites go down or recover
- **Web dashboard** — Real-time status dashboard with auto-refresh
- **SQLite storage** — Persistent storage of all check results
- **24-hour uptime percentages** — Calculated from stored history
- **Configurable** — YAML config + environment variable overrides
- **Graceful shutdown** — Clean shutdown on SIGINT/SIGTERM
- **Structured logging** — JSON logging with slog

## Quick Start

### Prerequisites

- Go 1.22 or later
- GNU Make (optional, for Makefile targets)

### Installation

```bash
git clone https://github.com/anthropic/sitemon.git
cd sitemon
go mod download
```

### Configuration

1. Copy the example environment file and fill in your secrets:

```bash
cp .env.example .env
# Edit .env with your Telegram credentials
```

2. Edit `configs/config.yaml` to configure sites and settings.

Environment variables (from `.env` or shell) override YAML values:

```bash
export SITEMON_TELEGRAM_BOT_TOKEN="your_bot_token"
export SITEMON_TELEGRAM_CHAT_ID="your_chat_id"
```

> **Note:** `.env` is git-ignored. Never commit secrets.

### Run

```bash
make run
```

Or build and run directly:

```bash
make build
./bin/sitemon -config ./configs/config.yaml
```

The dashboard will be available at `http://localhost:3000`.

### Run with Docker

```bash
docker compose up -d
```

## Configuration

Configuration is loaded from a YAML file and can be overridden with environment variables.

| YAML Key | Environment Variable | Default | Description |
|---|---|---|---|
| `check_interval` | `SITEMON_CHECK_INTERVAL` | `30s` | How often to check all sites |
| `timeout` | `SITEMON_TIMEOUT` | `10s` | Per-site HTTP timeout |
| `telegram.bot_token` | `SITEMON_TELEGRAM_BOT_TOKEN` | `""` | Telegram bot token |
| `telegram.chat_id` | `SITEMON_TELEGRAM_CHAT_ID` | `""` | Telegram chat ID |
| `server.addr` | `SITEMON_SERVER_ADDR` | `:3000` | Dashboard listen address |
| `storage.path` | `SITEMON_STORAGE_PATH` | `./sitemon.db` | SQLite database path |
| `sites` | `SITEMON_SITES` | `[]` | List of sites to monitor |

The `SITEMON_SITES` environment variable format is:
```
Name1|URL1|Status1,Name2|URL2|Status2
```

## Project Structure

```
sitemon/
├── cmd/sitemon/        # Application entry point
├── internal/
│   ├── checker/        # HTTP health check logic
│   ├── notifier/       # Telegram bot notifications
│   ├── storage/        # SQLite storage layer
│   ├── server/         # HTTP web dashboard
│   └── config/         # Configuration loading
├── web/
│   ├── templates/      # HTML templates
│   └── static/         # CSS styles
├── configs/            # Example configuration
├── Makefile
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## Makefile Targets

| Target | Description |
|---|---|
| `build` | Build the binary |
| `run` | Build and run locally |
| `test` | Run all tests with race detection |
| `docker-build` | Build Docker image |
| `docker-run` | Start with docker-compose |
| `lint` | Run golangci-lint |
| `clean` | Clean build artifacts |

## API

- `GET /` — HTML dashboard
- `GET /api/status` — JSON status of all sites

## Telegram Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and get the bot token
2. Get your chat ID by messaging [@userinfobot](https://t.me/userinfobot)
3. Set the `SITEMON_TELEGRAM_BOT_TOKEN` and `SITEMON_TELEGRAM_CHAT_ID` environment variables

## License

MIT
