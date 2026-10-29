package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

type nativeWorktreeServiceStub struct {
	createAccepted func(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
	create         func(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
	list           func(context.Context, string, bool) (*worktree.DetailedListing, error)
	resolve        func(context.Context, string, string) (*worktree.Worktree, error)
	inspect        func(context.Context, string, string) (*worktree.Inspection, error)
	remove         func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error)
	removeFound    func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error)
	deleteBranch   func(context.Context, string, string, bool) (worktree.BranchOutcome, error)
	capabilityErr  error
}

func (s *nativeWorktreeServiceStub) Create(
	ctx context.Context,
	workspaceID string,
	opts worktree.CreateOptions,
) (*worktree.Worktree, error) {
	if s.create == nil {
		return nil, fmt.Errorf("unexpected Create call")
	}
	return s.create(ctx, workspaceID, opts)
}

func (s *nativeWorktreeServiceStub) Capability(
	context.Context,
) (worktree.CapabilityDiagnostic, error) {
	return worktree.CapabilityDiagnostic{Available: s.capabilityErr == nil}, s.capabilityErr
}

func (s *nativeWorktreeServiceStub) CreateAccepted(
	ctx context.Context,
	workspaceID string,
	opts worktree.CreateOptions,
) (*worktree.Worktree, error) {
	if s.createAccepted != nil {
		return s.createAccepted(ctx, workspaceID, opts)
	}
	return nil, fmt.Errorf("unexpected CreateAccepted call")
}

func (s *nativeWorktreeServiceStub) CreateReady(
	ctx context.Context,
	workspaceID string,
	opts worktree.CreateOptions,
) (*worktree.Worktree, error) {
	if s.create == nil {
		return nil, fmt.Errorf("unexpected CreateReady call")
	}
	return s.create(ctx, workspaceID, opts)
}

func (s *nativeWorktreeServiceStub) CancelCreate(context.Context, string, string) error {
	return fmt.Errorf("unexpected CancelCreate call")
}

func (s *nativeWorktreeServiceStub) Adopt(context.Context, string, string, string) (*worktree.Worktree, error) {
	return nil, fmt.Errorf("unexpected Adopt call")
}

func (s *nativeWorktreeServiceStub) ListDetails(
	ctx context.Context,
	workspaceID string,
	refresh bool,
) (*worktree.DetailedListing, error) {
	if s.list == nil {
		return nil, fmt.Errorf("unexpected ListDetails call")
	}
	return s.list(ctx, workspaceID, refresh)
}

func (s *nativeWorktreeServiceStub) Inspect(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*worktree.Inspection, error) {
	if s.inspect == nil {
		return nil, fmt.Errorf("unexpected Inspect call")
	}
	return s.inspect(ctx, workspaceID, ref)
}

func (s *nativeWorktreeServiceStub) Resolve(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*worktree.Worktree, error) {
	if s.resolve == nil {
		return nil, fmt.Errorf("unexpected Resolve call")
	}
	return s.resolve(ctx, workspaceID, ref)
}

func (s *nativeWorktreeServiceStub) StatusDetails(
	context.Context,
	string,
	string,
	bool,
	bool,
) (*worktree.StatusDetails, error) {
	return nil, fmt.Errorf("unexpected StatusDetails call")
}

func (s *nativeWorktreeServiceStub) ExitPlan(
	context.Context,
	string,
	string,
) (*worktree.ExitPlan, error) {
	return nil, fmt.Errorf("unexpected ExitPlan call")
}

func (s *nativeWorktreeServiceStub) RunExitAction(
	context.Context,
	string,
	string,
	worktree.ExitActionRequest,
) (string, error) {
	return "", fmt.Errorf("unexpected RunExitAction call")
}

func (s *nativeWorktreeServiceStub) CancelExitAction(context.Context, string, string, string) error {
	return fmt.Errorf("unexpected CancelExitAction call")
}

func (s *nativeWorktreeServiceStub) RemoveDiscovered(
	ctx context.Context,
	workspaceID string,
	path string,
	options worktree.RemoveDiscoveredOptions,
) (*worktree.RemovalRefusal, error) {
	if s.removeFound == nil {
		return nil, fmt.Errorf("unexpected RemoveDiscovered call")
	}
	return s.removeFound(ctx, workspaceID, path, options.Force)
}

