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
	// CatalogScopeAll includes global tasks and the workspace selected by the query.
	CatalogScopeAll CatalogScope = "all"
	// CatalogScopeGlobal includes only global tasks.
	CatalogScopeGlobal CatalogScope = "global"
	// CatalogScopeWorkspace includes only tasks from the selected workspace.
	CatalogScopeWorkspace CatalogScope = "workspace"

	// CatalogSortRecent orders tasks by their latest durable activity.
	CatalogSortRecent CatalogSort = "recent"
	// CatalogSortPriority orders tasks by priority and then latest durable activity.
	CatalogSortPriority CatalogSort = "priority"

	// DefaultCatalogLimit bounds public task catalog reads when no limit is supplied.
	DefaultCatalogLimit = 50
	// MaxCatalogLimit bounds one public task catalog page.
	MaxCatalogLimit = 200

	taskCatalogCursorVersion = 1
	taskCatalogCursorKind    = "task_catalog"
)

var (
	// ErrCatalogCursorInvalid reports a malformed cursor or one reused with a different query.
	ErrCatalogCursorInvalid = errors.New("task: catalog cursor is invalid")
)

// CatalogScope identifies the visibility boundary applied to a catalog query.
type CatalogScope string

// Normalize returns the canonical catalog scope.
func (s CatalogScope) Normalize() CatalogScope {
	return CatalogScope(strings.ToLower(strings.TrimSpace(string(s))))
}

// CatalogSort identifies a supported task catalog ordering.
type CatalogSort string

// Normalize returns the canonical catalog sort.
func (s CatalogSort) Normalize() CatalogSort {
	return CatalogSort(strings.ToLower(strings.TrimSpace(string(s))))
}

// CatalogQuery captures one bounded task catalog request.
type CatalogQuery struct {
	Scope                CatalogScope
	WorkspaceID          string
	Status               Status
	Priority             Priority
	IncludeDrafts        bool
	ApprovalState        ApprovalState
	OwnerKind            OwnerKind
	OwnerRef             string
	ParentTaskID         string
	ParticipationChannel string
	Search               string
	Sort                 CatalogSort
	Cursor               string
	Limit                int
}

// CatalogStatusFacet is one exact status count before the page cut.
type CatalogStatusFacet struct {
	Status Status `json:"status"`
	Count  int    `json:"count"`
}

// CatalogOwnerFacet is one exact assigned-owner count before the page cut.
type CatalogOwnerFacet struct {
	Owner Ownership `json:"owner"`
	Count int       `json:"count"`
}

// CatalogPage is one stable, counted task catalog page.
type CatalogPage struct {
	Tasks        []Summary
	StatusFacets []CatalogStatusFacet
	OwnerFacets  []CatalogOwnerFacet
	NextCursor   string
	HasMore      bool
	Total        int
	Limit        int
}

// CatalogCursor is the captured ordering tuple of the last returned row.
type CatalogCursor struct {
	PriorityRank int       `json:"priority_rank"`
	ActivityAt   time.Time `json:"activity_at"`
	TaskID       string    `json:"task_id"`
}

type catalogFingerprint struct {
	Scope                CatalogScope  `json:"scope"`
	WorkspaceID          string        `json:"workspace_id"`
	Status               Status        `json:"status"`
	Priority             Priority      `json:"priority"`
	IncludeDrafts        bool          `json:"include_drafts"`
	ApprovalState        ApprovalState `json:"approval_state"`
	OwnerKind            OwnerKind     `json:"owner_kind"`
	OwnerRef             string        `json:"owner_ref"`
	ParentTaskID         string        `json:"parent_task_id"`
	ParticipationChannel string        `json:"participation_channel"`
	Search               string        `json:"q"`
	Sort                 CatalogSort   `json:"sort"`
}

// CatalogReader is the batched persistence capability used by public task catalogs.
type CatalogReader interface {
	ListTaskCatalog(ctx context.Context, query CatalogQuery) (CatalogPage, error)
}

// CatalogManager is the public task catalog capability exposed by Service.
type CatalogManager interface {
	ListTaskCatalog(ctx context.Context, query CatalogQuery, actor ActorContext) (CatalogPage, error)
}

// NormalizeCatalogQuery validates and canonicalizes one public task catalog request.
func NormalizeCatalogQuery(query CatalogQuery) (CatalogQuery, error) {
	query.Scope = query.Scope.Normalize()
	if query.Scope == "" {
		query.Scope = CatalogScopeAll
	}
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.Status = query.Status.Normalize()
	query.Priority = query.Priority.Normalize()
	query.ApprovalState = query.ApprovalState.Normalize()
	query.OwnerKind = query.OwnerKind.Normalize()
	query.OwnerRef = strings.TrimSpace(query.OwnerRef)
	query.ParentTaskID = strings.TrimSpace(query.ParentTaskID)
	query.ParticipationChannel = strings.TrimSpace(query.ParticipationChannel)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Sort = query.Sort.Normalize()
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Sort == "" {
		query.Sort = CatalogSortRecent
	}
	if query.Limit == 0 {
		query.Limit = DefaultCatalogLimit
	}

	if err := validateCatalogQuery(query); err != nil {
		return CatalogQuery{}, err
	}
	if query.Cursor != "" {
		if _, err := DecodeCatalogCursor(query); err != nil {
			return CatalogQuery{}, err
		}
	}
	return query, nil
}

