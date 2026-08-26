package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/contracts"
)

const resultContractInvalidCode = "task_result_invalid"

// ResultContractRepairAdmission atomically verifies completion ownership and
// records the one durable repair attempt for a run.
type ResultContractRepairAdmission struct {
	Event      Event
	ClaimToken string
	Now        time.Time
}

type resultContractRejectedPayload struct {
	Code         string `json:"code"`
	RepairPrompt string `json:"repair_prompt"`
}

func (m *Service) admitResultContractRepair(
	ctx context.Context,
	run Run,
	claimToken string,
	actor ActorContext,
	validationErr *ResultContractValidationError,
) (bool, error) {
	store, ok := m.store.(ResultContractRepairStore)
	if !ok {
		return false, errors.New("task: result contract repair store is unavailable")
	}
	prompt := contracts.BuildRepairPrompt(validationErr.Issues)
	event, err := m.newTaskEventWithID(
		resultContractRepairEventID(run.ID),
		run.TaskID,
		run.ID,
		taskEventRunRejected,
		actor,
		resultContractRejectedPayload{Code: resultContractInvalidCode, RepairPrompt: prompt},
	)
	if err != nil {
		return false, err
	}
	first, err := store.AdmitResultContractRepair(ctx, ResultContractRepairAdmission{
		Event: event, ClaimToken: strings.TrimSpace(claimToken), Now: m.now().UTC(),
	})
	if err != nil {
		return false, fmt.Errorf("task: admit result contract repair: %w", err)
	}
	if first {
		m.publishTaskEventsAfterCommand(ctx, []Event{event})
	}
	return first, nil
}

func resultContractRepairEventID(runID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(runID)))
	return "evt-result-contract-repair-" + hex.EncodeToString(digest[:16])
}

func invalidTaskResultFailure(validationErr *ResultContractValidationError) (RunFailure, error) {
	prompt := contracts.BuildRepairPrompt(validationErr.Issues)
	metadata, err := json.Marshal(map[string]string{
		"code":          resultContractInvalidCode,
		"repair_prompt": prompt,
	})
	if err != nil {
		return RunFailure{}, fmt.Errorf("task: marshal invalid result metadata: %w", err)
	}
	return RunFailure{
		Error:    resultContractInvalidCode + ": " + prompt,
		Metadata: metadata,
	}, nil
}

type invalidResultSettlement func(context.Context, RunFailure) (*Run, error)

func (m *Service) settleInvalidResultContract(
	ctx context.Context,
	run Run,
	claimToken string,
	actor ActorContext,
	validationErr *ResultContractValidationError,
	settle invalidResultSettlement,
) (*Run, error) {
	first, err := m.admitResultContractRepair(ctx, run, claimToken, actor, validationErr)
	if err != nil {
		return nil, err
	}
	if first {
		return nil, validationErr
	}
	failure, err := invalidTaskResultFailure(validationErr)
	if err != nil {
		return nil, err
	}
	failed, settlementErr := settle(ctx, failure)
	if settlementErr != nil {
		return nil, errors.Join(ErrResultInvalid, validationErr, settlementErr)
	}
	return failed, errors.Join(ErrResultInvalid, validationErr)
}

func requireResultContractRepairLease(run Run, rawToken string, now time.Time) error {
	if strings.TrimSpace(run.ClaimTokenHash) == "" || !VerifyClaimToken(rawToken, run.ClaimTokenHash) {
		return fmt.Errorf("%w: task run %q token mismatch", ErrInvalidClaimToken, run.ID)
	}
	switch run.Status.Normalize() {
	case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return fmt.Errorf(
			"%w: task run %q is not actively leased",
			ErrInvalidStatusTransition,
			run.ID,
		)
	}
	if run.LeaseUntil.IsZero() || !run.LeaseUntil.After(now.UTC()) {
		return fmt.Errorf("%w: task run %q lease expired", ErrLeaseExpired, run.ID)
	}
	return nil
}

func replaceRunResultStoredValue(result RunResult, stored json.RawMessage) (RunResult, error) {
	updated := result
	if updated.CoordinatorControl == nil {
		updated.Value = append(json.RawMessage(nil), stored...)
		return updated, nil
	}
	var control CoordinatorControlResult
	if err := json.Unmarshal(stored, &control); err != nil {
		return RunResult{}, fmt.Errorf("task: decode sanitized coordinator control: %w", err)
	}
	updated.CoordinatorControl = &control
	return updated, nil
}
