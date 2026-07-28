package task

import "testing"

func TestDeriveAgentSessionActorContextForOrigin(t *testing.T) {
	t.Parallel()

	t.Run("Should derive actor context for HTTP agent ingress", func(t *testing.T) {
		t.Parallel()

		actor, err := DeriveAgentSessionActorContextForOrigin(
			"sess-http",
			"ws-alpha",
			OriginKindHTTP,
			"agent.context",
		)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContextForOrigin() error = %v", err)
		}
		if actor.Actor.Kind != ActorKindAgentSession ||
			actor.Actor.Ref != "sess-http" ||
			actor.Origin.Kind != OriginKindHTTP ||
			actor.Origin.Ref != "agent.context" ||
			actor.Scope.SessionID != "sess-http" ||
			actor.Scope.WorkspaceID != "ws-alpha" {
			t.Fatalf("actor = %#v, want HTTP agent-session origin", actor)
		}
	})

	t.Run("Should reject browser web origin for agent-session ingress", func(t *testing.T) {
		t.Parallel()

		_, err := DeriveAgentSessionActorContextForOrigin(
			"sess-web",
			"ws-alpha",
			OriginKindWeb,
			"agent.context",
		)
		if err == nil {
			t.Fatal("DeriveAgentSessionActorContextForOrigin() error = nil, want validation error")
		}
	})

	t.Run("Should reject contradictory caller scope", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			scope CallerScope
		}{
			{
				name:  "Should reject an empty agent workspace",
				scope: CallerScope{SessionID: "sess-http"},
			},
			{
				name: "Should reject a mismatched scoped session",
				scope: CallerScope{
					SessionID:   "sess-other",
					WorkspaceID: "ws-alpha",
				},
			},
			{
				name: "Should reject operator authority for an agent session",
				scope: CallerScope{
					SessionID:   "sess-http",
					WorkspaceID: "ws-alpha",
					Operator:    true,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				actor := ActorContext{
					Actor:     ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-http"},
					Origin:    Origin{Kind: OriginKindHTTP, Ref: "agent.context"},
					Authority: FullAccessAuthority(),
					Scope:     tt.scope,
				}
				if err := actor.Validate(); err == nil {
					t.Fatal("ActorContext.Validate() error = nil, want scope validation error")
				}
			})
		}
	})
}
