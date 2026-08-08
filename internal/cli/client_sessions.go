package cli

import (
	"context"
	"encoding/json"

	"fmt"

	"net/http"
	"net/url"

	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"

	mcppkg "github.com/compozy/compozy/internal/mcp"
)

func (c *daemonClient) CreateSession(ctx context.Context, request CreateSessionRequest) (SessionRecord, error) {
	var response struct {
		Session SessionRecord `json:"session"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/sessions", nil, request, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *daemonClient) GetSessionHealth(ctx context.Context, id string) (SessionHealthRecord, error) {
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

func (c *daemonClient) GetSessionStatus(ctx context.Context, id string) (SessionStatusRecord, error) {
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

func (c *daemonClient) InspectSession(
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

func (c *daemonClient) RefreshSessionSoul(
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

func (c *daemonClient) StopSession(ctx context.Context, id string) error {
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

func (c *daemonClient) ArchiveSession(ctx context.Context, id string) (SessionRecord, error) {
	return c.setSessionArchived(ctx, id, "/archive")
}

func (c *daemonClient) UnarchiveSession(ctx context.Context, id string) (SessionRecord, error) {
	return c.setSessionArchived(ctx, id, "/unarchive")
}

func (c *daemonClient) setSessionArchived(
	ctx context.Context,
	id string,
	suffix string,
) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, suffix)
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *daemonClient) DeleteSession(ctx context.Context, id string) error {
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

func (c *daemonClient) ResumeSession(ctx context.Context, id string) (SessionRecord, error) {
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

func (c *daemonClient) SetSessionRuntime(
	ctx context.Context,
	id string,
	request SetSessionRuntimeRequest,
) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "/runtime")
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *daemonClient) ClearSessionRuntime(
	ctx context.Context,
	id string,
	expectedRevision int64,
) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "/runtime")
	if err != nil {
		return SessionRecord{}, err
	}
	query := url.Values{"expected_revision": []string{strconv.FormatInt(expectedRevision, 10)}}
	if err := c.doJSON(ctx, http.MethodDelete, path, query, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *daemonClient) SessionRecap(ctx context.Context, id string, limit int) (SessionRecapRecord, error) {
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

func (c *daemonClient) RepairSession(
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

func (c *daemonClient) RewindSession(
	ctx context.Context,
	id string,
	request SessionRewindRequest,
) (SessionRewindRecord, error) {
	var response SessionRewindRecord
	path, err := c.sessionScopedPath(ctx, id, "/rewind")
	if err != nil {
		return SessionRewindRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return SessionRewindRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) GetSessionTranscript(
	ctx context.Context,
	id string,
) (SessionTranscriptRecord, error) {
	var response SessionTranscriptRecord
	path, err := c.sessionScopedPath(ctx, id, "/transcript")
	if err != nil {
		return SessionTranscriptRecord{}, err
	}
	query := url.Values{}
	query.Set("limit", "1")
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return SessionTranscriptRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ApproveSession(
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

func (c *daemonClient) PromptSession(ctx context.Context, id string, message string) ([]AgentEventRecord, error) {
	messageID, err := store.NewID("msg")
	if err != nil {
		return nil, fmt.Errorf("cli: generate prompt message id: %w", err)
	}
	idempotencyKey, err := store.NewID("idem")
	if err != nil {
		return nil, fmt.Errorf("cli: generate prompt idempotency key: %w", err)
	}
	record, err := c.SendSessionPrompt(ctx, id, SessionPromptRequest{
		Message: message, MessageID: messageID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	if record.Events == nil {
		return []AgentEventRecord{}, nil
	}
	return record.Events, nil
}

func (c *daemonClient) SendSessionPrompt(
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

func (c *daemonClient) SteerSessionPrompt(
	ctx context.Context,
	id string,
	request contract.SteerPromptRequest,
) (SessionPromptRecord, error) {
	path, err := c.sessionScopedPath(ctx, id, "/steer")
	if err != nil {
		return SessionPromptRecord{}, err
	}
	return c.doSessionPrompt(ctx, http.MethodPost, path, nil, request)
}

func (c *daemonClient) SessionEvents(
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

func (c *daemonClient) StreamSessionEvents(
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

func (c *daemonClient) SessionHistory(
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

func (c *daemonClient) BindHostedMCP(
	ctx context.Context,
	request mcppkg.HostedBindRequest,
) (mcppkg.HostedBindResponse, error) {
	var response mcppkg.HostedBindResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/internal/hosted-mcp/bind", nil, request, &response); err != nil {
		return mcppkg.HostedBindResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) HostedMCPProjection(
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

func (c *daemonClient) StreamHostedMCPProjection(
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

func (c *daemonClient) CallHostedMCP(
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

func (c *daemonClient) ReleaseHostedMCP(
	ctx context.Context,
	request mcppkg.HostedReleaseRequest,
) error {
	return c.doJSON(ctx, http.MethodPost, "/api/internal/hosted-mcp/release", nil, request, nil)
}
