package daemon

import (
	"context"
	"testing"

	apitest "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestSessionLineageClassificationUsesSessionType(t *testing.T) {
	t.Parallel()

	t.Run("Should not enforce an empty spawned policy on a system provenance session", func(t *testing.T) {
		t.Parallel()

		inputs := toolspkg.PolicyInputs{}
		err := applySessionToolPolicy(&inputs, &session.Info{
			ID: "sess-goal", Type: session.SessionTypeSystem,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-orchestrator", RootSessionID: "sess-orchestrator", SpawnDepth: 1,
			},
		})
		if err != nil {
			t.Fatalf("applySessionToolPolicy(system provenance) error = %v", err)
		}
		if inputs.Session.Enforced {
			t.Fatalf("system provenance policy = %#v, want unenforced empty policy", inputs.Session)
		}
	})

	t.Run("Should continue enforcing an empty policy on a spawned session", func(t *testing.T) {
		t.Parallel()

		inputs := toolspkg.PolicyInputs{}
		err := applySessionToolPolicy(&inputs, &session.Info{
			ID: "sess-worker", Type: session.SessionTypeSpawned,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-root", RootSessionID: "sess-root", SpawnDepth: 1,
			},
		})
		if err != nil {
			t.Fatalf("applySessionToolPolicy(spawned) error = %v", err)
		}
		if !inputs.Session.Enforced || len(inputs.Session.Tools) != 0 {
			t.Fatalf("spawned policy = %#v, want enforced empty policy", inputs.Session)
		}
	})

	t.Run("Should classify only spawned session metadata as a memory subagent", func(t *testing.T) {
		t.Parallel()

		info := &session.Info{
			ID: "sess-goal", Type: session.SessionTypeSystem,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-orchestrator", RootSessionID: "sess-orchestrator", SpawnDepth: 1,
			},
		}
		native := &daemonNativeTools{deps: &daemonNativeToolsDeps{Sessions: apitest.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) { return info, nil },
		}}}
		kind, err := native.memoryCallerActorKind(
			context.Background(), toolspkg.Scope{SessionID: info.ID}, toolspkg.CallRequest{},
		)
		if err != nil || kind != nativeMemoryActorKindRoot {
			t.Fatalf("memoryCallerActorKind(system provenance) = %q, %v, want root", kind, err)
		}
		info.Type = session.SessionTypeSpawned
		kind, err = native.memoryCallerActorKind(
			context.Background(), toolspkg.Scope{SessionID: info.ID}, toolspkg.CallRequest{},
		)
		if err != nil || kind != nativeMemoryActorKindSubagent {
			t.Fatalf("memoryCallerActorKind(spawned) = %q, %v, want subagent", kind, err)
		}
	})
}
