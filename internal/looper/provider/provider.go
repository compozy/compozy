package provider

import "context"

// ReviewItem is the normalized output of a provider fetch operation.
type ReviewItem struct {
	Title       string
	File        string
	Line        int
	Severity    string
	Author      string
	Body        string
	ProviderRef string
}

// ResolvedIssue identifies an issue file that the agent marked as resolved.
type ResolvedIssue struct {
	FilePath    string
	ProviderRef string
}

// Provider abstracts review fetching and thread resolution for a specific source.
type Provider interface {
	Name() string
	FetchReviews(ctx context.Context, pr string) ([]ReviewItem, error)
	ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error
}
