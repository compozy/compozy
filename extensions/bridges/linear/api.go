package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
)

const (
	apiBodyKey      = "body"
	apiIssueIDValue = "issueId"
)

type linearAPI interface {
	ValidateAuth(ctx context.Context) (*linearViewer, error)
	CreateComment(ctx context.Context, issueID string, body string, parentID string) (*linearComment, error)
	UpdateComment(ctx context.Context, commentID string, body string) (*linearComment, error)
	DeleteComment(ctx context.Context, commentID string) error
	CreateAgentActivity(ctx context.Context, agentSessionID string, body string) (*linearAgentActivity, error)
}

type linearClient struct {
	cfg        resolvedInstanceConfig
	httpClient *http.Client
	now        func() time.Time
}

type linearViewer struct {
	ID             string
	DisplayName    string
	OrganizationID string
}

type linearComment struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	ParentID  string    `json:"parentId"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Issue     struct {
		ID string `json:"id"`
	} `json:"issue"`
}

type linearAgentActivity struct {
	ID            string `json:"id"`
	SourceComment *struct {
		ID string `json:"id"`
	} `json:"sourceComment"`
}

type linearOAuthTokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type linearOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type linearGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type linearGraphQLError struct {
	Message string `json:"message"`
}

type linearGraphQLResponse[T any] struct {
	Data   T                    `json:"data"`
	Errors []linearGraphQLError `json:"errors,omitempty"`
}

func (c *linearClient) ValidateAuth(ctx context.Context) (*linearViewer, error) {
	type viewerResponse struct {
		Viewer struct {
			ID           string `json:"id"`
			DisplayName  string `json:"displayName"`
			Organization struct {
				ID string `json:"id"`
			} `json:"organization"`
		} `json:"viewer"`
	}

	response, err := doLinearGraphQL[viewerResponse](ctx, c, linearGraphQLRequest{
		Query: `
query LinearProviderViewer {
  viewer {
    id
    displayName
    organization {
      id
    }
  }
}`,
	}, bridgesdk.HTTPResponseNoCommit)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Viewer.ID) == "" {
		return nil, &bridgesdk.AuthError{Err: errors.New("linear: viewer id is missing")}
	}
	if strings.TrimSpace(response.Viewer.Organization.ID) == "" {
		return nil, &bridgesdk.AuthError{Err: errors.New("linear: viewer organization id is missing")}
	}
	return &linearViewer{
		ID:             strings.TrimSpace(response.Viewer.ID),
		DisplayName:    strings.TrimSpace(response.Viewer.DisplayName),
		OrganizationID: strings.TrimSpace(response.Viewer.Organization.ID),
	}, nil
}

func (c *linearClient) CreateComment(
	ctx context.Context,
	issueID string,
	body string,
	parentID string,
) (*linearComment, error) {
	type createCommentResponse struct {
		CommentCreate struct {
			Success *bool         `json:"success"`
			Comment linearComment `json:"comment"`
		} `json:"commentCreate"`
	}

	variables := map[string]any{
		apiIssueIDValue: strings.TrimSpace(issueID),
		apiBodyKey:      body,
	}
	if strings.TrimSpace(parentID) != "" {
		variables["parentId"] = strings.TrimSpace(parentID)
	}
	response, err := doLinearGraphQL[createCommentResponse](ctx, c, linearGraphQLRequest{
		Query: `
mutation LinearProviderCreateComment($issueId: String!, $body: String!, $parentId: String) {
  commentCreate(input: { issueId: $issueId, body: $body, parentId: $parentId }) {
    success
    comment {
      id
      body
      parentId
      url
      createdAt
      updatedAt
      issue {
        id
      }
    }
  }
}`,
		Variables: variables,
	}, bridgesdk.HTTPResponseCommitOnSuccessStatus)
	if err != nil {
		return nil, err
	}
	if response.CommentCreate.Success == nil {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("linear: comment creation response omitted success"))
	}
	if !*response.CommentCreate.Success {
		return nil, &bridgesdk.PermanentError{Err: errors.New("linear: comment creation failed")}
	}
	if strings.TrimSpace(response.CommentCreate.Comment.ID) == "" {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("linear: comment creation response omitted id"))
	}
	comment := response.CommentCreate.Comment
	return &comment, nil
}

func (c *linearClient) UpdateComment(ctx context.Context, commentID string, body string) (*linearComment, error) {
	type updateCommentResponse struct {
		CommentUpdate struct {
			Success *bool         `json:"success"`
			Comment linearComment `json:"comment"`
		} `json:"commentUpdate"`
	}

	response, err := doLinearGraphQL[updateCommentResponse](ctx, c, linearGraphQLRequest{
		Query: `
