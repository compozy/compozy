package settings

import (
	"context"

	"errors"

	"time"

	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
)

type applyRecordInput struct {
	desiredHash  string
	activeHash   string
	generation   int64
	lifecycle    lifecycle.Lifecycle
	status       lifecycle.Status
	diagnostics  []diagnosticcontract.DiagnosticItem
	appliedAtNow bool
}

type runtimeApplyPlan struct {
	status          lifecycle.Status
	activeHash      string
	generation      int64
	applied         bool
	partialFailures []ApplyFailure
	diagnostics     []diagnosticcontract.DiagnosticItem
}

func (s *service) recordMutationApply(ctx context.Context, result MutationResult) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	desiredHash, desiredConfig, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, err
	}
	return s.recordProjectedMutationApply(
		ctx,
		result,
		&state,
		desiredHash,
		desiredHash,
		&desiredConfig,
	)
}

func (s *service) recordFailedApply(
	ctx context.Context,
	section SectionName,
	scope ScopeKind,
	workspaceID string,
	configLifecycle lifecycle.Lifecycle,
	cause error,
) (ApplyResult, error) {
	state, stateErr := s.ensureActiveConfigState(ctx)
	if stateErr != nil {
		return ApplyResult{}, stateErr
	}
	if configLifecycle == "" {
		configLifecycle = lifecycle.RestartRequired
	}
	desiredHash, _, hashErr := s.currentDesiredConfigHash()
	if hashErr != nil {
		desiredHash = state.hash
	}
	diagnostic := diagnostics.NewItem(
		"config.apply.failed",
		diagnosticcontract.CodeConfigInvalid,
		diagnosticcontract.CategoryConfig,
		"Config apply failed",
		cause.Error(),
		diagnosticcontract.SeverityError,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithSuggestedCommand("agh config validate"),
	)
	record, err := s.createTerminalApplyRecord(ctx, applyRecordInput{
		desiredHash: desiredHash,
		activeHash:  state.hash,
		generation:  state.generation,
		lifecycle:   configLifecycle,
		status:      lifecycle.StatusFailed,
		diagnostics: []diagnosticcontract.DiagnosticItem{diagnostic},
	})
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		Record:          record,
		Section:         section,
		Scope:           scope,
		WorkspaceID:     workspaceID,
		Applied:         false,
		NextAction:      lifecycle.NextActionRetry,
		RestartRequired: false,
	}, cause
}

func (s *service) createTerminalApplyRecord(
	ctx context.Context,
	input applyRecordInput,
) (ApplyRecord, error) {
	pending, err := s.createPendingApplyRecord(ctx, input)
	if err != nil {
		return ApplyRecord{}, err
	}
	return s.finalizeApplyRecord(ctx, pending, input)
}

func (s *service) createPendingApplyRecord(
	ctx context.Context,
	input applyRecordInput,
) (ApplyRecord, error) {
	if s.applyRecords == nil {
		return ApplyRecord{}, errors.New("settings: config apply records are not configured")
	}
	now := time.Now().UTC()
	return s.applyRecords.CreateApplyRecord(ctx, ApplyRecord{
		DesiredHash: input.desiredHash,
		ActiveHash:  input.activeHash,
		Generation:  input.generation,
		Actor:       mutationSourceFromContext(ctx),
		DiffClass:   lifecycle.DiffClass(input.lifecycle),
		Status:      lifecycle.StatusPendingApply,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (s *service) finalizeApplyRecord(
	ctx context.Context,
	pending ApplyRecord,
	input applyRecordInput,
) (ApplyRecord, error) {
	if s.applyRecords == nil {
		return ApplyRecord{}, errors.New("settings: config apply records are not configured")
	}
	pending.Status = input.status
	pending.Lifecycle = input.lifecycle
	pending.DiffClass = lifecycle.DiffClass(input.lifecycle)
	pending.NextAction = lifecycle.NextActionForLifecycle(input.lifecycle, input.status)
	pending.DesiredHash = input.desiredHash
	pending.ActiveHash = input.activeHash
	pending.Generation = input.generation
	pending.Diagnostics = input.diagnostics
	pending.UpdatedAt = time.Now().UTC()
	if input.appliedAtNow {
		appliedAt := pending.UpdatedAt
		pending.AppliedAt = &appliedAt
	}
	return s.applyRecords.UpdateApplyRecord(ctx, pending)
}
