package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/database"
)

func main() {
	// Set up logging
	logger := log.New(os.Stdout, "[Migration] ", log.LstdFlags)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Printf("Warning: Error loading .env file: %v", err)
	}

	// Initialize database
	db, err := database.New("data")
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	logger.Println("Database migrations completed successfully")
	fmt.Println("Database migrations completed successfully")
}
