package main

import (
	"encoding/json/v2"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func (a *mockAgent) writeSessionDiagnostics(
	event string,
	sessionID string,
	servers []acpsdk.McpServer,
	meta any,
) error {
	var decoded struct {
		ClaudeCode struct {
			Options struct {
				StrictMCPConfig *bool `json:"strictMcpConfig"`
			} `json:"options"`
		} `json:"claudeCode"`
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if len(servers) == 0 && decoded.ClaudeCode.Options.StrictMCPConfig == nil {
		return nil
	}
	return a.writeDiagnostics(acpmock.DiagnosticsRecord{
		AgentName:       a.agent.Name,
		SessionID:       sessionID,
		LifecycleEvent:  event,
		MCPServers:      append([]acpsdk.McpServer(nil), servers...),
		StrictMCPConfig: decoded.ClaudeCode.Options.StrictMCPConfig,
	})
}
