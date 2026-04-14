package sqlite

import (
	"context"
	"fmt"
)

// Stats returns the number of indexed documents for the given tenantID and project key.
func (s *SqliteStore) Stats(ctx context.Context, tenantID, projectKey string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT count(*) FROM issues_index WHERE tenant_id=? AND project_key=?",
		tenantID, projectKey,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: Stats: %w", err)
	}
	return count, nil
}
