package acp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/profiling"

	acpsdk "github.com/coder/acp-go-sdk"
)

const (
	sessionUpdateConfigOption = "config_option_update"
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
	Usage      *wireUsage        `json:"usage,omitzero"`
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

type wireResumeSessionRequest struct {
	Meta                  any                `json:"_meta,omitempty"`
	Cwd                   string             `json:"cwd"`
	McpServers            []acpsdk.McpServer `json:"mcpServers,omitempty"`
	AdditionalDirectories []string           `json:"additionalDirectories,omitempty"`
	SessionID             acpsdk.SessionId   `json:"sessionId"`
}

type wireUsage struct {
	InputTokens       *int64 `json:"inputTokens,omitzero"`
	OutputTokens      *int64 `json:"outputTokens,omitzero"`
	TotalTokens       *int64 `json:"totalTokens,omitzero"`
	ThoughtTokens     *int64 `json:"thoughtTokens,omitzero"`
	CachedReadTokens  *int64 `json:"cachedReadTokens,omitzero"`
	CachedWriteTokens *int64 `json:"cachedWriteTokens,omitzero"`
}

type wireUsageUpdate struct {
	Meta          json.RawMessage `json:"_meta,omitempty"`
	SessionUpdate string          `json:"sessionUpdate"`
	Used          *int64          `json:"used,omitzero"`
	Size          *int64          `json:"size,omitzero"`
	Cost          *wireCost       `json:"cost,omitzero"`
}

type wireCost struct {
	Amount   *float64 `json:"amount,omitzero"`
	Currency *string  `json:"currency,omitzero"`
}

func (p *AgentProcess) handleInbound(
	ctx context.Context,
	method string,
	params json.RawMessage,
) (any, *acpsdk.RequestError) {
	var result any
	var err *acpsdk.RequestError
	profiling.Do(
		ctx,
		func(ctx context.Context) { result, err = p.handleInboundRequest(ctx, method, params) },
		"operation",
		"acp",
		"session_id",
		p.compozySessionID,
	)
	return result, err
}

func (p *AgentProcess) handleInboundRequest(
	ctx context.Context,
	method string,
	params json.RawMessage,
) (any, *acpsdk.RequestError) {
	if method == acpsdk.ClientMethodSessionUpdate {
		if err := p.handleSessionUpdate(params); err != nil {
			return nil, requestError(err)
		}
		return nil, nil
	}
	if reqErr := p.rejectNonBoundSessionCallback(method, params); reqErr != nil {
		return nil, reqErr
	}

	switch method {
	case acpsdk.ClientMethodFsReadTextFile:
		return handleInboundRequest(ctx, params, p.handleReadTextFile)
	case acpsdk.ClientMethodFsWriteTextFile:
		return handleInboundRequest(ctx, params, p.handleWriteTextFile)
	case acpsdk.ClientMethodSessionRequestPermission:
		return handleInboundRequest(ctx, params, p.handleRequestPermission)
	case acpsdk.ClientMethodElicitationCreate:
		return p.handleCreateElicitation(ctx, params)
	case acpsdk.ClientMethodTerminalCreate:
		return handleInboundRequest(ctx, params, p.handleCreateTerminal)
	case acpsdk.ClientMethodTerminalKill:
		return handleInboundRequest(ctx, params, p.handleKillTerminal)
	case acpsdk.ClientMethodTerminalOutput:
		return handleInboundRequest(ctx, params, p.handleTerminalOutput)
	case acpsdk.ClientMethodTerminalWaitForExit:
		return handleInboundRequest(ctx, params, p.handleWaitForTerminalExit)
	case acpsdk.ClientMethodTerminalRelease:
		return handleInboundRequest(ctx, params, p.handleReleaseTerminal)
	case claudeSDKMessageMethod:
		if err := p.handleClaudeSDKMessage(params); err != nil {
			return nil, acpsdk.NewInvalidParams(map[string]any{EventTypeError: err.Error()})
		}
		return nil, nil
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

type readTextFileToolInput struct {
	Path  string `json:"path"`
	Line  *int   `json:"line,omitzero"`
	Limit *int   `json:"limit,omitzero"`
}

type writeTextFileToolInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type createTerminalToolInput struct {
	Command         string               `json:"command"`
	Args            []string             `json:"args,omitempty"`
	Cwd             *string              `json:"cwd,omitzero"`
	Env             []acpsdk.EnvVariable `json:"env,omitempty"`
	OutputByteLimit *int                 `json:"outputByteLimit,omitzero"`
}

func (p *AgentProcess) handleReadTextFile(
	ctx context.Context,
	request acpsdk.ReadTextFileRequest,
) (acpsdk.ReadTextFileResponse, error) {
	request, err := p.interceptReadTextFileRequest(ctx, request)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}

	host, err := p.toolHostOrDefault()
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	content, err := host.ReadTextFile(ctx, request.Path)
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
	host, err := p.toolHostOrDefault()
	if err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	if err := host.WriteTextFile(ctx, request.Path, request.Content); err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	return acpsdk.WriteTextFileResponse{}, nil
}

// handleRequestPermission serves the agent's inbound session/request_permission
// callback. Only the bound ACP session may ask the operator; a fork clone or
// foreign id is canceled before it reaches the active turn (ADR-003).
func (p *AgentProcess) handleRequestPermission(
	ctx context.Context,
	request acpsdk.RequestPermissionRequest,
) (acpsdk.RequestPermissionResponse, error) {
	if !p.permissionRequestIsBound(request.SessionId) {
		return acpsdk.RequestPermissionResponse{Outcome: acpsdk.NewRequestPermissionOutcomeCancelled()}, nil
	}
	return p.resolvePermissionRequest(ctx, request, permissionActorProvider)
}

// resolvePermissionRequest decides one permission request through the provider
// interceptor, the tool-host policy, or the operator prompt.
func (p *AgentProcess) resolvePermissionRequest(
	ctx context.Context,
	request acpsdk.RequestPermissionRequest,
	cancellationActor string,
) (acpsdk.RequestPermissionResponse, error) {
	if p.executionPolicy == ExecutionPolicyTriage {
		return p.triagePermission(ctx, request)
	}
	turnID, turnSpan := p.activeTraceContext()
	resource, title := permissionRequestDisplay(request)
	sessionID := string(request.SessionId)
	toolCallID := strings.TrimSpace(string(request.ToolCall.ToolCallId))
	requestID := p.nextPermissionRequestID(turnID, request)
	if response, err := p.interceptPermissionResponse(ctx, request, turnID, requestID); response != nil || err != nil {
		if response != nil {
			return *response, err
		}
		return acpsdk.RequestPermissionResponse{}, err
	}

	host, err := p.toolHostOrDefault()
	if err != nil {
		return acpsdk.RequestPermissionResponse{}, err
	}
	decision, interactive, resolvedBy := resolveHostPermissionDecision(host, request, cancellationActor)
	if interactive && p.answeredByHostedToolGate(request, cancellationActor, resolvedBy) {
		// The native tool policy decides this call (or the operator already did),
		// so no prompt and no second receipt for the same call.
		outcome, _ := selectPermissionOutcome(request.Options, decisionAllowOnce)
		return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	}

	if !interactive {
		outcome, appliedDecision, raw := p.automaticPermissionOutcome(requestID, request, decision, resolvedBy)
		p.emitPermissionEvent(
			sessionID, turnID, requestID, title, toolCallID, resource, appliedDecision, resolvedBy, raw,
		)
		return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	}

	digest := permissionRequestDigest(request)
	denialEpoch := p.permissionDenials.Epoch()
	if p.permissionDenials.Contains(digest) {
		slog.InfoContext(ctx, "permission.repeat_denied", "session_id", sessionID, "request_id", requestID)
		outcome, appliedDecision := selectPermissionOutcome(request.Options, decisionRejectOnce)
		raw := buildPermissionEventRaw(requestID, appliedDecision, request)
		p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource,
			appliedDecision, "repeat_denied", raw)
		return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	}
	requestID, pending, err := p.registerPendingPermission(turnID, request)
	if err != nil {
		return acpsdk.RequestPermissionResponse{}, err
	}
	defer p.clearPendingPermission(requestID)
	raw := buildPermissionEventRaw(requestID, decisionPending, request)
	expiresAt := pending.expiresAt
	p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, "", "", raw, expiresAt)
	wait := max(time.Until(expiresAt), 0)

	waitSpan := startPermissionWaitSpan(ctx, turnSpan)
	decision, actor := p.awaitPendingPermission(ctx, pending, wait, cancellationActor)
	outcome, appliedDecision := selectPermissionOutcome(request.Options, decision)
	if appliedDecision == "" {
		appliedDecision = decisionCanceled
	}
	endPermissionWaitSpan(waitSpan, appliedDecision)
	if p.permissionExpiryCountsAsDenial(actor, decision) {
		p.permissionDenials.Record(digest, denialEpoch)
	}
	if cancellationActor == permissionActorProvider {
		p.rememberHostedApproval(request, appliedDecision)
	}
	raw = buildPermissionEventRaw(requestID, appliedDecision, request)
	p.emitPermissionEvent(sessionID, turnID, requestID, title, toolCallID, resource, appliedDecision, actor, raw)
	return acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
}

