package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// rememberedAllow reports an operator "Always allow" stored for this exact read scope. Lookup
// failures fall back to asking, which never widens reach.
func (b *workspaceAccessPromptBridge) rememberedAllow(ctx context.Context, key *toolspkg.ApprovalGrantKey) bool {
	if key == nil {
		return false
	}
	grant, found, err := b.grants.LookupApprovalGrant(ctx, *key)
	if err != nil {
		b.logger.Warn(
			"daemon: workspace access grant lookup failed; asking instead",
			"workspace_id", key.WorkspaceID,
			"agent_name", key.AgentName,
			"tool_id", key.ToolID,
			"error", err,
		)
		return false
	}
	return found && grant.AccessScope == key.AccessScope && grant.AgentName == key.AgentName &&
		grant.Decision == toolspkg.ApprovalGrantAllow
}

func (b *workspaceAccessPromptBridge) rememberAllow(ctx context.Context, key toolspkg.ApprovalGrantKey) error {
	if _, err := b.grants.PutApprovalGrant(ctx, toolspkg.ApprovalGrant{
		ApprovalGrantKey: key,
		Decision:         toolspkg.ApprovalGrantAllow,
	}); err != nil {
		return fmt.Errorf("daemon: remember workspace access grant: %w", err)
	}
	return nil
}

func recordWorkspaceReadConsent(ctx context.Context, policy workspaceaccess.Policy, req workspaceaccess.Request) {
	if auditor, ok := policy.(interface {
		RecordToolConsent(context.Context, workspaceaccess.Request)
	}); ok {
		auditor.RecordToolConsent(ctx, req)
	}
}