mutation LinearProviderUpdateComment($id: String!, $body: String!) {
  commentUpdate(id: $id, input: { body: $body }) {
    success
    comment {
      id
      body
      parentId
      url
      createdAt
      updatedAt
      issue {
        id
      }
    }
  }
}`,
		Variables: map[string]any{
			"id":       strings.TrimSpace(commentID),
			apiBodyKey: body,
		},
	}, bridgesdk.HTTPResponseCommitOnSuccessStatus)
	if err != nil {
		return nil, err
	}
	if response.CommentUpdate.Success == nil {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("linear: comment update response omitted success"))
	}
	if !*response.CommentUpdate.Success {
		return nil, &bridgesdk.PermanentError{Err: errors.New("linear: comment update failed")}
	}
	if strings.TrimSpace(response.CommentUpdate.Comment.ID) == "" {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("linear: comment update response omitted id"))
	}
	comment := response.CommentUpdate.Comment
	return &comment, nil
}

func (c *linearClient) DeleteComment(ctx context.Context, commentID string) error {
	type deleteCommentResponse struct {
		CommentDelete struct {
			Success *bool `json:"success"`
		} `json:"commentDelete"`
	}

	response, err := doLinearGraphQL[deleteCommentResponse](ctx, c, linearGraphQLRequest{
		Query: `
mutation LinearProviderDeleteComment($id: String!) {
  commentDelete(id: $id) {
    success
  }
}`,
		Variables: map[string]any{
			"id": strings.TrimSpace(commentID),
		},
	}, bridgesdk.HTTPResponseCommitOnSuccessStatus)
	if err != nil {
		return err
	}
	if response.CommentDelete.Success == nil {
		return bridgesdk.MarkCommittedMutation(errors.New("linear: comment delete response omitted success"))
	}
	if !*response.CommentDelete.Success {
		return &bridgesdk.PermanentError{Err: errors.New("linear: comment delete failed")}
	}
	return nil
}

func (c *linearClient) CreateAgentActivity(
	ctx context.Context,
	agentSessionID string,
	body string,
) (*linearAgentActivity, error) {
	type createActivityResponse struct {
		AgentActivityCreate struct {
			Success       *bool               `json:"success"`
			AgentActivity linearAgentActivity `json:"agentActivity"`
		} `json:"agentActivityCreate"`
	}

	response, err := doLinearGraphQL[createActivityResponse](ctx, c, linearGraphQLRequest{
		Query: `
