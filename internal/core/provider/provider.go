package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrWatchStatusUnsupported = errors.New("review provider does not support watch status")

type FetchRequest struct {
	PR              string `json:"pr"`
	IncludeNitpicks bool   `json:"include_nitpicks,omitempty"`
}

type WatchStatusState string

const (
	WatchStatusPending         WatchStatusState = "pending"
	WatchStatusStale           WatchStatusState = "stale"
	WatchStatusCurrentReviewed WatchStatusState = "current_reviewed"
	WatchStatusCurrentSettled  WatchStatusState = "current_settled"
	WatchStatusUnsupported     WatchStatusState = "unsupported"
)

type WatchStatusRequest struct {
	PR string `json:"pr"`
}

// WatchStatus reports whether a provider review covers the current PR head.
type WatchStatus struct {
	PRHeadSHA                 string           `json:"pr_head_sha"`
	ReviewCommitSHA           string           `json:"review_commit_sha,omitempty"`
	ReviewID                  string           `json:"review_id,omitempty"`
	ReviewState               string           `json:"review_state,omitempty"`
	ProviderStatusState       string           `json:"provider_status_state,omitempty"`
	ProviderStatusDescription string           `json:"provider_status_description,omitempty"`
	ProviderStatusUpdatedAt   time.Time        `json:"provider_status_updated_at,omitempty"`
	State                     WatchStatusState `json:"state"`
	SubmittedAt               time.Time        `json:"submitted_at,omitempty"`
}

// ReviewItem is the normalized output of a provider fetch operation.
type ReviewItem struct {
	Title       string `json:"title"`
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Author      string `json:"author,omitempty"`
	Body        string `json:"body"`
	ProviderRef string `json:"provider_ref,omitempty"`

	ReviewHash              string `json:"review_hash,omitempty"`
	SourceReviewID          string `json:"source_review_id,omitempty"`
	SourceReviewSubmittedAt string `json:"source_review_submitted_at,omitempty"`
}

// ResolvedIssue identifies an issue file that the agent marked as resolved.
type ResolvedIssue struct {
	FilePath    string `json:"file_path"`
	ProviderRef string `json:"provider_ref,omitempty"`
}

// Provider abstracts review fetching and thread resolution for a specific source.
type Provider interface {
	Name() string
	FetchReviews(ctx context.Context, req FetchRequest) ([]ReviewItem, error)
	ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error
}

// WatchStatusProvider is an optional review-provider capability used by watch mode.
type WatchStatusProvider interface {
	Provider
	WatchStatus(ctx context.Context, req WatchStatusRequest) (WatchStatus, error)
}

func FetchWatchStatus(ctx context.Context, p Provider, req WatchStatusRequest) (WatchStatus, error) {
	watchProvider, ok := p.(WatchStatusProvider)
	if !ok {
		name := "<nil>"
		if p != nil {
			name = strings.TrimSpace(p.Name())
		}
		return WatchStatus{State: WatchStatusUnsupported}, fmt.Errorf("%w: %s", ErrWatchStatusUnsupported, name)
	}
	return watchProvider.WatchStatus(ctx, req)
}
