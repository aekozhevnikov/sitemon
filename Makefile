.PHONY: build run test docker-build docker-run lint clean

BINARY_NAME=sitemon
DOCKER_IMAGE=sitemon:latest
GOFLAGS=-ldflags="-s -w"

build:
	go build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/sitemon

run: build
	./bin/$(BINARY_NAME) -config ./configs/config.yaml

test:
	go test -v -race -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker compose up -d

docker-stop:
	docker compose down

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out sitemon.db
