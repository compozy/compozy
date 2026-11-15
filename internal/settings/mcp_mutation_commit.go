package settings

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// mcpServerWrite carries one normalized definition from validation to its approval-aware commit.
type mcpServerWrite struct {
	scope       ScopeKind
	workspaceID string
	profileName string
	root        string
	name        string
	target      compozyconfig.WriteTarget
	sources     map[string][]mcpSourceEntry
	server      compozyconfig.MCPServer
}

// commitMCPServerWithApproval records workspace consent around the write when a recorder is wired.
func (s *service) commitMCPServerWithApproval(
	ctx context.Context,
	w *mcpServerWrite,
) (mcpDefinitionMutationOutcome, string, error) {
	mutationOutcome := mcpDefinitionMutationUnchanged
	commit := func() error {
		var commitErr error
		mutationOutcome, commitErr = s.commitMCPServerDefinition(
			ctx, w.scope, w.workspaceID, w.profileName, w.root, w.name, w.target, w.sources, w.server,
		)
		return commitErr
	}
	if w.workspaceID == "" || s.workspaceMCPApprovals == nil {
		err := commit()
		return mutationOutcome, unavailableMCPWriteApprovalWarning(w.workspaceID, err), err
	}
	approvalWarning, err := s.workspaceMCPApprovals.RecordWorkspaceMCPWrite(ctx, WorkspaceMCPWrite{
		ProfileName: w.profileName,
		WorkspaceID: w.workspaceID,
		SourceID:    w.target.Path(),
		Server:      w.server,
		Write:       commit,
		ReadCommitted: func() (compozyconfig.MCPServer, error) {
			_, current, readErr := s.resolveMCPTargetContext(ctx, w.scope, w.workspaceID, w.profileName)
			if readErr != nil {
				return compozyconfig.MCPServer{}, readErr
			}
			entry, ok := mcpSourceForTarget(w.name, w.target.Kind(), current)
			if !ok {
				return compozyconfig.MCPServer{}, fmt.Errorf("settings: committed MCP definition is unavailable")
			}
			return entry.Server, nil
		},
	})
	return mutationOutcome, approvalWarning, err
}
