package testutil_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

// Canonical suite: IT-033 HTTP/UDS payload and error parity for the complete worktree route contract.
func TestWorktreeHTTPUDSTransportParityIT033(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should return byte-identical JSON or empty success bodies for every finite worktree route",
		func(t *testing.T) {
			t.Parallel()
			for _, test := range []struct {
				name           string
				method         string
				path           string
				body           string
				wantStatus     int
				wantRef        string
				wantWorktreeID string
			}{
				{name: "list", method: http.MethodGet, path: "/api/workspaces/ws-public/worktrees", wantStatus: http.StatusOK},
				{name: "create", method: http.MethodPost, path: "/api/workspaces/ws-public/worktrees", body: `{"name":"feature"}`, wantStatus: http.StatusAccepted},
				{name: "adopt", method: http.MethodPost, path: "/api/workspaces/ws-public/worktrees/adopt", body: `{"path":"/repo/feature"}`, wantStatus: http.StatusOK},
				{name: "inspect by name", method: http.MethodGet, path: "/api/workspaces/ws-public/worktrees/parity", wantStatus: http.StatusOK, wantRef: "parity"},
				{name: "status by name", method: http.MethodGet, path: "/api/workspaces/ws-public/worktrees/parity/status?refresh=true", wantStatus: http.StatusOK, wantRef: "parity", wantWorktreeID: "wt-parity"},
				{name: "exit plan by name", method: http.MethodGet, path: "/api/workspaces/ws-public/worktrees/parity/exit", wantStatus: http.StatusOK, wantRef: "parity"},
				{name: "exit action by name", method: http.MethodPost, path: "/api/workspaces/ws-public/worktrees/parity/exit/actions", body: `{"action":"commit"}`, wantStatus: http.StatusAccepted, wantRef: "parity"},
				{name: "exit cancel by name", method: http.MethodPost, path: "/api/workspaces/ws-public/worktrees/parity/exit/cancel", body: `{"op_id":"op-parity"}`, wantStatus: http.StatusNoContent, wantRef: "parity"},
				{name: "create cancel by name", method: http.MethodPost, path: "/api/workspaces/ws-public/worktrees/parity/cancel", wantStatus: http.StatusNoContent, wantRef: "parity"},
				{name: "remove by name", method: http.MethodDelete, path: "/api/workspaces/ws-public/worktrees/parity", wantStatus: http.StatusNoContent, wantRef: "parity"},
				{name: "dismiss by name", method: http.MethodPost, path: "/api/workspaces/ws-public/worktrees/parity/dismiss", wantStatus: http.StatusNoContent, wantRef: "parity"},
			} {
				t.Run("Should match "+test.name, func(t *testing.T) {
					t.Parallel()
					service := newParityWorktreeService()
					httpRouter := newWorktreeParityHTTPRouter(t, service)
					udsRouter := newWorktreeParityUDSRouter(t, service)
					httpResponse := performWorktreeParityRequest(t, httpRouter, test.method, test.path, test.body)
					udsResponse := performWorktreeParityRequest(t, udsRouter, test.method, test.path, test.body)
					assertWorktreeParityResponse(t, httpResponse, udsResponse, test.wantStatus)
					if test.wantRef != "" &&
						(len(service.refs) != 2 ||
							service.refs[0] != test.wantRef ||
							service.refs[1] != test.wantRef) {
						t.Fatalf("forwarded refs = %#v, want HTTP and UDS ref %q", service.refs, test.wantRef)
					}
					if test.wantWorktreeID != "" {
						var status contract.WorktreeStatusResponse
						if err := json.Unmarshal(httpResponse.Body.Bytes(), &status); err != nil {
							t.Fatalf("decode status response: %v", err)
						}
						if status.WorktreeID != test.wantWorktreeID {
							t.Fatalf("status worktree id = %q, want %q", status.WorktreeID, test.wantWorktreeID)
						}
					}
				})
			}
		},
	)

	t.Run("Should return byte-identical removal refusal tiers", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name    string
			refusal worktree.RemovalRefusal
		}{
			{name: "dirty", refusal: worktree.RemovalRefusal{
				Code: worktree.ErrDirtyRequiresForce.Error(),
				Risk: worktree.RemovalRisk{ChangedFiles: 2, Insertions: 4, Deletions: 1},
			}},
			{name: "unpushed", refusal: worktree.RemovalRefusal{
				Code: worktree.ErrUnpushedRequiresForce.Error(),
				Risk: worktree.RemovalRisk{UnpushedCommits: 3},
			}},
		} {
			t.Run("Should match "+test.name+" refusal", func(t *testing.T) {
				service := newParityWorktreeService()
				service.removal = &test.refusal
				httpResponse := performWorktreeParityRequest(
					t, newWorktreeParityHTTPRouter(t, service), http.MethodDelete,
					"/api/workspaces/ws-public/worktrees/wt-parity", "",
				)
				udsResponse := performWorktreeParityRequest(
					t, newWorktreeParityUDSRouter(t, service), http.MethodDelete,
					"/api/workspaces/ws-public/worktrees/wt-parity", "",
				)
				assertWorktreeParityResponse(t, httpResponse, udsResponse, http.StatusConflict)
			})
		}
	})

	t.Run("Should return byte-identical payloads for every worktree error code", func(t *testing.T) {
		t.Parallel()
		errorsByCode := worktreeParityErrors()
		if len(errorsByCode) != 31 {
			t.Fatalf("worktree parity error contract count = %d, want 31", len(errorsByCode))
		}
		for _, test := range errorsByCode {
			t.Run("Should match "+test.code, func(t *testing.T) {
				service := newParityWorktreeService()
				service.inspectErr = fmt.Errorf("parity: %w", test.err)
				httpResponse := performWorktreeParityRequest(
					t, newWorktreeParityHTTPRouter(t, service), http.MethodGet,
					"/api/workspaces/ws-public/worktrees/wt-parity", "",
				)
				udsResponse := performWorktreeParityRequest(
					t, newWorktreeParityUDSRouter(t, service), http.MethodGet,
					"/api/workspaces/ws-public/worktrees/wt-parity", "",
				)
				assertWorktreeParityResponse(t, httpResponse, udsResponse, core.StatusForWorktreeError(test.err))
				if !bytes.Contains(httpResponse.Body.Bytes(), []byte(`"code":"`+test.code+`"`)) {
					t.Fatalf("error response for %s = %s", test.code, httpResponse.Body.String())
				}
			})
		}
	})
}