mutation LinearProviderCreateAgentActivity($agentSessionId: String!, $body: String!) {
  agentActivityCreate(input: {
    agentSessionId: $agentSessionId
    content: {
      type: response
      body: $body
    }
  }) {
    success
    agentActivity {
      id
      sourceComment {
        id
      }
    }
  }
}`,
		Variables: map[string]any{
			"agentSessionId": strings.TrimSpace(agentSessionID),
			apiBodyKey:       body,
		},
	}, bridgesdk.HTTPResponseCommitOnSuccessStatus)
	if err != nil {
		return nil, err
	}
	if response.AgentActivityCreate.Success == nil {
		return nil, bridgesdk.MarkCommittedMutation(
			errors.New("linear: agent activity creation response omitted success"),
		)
	}
	if !*response.AgentActivityCreate.Success {
		return nil, &bridgesdk.PermanentError{Err: errors.New("linear: agent activity creation failed")}
	}
	if strings.TrimSpace(response.AgentActivityCreate.AgentActivity.ID) == "" {
		return nil, bridgesdk.MarkCommittedMutation(
			errors.New("linear: agent activity creation response omitted id"),
		)
	}
	activity := response.AgentActivityCreate.AgentActivity
	return &activity, nil
}

func (c *linearClient) authToken(ctx context.Context) string {
	token, err := c.authTokenForRequest(ctx)
	if err != nil {
		return ""
	}
	return token
}

func (c *linearClient) authTokenForRequest(ctx context.Context) (string, error) {
	if c.cfg.authMode != linearAuthModeOAuth {
		return c.cfg.apiKey, nil
	}
	return c.ensureOAuthToken(ctx)
}

func (c *linearClient) ensureOAuthToken(ctx context.Context) (result string, err error) {
	cache := c.cfg.oauthTokenCache
	if cache == nil {
		return "", &bridgesdk.PermanentError{Err: errors.New("linear: oauth token cache is required")}
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := c.now()
	if strings.TrimSpace(cache.token) != "" &&
		(cache.expiresAt.IsZero() || cache.expiresAt.After(now.Add(time.Minute))) {
		return cache.token, nil
	}

	values := url.Values{}
	values.Set("grant_type", "client_credentials")
	values.Set("client_id", c.cfg.clientID)
	values.Set("client_secret", c.cfg.clientSecret)
	values.Set("scope", strings.Join(defaultLinearOAuthScopes(c.cfg.mode), ","))

	if !validLinearCredentialedURL(c.cfg.oauthTokenURL) {
		clearLinearOAuthTokenCache(cache)
		return "", &bridgesdk.PermanentError{Err: errors.New("linear: oauth token url is invalid")}
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.cfg.oauthTokenURL,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		clearLinearOAuthTokenCache(cache)
		return "", fmt.Errorf("linear: build oauth token request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpResponse, err := bridgesdk.CredentialedHTTPClient(c.httpClient).Do(httpRequest)
	if err != nil {
		clearLinearOAuthTokenCache(cache)
		return "", classifyLinearTransportError(err)
	}
	defer func() {
		err = bridgesdk.FinalizeHTTPResponseBody(
			httpResponse.Body,
			err,
			bridgesdk.HTTPResponseCommitUnconfirmed,
			nil,
		)
	}()

	body, readErr := readLinearHTTPResponseBody(httpResponse, "oauth token")
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		clearLinearOAuthTokenCache(cache)
		return "", errors.Join(classifyLinearHTTPError(httpResponse.StatusCode, body), readErr)
	}
	if readErr != nil {
		clearLinearOAuthTokenCache(cache)
		return "", readErr
	}

	response := linearOAuthTokenResponse{}
	if err := json.Unmarshal(body, &response); err != nil {
		clearLinearOAuthTokenCache(cache)
		return "", &bridgesdk.TransientError{Err: fmt.Errorf("linear: decode oauth token response: %w", err)}
	}
	cache.token = strings.TrimSpace(response.AccessToken)
	if cache.token == "" {
		clearLinearOAuthTokenCache(cache)
		return "", &bridgesdk.AuthError{Err: errors.New("linear: oauth token response did not include access_token")}
	}
	if response.ExpiresIn > 0 {
		cache.expiresAt = now.Add(time.Duration(response.ExpiresIn) * time.Second)
	} else {
		cache.expiresAt = time.Time{}
	}
	return cache.token, nil
}

func clearLinearOAuthTokenCache(cache *linearOAuthTokenCache) {
	if cache == nil {
		return
	}
	cache.token = ""
	cache.expiresAt = time.Time{}
}

func readLinearHTTPResponseBody(response *http.Response, operation string) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("linear: %s response body is required", operation)
	}
	body, readErr := bridgesdk.ReadHTTPResponseBody(response.Body)
	if readErr != nil {
		return []byte(body), fmt.Errorf("linear: read %s response: %w", operation, readErr)
	}
	return []byte(body), nil
}

func defaultLinearOAuthScopes(mode string) []string {
	scopes := []string{"read", "write", "comments:create", "issues:create"}
	if strings.TrimSpace(mode) == linearModeAgentSessions {
		scopes = append(scopes, "app:mentionable")
	}
	return scopes
}

func classifyLinearHTTPError(statusCode int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(statusCode)
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &bridgesdk.AuthError{Err: errors.New(message)}
	case http.StatusTooManyRequests:
		return &bridgesdk.RateLimitError{Err: errors.New(message)}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return &bridgesdk.HTTPError{StatusCode: http.StatusRequestTimeout, Message: message}
	}
	if statusCode >= 500 {
		return &bridgesdk.TransientError{Err: errors.New(message)}
	}
	return &bridgesdk.PermanentError{Err: errors.New(message)}
}

func classifyLinearTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &bridgesdk.HTTPError{StatusCode: http.StatusRequestTimeout, Message: err.Error()}
	}
	if errors.Is(err, context.Canceled) {
		return &bridgesdk.TransientError{Err: err}
	}
	if netErr, ok := errors.AsType[net.Error](err); ok {
		if netErr.Timeout() {
			return &bridgesdk.HTTPError{StatusCode: http.StatusRequestTimeout, Message: err.Error()}
		}
		return &bridgesdk.TransientError{Err: err}
	}
	return &bridgesdk.TransientError{Err: err}
}
