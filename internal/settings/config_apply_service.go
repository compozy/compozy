package settings

import (
	"context"

	"errors"

	"sync"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
)

const (
	configApplyNoChangesReason = "no_changes_detected"
)

type activeConfigState struct {
	mu          sync.Mutex
	initialized bool
	hash        string
	generation  int64
	config      aghconfig.Config
}

// ApplySection persists a section mutation through the config apply lifecycle.
func (s *service) ApplySection(ctx context.Context, req SectionUpdateRequest) (ApplyResult, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	configLifecycle := s.classifySectionApplyRequest(ctx, req)
	result, err := s.UpdateSection(ctx, req)
	if err != nil {
		return s.recordFailedApply(ctx, req.Section, req.Scope, req.WorkspaceID, configLifecycle, err)
	}
	if result.Section == SectionNetwork {
		return s.recordNetworkSectionApply(ctx, result)
	}
	return s.recordMutationApply(ctx, result)
}

// ApplyCollectionItem persists a collection upsert through the config apply lifecycle.
func (s *service) ApplyCollectionItem(ctx context.Context, req CollectionItemPutRequest) (ApplyResult, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	before, err := s.collectionItemExistsBeforeMutation(ctx, req.CollectionRequest, req.Name)
	if err != nil {
		return ApplyResult{}, err
	}
	expected := applyCollectionLifecycle(MutationResult{}, req.Collection, collectionMutationPut, before)
	result, err := s.PutCollectionItem(ctx, req)
	if err != nil {
		return s.recordFailedApply(
			ctx,
			SectionName(req.Collection),
			req.Scope,
			req.WorkspaceID,
			expected.Lifecycle,
			err,
		)
	}
	result = applyCollectionLifecycle(result, req.Collection, collectionMutationPut, before)
	if req.Collection == CollectionProviders &&
		result.Lifecycle == lifecycle.Live &&
		!mutationResultHasNoChanges(result) {
		return s.recordProviderModelsMutationApply(ctx, result, req.Name)
	}
	return s.recordMutationApply(ctx, result)
}

// ApplyCollectionDelete persists a collection deletion through the config apply lifecycle.
func (s *service) ApplyCollectionDelete(
	ctx context.Context,
	req CollectionItemDeleteRequest,
) (ApplyResult, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	expected := applyCollectionLifecycle(MutationResult{}, req.Collection, collectionMutationDelete, true)
	result, err := s.DeleteCollectionItem(ctx, req)
	if err != nil {
		return s.recordFailedApply(
			ctx,
			SectionName(req.Collection),
			req.Scope,
			req.WorkspaceID,
			expected.Lifecycle,
			err,
		)
	}
	result = applyCollectionLifecycle(result, req.Collection, collectionMutationDelete, true)
	return s.recordMutationApply(ctx, result)
}

// Reload reconciles desired config.toml with the daemon active generation.
func (s *service) Reload(ctx context.Context) (ApplyResult, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	desiredHash, desiredConfig, err := s.currentDesiredConfigHash()
	if err != nil {
		return s.recordFailedApply(ctx, "", ScopeGlobal, "", lifecycle.RestartRequired, err)
	}
	if desiredHash == state.hash {
		return skippedReloadResult(&state, desiredHash), nil
	}

	configLifecycle := classifyReloadLifecycle(&state.config, &desiredConfig)
	record, plan, err := s.persistRuntimeApply(
		ctx,
		&state,
		desiredHash,
		desiredHash,
		&desiredConfig,
		configLifecycle,
		false,
	)
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		Record:          record,
		Applied:         plan.applied,
		NextAction:      lifecycle.NextActionForLifecycle(configLifecycle, plan.status),
		RestartRequired: configLifecycle == lifecycle.RestartRequired,
		RestartScope:    restartScopeForLifecycle(configLifecycle),
		PartialFailures: plan.partialFailures,
	}, nil
}

// ActiveConfig returns the daemon's last successfully applied config generation.
func (s *service) ActiveConfig(ctx context.Context) (aghconfig.Config, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return aghconfig.Config{}, err
	}
	return state.config, nil
}

// ListApplyRecords returns apply history rows.
func (s *service) ListApplyRecords(
	ctx context.Context,
	filter ApplyRecordFilter,
) ([]ApplyRecord, error) {
	if s.applyRecords == nil {
		return nil, errors.New("settings: config apply records are not configured")
	}
	return s.applyRecords.ListApplyRecords(ctx, filter)
}
