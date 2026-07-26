package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
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
	cleaned := filepath.Clean(strings.TrimSpace(agentDefinitionPath))
	if cleaned == "." || !strings.EqualFold(filepath.Base(cleaned), aghconfig.AgentDefinitionFileName) {
		return "", fmt.Errorf("heartbeat: purge agent history requires an AGENT.md source path")
	}
	agentDir := filepath.Dir(cleaned)
	agentsDir := filepath.Dir(agentDir)
	if filepath.Base(agentsDir) != aghconfig.AgentsDirName {
		return "", fmt.Errorf("heartbeat: purge agent history source path is outside an agents root")
	}
	root := filepath.Dir(agentsDir)
	if filepath.Base(root) == aghconfig.DirName {
		root = filepath.Dir(root)
	}
	relative, err := filepath.Rel(root, filepath.Join(agentDir, FileName))
	if err != nil {
		return "", fmt.Errorf("heartbeat: derive purge history source path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("heartbeat: purge history source path escapes its authored root")
	}
	return filepath.ToSlash(relative), nil
}
