package goal

import (
	"context"

	"github.com/compozy/agh/internal/loop"
)

// SessionProjection is the store-owned state needed to compose one canonical session Goal snapshot.
type SessionProjection struct {
	Found             bool
	Cleared           bool
	RunID             loop.RunID
	RunStatus         loop.Status
	DefinitionDigest  string
	OriginSessionID   string
	ContextNudgeRatio float64
	Checkpoint        *Checkpoint
	LastVerdict       *Turn
}

// SessionProjectionReader loads the newest session-origin Goal before applying its clear tombstone.
type SessionProjectionReader interface {
	GetSessionGoalProjection(
		context.Context,
		loop.WorkspaceID,
		string,
	) (SessionProjection, error)
}
