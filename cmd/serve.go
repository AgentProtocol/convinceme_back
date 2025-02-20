package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/server"
	"github.com/neo/convinceme_backend/internal/types"
	"github.com/spf13/cobra"
)

var (
	port     int
	certFile string
	keyFile  string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the ConvinceMe server",
	Long: `Start the ConvinceMe server with the specified configuration.
This will initialize the WebSocket server, load AI agents, and begin
accepting connections.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		// Ensure data directory exists
		if err := os.MkdirAll("data", 0755); err != nil {
			fmt.Printf("Error creating data directory: %v\n", err)
			os.Exit(1)
		}

		// Check for .env file
		if _, err := os.Stat(".env"); os.IsNotExist(err) {
			fmt.Println("Warning: .env file not found. Make sure to create it with your OPENAI_API_KEY")
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging
		logger := log.New(os.Stdout, "[ConvinceMe] ", log.LstdFlags|log.Lshortfile)

		// Load environment variables
		if err := godotenv.Load(); err != nil {
			logger.Printf("Warning: Error loading .env file: %v", err)
		}

		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY is not set in the environment variables")
		}

		// Create agent configurations
		agent1Config := agent.AgentConfig{
			Name:        "Socrates",
			Role:        "Ancient philosopher known for concise, pointed questions and brief, impactful insights. Master of the Socratic method, cutting to the heart of matters with minimal words.",
			Voice:       types.VoiceEcho,
			Temperature: 1.7,
			MaxTokens:   100, // Reduced for more concise responses
			TopP:        0.95,
		}

		agent2Config := agent.AgentConfig{
			Name:        "Nova",
			Role:        "Modern futurist and technological philosopher who delivers sharp, precise insights about consciousness and digital existence. Known for clear, impactful statements.",
			Voice:       types.VoiceNova,
			Temperature: 1.7,
			MaxTokens:   100, // Reduced for more concise responses
			TopP:        0.95,
		}

		// Create agents
		agent1, err := agent.NewAgent(apiKey, agent1Config)
		if err != nil {
			return fmt.Errorf("failed to create agent1: %v", err)
		}

		agent2, err := agent.NewAgent(apiKey, agent2Config)
		if err != nil {
			return fmt.Errorf("failed to create agent2: %v", err)
		}

		// Create and start the server
		srv := server.NewServer(map[string]*agent.Agent{
			agent1Config.Name: agent1,
			agent2Config.Name: agent2,
		})

		// Create a context that can be cancelled
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start server in a goroutine
		errChan := make(chan error, 1)
		go func() {
			addr := fmt.Sprintf(":%d", port)
			logger.Printf("Starting HTTP server on %s...", addr)
			err := srv.Run(addr, "", "") // Empty strings for cert/key files since we're not using TLS
			if err != nil {
				errChan <- fmt.Errorf("server error: %v", err)
			}
		}()

		// Wait for either server error or shutdown signal
		select {
		case err := <-errChan:
			return fmt.Errorf("server error: %v", err)
		case sig := <-sigChan:
			logger.Printf("Received signal %v, initiating shutdown...", sig)

			// Create shutdown context with timeout
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
			defer shutdownCancel()

			// Basic cleanup
			cancel() // Cancel the main context
			deadline, _ := shutdownCtx.Deadline()
			logger.Printf("Waiting up to %v for active connections to finish...", deadline.Sub(time.Now()).Round(time.Second))

			// Wait for context to be done
			<-shutdownCtx.Done()
			if err := shutdownCtx.Err(); err != context.DeadlineExceeded {
				logger.Printf("Shutdown completed gracefully")
			} else {
				logger.Printf("Shutdown deadline exceeded, forcing exit")
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Add flags for the serve command
	serveCmd.Flags().IntVarP(&port, "port", "p", 3000, "Port to run the server on")
}
