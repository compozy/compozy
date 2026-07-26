package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/gin-gonic/gin"
)

const (
	toolInvokeStatusCompleted = "completed"
	toolInvokeStatusCanceled  = "canceled"
	toolsetStatusExpanded     = "expanded"
	toolsetStatusDegraded     = "degraded"
)

// ListTools returns the operator-visible registry projection.
func (h *BaseHandlers) ListTools(c *gin.Context) {
	if h.Tools == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool registry is not configured"))
		return
	}
	scope := h.operatorToolScope(c)
	views, err := h.Tools.List(c.Request.Context(), scope)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsResponse{Tools: ToolPayloadsFromViews(views)})
}

// SearchTools searches the operator-visible registry projection.
func (h *BaseHandlers) SearchTools(c *gin.Context) {
	req, ok := h.bindToolSearch(c)
	if !ok {
		return
	}
	if h.Tools == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool registry is not configured"))
		return
	}
	scope := toolScopeFromSearch(h.operatorToolScope(c), req)
	views, err := h.Tools.Search(c.Request.Context(), scope, toolspkg.SearchQuery{
		Query: req.Query,
		Limit: req.Limit,
	})
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsResponse{Tools: ToolPayloadsFromViews(views)})
}

// GetTool returns one operator-visible tool projection.
func (h *BaseHandlers) GetTool(c *gin.Context) {
	if h.Tools == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool registry is not configured"))
		return
	}
	id, ok := h.toolIDParam(c)
	if !ok {
		return
	}
	view, err := h.Tools.Get(c.Request.Context(), h.operatorToolScope(c), id)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolResponse{Tool: ToolPayloadFromView(&view)})
}

// CreateToolApproval mints one daemon-memory approval reference for a concrete invocation.
func (h *BaseHandlers) CreateToolApproval(c *gin.Context) {
	if h.Tools == nil || h.ToolApprovals == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool approval service is not configured"))
		return
	}
	id, ok := h.toolIDParam(c)
	if !ok {
		return
	}
	var req contract.ToolApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondToolError(c, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			fmt.Sprintf("%s: decode tool approval request: %v", h.transportName(), err),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		))
		return
	}
	scope := h.operatorToolScope(c)
	effectiveScope, err := approvalScopeFromRequest(scope, req)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	scope = effectiveScope
	if _, err := h.Tools.Get(c.Request.Context(), scope, id); err != nil {
		h.respondToolError(c, err)
		return
	}
	grant, err := h.ToolApprovals.CreateToolApproval(c.Request.Context(), scope, toolspkg.ApprovalRequest{
		ToolID:      id,
		SessionID:   scope.SessionID,
		WorkspaceID: scope.WorkspaceID,
		AgentName:   scope.AgentName,
		Input:       cloneRawMessage(req.Input),
		InputDigest: req.InputDigest,
	})
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusCreated, contract.ToolApprovalResponse{Approval: contract.ToolApprovalPayload{
		ApprovalToken: grant.ApprovalToken,
		ExpiresAt:     grant.ExpiresAt,
		ToolID:        grant.ToolID,
		InputDigest:   grant.InputDigest,
	}})
}

// InvokeTool dispatches a concrete tool invocation through the registry.
func (h *BaseHandlers) InvokeTool(c *gin.Context) {
	if h.Tools == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool registry is not configured"))
		return
	}
	id, ok := h.toolIDParam(c)
	if !ok {
		return
	}
	var req contract.ToolInvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondToolError(c, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			fmt.Sprintf("%s: decode tool invoke request: %v", h.transportName(), err),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		))
		return
	}
	scope := h.operatorToolScope(c)
	scope.SessionID = firstNonEmpty(req.SessionID, scope.SessionID)
	scope.WorkspaceID = firstNonEmpty(req.WorkspaceID, scope.WorkspaceID)
	scope.AgentName = firstNonEmpty(req.AgentName, scope.AgentName)
	result, err := h.Tools.Call(c.Request.Context(), scope, toolspkg.CallRequest{
		ToolID:               id,
		ToolCallID:           req.ToolCallID,
		TurnID:               req.TurnID,
		SessionID:            scope.SessionID,
		WorkspaceID:          scope.WorkspaceID,
		AgentName:            scope.AgentName,
		CorrelationID:        req.CorrelationID,
		Input:                cloneRawMessage(req.Input),
		SensitiveInputFields: append([]string(nil), req.SensitiveInputFields...),
		ApprovalToken:        req.ApprovalToken,
	})
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolInvokeResponse{
		ToolID:     id,
		Status:     toolInvokeStatusCompleted,
		Result:     result,
		Truncated:  result.Truncated,
		DurationMS: result.DurationMS,
		Events:     []contract.ToolCallEventPayload{},
	})
}

