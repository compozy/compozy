package task

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/contracts"
)

func (m *Service) enforceResultBudget(payload json.RawMessage) error {
	if m == nil {
		return fmt.Errorf("task: result budget service is required")
	}
	if _, err := contracts.EnforceBudget(m.resultBudget, payload); err != nil {
		return fmt.Errorf("task: enforce result budget: %w", err)
	}
	return nil
}
