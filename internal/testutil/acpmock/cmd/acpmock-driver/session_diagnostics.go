package main

import (
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func (a *mockAgent) writeSessionDiagnostics(
	event string,
	sessionID string,
	servers []acpsdk.McpServer,
) error {
	if len(servers) == 0 {
		return nil
	}
	return a.writeDiagnostics(acpmock.DiagnosticsRecord{
		AgentName:      a.agent.Name,
		SessionID:      sessionID,
		LifecycleEvent: event,
		MCPServers:     append([]acpsdk.McpServer(nil), servers...),
	})
}
