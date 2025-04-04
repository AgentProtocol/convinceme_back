# ConvinceMe Backend

A debate platform that allows users to create and participate in AI-powered debates on various topics.

## Features

- **Multi-Debate Support**: Host multiple concurrent debates
- **Pre-Generated Topics**: Choose from a variety of debate topics across different categories
- **Real-Time Interaction**: WebSocket-based real-time communication
- **Argument Scoring**: AI-powered scoring of debate arguments
- **User Authentication**: JWT-based authentication system
- **Database Migrations**: Proper migration system for database schema changes
- **API Pagination**: Paginated API responses for better performance
- **Filtering & Sorting**: Filter and sort topics and debates

## Quick Start

```bash
# One command setup and run (recommended)
make start

# Or individual steps
make setup  # First time setup
make run    # Start server
```

## Essential Commands

```bash
# Server management
make start           # Kill existing server and start fresh
make kill-server     # Stop server on port 8080
make clean           # Clean all generated files

# Database operations
make db-check        # View all arguments and scores
make db-shell        # Open SQLite shell
make reset-db        # Reset database (remove and recreate)
make migrate         # Run database migrations

# Testing
make test            # Run all tests
make test-verbose    # Run tests with verbose output
make test-coverage   # Run tests with coverage report

# API testing
make api-check                # List all arguments and agents
make api-argument id=1        # Get specific argument
make api-start-debate        # Start new debate session
make api-topics              # List all topics
make api-topics-category category=crypto  # List topics by category
make api-topic id=1          # Get specific topic
make api-debates             # List all debates
make api-debate id=abc123    # Get specific debate
make api-create-debate topic_id=1  # Create debate from topic
```

## API Routes

### Authentication
- `POST /api/auth/register` - Register a new user
- `POST /api/auth/login` - Login
- `GET /api/auth/me` - Get current user (requires authentication)
- `PUT /api/auth/me` - Update current user (requires authentication)
- `POST /api/auth/change-password` - Change password (requires authentication)
- `DELETE /api/auth/me` - Delete current user (requires authentication)

### WebSocket
- `GET /ws/debate/:id` - Real-time debate connection

### Arguments
- `GET /api/arguments` - Get last 100 arguments with scores
- `GET /api/arguments/:id` - Get specific argument by ID

### Topics
- `GET /api/topics` - List all topics (with pagination and filtering)
- `GET /api/topics/category/:category` - List topics by category
- `GET /api/topics/:id` - Get specific topic details

### Debates
- `GET /api/debates` - List all debates (with pagination and filtering)
- `GET /api/debates/:id` - Get specific debate details
- `POST /api/debates` - Create a new debate from a topic

### Agents
- `GET /api/agents` - List available debate experts

### Audio
- `GET /api/audio/:id` - Stream generated audio response
- `POST /api/stt` - Speech-to-text conversion

## Database Migrations

The system uses a proper migration system to manage database schema changes:

```bash
# Run migrations
make migrate

# Reset database and run migrations
make reset-db
```

Migration files are stored in the `migrations` directory and are applied in order based on their numeric prefix.

## Testing

The project includes comprehensive tests for various components:

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage report
make test-coverage
```

## Environment Setup

```bash
# Required environment variables
OPENAI_API_KEY=your_key_here

# Optional
USE_HTTPS=false  # Enable for HTTPS
JWT_SECRET=your_secret_key  # Secret for JWT authentication
```
