package memory

import (
	"context"
	"fmt"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

// SourceHeaderCount returns the complete count of valid source headers in one scope.
func (s *Store) SourceHeaderCount(ctx context.Context, scope memcontract.Scope) (int, error) {
	headers, err := s.scan(ctx, scope, 0)
	if err != nil {
		return 0, fmt.Errorf("memory: count source headers: %w", err)
	}
	return len(headers), nil
}
