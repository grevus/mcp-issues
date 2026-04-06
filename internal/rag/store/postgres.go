package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgvectorStore persists and queries issue embeddings using Postgres + pgvector.
type PgvectorStore struct {
	pool *pgxpool.Pool
}

// New creates a new PgvectorStore connected to the given DSN.
func New(ctx context.Context, dsn string) (*PgvectorStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: New: %w", err)
	}
	return &PgvectorStore{pool: pool}, nil
}

// Close releases the connection pool held by the store.
func (s *PgvectorStore) Close() error {
	s.pool.Close()
	return nil
}
