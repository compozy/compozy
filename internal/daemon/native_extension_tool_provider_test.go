package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDaemonExtensionToolProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should project callable extension tools during the hosted session bootstrap", func(t *testing.T) {
		t.Parallel()

		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		provider := newDaemonScopedExtensionToolProvider(inner, &daemonExtensionWorkspaceResolverStub{})
		registry, err := toolspkg.NewRegistry(
			toolspkg.WithProviders(provider),
			toolspkg.WithPolicyInputs(toolspkg.PolicyInputs{
				SystemPermissionMode: toolspkg.PermissionModeApproveAll,
				ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			}, toolspkg.ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		views, err := registry.BootstrapSessionProjection(t.Context(), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("BootstrapSessionProjection() error = %v", err)
		}
		if got, want := len(views), 1; got != want {
			t.Fatalf("len(bootstrap tools) = %d, want %d: %#v", got, want, views)
		}
		if got, want := views[0].Descriptor.ID, specCycleImportTasksToolID; got != want {
			t.Fatalf("bootstrap tool = %q, want %q", got, want)
		}
	})

	t.Run("Should reject extension operations when workspace registration cannot be validated", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name        string
			resolver    daemonExtensionWorkspaceResolver
			cause       error
			message     string
			callMessage string
		}{
			{
				name:        "Should reject an unavailable resolver",
				cause:       workspacepkg.ErrWorkspaceResolverUnavailable,
				message:     `workspace "workspace-identity" cannot be resolved`,
				callMessage: "has no workspace resolver",
			},
			{
				name:        "Should reject an unknown workspace",
				resolver:    &daemonExtensionWorkspaceResolverStub{},
				cause:       workspacepkg.ErrWorkspaceNotFound,
				message:     `resolve extension tool workspace "workspace-identity"`,
				callMessage: `workspace "workspace-identity" is invalid`,
			},
			{
				name: "Should reject an empty registration id",
				resolver: &daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{RootDir: t.TempDir()},
					WorkspaceID: "workspace-identity",
				}},
				message:     "has no registered runtime id: daemon: resolved workspace registry id is empty",
				callMessage: "has no registered runtime id",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
				provider := newDaemonScopedExtensionToolProvider(inner, tc.resolver)
				scope := toolspkg.Scope{WorkspaceID: "workspace-identity"}
				if _, err := provider.List(
					t.Context(),
					scope,
				); err == nil ||
					!strings.Contains(err.Error(), tc.message) {
					t.Fatalf("List() error = %v, want registration validation error containing %q", err, tc.message)
				} else if tc.cause != nil &&
					!errors.Is(err, tc.cause) {
					t.Fatalf("List() error = %v, want wrapped cause %v", err, tc.cause)
				}
				if _, _, err := provider.Resolve(t.Context(), scope, specCycleImportTasksToolID); err == nil ||
					!strings.Contains(err.Error(), tc.message) {
					t.Fatalf("Resolve() error = %v, want registration validation error containing %q", err, tc.message)
				} else if tc.cause != nil && !errors.Is(err, tc.cause) {
					t.Fatalf("Resolve() error = %v, want wrapped cause %v", err, tc.cause)
				}
				source, ok := provider.(toolspkg.ProjectionGenerationProvider)
				if !ok {
					t.Fatal("provider does not expose projection generation")
				}
				if generation, known := source.ProjectionGeneration(t.Context(), scope); known || generation != "" {
					t.Fatalf("ProjectionGeneration() = %q, %v, want unknown", generation, known)
				}
				handle, ok, err := provider.Resolve(t.Context(), toolspkg.Scope{}, specCycleImportTasksToolID)
				if err != nil || !ok {
					t.Fatalf("Resolve(unscoped) = %v, %v, want handle", ok, err)
				}
				_, err = handle.Call(t.Context(), toolspkg.CallRequest{
					ToolID: specCycleImportTasksToolID, WorkspaceID: scope.WorkspaceID,
					Input: json.RawMessage(`{"pattern":"task_*.md"}`),
				})
				toolErr, matched := errors.AsType[*toolspkg.ToolError](err)
				if !matched || toolErr.Code != toolspkg.ErrorCodeInvalidInput ||
					toolErr.ToolID != specCycleImportTasksToolID ||
					!containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
					t.Fatalf("Call() error = %v, want invalid-input ToolError with scope mismatch", err)
				}
				if !strings.Contains(toolErr.Message, tc.callMessage) || !errors.Is(err, toolspkg.ErrToolInvalidInput) {
					t.Fatalf(
						"Call() error = %v, want registration validation error containing %q and ErrToolInvalidInput",
						err,
						tc.callMessage,
					)
				}
				if inner.handle.called {
					t.Fatal("inner handle called without valid registration")
				}
			})
		}
	})

	t.Run("Should canonicalize workspace scope and attach authority to arbitrary extension tools", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "workspace-registration", RootDir: root},
				WorkspaceID: "workspace-identity",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		scope := toolspkg.Scope{WorkspaceID: "workspace-identity", ProfileID: "profile-1", SessionID: "session-1"}
		if _, err := provider.List(t.Context(), scope); err != nil {
			t.Fatalf("List() error = %v", err)
		}
		canonical := scope
		canonical.WorkspaceID = "workspace-registration"
		if inner.listScope != canonical {
			t.Fatalf("List() scope = %#v, want %#v", inner.listScope, canonical)
		}
		generationSource, ok := provider.(toolspkg.ProjectionGenerationProvider)
		if !ok {
			t.Fatal("provider does not expose projection generation")
		}
		if generation, known := generationSource.ProjectionGeneration(
			t.Context(),
			scope,
		); !known ||
			generation != "generation-1" {
			t.Fatalf("ProjectionGeneration() = %q, %v, want generation-1, true", generation, known)
		}
		if inner.generationScope != canonical {
			t.Fatalf("ProjectionGeneration() scope = %#v, want %#v", inner.generationScope, canonical)
		}

		toolID := toolspkg.ToolID("ext__workspace_tool__search")
		handle, ok, err := provider.Resolve(
			t.Context(),
			scope,
			toolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}
		if inner.resolveScope != canonical {
			t.Fatalf("Resolve() scope = %#v, want %#v", inner.resolveScope, canonical)
		}
		if _, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      toolID,
			WorkspaceID: "workspace-identity",
			Input:       json.RawMessage(`{}`),
		}); err != nil {
			t.Fatalf("Call() error = %v", err)
		}
		if got, want := inner.handle.request.WorkspaceID, "workspace-registration"; got != want {
			t.Fatalf("Call() workspace = %q, want %q", got, want)
		}
		if got, want := inner.handle.request.TrustedWorkspaceRoot, root; got != want {
			t.Fatalf("Call() trusted root = %q, want %q", got, want)
		}
	})

	t.Run("Should attach resolved workspace authority to workspace-bound spec-cycle calls", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "workspace-registration", RootDir: root},
				WorkspaceID: "workspace-identity",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		for _, toolID := range []toolspkg.ToolID{
			specCycleWriteReviewArtifactsToolID,
			specCycleFinalizeReviewRoundToolID,
		} {
			handle, ok, err := provider.Resolve(
				t.Context(),
				toolspkg.Scope{WorkspaceID: "workspace-registration"},
				toolID,
			)
			if err != nil {
				t.Fatalf("Resolve(%s) error = %v", toolID, err)
			}
			if !ok {
				t.Fatalf("Resolve(%s) ok = false, want true", toolID)
			}
			_, err = handle.Call(t.Context(), toolspkg.CallRequest{
				ToolID:      toolID,
				WorkspaceID: "workspace-registration",
				Input:       json.RawMessage(`{}`),
			})
			if err != nil {
				t.Fatalf("Call(%s) error = %v", toolID, err)
			}
			if got, want := inner.handle.request.WorkspaceID, "workspace-registration"; got != want {
				t.Fatalf("%s request.WorkspaceID = %q, want %q", toolID, got, want)
			}
			if got, want := inner.handle.request.TrustedWorkspaceRoot, root; got != want {
				t.Fatalf("%s request.TrustedWorkspaceRoot = %q, want %q", toolID, got, want)
			}
		}
	})

	t.Run("Should require authenticated workspace scope for review artifact calls", func(t *testing.T) {
		t.Parallel()

		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		provider := newDaemonScopedExtensionToolProvider(inner, &daemonExtensionWorkspaceResolverStub{})
		handle, ok, err := provider.Resolve(t.Context(), toolspkg.Scope{}, specCycleFinalizeReviewRoundToolID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}
		_, err = handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID: specCycleFinalizeReviewRoundToolID,
			Input:  json.RawMessage(`{"task_name":"delivery","round":1}`),
		})
		if err == nil {
			t.Fatal("Call() error = nil, want workspace scope error")
		}
		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched || !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("Call() error = %v, want scope mismatch", err)
		}
		if inner.handle.called {
			t.Fatal("inner handle called without trusted workspace scope")
		}
	})

	t.Run("Should anchor spec-cycle import task patterns to workspace root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: root},
				WorkspaceID: "ws-1",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      specCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":".compozy/tasks/loops-refac/task_*.md"}`),
		})
		if err != nil {
			t.Fatalf("Call() error = %v", err)
		}
		if len(result.Structured) == 0 {
			t.Fatal("Call() returned empty structured result")
		}

		var input struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(inner.handle.request.Input, &input); err != nil {
			t.Fatalf("Unmarshal(patched input) error = %v", err)
		}
		want := filepath.Join(root, ".compozy", "tasks", "loops-refac", "task_*.md")
		if input.Pattern != want {
			t.Fatalf("patched pattern = %q, want %q", input.Pattern, want)
		}
	})

	t.Run("Should anchor spec-cycle import task patterns to the trusted worktree root", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		worktreeRoot := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
				WorkspaceID: "ws-1",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		_, err = handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:               specCycleImportTasksToolID,
			WorkspaceID:          "ws-1",
			TrustedWorkspaceRoot: worktreeRoot,
			Input:                json.RawMessage(`{"pattern":".compozy/tasks/example/task_*.md"}`),
		})
		if err != nil {
			t.Fatalf("Call() error = %v", err)
		}

		var input struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(inner.handle.request.Input, &input); err != nil {
			t.Fatalf("Unmarshal(patched input) error = %v", err)
		}
		want := filepath.Join(worktreeRoot, ".compozy", "tasks", "example", "task_*.md")
		if input.Pattern != want {
			t.Fatalf("patched pattern = %q, want trusted worktree pattern %q", input.Pattern, want)
		}
		if got := inner.handle.request.TrustedWorkspaceRoot; got != worktreeRoot {
			t.Fatalf("TrustedWorkspaceRoot = %q, want %q", got, worktreeRoot)
		}
	})

	t.Run("Should reject relative spec-cycle import task patterns that escape the workspace root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: root},
				WorkspaceID: "ws-1",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      specCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":"../outside/task_*.md"}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want error", result)
		}

		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("ToolError.Code = %q, want %q", toolErr.Code, toolspkg.ErrorCodeInvalidInput)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called for escaping pattern")
		}
	})

	t.Run("Should reject spec-cycle import task patterns that escape through a workspace symlink", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		outsideTasks := filepath.Join(t.TempDir(), "escaped-tasks", "delivery")
		if err := os.MkdirAll(outsideTasks, 0o750); err != nil {
			t.Fatalf("MkdirAll(outside tasks) error = %v", err)
		}
		compozyDir := filepath.Join(root, ".compozy")
		if err := os.MkdirAll(compozyDir, 0o750); err != nil {
			t.Fatalf("MkdirAll(.compozy) error = %v", err)
		}
		if err := os.Symlink(filepath.Dir(outsideTasks), filepath.Join(compozyDir, "tasks")); err != nil {
			t.Skipf("create workspace tasks symlink: %v", err)
		}
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: root},
				WorkspaceID: "ws-1",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      specCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":".compozy/tasks/delivery/task_*.md"}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want symlink escape error", result)
		}

		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called for symlink-escaping pattern")
		}
	})

	t.Run("Should reject empty spec-cycle import task patterns before extension dispatch", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: root},
				WorkspaceID: "ws-1",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      specCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":""}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want empty-pattern error", result)
		}

		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called for empty pattern")
		}
	})

	t.Run("Should reject absolute spec-cycle import task patterns", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{
			resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: root},
				WorkspaceID: "ws-1",
			},
		}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      specCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(fmt.Sprintf(`{"pattern":%q}`, filepath.Join(root, "task_*.md"))),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want error", result)
		}

		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("ToolError.Code = %q, want %q", toolErr.Code, toolspkg.ErrorCodeInvalidInput)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called for absolute pattern")
		}
	})

	t.Run("Should reject spec-cycle import task calls without workspace scope", func(t *testing.T) {
		t.Parallel()

		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{},
			specCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID: specCycleImportTasksToolID,
			Input:  json.RawMessage(`{"pattern":".compozy/tasks/task_*.md"}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want error", result)
		}

		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("ToolError.Code = %q, want %q", toolErr.Code, toolspkg.ErrorCodeInvalidInput)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called without workspace scope")
		}
	})
}

type daemonExtensionProviderStub struct {
	handle          *daemonExtensionHandleStub
	listScope       toolspkg.Scope
	resolveScope    toolspkg.Scope
	generationScope toolspkg.Scope
}

var _ toolspkg.Provider = (*daemonExtensionProviderStub)(nil)
var _ toolspkg.ProjectionGenerationProvider = (*daemonExtensionProviderStub)(nil)

func (p *daemonExtensionProviderStub) ID() toolspkg.SourceRef {
	return toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: "spec-cycle"}
}

func (p *daemonExtensionProviderStub) ProjectionGeneration(_ context.Context, scope toolspkg.Scope) (string, bool) {
	p.generationScope = scope
	return "generation-1", true
}

func (p *daemonExtensionProviderStub) List(
	_ context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.Descriptor, error) {
	p.listScope = scope
	return []toolspkg.Descriptor{p.handle.Descriptor()}, nil
}

func (p *daemonExtensionProviderStub) Resolve(
	_ context.Context,
	scope toolspkg.Scope,
	_ toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	p.resolveScope = scope
	return p.handle, true, nil
}

type daemonExtensionHandleStub struct {
	called  bool
	request toolspkg.CallRequest
}

var _ toolspkg.Handle = (*daemonExtensionHandleStub)(nil)

func (h *daemonExtensionHandleStub) Descriptor() toolspkg.Descriptor {
	return toolspkg.Descriptor{
		ID: specCycleImportTasksToolID,
		Source: toolspkg.SourceRef{
			Kind:  toolspkg.SourceExtension,
			Owner: "spec-cycle",
		},
		Backend: toolspkg.BackendRef{
			Kind:        toolspkg.BackendExtensionHost,
			ExtensionID: "spec-cycle",
			Handler:     "import_tasks",
		},
		ToolPresentation: toolspkg.NewToolPresentation("Import tasks", "", ""),
		Description:      "Import task files",
		InputSchema:      json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"}}}`),
		ReadOnly:         true,
		Risk:             toolspkg.RiskRead,
		Visibility:       toolspkg.VisibilitySession,
	}
}

