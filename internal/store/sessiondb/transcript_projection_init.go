package sessiondb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/agh/internal/transcript"
)

func initializeTranscriptProjectionState(ctx context.Context, db *sql.DB) error {
	if err := sqlcgen.New(db).InitializeTranscriptProjectionState(ctx, transcript.ProjectionVersion); err != nil {
		return fmt.Errorf("store: initialize transcript projection state: %w", err)
	}
	return nil
}
