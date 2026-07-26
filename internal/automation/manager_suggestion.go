package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

// SuggestionAcceptance is the durable suggestion transition and resulting Job.
type SuggestionAcceptance struct {
	Suggestion Suggestion `json:"suggestion"`
	Job        Job        `json:"job"`
}

// ListSuggestions lists exact-workspace suggestions after idempotently seeding
// the first-party starter catalog on first touch.
func (m *Manager) ListSuggestions(
	ctx context.Context,
	workspaceRef string,
	status SuggestionStatus,
) ([]Suggestion, error) {
	workspaceID, agentName, err := m.resolveSuggestionWorkspace(ctx, workspaceRef)
	if err != nil {
		return nil, err
	}
	if status != "" {
		if err := status.Validate("suggestion.status"); err != nil {
			return nil, err
		}
	}
	if err := m.ensureStarterSuggestions(ctx, workspaceID, agentName); err != nil {
		return nil, fmt.Errorf("automation: seed starter suggestions: %w", err)
	}
	return m.store.ListSuggestions(ctx, workspaceID, status)
}

// AcceptSuggestion durably accepts one pending proposal and creates its Job
// through the same validation and authoritative persistence path as CreateJob.
func (m *Manager) AcceptSuggestion(
	ctx context.Context,
	workspaceRef string,
	suggestionID string,
) (SuggestionAcceptance, error) {
	workspaceID, _, err := m.resolveSuggestionWorkspace(ctx, workspaceRef)
	if err != nil {
		return SuggestionAcceptance{}, err
	}
	suggestion, err := m.store.GetSuggestion(ctx, workspaceID, strings.TrimSpace(suggestionID))
	if err != nil {
		return SuggestionAcceptance{}, err
	}
	prepared, err := m.prepareSuggestedJob(ctx, suggestion)
	if err != nil {
		return SuggestionAcceptance{}, err
	}
	existing, jobExists, err := m.suggestedJobIdentityState(ctx, prepared)
	if err != nil {
		return SuggestionAcceptance{}, err
	}

	accepted, err := m.store.ResolveSuggestion(
		ctx,
		workspaceID,
		suggestion.ID,
		SuggestionStatusAccepted,
	)
	if err != nil {
		return SuggestionAcceptance{}, err
	}
	if jobExists {
		m.recordSuggestionTransition(ctx, accepted, existing.ID)
		return SuggestionAcceptance{Suggestion: accepted, Job: existing}, nil
	}
	created, err := m.createPreparedJob(ctx, prepared)
	if err != nil {
		return SuggestionAcceptance{}, m.handleAcceptedJobCreateFailure(ctx, accepted, prepared, err)
	}
	m.recordSuggestionTransition(ctx, accepted, created.ID)
	return SuggestionAcceptance{Suggestion: accepted, Job: created}, nil
}

// DismissSuggestion durably dismisses one pending proposal without creating a Job.
func (m *Manager) DismissSuggestion(
	ctx context.Context,
	workspaceRef string,
	suggestionID string,
) (Suggestion, error) {
	workspaceID, _, err := m.resolveSuggestionWorkspace(ctx, workspaceRef)
	if err != nil {
		return Suggestion{}, err
	}
	dismissed, err := m.store.ResolveSuggestion(
		ctx,
		workspaceID,
		strings.TrimSpace(suggestionID),
		SuggestionStatusDismissed,
	)
	if err != nil {
		return Suggestion{}, err
	}
	m.recordSuggestionTransition(ctx, dismissed, "")
	return dismissed, nil
}

func (m *Manager) resolveSuggestionWorkspace(
	ctx context.Context,
	workspaceRef string,
) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("automation: suggestion context is required")
	}
	workspaceRef = strings.TrimSpace(workspaceRef)
	if workspaceRef == "" {
		return "", "", errors.New("automation: suggestion workspace is required")
	}
	resolved, err := m.workspaceResolver.Resolve(ctx, workspaceRef)
	if err != nil {
		return "", "", fmt.Errorf("automation: resolve suggestion workspace: %w", err)
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	agentName := strings.TrimSpace(resolved.Config.Defaults.Agent)
	if workspaceID == "" {
		return "", "", errors.New("automation: resolved suggestion workspace id is required")
	}
	if agentName == "" {
		return "", "", errors.New("automation: resolved suggestion default agent is required")
	}
	return workspaceID, agentName, nil
}

func (m *Manager) prepareSuggestedJob(ctx context.Context, suggestion Suggestion) (Job, error) {
	job := cloneJob(suggestion.Payload)
	job.ID = suggestionJobID(suggestion.WorkspaceID, suggestion.DedupKey)
	job.Scope = AutomationScopeWorkspace
	job.WorkspaceID = suggestion.WorkspaceID
	job.Source = JobSourceDynamic
	prepared, err := m.prepareJobForCreate(ctx, job)
	if err != nil {
		return Job{}, fmt.Errorf("automation: validate suggestion %q job: %w", suggestion.ID, err)
	}
	return prepared, nil
}

func (m *Manager) suggestedJobIdentityState(ctx context.Context, prepared Job) (Job, bool, error) {
	current, err := m.authoritativeJobDefinition(ctx, prepared.ID)
	switch {
	case errors.Is(err, ErrJobNotFound):
		return Job{}, false, nil
	case err != nil:
		return Job{}, false, fmt.Errorf("automation: inspect suggested job %q: %w", prepared.ID, err)
	case sameJobDefinition(current, prepared):
		return current, true, nil
	default:
		return Job{}, false, fmt.Errorf(
			"automation: suggested job id %q conflicts with another definition",
			prepared.ID,
		)
	}
}

func (m *Manager) handleAcceptedJobCreateFailure(
	ctx context.Context,
	accepted Suggestion,
	prepared Job,
	createErr error,
) error {
	current, lookupErr := m.authoritativeJobDefinition(ctx, prepared.ID)
	switch {
	case errors.Is(lookupErr, ErrJobNotFound):
		if accepted.ResolvedAt == nil {
			return errors.Join(createErr, errors.New("automation: accepted suggestion resolved timestamp is missing"))
		}
		rollbackErr := m.store.RollbackSuggestionAcceptance(
			ctx,
			accepted.WorkspaceID,
			accepted.ID,
			*accepted.ResolvedAt,
		)
		return errors.Join(createErr, rollbackErr)
	case lookupErr != nil:
		return errors.Join(createErr, fmt.Errorf("automation: inspect accepted job: %w", lookupErr))
	case !sameJobDefinition(current, prepared):
		return errors.Join(
			createErr,
			fmt.Errorf("automation: accepted job %q conflicts with another definition", prepared.ID),
		)
	default:
		return createErr
	}
}

func (m *Manager) authoritativeJobDefinition(ctx context.Context, id string) (Job, error) {
	if !m.resourceDefinitionsEnabled() {
		return m.store.GetJob(ctx, id)
	}
	record, err := m.jobResources.Get(ctx, m.resourceActor, id)
	if errors.Is(err, resources.ErrNotFound) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}
	return cloneJob(record.Spec), nil
}

func (m *Manager) recordSuggestionTransition(ctx context.Context, suggestion Suggestion, jobID string) {
	if err := m.store.RecordSuggestionTransition(ctx, suggestion, jobID); err != nil {
		m.logger.Warn(
			"automation.suggestion.transition_record_failed",
			"suggestion_id", suggestion.ID,
			"workspace_id", suggestion.WorkspaceID,
			"status", suggestion.Status,
			"error", err,
		)
	}
}