func (s *nativeWorktreeServiceStub) Remove(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	force bool,
) (*worktree.RemovalRefusal, error) {
	if s.remove == nil {
		return nil, fmt.Errorf("unexpected Remove call")
	}
	return s.remove(ctx, workspaceID, worktreeID, force)
}

func (s *nativeWorktreeServiceStub) Dismiss(context.Context, string, string) error {
	return fmt.Errorf("unexpected Dismiss call")
}

func (s *nativeWorktreeServiceStub) DeleteLocalBranch(
	ctx context.Context,
	workspaceID, worktreeID string,
	force bool,
) (worktree.BranchOutcome, error) {
	if s.deleteBranch == nil {
		return worktree.BranchOutcome{}, fmt.Errorf("unexpected DeleteLocalBranch call")
	}
	return s.deleteBranch(ctx, workspaceID, worktreeID, force)
}

func TestNativeWorktreeTools(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		id     toolspkg.ToolID
		input  string
		cause  error
		code   string
		status int
		detail string
	}{
		{"Should identify the existing worktree name", toolspkg.ToolIDWorktreeCreate,
			`{"workspace":"ws-a","name":"existing"}`, fmt.Errorf("%w: existing", worktree.ErrNameTaken),
			"worktree_name_taken", http.StatusConflict, "existing"},
		{"Should explain the bound session refusal", toolspkg.ToolIDWorktreeRemove,
			`{"workspace":"ws-a","ref":"wt-existing"}`, fmt.Errorf("%w: sess-bound", worktree.ErrSessionActive),
			"worktree_session_active", http.StatusConflict, "sess-bound"},
		{"Should retain a sanitized prune failure", toolspkg.ToolIDWorktreeCreate,
			`{"workspace":"ws-a","name":"new"}`, errors.New("workspace: prune missing workspace /private/operator/repo: permission denied"),
			"tool_backend_failed", http.StatusBadGateway, "permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := &nativeWorktreeServiceStub{
				createAccepted: func(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error) {
					return nil, tc.cause
				},
				resolve: func(context.Context, string, string) (*worktree.Worktree, error) {
					return &worktree.Worktree{ID: "wt-existing", WorkspaceID: "registry-a"}, nil
				},
				remove: func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error) {
					return nil, tc.cause
				},
			}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Workspaces: nativeWorktreeWorkspaceService(), Worktrees: service,
			}, nativeApproveAllPolicyInputs())
			_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
				ToolID: tc.id, Input: json.RawMessage(tc.input),
			})
			status := core.StatusForToolError(err)
			payload := core.ToolErrorResponseForError(err, status, false).Error
			if string(payload.Code) != tc.code || status != tc.status ||
				!strings.Contains(payload.Message, tc.detail) ||
				strings.Contains(payload.Message, "/private") {
				t.Fatalf("native response = %#v status=%d, want %s status=%d", payload, status, tc.code, tc.status)
			}
			if !errors.Is(err, tc.cause) {
				t.Fatalf("cause lost: %v", err)
			}
		})
	}

	t.Run("Should reach guarded removal when the parent root is missing", func(t *testing.T) {
		t.Parallel()
		calls := 0
		service := &nativeWorktreeServiceStub{
			resolve: func(_ context.Context, ws, _ string) (*worktree.Worktree, error) {
				return &worktree.Worktree{
					ID:          "wt-missing",
					WorkspaceID: ws,
					ProfileID:   store.DefaultProfileID,
					State:       worktree.StateMissing,
				}, nil
			},
			remove: func(_ context.Context, ws, id string, _ bool) (*worktree.RemovalRefusal, error) {
				if ws != "registry-a" || id != "wt-missing" {
					t.Fatalf("wrong removal target %s/%s", ws, id)
				}
				calls++
				return nil, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: apitest.StubWorkspaceService{
				ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
				},
				GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
					return workspacepkg.Workspace{ID: "registry-a"}, nil
				},
			}, Worktrees: service,
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWorktreeRemove,
			Input:  json.RawMessage(`{"workspace":"registry-a","ref":"wt-missing"}`),
		})
		if err != nil || calls != 1 {
			t.Fatalf("removal calls = %d, error = %v", calls, err)
		}
	})

	t.Run(
		"Should resolve workspace identity to the registry boundary without leaking another workspace",
		func(t *testing.T) {
			t.Parallel()

			workspaces := nativeWorktreeWorkspaceService()
			sessions := apitest.StubSessionManager{StatusFn: func(
				_ context.Context,
				id string,
			) (*session.Info, error) {
				return &session.Info{
					ID: id, ProfileID: store.DefaultProfileID,
					WorkspaceID: "ws-a", AgentName: "coder", State: session.StateActive,
				}, nil
			}}
			calls := 0
			service := &nativeWorktreeServiceStub{list: func(
				_ context.Context,
				workspaceID string,
				refresh bool,
			) (*worktree.DetailedListing, error) {
				calls++
				if refresh {
					t.Fatal("ListDetails() refresh = true, want cached read")
				}
				name := "alpha"
				if workspaceID == "registry-b" {
					name = "beta"
				} else if workspaceID != "registry-a" {
					t.Fatalf("ListDetails() workspace = %q, want registry id", workspaceID)
				}
				return &worktree.DetailedListing{Worktrees: []worktree.Inspection{{
					Worktree: worktree.Worktree{
						ID: "wt-" + name, WorkspaceID: workspaceID, Name: name,
						State: worktree.StateReady, Origin: worktree.OriginManual,
					},
					AgentActivity: worktree.AgentActivityIdle,
				}}}, nil
			}}
			registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
				Sessions: sessions, Workspaces: workspaces, Worktrees: service,
			}, nativeApproveAllPolicyInputs())
			scope := toolspkg.Scope{SessionID: "sess-a", WorkspaceID: "ws-a", AgentName: "coder"}
			result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDWorktreeList, Input: json.RawMessage(`{}`),
			})
			if err != nil {
				t.Fatalf("worktree_list error = %v", err)
			}
			var payload contract.WorktreesResponse
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatalf("json.Unmarshal(worktree_list) error = %v", err)
			}
			if len(payload.Worktrees) != 1 || payload.Worktrees[0].ID != "wt-alpha" ||
				payload.Worktrees[0].WorkspaceID != "registry-a" {
				t.Fatalf("workspace A worktrees = %#v, want only registry-a", payload.Worktrees)
			}

			_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDWorktreeList,
				Input:  json.RawMessage(`{"workspace":"ws-b"}`),
			})
			requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonWorkspaceAccessDenied)
			if calls != 1 {
				t.Fatalf("ListDetails() calls = %d, want denied request stopped before service", calls)
			}

			result, err = registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDWorktreeList,
				Input:  json.RawMessage(`{"workspace":"ws-b"}`),
			})
			if err != nil {
				t.Fatalf("operator worktree_list error = %v", err)
			}
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatalf("json.Unmarshal(operator worktree_list) error = %v", err)
			}
			if len(payload.Worktrees) != 1 || payload.Worktrees[0].ID != "wt-beta" ||
				payload.Worktrees[0].WorkspaceID != "registry-b" {
				t.Fatalf("workspace B worktrees = %#v, want only registry-b", payload.Worktrees)
			}
		},
	)

	t.Run("Should carry the exact removal refusal as a partial tool result", func(t *testing.T) {
		t.Parallel()

		service := &nativeWorktreeServiceStub{resolve: func(
			_ context.Context,
			workspaceID, ref string,
		) (*worktree.Worktree, error) {
			if workspaceID != "registry-a" || ref != "feature-a" {
				t.Fatalf("Resolve() args = %q, %q", workspaceID, ref)
			}
			return &worktree.Worktree{
				ID: "wt-a", WorkspaceID: workspaceID, Name: ref, State: worktree.StateReady,
			}, nil
		}, remove: func(
			_ context.Context,
			workspaceID string,
			worktreeID string,
			force bool,
		) (*worktree.RemovalRefusal, error) {
			if workspaceID != "registry-a" || worktreeID != "wt-a" || force {
				t.Fatalf("Remove() args = %q, %q, %t", workspaceID, worktreeID, force)
			}
			return &worktree.RemovalRefusal{
				Code: worktree.ErrDirtyRequiresForce.Error(),
				Risk: worktree.RemovalRisk{
					ChangedFiles: 2, Insertions: 4, Deletions: 1, UnpushedCommits: 3,
				},
			}, worktree.ErrDirtyRequiresForce
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: nativeWorktreeWorkspaceService(), Worktrees: service,
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWorktreeRemove,
			Input:  json.RawMessage(`{"workspace":"ws-a","ref":"feature-a"}`),
		})
		toolErr, ok := errors.AsType[*toolspkg.ToolError](err)
		if !ok || toolErr.PartialResult == nil {
			t.Fatalf("worktree_remove error = %T %v, want partial ToolError", err, err)
		}
		var refusal contract.WorktreeRemovalRefusalPayload
		if err := json.Unmarshal(toolErr.PartialResult.Structured, &refusal); err != nil {
			t.Fatalf("json.Unmarshal(removal refusal) error = %v", err)
		}
		if refusal.Code != worktree.ErrDirtyRequiresForce.Error() ||
			refusal.Risk.ChangedFiles != 2 || refusal.Risk.UnpushedCommits != 3 {
			t.Fatalf("removal refusal = %#v", refusal)
		}
	})

	t.Run("Should remove a discovered path without resolving a record", func(t *testing.T) {
		t.Parallel()
		calls := 0
		service := &nativeWorktreeServiceStub{removeFound: func(
			_ context.Context,
			workspaceID, path string,
			force bool,
		) (*worktree.RemovalRefusal, error) {
			calls++
			if workspaceID != "registry-a" || path != "/private/tmp/detached-a" {
				t.Fatalf("RemoveDiscovered() args = %q, %q", workspaceID, path)
			}
			if !force {
				return &worktree.RemovalRefusal{
					Code: worktree.ErrUnpushedRequiresForce.Error(),
					Risk: worktree.RemovalRisk{UnpushedCommits: 1},
				}, worktree.ErrUnpushedRequiresForce
			}
			return nil, nil
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: nativeWorktreeWorkspaceService(), Worktrees: service,
		}, nativeApproveAllPolicyInputs())
		call := func(input string) (toolspkg.ToolResult, error) {
			return registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDWorktreeRemove, Input: json.RawMessage(input),
			})
		}
		_, err := call(`{"workspace":"ws-a","path":"/private/tmp/detached-a"}`)
		toolErr, ok := errors.AsType[*toolspkg.ToolError](err)
		var refusal contract.WorktreeRemovalRefusalPayload
		if !ok || toolErr.PartialResult == nil ||
			json.Unmarshal(toolErr.PartialResult.Structured, &refusal) != nil ||
			refusal.Code != worktree.ErrUnpushedRequiresForce.Error() || refusal.Risk.UnpushedCommits != 1 {
			t.Fatalf("worktree_remove(path) error = %T %v, want unpushed partial refusal", err, err)
		}
		result, err := call(`{"workspace":"ws-a","path":"/private/tmp/detached-a","force":true}`)
		var payload struct {
			Path  string `json:"path"`
			State string `json:"state"`
		}
		if err != nil || json.Unmarshal(result.Structured, &payload) != nil ||
			payload.Path != "/private/tmp/detached-a" || payload.State != string(worktree.StateRemoved) {
			t.Fatalf("worktree_remove(path, force) = %s, %v, want removed path", result.Structured, err)
		}
		for _, input := range []string{
			`{"workspace":"ws-a","path":"/private/tmp/detached-a","ref":"feature-a"}`,
			`{"workspace":"ws-a","path":"/private/tmp/detached-a","delete_branch":true}`,
		} {
			_, err := call(input)
			if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); !ok ||
				toolErr.Code != toolspkg.ErrorCodeInvalidInput {
				t.Fatalf("worktree_remove(%s) error = %v, want invalid input", input, err)
			}
		}
		if calls != 2 {
			t.Fatalf("RemoveDiscovered() calls = %d, want 2", calls)
		}
	})

	t.Run("Should report the canonical identity after removal by name", func(t *testing.T) {
		t.Parallel()
		service := &nativeWorktreeServiceStub{resolve: func(
			_ context.Context,
			workspaceID, ref string,
		) (*worktree.Worktree, error) {
			return &worktree.Worktree{
				ID: "wt-a", WorkspaceID: workspaceID, Name: ref, State: worktree.StateReady,
			}, nil
		}, remove: func(
			_ context.Context,
			workspaceID, worktreeID string,
			force bool,
		) (*worktree.RemovalRefusal, error) {
			if workspaceID != "registry-a" || worktreeID != "wt-a" || !force {
				t.Fatalf("Remove() args = %q, %q, %t", workspaceID, worktreeID, force)
			}
			return nil, nil
		}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: nativeWorktreeWorkspaceService(), Worktrees: service,
		}, nativeApproveAllPolicyInputs())
		result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWorktreeRemove,
			Input:  json.RawMessage(`{"workspace":"ws-a","ref":"feature-a","force":true}`),
		})
		if err != nil {
			t.Fatalf("worktree_remove error = %v", err)
		}
		var payload struct {
			WorktreeID string `json:"worktree_id"`
			State      string `json:"state"`
		}
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(worktree_remove) error = %v", err)
		}
		if payload.WorktreeID != "wt-a" || payload.State != string(worktree.StateRemoved) {
			t.Fatalf("worktree_remove payload = %#v", payload)
		}
	})

	t.Run("Should delete the local branch only when the removal opted in", func(t *testing.T) {
		t.Parallel()
		deletions := 0
		service := &nativeWorktreeServiceStub{
			resolve: func(_ context.Context, workspaceID, ref string) (*worktree.Worktree, error) {
				return &worktree.Worktree{
					ID: "wt-a", WorkspaceID: workspaceID, Name: ref, State: worktree.StateReady,
				}, nil
			},
			remove: func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error) { return nil, nil },
			deleteBranch: func(
				_ context.Context,
				workspaceID, worktreeID string,
				force bool,
			) (worktree.BranchOutcome, error) {
				deletions++
				if workspaceID != "registry-a" || worktreeID != "wt-a" || force {
					t.Fatalf("DeleteLocalBranch() args = %q, %q, %t", workspaceID, worktreeID, force)
				}
				return worktree.BranchOutcome{Name: "fix/a", Reason: worktree.BranchKeptDefaultBranch}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: nativeWorktreeWorkspaceService(), Worktrees: service,
		}, nativeApproveAllPolicyInputs())
		for _, input := range []string{
			`{"workspace":"ws-a","ref":"feature-a"}`,
			`{"workspace":"ws-a","ref":"feature-a","delete_branch":true}`,
		} {
			result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDWorktreeRemove, Input: json.RawMessage(input),
			})
			if err != nil {
				t.Fatalf("worktree_remove(%s) error = %v", input, err)
			}
			var payload struct {
				Branch *struct {
					Name    string `json:"name"`
					Deleted bool   `json:"deleted"`
					Reason  string `json:"reason"`
				} `json:"branch"`
			}
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatalf("json.Unmarshal(worktree_remove) error = %v", err)
			}
			optedIn := strings.Contains(input, "delete_branch")
			if optedIn != (payload.Branch != nil) {
				t.Fatalf("worktree_remove(%s) branch = %#v, want present only on opt-in", input, payload.Branch)
			}
			if optedIn && (payload.Branch.Deleted || payload.Branch.Reason != worktree.BranchKeptDefaultBranch) {
				t.Fatalf("worktree_remove branch = %#v, want kept default branch", payload.Branch)
			}
		}
		if deletions != 1 {
			t.Fatalf("DeleteLocalBranch() calls = %d, want one", deletions)
		}
	})

	t.Run("Should map resolve errors before attempting removal", func(t *testing.T) {
		t.Parallel()
		resolveErr := errors.New("resolve unavailable")
		removeCalls := 0
		service := &nativeWorktreeServiceStub{
			resolve: func(context.Context, string, string) (*worktree.Worktree, error) {
				return nil, resolveErr
			},
			remove: func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error) {
				removeCalls++
				return nil, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Workspaces: nativeWorktreeWorkspaceService(), Worktrees: service,
		}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDWorktreeRemove,
			Input:  json.RawMessage(`{"workspace":"ws-a","ref":"feature-a"}`),
		})
		_, ok := errors.AsType[*toolspkg.ToolError](err)
		if !ok || !errors.Is(err, resolveErr) {
			t.Fatalf("worktree_remove error = %T %v, want wrapped ToolError", err, err)
		}
		if removeCalls != 0 {
			t.Fatalf("Remove() calls = %d, want none", removeCalls)
		}
	})

	t.Run("Should expose deterministic Git capability diagnostics", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name   string
			err    error
			reason toolspkg.ReasonCode
		}{
			{name: "unavailable", err: worktree.ErrGitUnavailable, reason: toolspkg.ReasonWorktreeGitUnavailable},
			{name: "unsupported", err: worktree.ErrGitVersionUnsupported, reason: toolspkg.ReasonWorktreeGitVersionUnsupported},
		} {
			t.Run("Should report "+testCase.name, func(t *testing.T) {
				t.Parallel()
				native := &daemonNativeTools{deps: &daemonNativeToolsDeps{
					Workspaces: nativeWorktreeWorkspaceService(),
					Worktrees:  &nativeWorktreeServiceStub{capabilityErr: testCase.err},
				}}
				availability := native.worktreeAvailability()(t.Context(), toolspkg.Scope{})
				if availability.Available || len(availability.ReasonCodes) != 1 ||
					availability.ReasonCodes[0] != testCase.reason {
					t.Fatalf("worktree availability = %#v, want %q", availability, testCase.reason)
				}
			})
		}
	})

	t.Run("Should roll back a newly materialized session target after acceptance fails", func(t *testing.T) {
		t.Parallel()

		removed := 0
		service := &nativeWorktreeServiceStub{
			create: func(
				_ context.Context,
				workspaceID string,
				opts worktree.CreateOptions,
			) (*worktree.Worktree, error) {
				if workspaceID != "registry-a" || opts.Name != "native-fork" {
					t.Fatalf("CreateReady() workspace/options = %q/%#v", workspaceID, opts)
				}
				return &worktree.Worktree{ID: "wt-native", WorkspaceID: workspaceID, State: worktree.StateReady}, nil
			},
			remove: func(
				_ context.Context,
				workspaceID string,
				worktreeID string,
				force bool,
			) (*worktree.RemovalRefusal, error) {
				removed++
				if workspaceID != "registry-a" || worktreeID != "wt-native" || !force {
					t.Fatalf("Remove() = workspace %q worktree %q force %v", workspaceID, worktreeID, force)
				}
				return nil, nil
			},
		}
		native := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			Workspaces: nativeWorktreeWorkspaceService(),
			Worktrees:  service,
			Sessions: apitest.StubSessionManager{CreateAcceptedFn: func(
				context.Context,
				session.CreateAcceptedOpts,
			) (*session.Info, error) {
				return nil, errors.New("accept failed")
			}},
		}}
		_, err := native.sessionCreate(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDSessionCreate,
			Input:  json.RawMessage(`{"workspace":"ws-a","agent":"coder","new_worktree":{"name":"native-fork"}}`),
		})
		if err == nil || !strings.Contains(err.Error(), "accept failed") || removed != 1 {
			t.Fatalf("sessionCreate() error/removals = %v/%d, want acceptance failure and one removal", err, removed)
		}
	})
}

