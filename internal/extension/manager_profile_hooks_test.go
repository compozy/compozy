package extensionpkg

import (
	"strings"
	"testing"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func TestAppendProfileHookDeclarationsInjectsWorkspaceMatcherForSubagentHooks(t *testing.T) {
	t.Parallel()

	const profileID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	projected := func(decl hookspkg.HookDecl) *Extension {
		return &Extension{Hooks: []hookspkg.HookDecl{decl}}
	}

	// Invariant: a workspace-scoped installation injects workspace_id only where the
	// event family accepts it, and the injected declaration still validates.
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

	t.Run("Should not inject workspace id for event hooks that reject the field", func(t *testing.T) {
		t.Parallel()

		decls, err := appendProfileHookDeclarations(nil, projected(hookspkg.HookDecl{
			Name:   "event-observer",
			Event:  hookspkg.HookEventPreRecord,
			Source: hookspkg.HookSourceNative,
			Mode:   hookspkg.HookModeAsync,
			Matcher: hookspkg.HookMatcher{
				ACPEventType: "session_update",
			},
		}), profileID, "ws-1", nil)
		if err != nil {
			t.Fatalf("appendProfileHookDeclarations() error = %v", err)
		}
		if len(decls) != 1 {
			t.Fatalf("len(decls) = %d, want 1", len(decls))
		}
		if got := decls[0].Matcher.WorkspaceID; got != "" {
			t.Fatalf("decls[0].Matcher.WorkspaceID = %q, want empty", got)
		}
		if got, want := decls[0].Matcher.ACPEventType, "session_update"; got != want {
			t.Fatalf("decls[0].Matcher.ACPEventType = %q, want %q", got, want)
		}
		if err := hookspkg.ValidateMatcherForEvent(decls[0].Event, decls[0].Matcher); err != nil {
			t.Fatalf("ValidateMatcherForEvent() error = %v", err)
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
