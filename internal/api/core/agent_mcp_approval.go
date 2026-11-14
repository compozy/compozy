package core

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

// AgentMCPApprovalContext binds an agent sidecar write to the caller's workspace and profile.
type AgentMCPApprovalContext struct {
	Recorder    settingspkg.WorkspaceMCPApprovalRecorder
	WorkspaceID string
	ProfileName string
	Warnings    *[]string
}

func agentMCPApprovalContext(
	service SettingsService,
	workspaceID, profileName string,
	warnings *[]string,
) AgentMCPApprovalContext {
	recorder, _ := service.(settingspkg.WorkspaceMCPApprovalRecorder)
	return AgentMCPApprovalContext{
		Recorder:    recorder,
		WorkspaceID: workspaceID,
		ProfileName: profileName,
		Warnings:    warnings,
	}
}

func commitAgentMCPDefinition(
	ctx context.Context,
	path string,
	servers []config.MCPServer,
	write func() error,
	read func() (config.AgentDef, error),
	approval AgentMCPApprovalContext,
) error {
	if approval.WorkspaceID == "" ||
		!slices.ContainsFunc(servers, func(server config.MCPServer) bool { return !server.Reference }) {
		return write()
	}
	if approval.Recorder == nil {
		if err := write(); err != nil {
			return err
		}
		if approval.Warnings != nil {
			*approval.Warnings = append(
				*approval.Warnings,
				"MCP servers saved but approval is unavailable; waiting for approval",
			)
		}
		return nil
	}
	committed := false
	for _, server := range servers {
		if server.Reference {
			continue
		}
		warning, err := approval.Recorder.RecordWorkspaceMCPWrite(ctx, settingspkg.WorkspaceMCPWrite{
			ProfileName: approval.ProfileName,
			WorkspaceID: approval.WorkspaceID,
			SourceID:    filepath.Join(filepath.Dir(path), config.MCPJSONName),
			Server:      server,
			Write: func() error {
				if committed {
					return nil
				}
				if err := write(); err != nil {
					return err
				}
				committed = true
				return nil
			},
			ReadCommitted: func() (config.MCPServer, error) {
				agent, err := read()
				if err != nil {
					return config.MCPServer{}, err
				}
				index := slices.IndexFunc(
					agent.MCPServers,
					func(current config.MCPServer) bool { return current.Name == server.Name && !current.Reference },
				)
				if index < 0 {
					return config.MCPServer{}, fmt.Errorf("api: committed agent MCP definition is unavailable")
				}
				return agent.MCPServers[index], nil
			},
		})
		if err != nil {
			return err
		}
		if warning != "" && approval.Warnings != nil {
			*approval.Warnings = append(*approval.Warnings, warning)
		}
	}
	if !committed {
		return write()
	}
	return nil
}

// AgentMCPApprovalForSettings is shared by native agent creation and HTTP/UDS authoring.
func AgentMCPApprovalForSettings(service SettingsService) settingspkg.WorkspaceMCPApprovalRecorder {
	recorder, _ := service.(settingspkg.WorkspaceMCPApprovalRecorder)
	return recorder
}
