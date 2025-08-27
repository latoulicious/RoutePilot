package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

var (
	databaseURL string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "faas-cli",
	Short: "FaaS (Feature-flags-as-a-Service) CLI tool",
	Long: `FaaS CLI is a command-line tool for managing tenants, feature flags, 
experiments, and API keys in the FaaS feature flag service.

This tool provides administrative capabilities for bootstrapping and 
configuring the service without requiring a web interface.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&databaseURL, "database-url", "", "PostgreSQL database URL (required)")
	rootCmd.MarkPersistentFlagRequired("database-url")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// If database URL is not provided via flag, try environment variable
	if databaseURL == "" {
		if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
			databaseURL = envURL
		}
	}
}
