package store

import (
	"context"
	"fmt"
)

// Stats returns the number of indexed documents for the given project key.
func (s *PgvectorStore) Stats(ctx context.Context, projectKey string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM issues_index WHERE project_key=$1", projectKey).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: Stats: %w", err)
	}
	return count, nil
}