// ListSessionTools returns the session/model-callable projection.
func (h *BaseHandlers) ListSessionTools(c *gin.Context) {
	if h.Tools == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool registry is not configured"))
		return
	}
	routeScope, routeSessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope := h.sessionToolScope(c, routeScope.ID, routeSessionID)
	views, err := h.Tools.List(c.Request.Context(), scope)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsResponse{Tools: ToolPayloadsFromViews(views)})
}

// SearchSessionTools searches only within the session/model-callable projection.
func (h *BaseHandlers) SearchSessionTools(c *gin.Context) {
	req, ok := h.bindToolSearch(c)
	if !ok {
		return
	}
	if h.Tools == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool registry is not configured"))
		return
	}
	routeScope, routeSessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope := h.sessionToolScope(c, routeScope.ID, routeSessionID)
	if reqWorkspaceID := strings.TrimSpace(
		req.WorkspaceID,
	); reqWorkspaceID != "" &&
		reqWorkspaceID != scope.WorkspaceID {
		h.respondError(c, http.StatusBadRequest, errors.New("workspace_id does not match path"))
		return
	}
	if reqSessionID := strings.TrimSpace(req.SessionID); reqSessionID != "" && reqSessionID != scope.SessionID {
		h.respondError(c, http.StatusBadRequest, errors.New("session_id does not match path"))
		return
	}
	scope.AgentName = firstNonEmpty(req.AgentName, scope.AgentName)
	views, err := h.Tools.Search(c.Request.Context(), scope, toolspkg.SearchQuery{
		Query: req.Query,
		Limit: req.Limit,
	})
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsResponse{Tools: ToolPayloadsFromViews(views)})
}

// ListToolsets returns named toolsets with expansion diagnostics.
func (h *BaseHandlers) ListToolsets(c *gin.Context) {
	if h.Toolsets == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("toolset registry is not configured"))
		return
	}
	views, err := h.Toolsets.ListToolsets(c.Request.Context(), h.operatorToolScope(c))
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsetsResponse{Toolsets: ToolsetPayloadsFromViews(views)})
}

// GetToolset returns one named toolset with expansion diagnostics.
func (h *BaseHandlers) GetToolset(c *gin.Context) {
	if h.Toolsets == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("toolset registry is not configured"))
		return
	}
	id := toolspkg.ToolsetID(strings.TrimSpace(c.Param("id")))
	if err := id.Validate(); err != nil {
		h.respondToolError(c, err)
		return
	}
	view, err := h.Toolsets.GetToolset(c.Request.Context(), h.operatorToolScope(c), id)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsetResponse{Toolset: ToolsetPayloadFromView(view)})
}

// ToolPayloadsFromViews converts registry views into public DTOs.
func ToolPayloadsFromViews(views []toolspkg.ToolView) []contract.ToolPayload {
	payloads := make([]contract.ToolPayload, 0, len(views))
	for i := range views {
		payloads = append(payloads, ToolPayloadFromView(&views[i]))
	}
	return payloads
}

// ToolPayloadFromView converts one registry view into a public DTO.
func ToolPayloadFromView(view *toolspkg.ToolView) contract.ToolPayload {
	return contract.ToolPayload{
		Descriptor:   toolDescriptorPayload(view.Descriptor),
		Availability: toolAvailabilityPayload(view.Availability),
		Decision:     toolDecisionPayload(view.Decision),
	}
}

// ToolsetPayloadsFromViews converts toolset projections into public DTOs.
func ToolsetPayloadsFromViews(views []toolspkg.ToolsetView) []contract.ToolsetPayload {
	payloads := make([]contract.ToolsetPayload, 0, len(views))
	for i := range views {
		payloads = append(payloads, ToolsetPayloadFromView(views[i]))
	}
	return payloads
}

