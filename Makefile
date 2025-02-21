.PHONY: all build run test clean cert deps

# Default target
all: deps cert build run

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate TLS certificates
cert:
	@echo "Generating TLS certificates..."
	@if [ ! -f cert.pem ]; then \
		openssl genpkey -algorithm RSA -out key.pem; \
		openssl req -new -key key.pem -out cert.csr -subj "/CN=localhost"; \
		openssl x509 -req -days 365 -in cert.csr -signkey key.pem -out cert.pem; \
		rm cert.csr; \
	fi

# Build the project
build:
	@echo "Building project..."
	go build -o bin/convinceme main.go

# Run the project
run:
	@echo "Starting ConvinceMe server..."
	@if [ ! -f .env ]; then \
		echo "Warning: .env file not found. Make sure to create it with your OPENAI_API_KEY"; \
	fi
	@mkdir -p data
	./bin/convinceme

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -f bin/convinceme
	rm -f cert.pem key.pem
	rm -rf data/*

# Development mode with hot reload
dev:
	@if ! command -v air > /dev/null; then \
		echo "Installing air for hot reload..."; \
		go install github.com/cosmtrek/air@latest; \
	fi
	air 
