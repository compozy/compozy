package session

import (
	"context"
	"time"
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
	BoardInReview BoardColumn = "in_review"
	BoardDone     BoardColumn = "done"
	BadgeWaiting  Badge       = "waiting"
)

type WaitingOn string

const (
	WaitingOnSessions WaitingOn = "sessions"
	WaitingOnMonitor  WaitingOn = "monitor"
)

func BoardColumnFor(badge Badge, prState PullRequestState) BoardColumn {
	if ClassForBadge(badge) == AttentionNeedsYou {
		return BoardNeedsYou
	}
	switch badge {
	case BadgeRunning, BadgeHung, BadgeUnhealthy, BadgeWaiting:
		return BoardWorking
	}
	if prState == PullRequestOpen {
		return BoardInReview
	}
	return BoardDone
}