// awaitPendingPermission waits for an answer to one registered permission
// request, expiring it on timeout, disconnect, or cancellation.
func (p *AgentProcess) awaitPendingPermission(
	ctx context.Context,
	pending *pendingPermission,
	wait time.Duration,
	cancellationActor string,
) (permissionDecision, string) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case decision := <-pending.response:
		return decision, pending.resolvedBy
	case <-timer.C:
		return p.expirePendingPermission(pending, decisionRejectOnce, "timeout")
	case <-p.permissionConnectionDone():
		return p.expirePendingPermission(pending, decisionCanceled, permissionActorSystem)
	case <-ctx.Done():
		if p.stopWasRequested() || p.permissionConnectionClosed() {
			cancellationActor = permissionActorSystem
		}
		if cancellationActor == permissionActorSystem && errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			cancellationActor = "timeout"
		}
		return p.expirePendingPermission(pending, decisionCanceled, cancellationActor)
	}
}

func (p *AgentProcess) interceptPermissionResponse(ctx context.Context, request acpsdk.RequestPermissionRequest,
	turnID, requestID string) (*acpsdk.RequestPermissionResponse, error) {
	handled, err := p.interceptProviderNativePermissionRequest(ctx, request)
	if !handled || err == nil {
		return nil, nil
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return &acpsdk.RequestPermissionResponse{Outcome: acpsdk.NewRequestPermissionOutcomeCancelled()}, nil
	case errors.Is(err, ErrPermissionDenied):
		outcome, applied := selectPermissionOutcome(request.Options, decisionRejectOnce)
		resource, title := permissionRequestDisplay(request)
		p.emitPermissionEvent(string(request.SessionId), turnID, requestID, title,
			strings.TrimSpace(string(request.ToolCall.ToolCallId)), resource, applied, permissionActorProvider,
			buildPermissionEventRaw(requestID, applied, request))
		return &acpsdk.RequestPermissionResponse{Outcome: outcome}, nil
	default:
		return nil, err
	}
}

func permissionRequestDisplay(request acpsdk.RequestPermissionRequest) (string, string) {
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
	return resource, title
}