func newWorktreeParityHTTPRouter(t *testing.T, service core.WorktreeService) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", 2123
	if _, err := httpapi.New(
		httpapi.WithEngine(engine), httpapi.WithHomePaths(homePaths), httpapi.WithConfig(&cfg),
		httpapi.WithHost(cfg.HTTP.Host), httpapi.WithPort(cfg.HTTP.Port),
		httpapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		httpapi.WithStartedAt(parityWorktreeTime()), httpapi.WithNow(parityWorktreeTime),
		httpapi.WithSessionManager(testutil.StubSessionManager{}),
		httpapi.WithTaskService(&testutil.StubTaskManager{}), httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(parityWorkspaceService()), httpapi.WithWorktreeService(service),
	); err != nil {
		t.Fatalf("httpapi.New(worktree parity) error = %v", err)
	}
	return engine
}

func newWorktreeParityUDSRouter(t *testing.T, service core.WorktreeService) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	if _, err := udsapi.New(
		udsapi.WithEngine(engine), udsapi.WithHomePaths(homePaths), udsapi.WithConfig(&cfg),
		udsapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		udsapi.WithStartedAt(parityWorktreeTime()), udsapi.WithNow(parityWorktreeTime),
		udsapi.WithSessionManager(testutil.StubSessionManager{}),
		udsapi.WithTaskService(&testutil.StubTaskManager{}), udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(parityWorkspaceService()), udsapi.WithWorktreeService(service),
	); err != nil {
		t.Fatalf("udsapi.New(worktree parity) error = %v", err)
	}
	return engine
}

func parityWorkspaceService() testutil.StubWorkspaceService {
	return testutil.StubWorkspaceService{ResolveFn: func(
		context.Context,
		string,
	) (workspace.ResolvedWorkspace, error) {
		return workspace.ResolvedWorkspace{
			Workspace:   workspace.Workspace{ID: "ws-registry", Name: "Parity", RootDir: "/repo"},
			WorkspaceID: "ws-public",
		}, nil
	}}
}

