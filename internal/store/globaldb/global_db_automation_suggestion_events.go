package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

func (g *AutomationRepo) RecordSuggestionTransition(
	ctx context.Context,
	suggestion automation.Suggestion,
	jobID string,
) error {
	if err := g.checkReady(ctx, "record automation suggestion transition"); err != nil {
		return err
	}
	observation, err := g.prepareSuggestionTransition(ctx, suggestion, jobID)
	if err != nil {
		return err
	}
	observe := &ObserveRepo{repoBase: g.repoBase}
	if err := g.telemetry.WriteSuggestion(ctx, observation, func(ctx context.Context) error {
		return observe.validateEventSummaryOwner(ctx, observation)
	}); err != nil {
		return fmt.Errorf("store: record automation suggestion transition %q: %w", suggestion.ID, err)
	}
	return nil
}

func (g *AutomationRepo) prepareSuggestionTransition(
	ctx context.Context,
	suggestion automation.Suggestion,
	jobID string,
) (store.EventSummary, error) {
	if err := suggestion.Validate("suggestion"); err != nil {
		return store.EventSummary{}, err
	}
	if suggestion.Status != automation.SuggestionStatusAccepted &&
		suggestion.Status != automation.SuggestionStatusDismissed {
		return store.EventSummary{}, fmt.Errorf(
			"store: cannot record pending automation suggestion transition %q",
			suggestion.ID,
		)
	}
	if suggestion.ResolvedAt == nil || suggestion.ResolvedAt.IsZero() {
		return store.EventSummary{}, errors.New("store: automation suggestion resolved timestamp is required")
	}
	jobID = strings.TrimSpace(jobID)
	if suggestion.Status == automation.SuggestionStatusAccepted && jobID == "" {
		return store.EventSummary{}, errors.New("store: accepted automation suggestion job id is required")
	}
	content, err := json.Marshal(suggestionTransitionContent{
		WorkspaceID:  suggestion.WorkspaceID,
		SuggestionID: suggestion.ID,
		Source:       suggestion.Source,
		DedupKey:     suggestion.DedupKey,
		JobID:        jobID,
	})
	if err != nil {
		return store.EventSummary{}, fmt.Errorf("store: marshal automation suggestion transition: %w", err)
	}
	eventType := events.AutomationSuggestionDismissed
	summary := "automation suggestion dismissed"
	if suggestion.Status == automation.SuggestionStatusAccepted {
		eventType = events.AutomationSuggestionAccepted
		summary = "automation suggestion accepted"
	}
	observation := store.EventSummary{
		ProfileID:   suggestion.ProfileID,
		ID:          "automation-suggestion:" + suggestion.ID + ":" + string(suggestion.Status),
		WorkspaceID: suggestion.WorkspaceID, Type: eventType,
		ActorKind: "automation_suggestion", ActorID: suggestion.ID,
		Outcome: string(events.OutcomeFor(eventType)), Summary: summary, Timestamp: *suggestion.ResolvedAt,
	}
	observation.SetContent(content)
	observe := &ObserveRepo{repoBase: g.repoBase}
	if err := observe.prepareEventSummary(ctx, &observation); err != nil {
		return store.EventSummary{}, err
	}
	return observation, nil
}
