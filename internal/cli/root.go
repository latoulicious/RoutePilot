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
    Use:   "routepilot",
    Short: "RoutePilot CLI",
    Long: `RoutePilot CLI manages tenants, feature flags, experiments, and API keys
for the RoutePilot feature flag service.

This tool provides administrative capabilities for bootstrapping and
configuring the service without a web interface.`,
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
