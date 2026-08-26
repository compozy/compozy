package contracts

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// ResolveBudget applies a per-consumer override without allowing the configured cap to widen.
func ResolveBudget(override *ByteBudget, cfg CallsResultsConfig) (ByteBudget, error) {
	selected := cfg.DefaultBudget
	if override != nil {
		selected = *override
	}
	if cfg.MaxBudget <= 0 {
		return ByteBudget{}, fmt.Errorf("calls.results.max_budget must be positive")
	}
	if err := validateByteBudget(selected, cfg.MaxBudget); err != nil {
		return ByteBudget{}, err
	}
	return selected, nil
}

// EnforceBudget retains exact accepted bytes and applies the declared overflow policy.
func EnforceBudget(budget ByteBudget, payload json.RawMessage) (BudgetOutcome, error) {
	if err := validateByteBudget(budget, 0); err != nil {
		return BudgetOutcome{}, err
	}
	if len(payload) <= budget.MaxBytes {
		return BudgetOutcome{Payload: cloneRaw(payload), Preview: string(payload)}, nil
	}
	if budget.Overflow == OverflowReject {
		return BudgetOutcome{}, newError(
			CodeResultOverBudget,
			FaultChild,
			fmt.Sprintf("result is %d bytes; declared limit is %d bytes", len(payload), budget.MaxBytes),
			nil,
		)
	}
	if budget.Overflow != OverflowStore {
		return BudgetOutcome{}, fmt.Errorf("unknown result overflow mode %q", budget.Overflow)
	}
	return BudgetOutcome{
		Payload:    cloneRaw(payload),
		Preview:    boundedUTF8(payload, budget.MaxBytes),
		Overflowed: true,
	}, nil
}

func validateByteBudget(budget ByteBudget, maximum int) error {
	if budget.MaxBytes <= 0 {
		return fmt.Errorf("result budget must be positive: %d", budget.MaxBytes)
	}
	if maximum > 0 && budget.MaxBytes > maximum {
		return fmt.Errorf("result budget must be between 1 and %d bytes: %d", maximum, budget.MaxBytes)
	}
	if budget.Overflow != OverflowStore && budget.Overflow != OverflowReject {
		return fmt.Errorf("result overflow must be %q or %q", OverflowStore, OverflowReject)
	}
	return nil
}

func boundedUTF8(value []byte, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return string(value)
	}
	value = value[:limit]
	for len(value) > 0 && !utf8.Valid(value) {
		value = value[:len(value)-1]
	}
	return string(value)
}
