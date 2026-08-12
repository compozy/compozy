package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
}

type githubAPIError struct {
	cause  string
	status int
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
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("github forge: send request: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("github forge: close response: %w", closeErr))
		}
	}()
	limited := io.LimitReader(response.Body, githubResponseLimit+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("github forge: read response: %w", err)
	}
	if len(payload) > githubResponseLimit {
		return errors.New("github forge: response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return classifyGitHubStatus(response.StatusCode)
	}
	if responseBody == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, responseBody); err != nil {
		return fmt.Errorf("github forge: decode response: %w", err)
	}
	return nil
}

func classifyGitHubStatus(status int) error {
	cause := ""
	switch status {
	case http.StatusUnauthorized:
		cause = extensioncontract.ForgeCauseCredentialExpired
	case http.StatusForbidden, http.StatusTooManyRequests:
		cause = extensioncontract.ForgeCauseRateLimited
	}
	return &githubAPIError{cause: cause, status: status}
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
