//go:build integration && !windows

package daemon

import (
	"context"
	"testing"

	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func hostedMCPClientForSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
	sessionID string,
) *sdkmcp.ClientSession {
	t.Helper()
	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%s) = missing", agentName)
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%s) error = %v", agentName, err)
	}
	return startHostedMCPClient(
		t,
		ctx,
		requireHostedMCPStdioServer(
			t,
			acpmock.DiagnosticsForCompozySession(records, sessionID),
			hostedMCPServerEarliest,
		),
	)
}

func closeHostedMCPClient(t testing.TB, client *sdkmcp.ClientSession) {
	t.Helper()
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		t.Errorf("Close(hosted MCP client) error = %v", err)
	}
}
