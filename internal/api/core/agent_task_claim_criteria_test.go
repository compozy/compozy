package core

import (
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestAgentTaskClaimCriteria(t *testing.T) {
	t.Parallel()

	t.Run("Should copy session soul provenance into claim criteria", func(t *testing.T) {
		t.Parallel()

		handlers := &BaseHandlers{}
		caller := agentidentity.Caller{
			Session: agentidentity.SessionSnapshot{
				ID:             "sess-agent",
				AgentName:      "coder",
				WorkspaceID:    "ws-1",
				State:          session.StateActive,
				SoulSnapshotID: "soul-snapshot-1",
				SoulDigest:     "sha256:resolved",
			},
			Actor: taskpkg.ActorContext{
				Actor: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-agent"},
			},
		}

		criteria, err := handlers.agentTaskClaimCriteria(
			t.Context(),
			contract.AgentTaskClaimNextRequest{LeaseSeconds: 60},
			caller,
		)
		if err != nil {
			t.Fatalf("agentTaskClaimCriteria() error = %v", err)
		}
		if criteria.Soul == nil {
			t.Fatal("ClaimCriteria.Soul = nil, want session provenance")
		}
		if criteria.Soul.SnapshotID != "soul-snapshot-1" ||
			criteria.Soul.Digest != "sha256:resolved" ||
			criteria.Soul.AgentName != "coder" {
			t.Fatalf("ClaimCriteria.Soul = %#v, want caller soul provenance", criteria.Soul)
		}
	})

	t.Run("Should propagate a foreign workspace to the task authorization seam", func(t *testing.T) {
		t.Parallel()

		handlers := &BaseHandlers{}
		caller := agentidentity.Caller{
			Session: agentidentity.SessionSnapshot{
				ID:          "sess-agent",
				AgentName:   "coder",
				WorkspaceID: "ws-home",
				State:       session.StateActive,
			},
			Actor: taskpkg.ActorContext{
				Actor: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-agent"},
			},
		}

		criteria, err := handlers.agentTaskClaimCriteria(
			t.Context(),
			contract.AgentTaskClaimNextRequest{WorkspaceID: "ws-target", LeaseSeconds: 60},
			caller,
		)
		if err != nil {
			t.Fatalf("agentTaskClaimCriteria() error = %v", err)
		}
		if criteria.WorkspaceID != "ws-target" {
			t.Fatalf("ClaimCriteria.WorkspaceID = %q, want %q", criteria.WorkspaceID, "ws-target")
		}
	})
}
