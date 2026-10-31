package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type PullRequestState string
type ChecksState string
type ReviewDecision string
type Mergeable string
type LookupState string

const (
	PullRequestOpen        PullRequestState = "open"
	PullRequestClosed      PullRequestState = "closed"
	PullRequestMerged      PullRequestState = "merged"
	ChecksPassing          ChecksState      = "passing"
	ChecksFailing          ChecksState      = "failing"
	ChecksPending          ChecksState      = "pending"
	ChecksNone             ChecksState      = "none"
	ChecksUnknown          ChecksState      = "unknown"
	ReviewApproved         ReviewDecision   = "approved"
	ReviewChangesRequested ReviewDecision   = "changes_requested"
	ReviewRequired         ReviewDecision   = "review_required"
	ReviewNone             ReviewDecision   = "none"
	ReviewUnknown          ReviewDecision   = "unknown"
	MergeableYes           Mergeable        = "mergeable"
	MergeableConflicting   Mergeable        = "conflicting"
	MergeableUnknown       Mergeable        = "unknown"
	LookupOK               LookupState      = "ok"
	LookupPending          LookupState      = "pending"
	LookupError            LookupState      = "error"
	LookupIncomplete       LookupState      = "incomplete"
)

type PullRequestChecks struct {
	State       ChecksState
	Passed      int
	Failed      int
	Pending     int
	FailedNames []string
}

type PullRequest struct {
	Provider       string
	Number         int
	URL            string
	State          PullRequestState
	Draft          *bool
	HeadSHA        string
	Checks         PullRequestChecks
	ReviewDecision ReviewDecision
	Mergeable      Mergeable
	ReadyToMerge   bool
}

type PullRequestLookup struct {
	State      LookupState
	Cause      string
	ObservedAt time.Time
	Stale      bool
}

type BranchKey struct{ WorkspaceID, Branch string }
type PullRequestView struct {
	Lookup      PullRequestLookup
	PullRequest *PullRequest
}

type PullRequestReader interface {
	PullRequests(context.Context, []BranchKey) map[BranchKey]PullRequestView
}

type BranchReader interface {
	CurrentBranch(context.Context, string) (string, error)
}

type BoardColumn string

const (
	BoardNeedsYou BoardColumn = "needs_you"
	BoardWorking  BoardColumn = "working"
	BoardWaiting  BoardColumn = "waiting"
	BoardInReview BoardColumn = "in_review"
	BoardDone     BoardColumn = "done"
	BadgeWaiting  Badge       = "waiting"
)

type WaitingOn string

const (
	WaitingOnSessions WaitingOn = "sessions"
	WaitingOnMonitor  WaitingOn = "monitor"
)

// BoardColumnFor derives a session's board column. A waiting session (idle on a registered
// wake source) keeps its own column even with an open pull request: parked work beats review.
func BoardColumnFor(badge Badge, prState PullRequestState) BoardColumn {
	if ClassForBadge(badge) == AttentionNeedsYou {
		return BoardNeedsYou
	}
	switch badge {
	case BadgeRunning, BadgeHung, BadgeUnhealthy:
		return BoardWorking
	case BadgeWaiting:
		return BoardWaiting
	}
	if prState == PullRequestOpen {
		return BoardInReview
	}
	return BoardDone
}

// PullRequestRepositoryReader resolves current identities for scoped catalog SQL.
// A nil workspace list selects all readable workspaces. Missing keys never match.
type PullRequestRepositoryReader interface {
	RepositoryKeys(context.Context, store.ReadScope, []string) map[string]string
}

// PullRequestDefaultBranchReader shares default-branch suppression with durable board reads.
type PullRequestDefaultBranchReader interface {
	DefaultBranchNames(context.Context, store.ReadScope, []string) map[string][]string
}