func performWorktreeParityRequest(
	t testing.TB,
	handler http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertWorktreeParityResponse(
	t testing.TB,
	httpResponse *httptest.ResponseRecorder,
	udsResponse *httptest.ResponseRecorder,
	wantStatus int,
) {
	t.Helper()
	if httpResponse.Code != wantStatus || udsResponse.Code != wantStatus {
		t.Fatalf(
			"statuses = HTTP:%d UDS:%d, want %d; bodies HTTP=%s UDS=%s",
			httpResponse.Code, udsResponse.Code, wantStatus, httpResponse.Body.String(), udsResponse.Body.String(),
		)
	}
	if !bytes.Equal(httpResponse.Body.Bytes(), udsResponse.Body.Bytes()) {
		t.Fatalf("HTTP/UDS bodies differ:\nHTTP: %s\nUDS:  %s", httpResponse.Body.Bytes(), udsResponse.Body.Bytes())
	}
}

type parityWorktreeService struct {
	item       worktree.Worktree
	status     worktree.Status
	inspectErr error
	removal    *worktree.RemovalRefusal
	refs       []string
}

func (s *parityWorktreeService) recordRef(ref string) { s.refs = append(s.refs, ref) }

func newParityWorktreeService() *parityWorktreeService {
	dirty, ahead, behind, upstream, detached := 1, 0, 0, true, false
	branch, head := "feature/parity", "abc123"
	refreshed := parityWorktreeTime()
	return &parityWorktreeService{
		item: worktree.Worktree{
			ID: "wt-parity", WorkspaceID: "ws-registry", Name: "parity", Branch: branch,
			Path: "/repo/feature", State: worktree.StateReady, Origin: worktree.OriginManual,
			SetupState: worktree.SetupNone, BaseRef: "main", CreatedAt: refreshed, UpdatedAt: refreshed,
		},
		status: worktree.Status{
			WorktreeID: "wt-parity", Branch: &branch, Detached: &detached, HeadSHA: &head,
			DirtyFiles: &dirty, HasUpstream: &upstream, Ahead: &ahead, Behind: &behind, RefreshedAt: &refreshed,
		},
	}
}

func (s *parityWorktreeService) inspection() worktree.Inspection {
	return worktree.Inspection{
		Worktree: s.item, Status: &s.status,
		Repo:          worktree.RepoState{GitBacked: true, GitAvailable: true},
		AgentActivity: worktree.AgentActivityIdle,
	}
}

func (s *parityWorktreeService) CreateAccepted(
	context.Context,
	string,
	worktree.CreateOptions,
) (*worktree.Worktree, error) {
	item := s.item
	item.State = worktree.StatePending
	return &item, nil
}

func (s *parityWorktreeService) CreateReady(
	context.Context,
	string,
	worktree.CreateOptions,
) (*worktree.Worktree, error) {
	item := s.item
	return &item, nil
}

func (s *parityWorktreeService) CancelCreate(_ context.Context, _ string, ref string) error {
	s.recordRef(ref)
	return nil
}

func (s *parityWorktreeService) Adopt(context.Context, string, string, string) (*worktree.Worktree, error) {
	item := s.item
	item.Origin = worktree.OriginAdopted
	return &item, nil
}

func (s *parityWorktreeService) ListDetails(context.Context, string, bool) (*worktree.DetailedListing, error) {
	inspection := s.inspection()
	return &worktree.DetailedListing{
		Worktrees:  []worktree.Inspection{inspection},
		Discovered: []worktree.DiscoveredWorktree{},
		Repo:       worktree.RepoState{GitBacked: true, GitAvailable: true},
	}, nil
}

func (s *parityWorktreeService) Resolve(_ context.Context, _ string, ref string) (*worktree.Worktree, error) {
	s.recordRef(ref)
	item := s.item
	return &item, nil
}

func (s *parityWorktreeService) Inspect(_ context.Context, _ string, ref string) (*worktree.Inspection, error) {
	s.recordRef(ref)
	if s.inspectErr != nil {
		return nil, s.inspectErr
	}
	inspection := s.inspection()
	return &inspection, nil
}

func (s *parityWorktreeService) StatusDetails(
	_ context.Context,
	_ string,
	ref string,
	_ bool,
	_ bool,
) (*worktree.StatusDetails, error) {
	s.recordRef(ref)
	return &worktree.StatusDetails{WorktreeID: s.item.ID, Status: &s.status}, nil
}

func (s *parityWorktreeService) ExitPlan(_ context.Context, _ string, ref string) (*worktree.ExitPlan, error) {
	s.recordRef(ref)
	return &worktree.ExitPlan{
		WorktreeID: s.item.ID, Primary: worktree.ExitActionCommit,
		Actions:     []worktree.ExitActionPlan{{Action: worktree.ExitActionCommit, Label: "Commit", Enabled: true}},
		CommitScope: worktree.ExitCommitScope{ChangedFiles: 1}, Base: "main",
	}, nil
}

func (s *parityWorktreeService) RunExitAction(
	_ context.Context,
	_ string,
	ref string,
	_ worktree.ExitActionRequest,
) (string, error) {
	s.recordRef(ref)
	return "op-parity", nil
}

func (s *parityWorktreeService) CancelExitAction(_ context.Context, _ string, ref string, _ string) error {
	s.recordRef(ref)
	return nil
}

func (s *parityWorktreeService) Remove(
	_ context.Context,
	_ string,
	ref string,
	_ bool,
) (*worktree.RemovalRefusal, error) {
	s.recordRef(ref)
	return s.removal, nil
}

func (s *parityWorktreeService) Dismiss(_ context.Context, _ string, ref string) error {
	s.recordRef(ref)
	return nil
}

func parityWorktreeTime() time.Time {
	return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
}

func worktreeParityErrors() []struct {
	err  error
	code string
} {
	return []struct {
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
}

var _ core.WorktreeService = (*parityWorktreeService)(nil)
