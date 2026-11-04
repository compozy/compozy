package contract

import "time"

const (
	ForgeCauseRateLimited = "rate_limited"
	// #nosec G101 -- these are public error cause identifiers, not credentials.
	ForgeCauseCredentialExpired = "credential_expired"
	ForgeCauseUnsupportedRemote = "unsupported_remote"
	// #nosec G101 -- these are public error cause identifiers, not credentials.
	ForgeCauseCredentialAbsent = "credential_absent"
)

// Forge review states carried by ForgeReview.State.
const (
	ForgeReviewApproved         = "approved"
	ForgeReviewChangesRequested = "changes_requested"
	ForgeReviewCommented        = "commented"
	ForgeReviewDismissed        = "dismissed"
)

// Bounds on the facts one status response may carry.
const (
	MaxForgeFailedNames = 64
	MaxForgeReviews     = 100
	MaxForgeThreads     = 100
)

type ForgeCapabilitiesRequest struct {
	RemoteURLs []string `json:"remote_urls"`
}

type ForgeCapabilitiesResponse struct {
	SupportsPRDiscovery bool     `json:"supports_pr_discovery,omitempty"`
	Served              bool     `json:"served"`
	Available           bool     `json:"available"`
	Winner              string   `json:"winner,omitempty"`
	Provider            string   `json:"provider,omitempty"`
	ServedRemote        string   `json:"served_remote,omitempty"`
	RequestNoun         string   `json:"request_noun,omitempty"`
	OpenActionLabel     string   `json:"open_action_label,omitempty"`
	ViewActionLabel     string   `json:"view_action_label,omitempty"`
	SupportsDraft       bool     `json:"supports_draft,omitempty"`
	CompareURLTemplate  string   `json:"compare_url_template,omitempty"`
	TemplatePaths       []string `json:"template_paths,omitempty"`
	CredentialSource    string   `json:"credential_source,omitempty"`
	DefaultBranch       string   `json:"default_branch,omitempty"`
	Cause               string   `json:"cause,omitempty"`
	// RetryAfter is when a rate-limited provider may be asked again; it accompanies Cause rate_limited.
	RetryAfter *time.Time `json:"retry_after,omitempty"`
}

type ForgeStatusRequest struct {
	RemoteURLs []string `json:"remote_urls"`
	Branch     string   `json:"branch"`
	Base       string   `json:"base,omitempty"`
	HeadSHA    string   `json:"head_sha,omitempty"`
}

type ForgeChecks struct {
	State       string   `json:"state"`
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Pending     int      `json:"pending"`
	FailedNames []string `json:"failed_names"`
}

// ForgeReview is one submitted review. It never carries the review body.
type ForgeReview struct {
	ID          string     `json:"id"`
	Author      string     `json:"author,omitempty"`
	IsBot       bool       `json:"is_bot,omitempty"`
	State       string     `json:"state"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	Commit      string     `json:"commit,omitempty"`
}

// ForgeReviewThread is one unresolved review thread. Commit is the commit the thread was opened on, which
// stays fixed when the head moves; empty means the provider cannot tell.
type ForgeReviewThread struct {
	ID     string `json:"id"`
	Commit string `json:"commit,omitempty"`
}

// ForgeReviews is optional review activity. A provider that omits it still
// serves checks; consumers report reviews as not observable.
type ForgeReviews struct {
	Items []ForgeReview `json:"items"`
	// Threads lists unresolved threads, newest last. UnresolvedThreads is their total, which may exceed
	// this list only when Truncated is set.
	Threads           []ForgeReviewThread `json:"threads,omitempty"`
	UnresolvedThreads int                 `json:"unresolved_threads"`
	// Truncated reports that the provider stopped before reading every review or thread.
	Truncated bool `json:"truncated,omitempty"`
}

type ForgeStatusResponse struct {
	Title   string        `json:"title,omitempty"`
	Checks  *ForgeChecks  `json:"checks,omitempty"`
	Reviews *ForgeReviews `json:"reviews,omitempty"`
	// ReviewsUnavailable reports a provider that serves reviews but failed to read them this time. It is
	// distinct from omitting Reviews, which means the provider cannot observe them at all.
	ReviewsUnavailable bool       `json:"reviews_unavailable,omitempty"`
	ReviewDecision     string     `json:"review_decision,omitempty"`
	Mergeable          string     `json:"mergeable,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	Selection          string     `json:"selection,omitempty"`
	RetryAfter         *time.Time `json:"retry_after,omitempty"`
	Provider           string     `json:"provider"`
	PRNumber           *int       `json:"pr_number,omitempty"`
	PRState            *string    `json:"pr_state,omitempty"`
	PRURL              string     `json:"pr_url,omitempty"`
	Head               string     `json:"head,omitempty"`
	Base               string     `json:"base,omitempty"`
	HeadSHA            string     `json:"head_sha,omitempty"`
	Draft              *bool      `json:"draft,omitempty"`
	Merged             *bool      `json:"merged,omitempty"`
	FetchedAt          time.Time  `json:"fetched_at"`
	Cause              string     `json:"cause,omitempty"`
}

type ForgePRCreateRequest struct {
	RemoteURLs []string `json:"remote_urls"`
	Head       string   `json:"head"`
	HeadSHA    string   `json:"head_sha,omitempty"`
	Base       string   `json:"base"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Draft      bool     `json:"draft,omitempty"`
}

type ForgePRCreateResponse struct {
	Status string `json:"status"`
	Number int    `json:"number,omitempty"`
	URL    string `json:"url,omitempty"`
	Cause  string `json:"cause,omitempty"`
}

// ForgePRSearchRequest is an optional paged discovery operation. Providers scope
// every result to the repository selected by RemoteURLs.
type ForgePRSearchRequest struct {
	RemoteURLs []string `json:"remote_urls"`
	Query      string   `json:"query,omitempty"`
	Page       int      `json:"page,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}
type ForgePRReadRequest struct {
	RemoteURLs []string `json:"remote_urls"`
	Number     int      `json:"number"`
}
type ForgePRCandidate struct {
	HeadRepository string `json:"head_repository,omitempty"`
	Number         int    `json:"number"`
	Title          string `json:"title,omitempty"`
	URL            string `json:"url"`
	Branch         string `json:"branch"`
	State          string `json:"state"`
}
type ForgePRSearchResponse struct {
	Items   []ForgePRCandidate `json:"items"`
	HasMore bool               `json:"has_more"`
}
