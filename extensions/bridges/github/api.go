package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/compozy/agh/internal/bridgesdk"
)

const (
	apiBodyKey = "body"
)

type githubAPI interface {
	ValidateAuth(context.Context, int64) (*githubViewer, error)
	CreateIssueComment(context.Context, int64, string, int64) (*githubIssueComment, error)
	CreateReviewCommentReply(context.Context, int64, int64, string, int64) (*githubReviewComment, error)
	UpdateIssueComment(context.Context, int64, string, int64) (*githubIssueComment, error)
	UpdateReviewComment(context.Context, int64, string, int64) (*githubReviewComment, error)
	DeleteIssueComment(context.Context, int64, int64) error
	DeleteReviewComment(context.Context, int64, int64) error
}

type githubViewer struct {
	ID    int64  `json:"id,omitempty"`
	Login string `json:"login,omitempty"`
}

type githubHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type githubClient struct {
	cfg        resolvedInstanceConfig
	httpClient githubHTTPDoer
	now        func() time.Time

	mu                   sync.Mutex
	cachedInstallationID int64
	installationToken    string
	tokenExpiresAt       time.Time
}

type githubAccessTokenResponse struct {
	Token     string `json:"token,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

func validateGitHubAppCredentials(cfg *resolvedInstanceConfig) error {
	if cfg == nil {
		return errors.New("github: config is required")
	}
	if strings.TrimSpace(cfg.appID) == "" {
		return errors.New("github: app_id is required")
	}
	if _, err := strconv.ParseInt(strings.TrimSpace(cfg.appID), 10, 64); err != nil {
		return fmt.Errorf("github: app_id must be numeric: %w", err)
	}
	if _, err := parseGitHubPrivateKey(cfg.privateKey); err != nil {
		return err
	}
	return nil
}

func (c *githubClient) ValidateAuth(ctx context.Context, installationID int64) (*githubViewer, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/user", nil, installationID)
	if err != nil {
		return nil, err
	}
	viewer := githubViewer{}
	if err := c.doJSON(req, &viewer, bridgesdk.HTTPResponseNoCommit); err != nil {
		return nil, err
	}
	return &viewer, nil
}

func (c *githubClient) CreateIssueComment(
	ctx context.Context,
	issueNumber int64,
	body string,
	installationID int64,
) (*githubIssueComment, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", c.cfg.repoOwner, c.cfg.repoName, issueNumber),
		map[string]any{
			apiBodyKey: body,
		},
		installationID,
	)
	if err != nil {
		return nil, err
	}
	comment := githubIssueComment{}
	if err := c.doJSON(req, &comment, bridgesdk.HTTPResponseCommitOnSuccessStatus); err != nil {
		return nil, err
	}
	if comment.ID <= 0 {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("github: create issue comment response omitted id"))
	}
	return &comment, nil
}

func (c *githubClient) CreateReviewCommentReply(
	ctx context.Context,
	pullNumber int64,
	commentID int64,
	body string,
	installationID int64,
) (*githubReviewComment, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodPost,
		fmt.Sprintf(
			"/repos/%s/%s/pulls/%d/comments/%d/replies",
			c.cfg.repoOwner,
			c.cfg.repoName,
			pullNumber,
			commentID,
		),
		map[string]any{
			apiBodyKey: body,
		},
		installationID,
	)
	if err != nil {
		return nil, err
	}
	comment := githubReviewComment{}
	if err := c.doJSON(req, &comment, bridgesdk.HTTPResponseCommitOnSuccessStatus); err != nil {
		return nil, err
	}
	if comment.ID <= 0 {
		return nil, bridgesdk.MarkCommittedMutation(
			errors.New("github: create review comment reply response omitted id"),
		)
	}
	return &comment, nil
}

func (c *githubClient) UpdateIssueComment(
	ctx context.Context,
	commentID int64,
	body string,
	installationID int64,
) (*githubIssueComment, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodPatch,
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d", c.cfg.repoOwner, c.cfg.repoName, commentID),
		map[string]any{
			apiBodyKey: body,
		},
		installationID,
	)
	if err != nil {
		return nil, err
	}
	comment := githubIssueComment{}
	if err := c.doJSON(req, &comment, bridgesdk.HTTPResponseCommitOnSuccessStatus); err != nil {
		return nil, err
	}
	if comment.ID <= 0 {
		comment.ID = commentID
	}
	return &comment, nil
}

func (c *githubClient) UpdateReviewComment(
	ctx context.Context,
	commentID int64,
	body string,
	installationID int64,
) (*githubReviewComment, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodPatch,
		fmt.Sprintf("/repos/%s/%s/pulls/comments/%d", c.cfg.repoOwner, c.cfg.repoName, commentID),
		map[string]any{
			apiBodyKey: body,
		},
		installationID,
	)
	if err != nil {
		return nil, err
	}
	comment := githubReviewComment{}
	if err := c.doJSON(req, &comment, bridgesdk.HTTPResponseCommitOnSuccessStatus); err != nil {
		return nil, err
	}
	if comment.ID <= 0 {
		comment.ID = commentID
	}
	return &comment, nil
}

func (c *githubClient) DeleteIssueComment(ctx context.Context, commentID int64, installationID int64) error {
	req, err := c.newRequest(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d", c.cfg.repoOwner, c.cfg.repoName, commentID),
		nil,
		installationID,
	)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil, bridgesdk.HTTPResponseCommitOnSuccessStatus)
}

func (c *githubClient) DeleteReviewComment(ctx context.Context, commentID int64, installationID int64) error {
	req, err := c.newRequest(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("/repos/%s/%s/pulls/comments/%d", c.cfg.repoOwner, c.cfg.repoName, commentID),
		nil,
		installationID,
	)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil, bridgesdk.HTTPResponseCommitOnSuccessStatus)
}

func (c *githubClient) newRequest(
	ctx context.Context,
	method string,
	path string,
	body any,
	installationID int64,
) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	endpoint, err := joinGitHubURL(c.cfg.apiBaseURL, path)
	if err != nil {
		return nil, err
	}
	if err := validateGitHubEndpointURL(endpoint); err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		payload, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, marshalErr
		}
		reader = strings.NewReader(string(payload))
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agh-bridge-github/0.1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	authHeader, err := c.authHeader(ctx, installationID)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authHeader)
	return req, nil
}

func (c *githubClient) authHeader(ctx context.Context, installationID int64) (string, error) {
	switch c.cfg.mode {
	case githubModePAT:
		if strings.TrimSpace(c.cfg.token) == "" {
			return "", &bridgesdk.AuthError{Err: errors.New("github: PAT token is empty")}
		}
		return "Bearer " + strings.TrimSpace(c.cfg.token), nil
	case githubModeApp:
		token, err := c.installationAccessToken(ctx, installationID)
		if err != nil {
			return "", err
		}
		return "Bearer " + token, nil
	default:
		return "", &bridgesdk.AuthError{Err: fmt.Errorf("github: unsupported auth mode %q", c.cfg.mode)}
	}
}

func (c *githubClient) installationAccessToken(ctx context.Context, installationID int64) (string, error) {
	if installationID <= 0 {
		return "", &bridgesdk.AuthError{Err: errors.New("github: app mode requires installation id")}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	if c.now != nil {
		now = c.now().UTC()
	}
	if c.cachedInstallationID == installationID &&
		strings.TrimSpace(c.installationToken) != "" &&
		now.Add(30*time.Second).Before(c.tokenExpiresAt) {
		return c.installationToken, nil
	}

	jwtToken, err := signGitHubAppJWT(c.cfg.appID, c.cfg.privateKey, now)
	if err != nil {
		return "", &bridgesdk.AuthError{Err: err}
	}

	endpoint, err := joinGitHubURL(c.cfg.apiBaseURL, fmt.Sprintf("/app/installations/%d/access_tokens", installationID))
	if err != nil {
		return "", err
	}
	if err := validateGitHubEndpointURL(endpoint); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("User-Agent", "agh-bridge-github/0.1.0")

	response := githubAccessTokenResponse{}
	if err := c.doJSON(req, &response, bridgesdk.HTTPResponseNoCommit); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Token) == "" {
		return "", &bridgesdk.AuthError{Err: errors.New("github: installation token response omitted token")}
	}

	expiresAt := now.Add(50 * time.Minute)
	if parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(response.ExpiresAt)); parseErr == nil {
		expiresAt = parsed.UTC()
	}

	c.cachedInstallationID = installationID
	c.installationToken = strings.TrimSpace(response.Token)
	c.tokenExpiresAt = expiresAt
	return c.installationToken, nil
}

func signGitHubAppJWT(appID string, privateKeyPEM string, now time.Time) (string, error) {
	appNumericID, err := strconv.ParseInt(strings.TrimSpace(appID), 10, 64)
	if err != nil {
		return "", fmt.Errorf("github: parse app_id: %w", err)
	}
	privateKey, err := parseGitHubPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(appNumericID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("github: sign app jwt: %w", err)
	}
	return signed, nil
}

func parseGitHubPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, errors.New("github: private_key must be PEM encoded")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("github: parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github: private_key must contain an RSA key")
	}
	return key, nil
}

func joinGitHubURL(base string, path string) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}

func validateGitHubRequestURL(req *http.Request) error {
	if req == nil || req.URL == nil {
		return errors.New("github: request url is required")
	}
	return validateGitHubEndpointURL(req.URL.String())
}

func validateGitHubEndpointURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("github: parse api url: %w", err)
	}
	if parsed == nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return errors.New("github: api url host is required")
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return nil
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return fmt.Errorf("github: api url scheme %q is not allowed", parsed.Scheme)
	}

	host := strings.TrimSpace(parsed.Hostname())
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("github: insecure api host %q is not allowed", host)
}

func classifyGitHubHTTPError(statusCode int, retryAfterHeader string, raw string) error {
	message := strings.TrimSpace(firstNonEmpty(extractGitHubErrorMessage(raw), raw, http.StatusText(statusCode)))
	switch {
	case statusCode == http.StatusUnauthorized:
		return &bridgesdk.AuthError{Err: errors.New(message)}
	case statusCode == http.StatusForbidden && strings.Contains(strings.ToLower(message), "rate limit"):
		return &bridgesdk.RateLimitError{Err: errors.New(message), RetryAfter: parseRetryAfter(retryAfterHeader)}
	case statusCode == http.StatusTooManyRequests:
		return &bridgesdk.RateLimitError{Err: errors.New(message), RetryAfter: parseRetryAfter(retryAfterHeader)}
	case statusCode == http.StatusForbidden:
		return &bridgesdk.AuthError{Err: errors.New(message)}
	case statusCode >= 500:
		return &bridgesdk.TransientError{Err: errors.New(message)}
	default:
		return &bridgesdk.PermanentError{Err: errors.New(message)}
	}
}

func extractGitHubErrorMessage(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	payload := struct {
		Message string `json:"message,omitempty"`
		Error   string `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return firstNonEmpty(payload.Message, payload.Error)
}
