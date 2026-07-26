package main

import (
	"os"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func (a *mockAgent) writeSessionDiagnostics(
	event string,
	sessionID string,
	servers []acpsdk.McpServer,
) error {
	networkEnv := networkEnvironmentNames()
	if len(servers) == 0 && len(networkEnv) == 0 {
		return nil
	}
	return a.writeDiagnostics(acpmock.DiagnosticsRecord{
		AgentName:      a.agent.Name,
		SessionID:      sessionID,
		LifecycleEvent: event,
		MCPServers:     append([]acpsdk.McpServer(nil), servers...),
		NetworkEnv:     networkEnv,
	})
}

func networkEnvironmentNames() []string {
	names := make([]string, 0, 2)
	for _, name := range []string{"AGH_SESSION_CHANNEL", "AGH_PEER_ID"} {
		if _, present := os.LookupEnv(name); present {
			names = append(names, name)
		}
	}
	return names
}
