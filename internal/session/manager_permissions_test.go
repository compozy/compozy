package session

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestResumeUsesPersistedEffectivePermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a create-time permission override across resume", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.cfg.Permissions.Mode = aghconfig.PermissionModeDenyAll
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      h.workspaceID,
				RootDir: h.workspace,
				Name:    h.workspaceName,
			},
			Config: h.cfg,
			Agents: []aghconfig.AgentDef{
				{
					Name:     aghconfig.DefaultAgentName,
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
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got := h.driver.startCalls[0].Permissions; got != aghconfig.PermissionModeApproveAll {
			t.Fatalf("create start permissions = %q, want %q", got, aghconfig.PermissionModeApproveAll)
		}
		createdMeta := readMeta(t, created.MetaPath())
		if got, want := createdMeta.EffectivePermissions, string(aghconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("created metadata effective permissions = %q, want %q", got, want)
		}

		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
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

		if got := h.driver.startCalls[1].Permissions; got != aghconfig.PermissionModeApproveAll {
			t.Fatalf("resume start permissions = %q, want persisted %q", got, aghconfig.PermissionModeApproveAll)
		}
		resumedMeta := readMeta(t, resumed.MetaPath())
		if got, want := resumedMeta.EffectivePermissions, string(aghconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("resumed metadata effective permissions = %q, want %q", got, want)
		}
	})
}
