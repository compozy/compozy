package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/heartbeat/source"
	tools "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) agentHeartbeatSources(
	ctx context.Context,
	scope tools.Scope,
	req tools.CallRequest,
) (tools.ToolResult, error) {
	var input struct {
		AgentName   string `json:"agent"`
		WorkspaceID string `json:"workspace"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return tools.ToolResult{}, err
	}
	target, err := n.authoredAgentTarget(ctx, req.ToolID, scope.ProfileID, input.WorkspaceID, input.AgentName)
	if err != nil {
		return tools.ToolResult{}, err
	}
	if _, err := os.Stat(target.agentPath); err != nil {
		return tools.ToolResult{}, err
	}
	if n.deps.AgentInstances == nil {
		return tools.ToolResult{}, errors.New("source storage unavailable")
	}
	api := n.deps.AgentInstances()
	if api == nil {
		return tools.ToolResult{}, errors.New("source storage unavailable")
	}
	storage, ok := api.Store.(source.ApprovalReader)
	if !ok {
		return tools.ToolResult{}, errors.New("source storage unavailable")
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(target.agentPath))
	if err != nil {
		return tools.ToolResult{}, err
	}
	payload, err := core.HeartbeatSourceCatalog(
		ctx,
		target.agentName,
		dir,
		target.agentLayer == "extension",
		source.CatalogScope{ProfileID: target.profileID, WorkspaceID: target.workspaceID},
		target.heartbeatConfig.Sources,
		storage,
		api.SourceExtensions,
	)
	if err != nil {
		return tools.ToolResult{}, err
	}
	return agentHeartbeatSourcesViews.result(req.View, payload)
}
