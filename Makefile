.PHONY: build test clean generate migrate-up migrate-down

# Build all binaries
build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/cli ./cmd/cli

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Generate code using sqlc
generate:
	sqlc generate

# Build and test
all: generate build test

# Development database (requires Docker)
db-up:
docker run --name routepilot-postgres -e POSTGRES_PASSWORD=password -e POSTGRES_DB=routepilot -p 5432:5432 -d postgres:15

db-down:
docker stop routepilot-postgres || true
docker rm routepilot-postgres || true

# Install development dependencies
deps:
	go mod download
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
