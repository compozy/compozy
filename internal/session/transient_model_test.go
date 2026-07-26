package session

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
)

func TestInvokeTransientModel(t *testing.T) {
	t.Parallel()

	t.Run("Should execute one bounded turn without publishing a durable session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		provider := "claude"
		result, err := h.manager.InvokeTransientModel(testutil.Context(t), TransientModelCall{
			Config:         &h.cfg,
			Provider:       provider,
			Model:          h.cfg.Providers[provider].Models.Default,
			CWD:            h.workspace,
			Prompt:         "Choose the memory operation.",
			MaxOutputBytes: 64,
		})
		if err != nil {
			t.Fatalf("InvokeTransientModel() error = %v", err)
		}
		if !result.Accepted || result.Output != "reply" {
			t.Fatalf("InvokeTransientModel() = %#v, want accepted reply", result)
		}
		if sessions := h.manager.List(); len(sessions) != 0 {
			t.Fatalf("List() = %#v, want no durable transient session", sessions)
		}
		if len(h.driver.startCalls) != 1 || len(h.driver.promptCalls) != 1 || h.driver.stopCalls != 1 {
			t.Fatalf(
				"driver start/prompt/stop calls = %d/%d/%d, want 1/1/1",
				len(h.driver.startCalls),
				len(h.driver.promptCalls),
				h.driver.stopCalls,
			)
		}
		if h.driver.startCalls[0].Permissions != aghconfig.PermissionModeDenyAll {
			t.Fatalf(
				"Start().Permissions = %q, want %q",
				h.driver.startCalls[0].Permissions,
				aghconfig.PermissionModeDenyAll,
			)
		}
	})
}
