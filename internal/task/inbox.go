package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/listcursor"
)

const (
	// InboxLaneMyWork identifies directly assigned or actively owned work.
	InboxLaneMyWork InboxLane = "my_work"
	// InboxLaneApprovals identifies approval-gated work awaiting a decision.
	InboxLaneApprovals InboxLane = "approvals"
	// InboxLaneFailedRuns identifies tasks whose latest execution failed.
	InboxLaneFailedRuns InboxLane = "failed_runs"
	// InboxLaneBlocked identifies blocked tasks that are not awaiting approval.
	InboxLaneBlocked InboxLane = "blocked"
	// InboxLaneArchived identifies tasks archived by the current actor.
	InboxLaneArchived InboxLane = "archived"

	taskInboxCursorVersion = 1
	taskInboxCursorKind    = "task_inbox"
)

var (
	// ErrInboxCursorInvalid reports a malformed cursor or one reused with another actor or query.
	ErrInboxCursorInvalid = errors.New("task: inbox cursor is invalid")
)

// InboxLane identifies one actor-scoped inbox grouping.
type InboxLane string

// Normalize returns the canonical inbox lane.
func (l InboxLane) Normalize() InboxLane {
	return InboxLane(strings.ToLower(strings.TrimSpace(string(l))))
}

// Validate ensures the inbox lane is supported.
func (l InboxLane) Validate(path string) error {
	switch l.Normalize() {
	case "", InboxLaneMyWork, InboxLaneApprovals, InboxLaneFailedRuns, InboxLaneBlocked, InboxLaneArchived:
		return nil
	default:
		return fmt.Errorf("%w: %s has unsupported lane %q", ErrValidation, path, l)
	}
}

// InboxQuery captures one bounded actor-scoped task inbox request.
type InboxQuery struct {
	Scope       CatalogScope
	WorkspaceID string
	OwnerKind   OwnerKind
	OwnerRef    string
	Lane        InboxLane
	Status      Status
	Priority    Priority
	Unread      *bool
	Search      string
	Cursor      string
	Limit       int
}

// InboxItem is one action-ready task inbox item.
type InboxItem struct {
	Task             Reference      `json:"task"`
	Lane             InboxLane      `json:"lane"`
	ApprovalPolicy   ApprovalPolicy `json:"approval_policy,omitempty"`
	ApprovalState    ApprovalState  `json:"approval_state,omitempty"`
	BlockingReason   string         `json:"blocking_reason,omitempty"`
	LatestActivityAt time.Time      `json:"latest_activity_at"`
	Run              *RunSummary    `json:"run,omitempty"`
	Triage           TriageState    `json:"triage"`
}

// InboxLaneGroup groups page items by lane while preserving exact filtered counts.
type InboxLaneGroup struct {
	Lane        InboxLane   `json:"lane"`
	Count       int         `json:"count"`
	UnreadCount int         `json:"unread_count"`
	Items       []InboxItem `json:"items,omitempty"`
}

// InboxStatusFacet is one exact filtered status count.
type InboxStatusFacet struct {
	Status Status `json:"status"`
	Count  int    `json:"count"`
}

// InboxPriorityFacet is one exact filtered priority count.
type InboxPriorityFacet struct {
	Priority Priority `json:"priority"`
	Count    int      `json:"count"`
}

// InboxPage is one global stable page with exact pre-cursor counts.
type InboxPage struct {
	Groups         []InboxLaneGroup
	StatusFacets   []InboxStatusFacet
	PriorityFacets []InboxPriorityFacet
	NextCursor     string
	HasMore        bool
	Total          int
	UnreadTotal    int
	ArchivedTotal  int
	Limit          int
}

// InboxCursor captures the last returned row's complete ordering tuple.
type InboxCursor struct {
	Unread       bool      `json:"unread"`
	ActivityAt   time.Time `json:"activity_at"`
	PriorityRank int       `json:"priority_rank"`
	TaskID       string    `json:"task_id"`
}

type inboxFingerprint struct {
	ActorKind   ActorKind    `json:"actor_kind"`
	ActorRef    string       `json:"actor_ref"`
	Scope       CatalogScope `json:"scope"`
	WorkspaceID string       `json:"workspace_id"`
	OwnerKind   OwnerKind    `json:"owner_kind"`
	OwnerRef    string       `json:"owner_ref"`
	Lane        InboxLane    `json:"lane"`
	Status      Status       `json:"status"`
	Priority    Priority     `json:"priority"`
	Unread      *bool        `json:"unread"`
	Search      string       `json:"q"`
}

// InboxReader is the batched persistence capability consumed by Observer.
type InboxReader interface {
	ListTaskInbox(ctx context.Context, query InboxQuery, actor ActorIdentity) (InboxPage, error)
}

