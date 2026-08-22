package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

type worktreeServiceStub struct {
	create        func(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
	remove        func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error)
	inspect       func(context.Context, string, string) (*worktree.Inspection, error)
	exitPlan      func(context.Context, string, string) (*worktree.ExitPlan, error)
	runExit       func(context.Context, string, string, worktree.ExitActionRequest) (string, error)
	cancelExit    func(context.Context, string, string, string) error
	catalogEvents <-chan worktree.CatalogEvent
}

func TestWorktreeOwnerProjection(t *testing.T) {
	t.Parallel()

	payload := WorktreePayloadFromInspection(worktree.Inspection{Worktree: worktree.Worktree{
		ID: "wt-profile-owner", ProfileID: "profile-owner", ProfileName: "Platform",
		ProfileColor: "#5FBF85", ProfileIcon: "circle", ProfileEmoji: "🛠️", ProfileArchived: true,
	}})
	if payload.ProfileID != "profile-owner" || payload.ProfileName != "Platform" ||
		payload.ProfileColor != "#5FBF85" || payload.ProfileIcon != "circle" ||
		payload.ProfileEmoji != "🛠️" || !payload.ProfileArchived {
		t.Fatalf("WorktreePayloadFromInspection() owner = %#v, want complete owner label", payload)
	}
}

func (s worktreeServiceStub) Create(
	ctx context.Context,
	workspaceID string,
	opts worktree.CreateOptions,
) (*worktree.Worktree, error) {
	if s.create != nil {
		return s.create(ctx, workspaceID, opts)
	}
	return nil, fmt.Errorf("unexpected Create call")
}

type worktreeObserverStub struct {
	Observer
	queryEvents func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error)
}

type worktreeSessionAcceptanceFunc func(
	context.Context,
	session.CreateAcceptedOpts,
) (*session.Info, error)

func (fn worktreeSessionAcceptanceFunc) CreateAccepted(
	ctx context.Context,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	return fn(ctx, opts)
}

type promptingSessionManager struct {
	sessionManagerStub
	createAccepted     func(context.Context, session.CreateAcceptedOpts) (*session.Info, error)
	isPrompting        func(string) bool
	worktreeForkOrigin string
}

func (m *promptingSessionManager) IsPrompting(sessionID string) bool {
	return m.isPrompting != nil && m.isPrompting(sessionID)
}

func (m *promptingSessionManager) CreateAccepted(
	ctx context.Context,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	return m.createAccepted(ctx, opts)
}

func (m *promptingSessionManager) CreateWorktreeForkAccepted(
	ctx context.Context,
	originSessionID string,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	m.worktreeForkOrigin = originSessionID
	return m.createAccepted(ctx, opts)
}

func (s worktreeObserverStub) QueryEvents(
	ctx context.Context,
	query store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return s.queryEvents(ctx, query)
}

func (s worktreeServiceStub) CreateAccepted(
	context.Context,
	string,
	worktree.CreateOptions,
) (*worktree.Worktree, error) {
	return nil, fmt.Errorf("unexpected CreateAccepted call")
}

func (s worktreeServiceStub) CreateReady(
	ctx context.Context,
	workspaceID string,
	opts worktree.CreateOptions,
) (*worktree.Worktree, error) {
	if s.create != nil {
		return s.create(ctx, workspaceID, opts)
	}
	return nil, fmt.Errorf("unexpected CreateReady call")
}

func (s worktreeServiceStub) CancelCreate(context.Context, string, string) error {
	return fmt.Errorf("unexpected CancelCreate call")
}

func (s worktreeServiceStub) Adopt(context.Context, string, string, string) (*worktree.Worktree, error) {
	return nil, fmt.Errorf("unexpected Adopt call")
}

func (s worktreeServiceStub) ListDetails(context.Context, string, bool) (*worktree.DetailedListing, error) {
	return nil, fmt.Errorf("unexpected ListDetails call")
}

func (s worktreeServiceStub) Resolve(context.Context, string, string) (*worktree.Worktree, error) {
	return nil, fmt.Errorf("unexpected Resolve call")
}

func (s worktreeServiceStub) Inspect(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
) (*worktree.Inspection, error) {
	if s.inspect != nil {
		return s.inspect(ctx, workspaceID, worktreeID)
	}
	return nil, fmt.Errorf("unexpected Inspect call")
}

