package session

import (
	"context"
	"github.com/compozy/compozy/internal/store"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestResumeUsesPersistedEffectivePermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a create-time permission override across resume", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.cfg.Permissions.Mode = compozyconfig.PermissionModeDenyAll
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
			Config:  h.cfg,
			Agents: []compozyconfig.AgentDef{
				{
					Name:     compozyconfig.DefaultAgentName,
					Provider: "claude",
					Prompt:   "You are a coding assistant.",
				},
				{
					Name:     "coder",
					Provider: "claude",
					Prompt:   "You are a coding assistant.",
				},
			},
		})
		h.manager = newManagerWithHarness(t, h)

		created, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:   "coder",
			Workspace:   h.workspaceID,
			Permissions: compozyconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got := h.driver.startCalls[0].Permissions; got != compozyconfig.PermissionModeApproveAll {
			t.Fatalf("create start permissions = %q, want %q", got, compozyconfig.PermissionModeApproveAll)
		}
		createdPermissions := created.Info().EffectivePermissions
		if got, want := createdPermissions, string(compozyconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("created info effective permissions = %q, want %q", got, want)
		}
		createdMeta := readMeta(t, created.MetaPath())
		got := createdMeta.EffectivePermissionsValue()
		want := string(compozyconfig.PermissionModeApproveAll)
		if got != want {
			t.Fatalf("created metadata effective permissions = %q, want %q", got, want)
		}
		wantAuthMode := string(compozyconfig.ProviderAuthModeNativeCLI)
		if got, want := createdMeta.EffectiveProviderAuthModeValue(), wantAuthMode; got != want {
			t.Fatalf("created metadata provider auth mode = %q, want %q", got, want)
		}

		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped, err := h.manager.Status(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Status(stopped) error = %v", err)
		}
		if got, want := stopped.EffectivePermissions, string(compozyconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("stopped info effective permissions = %q, want %q", got, want)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		if got := h.driver.startCalls[1].Permissions; got != compozyconfig.PermissionModeApproveAll {
			t.Fatalf("resume start permissions = %q, want persisted %q", got, compozyconfig.PermissionModeApproveAll)
		}
		resumedPermissions := resumed.Info().EffectivePermissions
		if got, want := resumedPermissions, string(compozyconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("resumed info effective permissions = %q, want %q", got, want)
		}
		resumedMeta := readMeta(t, resumed.MetaPath())
		got = resumedMeta.EffectivePermissionsValue()
		if got != want {
			t.Fatalf("resumed metadata effective permissions = %q, want %q", got, want)
		}
		if got, want := resumedMeta.EffectiveProviderAuthModeValue(), wantAuthMode; got != want {
			t.Fatalf("resumed metadata provider auth mode = %q, want %q", got, want)
		}
	})
}

func TestInstancePermissionMode(t *testing.T) {
	for _, tc := range []struct{ mode, want compozyconfig.PermissionMode }{
		{compozyconfig.PermissionModeApproveAll, compozyconfig.PermissionModeApproveReads},
		{compozyconfig.PermissionModeApproveReads, compozyconfig.PermissionModeApproveReads},
		{compozyconfig.PermissionModeDenyAll, compozyconfig.PermissionModeDenyAll},
	} {
		t.Run("Should retain bounded "+string(tc.mode)+" permissions across resume and context replacement", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.manager = newManagerWithHarness(t, h)
			cleanupTestManager(t, h.manager)
			h.manager.SetInstanceCeilingReader(func(context.Context, string, string) (*store.SessionPermissionPolicy, string, error) {
				return &store.SessionPermissionPolicy{WorkspacePaths: []string{h.workspace}}, "", nil
			})
			created, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Permissions: tc.mode})
			if err != nil {
				t.Fatal(err)
			}
			if got := created.Info().EffectivePermissions; got != string(tc.want) {
				t.Fatalf("launch permissions=%s, want %s", got, tc.want)
			}
			if err := h.manager.Stop(t.Context(), created.ID); err != nil {
				t.Fatal(err)
			}
			resumed, err := h.manager.Resume(t.Context(), created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got := resumed.Info().EffectivePermissions; got != string(tc.want) {
				t.Fatalf("resume permissions=%s, want %s", got, tc.want)
			}
			cleared, err := h.manager.ClearConversation(t.Context(), resumed.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := waitForSessionStartRun(t.Context(), h.manager.sessionStartRun(cleared.ID)); err != nil {
				t.Fatal(err)
			}
			if got := cleared.Info().EffectivePermissions; got != string(tc.want) {
				t.Fatalf("replacement permissions=%s, want %s", got, tc.want)
			}
			events, err := h.manager.Prompt(t.Context(), cleared.ID, "Continue within the instance permissions")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, events)
			h.driver.mu.Lock()
			defer h.driver.mu.Unlock()
			if len(h.driver.startCalls) != 3 {
				t.Fatalf("provider launches=%d, want 3", len(h.driver.startCalls))
			}
			for _, launch := range h.driver.startCalls {
				if launch.Permissions != tc.want {
					t.Fatalf("provider permissions=%s, want %s", launch.Permissions, tc.want)
				}
			}
		})
	}
}
