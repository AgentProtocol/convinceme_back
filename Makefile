# Project variables
PROJECT_NAME := convinceme_backend
PORT := 8081
DB_PATH := data/arguments.db
QUERY_PATH := sql/queries.sql

# Directory structure
.PHONY: dirs
dirs:
	@echo "Creating required directories..."
	@mkdir -p data static/hls ssl bin

# SSL certificate generation
.PHONY: ssl
ssl:
	@echo "Generating SSL certificates..."
	@openssl genpkey -algorithm RSA -out key.pem
	@openssl req -new -key key.pem -out cert.csr
	@openssl req -x509 -key key.pem -in cert.csr -out cert.pem -days 365

# Go module management
.PHONY: tidy
tidy:
	@echo "Tidying Go modules..."
	@go mod tidy

# Database initialization
.PHONY: init-db
init-db: dirs
	@echo "Initializing database through migrations..."
	@go run cmd/migrate.go

# Full setup
.PHONY: setup
setup: dirs init-db tidy
	@echo "Setup completed. You can now run the server."

# Build and run commands
.PHONY: run
run: dirs tidy
	@echo "Starting $(PROJECT_NAME) server..."
	@go run cmd/main.go

# One command to rule them all
.PHONY: start
start: kill-server dirs tidy
	@echo "Starting fresh instance of $(PROJECT_NAME) server..."
	@go run cmd/main.go

.PHONY: build
build: dirs tidy
	@echo "Building $(PROJECT_NAME)..."
	@go build -o bin/$(PROJECT_NAME) cmd/main.go

