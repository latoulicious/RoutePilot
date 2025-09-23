package cli

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/spf13/cobra"

    "github.com/latoulicious/RoutePilot/internal/adapters/db"
    "github.com/latoulicious/RoutePilot/internal/domain/flags"
)

var apiKeyCmd = &cobra.Command{
    Use:   "apikey",
    Short: "Manage API keys",
}

var apiKeyIssueCmd = &cobra.Command{
    Use:   "issue [tenant_id] [name]",
    Short: "Issue a new API key",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        tenantStr := args[0]
        name := args[1]
        return issueAPIKey(cmd.Context(), tenantStr, name)
    },
}

func init() {
    rootCmd.AddCommand(apiKeyCmd)
    apiKeyCmd.AddCommand(apiKeyIssueCmd)
}

func issueAPIKey(ctx context.Context, tenantStr, name string) error {
    if databaseURL == "" {
        return fmt.Errorf("database URL is required. Set --database-url or DATABASE_URL")
    }
    tenantID, err := uuid.Parse(tenantStr)
    if err != nil {
        return fmt.Errorf("invalid tenant_id: %w", err)
    }
    if name == "" {
        return fmt.Errorf("name cannot be empty")
    }

    // Connect DB
    pool, err := db.Connect(databaseURL)
    if err != nil {
        return fmt.Errorf("failed to connect to database: %w", err)
    }
    defer pool.Close()

    queries := db.New(pool)
    repo := db.NewAPIKeysRepositoryAdapter(queries)

    // Generate a random 32-byte secret and hex-encode for printable secret
    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil {
        return fmt.Errorf("failed generating secret: %w", err)
    }
    secretPrintable := hex.EncodeToString(raw)

    // Encrypt under KEK
    kek, err := loadKEK()
    if err != nil {
        return err
    }
    enc, err := encryptAESGCM(kek, []byte(secretPrintable))
    if err != nil {
        return fmt.Errorf("encryption failed: %w", err)
    }

    ak := &flags.APIKey{
        ID:           uuid.Nil,
        TenantID:     tenantID,
        Name:         name,
        EncryptedKey: string(enc),
        CreatedAt:    time.Now(),
    }

    if err := repo.CreateAPIKey(ctx, ak); err != nil {
        return err
    }

    if outputJSON {
        return printJSON(map[string]string{
            "api_key_id": ak.ID.String(),
            "api_secret": secretPrintable,
        })
    }
    fmt.Printf("✅ API key issued!\n")
    fmt.Printf("   api_key_id: %s\n", ak.ID.String())
    fmt.Printf("   api_secret: %s\n", secretPrintable)
    fmt.Printf("   note: store this secret securely; it will not be shown again.\n")
    return nil
}
