package provider

import "context"

type FetchRequest struct {
	PR              string
	IncludeNitpicks bool
}

// ReviewItem is the normalized output of a provider fetch operation.
type ReviewItem struct {
	Title       string
	File        string
	Line        int
	Severity    string
	Author      string
	Body        string
	ProviderRef string

	ReviewHash              string
	SourceReviewID          string
	SourceReviewSubmittedAt string
}

// ResolvedIssue identifies an issue file that the agent marked as resolved.
type ResolvedIssue struct {
	FilePath    string
	ProviderRef string
}

// Provider abstracts review fetching and thread resolution for a specific source.
type Provider interface {
	Name() string
	FetchReviews(ctx context.Context, req FetchRequest) ([]ReviewItem, error)
	ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error
}
