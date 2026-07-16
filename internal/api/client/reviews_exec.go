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

	trimmedSlug, err := normalizeClientRouteSlug(slug)
	if err != nil {
		return apicore.ReviewFetchResult{}, err
	}

	var response contract.ReviewFetchResponse
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/fetch"
	statusCode, err := c.doJSON(ctx, http.MethodPost, path, contract.ReviewFetchRequest{
		Workspace: strings.TrimSpace(workspace),
		PackageID: strings.TrimSpace(req.PackageID),
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
	return c.GetLatestReviewForPackage(ctx, workspace, slug, "")
}

// GetLatestReviewForPackage loads the latest review summary scoped by an optional package ID.
func (c *Client) GetLatestReviewForPackage(
	ctx context.Context,
	workspace string,
	slug string,
	packageID string,
) (apicore.ReviewSummary, error) {
	if c == nil {
		return apicore.ReviewSummary{}, ErrDaemonClientRequired
	}

	trimmedSlug, err := normalizeClientRouteSlug(slug)
	if err != nil {
		return apicore.ReviewSummary{}, err
	}

	var response contract.ReviewSummaryResponse
	values := url.Values{"workspace": []string{strings.TrimSpace(workspace)}}
	if selectedPackage := strings.TrimSpace(packageID); selectedPackage != "" {
		values.Set("package_id", selectedPackage)
	}
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "?" + values.Encode()
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
	return c.GetReviewRoundForPackage(ctx, workspace, slug, round, "")
}

// GetReviewRoundForPackage loads a review round scoped by an optional package ID.
func (c *Client) GetReviewRoundForPackage(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
	packageID string,
) (apicore.ReviewRound, error) {
	if c == nil {
		return apicore.ReviewRound{}, ErrDaemonClientRequired
	}

	trimmedSlug, err := normalizeClientRouteSlug(slug)
	if err != nil {
		return apicore.ReviewRound{}, err
	}

	var response contract.ReviewRoundResponse
	values := url.Values{"workspace": []string{strings.TrimSpace(workspace)}}
	if selectedPackage := strings.TrimSpace(packageID); selectedPackage != "" {
		values.Set("package_id", selectedPackage)
	}
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/rounds/" + strconv.Itoa(round) + "?" + values.Encode()
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
	return c.ListReviewIssuesForPackage(ctx, workspace, slug, round, "")
}

// ListReviewIssuesForPackage loads review issues scoped by an optional package ID.
func (c *Client) ListReviewIssuesForPackage(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
	packageID string,
) ([]apicore.ReviewIssue, error) {
	if c == nil {
		return nil, ErrDaemonClientRequired
	}

	trimmedSlug, err := normalizeClientRouteSlug(slug)
	if err != nil {
		return nil, err
	}

	var response contract.ReviewIssuesResponse
	values := url.Values{"workspace": []string{strings.TrimSpace(workspace)}}
	if selectedPackage := strings.TrimSpace(packageID); selectedPackage != "" {
		values.Set("package_id", selectedPackage)
	}
	path := "/api/reviews/" + url.PathEscape(
		trimmedSlug,
	) + "/rounds/" + strconv.Itoa(
		round,
	) + "/issues?" + values.Encode()
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

	trimmedSlug, err := normalizeClientRouteSlug(slug)
	if err != nil {
		return apicore.Run{}, err
	}

	var response contract.RunResponse
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/rounds/" + strconv.Itoa(round) + "/runs"
	if _, err := c.doJSON(ctx, http.MethodPost, path, contract.ReviewRunRequest{
		Workspace:        strings.TrimSpace(workspace),
		PackageID:        strings.TrimSpace(req.PackageID),
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

	trimmedSlug, err := normalizeClientRouteSlug(slug)
	if err != nil {
		return apicore.Run{}, err
	}

	var response contract.RunResponse
	path := "/api/reviews/" + url.PathEscape(trimmedSlug) + "/watch"
	if _, err := c.doJSON(ctx, http.MethodPost, path, contract.ReviewWatchRequest{
		Workspace:        strings.TrimSpace(workspace),
		PackageID:        strings.TrimSpace(req.PackageID),
		PresentationMode: strings.TrimSpace(req.PresentationMode),
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
