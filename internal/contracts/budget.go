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
	if selected.MaxBytes <= 0 || selected.MaxBytes > cfg.MaxBudget {
		return ByteBudget{}, fmt.Errorf(
			"result budget must be between 1 and %d bytes: %d",
			cfg.MaxBudget,
			selected.MaxBytes,
		)
	}
	if selected.Overflow != OverflowStore && selected.Overflow != OverflowReject {
		return ByteBudget{}, fmt.Errorf("result overflow must be %q or %q", OverflowStore, OverflowReject)
	}
	return selected, nil
}

// EnforceBudget retains exact accepted bytes and applies the declared overflow policy.
func EnforceBudget(budget ByteBudget, payload json.RawMessage) (BudgetOutcome, error) {
	if budget.MaxBytes <= 0 {
		return BudgetOutcome{}, fmt.Errorf("result budget must be positive: %d", budget.MaxBytes)
	}
	complete := cloneRaw(payload)
	if len(payload) <= budget.MaxBytes {
		return BudgetOutcome{Payload: complete, Preview: string(payload)}, nil
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
		Payload:    complete,
		Preview:    boundedUTF8(payload, budget.MaxBytes),
		Overflowed: true,
	}, nil
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
