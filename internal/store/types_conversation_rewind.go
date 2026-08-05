package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrConversationRewindTargetNotFound      = errors.New("store: conversation rewind target not found")
	ErrConversationRewindTargetInvalid       = errors.New("store: conversation rewind target is invalid")
	ErrConversationRewindFenceConflict       = errors.New("store: conversation rewind fence conflict")
	ErrConversationRewindIdempotencyConflict = errors.New("store: conversation rewind idempotency conflict")
)

type ConversationRewindTarget struct {
	MessageID     string
	StartSequence int64
	DraftText     string
	Generation    int64
	MaxSequence   int64
}

type ConversationRewindState struct {
	TargetMessageID        string
	CoveredThroughSequence int64
	MessagesJSON           string
	UpdatedAt              time.Time
}

type ConversationRewindRequest struct {
	MessageID           string
	IdempotencyKey      string
	RequestHash         string
	ExpectedGeneration  int64
	ExpectedMaxSequence int64
	TranscriptEpoch     int64
	BaselineJSON        string
}

func (r ConversationRewindRequest) Validate() error {
	if strings.TrimSpace(r.MessageID) == "" {
		return errors.New("store: conversation rewind message id is required")
	}
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		return errors.New("store: conversation rewind idempotency key is required")
	}
	if strings.TrimSpace(r.RequestHash) == "" {
		return errors.New("store: conversation rewind request hash is required")
	}
	if r.ExpectedGeneration < 0 {
		return fmt.Errorf("store: invalid conversation rewind generation %d", r.ExpectedGeneration)
	}
	if r.ExpectedMaxSequence < 0 {
		return fmt.Errorf("store: invalid conversation rewind max sequence %d", r.ExpectedMaxSequence)
	}
	if r.TranscriptEpoch <= 0 {
		return errors.New("store: conversation rewind transcript epoch must be positive")
	}
	if strings.TrimSpace(r.BaselineJSON) == "" {
		return errors.New("store: conversation rewind baseline is required")
	}
	return nil
}

type ConversationRewindResult struct {
	IdempotencyKey       string
	TargetMessageID      string
	ArchivedFromSequence int64
	ArchivedToSequence   int64
	ArchivedEventCount   int64
	Generation           int64
	MaxSequence          int64
	TranscriptEpoch      int64
	DraftText            string
	CreatedAt            time.Time
	Replayed             bool
}

type ConversationRewindReader interface {
	ConversationRewindTarget(context.Context, string) (ConversationRewindTarget, error)
	ConversationRewindState(context.Context) (ConversationRewindState, bool, error)
	ConversationRewindReceipt(context.Context, string, string) (ConversationRewindResult, bool, error)
}

type ConversationRewinder interface {
	ConversationRewindReader
	RewindConversation(context.Context, ConversationRewindRequest) (ConversationRewindResult, error)
}
