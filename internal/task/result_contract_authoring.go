package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

func (m *Service) applyNewTaskExpectation(ctx context.Context, task *Task, spec CreateTask) error {
	digest, budget, err := m.prepareTaskExpectation(ctx, spec.Expect, spec.ResultBudget)
	if err != nil {
		return err
	}
	task.ExpectDigest = digest
	task.ResultBudget = budget
	return nil
}

func (m *Service) applyTaskExpectationPatch(
	ctx context.Context,
	task *Task,
	patch Patch,
	changedFields []string,
) ([]string, error) {
	if patch.Expect == nil {
		return changedFields, nil
	}
	digest, budget, err := m.prepareTaskExpectation(ctx, *patch.Expect, patch.ResultBudget)
	if err != nil {
		return nil, err
	}
	if task.ExpectDigest != digest {
		task.ExpectDigest = digest
		changedFields = append(changedFields, TaskFieldExpectDigest)
	}
	if !sameResultBudget(task.ResultBudget, budget) {
		task.ResultBudget = cloneResultBudget(budget)
		changedFields = append(changedFields, TaskFieldResultBudget)
	}
	return changedFields, nil
}

func (m *Service) prepareTaskExpectation(
	ctx context.Context,
	expect json.RawMessage,
	override *contracts.ByteBudget,
) (string, *contracts.ByteBudget, error) {
	if len(bytes.TrimSpace(expect)) == 0 {
		if override != nil {
			return "", nil, fmt.Errorf("%w: result_budget requires expect", ErrValidation)
		}
		return "", nil, nil
	}
	if m.resultContracts == nil {
		return "", nil, fmt.Errorf("task: result contract registry is required")
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
		return "", nil, fmt.Errorf("%w: result_budget: %w", ErrValidation, err)
	}
	contract, err := m.resultContracts.Pin(ctx, expect)
	if err != nil {
		return "", nil, fmt.Errorf("task: pin result contract: %w", err)
	}
	return contract.Digest, &resolved, nil
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
	hasDigest := strings.TrimSpace(task.ExpectDigest) != ""
	hasBudget := task.ResultBudget != nil
	if hasDigest != hasBudget {
		return fmt.Errorf(
			"%w: task.expect_digest and task.result_budget must be set together",
			ErrValidation,
		)
	}
	if !hasBudget {
		return nil
	}
	if task.ResultBudget.MaxBytes <= 0 {
		return fmt.Errorf("%w: task.result_budget.max_bytes must be positive", ErrValidation)
	}
	if task.ResultBudget.Overflow != contracts.OverflowStore &&
		task.ResultBudget.Overflow != contracts.OverflowReject {
		return fmt.Errorf(
			"%w: task.result_budget.overflow must be %q or %q",
			ErrValidation,
			contracts.OverflowStore,
			contracts.OverflowReject,
		)
	}
	return nil
}
