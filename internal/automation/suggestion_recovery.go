package automation

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/resources"
)

func (m *Manager) recoverAcceptedSuggestionsLocked(ctx context.Context) error {
	accepted, err := m.store.ListAcceptedSuggestions(ctx)
	if err != nil {
		return fmt.Errorf("automation: list accepted suggestions for recovery: %w", err)
	}
	for _, suggestion := range accepted {
		prepared, prepareErr := m.prepareSuggestedJob(ctx, suggestion)
		if prepareErr != nil {
			return prepareErr
		}
		current, lookupErr := m.authoritativeJobDefinition(ctx, prepared.ID)
		switch {
		case errors.Is(lookupErr, ErrJobNotFound):
			current, lookupErr = m.persistPreparedJobDefinition(ctx, prepared)
		case lookupErr == nil && !sameJobDefinition(current, prepared):
			lookupErr = fmt.Errorf(
				"automation: accepted suggestion %q conflicts with job %q",
				suggestion.ID,
				prepared.ID,
			)
		}
		if lookupErr != nil {
			return fmt.Errorf("automation: recover accepted suggestion %q: %w", suggestion.ID, lookupErr)
		}
		m.recordSuggestionTransition(ctx, suggestion, current.ID)
	}
	return nil
}

func (m *Manager) persistPreparedJobDefinition(ctx context.Context, prepared Job) (Job, error) {
	if !m.resourceDefinitionsEnabled() {
		return m.store.CreateJob(ctx, prepared)
	}
	record, err := m.jobResources.Put(
		ctx,
		m.resourceActorForSource(JobSourceDynamic),
		resources.Draft[Job]{
			ID:              prepared.ID,
			Scope:           ResourceScopeForAutomation(prepared.Scope, prepared.WorkspaceID),
			ExpectedVersion: 0,
			Spec:            prepared,
		},
	)
	if err != nil {
		return Job{}, err
	}
	return cloneJob(record.Spec), nil
}
