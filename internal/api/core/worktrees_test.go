package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

type worktreeServiceStub struct {
	remove        func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error)
	inspect       func(context.Context, string, string) (*worktree.Inspection, error)
	catalogEvents <-chan worktree.CatalogEvent
}

type worktreeObserverStub struct {
	Observer
	queryEvents func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error)
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

func (s worktreeServiceStub) CancelCreate(context.Context, string, string) error {
	return fmt.Errorf("unexpected CancelCreate call")
}

func (s worktreeServiceStub) Adopt(context.Context, string, string) (*worktree.Worktree, error) {
	return nil, fmt.Errorf("unexpected Adopt call")
}

func (s worktreeServiceStub) ListDetails(context.Context, string, bool) (*worktree.DetailedListing, error) {
	return nil, fmt.Errorf("unexpected ListDetails call")
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
		request := httptest.NewRequest(
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

	t.Run("Should replay durable worktree events in order after the requested sequence", func(t *testing.T) {
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
					worktreeID string,
				) (*worktree.Inspection, error) {
					return &worktree.Inspection{Worktree: worktree.Worktree{
						ID: worktreeID, WorkspaceID: workspaceID,
					}}, nil
				}},
			}
			router := gin.New()
			router.GET("/workspaces/:workspace_id/worktrees/:worktree_id/stream", handlers.StreamWorktree)
			request := httptest.NewRequest(
				http.MethodGet,
				fmt.Sprintf("/workspaces/workspace-a/worktrees/wt-a/stream?after_sequence=%d", afterSequence),
				http.NoBody,
			).WithContext(requestContext)
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
		if !(strings.Index(initial, "id: 1\n") < strings.Index(initial, "id: 2\n") &&
			strings.Index(initial, "id: 2\n") < strings.Index(initial, "id: 3\n")) {
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
		request := httptest.NewRequest(http.MethodGet, "/worktrees/catalog-stream", http.NoBody)
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
