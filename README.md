# sitemon — Site Monitor with Telegram Alerts

A lightweight service monitoring tool written in Go that periodically checks HTTP endpoints, stores results in SQLite, sends Telegram notifications on status changes, and provides a web dashboard.

## Features

- **Periodic HTTP health checks** — Concurrent checks for all configured sites
- **Response time tracking** — Measures and stores response times
- **Telegram notifications** — Alerts when sites go down or recover
- **Web dashboard** — Real-time status dashboard with auto-refresh
- **SQLite storage with WAL mode** — Persistent storage with concurrent read/write
- **24-hour uptime percentages** — Calculated from stored history
- **Configurable** — YAML config + environment variable overrides
- **Graceful shutdown** — Clean shutdown on SIGINT/SIGTERM with WaitGroup
- **Structured logging** — Text logging with slog
- **TTL-cached API** — 5-second cache for `/api/status` to reduce DB load
- **Shared HTTP transport** — Connection pooling between checker and notifier

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

The `SITES` environment variable format is:
```
Name1|URL1|Status1,Name2|URL2|Status2
```

## Project Structure

```
sitemon/
├── cmd/sitemon/              # Application entry point
├── internal/
│   ├── checker/              # HTTP health check logic
│   ├── notifier/             # Telegram bot notifications
│   ├── storage/              # SQLite storage layer (WAL mode, connection pool)
│   ├── server/               # HTTP web dashboard (TTL-cached API)
│   └── config/               # Configuration loading + validation
├── web/
│   ├── templates/            # HTML templates (embedded)
│   └── static/               # CSS styles (embedded)
├── tests/
│   ├── unit/                 # Unit tests per package
│   ├── integration/          # End-to-end integration tests
│   └── benchmarks/           # Performance benchmarks
├── configs/
│   ├── config.yaml           # Main configuration (120 sites)
│   └── config.yaml.example   # Example configuration
├── .idea/runConfigurations/  # IntelliJ IDEA run configurations
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
| `test-unit` | Run unit tests only |
| `test-integration` | Run integration tests only |
| `bench` | Run all benchmarks |
| `docker-build` | Build Docker image |
| `docker-run` | Start with docker-compose |
| `lint` | Run golangci-lint |
| `clean` | Clean build artifacts |

## Benchmarks

Run benchmarks to measure performance:

```bash
# All benchmarks
make bench

# Specific benchmark suites
go test ./tests/benchmarks/... -bench=BenchmarkCheckAll -benchmem
go test ./tests/benchmarks/... -bench=BenchmarkSave -benchmem
go test ./tests/benchmarks/... -bench=BenchmarkHandleAPIStatus -benchmem
go test ./tests/benchmarks/... -bench=BenchmarkHTTP -benchmem
```

### Benchmark Results (Apple M2)

| Benchmark | Time (ms) | Memory | Allocs |
|---|-----------|---|---|
| `CheckAll_10 sites` | 6,300     | 114 KB | 1030 |
| `CheckAll_100 sites` | 79,000    | 1.3 MB | 9428 |
| `CheckAll_500 sites` | 404,000   | 5.8 MB | 47351 |
| `APIStatus cached (10 sites)` | 5.8       | 8.7 KB | 30 |
| `APIStatus cached (100 sites)` | 43        | 30 KB | 120 |
| `SaveCheckResult (single)` | 39        | 666 B | 9 |
| `GetSiteStatuses (100 records)` | 338       | 15 KB | 361 |
| `GetSiteStatuses (10K records)` | 24,000    | 69 KB | 1683 |
| `HTTP NewTransport/request` | 172       | 19 KB | 129 |
| `HTTP SharedTransport` | 10.5      | 4.7 KB | 53 |

### Key Optimizations

| Optimization | Before            | After             | Improvement |
|---|-------------------|-------------------|---|
| **Shared HTTP Transport** | 172 ms/req        | 10.5 ms/req       | **16x** |
| **TTL-cache for API** | SQL query ~2-5ms  | Cache hit ~5-43ms | **50-500x** |
| **SQLite WAL mode** | fsync per write   | Batched fsync     | **5-10x** |
| **Buffered channel** | Blocking consumer | Non-blocking      | At 100+ sites |
| **Connection pool (10)** | 1 connection      | 10 concurrent     | Parallel writes |

## API

- `GET /` — HTML dashboard
- `GET /api/status` — JSON status of all sites (5s TTL cache)

## Telegram Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and get the bot token
2. Get your chat ID by messaging [@userinfobot](https://t.me/userinfobot)
3. Set the `SITEMON_TELEGRAM_BOT_TOKEN` and `SITEMON_TELEGRAM_CHAT_ID` environment variables

## License

MIT
