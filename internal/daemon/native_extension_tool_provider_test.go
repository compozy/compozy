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

	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestDaemonExtensionToolProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should anchor dev cycle import task patterns to workspace root", func(t *testing.T) {
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
			devCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      devCycleImportTasksToolID,
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

	t.Run("Should reject relative dev cycle import task patterns that escape the workspace root", func(t *testing.T) {
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
			devCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      devCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":"../outside/task_*.md"}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want error", result)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
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

	t.Run("Should reject dev cycle import task patterns that escape through a workspace symlink", func(t *testing.T) {
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
			devCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      devCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":".compozy/tasks/delivery/task_*.md"}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want symlink escape error", result)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called for symlink-escaping pattern")
		}
	})

	t.Run("Should reject empty dev cycle import task patterns before extension dispatch", func(t *testing.T) {
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
			devCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      devCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(`{"pattern":""}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want empty-pattern error", result)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Call() error = %T %[1]v, want *tools.ToolError", err)
		}
		if !containsReason(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("ToolError.ReasonCodes = %#v, want scope_mismatch", toolErr.ReasonCodes)
		}
		if inner.handle.called {
			t.Fatal("inner handle was called for empty pattern")
		}
	})

	t.Run("Should reject absolute dev cycle import task patterns", func(t *testing.T) {
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
			devCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID:      devCycleImportTasksToolID,
			WorkspaceID: "ws-1",
			Input:       json.RawMessage(fmt.Sprintf(`{"pattern":%q}`, filepath.Join(root, "task_*.md"))),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want error", result)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
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

	t.Run("Should reject dev cycle import task calls without workspace scope", func(t *testing.T) {
		t.Parallel()

		inner := &daemonExtensionProviderStub{handle: &daemonExtensionHandleStub{}}
		resolver := &daemonExtensionWorkspaceResolverStub{}
		provider := newDaemonScopedExtensionToolProvider(inner, resolver)
		handle, ok, err := provider.Resolve(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-1"},
			devCycleImportTasksToolID,
		)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if !ok {
			t.Fatal("Resolve() ok = false, want true")
		}

		result, err := handle.Call(t.Context(), toolspkg.CallRequest{
			ToolID: devCycleImportTasksToolID,
			Input:  json.RawMessage(`{"pattern":".compozy/tasks/task_*.md"}`),
		})
		if err == nil {
			t.Fatalf("Call() result = %#v, want error", result)
		}
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
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
	handle *daemonExtensionHandleStub
}

var _ toolspkg.Provider = (*daemonExtensionProviderStub)(nil)

func (p *daemonExtensionProviderStub) ID() toolspkg.SourceRef {
	return toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: "dev-cycle"}
}

func (p *daemonExtensionProviderStub) List(
	context.Context,
	toolspkg.Scope,
) ([]toolspkg.Descriptor, error) {
	return []toolspkg.Descriptor{p.handle.Descriptor()}, nil
}

func (p *daemonExtensionProviderStub) Resolve(
	context.Context,
	toolspkg.Scope,
	toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	return p.handle, true, nil
}

type daemonExtensionHandleStub struct {
	called  bool
	request toolspkg.CallRequest
}

var _ toolspkg.Handle = (*daemonExtensionHandleStub)(nil)

func (h *daemonExtensionHandleStub) Descriptor() toolspkg.Descriptor {
	return toolspkg.Descriptor{
		ID: devCycleImportTasksToolID,
		Source: toolspkg.SourceRef{
			Kind:  toolspkg.SourceExtension,
			Owner: "dev-cycle",
		},
		Backend: toolspkg.BackendRef{
			Kind:        toolspkg.BackendExtensionHost,
			ExtensionID: "dev-cycle",
			Handler:     "import_tasks",
		},
		DisplayTitle: "Import tasks",
		Description:  "Import task files",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"}}}`),
		ReadOnly:     true,
		Risk:         toolspkg.RiskRead,
		Visibility:   toolspkg.VisibilitySession,
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