# Clean up
.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -f key.pem cert.pem cert.csr
	@rm -rf bin/*
	@rm -f $(DB_PATH)

# Database commands
.PHONY: db-check
db-check:
	@echo "Checking database with SQL queries..."
	@sqlite3 $(DB_PATH) ".read $(QUERY_PATH)"

.PHONY: db-shell
db-shell:
	@echo "Opening SQLite shell..."
	@sqlite3 $(DB_PATH)

.PHONY: reset-db
reset-db:
	@echo "Resetting database..."
	@rm -f $(DB_PATH)
	@mkdir -p data
	@go run cmd/migrate.go
	@echo "Database reset complete. Use 'make run' to start the server."

.PHONY: reset-and-start
reset-and-start: kill-server reset-db
	@echo "Starting server with fresh database..."
	@go run cmd/main.go

.PHONY: migrate
migrate:
	@echo "Running database migrations..."
	@go run cmd/migrate.go

.PHONY: migrate-only
migrate-only:
	@echo "Running database migrations only (without starting server)..."
	@go run cmd/migrate.go

.PHONY: create-test-debates
create-test-debates:
	@echo "Creating test debates..."
	@go run cmd/create_test_debates.go

# Testing commands
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./...

.PHONY: test-verbose
test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

.PHONY: test-auth
test-auth:
	@echo "Running authentication tests..."
	@go test -v ./internal/auth

.PHONY: test-database
test-database:
	@echo "Running database tests..."
	@go test -v ./internal/database

.PHONY: test-server
test-server:
	@echo "Running server tests..."
	@go test -v ./internal/server

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

.PHONY: test-short
test-short:
	@echo "Running short tests..."
	@go test -short ./...

# API test commands
.PHONY: api-check
api-check:
	@echo "\nChecking all arguments:"
	@curl -s http://localhost:$(PORT)/api/arguments | jq '.'
	@echo "\nChecking available agents:"
	@curl -s http://localhost:$(PORT)/api/agents | jq '.'

.PHONY: api-argument
api-argument:
	@if [ -z "$(id)" ]; then \
		echo "Usage: make api-argument id=<argument_id>"; \
		exit 1; \
	fi
	@echo "\nChecking argument with ID $(id):"
	@curl -s http://localhost:$(PORT)/api/arguments/$(id) | jq '.'

.PHONY: api-start-debate
api-start-debate:
	@echo "\nStarting new debate session:"
	@curl -s -X POST http://localhost:$(PORT)/api/conversation/start \
		-H "Content-Type: application/json" \
		-d '{"topic": "Who's the GOAT of football: Messi or Ronaldo?"}' | jq '.'

# Topic API endpoints
.PHONY: api-topics
api-topics:
	@echo "\nListing all topics:"
	@curl -s http://localhost:$(PORT)/api/topics | jq '.'

.PHONY: api-topics-category
api-topics-category:
	@if [ -z "$(category)" ]; then \
		echo "Usage: make api-topics-category category=<category_name>"; \
		exit 1; \
	fi
	@echo "\nListing topics in category $(category):"
	@curl -s http://localhost:$(PORT)/api/topics/category/$(category) | jq '.'

.PHONY: api-topic
api-topic:
	@if [ -z "$(id)" ]; then \
		echo "Usage: make api-topic id=<topic_id>"; \
		exit 1; \
	fi
	@echo "\nGetting topic with ID $(id):"
	@curl -s http://localhost:$(PORT)/api/topics/$(id) | jq '.'

# Debate API endpoints
.PHONY: api-debates
api-debates:
	@echo "\nListing all debates:"
	@curl -s http://localhost:$(PORT)/api/debates | jq '.'

.PHONY: api-debate
api-debate:
	@if [ -z "$(id)" ]; then \
		echo "Usage: make api-debate id=<debate_id>"; \
		exit 1; \
	fi
	@echo "\nGetting debate with ID $(id):"
	@curl -s http://localhost:$(PORT)/api/debates/$(id) | jq '.'

.PHONY: api-create-debate
api-create-debate:
	@if [ -z "$(topic_id)" ]; then \
		echo "Usage: make api-create-debate topic_id=<topic_id>"; \
		exit 1; \
	fi
	@echo "\nCreating debate from topic $(topic_id):"
	@curl -s -X POST http://localhost:$(PORT)/api/debates \
		-H "Content-Type: application/json" \
		-d '{"topic_id": $(topic_id)}' | jq '.'

# Kill server if port is in use
.PHONY: kill-server
kill-server:
	@echo "Killing process on port $(PORT)..."
	@lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || echo "No process running on port $(PORT)"

# Help command
.PHONY: help
help:
	@echo "Setup commands:"
	@echo "  make setup            - Full setup (dirs, database, SSL, tidy)"
	@echo "  make dirs            - Create required directories"
	@echo "  make ssl             - Generate SSL certificates"
	@echo "  make init-db         - Initialize database"
	@echo "  make tidy            - Tidy Go modules"
	@echo "  make clean           - Clean up generated files"
	@echo "\nServer commands:"
	@echo "  make start           - Kill existing server and start fresh (recommended)"
	@echo "  make run             - Start the server"
	@echo "  make build           - Build the project"
	@echo "  make kill-server     - Kill process using port $(PORT)"
	@echo "\nDatabase commands:"
	@echo "  make db-check        - Check database with SQL queries"
	@echo "  make db-shell        - Open SQLite shell"
	@echo "  make reset-db        - Reset database (remove and recreate) - DOES NOT START SERVER"
	@echo "  make reset-and-start - Reset database and start server"
	@echo "  make migrate         - Run database migrations"
	@echo "  make create-test-debates - Create test debates for development"
	@echo "\nTesting commands:"
	@echo "  make test            - Run all tests"
	@echo "  make test-verbose    - Run all tests with verbose output"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo "  make test-auth       - Run authentication tests only"
	@echo "  make test-database   - Run database tests only"
	@echo "  make test-server     - Run server tests only"
	@echo "  make test-race       - Run tests with race detector"
	@echo "  make test-short      - Run short tests only"
	@echo "\nAPI commands:"
	@echo "  make api-check       - Check all arguments and agents"
	@echo "  make api-argument id=1 - Check specific argument by ID"
	@echo "  make api-start-debate  - Start a new debate session"
	@echo "  make api-topics      - List all topics"
	@echo "  make api-topics-category category=crypto - List topics by category"
	@echo "  make api-topic id=1  - Get specific topic"
	@echo "  make api-debates     - List all debates"
	@echo "  make api-debate id=abc123 - Get specific debate"
	@echo "  make api-create-debate topic_id=1 - Create debate from topic"

# Default target
.DEFAULT_GOAL := help
