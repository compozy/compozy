package heartbeat

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/managedsidecar"
)

// WorkspaceRef identifies the workspace scope that owns managed authoring history.
type WorkspaceRef struct {
	WorkspaceID string
}

// PurgeAgentHistory removes managed revisions owned by one effective definition.
func (s *ManagedHeartbeatAuthoringService) PurgeAgentHistory(
	ctx context.Context,
	ref WorkspaceRef,
	agentName string,
	sourcePath string,
) error {
	if ctx == nil {
		return fmt.Errorf("heartbeat: purge agent history context is required")
	}
	if s == nil || s.store == nil {
		return fmt.Errorf("heartbeat: purge agent history store is required")
	}
	workspaceID := strings.TrimSpace(ref.WorkspaceID)
	name := strings.TrimSpace(agentName)
	path, err := heartbeatRevisionSourcePath(sourcePath)
	if err != nil {
		return err
	}
	switch {
	case workspaceID == "":
		return fmt.Errorf("heartbeat: purge agent history workspace id is required")
	case name == "":
		return fmt.Errorf("heartbeat: purge agent history agent name is required")
	case path == "":
		return fmt.Errorf("heartbeat: purge agent history source path is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.DeleteHeartbeatAgentHistory(ctx, workspaceID, name, path); err != nil {
		return fmt.Errorf("heartbeat: purge agent history: %w", err)
	}
	return nil
}

func heartbeatRevisionSourcePath(agentDefinitionPath string) (string, error) {
	path, err := managedsidecar.RevisionSourcePath(agentDefinitionPath, FileName)
	if err != nil {
		return "", fmt.Errorf("heartbeat: %w", err)
	}
	return path, nil
}
