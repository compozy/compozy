package workspace

import (
	"context"
	"errors"
	"testing"

	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestCoordinationCommandsAuthorizeWorkspaceAccess(t *testing.T) {
	t.Parallel()

	actor, err := taskpkg.DeriveAgentSessionActorContext("sess-a", "ws-a")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
	}
	ref := CoordinationRef{WorkspaceID: "ws-b", ScopeKind: InvitationScopeWorkspace}

	t.Run("Should allow a foreign read through the policy", func(t *testing.T) {
		t.Parallel()

		store := &recordingCoordinationCommandStore{}
		policy := &coordinationWorkspaceAccessPolicy{decision: workspaceaccess.Decision{Allowed: true}}
		commands := NewCoordinationService(store, policy)
		if _, err := commands.Get(t.Context(), ref, actor); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if store.getCalls != 1 || policy.calls != 1 || policy.last.Seam != workspaceaccess.SeamCoordination {
			t.Fatalf(
				"store/policy/request = %d/%d/%#v, want one authorized coordination read",
				store.getCalls,
				policy.calls,
				policy.last,
			)
		}
	})

	t.Run("Should deny a foreign write before consulting the policy", func(t *testing.T) {
		t.Parallel()

		store := &recordingCoordinationCommandStore{}
		policy := &coordinationWorkspaceAccessPolicy{decision: workspaceaccess.Decision{Allowed: true}}
		commands := NewCoordinationService(store, policy)
		_, err := commands.Set(t.Context(), SetCoordination{Ref: ref}, actor)
		if !errors.Is(err, taskpkg.ErrPermissionDenied) {
			t.Fatalf("Set() error = %v, want ErrPermissionDenied", err)
		}
		if store.setCalls != 0 || policy.calls != 0 {
			t.Fatalf("store/policy calls = %d/%d, want zero", store.setCalls, policy.calls)
		}
	})

	t.Run("Should deny an unscoped automation read through the actor-kind policy gate", func(t *testing.T) {
		t.Parallel()

		automation, err := taskpkg.DeriveAutomationActorContext("rule:nightly", "coordination.read")
		if err != nil {
			t.Fatalf("DeriveAutomationActorContext() error = %v", err)
		}
		store := &recordingCoordinationCommandStore{}
		policy := &coordinationWorkspaceAccessPolicy{}
		commands := NewCoordinationService(store, policy)
		_, err = commands.Get(t.Context(), ref, automation)
		if !errors.Is(err, taskpkg.ErrPermissionDenied) {
			t.Fatalf("Get(automation) error = %v, want ErrPermissionDenied", err)
		}
		if store.getCalls != 0 || policy.calls != 1 || policy.last.Actor.Kind != workspaceaccess.ActorAutomation ||
			policy.last.Actor.WorkspaceID != "" || policy.last.TargetWorkspaceID != ref.WorkspaceID {
			t.Fatalf("store/policy/request = %d/%d/%#v, want one denied automation policy call", store.getCalls, policy.calls, policy.last)
		}
	})
}

type coordinationWorkspaceAccessPolicy struct {
	decision workspaceaccess.Decision
	calls    int
	last     workspaceaccess.Request
}

func (p *coordinationWorkspaceAccessPolicy) Authorize(
	_ context.Context,
	req workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	p.calls++
	p.last = req
	return p.decision, nil
}

type recordingCoordinationCommandStore struct {
	getCalls int
	setCalls int
}

func (s *recordingCoordinationCommandStore) GetCoordination(
	context.Context,
	CoordinationRef,
) (CoordinationView, error) {
	s.getCalls++
	return CoordinationView{}, nil
}

func (s *recordingCoordinationCommandStore) SetCoordination(
	context.Context,
	SetCoordination,
	taskpkg.ActorContext,
) (CoordinationView, error) {
	s.setCalls++
	return CoordinationView{}, nil
}

func (*recordingCoordinationCommandStore) SetCoordinationInvitation(
	context.Context,
	SetInvitation,
	taskpkg.ActorContext,
) (CoordinationView, error) {
	return CoordinationView{}, nil
}
