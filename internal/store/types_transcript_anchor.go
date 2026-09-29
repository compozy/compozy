package store

import (
	"context"
	"errors"
)

var (
	// ErrTranscriptAnchorNotFound reports a message id with no active transcript entry.
	ErrTranscriptAnchorNotFound = errors.New("store: transcript anchor not found")
	// ErrTranscriptAnchorInvalid reports an anchor that is not a complete user entry.
	ErrTranscriptAnchorInvalid = errors.New("store: transcript anchor is not a complete user message")
)

// TranscriptUserAnchor is a durable user message resolved to its turn. The lookup sits
// below rewind's destructive-eligibility rules: an archived prefix before the anchor
// does not invalidate it.
type TranscriptUserAnchor struct {
	MessageID     string
	TurnID        string
	StartSequence int64
	Complete      bool
}

// TranscriptAnchorReader resolves durable user-message anchors for non-destructive cuts.
type TranscriptAnchorReader interface {
	TranscriptUserAnchor(ctx context.Context, messageID string) (TranscriptUserAnchor, error)
}
