package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
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

	var response contract.ReviewFetchResponse
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/fetch"
	statusCode, err := c.doJSON(ctx, http.MethodPost, path, contract.ReviewFetchRequest{
		Workspace: strings.TrimSpace(workspace),
		Provider:  strings.TrimSpace(req.Provider),
		PRRef:     strings.TrimSpace(req.PRRef),
		Round:     req.Round,
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

	var response contract.ReviewSummaryResponse
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

	var response contract.ReviewRoundResponse
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

	var response contract.ReviewIssuesResponse
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

	var response contract.RunResponse
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/rounds/" + strconv.Itoa(round) + "/runs"
	if _, err := c.doJSON(ctx, http.MethodPost, path, contract.ReviewRunRequest{
		Workspace:        strings.TrimSpace(workspace),
		PresentationMode: strings.TrimSpace(req.PresentationMode),
		RuntimeOverrides: req.RuntimeOverrides,
		Batching:         req.Batching,
	}, &response); err != nil {
		return apicore.Run{}, err
	}
	return response.Run, nil
}

// StartReviewWatch starts one daemon-owned review-watch parent run.
func (c *Client) StartReviewWatch(
	ctx context.Context,
	workspace string,
	slug string,
	req apicore.ReviewWatchRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, ErrDaemonClientRequired
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return apicore.Run{}, ErrWorkflowSlugRequired
	}

	var response contract.RunResponse
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/watch"
	if _, err := c.doJSON(ctx, http.MethodPost, path, contract.ReviewWatchRequest{
		Workspace:        strings.TrimSpace(workspace),
		Provider:         strings.TrimSpace(req.Provider),
		PRRef:            strings.TrimSpace(req.PRRef),
		UntilClean:       req.UntilClean,
		MaxRounds:        req.MaxRounds,
		AutoPush:         req.AutoPush,
		PushRemote:       strings.TrimSpace(req.PushRemote),
		PushBranch:       strings.TrimSpace(req.PushBranch),
		PollInterval:     strings.TrimSpace(req.PollInterval),
		ReviewTimeout:    strings.TrimSpace(req.ReviewTimeout),
		QuietPeriod:      strings.TrimSpace(req.QuietPeriod),
		RuntimeOverrides: req.RuntimeOverrides,
		Batching:         req.Batching,
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

	var response contract.RunResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/api/exec", req, &response); err != nil {
		return apicore.Run{}, err
	}
	return response.Run, nil
}
