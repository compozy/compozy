package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
)

var (
	// ErrDaemonClientRequired reports that the receiver client was nil.
	ErrDaemonClientRequired = errors.New("daemon client is required")
	// ErrDaemonContextRequired reports that the caller did not provide a request context.
	ErrDaemonContextRequired = errors.New("daemon request context is required")
	// ErrWorkflowSlugRequired reports that a workflow slug argument was blank.
	ErrWorkflowSlugRequired = errors.New("workflow slug is required")
	// ErrRunIDRequired reports that a run identifier argument was blank.
	ErrRunIDRequired = errors.New("run id is required")
)

// Target identifies one daemon transport endpoint.
type Target struct {
	SocketPath string
	HTTPPort   int
}

// Validate ensures the target can be dialed over UDS or localhost HTTP.
func (t Target) Validate() error {
	socketPath := strings.TrimSpace(t.SocketPath)
	if socketPath != "" {
		return nil
	}
	if t.HTTPPort <= 0 || t.HTTPPort > 65535 {
		return errors.New("daemon transport target is invalid")
	}
	return nil
}

// String renders the transport target for logs and user-facing diagnostics.
func (t Target) String() string {
	socketPath := strings.TrimSpace(t.SocketPath)
	if socketPath != "" {
		return "unix://" + socketPath
	}
	if t.HTTPPort > 0 {
		return "http://127.0.0.1:" + strconv.Itoa(t.HTTPPort)
	}
	return "unknown"
}

// Client issues daemon API requests over UDS by default with localhost HTTP fallback.
type Client struct {
	target     Target
	baseURL    string
	httpClient *http.Client
}

// RemoteError captures the daemon's non-2xx transport error envelope.
type RemoteError struct {
	StatusCode int
	Envelope   apicore.TransportError
}

func (e *RemoteError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Envelope.Message)
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if requestID := strings.TrimSpace(e.Envelope.RequestID); requestID != "" {
		return fmt.Sprintf("%s (request_id=%s)", message, requestID)
	}
	return message
}

// New constructs a transport client for one daemon target.
func New(target Target) (*Client, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}

	socketPath := strings.TrimSpace(target.SocketPath)
	if socketPath != "" {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		}
		return &Client{
			target:  target,
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Transport: transport,
			},
		}, nil
	}

	baseURL := "http://127.0.0.1:" + strconv.Itoa(target.HTTPPort)
	return &Client{
		target:     target,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}, nil
}

// Target reports the transport endpoint in use.
func (c *Client) Target() Target {
	if c == nil {
		return Target{}
	}
	return c.target
}

// Health probes the daemon readiness endpoint. A 503 response with a valid
// health payload is treated as a successful probe result rather than a transport error.
func (c *Client) Health(ctx context.Context) (apicore.DaemonHealth, error) {
	if c == nil {
		return apicore.DaemonHealth{}, ErrDaemonClientRequired
	}
	response := struct {
		Health apicore.DaemonHealth `json:"health"`
	}{}

	statusCode, err := c.doJSON(ctx, http.MethodGet, "/api/daemon/health", nil, &response)
	if err != nil {
		return apicore.DaemonHealth{}, err
	}
	switch statusCode {
	case http.StatusOK, http.StatusServiceUnavailable:
		return response.Health, nil
	default:
		return apicore.DaemonHealth{}, fmt.Errorf("unexpected daemon health status: %d", statusCode)
	}
}

// StartTaskRun starts one daemon-backed task workflow run.
func (c *Client) StartTaskRun(
	ctx context.Context,
	slug string,
	req apicore.TaskRunRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, ErrDaemonClientRequired
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return apicore.Run{}, ErrWorkflowSlugRequired
	}

	body := map[string]any{
		"workspace":         strings.TrimSpace(req.Workspace),
		"presentation_mode": strings.TrimSpace(req.PresentationMode),
	}
	if len(req.RuntimeOverrides) > 0 {
		body["runtime_overrides"] = req.RuntimeOverrides
	}

	response := struct {
		Run apicore.Run `json:"run"`
	}{}
	path := "/api/tasks/" + url.PathEscape(slug) + "/runs"
	if _, err := c.doJSON(ctx, http.MethodPost, path, body, &response); err != nil {
		return apicore.Run{}, err
	}
	return response.Run, nil
}

func (c *Client) doJSON(
	ctx context.Context,
	method string,
	path string,
	requestBody any,
	responseBody any,
) (int, error) {
	if c == nil {
		return 0, ErrDaemonClientRequired
	}
	if ctx == nil {
		return 0, ErrDaemonContextRequired
	}
	ctx, cancel := withRequestTimeout(ctx, c.requestTimeout(method, path))
	defer cancel()

	bodyReader, hasBody, err := marshalRequestBody(requestBody)
	if err != nil {
		return 0, err
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("build daemon request: %w", err)
	}
	if err := applyRequestPath(request, path); err != nil {
		return 0, err
	}
	if hasBody {
		request.Header.Set("Content-Type", "application/json")
	}

	statusCode, payload, err := c.doRequest(request)
	if err != nil {
		return statusCode, err
	}

	if err := c.handleStatus(path, statusCode, payload, responseBody); err != nil {
		return statusCode, err
	}
	if err := decodeResponseBody(payload, responseBody); err != nil {
		return statusCode, err
	}
	return statusCode, nil
}

func marshalRequestBody(requestBody any) (io.Reader, bool, error) {
	if requestBody == nil {
		return nil, false, nil
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, false, fmt.Errorf("encode daemon request: %w", err)
	}
	return bytes.NewReader(payload), true, nil
}

func (c *Client) doRequest(request *http.Request) (int, []byte, error) {
	response, err := c.roundTrip(request)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, fmt.Errorf("read daemon response: %w", err)
	}
	return response.StatusCode, payload, nil
}

func (c *Client) roundTrip(request *http.Request) (*http.Response, error) {
	if c == nil || c.httpClient == nil {
		return nil, ErrDaemonClientRequired
	}

	transport := c.httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.target.String(), err)
	}
	return response, nil
}

func (c *Client) handleStatus(
	path string,
	statusCode int,
	payload []byte,
	responseBody any,
) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}
	if statusCode == http.StatusServiceUnavailable &&
		strings.HasSuffix(path, "/daemon/health") &&
		responseBody != nil {
		if err := json.Unmarshal(payload, responseBody); err == nil {
			return nil
		}
	}

	var envelope apicore.TransportError
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("daemon request failed with status %d", statusCode)
	}
	return &RemoteError{
		StatusCode: statusCode,
		Envelope:   envelope,
	}
}

func decodeResponseBody(payload []byte, responseBody any) error {
	if responseBody == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, responseBody); err != nil {
		return fmt.Errorf("decode daemon response: %w", err)
	}
	return nil
}

func applyRequestPath(request *http.Request, requestPath string) error {
	if request == nil {
		return errors.New("daemon request is required")
	}

	parsed, err := url.ParseRequestURI(strings.TrimSpace(requestPath))
	if err != nil {
		return fmt.Errorf("validate daemon request path: %w", err)
	}
	if parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/api/") {
		return fmt.Errorf("validate daemon request path: %q is not a daemon API path", requestPath)
	}

	request.URL.Path = parsed.Path
	request.URL.RawPath = parsed.EscapedPath()
	request.URL.RawQuery = parsed.RawQuery
	return nil
}

func (c *Client) requestTimeout(method string, path string) time.Duration {
	return contract.DefaultTimeout(contract.TimeoutClassForRoute(method, path))
}

func withRequestTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
