package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

func (m *Service) prepareResultContract(
	ctx context.Context,
	run Run,
	payload json.RawMessage,
) (json.RawMessage, error) {
	if m == nil {
		return nil, fmt.Errorf("task: result budget service is required")
	}
	prepared, err := m.sanitizeAndValidateTaskResult(ctx, run, payload)
	if err != nil {
		return nil, err
	}
	budget := m.resultBudget
	if resultBudget := run.ResultBudgetValue(); resultBudget != nil {
		budget = *resultBudget
	}
	outcome, err := contracts.EnforceBudget(budget, prepared)
	if err != nil {
		return nil, &ResultContractValidationError{
			Issues: []contracts.ValidationIssue{{
				Path:    "$",
				Message: "result exceeds its immutable byte budget",
			}},
			Cause: err,
		}
	}
	return outcome.Payload, nil
}

func (m *Service) sanitizeAndValidateTaskResult(
	ctx context.Context,
	run Run,
	payload json.RawMessage,
) (json.RawMessage, error) {
	expectDigest := run.ExpectDigestValue()
	hasContract := strings.TrimSpace(expectDigest) != ""
	if len(bytes.TrimSpace(payload)) == 0 {
		if !hasContract {
			return nil, nil
		}
		return nil, taskResultIssues(contracts.ValidationIssue{
			Path: "$", Message: "contracted result must be valid JSON",
		})
	}
	if !json.Valid(payload) {
		return nil, taskResultIssues(contracts.ValidationIssue{Path: "$", Message: "result must be valid JSON"})
	}
	if !hasContract {
		clean, _, reject := contracts.SanitizeText(string(payload))
		if reject || !json.Valid([]byte(clean)) {
			return nil, taskResultIssues(contracts.ValidationIssue{
				Path: "$", Message: "result contains unsafe secret material",
			})
		}
		return json.RawMessage(clean), nil
	}
	if m.resultContracts == nil {
		return nil, fmt.Errorf("task: result contract registry is required")
	}
	contract, err := m.resultContracts.Resolve(ctx, expectDigest)
	if err != nil {
		return nil, fmt.Errorf("task: resolve result contract: %w", err)
	}
	redacted, redactions, redactErr := contracts.RedactPreservingContract(contract, payload)
	if redactErr == nil {
		return m.validatePreparedTaskResult(ctx, expectDigest, redacted)
	}
	if len(redactions) > 0 {
		return nil, taskResultIssues(contracts.ValidationIssue{
			Path: "$", Message: "result contains secret material in a contract-constrained field",
		})
	}
	clean, _, reject := contracts.SanitizeText(string(payload))
	if reject || !json.Valid([]byte(clean)) {
		return nil, taskResultIssues(contracts.ValidationIssue{
			Path: "$", Message: "result contains unsafe secret material",
		})
	}
	return m.validatePreparedTaskResult(ctx, expectDigest, json.RawMessage(clean))
}

func (m *Service) validatePreparedTaskResult(
	ctx context.Context,
	digest string,
	payload json.RawMessage,
) (json.RawMessage, error) {
	verdict, err := m.resultContracts.Validate(ctx, digest, payload)
	if err != nil {
		return nil, fmt.Errorf("task: validate result contract: %w", err)
	}
	if !verdict.Valid {
		return nil, &ResultContractValidationError{Issues: verdict.Issues}
	}
	if verdict.Unwrapped {
		return contracts.UnwrapSingleObject(payload), nil
	}
	return append(json.RawMessage(nil), payload...), nil
}

func taskResultIssues(issues ...contracts.ValidationIssue) error {
	return &ResultContractValidationError{Issues: issues}
}