// ToolsetPayloadFromView converts one toolset projection into a public DTO.
func ToolsetPayloadFromView(view toolspkg.ToolsetView) contract.ToolsetPayload {
	status := toolsetStatusExpanded
	if len(view.ReasonCodes) > 0 {
		status = toolsetStatusDegraded
	}
	return contract.ToolsetPayload{
		ID:            view.Toolset.ID,
		Tools:         append([]string(nil), view.Toolset.Tools...),
		Toolsets:      append([]toolspkg.ToolsetID(nil), view.Toolset.Toolsets...),
		ExpandedTools: append([]toolspkg.ToolID(nil), view.ExpandedTools...),
		Status:        status,
		ReasonCodes:   append([]toolspkg.ReasonCode(nil), view.ReasonCodes...),
	}
}

// toolDescriptorPayload detaches registry-owned descriptor data from transport DTOs.
func toolDescriptorPayload(d toolspkg.Descriptor) contract.ToolDescriptorPayload {
	presentation := d.Presentation()
	return contract.ToolDescriptorPayload{
		ToolID:              d.ID,
		Backend:             toolBackendPayload(d.Backend),
		DisplayTitle:        d.DisplayTitle,
		FriendlyVerb:        presentation.FriendlyVerb,
		Preview:             presentation.Preview,
		Description:         d.Description,
		InputSchema:         cloneRawMessage(d.InputSchema),
		OutputSchema:        cloneRawMessage(d.OutputSchema),
		InputSchemaDigest:   d.InputSchemaDigest,
		OutputSchemaDigest:  d.OutputSchemaDigest,
		Source:              toolSourcePayload(d.Source),
		Visibility:          d.Visibility,
		Risk:                d.Risk,
		ReadOnly:            d.ReadOnly,
		Destructive:         d.Destructive,
		OpenWorld:           d.OpenWorld,
		RequiresInteraction: d.RequiresInteraction,
		ConcurrencySafe:     d.ConcurrencySafe,
		MaxResultBytes:      d.MaxResultBytes,
		Toolsets:            append([]toolspkg.ToolsetID(nil), d.Toolsets...),
		Tags:                append([]string(nil), d.Tags...),
		SearchHints:         append([]string(nil), d.SearchHints...),
	}
}

// toolBackendPayload preserves backend routing metadata without exposing registry internals.
func toolBackendPayload(backend toolspkg.BackendRef) contract.ToolBackendRefPayload {
	return contract.ToolBackendRefPayload{
		Kind:                 backend.Kind,
		ExtensionID:          backend.ExtensionID,
		Handler:              backend.Handler,
		MCPServer:            backend.MCPServer,
		MCPTool:              backend.MCPTool,
		NativeName:           backend.NativeName,
		RequiresCapabilities: append([]string(nil), backend.RequiresCapabilities...),
	}
}

// toolSourcePayload carries provenance fields needed for operator audits.
func toolSourcePayload(source toolspkg.SourceRef) contract.ToolSourceRefPayload {
	return contract.ToolSourceRefPayload{
		Kind:            source.Kind,
		Owner:           source.Owner,
		RawServerName:   source.RawServerName,
		RawToolName:     source.RawToolName,
		ResourceID:      source.ResourceID,
		ResourceVersion: source.ResourceVersion,
		WorkspaceID:     source.WorkspaceID,
		Scope:           source.Scope,
	}
}

// toolAvailabilityPayload keeps availability reasons stable across transports.
func toolAvailabilityPayload(availability toolspkg.Availability) contract.ToolAvailabilityPayload {
	return contract.ToolAvailabilityPayload{
		Registered:  availability.Registered,
		Enabled:     availability.Enabled,
		Available:   availability.Available,
		Authorized:  availability.Authorized,
		Executable:  availability.Executable,
		Conflicted:  availability.Conflicted,
		ReasonCodes: append([]toolspkg.ReasonCode(nil), availability.ReasonCodes...),
	}
}

