package devcycle

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type codeRabbitFetchInput struct {
	PR              any  `json:"pr"`
	IncludeNitpicks bool `json:"include_nitpicks,omitempty"`
}

func (i *codeRabbitFetchInput) UnmarshalJSON(raw []byte) error {
	type fetchInputWire struct {
		PR              any `json:"pr"`
		IncludeNitpicks any `json:"include_nitpicks,omitempty"`
	}
	var payload fetchInputWire
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	includeNitpicks, err := decodeBoolLike(payload.IncludeNitpicks)
	if err != nil {
		return fmt.Errorf("include_nitpicks: %w", err)
	}
	i.PR = payload.PR
	i.IncludeNitpicks = includeNitpicks
	return nil
}

func decodeBoolLike(value any) (bool, error) {
	switch typed := value.(type) {
	case nil:
		return false, nil
	case bool:
		return typed, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return false, nil
		}
		parsed, err := strconv.ParseBool(trimmed)
		if err != nil {
			return false, err
		}
		return parsed, nil
	default:
		return false, fmt.Errorf("unsupported boolean value %T", value)
	}
}

type codeRabbitResolveInput struct {
	PR      any                  `json:"pr"`
	Issues  []codeRabbitIssue    `json:"issues,omitempty"`
	Results []codeRabbitFixEntry `json:"results,omitempty"`
}

type codeRabbitWatchSpec struct {
	Kind        string `json:"kind"`
	PR          any    `json:"pr"`
	QuietPeriod string `json:"quiet_period,omitempty"`
}

type codeRabbitIssue struct {
	ID                      string `json:"id"`
	Title                   string `json:"title"`
	Body                    string `json:"body,omitempty"`
	BodyRef                 string `json:"body_ref,omitempty"`
	File                    string `json:"file,omitempty"`
	Line                    int    `json:"line,omitempty"`
	Severity                string `json:"severity,omitempty"`
	Author                  string `json:"author,omitempty"`
	ProviderRef             string `json:"provider_ref,omitempty"`
	ReviewHash              string `json:"review_hash,omitempty"`
	SourceReviewID          string `json:"source_review_id,omitempty"`
	SourceReviewSubmittedAt string `json:"source_review_submitted_at,omitempty"`
}

type codeRabbitFixEntry struct {
	ID          string `json:"id"`
	Triage      string `json:"triage,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	ProviderRef string `json:"provider_ref,omitempty"`
}

type codeRabbitFetchOutput struct {
	PR              string            `json:"pr"`
	UnresolvedCount int               `json:"unresolved_count"`
	Issues          []codeRabbitIssue `json:"issues"`
}

type codeRabbitResolveOutput struct {
	PR              string   `json:"pr"`
	ResolvedCount   int      `json:"resolved_count"`
	ResolvedThreads []string `json:"resolved_threads"`
}

type codeRabbitWatchPayload struct {
	PR            string            `json:"pr"`
	Review        codeRabbitReview  `json:"review"`
	ProviderState codeRabbitStatus  `json:"provider_status"`
	SubmittedAt   *time.Time        `json:"submitted_at,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type codeRabbitReview struct {
	HeadSHA         string `json:"head_sha"`
	LocalHeadSHA    string `json:"local_head_sha,omitempty"`
	ReviewCommitSHA string `json:"review_commit_sha,omitempty"`
	ReviewID        string `json:"review_id,omitempty"`
	ReviewState     string `json:"review_state"`
}

type codeRabbitStatus struct {
	State         string     `json:"state"`
	ProviderState string     `json:"provider_state,omitempty"`
	Description   string     `json:"description,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

type graphQLResponse struct {
	Data   graphQLData    `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

type graphQLData struct {
	Repository graphQLRepository `json:"repository"`
}

type graphQLRepository struct {
	PullRequest graphQLPullRequest `json:"pullRequest"`
}

type graphQLPullRequest struct {
	HeadRefOid    string            `json:"headRefOid"`
	ReviewThreads graphQLThreadList `json:"reviewThreads"`
}

type graphQLThreadList struct {
	Nodes []graphQLThread `json:"nodes"`
}

type graphQLThread struct {
	ID         string          `json:"id"`
	IsResolved bool            `json:"isResolved"`
	Comments   graphQLComments `json:"comments"`
}

type graphQLComments struct {
	Nodes []graphQLComment `json:"nodes"`
}

type graphQLComment struct {
	Body      string        `json:"body"`
	Path      string        `json:"path"`
	Line      *int          `json:"line"`
	Author    graphQLAuthor `json:"author"`
	CreatedAt *time.Time    `json:"createdAt"`
}

type graphQLAuthor struct {
	Login string `json:"login"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type pullRequestReview struct {
	ID          int    `json:"id"`
	Body        string `json:"body"`
	CommitID    string `json:"commit_id"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

type commitStatus struct {
	State       string `json:"state"`
	Description string `json:"description"`
	Context     string `json:"context"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}
