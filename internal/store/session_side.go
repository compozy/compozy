package store

import (
	"context"
	"errors"
)

var ErrSessionSideParentArchived = errors.New("session: side chat parent is archived")

type SessionSideStore interface {
	ListSessionSides(context.Context, string, string, bool) ([]SessionInfo, error)
}
