package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.SessionOutboxStore = (*GoalRepo)(nil)

// EnqueueGoalSessionOutbox inserts or returns one byte-identical projection event.
func (g *GoalRepo) EnqueueGoalSessionOutbox(
	ctx context.Context,
	request goal.EnqueueSessionOutboxRequest,
) (goal.SessionOutboxEvent, error) {
	if err := g.checkReady(ctx, "enqueue goal session outbox"); err != nil {
		return goal.SessionOutboxEvent{}, err
	}

	var event goal.SessionOutboxEvent
	err := g.withImmediateTransaction(ctx, "enqueue goal session outbox", func(exec globalSQLExecutor) error {
		var enqueueErr error
		event, enqueueErr = enqueueGoalSessionOutboxWithExecutor(ctx, exec, request)
		return enqueueErr
	})
	if err != nil {
		return goal.SessionOutboxEvent{}, err
	}
	return event, nil
}

// ClaimGoalSessionOutbox lists the oldest pending projection events for retryable relay.
func (g *GoalRepo) ClaimGoalSessionOutbox(
	ctx context.Context,
	limit int,
) (events []goal.SessionOutboxEvent, err error) {
	if err := g.checkReady(ctx, "claim goal session outbox"); err != nil {
		return nil, err
	}
	if limit < 0 || limit > 200 {
		return nil, fmt.Errorf("%w: goal outbox claim limit must be between 0 and 200", looppkg.ErrValidation)
	}
	if limit == 0 {
		limit = 50
	}
	rows, err := sqlcgen.New(g.db).ClaimGoalSessionOutbox(ctx, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("store: claim goal session outbox: %w", err)
	}
	events = make([]goal.SessionOutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, goalSessionOutboxFromGenerated(row))
	}
	return events, nil
}

// AcknowledgeGoalSessionOutbox records the first successful delivery timestamp.
func (g *GoalRepo) AcknowledgeGoalSessionOutbox(
	ctx context.Context,
	eventID string,
	deliveredAt time.Time,
) error {
	if err := g.checkReady(ctx, "acknowledge goal session outbox"); err != nil {
		return err
	}
	normalizedEventID := strings.TrimSpace(eventID)
	if normalizedEventID == "" {
		return fmt.Errorf("%w: goal outbox event_id is required", looppkg.ErrValidation)
	}
	if deliveredAt.IsZero() {
		return fmt.Errorf("%w: goal outbox delivered_at is required", looppkg.ErrValidation)
	}
	deliveredAt = deliveredAt.UTC()
	return g.withImmediateTransaction(ctx, "acknowledge goal session outbox", func(exec globalSQLExecutor) error {
		event, err := loadGoalSessionOutboxByEventID(ctx, exec, normalizedEventID)
		if err != nil {
			return err
		}
		if deliveredAt.Before(event.CreatedAt) {
			return fmt.Errorf("%w: goal outbox delivered_at precedes created_at", looppkg.ErrValidation)
		}
		if event.DeliveredAt != nil {
			return nil
		}
		if err := sqlcgen.New(exec).AcknowledgeGoalSessionOutbox(ctx, sqlcgen.AcknowledgeGoalSessionOutboxParams{
			DeliveredAt: store.FormatTimestamp(deliveredAt), EventID: normalizedEventID,
		}); err != nil {
			return fmt.Errorf("store: acknowledge goal session outbox event %q: %w", normalizedEventID, err)
		}
		return nil
	})
}

func enqueueGoalSessionOutboxWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	request goal.EnqueueSessionOutboxRequest,
) (goal.SessionOutboxEvent, error) {
	request = normalizeGoalSessionOutboxRequest(request)
	if err := request.Validate(); err != nil {
		return goal.SessionOutboxEvent{}, err
	}
	if err := validateGoalOutboxTarget(ctx, exec, request); err != nil {
		return goal.SessionOutboxEvent{}, err
	}
	if err := sqlcgen.New(exec).EnqueueGoalSessionOutbox(ctx, sqlcgen.EnqueueGoalSessionOutboxParams{
		EventID: request.EventID, WorkspaceID: string(request.WorkspaceID), OriginSessionID: request.OriginSessionID,
		LoopRunID: string(request.LoopRunID), BoundSessionID: goalNullableStringPointer(request.BoundSessionID),
		Cause: string(request.Cause), CreatedAt: store.FormatTimestamp(request.CreatedAt),
	}); err != nil {
		return goal.SessionOutboxEvent{}, fmt.Errorf(
			"store: insert goal session outbox event %q: %w",
			request.EventID,
			err,
		)
	}
	loaded, err := loadGoalSessionOutboxByEventID(ctx, exec, request.EventID)
	if err != nil {
		return goal.SessionOutboxEvent{}, err
	}
	if !goalSessionOutboxMatchesRequest(loaded, request) {
		return goal.SessionOutboxEvent{}, fmt.Errorf(
			"%w: goal outbox event %q already has a different payload",
			looppkg.ErrTransitionConflict,
			request.EventID,
		)
	}
	return loaded, nil
}

