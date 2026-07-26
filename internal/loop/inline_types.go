package loop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/task"
)

// InlineGoalLoopName is the reserved non-catalog display identity for session Goals.
const InlineGoalLoopName = "__session_goal__"

// RunOriginKind classifies the durable source of one Loop Run.
type RunOriginKind string

const (
	// RunOriginCatalog identifies a catalog-backed Run.
	RunOriginCatalog RunOriginKind = "catalog"
	// RunOriginSession identifies a snapshot-only Run created from a session.
	RunOriginSession RunOriginKind = "session"
)

// RunOrigin pins session creation truth for inline execution and reseed.
type RunOrigin struct {
	Kind               RunOriginKind
	SessionID          string
	CreationProfileRef string
	PolicySpecDigest   string
	CreationDigest     string
}

// Normalize trims one Run origin without changing its meaning.
func (o RunOrigin) Normalize() RunOrigin {
	o.Kind = RunOriginKind(strings.TrimSpace(string(o.Kind)))
	o.SessionID = strings.TrimSpace(o.SessionID)
	o.CreationProfileRef = strings.TrimSpace(o.CreationProfileRef)
	o.PolicySpecDigest = strings.TrimSpace(o.PolicySpecDigest)
	o.CreationDigest = strings.TrimSpace(o.CreationDigest)
	if o.Kind == "" {
		o.Kind = RunOriginCatalog
	}
	return o
}

// Validate enforces catalog/session origin field presence.
func (o RunOrigin) Validate() error {
	o = o.Normalize()
	switch o.Kind {
	case RunOriginCatalog:
		if o.SessionID != "" || o.CreationProfileRef != "" || o.PolicySpecDigest != "" || o.CreationDigest != "" {
			return fmt.Errorf("%w: catalog Run origin cannot contain session identity", ErrValidation)
		}
		return nil
	case RunOriginSession:
		if o.SessionID == "" || o.CreationProfileRef == "" || o.PolicySpecDigest == "" || o.CreationDigest == "" {
			return fmt.Errorf("%w: session Run origin identity is incomplete", ErrValidation)
		}
		return nil
	default:
		return fmt.Errorf("%w: Run origin kind is invalid: %q", ErrValidation, o.Kind)
	}
}

// InlineReplaceResult is the public result of one successful expected-Run replacement.
type InlineReplaceResult struct {
	ReplacedRunID RunID
	Run           *Run
}

// InlineReplaceStoreRequest carries one fully prepared replacement into the atomic store boundary.
type InlineReplaceStoreRequest struct {
	ExpectedRunID RunID
	Run           *Run
	Actor         task.ActorContext
	ReplacedAt    time.Time
}

// InlineReplaceStoreResult carries committed replacement state and leases canceled after commit.
type InlineReplaceStoreResult struct {
	ReplacedRunID       RunID
	ReplacedRun         Run
	Run                 Run
	RevokedPromptLeases []GoalPromptLease
}

// InlineGoalClearStoreRequest identifies the newest visible session Goal to clear atomically.
type InlineGoalClearStoreRequest struct {
	WorkspaceID     WorkspaceID
	OriginSessionID string
	Actor           task.ActorContext
	ClearedAt       time.Time
}

// InlineGoalClearStoreResult carries the cleared Run and leases canceled after commit.
type InlineGoalClearStoreResult struct {
	Run                 Run
	Terminalized        bool
	RevokedPromptLeases []GoalPromptLease
}

// InlineGoalClearStore owns newest-session selection, live revocation, and the clear tombstone.
type InlineGoalClearStore interface {
	ClearInlineGoal(context.Context, InlineGoalClearStoreRequest) (InlineGoalClearStoreResult, error)
}

// InlineRunStore owns atomic session-origin start and replacement writes.
type InlineRunStore interface {
	CreateInlineLoopRunForStart(context.Context, Run) (Run, error)
	ReplaceInlineLoopRun(context.Context, InlineReplaceStoreRequest) (InlineReplaceStoreResult, error)
}
