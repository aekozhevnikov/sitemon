# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go module files first for better caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy source code.
COPY . .

# Build the binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sitemon ./cmd/sitemon

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

# Create non-root user.
RUN adduser -D -g '' sitemon

WORKDIR /app

# Copy the binary from builder.
COPY --from=builder /sitemon /usr/local/bin/sitemon
COPY configs/config.yaml.example /app/configs/config.yaml

# Switch to non-root user.
USER sitemon

EXPOSE 8080

ENTRYPOINT ["sitemon"]
CMD ["-config", "/app/configs/config.yaml"]