func nativeWorktreeWorkspaceService() core.WorkspaceService {
	return apitest.StubWorkspaceService{ResolveFn: func(
		_ context.Context,
		ref string,
	) (workspacepkg.ResolvedWorkspace, error) {
		switch ref {
		case "ws-a", "registry-a":
			return workspacepkg.ResolvedWorkspace{
				ID: "registry-a", Name: "alpha",
				WorkspaceID: "ws-a",
			}, nil
		case "ws-b", "registry-b":
			return workspacepkg.ResolvedWorkspace{
				ID: "registry-b", Name: "beta",
				WorkspaceID: "ws-b",
			}, nil
		default:
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
		}
	}}
}

// Invariant: native binding preserves scope, expected binding, and typed refusals.
// Owner: native tool adapter; canonical worktree tools suite.
type nativeSessionBindingStub struct {
	apitest.StubSessionManager
	change func(context.Context, string, string, *string) (*session.Info, error)
}

func (s *nativeSessionBindingStub) ChangeWorktreeBinding(
	ctx context.Context,
	id, ref string,
	expected *string,
) (*session.Info, error) {
	return s.change(ctx, id, ref, expected)
}
func TestNativeSessionWorktreeBinding(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		tool        toolspkg.ToolID
		failure     error
		code        toolspkg.ErrorCode
	}{
		{name: "bind", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`},
		{name: "unbind", tool: toolspkg.ToolIDSessionWorktreeUnbind, input: `{"session_id":"sess-a","expected_worktree_id":""}`},
		{name: "busy", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: session.ErrPromptInProgress, code: toolspkg.ErrorCodeSessionBusy},
		{name: "NotFound", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: worktree.ErrNotFound, code: toolspkg.ErrorCodeWorktreeNotFound},
		{name: "NotReady", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: worktree.ErrNotReady, code: toolspkg.ErrorCodeWorktreeNotReady},
		{name: "Missing", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: worktree.ErrMissing, code: toolspkg.ErrorCodeWorktreeMissing},
		{name: "OperationInProgress", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: worktree.ErrOperationInProgress, code: toolspkg.ErrorCodeWorktreeOperationInProgress},
		{name: "SessionActive", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: worktree.ErrSessionActive, code: toolspkg.ErrorCodeWorktreeSessionActive},
		{name: "stale", tool: toolspkg.ToolIDSessionWorktreeBind, input: `{"session_id":"sess-a","worktree":"wt-new"}`, failure: store.ErrSessionWorktreeConflict, code: toolspkg.ErrorCodeSessionWorktreeConflict},
	} {
		t.Run("Should handle "+tc.name, func(t *testing.T) {
			calls := 0
			sessions := &nativeSessionBindingStub{
				StatusFn: func(context.Context, string) (*session.Info, error) {
					return &session.Info{
						ID:          "sess-a",
						WorkspaceID: "registry-a",
						ProfileID:   store.DefaultProfileID,
						WorktreeID:  "wt-old",
					}, nil
				},
				change: func(_ context.Context, id, ref string, expected *string) (*session.Info, error) {
					calls++
					wantRef, wantExpected := "wt-new", "wt-old"
					if tc.name == "unbind" {
						wantRef, wantExpected = "", ""
					}
					if id != "sess-a" || ref != wantRef || expected == nil || *expected != wantExpected {
						t.Fatalf("binding args: %q %q %v", id, ref, expected)
					}
					return &session.Info{
						ID:          id,
						WorkspaceID: "registry-a",
						ProfileID:   store.DefaultProfileID,
						WorktreeID:  ref,
					}, tc.failure
				},
			}
			native := &daemonNativeTools{
				deps: &daemonNativeToolsDeps{Workspaces: nativeWorktreeWorkspaceService(), Sessions: sessions},
			}
			_, err := native.sessionWorktreeBinding(
				t.Context(),
				toolspkg.Scope{WorkspaceID: "ws-a", ProfileID: store.DefaultProfileID},
				toolspkg.CallRequest{ToolID: tc.tool, Input: json.RawMessage(tc.input)},
			)
			if calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if tc.failure == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if typed, ok := errors.AsType[*toolspkg.ToolError](err); !ok || typed.Code != tc.code {
				t.Fatalf("error=%v want %s", err, tc.code)
			}
			wantStatus := http.StatusConflict
			if tc.failure == worktree.ErrNotFound {
				wantStatus = http.StatusNotFound
			}
			if got := core.StatusForToolError(err); got != wantStatus {
				t.Fatalf("tool status=%d want %d", got, wantStatus)
			}
		})
	}
}
