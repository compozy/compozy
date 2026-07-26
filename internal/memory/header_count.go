package memory

import (
	"fmt"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// SourceHeaderCount returns the complete count of valid source headers in one scope.
func (s *Store) SourceHeaderCount(scope memcontract.Scope) (int, error) {
	headers, err := s.scan(scope, 0)
	if err != nil {
		return 0, fmt.Errorf("memory: count source headers: %w", err)
	}
	return len(headers), nil
}