func (s worktreeServiceStub) StatusDetails(
	context.Context,
	string,
	string,
	bool,
	bool,
) (*worktree.StatusDetails, error) {
	return nil, fmt.Errorf("unexpected StatusDetails call")
}

func (s worktreeServiceStub) ExitPlan(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
) (*worktree.ExitPlan, error) {
	if s.exitPlan != nil {
		return s.exitPlan(ctx, workspaceID, worktreeID)
	}
	return nil, fmt.Errorf("unexpected ExitPlan call")
}

func (s worktreeServiceStub) RunExitAction(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	request worktree.ExitActionRequest,
) (string, error) {
	if s.runExit != nil {
		return s.runExit(ctx, workspaceID, worktreeID, request)
	}
	return "", fmt.Errorf("unexpected RunExitAction call")
}

func (s worktreeServiceStub) CancelExitAction(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	opID string,
) error {
	if s.cancelExit != nil {
		return s.cancelExit(ctx, workspaceID, worktreeID, opID)
	}
	return fmt.Errorf("unexpected CancelExitAction call")
}

func (s worktreeServiceStub) Remove(
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

func (s worktreeServiceStub) Dismiss(context.Context, string, string) error {
	return fmt.Errorf("unexpected Dismiss call")
}

func (s worktreeServiceStub) SubscribeWorktreeCatalogEvents(
	context.Context,
) (<-chan worktree.CatalogEvent, func(), error) {
	return s.catalogEvents, func() {}, nil
}

func TestCreateSessionWorktreeBinding(t *testing.T) {
	t.Parallel()

	accepted := make([]session.CreateOpts, 0, 2)
	acceptFailure := false
	manager := worktreeSessionAcceptanceFunc(
		func(_ context.Context, opts session.CreateAcceptedOpts) (*session.Info, error) {
			accepted = append(accepted, opts.Session)
			if acceptFailure {
				return nil, errors.New("accept failed")
			}
			return &session.Info{
				ID: "sess-worktree", AgentName: opts.Session.AgentName,
				WorkspaceID: "ws-stable", WorktreeID: opts.Session.Worktree,
				State: session.StateActive,
			}, nil
		},
	)
	createdWorktrees := 0
	removedWorktrees := 0
	handlers := &BaseHandlers{
		TransportName:     "api-worktree-test",
		SessionAcceptance: manager,
		Workspaces: workspaceServiceStub{get: func(
			_ context.Context,
			ref string,
		) (workspacepkg.Workspace, error) {
			if ref != "workspace-ref" {
				t.Fatalf("Get() ref = %q, want %q", ref, "workspace-ref")
			}
			return workspacepkg.Workspace{ID: "ws-stable"}, nil
		}},
		Worktrees: worktreeServiceStub{
			create: func(
				_ context.Context,
				workspaceID string,
				opts worktree.CreateOptions,
			) (*worktree.Worktree, error) {
				createdWorktrees++
				if workspaceID != "ws-stable" || opts.Name != "feature-session" ||
					opts.Origin != worktree.OriginManual {
					t.Fatalf("Create() = workspace %q opts %#v", workspaceID, opts)
				}
				return &worktree.Worktree{ID: "wt-created", WorkspaceID: workspaceID}, nil
			},
			remove: func(
				_ context.Context,
				workspaceID string,
				worktreeID string,
				force bool,
			) (*worktree.RemovalRefusal, error) {
				removedWorktrees++
				if workspaceID != "ws-stable" || worktreeID != "wt-created" || !force {
					t.Fatalf("Remove() = workspace %q worktree %q force %v", workspaceID, worktreeID, force)
				}
				return nil, nil
			},
		},
	}
	router := gin.New()
	router.POST("/sessions", handlers.CreateSession)
	request := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/sessions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	direct := request(`{"agent_name":"coder","workspace":"workspace-ref","worktree":"branch-ref"}`)
	if direct.Code != http.StatusCreated {
		t.Fatalf("direct binding status = %d, want %d; body=%s", direct.Code, http.StatusCreated, direct.Body.String())
	}
	if len(accepted) != 1 || accepted[0].Worktree != "branch-ref" {
		t.Fatalf("direct binding accepted opts = %#v, want worktree ref", accepted)
	}
	var directPayload contract.SessionResponse
	if err := json.Unmarshal(direct.Body.Bytes(), &directPayload); err != nil {
		t.Fatalf("json.Unmarshal(direct response) error = %v", err)
	}
	if directPayload.Session.WorktreeID != "branch-ref" {
		t.Fatalf("direct response worktree_id = %q, want %q", directPayload.Session.WorktreeID, "branch-ref")
	}

	materialized := request(
		`{"agent_name":"coder","workspace":"workspace-ref","new_worktree":{"name":"feature-session"}}`,
	)
	if materialized.Code != http.StatusCreated {
		t.Fatalf(
			"new worktree status = %d, want %d; body=%s",
			materialized.Code,
			http.StatusCreated,
			materialized.Body.String(),
		)
	}
	if createdWorktrees != 1 || len(accepted) != 2 || accepted[1].Worktree != "wt-created" {
		t.Fatalf("new worktree calls = %d accepted = %#v", createdWorktrees, accepted)
	}

	invalid := []string{
		`{"workspace":"workspace-ref","worktree":"wt-a","new_worktree":{}}`,
		`{"workspace_path":"/tmp/project","worktree":"wt-a"}`,
		`{"workspace_path":"/tmp/project","new_worktree":{}}`,
	}
	for _, body := range invalid {
		beforeAccepted := len(accepted)
		beforeCreated := createdWorktrees
		response := request(body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf(
				"invalid binding status = %d, want %d; body=%s",
				response.Code,
				http.StatusBadRequest,
				response.Body.String(),
			)
		}
		if len(accepted) != beforeAccepted || createdWorktrees != beforeCreated {
			t.Fatalf("invalid binding caused mutation: accepted=%d worktrees=%d", len(accepted), createdWorktrees)
		}
	}

	acceptFailure = true
	failed := request(
		`{"agent_name":"coder","workspace":"workspace-ref","new_worktree":{"name":"feature-session"}}`,
	)
	if failed.Code != http.StatusInternalServerError || removedWorktrees != 1 {
		t.Fatalf(
			"failed session status/removals = %d/%d, want 500/1; body=%s",
			failed.Code,
			removedWorktrees,
			failed.Body.String(),
		)
	}
}

func TestForkSessionToWorktree(t *testing.T) {
	t.Parallel()

	newHandlers := func(
		t *testing.T,
		prompting func(string) bool,
		createWorktree func(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error),
		removeWorktree func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error),
		accept func(context.Context, session.CreateAcceptedOpts) (*session.Info, error),
	) *BaseHandlers {
		t.Helper()
		if removeWorktree == nil {
			removeWorktree = func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error) {
				return nil, nil
			}
		}
		manager := &promptingSessionManager{
			sessionManagerStub: sessionManagerStub{
				status: func(_ context.Context, id string) (*session.Info, error) {
					if id != "sess-origin" {
						t.Fatalf("Status() id = %q, want sess-origin", id)
					}
					return &session.Info{
						ID: "sess-origin", AgentName: "coder", WorkspaceID: "registry-a",
						State: session.StateActive,
					}, nil
				},
			},
			createAccepted: accept,
			isPrompting:    prompting,
		}
		return &BaseHandlers{
			Sessions:          manager,
			SessionAcceptance: manager,
			Workspaces: workspaceServiceStub{resolve: func(
				_ context.Context,
				ref string,
			) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "workspace-a" {
					t.Fatalf("Resolve() ref = %q, want workspace-a", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "registry-a"},
					WorkspaceID: "workspace-a",
				}, nil
			}},
			Worktrees: worktreeServiceStub{create: createWorktree, remove: removeWorktree},
		}
	}
	request := func(t *testing.T, handlers *BaseHandlers, body string) *httptest.ResponseRecorder {
		t.Helper()
		router := gin.New()
		router.POST(
			"/workspaces/:workspace_id/sessions/:session_id/worktree-fork",
			handlers.ForkSessionToWorktree,
		)
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/workspaces/workspace-a/sessions/sess-origin/worktree-fork",
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	t.Run("Should require explicit confirmation before creating anything", func(t *testing.T) {
		t.Parallel()
		accepted := 0
		handlers := newHandlers(t, nil, nil, nil, func(
			context.Context,
			session.CreateAcceptedOpts,
		) (*session.Info, error) {
			accepted++
			return nil, nil
		})
		response := request(t, handlers, `{"worktree":"wt-existing","confirmed":false}`)
		if response.Code != http.StatusBadRequest || accepted != 0 {
			t.Fatalf("status/accepted = %d/%d, want 400/0; body=%s", response.Code, accepted, response.Body.String())
		}
	})

	t.Run("Should create one clean child session in an existing worktree", func(t *testing.T) {
		t.Parallel()
		var accepted session.CreateOpts
		promptChecks := 0
		handlers := newHandlers(t, func(id string) bool {
			if id != "sess-origin" {
				t.Fatalf("IsPrompting() id = %q, want sess-origin", id)
			}
			promptChecks++
			return false
		}, nil, nil, func(_ context.Context, opts session.CreateAcceptedOpts) (*session.Info, error) {
			accepted = opts.Session
			return &session.Info{
				ID: "sess-child", AgentName: accepted.AgentName, WorkspaceID: accepted.Workspace,
				WorktreeID: accepted.Worktree, State: session.StateActive, Lineage: accepted.Lineage,
			}, nil
		})
		response := request(t, handlers, `{"worktree":"wt-existing","confirmed":true}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", response.Code, response.Body.String())
		}
		if promptChecks != 2 || accepted.AgentName != "coder" || accepted.Workspace != "registry-a" ||
			accepted.Worktree != "wt-existing" || accepted.Type != session.SessionTypeUser ||
			accepted.Lineage == nil || accepted.Lineage.ParentSessionID != "sess-origin" ||
			accepted.CWD != "" || accepted.Name != "" || accepted.Provider != "" {
			t.Fatalf("accepted fork = %#v; prompt checks = %d", accepted, promptChecks)
		}
		manager := handlers.SessionAcceptance.(*promptingSessionManager)
		if manager.worktreeForkOrigin != "sess-origin" {
			t.Fatalf("fork origin = %q, want sess-origin", manager.worktreeForkOrigin)
		}
	})

	t.Run("Should materialize a new target before accepting the child session", func(t *testing.T) {
		t.Parallel()
		created := 0
		accepted := 0
		handlers := newHandlers(t, func(string) bool { return false }, func(
			_ context.Context,
			workspaceID string,
			opts worktree.CreateOptions,
		) (*worktree.Worktree, error) {
			created++
			if workspaceID != "registry-a" || opts.Name != "fork-target" || opts.Origin != worktree.OriginManual {
				t.Fatalf("CreateReady() workspace/options = %q/%#v", workspaceID, opts)
			}
			return &worktree.Worktree{ID: "wt-created", WorkspaceID: workspaceID, State: worktree.StateReady}, nil
		}, nil, func(_ context.Context, opts session.CreateAcceptedOpts) (*session.Info, error) {
			accepted++
			return &session.Info{
				ID: "sess-child", AgentName: opts.Session.AgentName, WorkspaceID: opts.Session.Workspace,
				WorktreeID: opts.Session.Worktree, State: session.StateActive,
			}, nil
		})
		response := request(t, handlers, `{"new_worktree":{"name":"fork-target"},"confirmed":true}`)
		if response.Code != http.StatusCreated || created != 1 || accepted != 1 {
			t.Fatalf(
				"status/created/accepted = %d/%d/%d, want 201/1/1; body=%s",
				response.Code,
				created,
				accepted,
				response.Body.String(),
			)
		}
	})

	t.Run("Should recheck prompt activity after materialization and refuse the race", func(t *testing.T) {
		t.Parallel()
		promptChecks := 0
		accepted := 0
		handlers := newHandlers(t, func(string) bool {
			promptChecks++
			return promptChecks == 2
		}, func(
			context.Context,
			string,
			worktree.CreateOptions,
		) (*worktree.Worktree, error) {
			return &worktree.Worktree{ID: "wt-created", State: worktree.StateReady}, nil
		}, nil, func(context.Context, session.CreateAcceptedOpts) (*session.Info, error) {
			accepted++
			return nil, nil
		})
		response := request(t, handlers, `{"new_worktree":{},"confirmed":true}`)
		if response.Code != http.StatusConflict || promptChecks != 2 || accepted != 0 {
			t.Fatalf(
				"status/checks/accepted = %d/%d/%d, want 409/2/0; body=%s",
				response.Code,
				promptChecks,
				accepted,
				response.Body.String(),
			)
		}
	})

	t.Run("Should roll back a newly materialized target when child acceptance fails", func(t *testing.T) {
		t.Parallel()
		removed := 0
		handlers := newHandlers(t, func(string) bool { return false }, func(
			context.Context,
			string,
			worktree.CreateOptions,
		) (*worktree.Worktree, error) {
			return &worktree.Worktree{ID: "wt-created", WorkspaceID: "registry-a", State: worktree.StateReady}, nil
		}, func(
			_ context.Context,
			workspaceID string,
			worktreeID string,
			force bool,
		) (*worktree.RemovalRefusal, error) {
			removed++
			if workspaceID != "registry-a" || worktreeID != "wt-created" || !force {
				t.Fatalf("Remove() = workspace %q worktree %q force %v", workspaceID, worktreeID, force)
			}
			return nil, nil
		}, func(context.Context, session.CreateAcceptedOpts) (*session.Info, error) {
			return nil, errors.New("accept failed")
		})
		response := request(t, handlers, `{"new_worktree":{},"confirmed":true}`)
		if response.Code != http.StatusInternalServerError || removed != 1 {
			t.Fatalf("status/removed = %d/%d, want 500/1; body=%s", response.Code, removed, response.Body.String())
		}
	})
}

func TestWorktreeErrorCodes(t *testing.T) {
	t.Parallel()

	t.Run("Should map every sentinel to one unique wire code", func(t *testing.T) {
		t.Parallel()

		expected := []struct {
			err  error
			code string
		}{
			{worktree.ErrGitUnavailable, "worktree_git_unavailable"},
			{worktree.ErrGitVersionUnsupported, "worktree_git_version_unsupported"},
			{worktree.ErrWorkspaceNotGitBacked, "workspace_not_git_backed"},
			{worktree.ErrNameTaken, "worktree_name_taken"},
			{worktree.ErrPathExists, "worktree_path_exists"},
			{worktree.ErrBranchHeld, "branch_held_by_worktree"},
			{worktree.ErrBranchCheckedOutAtRoot, "branch_checked_out_at_root"},
			{worktree.ErrBaseRefNotFound, "base_ref_not_found"},
			{worktree.ErrRepoHasNoCommits, "repo_has_no_commits"},
			{worktree.ErrNotFound, "worktree_not_found"},
			{worktree.ErrNotReady, "worktree_not_ready"},
			{worktree.ErrPending, "worktree_pending"},
			{worktree.ErrMissing, "worktree_missing"},
			{worktree.ErrRefInvalid, "worktree_ref_invalid"},
			{worktree.ErrAdoptionMainCheckout, "adoption_main_checkout"},
			{worktree.ErrAdoptionForeignRepo, "adoption_foreign_repository"},
			{worktree.ErrAdoptionUnreadable, "adoption_unreadable"},
			{worktree.ErrOperationInProgress, "worktree_operation_in_progress"},
			{worktree.ErrSessionActive, "worktree_session_active"},
			{worktree.ErrStatusUnreadable, "worktree_status_unreadable"},
			{worktree.ErrDirtyRequiresForce, "worktree_dirty_requires_force"},
			{worktree.ErrUnpushedRequiresForce, "worktree_unpushed_requires_force"},
			{worktree.ErrSafetyCheckFailed, "worktree_safety_check_failed"},
			{worktree.ErrRemovalFailed, "worktree_removal_failed"},
			{worktree.ErrPerRunMaterialization, "per_run_materialization_failed"},
			{worktree.ErrConfigInvalid, "worktree_config_invalid"},
			{worktree.ErrDeniedByHook, "worktree_denied_by_hook"},
			{worktree.ErrNotPending, "worktree_not_pending"},
			{worktree.ErrForgeUnavailable, "forge_unavailable"},
			{worktree.ErrForge, "forge_error"},
			{worktree.ErrExitActionInvalid, "worktree_exit_action_invalid"},
		}

		if len(worktreeErrorCodes) != len(expected) {
			t.Fatalf("worktree error map count = %d, want %d", len(worktreeErrorCodes), len(expected))
		}
		seen := make(map[string]error, len(expected))
		for _, item := range expected {
			got := WorktreeErrorCode(fmt.Errorf("wrapped: %w", item.err))
			if got != item.code {
				t.Fatalf("WorktreeErrorCode(%v) = %q, want %q", item.err, got, item.code)
			}
			if previous, exists := seen[got]; exists {
				t.Fatalf("wire code %q maps both %v and %v", got, previous, item.err)
			}
			seen[got] = item.err
		}
	})

	t.Run("Should expose a safe machine-readable forge failure cause", func(t *testing.T) {
		t.Parallel()
		failure := worktree.NewForgeFailure(
			worktree.ErrForge, "rate_limited", errors.New("provider internal detail"),
		)
		payload := ErrorPayloadForStatus(http.StatusBadGateway, failure, true)
		if payload.Error != "forge_error" || payload.Code != "forge_error" ||
			payload.Details["cause"] != "rate_limited" || strings.Contains(payload.Error, "internal") {
			t.Fatalf("forge failure payload = %#v, want masked error with typed cause", payload)
		}
	})
}

func TestWorktreeExitHandlers(t *testing.T) {
	t.Parallel()

	var runRequest worktree.ExitActionRequest
	var canceledOperation string
	handlers := &BaseHandlers{
		Workspaces: workspaceServiceStub{resolve: func(
			_ context.Context,
			ref string,
		) (workspacepkg.ResolvedWorkspace, error) {
			if ref != "workspace-a" {
				t.Fatalf("Resolve() ref = %q, want workspace-a", ref)
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{ID: "registry-a"}, WorkspaceID: "workspace-a",
			}, nil
		}},
		Worktrees: worktreeServiceStub{
			exitPlan: func(_ context.Context, workspaceID, worktreeID string) (*worktree.ExitPlan, error) {
				if workspaceID != "registry-a" || worktreeID != "wt-a" {
					t.Fatalf("ExitPlan() args = %q, %q", workspaceID, worktreeID)
				}
				return &worktree.ExitPlan{
					WorktreeID: worktreeID, Primary: worktree.ExitActionCommitPush, Base: "main",
					Actions: []worktree.ExitActionPlan{{
						Action: worktree.ExitActionCommitPush, Label: "Commit and push", Enabled: true, Publish: true,
					}},
					CommitScope: worktree.ExitCommitScope{
						ChangedFiles: 2, Insertions: 3, Deletions: 1, UntrackedFiles: []string{"new.txt"},
						UntrackedTotal: 1,
					},
					Forge: &worktree.ForgeCapabilities{
						Provider: "github", RequestNoun: "PR", OpenActionLabel: "Open PR",
						ViewActionLabel: "View PR", SupportsDraft: true, CredentialSource: "binding",
					},
					PRPrefill:  &worktree.ExitPRPrefill{Title: "feature/exit", Body: "## Summary\nReady."},
					Cleanup:    worktree.ExitCleanupEvidence{ForgeState: "open", Safe: true, Source: "forge"},
					RemoteURLs: []string{"https://token@github.com/acme/repo.git"},
				}, nil
			},
			runExit: func(
				_ context.Context,
				workspaceID, worktreeID string,
				request worktree.ExitActionRequest,
			) (string, error) {
				if workspaceID != "registry-a" || worktreeID != "wt-a" {
					t.Fatalf("RunExitAction() args = %q, %q", workspaceID, worktreeID)
				}
				runRequest = request
				return "op-exit", nil
			},
			cancelExit: func(_ context.Context, workspaceID, worktreeID, opID string) error {
				if workspaceID != "registry-a" || worktreeID != "wt-a" {
					t.Fatalf("CancelExitAction() args = %q, %q", workspaceID, worktreeID)
				}
				canceledOperation = opID
				return nil
			},
		},
	}
	router := gin.New()
	path := "/workspaces/:workspace_id/worktrees/:worktree_id/exit"
	router.GET(path, handlers.GetWorktreeExitPlan)
	router.POST(path+"/actions", handlers.RunWorktreeExitAction)
	router.POST(path+"/cancel", handlers.CancelWorktreeExitAction)

	t.Run("Should return the complete plan without internal remote URLs", func(t *testing.T) {
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/workspaces/workspace-a/worktrees/wt-a/exit", http.NoBody,
		)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var payload contract.WorktreeExitPlanResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode exit plan: %v", err)
		}
		if response.Code != http.StatusOK || payload.WorktreeID != "wt-a" || payload.Primary != "commit_push" ||
			len(payload.Actions) != 1 || payload.CommitScope.UntrackedTotal != 1 || payload.Forge == nil ||
			payload.Forge.Provider != "github" || payload.PRPrefill == nil || payload.PRPrefill.Title != "feature/exit" ||
			payload.PRPrefill.Body != "## Summary\nReady." || !payload.Cleanup.Safe ||
			strings.Contains(response.Body.String(), "token@") {
			t.Fatalf("exit plan status/body = %d/%s", response.Code, response.Body.String())
		}
	})

	t.Run("Should serialize an empty untracked scope as an array", func(t *testing.T) {
		t.Parallel()
		payload := WorktreeExitPlanPayload(&worktree.ExitPlan{})
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal exit plan: %v", err)
		}
		if !strings.Contains(string(encoded), `"untracked_files":[]`) {
			t.Fatalf("empty exit plan = %s, want untracked_files array", encoded)
		}
	})

	t.Run("Should accept an action with snake-case options", func(t *testing.T) {
		body := `{"action":"open_pr","message":"Commit","title":"Exit","body":"Ready","draft":true,"base":"trunk"}`
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost, "/workspaces/workspace-a/worktrees/wt-a/exit/actions", strings.NewReader(body),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusAccepted || response.Body.String() != `{"op_id":"op-exit"}` ||
			runRequest.Action != worktree.ExitActionOpenPR || runRequest.Message != "Commit" ||
			runRequest.Title != "Exit" || runRequest.Body != "Ready" || !runRequest.Draft || runRequest.Base != "trunk" {
			t.Fatalf("action status/body/request = %d/%s/%#v", response.Code, response.Body.String(), runRequest)
		}
	})

	t.Run("Should reject an unknown action before mutation", func(t *testing.T) {
		before := runRequest
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost, "/workspaces/workspace-a/worktrees/wt-a/exit/actions",
			strings.NewReader(`{"action":"publish_everything"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || runRequest != before {
			t.Fatalf("invalid action status/request = %d/%#v", response.Code, runRequest)
		}
	})

	t.Run("Should cancel the named operation", func(t *testing.T) {
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost, "/workspaces/workspace-a/worktrees/wt-a/exit/cancel",
			strings.NewReader(`{"op_id":" op-exit "}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || canceledOperation != "op-exit" {
			t.Fatalf("cancel status/op = %d/%q", response.Code, canceledOperation)
		}
	})
}

func TestRemoveWorktreeRefusal(t *testing.T) {
	t.Parallel()

	t.Run("Should return the exact risk inventory with status 409", func(t *testing.T) {
		t.Parallel()

		handlers := &BaseHandlers{
			Workspaces: workspaceServiceStub{resolve: func(
				_ context.Context,
				ref string,
			) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "registry-a", Name: ref},
					WorkspaceID: "workspace-a",
				}, nil
			}},
			Worktrees: worktreeServiceStub{remove: func(
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
			}},
		}
		router := gin.New()
		router.DELETE("/workspaces/:workspace_id/worktrees/:worktree_id", handlers.RemoveWorktree)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodDelete,
			"/workspaces/workspace-a/worktrees/wt-a",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
		}
		const want = `{"code":"worktree_dirty_requires_force","risk":{"changed_files":2,"insertions":4,"deletions":1,"unpushed_commits":3,"exists_on_remote":false},"downgrade":false}`
		if got := response.Body.String(); got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
	})
}

func TestWorktreeStreams(t *testing.T) {
	t.Parallel()

	t.Run("Should replay canonical worktree events by name in order after the requested sequence", func(t *testing.T) {
		t.Parallel()

		events := []store.EventSummary{
			{Sequence: 1, Type: worktree.EventCreated, Content: json.RawMessage(`{"state":"ready"}`)},
			{Sequence: 2, Type: worktree.EventStatusRefreshed, Content: json.RawMessage(`{"dirty":true}`)},
			{Sequence: 3, Type: worktree.EventRemoved, Content: json.RawMessage(`{"state":"removed"}`)},
		}
		stream := func(afterSequence int64) string {
			t.Helper()
			requestContext, cancelRequest := context.WithCancel(context.Background())
			observer := worktreeObserverStub{queryEvents: func(
				_ context.Context,
				query store.EventSummaryQuery,
			) ([]store.EventSummary, error) {
				if query.WorkspaceID != "registry-a" || query.WorktreeID != "wt-a" ||
					query.AfterSequence != afterSequence || query.Limit != worktreeReplayLimit {
					t.Fatalf("worktree stream query = %#v", query)
				}
				result := make([]store.EventSummary, 0, len(events))
				for _, event := range events {
					if event.Sequence > query.AfterSequence {
						result = append(result, event)
					}
				}
				cancelRequest()
				return result, nil
			}}
			handlers := &BaseHandlers{
				PollInterval: time.Hour,
				Observer:     observer,
				Workspaces: workspaceServiceStub{resolve: func(
					_ context.Context,
					ref string,
				) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "registry-a", Name: ref},
						WorkspaceID: "workspace-a",
					}, nil
				}},
				Worktrees: worktreeServiceStub{inspect: func(
					_ context.Context,
					workspaceID string,
					worktreeRef string,
				) (*worktree.Inspection, error) {
					if worktreeRef != "parity" {
						t.Fatalf("Inspect() ref = %q, want name parity", worktreeRef)
					}
					return &worktree.Inspection{Worktree: worktree.Worktree{
						ID: "wt-a", WorkspaceID: workspaceID,
					}}, nil
				}},
			}
			router := gin.New()
			router.GET("/workspaces/:workspace_id/worktrees/:worktree_id/stream", handlers.StreamWorktree)
			request := httptest.NewRequestWithContext(
				requestContext,
				http.MethodGet,
				fmt.Sprintf("/workspaces/workspace-a/worktrees/parity/stream?after_sequence=%d", afterSequence),
				http.NoBody,
			)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("stream status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
			return response.Body.String()
		}

		initial := stream(0)
		for sequence := int64(1); sequence <= 3; sequence++ {
			if count := strings.Count(initial, fmt.Sprintf("id: %d\n", sequence)); count != 1 {
				t.Fatalf("initial stream sequence %d count = %d; body=%s", sequence, count, initial)
			}
		}
		if strings.Index(initial, "id: 1\n") >= strings.Index(initial, "id: 2\n") ||
			strings.Index(initial, "id: 2\n") >= strings.Index(initial, "id: 3\n") {
			t.Fatalf("initial stream order is not monotonic: %s", initial)
		}

		replayed := stream(1)
		if strings.Contains(replayed, "id: 1\n") ||
			strings.Count(replayed, "id: 2\n") != 1 ||
			strings.Count(replayed, "id: 3\n") != 1 {
			t.Fatalf("replayed stream has a gap or duplicate: %s", replayed)
		}
	})

	t.Run("Should emit workspace-attributed catalog changes", func(t *testing.T) {
		t.Parallel()

		events := make(chan worktree.CatalogEvent, 1)
		events <- worktree.CatalogEvent{
			Kind: worktree.CatalogEventUpserted, WorkspaceID: "registry-a", WorktreeID: "wt-a",
		}
		close(events)
		handlers := &BaseHandlers{Worktrees: worktreeServiceStub{catalogEvents: events}}
		router := gin.New()
		router.GET("/worktrees/catalog-stream", handlers.StreamWorktreeCatalog)
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/worktrees/catalog-stream", http.NoBody,
		)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		body := response.Body.String()
		if response.Code != http.StatusOK ||
			!strings.Contains(body, "event: worktree_catalog_changed\n") ||
			!strings.Contains(body, `"workspace_id":"registry-a"`) ||
			!strings.Contains(body, `"worktree_id":"wt-a"`) {
			t.Fatalf("catalog stream status/body = %d/%s", response.Code, body)
		}
	})
}
