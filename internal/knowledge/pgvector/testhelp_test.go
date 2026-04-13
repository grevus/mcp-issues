//go:build integration

package pgvector

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupPgvector starts a pgvector/pgvector:pg17 container, runs migrations and
// returns a ready *PgvectorStore along with a cleanup function that closes the
// store and terminates the container.
func setupPgvector(t *testing.T) (*PgvectorStore, func()) {
	t.Helper()

	ctx := context.Background()

	ctr, err := tcpostgres.Run(
		ctx,
		"pgvector/pgvector:pg17",
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	st, err := New(ctx, dsn)
	require.NoError(t, err)

	// Open a *sql.DB via pgx stdlib driver to run goose migrations.
	connCfg := st.pool.Config().ConnConfig.Copy()
	sqlDB := stdlib.OpenDB(*connCfg)

	err = Migrate(ctx, sqlDB)
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	cleanup := func() {
		_ = st.Close()
		_ = ctr.Terminate(ctx)
	}

	return st, cleanup
}