// NormalizeInboxQuery validates and canonicalizes an actor-bound inbox query.
func NormalizeInboxQuery(query InboxQuery, actor ActorIdentity) (InboxQuery, ActorIdentity, error) {
	normalizedActor := ActorIdentity{Kind: actor.Kind.Normalize(), Ref: strings.TrimSpace(actor.Ref)}
	if err := normalizedActor.Validate("task_inbox.actor"); err != nil {
		return InboxQuery{}, ActorIdentity{}, err
	}

	query = normalizeInboxQueryFields(query)
	if err := validateInboxQuery(query); err != nil {
		return InboxQuery{}, ActorIdentity{}, err
	}
	if query.Cursor != "" {
		if _, err := DecodeInboxCursor(query, normalizedActor); err != nil {
			return InboxQuery{}, ActorIdentity{}, err
		}
	}
	return query, normalizedActor, nil
}

// Validate reports whether the query fields are supported without decoding an actor-bound cursor.
func (q InboxQuery) Validate() error {
	return validateInboxQuery(normalizeInboxQueryFields(q))
}

func normalizeInboxQueryFields(query InboxQuery) InboxQuery {
	query.Scope = query.Scope.Normalize()
	if query.Scope == "" {
		query.Scope = CatalogScopeAll
	}
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.OwnerKind = query.OwnerKind.Normalize()
	query.OwnerRef = strings.TrimSpace(query.OwnerRef)
	query.Lane = query.Lane.Normalize()
	query.Status = query.Status.Normalize()
	query.Priority = query.Priority.Normalize()
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Limit == 0 {
		query.Limit = DefaultCatalogLimit
	}
	return query
}

func validateInboxQuery(query InboxQuery) error {
	catalogQuery := CatalogQuery{
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		Status:      query.Status,
		Priority:    query.Priority,
		OwnerKind:   query.OwnerKind,
		OwnerRef:    query.OwnerRef,
		Limit:       query.Limit,
		Sort:        CatalogSortRecent,
	}
	if err := validateCatalogQuery(catalogQuery); err != nil {
		return err
	}
	return query.Lane.Validate("task_inbox.lane")
}

// EncodeInboxCursor captures the last returned item without later anchor lookup.
func EncodeInboxCursor(query InboxQuery, actor ActorIdentity, item InboxItem) (string, error) {
	query.Cursor = ""
	normalized, normalizedActor, err := NormalizeInboxQuery(query, actor)
	if err != nil {
		return "", err
	}
	fingerprint, err := taskInboxFingerprint(normalized, normalizedActor)
	if err != nil {
		return "", err
	}
	encoded, err := listcursor.Encode(
		taskInboxCursorVersion,
		taskInboxCursorKind,
		fingerprint,
		InboxCursor{
			Unread:       !item.Triage.Read,
			ActivityAt:   item.LatestActivityAt.UTC(),
			PriorityRank: PriorityRank(item.Task.Priority),
			TaskID:       strings.TrimSpace(item.Task.ID),
		},
	)
	if err != nil {
		return "", fmt.Errorf("task: encode inbox cursor: %w", err)
	}
	return encoded, nil
}

// DecodeInboxCursor verifies and decodes one actor- and query-bound inbox cursor.
func DecodeInboxCursor(query InboxQuery, actor ActorIdentity) (InboxCursor, error) {
	fingerprint, err := taskInboxFingerprint(query, actor)
	if err != nil {
		return InboxCursor{}, err
	}
	cursor, err := listcursor.Decode[InboxCursor](
		query.Cursor,
		taskInboxCursorVersion,
		taskInboxCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return InboxCursor{}, fmt.Errorf("%w: %w", ErrInboxCursorInvalid, err)
	}
	if cursor.ActivityAt.IsZero() || strings.TrimSpace(cursor.TaskID) == "" {
		return InboxCursor{}, fmt.Errorf("%w: cursor position is incomplete", ErrInboxCursorInvalid)
	}
	return cursor, nil
}

func taskInboxFingerprint(query InboxQuery, actor ActorIdentity) (string, error) {
	fingerprint, err := listcursor.Fingerprint(inboxFingerprint{
		ActorKind:   actor.Kind.Normalize(),
		ActorRef:    strings.TrimSpace(actor.Ref),
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		OwnerKind:   query.OwnerKind,
		OwnerRef:    query.OwnerRef,
		Lane:        query.Lane,
		Status:      query.Status,
		Priority:    query.Priority,
		Unread:      query.Unread,
		Search:      query.Search,
	})
	if err != nil {
		return "", fmt.Errorf("task: fingerprint inbox query: %w", err)
	}
	return fingerprint, nil
}

// InboxLanes returns the canonical lane order for a filtered or complete view.
func InboxLanes(filter InboxLane) []InboxLane {
	if normalized := filter.Normalize(); normalized != "" {
		return []InboxLane{normalized}
	}
	return []InboxLane{
		InboxLaneMyWork,
		InboxLaneApprovals,
		InboxLaneFailedRuns,
		InboxLaneBlocked,
		InboxLaneArchived,
	}
}
