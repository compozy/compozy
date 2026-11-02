package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// DeleteHeartbeatAgentHistory removes revisions owned by one effective authored definition.
func (g *HeartbeatRepo) DeleteHeartbeatAgentHistory(
	ctx context.Context,
	workspaceID string,
	agentName string,
	sourcePath string,
	profileID string,
) error {
	if err := g.checkReady(ctx, "delete heartbeat agent history"); err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	profileID = strings.TrimSpace(profileID)
	if profileID == "" && workspaceID != "" {
		profileID = store.DefaultProfileID
	}
	agentName = strings.TrimSpace(agentName)
	sourcePath = strings.TrimSpace(sourcePath)
	if (workspaceID == "" && profileID == "") || agentName == "" || sourcePath == "" {
		return fmt.Errorf("store: delete heartbeat agent history requires workspace id, agent name, and source path")
	}
	if err := g.queries.DeleteHeartbeatAgentHistory(ctx, sqlcgen.DeleteHeartbeatAgentHistoryParams{
		ProfileID: profileID, WorkspaceID: workspaceID, AgentName: agentName, SourcePath: sourcePath,
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
	profileID string,
) error {
	if err := g.checkReady(ctx, "delete soul agent history"); err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	profileID = strings.TrimSpace(profileID)
	if profileID == "" && workspaceID != "" {
		profileID = store.DefaultProfileID
	}
	agentName = strings.TrimSpace(agentName)
	sourcePath = strings.TrimSpace(sourcePath)
	if (workspaceID == "" && profileID == "") || agentName == "" || sourcePath == "" {
		return fmt.Errorf("store: delete soul agent history requires workspace id, agent name, and source path")
	}
	if err := g.queries.DeleteSoulAgentHistory(ctx, sqlcgen.DeleteSoulAgentHistoryParams{
		ProfileID: profileID, WorkspaceID: workspaceID, AgentName: agentName, SourcePath: sourcePath,
	}); err != nil {
		return fmt.Errorf("store: delete soul agent history: %w", err)
	}
	return nil
}

func authoringProfileID(profileID string) string {
	if id := strings.TrimSpace(profileID); id != "" {
		return id
	}
	return store.DefaultProfileID
}
