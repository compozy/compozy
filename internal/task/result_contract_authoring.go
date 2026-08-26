package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

func (m *Service) applyNewTaskExpectation(task *Task, spec CreateTask) (*contracts.Contract, error) {
	contract, budget, err := m.prepareTaskExpectation(spec.Expect, spec.ResultBudget)
	if err != nil {
		return nil, err
	}
	if contract != nil {
		task.ExpectDigest = contract.Digest
	}
	task.ResultBudget = budget
	return contract, nil
}

func (m *Service) applyTaskExpectationPatch(
	task *Task,
	patch Patch,
	changedFields []string,
) ([]string, *contracts.Contract, error) {
	if patch.Expect == nil {
		return changedFields, nil, nil
	}
	contract, budget, err := m.prepareTaskExpectation(*patch.Expect, patch.ResultBudget)
	if err != nil {
		return nil, nil, err
	}
	digest := ""
	if contract != nil {
		digest = contract.Digest
	}
	if task.ExpectDigest != digest {
		task.ExpectDigest = digest
		changedFields = append(changedFields, TaskFieldExpectDigest)
	}
	if !sameResultBudget(task.ResultBudget, budget) {
		task.ResultBudget = cloneResultBudget(budget)
		changedFields = append(changedFields, TaskFieldResultBudget)
	}
	return changedFields, contract, nil
}

func (m *Service) prepareTaskExpectation(
	expect json.RawMessage,
	override *contracts.ByteBudget,
) (*contracts.Contract, *contracts.ByteBudget, error) {
	if len(bytes.TrimSpace(expect)) == 0 {
		if override != nil {
			return nil, nil, fmt.Errorf("%w: result_budget requires expect", ErrValidation)
		}
		return nil, nil, nil
	}
	budget := m.resultBudgetConfig.DefaultBudget
	if override != nil {
		if override.MaxBytes != 0 {
			budget.MaxBytes = override.MaxBytes
		}
		if strings.TrimSpace(string(override.Overflow)) != "" {
			budget.Overflow = override.Overflow
		}
	}
	resolved, err := contracts.ResolveBudget(&budget, m.resultBudgetConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: result_budget: %w", ErrValidation, err)
	}
	contract, err := contracts.Prepare(expect)
	if err != nil {
		return nil, nil, fmt.Errorf("task: prepare result contract: %w", err)
	}
	return &contract, &resolved, nil
}

func cloneResultBudget(budget *contracts.ByteBudget) *contracts.ByteBudget {
	if budget == nil {
		return nil
	}
	cloned := *budget
	return &cloned
}

func sameResultBudget(left, right *contracts.ByteBudget) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func validatePersistedTaskExpectation(task Task) error {
	return validatePersistedResultContract(task.ExpectDigest, task.ResultBudget, "task")
}

func validatePersistedResultContract(
	digest string,
	budget *contracts.ByteBudget,
	path string,
) error {
	hasDigest := strings.TrimSpace(digest) != ""
	if hasDigest != (budget != nil) {
		return fmt.Errorf("%w: %s result contract snapshot must be entirely set or empty", ErrValidation, path)
	}
	if budget == nil {
		return nil
	}
	if budget.MaxBytes <= 0 {
		return fmt.Errorf("%w: %s.result_budget.max_bytes must be positive", ErrValidation, path)
	}
	if budget.Overflow != contracts.OverflowStore && budget.Overflow != contracts.OverflowReject {
		return fmt.Errorf(
			"%w: %s.result_budget.overflow must be %q or %q",
			ErrValidation,
			path,
			contracts.OverflowStore,
			contracts.OverflowReject,
		)
	}
	return nil
}
