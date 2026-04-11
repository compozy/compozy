package provider

import "context"

type FetchRequest struct {
	PR              string `json:"pr"`
	IncludeNitpicks bool   `json:"include_nitpicks,omitempty"`
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
