.PHONY: build run test test-unit test-integration bench coverage coverage-html docker-build docker-run lint clean

BINARY_NAME=sitemon
DOCKER_IMAGE=sitemon:latest
GOFLAGS=-ldflags="-s -w"

# Unset sitemon env vars so tests/benchmarks are not affected by .env file.
# Tests use their own t.Setenv() for specific overrides.
SITEMON_ENV=unset SITEMON_SERVER_ADDR SITEMON_CHECK_INTERVAL SITEMON_TIMEOUT SITEMON_STORAGE_PATH SITEMON_TELEGRAM_BOT_TOKEN SITEMON_TELEGRAM_CHAT_ID SITEMON_SITES;

build:
	go build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/sitemon

run: build
	./bin/$(BINARY_NAME) -config ./configs/config.yaml

test:
	$(SITEMON_ENV) go test -v -race -coverprofile=coverage.out -coverpkg=./internal/... ./tests/...
	go tool cover -func=coverage.out

test-unit:
	$(SITEMON_ENV) go test -v -race -coverprofile=coverage_unit.out -coverpkg=./internal/... ./tests/unit/...
	go tool cover -func=coverage_unit.out

test-integration:
	$(SITEMON_ENV) go test -v -race -count=1 ./tests/integration/...

bench:
	$(SITEMON_ENV) go test ./tests/benchmarks/... -bench=. -benchmem -benchtime=3s -count=1

coverage:
	$(SITEMON_ENV) go test -race -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/...
	go tool cover -func=coverage.out

coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open file://$(shell pwd)/coverage.html in browser"

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker compose up -d

docker-stop:
	docker compose down

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage_unit.out coverage.html sitemon.db bench_results*.txt
