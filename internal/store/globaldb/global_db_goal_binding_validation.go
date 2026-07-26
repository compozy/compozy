package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func validateActiveGoalPromptBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	handle string,
	bindingEpoch int64,
	sessionID string,
) error {
	_, err := sqlcgen.New(exec).ActiveGoalPromptBindingExists(ctx, sqlcgen.ActiveGoalPromptBindingExistsParams{
		LoopRunID: string(key.LoopRunID), WorkspaceID: string(key.WorkspaceID),
		Handle: strings.TrimSpace(handle), BindingEpoch: bindingEpoch, SessionID: strings.TrimSpace(sessionID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goalBindingMismatchError("Goal prompt binding is no longer active")
	}
	if err != nil {
		return fmt.Errorf("store: validate active Goal prompt binding: %w", err)
	}
	return nil
}
