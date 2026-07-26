package transcript

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

const (
	// ProjectionVersion identifies the deterministic transcript projection contract.
	ProjectionVersion = 1
	defaultPageLimit  = 200
	maxPageLimit      = 1000
)

var (
	// ErrProjectionCorrupt reports materialized transcript data that cannot be decoded.
	ErrProjectionCorrupt = errors.New("transcript: materialized projection is corrupt")
	// ErrProjectionIncompatible reports a projection produced by an unsupported contract version.
	ErrProjectionIncompatible = errors.New("transcript: materialized projection is incompatible")
)

// PageQuery selects one bounded chronological window ending before a stable start cursor.
type PageQuery struct {
	BeforeSequence int64
	Limit          int
}

// Normalize validates and applies bounded defaults to a page query.
func (q PageQuery) Normalize() (PageQuery, error) {
	if q.BeforeSequence < 0 {
		return PageQuery{}, errors.New("transcript: before sequence cannot be negative")
	}
	if q.Limit < 0 || q.Limit > maxPageLimit {
		return PageQuery{}, fmt.Errorf("transcript: page limit must be between 0 and %d", maxPageLimit)
	}
	if q.Limit == 0 {
		q.Limit = defaultPageLimit
	}
	return q, nil
}

// Page is one bounded chronological transcript window.
type Page struct {
	Entries            []Entry `json:"entries"`
	Generation         int64   `json:"generation"`
	MaxSequence        int64   `json:"max_sequence"`
	HasOlder           bool    `json:"has_older"`
	NextBeforeSequence int64   `json:"next_before_sequence,omitempty"`
}

// ChangeQuery selects materialized entries updated after an event cursor.
type ChangeQuery struct {
	AfterSequence int64
	Limit         int
}

// Normalize validates and applies bounded defaults to a change query.
func (q ChangeQuery) Normalize() (ChangeQuery, error) {
	if q.AfterSequence < 0 {
		return ChangeQuery{}, errors.New("transcript: after sequence cannot be negative")
	}
	if q.Limit < 0 || q.Limit > maxPageLimit {
		return ChangeQuery{}, fmt.Errorf("transcript: change limit must be between 0 and %d", maxPageLimit)
	}
	if q.Limit == 0 {
		q.Limit = defaultPageLimit
	}
	return q, nil
}

// ChangePage is one bounded set of entries shaped by events after a cursor.
type ChangePage struct {
	Entries     []Entry `json:"entries"`
	Generation  int64   `json:"generation"`
	MaxSequence int64   `json:"max_sequence"`
	HasMore     bool    `json:"has_more"`
	NextAfter   int64   `json:"next_after_sequence,omitempty"`
}

// Reader exposes proportional transcript page and change reads.
type Reader interface {
	TranscriptPage(context.Context, PageQuery) (Page, error)
	TranscriptChanges(context.Context, ChangeQuery) (ChangePage, error)
}

// ProjectionState is the durable routing state for incremental projection.
type ProjectionState struct {
	Version        int
	Generation     int64
	ActiveEntryKey string
}

// EntryKind classifies one independently materialized transcript segment.
type EntryKind string

const (
	EntryKindAssistant EntryKind = "assistant"
	EntryKindUser      EntryKind = "user"
	EntryKindSystem    EntryKind = "system"
	EntryKindMarker    EntryKind = "marker"
)

// EntryIdentity is the stable metadata for one logical transcript segment.
type EntryIdentity struct {
	Key             string
	Kind            EntryKind
	LogicalID       string
	TurnID          string
	BaseMessageID   string
	MessageID       string
	StartSequence   int64
	UpdatedSequence int64
	Complete        bool
}

// ProjectionResolver resolves durable routing identities without replaying history.
type ProjectionResolver interface {
	EntryIdentity(context.Context, string) (EntryIdentity, bool, error)
	ToolEntryIdentity(context.Context, string) (EntryIdentity, bool, error)
	LatestAssistantIdentity(context.Context, string) (EntryIdentity, bool, error)
}

// Assignment binds one canonical event to its stable logical entry.
type Assignment struct {
	Entry         EntryIdentity
	CompletedKeys []string
	ToolKey       string
}

// ProjectionSegment is one complete rebuildable materialized entry.
type ProjectionSegment struct {
	Identity EntryIdentity
	Events   []store.SessionEvent
	Entry    *Entry
}
