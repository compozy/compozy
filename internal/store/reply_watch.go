package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

const (
	ReplyWatchArmed       = "armed"
	ReplyWatchFired       = "fired"
	ReplyWatchDelivered   = "delivered"
	ReplyWatchAbandoned   = "abandoned"
	ReplyOutcomeCompleted = "completed"
	ReplyOutcomeFailed    = "failed"
	ReplyOutcomeCanceled  = "canceled"
	ReplyOutcomeDropped   = "dropped"
	ReplyOutcomeUnknown   = "unknown"
)

// DBTX allows registration to join the admission owner's transaction.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type ReplyWatch struct {
	ID                string
	WorkspaceID       string
	SenderSessionID   string
	TargetWorkspaceID string
	TargetSessionID   string
	MessageID         string
	AdmissionID       string
	TurnID            string
	QueueEntryID      string
	DeliveredInputID  string
	AbandonReason     string
	Hop               int
	State             string
	Outcome           string
	ReplyText         string
	ReplyTruncated    bool
	CreatedAt         time.Time
	FiredAt           *time.Time
	DeliveredAt       *time.Time
}

type ReplyWatchRegistration struct {
	WorkspaceID       string
	SenderSessionID   string
	TargetWorkspaceID string
	TargetSessionID   string
	MessageID         string
	AdmissionID       string
	TurnID            string
	QueueEntryID      string
	Hop               int
	CreatedAt         time.Time
}

type ReplyWatchFire struct {
	Outcome   string
	Text      string
	Truncated bool
}

type ReplyWatchFilter struct {
	State           string
	WorkspaceID     string
	SenderSessionID string
	TargetSessionID string
}

type ReplyWatchStore interface {
	InsertReplyWatchTx(context.Context, DBTX, ReplyWatchRegistration) (ReplyWatch, error)
	GetReplyWatch(context.Context, string) (ReplyWatch, error)
	ListReplyWatches(context.Context, ReplyWatchFilter) ([]ReplyWatch, error)
	BindReplyWatch(context.Context, string, string, string) error
	FireReplyWatch(context.Context, string, ReplyWatchFire) (bool, error)
	DeliverReplyWatch(context.Context, string, SessionInputQueueInsert) (bool, error)
	AbandonReplyWatch(context.Context, string, string) error
	ReplyWatchEvidence(context.Context, ReplyWatch) (SessionPromptAdmission, []SessionInputQueueEntry, error)
}

// ReplyWatchID is stable across admission retries and process restarts.
func ReplyWatchID(target, message string) string {
	digest := sha256.Sum256([]byte(target + "\x00" + message))
	return fmt.Sprintf("rw-%x", digest[:8])
}
