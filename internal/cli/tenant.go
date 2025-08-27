package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/latoulicious/RoutePilot/internal/adapters/db"
    "github.com/latoulicious/RoutePilot/internal/domain/flags"
)

// tenantCmd represents the tenant command
var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenants",
Long:  `Create and manage tenants in the RoutePilot system.`,
}

// tenantCreateCmd represents the tenant create command
var tenantCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new tenant",
	Long: `Create a new tenant with the specified name.
	
The tenant name should be descriptive and unique within your organization.
A unique tenant ID will be automatically generated.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return createTenant(cmd.Context(), args[0])
	},
}

func init() {
	rootCmd.AddCommand(tenantCmd)
	tenantCmd.AddCommand(tenantCreateCmd)
}

func createTenant(ctx context.Context, name string) error {
	// Validate database URL is provided
	if databaseURL == "" {
		return fmt.Errorf("database URL is required. Set --database-url flag or DATABASE_URL environment variable")
	}

	// Validate tenant name
	if name == "" {
		return fmt.Errorf("tenant name cannot be empty")
	}

	if len(name) > 100 {
		return fmt.Errorf("tenant name cannot exceed 100 characters")
	}

	// Connect to database
	pool, err := db.Connect(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Create queries instance
	queries := db.New(pool)

	// Create tenant repository adapter
	tenantRepo := db.NewTenantRepositoryAdapter(queries)

	// Create new tenant
	tenant := &flags.Tenant{
		ID:        uuid.New(),
		Name:      name,
		CreatedAt: time.Now(),
	}

	// Save tenant to database
	if err := tenantRepo.CreateTenant(ctx, tenant); err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

    // Output
    if outputJSON {
        return printJSON(map[string]interface{}{
            "tenant_id": tenant.ID,
            "name":      tenant.Name,
            "created":   tenant.CreatedAt.Format(time.RFC3339),
        })
    }
    fmt.Printf("✅ Tenant created successfully!\n")
    fmt.Printf("   ID: %s\n", tenant.ID)
    fmt.Printf("   Name: %s\n", tenant.Name)
    fmt.Printf("   Created: %s\n", tenant.CreatedAt.Format(time.RFC3339))

	return nil
}
