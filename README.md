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

## Recent Improvements

- **Database Migration System**: Replaced manual schema creation with versioned migrations
- **Pagination and Filtering**: Added support for paginated API responses and filtering options
- **Authentication System**: Implemented JWT-based authentication with user management
- **Testing Framework**: Added comprehensive tests for core components
- **Performance Optimizations**: Implemented connection pooling and improved error handling
- **Code Organization**: Improved project structure and removed redundant scripts directory
- **Documentation**: Enhanced README and added code comments

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
# Run migrations (will start the server afterward)
make migrate

# Run migrations only (without starting the server)
make migrate-only

# Reset database and run migrations
make reset-db
```

### How Migrations Work

1. Migration files are stored in the `migrations/` directory
2. Files are named with a numeric prefix (e.g., `001_initial_schema.sql`, `002_add_topics_table.sql`)
3. Migrations are applied in sequential order based on their numeric prefix
4. Each migration is only applied once, and the system keeps track of which migrations have been applied
5. When you run `make migrate`, the system checks which migrations have already been applied and only runs the new ones

### Adding New Migrations

To add a new database change:

1. Create a new SQL file in the `migrations/` directory
2. Name it with the next sequential number (e.g., if the last migration is `007_feedback.sql`, name yours `008_your_migration.sql`)
3. Include clear SQL statements with comments explaining the changes
4. Run `make migrate` to apply your new migration

> **Important**: Never modify existing migration files after they've been applied to a database. Instead, create a new migration file to make additional changes.

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

## Project Structure

```
├── cmd/                  # Command-line applications
│   ├── main.go          # Main application entry point
│   └── create_test_debates.go  # Utility to create test debates
├── internal/            # Internal packages
│   ├── agent/           # AI agent implementation
│   ├── audio/           # Audio processing
│   ├── auth/            # Authentication
│   ├── conversation/    # Conversation management
│   ├── database/        # Database access and models
│   ├── player/          # Player management
│   ├── scoring/         # Argument scoring
│   ├── server/          # HTTP server and API handlers
│   ├── tools/           # Utility tools
│   └── types/           # Common type definitions
├── migrations/          # Database migration files
│   ├── 001_initial_schema.sql  # Base tables (arguments, scores, debates)
│   ├── 002_add_topics_table.sql # Topics table for pre-generated debates
│   ├── 003_seed_topics.sql      # Initial topic data
│   ├── 004_add_users_table.sql  # User authentication
│   ├── 005_enhance_users_table.sql # User roles and security
│   ├── 006_invitation_codes.sql # Invitation system
│   ├── 007_feedback.sql        # User feedback system
│   └── 008_add_missing_indexes.sql # Performance optimization
├── sql/                 # SQL utilities
│   └── queries.sql       # Debugging and inspection queries
├── test.html            # Test interface for development
└── Makefile             # Build and development commands
```

## Development Workflow

1. **Setup Environment**:

   ```bash
   # Clone the repository
   git clone https://github.com/AgentProtocol/convinceme_back.git
   cd convinceme_back

   # Set up environment variables
   cp .env.example .env
   # Edit .env with your API keys

   # Install dependencies
   make setup
   ```

2. **Database Setup**:

   ```bash
   # Run migrations to set up the database
   make migrate

   # Create test debates (optional)
   make create-test-debates
   ```

3. **Run the Server**:

   ```bash
   make run
   ```

4. **Testing**:

   ```bash
   # Run all tests
   make test

   # Run specific tests
   go test ./internal/server -v
   ```

5. **API Testing**:

   ```bash
   # List all topics
   make api-topics

   # Create a debate from a topic
   make api-create-debate topic_id=1
   ```

## Environment Setup

```bash
# Required environment variables
OPENAI_API_KEY=your_key_here

# Optional
USE_HTTPS=false  # Enable for HTTPS
JWT_SECRET=your_secret_key  # Secret for JWT authentication
PORT=8081        # Server port (default: 8080)
```
