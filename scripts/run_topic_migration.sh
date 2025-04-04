#!/bin/bash

# Get the database path from environment or use default
DB_PATH=${DB_PATH:-"data/arguments.db"}

# Check if the database exists
if [ ! -f "$DB_PATH" ]; then
    echo "Database file not found at $DB_PATH"
    echo "Creating database directory if it doesn't exist..."
    mkdir -p $(dirname "$DB_PATH")
    echo "Database will be created when migration runs."
fi

# Run the migration
echo "Running topic migration on database: $DB_PATH"
sqlite3 "$DB_PATH" < sql/topic_migration.sql

# Check if migration was successful
if [ $? -eq 0 ]; then
    echo "Migration completed successfully!"
    
    # Count topics in the database
    TOPIC_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM topics;")
    echo "Total topics in database: $TOPIC_COUNT"
    
    # List topic titles
    echo "Available topics:"
    sqlite3 "$DB_PATH" "SELECT id, title FROM topics ORDER BY id;"
else
    echo "Migration failed!"
fi
