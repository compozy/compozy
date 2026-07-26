package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type suggestionTransitionContent struct {
	WorkspaceID  string                      `json:"workspace_id"`
	SuggestionID string                      `json:"suggestion_id"`
	Source       automation.SuggestionSource `json:"source"`
	DedupKey     string                      `json:"dedup_key"`
	JobID        string                      `json:"job_id,omitempty"`
}

func (g *AutomationRepo) CreateSuggestion(
	ctx context.Context,
	suggestion automation.Suggestion,
	pendingCap int,
) (created automation.Suggestion, err error) {
	if err := g.checkReady(ctx, "create automation suggestion"); err != nil {
		return automation.Suggestion{}, err
	}
	if pendingCap <= 0 {
		return automation.Suggestion{}, fmt.Errorf(
			"store: automation suggestion pending cap must be positive: %d",
			pendingCap,
		)
	}
	normalized, payloadJSON, err := g.normalizeSuggestionForCreate(suggestion)
	if err != nil {
		return automation.Suggestion{}, err
	}

	err = store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		existing, lookupErr := queries.GetAutomationSuggestionByDedupKey(
			ctx,
			sqlcgen.GetAutomationSuggestionByDedupKeyParams{
				WorkspaceID: normalized.WorkspaceID,
				DedupKey:    normalized.DedupKey,
			},
		)
		if lookupErr == nil {
			mapped, mapErr := automationSuggestionFromGenerated(existing)
			if mapErr != nil {
				return mapErr
			}
			created = mapped
			return nil
		}
		if !errors.Is(lookupErr, sql.ErrNoRows) {
			return fmt.Errorf("store: find automation suggestion dedup latch: %w", lookupErr)
		}

		pending, countErr := queries.CountPendingAutomationSuggestions(ctx, normalized.WorkspaceID)
		if countErr != nil {
			return fmt.Errorf("store: count pending automation suggestions: %w", countErr)
		}
		if pending >= int64(pendingCap) {
			return automation.ErrSuggestionPendingCap
		}
		if insertErr := queries.InsertAutomationSuggestion(ctx, sqlcgen.InsertAutomationSuggestionParams{
			ID: normalized.ID, WorkspaceID: normalized.WorkspaceID, Source: string(normalized.Source),
			DedupKey: normalized.DedupKey, Status: string(normalized.Status), Payload: payloadJSON,
			CreatedAt: store.FormatTimestamp(normalized.CreatedAt), ResolvedAt: nullableAutomationTime(nil),
		}); insertErr != nil {
			return fmt.Errorf("store: insert automation suggestion %q: %w", normalized.ID, insertErr)
		}
		created = normalized
		return nil
	})
	if err != nil {
		return automation.Suggestion{}, err
	}
	return created, nil
}

