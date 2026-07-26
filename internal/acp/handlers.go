package acp

import (
	"context"
	"encoding/json"
	"errors"

	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

const (
	sessionUpdateConfigOption = "config_option_update"
	steerDispatchTimeout      = 10 * time.Second
)

type wireSessionNotification struct {
	SessionID acpsdk.SessionId `json:"sessionId"`
	Update    json.RawMessage  `json:"update"`
}

type wireSessionUpdateEnvelope struct {
	SessionUpdate string `json:"sessionUpdate"`
}

type wirePromptResponse struct {
	StopReason acpsdk.StopReason `json:"stopReason"`
	Usage      *wireUsage        `json:"usage,omitempty"`
}

// wireNewSessionRequest keeps the workspace extension on the top-level
// session/new payload. The workspace techspec requires the JSON-RPC field name
// `additional_dirs`, even though the upstream ACP SDK does not model it yet.
type wireNewSessionRequest struct {
	Meta           any                `json:"_meta,omitempty"`
	Cwd            string             `json:"cwd"`
	McpServers     []acpsdk.McpServer `json:"mcpServers"`
	AdditionalDirs []string           `json:"additional_dirs,omitempty"`
}

// wireLoadSessionRequest mirrors session/load with the same top-level
// `additional_dirs` field name required by the workspace techspec.
type wireLoadSessionRequest struct {
	Meta           any                `json:"_meta,omitempty"`
	Cwd            string             `json:"cwd"`
	McpServers     []acpsdk.McpServer `json:"mcpServers"`
	AdditionalDirs []string           `json:"additional_dirs,omitempty"`
	SessionID      acpsdk.SessionId   `json:"sessionId"`
}

type wireUsage struct {
	InputTokens      *int64 `json:"inputTokens,omitempty"`
	OutputTokens     *int64 `json:"outputTokens,omitempty"`
	TotalTokens      *int64 `json:"totalTokens,omitempty"`
	ThoughtTokens    *int64 `json:"thoughtTokens,omitempty"`
	CacheReadTokens  *int64 `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens *int64 `json:"cacheWriteTokens,omitempty"`
}

type wireUsageUpdate struct {
	SessionUpdate string    `json:"sessionUpdate"`
	Used          *int64    `json:"used,omitempty"`
	Size          *int64    `json:"size,omitempty"`
	Cost          *wireCost `json:"cost,omitempty"`
}

type wireCost struct {
	Amount   *float64 `json:"amount,omitempty"`
	Currency *string  `json:"currency,omitempty"`
}

func (p *AgentProcess) handleInbound(
	ctx context.Context,
	method string,
	params json.RawMessage,
) (any, *acpsdk.RequestError) {
	if method == acpsdk.ClientMethodSessionUpdate {
		if err := p.handleSessionUpdateWithContext(ctx, params); err != nil {
			return nil, requestError(err)
		}
		return nil, nil
	}

	switch method {
	case acpsdk.ClientMethodFsReadTextFile:
		return handleInboundRequest(ctx, params, p.handleReadTextFile)
	case acpsdk.ClientMethodFsWriteTextFile:
		return handleInboundRequest(ctx, params, p.handleWriteTextFile)
	case acpsdk.ClientMethodSessionRequestPermission:
		return handleInboundRequest(ctx, params, p.handleRequestPermission)
	case acpsdk.ClientMethodTerminalCreate:
		return handleInboundRequest(ctx, params, p.handleCreateTerminal)
	case acpsdk.ClientMethodTerminalKill:
		return handleInboundRequestNoContext(params, p.handleKillTerminal)
	case acpsdk.ClientMethodTerminalOutput:
		return handleInboundRequestNoContext(params, p.handleTerminalOutput)
	case acpsdk.ClientMethodTerminalWaitForExit:
		return handleInboundRequest(ctx, params, p.handleWaitForTerminalExit)
	case acpsdk.ClientMethodTerminalRelease:
		return handleInboundRequestNoContext(params, p.handleReleaseTerminal)
	default:
		return nil, acpsdk.NewMethodNotFound(method)
	}
}

func handleInboundRequest[Req any, Resp any](
	ctx context.Context,
	params json.RawMessage,
	fn func(context.Context, Req) (Resp, error),
) (any, *acpsdk.RequestError) {
	var request Req
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, acpsdk.NewInvalidParams(map[string]any{EventTypeError: err.Error()})
	}

	response, err := fn(ctx, request)
	if err != nil {
		return nil, requestError(err)
	}
	return response, nil
}

func handleInboundRequestNoContext[Req any, Resp any](
	params json.RawMessage,
	fn func(Req) (Resp, error),
) (any, *acpsdk.RequestError) {
	var request Req
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, acpsdk.NewInvalidParams(map[string]any{EventTypeError: err.Error()})
	}

	response, err := fn(request)
	if err != nil {
		return nil, requestError(err)
	}
	return response, nil
}

type readTextFileToolInput struct {
	Path  string `json:"path"`
	Line  *int   `json:"line,omitempty"`
	Limit *int   `json:"limit,omitempty"`
}

type writeTextFileToolInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type createTerminalToolInput struct {
	Command         string               `json:"command"`
	Args            []string             `json:"args,omitempty"`
	Cwd             *string              `json:"cwd,omitempty"`
	Env             []acpsdk.EnvVariable `json:"env,omitempty"`
	OutputByteLimit *int                 `json:"outputByteLimit,omitempty"`
}

func (p *AgentProcess) handleReadTextFile(
	ctx context.Context,
	request acpsdk.ReadTextFileRequest,
) (acpsdk.ReadTextFileResponse, error) {
	request, err := p.interceptReadTextFileRequest(ctx, request)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}

	content, err := p.toolHostOrDefault().ReadTextFile(ctx, request.Path)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	return acpsdk.ReadTextFileResponse{Content: sliceLines(content, request.Line, request.Limit)}, nil
}

func (p *AgentProcess) handleWriteTextFile(
	ctx context.Context,
	request acpsdk.WriteTextFileRequest,
) (acpsdk.WriteTextFileResponse, error) {
	request, err := p.interceptWriteTextFileRequest(ctx, request)
	if err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	if p.isNetworkTurn() {
		return acpsdk.WriteTextFileResponse{}, ErrToolBlockedForNetworkTurn
	}
	if err := p.toolHostOrDefault().WriteTextFile(ctx, request.Path, request.Content); err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	return acpsdk.WriteTextFileResponse{}, nil
}

func (p *AgentProcess) handleRequestPermission(
	ctx context.Context,
	request acpsdk.RequestPermissionRequest,
) (acpsdk.RequestPermissionResponse, error) {
	turnID := p.activeTurnID()
	resource := ""
	if request.ToolCall.Title != nil {
		resource = *request.ToolCall.Title
	}
	if len(request.ToolCall.Locations) > 0 {
		resource = request.ToolCall.Locations[0].Path
	}
	title := ""
	if request.ToolCall.Title != nil {
		title = *request.ToolCall.Title
	}
	sessionID := string(request.SessionId)
	toolCallID := strings.TrimSpace(string(request.ToolCall.ToolCallId))
	requestID := p.nextPermissionRequestID(turnID, request)
	if handled, err := p.interceptProviderNativePermissionRequest(ctx, request); handled {
		switch {
		case err == nil:
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return acpsdk.RequestPermissionResponse{
				Outcome: acpsdk.NewRequestPermissionOutcomeCancelled(),
			}, nil
		case errors.Is(err, ErrPermissionDenied):
			outcome, appliedDecision := selectPermissionOutcome(request.Options, decisionRejectOnce)
			raw := buildPermissionEventRaw(requestID, appliedDecision, request)
			p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, appliedDecision, raw)
			return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
		default:
			return acpsdk.RequestPermissionResponse{}, err
		}
	}

	decision, interactive := p.toolHostOrDefault().PermissionDecision(request)

	if !interactive {
		outcome, appliedDecision := selectPermissionOutcome(request.Options, decision)
		raw := buildPermissionEventRaw(requestID, appliedDecision, request)
		p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, appliedDecision, raw)
		return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	}

	requestID, pending := p.registerPendingPermission(turnID, request)
	defer p.clearPendingPermission(requestID)
	raw := buildPermissionEventRaw(requestID, decisionPending, request)
	p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, "", raw)

	timer := time.NewTimer(p.permissionTimeoutOrDefault())
	defer timer.Stop()

	select {
	case resolvedDecision := <-pending.response:
		outcome, appliedDecision := selectPermissionOutcome(request.Options, resolvedDecision)
		raw = buildPermissionEventRaw(requestID, appliedDecision, request)
		p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, appliedDecision, raw)
		return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	case <-timer.C:
		outcome, appliedDecision := selectPermissionOutcome(request.Options, decisionRejectOnce)
		raw = buildPermissionEventRaw(requestID, appliedDecision, request)
		p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, appliedDecision, raw)
		return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	case <-ctx.Done():
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.NewRequestPermissionOutcomeCancelled(),
		}, nil
	}
}
