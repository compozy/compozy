//go:build integration && !windows

package daemon

import (
	"context"
	"testing"
	"time"

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
	var lastRecords []acpmock.DiagnosticsRecord
	var lastSessionRecords []acpmock.DiagnosticsRecord
	var lastErr error
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		lastRecords, lastErr = acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if lastErr == nil {
			lastSessionRecords = acpmock.DiagnosticsForCompozySession(lastRecords, sessionID)
			if server, found := findHostedMCPStdioServer(lastSessionRecords, hostedMCPServerEarliest); found {
				return startHostedMCPClient(t, ctx, server)
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf(
				"wait for %s session %s hosted MCP diagnostics: %v; last error=%v; records=%#v",
				agentName,
				sessionID,
				ctx.Err(),
				lastErr,
				lastSessionRecords,
			)
		case <-ticker.C:
		}
	}
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
