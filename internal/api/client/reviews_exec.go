package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
)

// FetchReview imports provider feedback into a daemon-backed review round.
func (c *Client) FetchReview(
	ctx context.Context,
	workspace string,
	slug string,
	req apicore.ReviewFetchRequest,
) (apicore.ReviewFetchResult, error) {
	if c == nil {
		return apicore.ReviewFetchResult{}, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return apicore.ReviewFetchResult{}, ErrWorkflowSlugRequired
	}

	response := struct {
		Review apicore.ReviewSummary `json:"review"`
	}{}
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/fetch"
	statusCode, err := c.doJSON(ctx, http.MethodPost, path, map[string]any{
		"workspace": strings.TrimSpace(workspace),
		"provider":  strings.TrimSpace(req.Provider),
		"pr_ref":    strings.TrimSpace(req.PRRef),
		"round":     req.Round,
	}, &response)
	if err != nil {
		return apicore.ReviewFetchResult{}, err
	}
	return apicore.ReviewFetchResult{
		Summary: response.Review,
		Created: statusCode == http.StatusCreated,
	}, nil
}

// GetLatestReview loads the latest review summary for one workflow.
func (c *Client) GetLatestReview(ctx context.Context, workspace string, slug string) (apicore.ReviewSummary, error) {
	if c == nil {
		return apicore.ReviewSummary{}, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return apicore.ReviewSummary{}, ErrWorkflowSlugRequired
	}

	response := struct {
		Review apicore.ReviewSummary `json:"review"`
	}{}
	path := "/api/reviews/" + url.PathEscape(
		trimmedSlug,
	) + "?workspace=" + url.QueryEscape(
		strings.TrimSpace(workspace),
	)
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return apicore.ReviewSummary{}, err
	}
	return response.Review, nil
}

// GetReviewRound loads one daemon-backed review round summary.
func (c *Client) GetReviewRound(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
) (apicore.ReviewRound, error) {
	if c == nil {
		return apicore.ReviewRound{}, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return apicore.ReviewRound{}, ErrWorkflowSlugRequired
	}

	response := struct {
		Round apicore.ReviewRound `json:"round"`
	}{}
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/rounds/" + strconv.Itoa(round) +
		"?workspace=" + url.QueryEscape(strings.TrimSpace(workspace))
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return apicore.ReviewRound{}, err
	}
	return response.Round, nil
}

// ListReviewIssues loads the issue rows for one review round.
func (c *Client) ListReviewIssues(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
) ([]apicore.ReviewIssue, error) {
	if c == nil {
		return nil, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, ErrWorkflowSlugRequired
	}

	response := struct {
		Issues []apicore.ReviewIssue `json:"issues"`
	}{}
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/rounds/" + strconv.Itoa(round) +
		"/issues?workspace=" + url.QueryEscape(strings.TrimSpace(workspace))
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Issues, nil
}

// StartReviewRun starts one daemon-backed review-fix run.
func (c *Client) StartReviewRun(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return apicore.Run{}, ErrWorkflowSlugRequired
	}

	response := struct {
		Run apicore.Run `json:"run"`
	}{}
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/rounds/" + strconv.Itoa(round) + "/runs"
	if _, err := c.doJSON(ctx, http.MethodPost, path, map[string]any{
		"workspace":         strings.TrimSpace(workspace),
		"presentation_mode": strings.TrimSpace(req.PresentationMode),
		"runtime_overrides": req.RuntimeOverrides,
		"batching":          req.Batching,
	}, &response); err != nil {
		return apicore.Run{}, err
	}
	return response.Run, nil
}

// StartExecRun starts one daemon-backed exec run.
func (c *Client) StartExecRun(ctx context.Context, req apicore.ExecRequest) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, ErrDaemonClientRequired
	}

	response := struct {
		Run apicore.Run `json:"run"`
	}{}
	if _, err := c.doJSON(ctx, http.MethodPost, "/api/exec", req, &response); err != nil {
		return apicore.Run{}, err
	}
	return response.Run, nil
}
