package providertest

import (
	"context"
	"errors"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/sandbox"
)

func TestRunLifecycleExercisesProviderContract(t *testing.T) {
	t.Parallel()

	provider := &suiteTestProvider{backend: sandbox.BackendLocal}
	prepared := RunLifecycle(t, LifecycleCase{
		Provider: provider,
		Backend:  sandbox.BackendLocal,
		PrepareRequest: sandbox.PrepareRequest{
			SandboxID:    "env-suite",
			LocalRootDir: t.TempDir(),
			Sandbox:      sandbox.Resolved{Backend: sandbox.BackendLocal},
		},
		AssertPrepared: func(t *testing.T, prepared sandbox.Prepared) {
			t.Helper()
			if prepared.State.SandboxID != "env-suite" {
				t.Fatalf("prepared sandbox id = %q, want env-suite", prepared.State.SandboxID)
			}
		},
		AssertFinalState: func(t *testing.T, state sandbox.SessionState) {
			t.Helper()
			if state.Backend != sandbox.BackendLocal {
				t.Fatalf("final state backend = %q, want %q", state.Backend, sandbox.BackendLocal)
			}
		},
	})

	if prepared.State.SandboxID != "env-suite" {
		t.Fatalf("RunLifecycle() state sandbox id = %q, want env-suite", prepared.State.SandboxID)
	}
	if !provider.syncedToRuntime {
		t.Fatal("RunLifecycle() did not call SyncToRuntime")
	}
	if !provider.syncedFromRuntime {
		t.Fatal("RunLifecycle() did not call SyncFromRuntime")
	}
	if !provider.destroyed {
		t.Fatal("RunLifecycle() did not call Destroy")
	}
}

func TestRunLifecycleDetectsInvalidProviderCases(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("provider failure")
	tests := []struct {
		name     string
		provider sandbox.Provider
		backend  sandbox.Backend
		wantText string
	}{
		{name: "nil provider", provider: nil, backend: sandbox.BackendLocal, wantText: "provider = nil"},
		{
			name:     "backend mismatch",
			provider: &suiteTestProvider{backend: sandbox.BackendDaytona},
			backend:  sandbox.BackendLocal,
			wantText: "Provider.Backend()",
		},
		{
			name:     "prepare error",
			provider: &suiteTestProvider{backend: sandbox.BackendLocal, prepareErr: wantErr},
			backend:  sandbox.BackendLocal,
			wantText: "Provider.Prepare()",
		},
		{
			name: "missing launcher",
			provider: &suiteTestProvider{
				backend:  sandbox.BackendLocal,
				launcher: nil,
				toolHost: suiteTestToolHost{},
			},
			backend:  sandbox.BackendLocal,
			wantText: "Prepared.Launcher",
		},
		{
			name: "missing tool host",
			provider: &suiteTestProvider{
				backend:  sandbox.BackendLocal,
				launcher: suiteTestLauncher{},
				toolHost: nil,
			},
			backend:  sandbox.BackendLocal,
			wantText: "Prepared.ToolHost",
		},
		{
			name:     "sync to runtime error",
			provider: &suiteTestProvider{backend: sandbox.BackendLocal, syncToErr: wantErr},
			backend:  sandbox.BackendLocal,
			wantText: "Provider.SyncToRuntime()",
		},
		{
			name:     "sync from runtime error",
			provider: &suiteTestProvider{backend: sandbox.BackendLocal, syncFromErr: wantErr},
			backend:  sandbox.BackendLocal,
			wantText: "Provider.SyncFromRuntime()",
		},
		{
			name:     "destroy error",
			provider: &suiteTestProvider{backend: sandbox.BackendLocal, destroyErr: wantErr},
			backend:  sandbox.BackendLocal,
			wantText: "Provider.Destroy()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := runLifecycle(context.Background(), LifecycleCase{
				Provider: tt.provider,
				Backend:  tt.backend,
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
		})
	}
}

type suiteTestProvider struct {
	backend           sandbox.Backend
	prepareErr        error
	launcher          sandbox.Launcher
	toolHost          sandbox.ToolHost
	syncToErr         error
	syncFromErr       error
	destroyErr        error
	syncedToRuntime   bool
	syncedFromRuntime bool
	destroyed         bool
}

func (p *suiteTestProvider) Backend() sandbox.Backend {
	return p.backend
}

func (p *suiteTestProvider) Prepare(
	_ context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	if p.prepareErr != nil {
		return sandbox.Prepared{}, p.prepareErr
	}
	launcher := p.launcher
	if launcher == nil && p.toolHost == nil {
		launcher = suiteTestLauncher{}
	}
	toolHost := p.toolHost
	if toolHost == nil && p.launcher == nil {
		toolHost = suiteTestToolHost{}
	}
	return sandbox.Prepared{
		State: sandbox.SessionState{
			SandboxID:      req.SandboxID,
			Backend:        sandbox.BackendLocal,
			RuntimeRootDir: req.LocalRootDir,
		},
		RuntimeRootDir: req.LocalRootDir,
		Launcher:       launcher,
		ToolHost:       toolHost,
	}, nil
}

func (p *suiteTestProvider) SyncToRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	if p.syncToErr != nil {
		return sandbox.SyncResult{}, p.syncToErr
	}
	p.syncedToRuntime = true
	return sandbox.SyncResult{}, nil
}

func (p *suiteTestProvider) SyncFromRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	if p.syncFromErr != nil {
		return sandbox.SyncResult{}, p.syncFromErr
	}
	p.syncedFromRuntime = true
	return sandbox.SyncResult{}, nil
}

func (p *suiteTestProvider) Destroy(context.Context, sandbox.SessionState) error {
	if p.destroyErr != nil {
		return p.destroyErr
	}
	p.destroyed = true
	return nil
}

type suiteTestLauncher struct{}

func (suiteTestLauncher) Launch(context.Context, sandbox.LaunchSpec) (sandbox.Handle, error) {
	return nil, nil
}

type suiteTestToolHost struct{}

func (suiteTestToolHost) ReadTextFile(context.Context, string) (string, error) {
	return "", nil
}

func (suiteTestToolHost) WriteTextFile(context.Context, string, string) error {
	return nil
}

func (suiteTestToolHost) ResolvePath(path string) (string, error) {
	return path, nil
}

func (suiteTestToolHost) Authorize(sandbox.PermissionOperation) error {
	return nil
}

func (suiteTestToolHost) PermissionDecision(
	acpsdk.RequestPermissionRequest,
) (sandbox.PermissionDecision, bool) {
	return sandbox.PermissionDecisionAllowOnce, false
}

func (suiteTestToolHost) CreateTerminal(
	context.Context,
	acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, nil
}

func (suiteTestToolHost) KillTerminal(string) error {
	return nil
}

func (suiteTestToolHost) TerminalOutput(string) (string, error) {
	return "", nil
}

func (suiteTestToolHost) WaitForTerminalExit(context.Context, string) (int, error) {
	return 0, nil
}

func (suiteTestToolHost) ReleaseTerminal(string) error {
	return nil
}
