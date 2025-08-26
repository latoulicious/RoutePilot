package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository wraps the generated queries with additional functionality
type Repository struct {
	*Queries
	pool *pgxpool.Pool
}

// NewRepository creates a new repository instance
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		Queries: New(pool),
		pool:    pool,
	}
}

// WithTx executes a function within a database transaction
func (r *Repository) WithTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := r.Queries.WithTx(tx)
	if err := fn(qtx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// BeginTx starts a new transaction and returns the transaction queries
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, *Queries, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	return tx, r.Queries.WithTx(tx), nil
}