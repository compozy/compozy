package extensionpkg

import (
	"strings"
	"testing"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func TestAppendProfileHookDeclarationsInjectsWorkspaceScopedMatchers(t *testing.T) {
	t.Parallel()

	const profileID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	projected := func(decl hookspkg.HookDecl) *Extension {
		return &Extension{Hooks: []hookspkg.HookDecl{decl}}
	}

	// Invariant: a workspace-scoped installation always binds the workspace into the
	// matcher, and the injected declaration validates and matches only that workspace.
	// A workspace-owned hook must never be stored unscoped.
	// Owner: extension profile hook projection; canonical suite: this projection.
	t.Run("Should inject workspace id for workspace-scoped subagent hooks", func(t *testing.T) {
		t.Parallel()

		decls, err := appendProfileHookDeclarations(nil, projected(hookspkg.HookDecl{
			Name:   "subagent-audit",
			Event:  hookspkg.HookSubagentSettled,
			Source: hookspkg.HookSourceNative,
			Mode:   hookspkg.HookModeAsync,
		}), profileID, "ws-1", nil)
		if err != nil {
			t.Fatalf("appendProfileHookDeclarations() error = %v", err)
		}
		if len(decls) != 1 {
			t.Fatalf("len(decls) = %d, want 1", len(decls))
		}
		if got, want := decls[0].Matcher.WorkspaceID, "ws-1"; got != want {
			t.Fatalf("decls[0].Matcher.WorkspaceID = %q, want %q", got, want)
		}
		if err := hookspkg.ValidateMatcherForEvent(decls[0].Event, decls[0].Matcher); err != nil {
			t.Fatalf("ValidateMatcherForEvent() error = %v", err)
		}
	})

	t.Run("Should not inject workspace id for global subagent hooks", func(t *testing.T) {
		t.Parallel()

		decls, err := appendProfileHookDeclarations(nil, projected(hookspkg.HookDecl{
			Name:   "subagent-audit",
			Event:  hookspkg.HookSubagentSettled,
			Source: hookspkg.HookSourceNative,
			Mode:   hookspkg.HookModeAsync,
		}), profileID, "", []string{"ws-1"})
		if err != nil {
			t.Fatalf("appendProfileHookDeclarations() error = %v", err)
		}
		if len(decls) != 1 {
			t.Fatalf("len(decls) = %d, want 1", len(decls))
		}
		if got := decls[0].Matcher.WorkspaceID; got != "" {
			t.Fatalf("decls[0].Matcher.WorkspaceID = %q, want empty", got)
		}
		if err := hookspkg.ValidateMatcherForEvent(decls[0].Event, decls[0].Matcher); err != nil {
			t.Fatalf("ValidateMatcherForEvent() error = %v", err)
		}
	})

	scopedFamilies := []struct {
		name             string
		decl             hookspkg.HookDecl
		matchesWorkspace func(hookspkg.HookMatcher, string) bool
	}{
		{
			name: "event hooks",
			decl: hookspkg.HookDecl{
				Name:   "event-observer",
				Event:  hookspkg.HookEventPostRecord,
				Source: hookspkg.HookSourceNative,
				Mode:   hookspkg.HookModeAsync,
			},
			matchesWorkspace: func(matcher hookspkg.HookMatcher, workspaceID string) bool {
				var payload hookspkg.EventRecordPayload
				payload.WorkspaceID = workspaceID
				return matcher.MatchesEvent(payload)
			},
		},
		{
			name: "message hooks",
			decl: hookspkg.HookDecl{
				Name:   "message-observer",
				Event:  hookspkg.HookMessageDelta,
				Source: hookspkg.HookSourceNative,
				Mode:   hookspkg.HookModeAsync,
			},
			matchesWorkspace: func(matcher hookspkg.HookMatcher, workspaceID string) bool {
				var payload hookspkg.MessagePayload
				payload.WorkspaceID = workspaceID
				return matcher.MatchesMessage(payload)
			},
		},
		{
			name: "context hooks",
			decl: hookspkg.HookDecl{
				Name:   "context-observer",
				Event:  hookspkg.HookContextPreCompact,
				Source: hookspkg.HookSourceNative,
				Mode:   hookspkg.HookModeSync,
			},
			matchesWorkspace: func(matcher hookspkg.HookMatcher, workspaceID string) bool {
				var payload hookspkg.ContextCompactionPayload
				payload.WorkspaceID = workspaceID
				return matcher.MatchesContextCompact(payload)
			},
		},
	}

	t.Run("Should scope every family whose matcher compares the workspace", func(t *testing.T) {
		for _, tc := range scopedFamilies {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				decls, err := appendProfileHookDeclarations(
					nil,
					projected(tc.decl),
					profileID,
					"ws-1",
					nil,
				)
				if err != nil {
					t.Fatalf("appendProfileHookDeclarations() error = %v", err)
				}
				if len(decls) != 1 {
					t.Fatalf("len(decls) = %d, want 1", len(decls))
				}
				if got, want := decls[0].Matcher.WorkspaceID, "ws-1"; got != want {
					t.Fatalf("decls[0].Matcher.WorkspaceID = %q, want %q", got, want)
				}
				if err := hookspkg.ValidateMatcherForEvent(decls[0].Event, decls[0].Matcher); err != nil {
					t.Fatalf("ValidateMatcherForEvent() error = %v", err)
				}
				if !tc.matchesWorkspace(decls[0].Matcher, "ws-1") {
					t.Fatal("matcher does not match the installing workspace")
				}
				if tc.matchesWorkspace(decls[0].Matcher, "ws-2") {
					t.Fatal("matcher matches a foreign workspace, want workspace-scoped hook")
				}
			})
		}
	})

	t.Run("Should reject a declared workspace that differs from the installation", func(t *testing.T) {
		t.Parallel()

		_, err := appendProfileHookDeclarations(nil, projected(hookspkg.HookDecl{
			Name:   "subagent-audit",
			Event:  hookspkg.HookSubagentSettled,
			Source: hookspkg.HookSourceNative,
			Mode:   hookspkg.HookModeAsync,
			Matcher: hookspkg.HookMatcher{
				WorkspaceID: "ws-2",
			},
		}), profileID, "ws-1", nil)
		if err == nil {
			t.Fatal("appendProfileHookDeclarations() error = nil, want workspace mismatch failure")
		}
		if !strings.Contains(err.Error(), `declares workspace "ws-2"`) {
			t.Fatalf("appendProfileHookDeclarations() error = %v, want declared workspace mismatch", err)
		}
	})
}
