package store

import (
	"context"
	"errors"
)

var ErrSessionSideParentArchived = errors.New("Unarchive its session first.")

type SessionSideStore interface {
	ListSessionSides(context.Context, string, string, bool) ([]SessionInfo, error)
}
