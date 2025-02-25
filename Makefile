# Project variables
PROJECT_NAME := convinceme_backend
PORT := 8080
DB_PATH := data/arguments.db
SCHEMA_PATH := sql/schema.sql
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
	@echo "Initializing database schema..."
	@sqlite3 $(DB_PATH) < $(SCHEMA_PATH)

# Full setup
.PHONY: setup
setup: dirs init-db ssl tidy
	@echo "Setup completed. You can now run the server."

# Build and run commands
.PHONY: run
run: setup
	@echo "Starting $(PROJECT_NAME) server..."
	@go run cmd/main.go

# One command to rule them all
.PHONY: start
start: kill-server setup
	@echo "Starting fresh instance of $(PROJECT_NAME) server..."
	@go run cmd/main.go

.PHONY: build
build: setup
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
		-d '{"topic": "Are memecoins net negative or positive for the crypto space?"}' | jq '.'

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
	@echo "\nAPI commands:"
	@echo "  make api-check       - Check all arguments and agents"
	@echo "  make api-argument id=1 - Check specific argument by ID"
	@echo "  make api-start-debate  - Start a new debate session"

# Default target
.DEFAULT_GOAL := help 