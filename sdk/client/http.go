package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
)

type apiEnvelope struct {
	Status  int               `json:"status"`
	Message string            `json:"message"`
	Data    json.RawMessage   `json:"data"`
	Error   *router.ErrorInfo `json:"error"`
}

// APIError represents an error returned by the Compozy API.
type APIError struct {
	Status  int
	Code    string
	Message string
	Details string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("%s: %s (status %d)", e.Code, e.Message, e.Status)
	}
	if e.Message != "" {
		return fmt.Sprintf("%s (status %d)", e.Message, e.Status)
	}
	return fmt.Sprintf("request failed with status %d", e.Status)
}

func (c *Client) postJSON(ctx context.Context, path string, payload any) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, path, nil, payload)
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	payload any,
) (*http.Response, error) {
	if c == nil || c.baseURL == nil {
		return nil, fmt.Errorf("client is not initialized")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	fullURL := c.resolve(path, query)
	var body io.Reader
	headers := make(http.Header, 4)
	headers.Set("Accept", "application/json")
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode payload: %w", err)
		}
		body = bytes.NewReader(data)
		headers.Set("Content-Type", "application/json")
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = headers
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func (c *Client) resolve(path string, query url.Values) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	ref := &url.URL{Path: path}
	if len(query) > 0 {
		ref.RawQuery = query.Encode()
	}
	return c.baseURL.ResolveReference(ref).String()
}

func decodeEnvelope[T any](resp *http.Response, allowed ...int) (T, error) {
	var zero T
	if resp == nil {
		return zero, fmt.Errorf("nil response")
	}
	decoder := json.NewDecoder(resp.Body)
	var envelope apiEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	if !statusAllowed(resp.StatusCode, allowed) {
		return zero, toAPIError(resp.StatusCode, envelope.Error, envelope.Message)
	}
	if envelope.Error != nil {
		return zero, toAPIError(resp.StatusCode, envelope.Error, envelope.Message)
	}
	if len(envelope.Data) == 0 {
		return zero, nil
	}
	var out T
	if err := json.Unmarshal(envelope.Data, &out); err != nil {
		return zero, fmt.Errorf("decode payload: %w", err)
	}
	return out, nil
}

func statusAllowed(status int, allowed []int) bool {
	if len(allowed) == 0 {
		return status >= 200 && status < 300
	}
	for _, code := range allowed {
		if status == code {
			return true
		}
	}
	return false
}

func toAPIError(status int, info *router.ErrorInfo, message string) *APIError {
	if info == nil {
		return &APIError{Status: status, Message: message}
	}
	return &APIError{
		Status:  status,
		Code:    info.Code,
		Message: info.Message,
		Details: info.Details,
	}
}