func normalizeGoalSessionOutboxRequest(
	request goal.EnqueueSessionOutboxRequest,
) goal.EnqueueSessionOutboxRequest {
	request.EventID = strings.TrimSpace(request.EventID)
	request.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(request.WorkspaceID)))
	request.OriginSessionID = strings.TrimSpace(request.OriginSessionID)
	request.LoopRunID = looppkg.RunID(strings.TrimSpace(string(request.LoopRunID)))
	if request.BoundSessionID != nil {
		normalizedBoundSessionID := strings.TrimSpace(*request.BoundSessionID)
		request.BoundSessionID = &normalizedBoundSessionID
	}
	request.Cause = goal.SessionOutboxCause(strings.TrimSpace(string(request.Cause)))
	request.CreatedAt = request.CreatedAt.UTC()
	return request
}

func validateGoalOutboxTarget(
	ctx context.Context,
	exec globalSQLExecutor,
	request goal.EnqueueSessionOutboxRequest,
) error {
	key := goal.TurnKey{
		WorkspaceID: request.WorkspaceID,
		LoopRunID:   request.LoopRunID,
		Generation:  1,
		NodeID:      "session-outbox",
	}
	if err := validateGoalRunWorkspace(ctx, exec, key); err != nil {
		return err
	}
	origin, err := sqlcgen.New(exec).GetGoalRunOrigin(ctx, sqlcgen.GetGoalRunOriginParams{
		ID: string(request.LoopRunID), WorkspaceID: string(request.WorkspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: load goal outbox run origin %q: %w", request.LoopRunID, err)
	}
	if origin.OriginKind != goalRunOriginKindSession || !origin.OriginSessionID.Valid ||
		strings.TrimSpace(origin.OriginSessionID.String) != request.OriginSessionID {
		return fmt.Errorf(
			"%w: goal outbox origin does not match session-origin run %q",
			looppkg.ErrTransitionConflict,
			request.LoopRunID,
		)
	}
	if err := validateGoalOutboxSessionWorkspace(
		ctx,
		exec,
		request.OriginSessionID,
		request.WorkspaceID,
	); err != nil {
		return err
	}
	if request.BoundSessionID != nil {
		if err := validateGoalOutboxSessionWorkspace(
			ctx,
			exec,
			*request.BoundSessionID,
			request.WorkspaceID,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateGoalOutboxSessionWorkspace(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	workspaceID looppkg.WorkspaceID,
) error {
	_, err := sqlcgen.New(exec).ValidateGoalSessionWorkspace(ctx, sqlcgen.ValidateGoalSessionWorkspaceParams{
		ID: sessionID, WorkspaceID: string(workspaceID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if err != nil {
		return fmt.Errorf("store: validate goal outbox session %q: %w", sessionID, err)
	}
	return nil
}

func loadGoalSessionOutboxByEventID(
	ctx context.Context,
	exec globalSQLExecutor,
	eventID string,
) (goal.SessionOutboxEvent, error) {
	row, err := sqlcgen.New(exec).GetGoalSessionOutboxByEventID(ctx, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return goal.SessionOutboxEvent{}, fmt.Errorf("%w: %s", goal.ErrOutboxEventNotFound, eventID)
	}
	if err != nil {
		return goal.SessionOutboxEvent{}, fmt.Errorf("store: load goal session outbox event %q: %w", eventID, err)
	}
	return goalSessionOutboxFromGenerated(row), nil
}

func goalSessionOutboxMatchesRequest(
	event goal.SessionOutboxEvent,
	request goal.EnqueueSessionOutboxRequest,
) bool {
	return event.EventID == request.EventID &&
		event.WorkspaceID == request.WorkspaceID &&
		event.OriginSessionID == request.OriginSessionID &&
		event.LoopRunID == request.LoopRunID &&
		goalOptionalStringEqual(event.BoundSessionID, request.BoundSessionID) &&
		event.Cause == request.Cause &&
		event.CreatedAt.Equal(request.CreatedAt)
}

func goalNullableStringPointer(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return goalNullableString(*value)
}

func goalOptionalStringEqual(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
