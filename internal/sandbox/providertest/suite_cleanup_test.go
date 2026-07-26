package providertest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/sandbox"
)

func TestRunLifecycleCleanupContract(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("provider failure")
	tests := []struct {
		name     string
		provider *suiteTestProvider
		wantText string
	}{
		{
			name: "Should destroy prepared provider after missing launcher validation",
			provider: &suiteTestProvider{
				backend:  sandbox.BackendLocal,
				launcher: nil,
				toolHost: suiteTestToolHost{},
			},
			wantText: "Prepared.Launcher",
		},
		{
			name: "Should destroy prepared provider after missing tool host validation",
			provider: &suiteTestProvider{
				backend:  sandbox.BackendLocal,
				launcher: suiteTestLauncher{},
				toolHost: nil,
			},
			wantText: "Prepared.ToolHost",
		},
		{
			name:     "Should destroy prepared provider after sync to runtime failure",
			provider: &suiteTestProvider{backend: sandbox.BackendLocal, syncToErr: wantErr},
			wantText: "Provider.SyncToRuntime()",
		},
		{
			name:     "Should destroy prepared provider after sync from runtime failure",
			provider: &suiteTestProvider{backend: sandbox.BackendLocal, syncFromErr: wantErr},
			wantText: "Provider.SyncFromRuntime()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := runLifecycle(context.Background(), LifecycleCase{
				Provider: tt.provider,
				Backend:  sandbox.BackendLocal,
				PrepareRequest: sandbox.PrepareRequest{
					SandboxID:    "env-suite",
					LocalRootDir: t.TempDir(),
				},
			})
			if err == nil {
				t.Fatal("runLifecycle() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("runLifecycle() error = %q, want text %q", err, tt.wantText)
			}
			if !tt.provider.destroyed {
				t.Fatal("runLifecycle() did not destroy provider after post-Prepare failure")
			}
		})
	}
}
