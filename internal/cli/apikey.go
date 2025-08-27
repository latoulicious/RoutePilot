package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

	// Generate API key components
	keyID, secret, secretHash, encryptedSecret, err := generateAPIKeyComponents()
	if err != nil {
		return fmt.Errorf("failed to generate API key components: %w", err)
	}

	ak := &flags.APIKey{
		ID:           uuid.Nil, // Will be set by database
		TenantID:     tenantID,
		Name:         name,
		KeyID:        keyID,
		SecretHash:   secretHash,
		EncryptedKey: encryptedSecret,
		CreatedAt:    time.Now(),
	}

	if err := repo.CreateAPIKey(ctx, ak); err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	if outputJSON {
		return printJSON(map[string]string{
			"api_key_id": ak.ID.String(),
			"key_id":     keyID,
			"api_secret": secret,
		})
	}
	fmt.Printf("✅ API key issued!\n")
	fmt.Printf("   api_key_id: %s\n", ak.ID.String())
	fmt.Printf("   key_id: %s\n", keyID)
	fmt.Printf("   api_secret: %s\n", secret)
	fmt.Printf("   ⚠️  IMPORTANT: Store this secret securely; it will not be shown again.\n")
	return nil
}

// generateAPIKeyComponents generates all the components needed for an API key
func generateAPIKeyComponents() (keyID, secret, secretHash, encryptedSecret string, err error) {
	// Generate a unique key ID (8 bytes = 16 hex chars)
	keyIDBytes := make([]byte, 8)
	if _, err = rand.Read(keyIDBytes); err != nil {
		return "", "", "", "", fmt.Errorf("failed generating key ID: %w", err)
	}
	keyID = hex.EncodeToString(keyIDBytes)

	// Generate a random 32-byte secret and hex-encode for printable secret
	secretBytes := make([]byte, 32)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", "", "", fmt.Errorf("failed generating secret: %w", err)
	}
	secret = hex.EncodeToString(secretBytes)

	// Create hash of the secret for verification
	hash := sha256.Sum256([]byte(secret))
	secretHash = hex.EncodeToString(hash[:])

	// Encrypt the secret under KEK
	kek, err := loadKEK()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to load KEK: %w", err)
	}

	encryptedBytes, err := encryptAESGCM(kek, []byte(secret))
	if err != nil {
		return "", "", "", "", fmt.Errorf("encryption failed: %w", err)
	}
	encryptedSecret = string(encryptedBytes)

	return keyID, secret, secretHash, encryptedSecret, nil
}
