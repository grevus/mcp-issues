package pgvector

import (
	"context"
	"fmt"
)

// Stats returns the number of indexed documents for the given tenantID and project key.
func (s *PgvectorStore) Stats(ctx context.Context, tenantID, projectKey string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM issues_index WHERE tenant_id=$1 AND project_key=$2",
		tenantID, projectKey,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("pgvector: Stats: %w", err)
	}
	return count, nil
}
