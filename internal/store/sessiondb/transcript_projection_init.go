package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

func initializeTranscriptProjectionState(ctx context.Context, db *sql.DB) error {
	_, err := sqlcgen.New(db).GetTranscriptProjectionState(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: inspect transcript projection state: %w", err)
	}
	if err := sqlcgen.New(db).InitializeTranscriptProjectionState(ctx, transcript.ProjectionVersion); err != nil {
		return fmt.Errorf("store: initialize transcript projection state: %w", err)
	}
	return nil
}
