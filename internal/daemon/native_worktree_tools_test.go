package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

type nativeWorktreeServiceStub struct {
	create        func(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
	list          func(context.Context, string, bool) (*worktree.DetailedListing, error)
	remove        func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error)
	capabilityErr error
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
	context.Context,
	string,
	worktree.CreateOptions,
) (*worktree.Worktree, error) {
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

func (s *nativeWorktreeServiceStub) Adopt(context.Context, string, string) (*worktree.Worktree, error) {
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
	context.Context,
	string,
	string,
) (*worktree.Inspection, error) {
	return nil, fmt.Errorf("unexpected Inspect call")
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

func TestNativeWorktreeTools(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve workspace identity to the registry boundary without leaking another workspace", func(t *testing.T) {
		t.Parallel()

		workspaces := nativeWorktreeWorkspaceService()
		sessions := apitest.StubSessionManager{StatusFn: func(
			_ context.Context,
			id string,
		) (*session.Info, error) {
			return &session.Info{
				ID: id, WorkspaceID: "ws-a", AgentName: "coder", State: session.StateActive,
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
	})

	t.Run("Should carry the exact removal refusal as a partial tool result", func(t *testing.T) {
		t.Parallel()

		service := &nativeWorktreeServiceStub{remove: func(
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
			Input:  json.RawMessage(`{"workspace":"ws-a","ref":"wt-a"}`),
		})
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.PartialResult == nil {
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
				Workspace:   workspacepkg.Workspace{ID: "registry-a", Name: "alpha"},
				WorkspaceID: "ws-a",
			}, nil
		case "ws-b", "registry-b":
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "registry-b", Name: "beta"},
				WorkspaceID: "ws-b",
			}, nil
		default:
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
		}
	}}
}
