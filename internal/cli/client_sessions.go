package cli

import (
	"context"
	"encoding/json"

	"fmt"

	"net/http"
	"net/url"

	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	mcppkg "github.com/compozy/agh/internal/mcp"
)

func (c *unixSocketClient) CreateSession(ctx context.Context, request CreateSessionRequest) (SessionRecord, error) {
	var response struct {
		Session SessionRecord `json:"session"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/sessions", nil, request, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *unixSocketClient) GetSessionHealth(ctx context.Context, id string) (SessionHealthRecord, error) {
	var response struct {
		Health SessionHealthRecord `json:"health"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/health")
	if err != nil {
		return SessionHealthRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SessionHealthRecord{}, err
	}
	return response.Health, nil
}

func (c *unixSocketClient) GetSessionStatus(ctx context.Context, id string) (SessionStatusRecord, error) {
	var response SessionStatusRecord
	path, err := c.sessionScopedPath(ctx, id, "/status")
	if err != nil {
		return SessionStatusRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SessionStatusRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) InspectSession(
	ctx context.Context,
	id string,
	query SessionInspectQuery,
) (SessionInspectRecord, error) {
	var response SessionInspectRecord
	path, err := c.sessionScopedPath(ctx, id, "/inspect")
	if err != nil {
		return SessionInspectRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, sessionInspectValues(query), nil, &response); err != nil {
		return SessionInspectRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RefreshSessionSoul(
	ctx context.Context,
	id string,
	request SessionSoulRefreshRequest,
) (AgentSoulRecord, error) {
	var response AgentSoulRecord
	path, err := c.sessionScopedPath(ctx, id, "/soul/refresh")
	if err != nil {
		return AgentSoulRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return AgentSoulRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) StopSession(ctx context.Context, id string) error {
	path, err := c.sessionScopedPath(ctx, id, "/stop")
	if err != nil {
		return err
	}
	return c.doJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		nil,
		nil,
	)
}

func (c *unixSocketClient) DeleteSession(ctx context.Context, id string) error {
	path, err := c.sessionScopedPath(ctx, id, "")
	if err != nil {
		return err
	}
	return c.doJSON(
		ctx,
		http.MethodDelete,
		path,
		nil,
		nil,
		nil,
	)
}

func (c *unixSocketClient) ResumeSession(ctx context.Context, id string) (SessionRecord, error) {
	var response struct {
		Session SessionRecord `json:"session"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/attach")
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		nil,
		&response,
	); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *unixSocketClient) SessionRecap(ctx context.Context, id string, limit int) (SessionRecapRecord, error) {
	var response struct {
		Recap SessionRecapRecord `json:"recap"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/recap")
	if err != nil {
		return SessionRecapRecord{}, err
	}
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return SessionRecapRecord{}, err
	}
	return response.Recap, nil
}

func (c *unixSocketClient) RepairSession(
	ctx context.Context,
	id string,
	query SessionRepairQuery,
) (SessionRepairRecord, error) {
	var response struct {
		Repair SessionRepairRecord `json:"repair"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/repair")
	if err != nil {
		return SessionRepairRecord{}, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		path,
		sessionRepairValues(query),
		nil,
		&response,
	); err != nil {
		return SessionRepairRecord{}, err
	}
	return response.Repair, nil
}

func (c *unixSocketClient) ApproveSession(
	ctx context.Context,
	id string,
	request SessionApprovalRequest,
) (SessionApprovalRecord, error) {
	var response SessionApprovalRecord
	path, err := c.sessionScopedPath(ctx, id, "/approve")
	if err != nil {
		return SessionApprovalRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return SessionApprovalRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PromptSession(ctx context.Context, id string, message string) ([]AgentEventRecord, error) {
	record, err := c.SendSessionPrompt(ctx, id, SessionPromptRequest{Message: message})
	if err != nil {
		return nil, err
	}
	if record.Events == nil {
		return []AgentEventRecord{}, nil
	}
	return record.Events, nil
}

func (c *unixSocketClient) SendSessionPrompt(
	ctx context.Context,
	id string,
	request SessionPromptRequest,
) (SessionPromptRecord, error) {
	query := url.Values{}
	query.Set("format", "raw")
	path, err := c.sessionScopedPath(ctx, id, "/prompt")
	if err != nil {
		return SessionPromptRecord{}, err
	}
	return c.doSessionPrompt(ctx, http.MethodPost, path, query, request)
}

func (c *unixSocketClient) SteerSessionPrompt(
	ctx context.Context,
	id string,
	text string,
) (SessionPromptRecord, error) {
	path, err := c.sessionScopedPath(ctx, id, "/steer")
	if err != nil {
		return SessionPromptRecord{}, err
	}
	return c.doSessionPrompt(ctx, http.MethodPost, path, nil, contract.SteerPromptRequest{Text: text})
}

func (c *unixSocketClient) CancelQueuedSessionPrompt(
	ctx context.Context,
	id string,
	queueEntryID string,
) (SessionPromptRecord, error) {
	entryID, err := requireNetworkPathValue("queue_entry_id", queueEntryID)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	path, err := c.sessionScopedPath(ctx, id, "/prompt/queue/"+url.PathEscape(entryID))
	if err != nil {
		return SessionPromptRecord{}, err
	}
	return c.doSessionPrompt(ctx, http.MethodDelete, path, nil, nil)
}

func (c *unixSocketClient) SessionEvents(
	ctx context.Context,
	id string,
	query SessionEventQuery,
) ([]SessionEventRecord, error) {
	var response struct {
		Events []SessionEventRecord `json:"events"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/events")
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		sessionEventValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Events, nil
}

func (c *unixSocketClient) StreamSessionEvents(
	ctx context.Context,
	id string,
	query SessionEventQuery,
	lastEventID string,
	handler SSEHandler,
) error {
	path, err := c.sessionScopedPath(ctx, id, "/stream")
	if err != nil {
		return err
	}
	values := sessionEventValues(query)
	values.Set("frames", contract.SessionStreamFrameRaw)
	return c.doSSE(
		ctx,
		path,
		values,
		lastEventID,
		handler,
	)
}

func (c *unixSocketClient) SessionHistory(
	ctx context.Context,
	id string,
	query SessionEventQuery,
) ([]TurnHistoryRecord, error) {
	var response struct {
		History []TurnHistoryRecord `json:"history"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/history")
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		sessionEventValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.History, nil
}

func (c *unixSocketClient) BindHostedMCP(
	ctx context.Context,
	request mcppkg.HostedBindRequest,
) (mcppkg.HostedBindResponse, error) {
	var response mcppkg.HostedBindResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/internal/hosted-mcp/bind", nil, request, &response); err != nil {
		return mcppkg.HostedBindResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) HostedMCPProjection(
	ctx context.Context,
	bindID string,
) (mcppkg.HostedProjectionResponse, error) {
	query := url.Values{}
	query.Set("bind_id", strings.TrimSpace(bindID))
	var response mcppkg.HostedProjectionResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/internal/hosted-mcp/projection", query, nil, &response); err != nil {
		return mcppkg.HostedProjectionResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) StreamHostedMCPProjection(
	ctx context.Context,
	bindID string,
	lastDigest string,
	handler mcppkg.HostedProjectionHandler,
) error {
	query := url.Values{}
	query.Set("bind_id", strings.TrimSpace(bindID))
	if trimmed := strings.TrimSpace(lastDigest); trimmed != "" {
		query.Set("last_digest", trimmed)
	}
	return c.doSSE(
		ctx,
		"/api/internal/hosted-mcp/projection/stream",
		query,
		"",
		func(event SSEEvent) error {
			if event.Event == clientErrorKey {
				return readAPIErrorBody(0, "", event.Data)
			}
			if event.Event != "projection" {
				return nil
			}
			var snapshot mcppkg.HostedProjectionResponse
			if err := json.Unmarshal(event.Data, &snapshot); err != nil {
				return fmt.Errorf("cli: decode hosted MCP projection event: %w", err)
			}
			if handler == nil {
				return nil
			}
			return handler(snapshot)
		},
	)
}

func (c *unixSocketClient) CallHostedMCP(
	ctx context.Context,
	request mcppkg.HostedCallRequest,
) (mcppkg.HostedCallResponse, error) {
	var response mcppkg.HostedCallResponse
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/internal/hosted-mcp/tools/call",
		nil,
		request,
		&response,
	); err != nil {
		return mcppkg.HostedCallResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ReleaseHostedMCP(
	ctx context.Context,
	request mcppkg.HostedReleaseRequest,
) error {
	return c.doJSON(ctx, http.MethodPost, "/api/internal/hosted-mcp/release", nil, request, nil)
}
