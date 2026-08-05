//go:build integration

package session

import (
	"testing"

	"github.com/compozy/compozy/internal/testutil"
)

func TestManagerIntegrationProviderPersistsAcrossCreateStatusListAndResume(t *testing.T) {
	t.Run("Should persist provider across create/status/list/resume", func(t *testing.T) {
		h := newHarness(t)

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "provider-persisted",
			Workspace: h.workspaceID,
			Provider:  "codex",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if got := session.Info().Provider; got != "codex" {
			t.Fatalf("Create().Provider = %q, want %q", got, "codex")
		}
		if meta := readMeta(t, session.MetaPath()); meta.Provider != "codex" {
			t.Fatalf("create meta.Provider = %q, want %q", meta.Provider, "codex")
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		status, err := h.manager.Status(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got := status.Provider; got != "codex" {
			t.Fatalf("Status().Provider = %q, want %q", got, "codex")
		}

		infos, err := h.manager.ListAll(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		info := findSessionInfoByID(t, infos, session.ID)
		if got := info.Provider; got != "codex" {
			t.Fatalf("ListAll().Provider = %q, want %q", got, "codex")
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Errorf("Stop(%q) cleanup error = %v", resumed.ID, err)
			}
		})

		if got := resumed.Info().Provider; got != "codex" {
			t.Fatalf("Resume().Provider = %q, want %q", got, "codex")
		}
		if got, want := len(h.driver.startCalls), 2; got != want {
			t.Fatalf("len(startCalls) = %d, want %d", got, want)
		}
		if got := h.driver.startCalls[1].Command; got != h.driver.startCalls[0].Command {
			t.Fatalf("resume start command = %q, want %q", got, h.driver.startCalls[0].Command)
		}
		if meta := readMeta(t, resumed.MetaPath()); meta.Provider != "codex" {
			t.Fatalf("resume meta.Provider = %q, want %q", meta.Provider, "codex")
		}
	})
}

func findSessionInfoByID(t *testing.T, infos []*Info, id string) *Info {
	t.Helper()

	for _, info := range infos {
		if info != nil && info.ID == id {
			return info
		}
	}

	t.Fatalf("session info %q not found in %#v", id, infos)
	return nil
}