func validateCatalogQuery(query CatalogQuery) error {
	switch query.Scope {
	case CatalogScopeAll:
	case CatalogScopeGlobal:
		if query.WorkspaceID != "" {
			return fmt.Errorf("%w: task_catalog.workspace_id must be empty for global scope", ErrValidation)
		}
	case CatalogScopeWorkspace:
		if query.WorkspaceID == "" {
			return fmt.Errorf("%w: task_catalog.workspace_id is required for workspace scope", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: task_catalog.scope has unsupported value %q", ErrValidation, query.Scope)
	}
	if query.Status != "" {
		if err := query.Status.Validate("task_catalog.status"); err != nil {
			return err
		}
	}
	if query.Priority != "" {
		if err := query.Priority.Validate("task_catalog.priority"); err != nil {
			return err
		}
	}
	if query.ApprovalState != "" {
		if err := query.ApprovalState.Validate("task_catalog.approval_state"); err != nil {
			return err
		}
	}
	if query.OwnerKind != "" {
		if err := query.OwnerKind.Validate("task_catalog.owner_kind"); err != nil {
			return err
		}
	}
	if query.Sort != CatalogSortRecent && query.Sort != CatalogSortPriority {
		return fmt.Errorf("%w: task_catalog.sort has unsupported value %q", ErrValidation, query.Sort)
	}
	if query.Limit < 1 || query.Limit > MaxCatalogLimit {
		return fmt.Errorf(
			"%w: task_catalog.limit must be between 1 and %d: %d",
			ErrValidation,
			MaxCatalogLimit,
			query.Limit,
		)
	}
	return nil
}

// EncodeCatalogCursor captures the last returned row without relying on later anchor lookup.
func EncodeCatalogCursor(query CatalogQuery, summary *Summary) (string, error) {
	normalized, err := NormalizeCatalogQuery(CatalogQueryWithoutCursor(query))
	if err != nil {
		return "", err
	}
	fingerprint, err := taskCatalogFingerprint(normalized)
	if err != nil {
		return "", err
	}
	encoded, err := listcursor.Encode(
		taskCatalogCursorVersion,
		taskCatalogCursorKind,
		fingerprint,
		CatalogCursor{
			PriorityRank: PriorityRank(summary.Priority),
			ActivityAt:   summary.LastActivityAt.UTC(),
			TaskID:       strings.TrimSpace(summary.ID),
		},
	)
	if err != nil {
		return "", fmt.Errorf("task: encode catalog cursor: %w", err)
	}
	return encoded, nil
}

// DecodeCatalogCursor verifies and decodes a query-bound task catalog cursor.
func DecodeCatalogCursor(query CatalogQuery) (CatalogCursor, error) {
	fingerprint, err := taskCatalogFingerprint(query)
	if err != nil {
		return CatalogCursor{}, err
	}
	cursor, err := listcursor.Decode[CatalogCursor](
		query.Cursor,
		taskCatalogCursorVersion,
		taskCatalogCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return CatalogCursor{}, fmt.Errorf("%w: %w", ErrCatalogCursorInvalid, err)
	}
	if cursor.ActivityAt.IsZero() || strings.TrimSpace(cursor.TaskID) == "" {
		return CatalogCursor{}, fmt.Errorf("%w: cursor position is incomplete", ErrCatalogCursorInvalid)
	}
	return cursor, nil
}

// CatalogQueryWithoutCursor returns the query identity used to encode a next cursor.
func CatalogQueryWithoutCursor(query CatalogQuery) CatalogQuery {
	query.Cursor = ""
	return query
}

func taskCatalogFingerprint(query CatalogQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(catalogFingerprint{
		Scope:                query.Scope,
		WorkspaceID:          query.WorkspaceID,
		Status:               query.Status,
		Priority:             query.Priority,
		IncludeDrafts:        query.IncludeDrafts,
		ApprovalState:        query.ApprovalState,
		OwnerKind:            query.OwnerKind,
		OwnerRef:             query.OwnerRef,
		ParentTaskID:         query.ParentTaskID,
		ParticipationChannel: query.ParticipationChannel,
		Search:               query.Search,
		Sort:                 query.Sort,
	})
	if err != nil {
		return "", fmt.Errorf("task: fingerprint catalog query: %w", err)
	}
	return fingerprint, nil
}

// PriorityRank returns the canonical highest-first task priority rank.
func PriorityRank(priority Priority) int {
	switch priority.Normalize() {
	case PriorityUrgent:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}
