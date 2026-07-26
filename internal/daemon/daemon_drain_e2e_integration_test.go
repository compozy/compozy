//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/admission"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

func TestDaemonE2EDrainRefusesNewSessionsUntilUndrained(t *testing.T) {
	t.Parallel()
	acpmock.RequireDriver(t)

	const agentName = "drain-agent"
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
				FixtureAgent: "alpha",
				AgentName:    agentName,
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	active := createFixtureBackedSession(t, ctx, harness, agentName, "admitted-before-drain")
	var draining aghcontract.DrainStatusResponse
	if err := harness.HTTPJSON(ctx, http.MethodPost, "/api/drain", nil, &draining); err != nil {
		t.Fatalf("HTTP drain error = %v", err)
	}
	if draining.State != aghcontract.DrainStateDraining {
		t.Fatalf("drain state = %q, want %q", draining.State, aghcontract.DrainStateDraining)
	}

	status, payload := createSessionHTTPFailure(t, ctx, harness, aghcontract.CreateSessionRequest{
		AgentName:     agentName,
		Name:          "refused-while-draining",
		WorkspacePath: harness.WorkspaceRoot,
	})
	if status != http.StatusServiceUnavailable || payload.Error != admission.ErrDraining.Error() {
		t.Fatalf("draining create = status %d payload %#v, want 503 %q", status, payload, admission.ErrDraining)
	}
	visible, err := harness.GetSession(ctx, active.ID)
	if err != nil {
		t.Fatalf("GetSession(admitted) error = %v", err)
	}
	if visible.ID != active.ID {
		t.Fatalf("visible session id = %q, want %q", visible.ID, active.ID)
	}

	var activeState aghcontract.DrainStatusResponse
	if err := harness.UDSJSON(ctx, http.MethodPost, "/api/undrain", nil, &activeState); err != nil {
		t.Fatalf("UDS undrain error = %v", err)
	}
	if activeState.State != aghcontract.DrainStateActive {
		t.Fatalf("undrain state = %q, want %q", activeState.State, aghcontract.DrainStateActive)
	}
	restored := createFixtureBackedSession(t, ctx, harness, agentName, "admitted-after-undrain")
	if restored.ID == active.ID {
		t.Fatalf("restored session id = %q, want a new session", restored.ID)
	}
}
