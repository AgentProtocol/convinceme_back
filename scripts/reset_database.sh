#!/bin/bash

# Get the database path from environment or use default
DB_PATH=${DB_PATH:-"data/arguments.db"}

# Check if the database exists
if [ -f "$DB_PATH" ]; then
    echo "Removing existing database at $DB_PATH"
    rm "$DB_PATH"
fi

echo "Creating database directory if it doesn't exist..."
mkdir -p $(dirname "$DB_PATH")

# Apply the new schema
echo "Applying new schema to database: $DB_PATH"
sqlite3 "$DB_PATH" < sql/schema_v2.sql

# Check if schema was applied successfully
if [ $? -eq 0 ]; then
    echo "Schema applied successfully!"
    
    # Apply topic migration
    echo "Applying topic migration..."
    sqlite3 "$DB_PATH" < sql/topic_migration.sql
    
    if [ $? -eq 0 ]; then
        echo "Topic migration completed successfully!"
        
        # Count topics in the database
        TOPIC_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM topics;")
        echo "Total topics in database: $TOPIC_COUNT"
        
        # List topic titles
        echo "Available topics:"
        sqlite3 "$DB_PATH" "SELECT id, title FROM topics ORDER BY id;"
    else
        echo "Topic migration failed!"
    fi
else
    echo "Schema application failed!"
fi
