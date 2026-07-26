package providertest

import (
	"testing"

	"github.com/compozy/agh/internal/sandbox"
)

func TestRunLifecycleAssertPreparedOrderContract(t *testing.T) {
	t.Parallel()

	t.Run("Should invoke AssertPrepared before sync and destroy", func(t *testing.T) {
		t.Parallel()

		provider := &suiteTestProvider{backend: sandbox.BackendLocal}
		RunLifecycle(t, LifecycleCase{
			Provider: provider,
			Backend:  sandbox.BackendLocal,
			PrepareRequest: sandbox.PrepareRequest{
				SandboxID:    "env-suite-order",
				LocalRootDir: t.TempDir(),
				Sandbox:      sandbox.Resolved{Backend: sandbox.BackendLocal},
			},
			AssertPrepared: func(t *testing.T, prepared sandbox.Prepared) {
				t.Helper()
				if provider.syncedToRuntime || provider.syncedFromRuntime || provider.destroyed {
					t.Fatalf(
						"AssertPrepared ran after lifecycle mutation: syncedToRuntime=%t syncedFromRuntime=%t destroyed=%t",
						provider.syncedToRuntime,
						provider.syncedFromRuntime,
						provider.destroyed,
					)
				}
				if prepared.State.SandboxID != "env-suite-order" {
					t.Fatalf("prepared sandbox id = %q, want env-suite-order", prepared.State.SandboxID)
				}
			},
		})
		if !provider.syncedToRuntime || !provider.syncedFromRuntime || !provider.destroyed {
			t.Fatalf(
				"RunLifecycle completed with syncedToRuntime=%t syncedFromRuntime=%t destroyed=%t, want all true",
				provider.syncedToRuntime,
				provider.syncedFromRuntime,
				provider.destroyed,
			)
		}
	})
}
