package db

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrateFromFS runs database migrations from an embedded filesystem
func MigrateFromFS(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS) error {
	// Create migrations table if it doesn't exist
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	files, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			migrations = append(migrations, file.Name())
		}
	}
	sort.Strings(migrations)

	// Apply each migration
	for _, migration := range migrations {
		// Check if migration was already applied
		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1", migration).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migration, err)
		}

		if count > 0 {
			continue // Migration already applied
		}

		// Read migration file
		content, err := fs.ReadFile(migrationsFS, migration)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migration, err)
		}

		// Execute migration in a transaction
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration, err)
		}

		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %s: %w", migration, err)
		}

		_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", migration)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", migration, err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration, err)
		}

		fmt.Printf("Applied migration: %s\n", migration)
	}

	return nil
}

// MigrateFromDir runs database migrations from a directory
func MigrateFromDir(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	return MigrateFromFS(ctx, pool, os.DirFS(migrationsDir))
}