func (g *AutomationRepo) GetSuggestion(
	ctx context.Context,
	workspaceID string,
	id string,
) (automation.Suggestion, error) {
	if err := g.checkReady(ctx, "get automation suggestion"); err != nil {
		return automation.Suggestion{}, err
	}
	workspaceID, id, err := requireSuggestionIdentity(workspaceID, id)
	if err != nil {
		return automation.Suggestion{}, err
	}
	row, err := g.queries.GetAutomationSuggestion(ctx, sqlcgen.GetAutomationSuggestionParams{
		WorkspaceID: workspaceID,
		ID:          id,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return automation.Suggestion{}, automation.ErrSuggestionNotFound
	}
	if err != nil {
		return automation.Suggestion{}, fmt.Errorf("store: get automation suggestion %q: %w", id, err)
	}
	return automationSuggestionFromGenerated(row)
}

func (g *AutomationRepo) ListSuggestions(
	ctx context.Context,
	workspaceID string,
	status automation.SuggestionStatus,
) ([]automation.Suggestion, error) {
	if err := g.checkReady(ctx, "list automation suggestions"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("store: automation suggestion workspace id is required")
	}
	if status != "" {
		if err := status.Validate("suggestion.status"); err != nil {
			return nil, err
		}
	}
	rows, err := g.queries.ListAutomationSuggestions(ctx, sqlcgen.ListAutomationSuggestionsParams{
		WorkspaceID: workspaceID,
		Status:      string(status),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list automation suggestions: %w", err)
	}
	return automationSuggestionsFromGenerated(rows)
}

func (g *AutomationRepo) ListAcceptedSuggestions(ctx context.Context) ([]automation.Suggestion, error) {
	if err := g.checkReady(ctx, "list accepted automation suggestions"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListAcceptedAutomationSuggestions(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list accepted automation suggestions: %w", err)
	}
	return automationSuggestionsFromGenerated(rows)
}

func (g *AutomationRepo) ResolveSuggestion(
	ctx context.Context,
	workspaceID string,
	id string,
	to automation.SuggestionStatus,
) (resolved automation.Suggestion, err error) {
	if err := g.checkReady(ctx, "resolve automation suggestion"); err != nil {
		return automation.Suggestion{}, err
	}
	workspaceID, id, err = requireSuggestionIdentity(workspaceID, id)
	if err != nil {
		return automation.Suggestion{}, err
	}
	if to != automation.SuggestionStatusAccepted && to != automation.SuggestionStatusDismissed {
		return automation.Suggestion{}, fmt.Errorf(
			"store: suggestion resolution status must be accepted or dismissed: %q",
			to,
		)
	}
	resolvedAt := g.now().UTC()
	err = store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		affected, updateErr := queries.ResolveAutomationSuggestion(ctx, sqlcgen.ResolveAutomationSuggestionParams{
			Status: string(to), ResolvedAt: nullableAutomationTime(&resolvedAt),
			WorkspaceID: workspaceID, ID: id,
		})
		if updateErr != nil {
			return fmt.Errorf("store: resolve automation suggestion %q: %w", id, updateErr)
		}
		if affected == 0 {
			_, lookupErr := queries.GetAutomationSuggestion(ctx, sqlcgen.GetAutomationSuggestionParams{
				WorkspaceID: workspaceID,
				ID:          id,
			})
			if errors.Is(lookupErr, sql.ErrNoRows) {
				return automation.ErrSuggestionNotFound
			}
			if lookupErr != nil {
				return fmt.Errorf("store: verify automation suggestion resolution %q: %w", id, lookupErr)
			}
			return automation.ErrSuggestionResolved
		}
		row, lookupErr := queries.GetAutomationSuggestion(ctx, sqlcgen.GetAutomationSuggestionParams{
			WorkspaceID: workspaceID,
			ID:          id,
		})
		if lookupErr != nil {
			return fmt.Errorf("store: reload resolved automation suggestion %q: %w", id, lookupErr)
		}
		mapped, mapErr := automationSuggestionFromGenerated(row)
		if mapErr != nil {
			return mapErr
		}
		resolved = mapped
		return nil
	})
	if err != nil {
		return automation.Suggestion{}, err
	}
	return resolved, nil
}

func (g *AutomationRepo) RollbackSuggestionAcceptance(
	ctx context.Context,
	workspaceID string,
	id string,
	resolvedAt time.Time,
) error {
	if err := g.checkReady(ctx, "rollback automation suggestion acceptance"); err != nil {
		return err
	}
	workspaceID, id, err := requireSuggestionIdentity(workspaceID, id)
	if err != nil {
		return err
	}
	if resolvedAt.IsZero() {
		return errors.New("store: automation suggestion resolved timestamp is required")
	}
	affected, err := g.queries.RollbackAutomationSuggestionAcceptance(
		ctx,
		sqlcgen.RollbackAutomationSuggestionAcceptanceParams{
			WorkspaceID: workspaceID,
			ID:          id,
			ResolvedAt:  nullableAutomationTime(&resolvedAt),
		},
	)
	if err != nil {
		return fmt.Errorf("store: rollback automation suggestion acceptance %q: %w", id, err)
	}
	if affected == 0 {
		return automation.ErrSuggestionResolved
	}
	return nil
}

func (g *AutomationRepo) RecordSuggestionTransition(
	ctx context.Context,
	suggestion automation.Suggestion,
	jobID string,
) error {
	if err := g.checkReady(ctx, "record automation suggestion transition"); err != nil {
		return err
	}
	if err := suggestion.Validate("suggestion"); err != nil {
		return err
	}
	if suggestion.Status != automation.SuggestionStatusAccepted &&
		suggestion.Status != automation.SuggestionStatusDismissed {
		return fmt.Errorf("store: cannot record pending automation suggestion transition %q", suggestion.ID)
	}
	if suggestion.ResolvedAt == nil || suggestion.ResolvedAt.IsZero() {
		return errors.New("store: automation suggestion resolved timestamp is required")
	}
	jobID = strings.TrimSpace(jobID)
	if suggestion.Status == automation.SuggestionStatusAccepted && jobID == "" {
		return errors.New("store: accepted automation suggestion job id is required")
	}
	content, err := json.Marshal(suggestionTransitionContent{
		WorkspaceID:  suggestion.WorkspaceID,
		SuggestionID: suggestion.ID,
		Source:       suggestion.Source,
		DedupKey:     suggestion.DedupKey,
		JobID:        jobID,
	})
	if err != nil {
		return fmt.Errorf("store: marshal automation suggestion transition: %w", err)
	}
	eventType := events.AutomationSuggestionDismissed
	summary := "automation suggestion dismissed"
	if suggestion.Status == automation.SuggestionStatusAccepted {
		eventType = events.AutomationSuggestionAccepted
		summary = "automation suggestion accepted"
	}
	if err := g.queries.InsertAutomationSuggestionEventSummary(
		ctx,
		sqlcgen.InsertAutomationSuggestionEventSummaryParams{
			ID:          "automation-suggestion:" + suggestion.ID + ":" + string(suggestion.Status),
			WorkspaceID: suggestion.WorkspaceID, Type: eventType, ContentJson: string(content),
			ActorID: suggestion.ID, Outcome: string(events.OutcomeFor(eventType)),
			Summary: nullableObserveString(summary), Timestamp: store.FormatTimestamp(*suggestion.ResolvedAt),
		},
	); err != nil {
		return fmt.Errorf("store: record automation suggestion transition %q: %w", suggestion.ID, err)
	}
	return nil
}

func (g *AutomationRepo) normalizeSuggestionForCreate(
	suggestion automation.Suggestion,
) (automation.Suggestion, string, error) {
	normalized := suggestion
	normalized.ID = strings.TrimSpace(normalized.ID)
	if normalized.ID == "" {
		normalized.ID = store.NewID("sug")
	}
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.Source = automation.SuggestionSource(strings.TrimSpace(string(normalized.Source)))
	normalized.DedupKey = strings.TrimSpace(normalized.DedupKey)
	if normalized.Status == "" {
		normalized.Status = automation.SuggestionStatusPending
	}
	if normalized.Status != automation.SuggestionStatusPending {
		return automation.Suggestion{}, "", errors.New("store: new automation suggestion must be pending")
	}
	normalized.ResolvedAt = nil
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now().UTC()
	} else {
		normalized.CreatedAt = normalized.CreatedAt.UTC()
	}
	if err := normalized.Validate("suggestion"); err != nil {
		return automation.Suggestion{}, "", err
	}
	payload, err := json.Marshal(normalized.Payload)
	if err != nil {
		return automation.Suggestion{}, "", fmt.Errorf("store: marshal automation suggestion payload: %w", err)
	}
	return normalized, string(payload), nil
}