func (h *daemonExtensionHandleStub) Availability(context.Context, toolspkg.Scope) toolspkg.Availability {
	return toolspkg.Availability{
		Registered: true,
		Enabled:    true,
		Available:  true,
		Authorized: true,
		Executable: true,
	}
}

func (h *daemonExtensionHandleStub) Call(
	_ context.Context,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	h.called = true
	h.request = req
	return toolspkg.ToolResult{Structured: json.RawMessage(`{"ok":true}`)}, nil
}

type daemonExtensionWorkspaceResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
}

var _ workspacepkg.RuntimeResolver = (*daemonExtensionWorkspaceResolverStub)(nil)
var _ daemonExtensionWorkspaceResolver = (*daemonExtensionWorkspaceResolverStub)(nil)

func (r *daemonExtensionWorkspaceResolverStub) ResolveRegistration(
	ctx context.Context,
	ref string,
) (workspacepkg.Workspace, error) {
	resolved, err := r.Resolve(ctx, ref)
	return resolved.Workspace, err
}

func (r *daemonExtensionWorkspaceResolverStub) Resolve(
	ctx context.Context,
	ref string,
) (workspacepkg.ResolvedWorkspace, error) {
	if err := ctx.Err(); err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	if strings.TrimSpace(ref) != r.resolved.ID && strings.TrimSpace(ref) != r.resolved.WorkspaceID {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return r.resolved, nil
}

func (r *daemonExtensionWorkspaceResolverStub) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if err := ctx.Err(); err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	if strings.TrimSpace(path) != r.resolved.RootDir {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return r.resolved, nil
}

func containsReason(values []toolspkg.ReasonCode, want toolspkg.ReasonCode) bool {
	return slices.Contains(values, want)
}
