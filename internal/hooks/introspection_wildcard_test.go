package hooks

import "testing"

func TestHooksCatalogWildcardMatchersContract(t *testing.T) {
	t.Run("Should include wildcard hook matchers that dispatch would run", func(t *testing.T) {
		t.Parallel()

		hooks := newTestHooks(
			t,
			WithConfigDeclarations([]HookDecl{
				{
					Name:  "wildcard-session",
					Event: HookSessionPostCreate,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						AgentName:     "cod*",
						WorkspaceID:   "ws-*",
						WorkspaceRoot: "/workspace/*",
					},
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
				{
					Name:  "other-workspace-pattern",
					Event: HookSessionPostCreate,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						AgentName:     "cod*",
						WorkspaceID:   "other-*",
						WorkspaceRoot: "/workspace/*",
					},
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
				{
					Name:  "other-agent-pattern",
					Event: HookSessionPostCreate,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						AgentName:     "review*",
						WorkspaceID:   "ws-*",
						WorkspaceRoot: "/workspace/*",
					},
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
				{
					Name:  "other-root-pattern",
					Event: HookSessionPostCreate,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						AgentName:     "cod*",
						WorkspaceID:   "ws-*",
						WorkspaceRoot: "/tmp/*",
					},
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
			}),
		)

		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		entries, err := hooks.Catalog(CatalogFilter{
			AgentName:     "coder",
			WorkspaceID:   "ws-alpha",
			WorkspaceRoot: "/workspace/alpha",
		})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if got, want := len(entries), 1; got != want {
			t.Fatalf("len(entries) = %d, want %d", got, want)
		}
		if entries[0].Name != "wildcard-session" {
			t.Fatalf("entries[0].Name = %q, want wildcard-session", entries[0].Name)
		}
	})
}