// toolDecisionPayload exposes policy outcomes without sharing mutable registry state.
func toolDecisionPayload(decision toolspkg.EffectiveToolDecision) contract.ToolPolicyDecisionPayload {
	return contract.ToolPolicyDecisionPayload{
		VisibleToOperator:    decision.VisibleToOperator,
		VisibleToSession:     decision.VisibleToSession,
		Callable:             decision.Callable,
		ApprovalRequired:     decision.ApprovalRequired,
		SystemPermissionMode: decision.SystemPermissionMode,
		SessionPolicyResult:  decision.SessionPolicyResult,
		AgentPolicyResult:    decision.AgentPolicyResult,
		RegistryPolicyResult: decision.RegistryPolicyResult,
		SourcePolicyResult:   decision.SourcePolicyResult,
		AvailabilityResult:   decision.AvailabilityResult,
		HookResult:           decision.HookResult,
		ReasonCodes:          append([]toolspkg.ReasonCode(nil), decision.ReasonCodes...),
	}
}

// bindToolSearch normalizes malformed search input into the shared tool error contract.
func (h *BaseHandlers) bindToolSearch(c *gin.Context) (contract.ToolSearchRequest, bool) {
	var req contract.ToolSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondToolError(c, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			"",
			fmt.Sprintf("%s: decode tool search request: %v", h.transportName(), err),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		))
		return contract.ToolSearchRequest{}, false
	}
	return req, true
}

// toolIDParam validates route IDs before they reach registry lookups.
func (h *BaseHandlers) toolIDParam(c *gin.Context) (toolspkg.ToolID, bool) {
	id := toolspkg.ToolID(strings.TrimSpace(c.Param("id")))
	if err := id.Validate(); err != nil {
		h.respondToolError(c, err)
		return "", false
	}
	return id, true
}

// operatorToolScope builds privileged projections from query parameters only.
func (h *BaseHandlers) operatorToolScope(c *gin.Context) toolspkg.Scope {
	return toolspkg.Scope{
		WorkspaceID: strings.TrimSpace(firstNonEmpty(c.Query("workspace_id"), c.Query("workspace"))),
		SessionID:   strings.TrimSpace(c.Query("session_id")),
		AgentName:   strings.TrimSpace(c.Query("agent_name")),
		Operator:    true,
	}
}

// sessionToolScope anchors session projections to the resolved route workspace and session IDs.
func (h *BaseHandlers) sessionToolScope(c *gin.Context, workspaceID string, sessionID string) toolspkg.Scope {
	return toolspkg.Scope{
		WorkspaceID: strings.TrimSpace(workspaceID),
		SessionID:   strings.TrimSpace(sessionID),
		AgentName:   strings.TrimSpace(c.Query("agent_name")),
	}
}

// toolScopeFromSearch lets request bodies narrow default scope without losing route context.
func toolScopeFromSearch(scope toolspkg.Scope, req contract.ToolSearchRequest) toolspkg.Scope {
	scope.WorkspaceID = firstNonEmpty(req.WorkspaceID, scope.WorkspaceID)
	scope.SessionID = firstNonEmpty(req.SessionID, scope.SessionID)
	scope.AgentName = firstNonEmpty(req.AgentName, scope.AgentName)
	return scope
}

func approvalScopeFromRequest(
	scope toolspkg.Scope,
	req contract.ToolApprovalRequest,
) (toolspkg.Scope, error) {
	sessionID, err := approvalScopeField("session_id", scope.SessionID, req.SessionID)
	if err != nil {
		return toolspkg.Scope{}, err
	}
	workspaceID, err := approvalScopeField("workspace_id", scope.WorkspaceID, req.WorkspaceID)
	if err != nil {
		return toolspkg.Scope{}, err
	}
	agentName, err := approvalScopeField("agent_name", scope.AgentName, req.AgentName)
	if err != nil {
		return toolspkg.Scope{}, err
	}
	scope.SessionID = sessionID
	scope.WorkspaceID = workspaceID
	scope.AgentName = agentName
	return scope, nil
}

func approvalScopeField(field string, scoped string, requested string) (string, error) {
	scoped = strings.TrimSpace(scoped)
	requested = strings.TrimSpace(requested)
	if scoped != "" && requested != "" && requested != scoped {
		return "", toolspkg.NewValidationError(
			field,
			toolspkg.ReasonApprovalTokenMismatch,
			field+" does not match approval scope",
		)
	}
	if requested != "" {
		return requested, nil
	}
	return scoped, nil
}
