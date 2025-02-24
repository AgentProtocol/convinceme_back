# Argument Scoring System

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

# API testing
make api-check                # List all arguments and agents
make api-argument id=1        # Get specific argument
make api-start-debate        # Start new debate session
```

## API Routes

### WebSocket
- `GET /ws/conversation` - Real-time debate connection

### Arguments
- `GET /api/arguments` - Get last 100 arguments with scores
- `GET /api/arguments/:id` - Get specific argument by ID

### Agents
- `GET /api/agents` - List available debate experts
- `POST /api/conversation/start` - Start new debate session

### Audio
- `GET /api/audio/:id` - Stream generated audio response
- `POST /api/stt` - Speech-to-text conversion

## Database Queries

Check arguments and scores using SQLite:
```sql
# View all arguments with scores
sqlite3 data/arguments.db ".read sql/queries.sql"

# Direct database queries
sqlite3 data/arguments.db
```

Available SQL queries (in `sql/queries.sql`):
- All arguments with scores (formatted)
- Full explanation for specific argument
- Average scores by topic

## Environment Setup
```bash
# Required environment variables
OPENAI_API_KEY=your_key_here

# Optional
USE_HTTPS=false  # Enable for HTTPS
```
