package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "convinceme",
	Short: "ConvinceMe - AI Agent Conversation Platform",
	Long: `ConvinceMe is a platform that enables interactive conversations between AI agents.
It provides real-time text and voice interactions with context-aware AI personalities.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is .env)")
}
