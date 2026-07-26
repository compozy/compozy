package extensionpkg

import (
	"context"
	"testing"

	apicontract "github.com/compozy/agh/internal/api/contract"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/heartbeat"
)

func TestHostAPIAuthoredContextWriteBypassRejections(t *testing.T) {
	t.Parallel()

	t.Run("Should reject Soul write without write grant before managed authoring runs", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		soulAuthoring := &hostAPITestSoulAuthoring{
			result: hostAPITestSoulMutationResult(env.workspaceID, "coder", env.workspace.RootDir),
		}
		env.handler.soulAuthoring = soulAuthoring
		env.grant(
			"ext-soul-readonly",
			[]string{string(extensioncontract.HostAPIMethodAgentsSoulGet)},
			[]string{"soul.read"},
		)

		_, err := env.handler.Handle(
			t.Context(),
			"ext-soul-readonly",
			string(extensioncontract.HostAPIMethodAgentsSoulPut),
			mustHostAPIAuthoredJSON(t, apicontract.AgentSoulPutRequest{
				WorkspaceID:    env.workspaceID,
				AgentName:      "coder",
				Body:           "attempted direct write",
				ExpectedDigest: "sha256:old",
			}),
		)
		assertCapabilityDenied(t, err, string(extensioncontract.HostAPIMethodAgentsSoulPut))
		if soulAuthoring.putCalls != 0 {
			t.Fatalf("soul put calls = %d, want 0 without write grant", soulAuthoring.putCalls)
		}
	})

	t.Run("Should reject Heartbeat write without write grant before managed authoring runs", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		heartbeatAuthoring := &hostAPITestHeartbeatAuthoringProbe{}
		env.handler.heartbeatAuthor = heartbeatAuthoring
		env.grant(
			"ext-heartbeat-readonly",
			[]string{string(extensioncontract.HostAPIMethodAgentsHeartbeatStatus)},
			[]string{"heartbeat.read"},
		)

		_, err := env.handler.Handle(
			t.Context(),
			"ext-heartbeat-readonly",
			string(extensioncontract.HostAPIMethodAgentsHeartbeatPut),
			mustHostAPIAuthoredJSON(t, apicontract.HeartbeatPutRequest{
				WorkspaceID:    env.workspaceID,
				AgentName:      "coder",
				Body:           "---\nversion: 1\nsummary: denied\n---\nAttempted direct write.",
				ExpectedDigest: "sha256:old",
			}),
		)
		assertCapabilityDenied(t, err, string(extensioncontract.HostAPIMethodAgentsHeartbeatPut))
		if heartbeatAuthoring.putCalls != 0 {
			t.Fatalf("heartbeat put calls = %d, want 0 without write grant", heartbeatAuthoring.putCalls)
		}
	})

	t.Run("Should reject Heartbeat wake without write grant before wake service runs", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		wake := &hostAPITestHeartbeatWake{}
		env.handler.heartbeatWake = wake
		env.grant(
			"ext-heartbeat-readonly-wake",
			[]string{string(extensioncontract.HostAPIMethodAgentsHeartbeatStatus)},
			[]string{"heartbeat.read"},
		)

		_, err := env.handler.Handle(
			t.Context(),
			"ext-heartbeat-readonly-wake",
			string(extensioncontract.HostAPIMethodAgentsHeartbeatWake),
			mustHostAPIAuthoredJSON(t, apicontract.HeartbeatWakeRequest{
				WorkspaceID: env.workspaceID,
				AgentName:   "coder",
				SessionID:   "sess-health",
				Source:      apicontract.HeartbeatWakeSourceManual,
			}),
		)
		assertCapabilityDenied(t, err, string(extensioncontract.HostAPIMethodAgentsHeartbeatWake))
		if wake.calls != 0 {
			t.Fatalf("heartbeat wake calls = %d, want 0 without write grant", wake.calls)
		}
	})
}

type hostAPITestHeartbeatAuthoringProbe struct {
	putCalls int
}

func (h *hostAPITestHeartbeatAuthoringProbe) Validate(
	context.Context,
	heartbeat.ValidateRequest,
) (heartbeat.ValidateResult, error) {
	return heartbeat.ValidateResult{}, nil
}

func (h *hostAPITestHeartbeatAuthoringProbe) Put(
	context.Context,
	heartbeat.PutRequest,
) (heartbeat.MutationResult, error) {
	h.putCalls++
	return heartbeat.MutationResult{}, nil
}

func (h *hostAPITestHeartbeatAuthoringProbe) Delete(
	context.Context,
	heartbeat.DeleteRequest,
) (heartbeat.MutationResult, error) {
	return heartbeat.MutationResult{}, nil
}

func (h *hostAPITestHeartbeatAuthoringProbe) History(
	context.Context,
	heartbeat.HistoryRequest,
) (heartbeat.HistoryResult, error) {
	return heartbeat.HistoryResult{}, nil
}

func (h *hostAPITestHeartbeatAuthoringProbe) Rollback(
	context.Context,
	heartbeat.RollbackRequest,
) (heartbeat.MutationResult, error) {
	return heartbeat.MutationResult{}, nil
}