func automationSuggestionFromGenerated(row sqlcgen.AutomationSuggestion) (automation.Suggestion, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return automation.Suggestion{}, fmt.Errorf("store: parse automation suggestion created_at: %w", err)
	}
	var resolvedAt *time.Time
	if row.ResolvedAt.Valid {
		parsed, parseErr := store.ParseTimestamp(row.ResolvedAt.String)
		if parseErr != nil {
			return automation.Suggestion{}, fmt.Errorf("store: parse automation suggestion resolved_at: %w", parseErr)
		}
		resolvedAt = &parsed
	}
	var payload automation.Job
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		return automation.Suggestion{}, fmt.Errorf("store: decode automation suggestion payload: %w", err)
	}
	suggestion := automation.Suggestion{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Source: automation.SuggestionSource(row.Source),
		DedupKey: row.DedupKey, Status: automation.SuggestionStatus(row.Status), Payload: payload,
		CreatedAt: createdAt, ResolvedAt: resolvedAt,
	}
	if err := suggestion.Validate("suggestion"); err != nil {
		return automation.Suggestion{}, fmt.Errorf("store: invalid automation suggestion row: %w", err)
	}
	return suggestion, nil
}

func automationSuggestionsFromGenerated(rows []sqlcgen.AutomationSuggestion) ([]automation.Suggestion, error) {
	suggestions := make([]automation.Suggestion, 0, len(rows))
	for _, row := range rows {
		suggestion, err := automationSuggestionFromGenerated(row)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, suggestion)
	}
	return suggestions, nil
}

func requireSuggestionIdentity(workspaceID string, id string) (string, string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	id = strings.TrimSpace(id)
	if workspaceID == "" {
		return "", "", errors.New("store: automation suggestion workspace id is required")
	}
	if id == "" {
		return "", "", errors.New("store: automation suggestion id is required")
	}
	return workspaceID, id, nil
}
