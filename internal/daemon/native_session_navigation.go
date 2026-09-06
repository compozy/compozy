package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/transcript"
)

type sessionSearchInput struct {
	Workspace string `json:"workspace"`
	SessionID string `json:"session_id"`
	Query     string `json:"q"`
	Limit     int    `json:"limit,omitempty"`
}

func (n *daemonNativeTools) sessionSearch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionSearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := (transcript.SearchQuery{Query: input.Query, Limit: input.Limit}).Normalize()
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	id, err := n.nativeSessionInputScope(ctx, scope, req.ToolID, input.Workspace, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	reader, ok := n.deps.Sessions.(core.SessionNavigationReader)
	if !ok {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session navigation is unavailable")
	}
	result, err := reader.TranscriptSearch(ctx, id, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		result,
		fmt.Sprintf("%d matching messages (truncated: %t)", len(result.Matches), result.Truncated),
	)
}

func (n *daemonNativeTools) sessionOutline(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	id, err := n.nativeSessionInputScope(ctx, scope, req.ToolID, input.WorkspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	reader, ok := n.deps.Sessions.(core.SessionNavigationReader)
	if !ok {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session navigation is unavailable")
	}
	result, err := reader.TranscriptOutline(ctx, id)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(result, fmt.Sprintf("%d operator messages", len(result.Entries)))
}

func (n *daemonNativeTools) sessionNavigationToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionSearch:  {call: n.sessionSearch, availability: availability},
		toolspkg.ToolIDSessionOutline: {call: n.sessionOutline, availability: availability},
	}
}
