package task

import (
	"fmt"
	"strings"
	"time"
)

// Normalize returns a validated completion request with default time applied.
func (c LeaseCompletion) Normalize(defaultNow time.Time) (LeaseCompletion, error) {
	return c.NormalizeWithResultLimit(defaultNow, MaxResultBytes)
}

// NormalizeWithResultLimit normalizes one completion under an explicit structural result bound.
func (c LeaseCompletion) NormalizeWithResultLimit(
	defaultNow time.Time,
	maxResultBytes int,
) (LeaseCompletion, error) {
	normalized := c
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	normalized.CreatedTaskIDs = normalizeCreatedTaskIDs(normalized.CreatedTaskIDs)
	normalized.Now = normalizeLeaseNow(normalized.Now, defaultNow)
	result, err := normalizeRunResultWithLimit(normalized.Result, maxResultBytes)
	if err != nil {
		return LeaseCompletion{}, err
	}
	normalized.Result = result
	if err := normalized.ValidateWithResultLimit("lease_completion", maxResultBytes); err != nil {
		return LeaseCompletion{}, err
	}
	return normalized, nil
}

// Validate reports whether the completion request is internally consistent.
func (c LeaseCompletion) Validate(path string) error {
	return c.ValidateWithResultLimit(path, MaxResultBytes)
}

// ValidateWithResultLimit validates a completion under an explicit result-value bound.
func (c LeaseCompletion) ValidateWithResultLimit(path string, maxResultBytes int) error {
	if err := validateLeaseRunToken(c.RunID, c.ClaimToken, path); err != nil {
		return err
	}
	if err := validateAuditCollectionSize(
		len(c.CreatedTaskIDs),
		nestedPath(path, "created_task_ids"),
	); err != nil {
		return err
	}
	for idx, taskID := range c.CreatedTaskIDs {
		if strings.TrimSpace(taskID) == "" {
			return fmt.Errorf(
				"%w: %s is required",
				ErrValidation,
				nestedPath(path, fmt.Sprintf("created_task_ids[%d]", idx)),
			)
		}
		if err := ValidateReferenceSize(
			taskID,
			nestedPath(path, fmt.Sprintf("created_task_ids[%d]", idx)),
		); err != nil {
			return err
		}
	}
	if c.TokensUsed < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "tokens_used"),
			c.TokensUsed,
		)
	}
	return c.Result.ValidateWithValueLimit(nestedPath(path, "result"), maxResultBytes)
}

// ValidateForRun applies the authoritative persisted-envelope or Loop action result limit.
func (c LeaseCompletion) ValidateForRun(path string, run Run) error {
	maxResultBytes := MaxResultBytes
	if run.IsLoopWorker() && c.Result.CoordinatorControl == nil {
		maxResultBytes = c.actionResultMaxBytes
		if maxResultBytes <= 0 || maxResultBytes > MaxActionResultBytes {
			maxResultBytes = MaxActionResultBytes
		}
	}
	return c.ValidateWithResultLimit(path, maxResultBytes)
}
