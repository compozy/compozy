package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// DeleteHeartbeatAgentHistory removes revisions owned by one effective authored definition.
func (g *HeartbeatRepo) DeleteHeartbeatAgentHistory(
	ctx context.Context,
	workspaceID string,
	agentName string,
	sourcePath string,
) error {
	if err := g.checkReady(ctx, "delete heartbeat agent history"); err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	agentName = strings.TrimSpace(agentName)
	sourcePath = strings.TrimSpace(sourcePath)
	if workspaceID == "" || agentName == "" || sourcePath == "" {
		return fmt.Errorf("store: delete heartbeat agent history requires workspace id, agent name, and source path")
	}
	if err := g.queries.DeleteHeartbeatAgentHistory(ctx, sqlcgen.DeleteHeartbeatAgentHistoryParams{
		WorkspaceID: workspaceID, AgentName: agentName, SourcePath: sourcePath,
	}); err != nil {
		return fmt.Errorf("store: delete heartbeat agent history: %w", err)
	}
	return nil
}

// DeleteSoulAgentHistory removes revisions owned by one effective authored definition.
func (g *SoulRepo) DeleteSoulAgentHistory(
	ctx context.Context,
	workspaceID string,
	agentName string,
	sourcePath string,
) error {
	if err := g.checkReady(ctx, "delete soul agent history"); err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	agentName = strings.TrimSpace(agentName)
	sourcePath = strings.TrimSpace(sourcePath)
	if workspaceID == "" || agentName == "" || sourcePath == "" {
		return fmt.Errorf("store: delete soul agent history requires workspace id, agent name, and source path")
	}
	if err := g.queries.DeleteSoulAgentHistory(ctx, sqlcgen.DeleteSoulAgentHistoryParams{
		WorkspaceID: workspaceID, AgentName: agentName, SourcePath: sourcePath,
	}); err != nil {
		return fmt.Errorf("store: delete soul agent history: %w", err)
	}
	return nil
}
