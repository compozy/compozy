package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const (
	githubAPIRoot       = "https://api.github.com"
	githubRequestLimit  = 10 * time.Second
	githubResponseLimit = 1 << 20
)

type githubClient struct {
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}

type githubAPIError struct {
	retryAfter *time.Time
	cause      string
	status     int
}

func (e *githubAPIError) Error() string {
	return fmt.Sprintf("github forge: request failed with status %d", e.status)
}

func newGitHubClient() githubClient {
	return githubClient{
		baseURL: githubAPIRoot,
		client: &http.Client{
			Timeout: githubRequestLimit,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("github forge: redirects are disabled")
			},
		},
	}
}

func (c githubClient) request(
	ctx context.Context,
	method string,
	path string,
	token string,
	requestBody any,
	responseBody any,
) error {
	return c.requestWithHeaders(ctx, method, path, token, requestBody, responseBody, nil)
}

func (c githubClient) requestWithHeaders(
	ctx context.Context, method, path, token string, requestBody, responseBody any, headers *http.Header,
) (resultErr error) {
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("github forge: encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL, "/")+path, body)
	if err != nil {
		return fmt.Errorf("github forge: build request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "compozy-forge-github")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	var status int
	var header http.Header
	var payload []byte
	defer func() { c.logCall(ctx, method, path, status, header, payload, resultErr) }()
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("github forge: send request: %w", err)
	}
	status, header = response.StatusCode, response.Header
	defer func() {
		if _, drainErr := io.CopyN(io.Discard, response.Body, 4096); drainErr != nil && !errors.Is(drainErr, io.EOF) {
			slog.DebugContext(ctx, "forge.response_drain", "failed", true)
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("github forge: close response: %w", closeErr))
		}
	}()
	if headers != nil {
		*headers = response.Header.Clone()
	}
	limited := io.LimitReader(response.Body, githubResponseLimit+1)
	payload, err = io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("github forge: read response: %w", err)
	}
	if len(payload) > githubResponseLimit {
		return errors.New("github forge: response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return classifyGitHubResponse(response.StatusCode, response.Header, payload, time.Now())
	}
	if responseBody == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, responseBody); err != nil {
		return fmt.Errorf("github forge: decode response: %w", err)
	}
	return nil
}

// classifyGitHubResponse separates rate limits from other refusals. GitHub answers both primary and
// secondary limits with 403 or 429; a 403 is a rate limit only when its headers or message say so,
// so a permission refusal is not mistaken for one.
func classifyGitHubResponse(status int, header http.Header, payload []byte, now time.Time) error {
	failure := &githubAPIError{status: status}
	switch {
	case status == http.StatusUnauthorized:
		failure.cause = extensioncontract.ForgeCauseCredentialExpired
	case status == http.StatusTooManyRequests ||
		(status == http.StatusForbidden && githubRateLimitSignalled(header, payload)):
		failure.cause = extensioncontract.ForgeCauseRateLimited
		failure.retryAfter = githubRetryAfter(header, now)
	}
	return failure
}

func githubRateLimitSignalled(header http.Header, payload []byte) bool {
	return strings.TrimSpace(header.Get("X-RateLimit-Remaining")) == "0" ||
		strings.TrimSpace(header.Get("Retry-After")) != "" ||
		strings.Contains(strings.ToLower(string(payload)), "rate limit")
}

// githubRetryAfter returns when GitHub accepts requests again. Retry-After, which secondary limits send,
// applies; while the primary quota is exhausted the later of it and X-RateLimit-Reset wins, so a short
// Retry-After never reopens reads early. A secondary limit with quota left uses only Retry-After, and nil
// leaves the wait to the daemon's one-minute fallback, as GitHub advises.
func githubRetryAfter(header http.Header, now time.Time) *time.Time {
	var retryAt *time.Time
	if value := strings.TrimSpace(header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
			retryAt = new(now.Add(time.Duration(seconds) * time.Second).UTC())
		} else if at, err := http.ParseTime(value); err == nil {
			retryAt = new(at.UTC())
		}
	}
	remaining := strings.TrimSpace(header.Get("X-RateLimit-Remaining"))
	reset, err := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64)
	if err != nil || reset <= 0 || (remaining != "" && remaining != "0") {
		return retryAt
	}
	resetAt := time.Unix(reset, 0).UTC()
	if retryAt == nil || resetAt.After(*retryAt) {
		return &resetAt
	}
	return retryAt
}

func repositoryAPIPath(repo repository, suffix string) string {
	return "/repos/" + url.PathEscape(repo.owner) + "/" + url.PathEscape(repo.name) + suffix
}

func pullListPath(repo repository, branch, state string) string {
	query := url.Values{
		"head":     []string{repo.owner + ":" + branch},
		"state":    []string{state},
		"per_page": []string{strconv.Itoa(100)},
	}
	return repositoryAPIPath(repo, "/pulls?") + query.Encode()
}
