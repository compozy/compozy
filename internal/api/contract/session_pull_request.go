package contract

import (
	"time"

	"github.com/compozy/compozy/internal/session"
)

type SessionPullRequestChecksPayload struct {
	State       session.ChecksState `json:"state"`
	Passed      int                 `json:"passed"`
	Failed      int                 `json:"failed"`
	Pending     int                 `json:"pending"`
	FailedNames []string            `json:"failed_names"`
}

type SessionPullRequestPayload struct {
	Provider       string                          `json:"provider"`
	Number         int                             `json:"number"`
	URL            string                          `json:"url"`
	State          session.PullRequestState        `json:"state"`
	Draft          *bool                           `json:"draft"`
	HeadSHA        string                          `json:"head_sha"`
	Checks         SessionPullRequestChecksPayload `json:"checks"`
	ReviewDecision session.ReviewDecision          `json:"review_decision"`
	Mergeable      session.Mergeable               `json:"mergeable"`
	ReadyToMerge   bool                            `json:"ready_to_merge"`
}

type SessionPullRequestLookupPayload struct {
	State      session.LookupState `json:"state"`
	Cause      string              `json:"cause"`
	ObservedAt *time.Time          `json:"observed_at"`
	Stale      bool                `json:"stale"`
}

type SessionBoardCountsPayload struct {
	NeedsYou int `json:"needs_you"`
	Working  int `json:"working"`
	Waiting  int `json:"waiting"`
	InReview int `json:"in_review"`
	Done     int `json:"done"`
}

type SessionChildrenPayload struct {
	Total    int `json:"total"`
	NeedsYou int `json:"needs_you"`
	Working  int `json:"working"`
	Waiting  int `json:"waiting"`
	InReview int `json:"in_review"`
	Done     int `json:"done"`
}

type SessionPullRequestResponse struct {
	SessionID         string                           `json:"session_id"`
	Branch            string                           `json:"branch"`
	PullRequest       *SessionPullRequestPayload       `json:"pull_request"`
	PullRequestLookup *SessionPullRequestLookupPayload `json:"pull_request_lookup"`
}

// SessionPullRequestErrorPayload keeps the common CLI error while exposing the
// refresh-specific cause and message to structured clients.
type SessionPullRequestErrorPayload struct {
	ErrorPayload
	Message string `json:"message"`
	Cause   string `json:"cause,omitempty"`
}
