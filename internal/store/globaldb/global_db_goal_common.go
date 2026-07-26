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

func validateGoalRunWorkspace(ctx context.Context, exec taskSQLExecutor, key goal.TurnKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	_, err := sqlcgen.New(exec).GoalRunWorkspaceExists(ctx, sqlcgen.GoalRunWorkspaceExistsParams{
		ID: string(key.LoopRunID), WorkspaceID: string(key.WorkspaceID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, key.LoopRunID)
	}
	if err != nil {
		return fmt.Errorf("store: validate goal run workspace %q: %w", key.LoopRunID, err)
	}
	return nil
}

func validateBindingRunWorkspace(ctx context.Context, exec taskSQLExecutor, key goal.BindingKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	return validateGoalRunWorkspace(ctx, exec, goal.TurnKey{
		WorkspaceID: key.WorkspaceID,
		LoopRunID:   key.LoopRunID,
		Generation:  1,
		NodeID:      "binding",
	})
}

func goalControlStaleError(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalControlStale,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}

func goalBindingMismatchError(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeContinuousBindingMismatch,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}

func goalNotActiveError(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalNotActive,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}

func goalPromptFencedError(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalPromptFenced,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}

func parseGoalTimestampValue(value any, field string) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), nil
	case string:
		return parseGoalTimestampText(typed, field)
	case []byte:
		return parseGoalTimestampText(string(typed), field)
	default:
		return time.Time{}, fmt.Errorf("store: parse goal %s from %T", field, value)
	}
}

func parseOptionalGoalTimestampValue(value any, field string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := parseGoalTimestampValue(value, field)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseGoalTimestampText(value string, field string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if parsed, err := store.ParseTimestamp(trimmed); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse goal %s: %w", field, err)
	}
	return parsed.UTC(), nil
}
