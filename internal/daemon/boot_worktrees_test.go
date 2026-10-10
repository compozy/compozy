package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/core"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

// Canonical suite: daemon translation from resolved workspaces into the worktree domain.
func TestDaemonWorktreeWorkspaceResolver(t *testing.T) {
	t.Parallel()

	homePaths, err := config.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	worktreesRoot := filepath.Join(t.TempDir(), "workspace-worktrees")
	resolvedConfig := config.DefaultWithHome(homePaths)
	resolvedConfig.Worktrees.Root = worktreesRoot
	resolvedConfig.Worktrees.SetupCommand = "bun install"
	resolved := workspacepkg.ResolvedWorkspace{
		ID: "ws-resolved", Name: "Resolved", RootDir: workspaceRoot,
		Config: resolvedConfig,
	}
	stub := &daemonWorktreeResolverStub{
		resolved: resolved,
		listed: []workspacepkg.Workspace{
			resolved.Workspace,
			{ID: "ws-other", Name: "Other", RootDir: t.TempDir()},
		},
	}
	resolver := daemonWorktreeWorkspaceResolver{resolver: stub, homePaths: homePaths}

	got, err := resolver.ResolveWorktreeWorkspace(t.Context(), resolved.ID)
	if err != nil {
		t.Fatalf("ResolveWorktreeWorkspace() error = %v", err)
	}
	if got.ID != resolved.ID || got.Root != workspaceRoot || got.WorktreesRoot != worktreesRoot ||
		got.Worktrees.SetupCommand != "bun install" {
		t.Fatalf("ResolveWorktreeWorkspace() = %#v, want resolved workspace overlay", got)
	}
	listed, err := resolver.ListWorktreeWorkspaces(t.Context())
	if err != nil {
		t.Fatalf("ListWorktreeWorkspaces() error = %v", err)
	}
	if len(listed) != 2 || listed[0].ID != "ws-resolved" || listed[1].ID != "ws-other" {
		t.Fatalf("ListWorktreeWorkspaces() = %#v, want both registered roots", listed)
	}

	empty := daemonWorktreeWorkspaceResolver{homePaths: homePaths}
	if _, err := empty.ResolveWorktreeWorkspace(t.Context(), "missing"); !errors.Is(
		err, worktree.ErrNotFound,
	) {
		t.Fatalf("ResolveWorktreeWorkspace(nil) error = %v, want ErrNotFound", err)
	}
}

func TestDaemonExecutionWorktreesResolveServiceAfterConsumerBoot(t *testing.T) {
	t.Parallel()

	var current executionWorktreeService
	resolver := daemonExecutionWorktrees{lookup: func() executionWorktreeService { return current }}
	if _, err := resolver.MaterializeForRun(
		t.Context(),
		"ws-late",
		worktree.RunWorktreeRequest{TaskSlug: "late", RunID: "run-late"},
	); !errors.Is(err, worktree.ErrPerRunMaterialization) {
		t.Fatalf("MaterializeForRun(before boot) error = %v, want %v", err, worktree.ErrPerRunMaterialization)
	}

	backing := &recordingTaskBridgeWorktrees{materialized: &worktree.Worktree{
		ID: "wt-late", WorkspaceID: "ws-late", RunID: "run-late",
	}}
	current = backing
	item, err := resolver.MaterializeForRun(
		t.Context(),
		"ws-late",
		worktree.RunWorktreeRequest{TaskSlug: "late", RunID: "run-late"},
	)
	if err != nil {
		t.Fatalf("MaterializeForRun(after boot) error = %v", err)
	}
	if item == nil || item.ID != "wt-late" || len(backing.materializeCalls) != 1 {
		t.Fatalf(
			"MaterializeForRun(after boot) = %#v calls=%#v, want late-bound service",
			item,
			backing.materializeCalls,
		)
	}
}

func TestDaemonForgeRuntimeErrorTranslation(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve unavailable extension errors as forge unavailable", func(t *testing.T) {
		t.Parallel()

		err := mapForgeRuntimeError(fmt.Errorf("provider absent: %w", toolspkg.ErrToolUnavailable))
		if !errors.Is(err, worktree.ErrForgeUnavailable) || errors.Is(err, worktree.ErrForge) {
			t.Fatalf("mapForgeRuntimeError(unavailable) = %v, want only ErrForgeUnavailable", err)
		}
	})

	t.Run("Should classify other extension failures as forge errors", func(t *testing.T) {
		t.Parallel()

		err := mapForgeRuntimeError(errors.New("provider protocol failed"))
		if !errors.Is(err, worktree.ErrForge) || errors.Is(err, worktree.ErrForgeUnavailable) {
			t.Fatalf("mapForgeRuntimeError(protocol) = %v, want only ErrForge", err)
		}
	})

	t.Run("Should classify transient provider causes as forge errors", func(t *testing.T) {
		t.Parallel()
		for _, cause := range []string{"rate_limited", "credential_expired"} {
			t.Run("Should classify "+cause, func(t *testing.T) {
				t.Parallel()
				if got := forgeFailureKind(cause); !errors.Is(got, worktree.ErrForge) {
					t.Fatalf("forgeFailureKind(%q) = %v, want ErrForge", cause, got)
				}
			})
		}
	})
}

func TestDaemonSessionWorktreeResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve only ready attached worktrees", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		var gotWorkspace, gotRef string
		resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
			return sessionWorktreeLookupFunc(
				func(_ context.Context, workspaceID, ref string) (*worktree.Worktree, error) {
					gotWorkspace, gotRef = workspaceID, ref
					return &worktree.Worktree{
						ID:          "wt-ready",
						WorkspaceID: workspaceID,
						Path:        root,
						State:       worktree.StateReady,
					}, nil
				},
			)
		}}

		id, gotRoot, err := resolver.ResolveSessionWorktree(t.Context(), " ws-1 ", " wt-ref ")
		if err != nil {
			t.Fatalf("ResolveSessionWorktree(ready) error = %v", err)
		}
		if id != "wt-ready" || gotRoot != root || gotWorkspace != "ws-1" || gotRef != "wt-ref" {
			t.Fatalf(
				"ResolveSessionWorktree(ready) = (%q, %q), lookup (%q, %q)",
				id,
				gotRoot,
				gotWorkspace,
				gotRef,
			)
		}
	})

	for _, test := range []struct {
		name    string
		state   worktree.State
		getErr  error
		path    string
		wantErr error
	}{
		{name: "unknown", getErr: worktree.ErrNotFound, wantErr: worktree.ErrNotFound},
		{name: "pending", state: worktree.StatePending, wantErr: worktree.ErrNotReady},
		{name: "failed", state: worktree.StateFailed, wantErr: worktree.ErrNotReady},
		{name: "removing", state: worktree.StateRemoving, wantErr: worktree.ErrNotReady},
		{name: "missing", state: worktree.StateMissing, wantErr: worktree.ErrMissing},
		{name: "removed", state: worktree.StateRemoved, wantErr: worktree.ErrMissing},
		{name: "ready with vanished path", state: worktree.StateReady, path: filepath.Join(t.TempDir(), "gone"), wantErr: worktree.ErrMissing},
	} {
		t.Run("Should classify "+test.name+" deterministically", func(t *testing.T) {
			t.Parallel()

			resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
				return sessionWorktreeLookupFunc(func(context.Context, string, string) (*worktree.Worktree, error) {
					if test.getErr != nil {
						return nil, test.getErr
					}
					return &worktree.Worktree{ID: "wt-target", Path: test.path, State: test.state}, nil
				})
			}}
			_, _, err := resolver.ResolveSessionWorktree(t.Context(), "ws-1", "target")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ResolveSessionWorktree(%s) error = %v, want %v", test.name, err, test.wantErr)
			}
		})
	}
}

func TestWorktreeTurnRefreshNotifier(t *testing.T) {
	t.Parallel()

	refresher := &recordingTurnWorktreeRefresher{}
	notify := worktreeTurnRefreshNotifier(
		turnWorktreeSessionReader{info: &session.Info{
			ID: "sess-bound", WorkspaceID: "ws-bound", WorktreeID: "wt-bound",
		}},
		refresher,
		nil,
	)
	notify(t.Context(), session.PromptRunIdentity{SessionID: "sess-bound"})
	if refresher.workspaceID != "ws-bound" || refresher.worktreeID != "wt-bound" || !refresher.refresh {
		t.Fatalf(
			"Status() = workspace %q worktree %q refresh %v, want exact bound worktree refresh",
			refresher.workspaceID,
			refresher.worktreeID,
			refresher.refresh,
		)
	}
}

type turnWorktreeSessionReader struct {
	info *session.Info
}

func (r turnWorktreeSessionReader) Status(context.Context, string) (*session.Info, error) {
	return r.info, nil
}

type recordingTurnWorktreeRefresher struct {
	workspaceID string
	worktreeID  string
	refresh     bool
}

func (r *recordingTurnWorktreeRefresher) Status(
	_ context.Context,
	workspaceID string,
	worktreeID string,
	refresh bool,
) (*worktree.Status, error) {
	r.workspaceID = workspaceID
	r.worktreeID = worktreeID
	r.refresh = refresh
	return &worktree.Status{WorktreeID: worktreeID}, nil
}

type sessionWorktreeLookupFunc func(context.Context, string, string) (*worktree.Worktree, error)

func (f sessionWorktreeLookupFunc) Get(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*worktree.Worktree, error) {
	return f(ctx, workspaceID, ref)
}

type daemonWorktreeResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
	listed   []workspacepkg.Workspace
}

func (r *daemonWorktreeResolverStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.resolved, nil
}

func (r *daemonWorktreeResolverStub) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.resolved, nil
}

func (r *daemonWorktreeResolverStub) List(context.Context) ([]workspacepkg.Workspace, error) {
	return append([]workspacepkg.Workspace(nil), r.listed...), nil
}

// Terminal root authority belongs to the daemon's active session/worktree binding.
func TestDaemonTerminalExecutionRoot(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bound", "unbound", "foreign workspace", "foreign profile", "foreign run", "stale generation", "missing worktree", "mismatched worktree", "foreign worktree profile", "inactive run", "missing session", "pending worktree", "backend unavailable"} {
		t.Run("Should resolve or reject "+name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			info := &session.Info{
				ID:                "sess-a",
				WorkspaceID:       "ws-a",
				ProfileID:         "profile-a",
				WorktreeID:        "wt-a",
				RuntimeGeneration: 2,
			}
			active := session.PromptRunIdentity{
				SessionID:   info.ID,
				WorkspaceID: info.WorkspaceID,
				ProfileID:   info.ProfileID,
				RunID:       "run-a",
				Generation:  2,
			}
			actor := terminalpkg.Actor{
				Kind:       terminalpkg.ActorKindAgent,
				ID:         "agent-a",
				SessionID:  info.ID,
				ProfileID:  info.ProfileID,
				RunID:      active.RunID,
				Generation: 2,
			}
			switch name {
			case "unbound":
				info.WorktreeID = ""
			case "foreign workspace":
				active.WorkspaceID = "ws-b"
			case "foreign profile":
				actor.ProfileID = "profile-b"
			case "foreign run":
				actor.RunID = "run-b"
			case "stale generation":
				actor.Generation = 1
			}
			sessions := &fakeSessionManager{
				infos: []*session.Info{info},
				activePromptRunHook: func(context.Context, string) (session.PromptRunIdentity, error) {
					if name == "inactive run" {
						return session.PromptRunIdentity{}, session.ErrPromptNotActive
					}
					return active, nil
				},
			}
			if name == "missing session" {
				sessions.infos = nil
			}
			resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
				return sessionWorktreeLookupFunc(func(_ context.Context, ws, ref string) (*worktree.Worktree, error) {
					if ws != "ws-a" || ref != "wt-a" {
						t.Fatalf("lookup = %q, %q", ws, ref)
					}
					if name == "missing worktree" {
						return nil, worktree.ErrMissing
					}
					id := ref
					if name == "mismatched worktree" {
						id = "wt-b"
					}
					profileID, state := "profile-a", worktree.StateReady
					if name == "foreign worktree profile" {
						profileID = "profile-b"
					}
					if name == "pending worktree" {
						state = worktree.StatePending
					}
					if name == "backend unavailable" {
						return nil, errors.New("private backend detail")
					}
					return &worktree.Worktree{
						ID:          id,
						WorkspaceID: ws,
						ProfileID:   profileID,
						Path:        root,
						State:       state,
					}, nil
				})
			}}
			got, err := resolveTerminalExecutionRoot(t.Context(), sessions, resolver, "ws-a", actor)
			switch name {
			case "bound":
				if err != nil || got != root {
					t.Fatalf("bound root = %q, %v", got, err)
				}
			case "unbound":
				if err != nil || got != "" {
					t.Fatalf("unbound root = %q, %v", got, err)
				}
			default:
				wantErr, wantCode := terminalpkg.ErrNotFound, terminalpkg.ErrorCodeNotFound
				switch name {
				case "stale generation", "inactive run":
					wantErr, wantCode = terminalpkg.ErrGenerationFenced, terminalpkg.ErrorCodeGenerationFenced
				case "missing worktree":
					wantErr, wantCode = worktree.ErrMissing, terminalpkg.ErrorCodeInvalidCwd
				case "pending worktree":
					wantErr, wantCode = worktree.ErrNotReady, terminalpkg.ErrorCodeInvalidCwd
				case "backend unavailable":
					wantErr, wantCode = terminalpkg.ErrServiceUnavailable, ""
				}
				if !errors.Is(err, wantErr) || got != "" {
					t.Fatalf("invalid binding root=%q error=%v, want %v", got, err, wantErr)
				}
				wantStatus := http.StatusNotFound
				switch wantCode {
				case terminalpkg.ErrorCodeGenerationFenced:
					wantStatus = http.StatusConflict
				case terminalpkg.ErrorCodeInvalidCwd:
					wantStatus = http.StatusUnprocessableEntity
				case "":
					wantStatus = http.StatusServiceUnavailable
				}
				status, _, _ := core.TerminalErrorStatus(err)
				if status != wantStatus {
					t.Fatalf("HTTP status = %d, want %d", status, wantStatus)
				}
				nativeErr := terminalToolError(toolspkg.ToolIDTerminalExec, err)
				toolErr, ok := errors.AsType[*toolspkg.ToolError](nativeErr)
				wantNativeCode := toolspkg.ErrorCode(wantCode)
				if wantCode == "" {
					wantNativeCode = toolspkg.ErrorCodeUnavailable
				}
				if !ok || toolErr.Code != wantNativeCode {
					t.Fatalf("native error = %v, want %s", nativeErr, wantNativeCode)
				}
				if wantCode != "" {
					typed, ok := errors.AsType[*terminalpkg.Error](err)
					if !ok || typed.Code != wantCode {
						t.Fatalf("error=%v, want code %s", err, wantCode)
					}
				}
			}
		})
	}
}

// Canonical boot-worktree suite owns deferred journal recovery and cancellation with real SQLite/filesystem.
func TestDaemonManagedDeliveryRecovery(t *testing.T) {
	t.Parallel()
	// Invariant: restored inventory survives SQLite contention without duplicate receipts; shutdown joins retry.
	// Owner: daemon managed-delivery boot lifecycle; existing boot-worktree suite.
	for _, mode := range []string{"restore", "sqlite-busy", "initial-sqlite-busy", "shutdown", "boot-failure"} {
		t.Run("Should own deferred inventory recovery through "+mode, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(t.TempDir(), "compozy.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(testutil.Context(t)); err != nil {
					t.Errorf("close recovery fixture: %v", err)
				}
			})
			now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
			const workspaceID = "ws-inventory-retry"
			if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
				ID: workspaceID, Name: "Inventory retry", RootDir: t.TempDir(), CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				t.Fatal(err)
			}
			item := worktree.Worktree{ID: "wt-inventory-retry", ProfileID: store.DefaultProfileID,
				WorkspaceID: workspaceID, Name: "inventory-retry", Path: t.TempDir(), State: worktree.StateReady,
				Origin: worktree.OriginManual, SetupState: worktree.SetupNone, CreatedAt: now, UpdatedAt: now}
			if err := db.Worktrees.Insert(ctx, item); err != nil {
				t.Fatal(err)
			}
			operation := worktree.ExitOperation{ID: "op-inventory-retry", ProfileID: item.ProfileID,
				WorkspaceID: workspaceID, WorktreeID: item.ID, Action: string(worktree.ExitActionDeliver),
				State: "running", StartedAt: now}
			if err := db.Worktrees.InsertExitOperation(ctx, operation); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			directory := filepath.Join(root, ".delivery")
			if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			restoreInventory := func() {
				if err := os.Remove(directory); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(struct {
					Version     int               `json:"version"`
					OperationID string            `json:"operation_id"`
					Item        worktree.Worktree `json:"worktree"`
					Phase       string            `json:"phase"`
				}{Version: 1, OperationID: operation.ID, Item: item, Phase: "completed"})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "restored.json"), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "initial-sqlite-busy" {
				restoreInventory()
			}
			var events worktree.EventSink = db
			var busy *managedDeliveryBusyEvents
			var unlock func()
			if mode == "sqlite-busy" || mode == "initial-sqlite-busy" {
				busy, unlock = newManagedDeliveryBusyEvents(t, db)
				events = busy
			}
			service := worktree.NewService(db.Worktrees, nil,
				worktree.WithConfig(config.WorktreesConfig{}, root), worktree.WithEvents(events))
			state := &bootState{worktrees: service, logger: slog.Default()}
			cleanup := &bootCleanup{}
			if err := new(Daemon).bootManagedDeliveries(ctx, state, cleanup); err != nil {
				t.Fatalf("transient recovery failure must not abort boot: %v", err)
			}
			worker := state.runtimeWorkers.managedDeliveries
			if worker == nil {
				t.Fatal("deferred recovery lacks boot/shutdown ownership")
			}
			t.Cleanup(func() {
				stopCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
				defer cancel()
				if err := stopManagedDeliveryRecovery(stopCtx, worker); err != nil {
					t.Errorf("stop recovery fixture: %v", err)
				}
			})
			if running, err := db.Worktrees.ListRunningExitOperations(ctx); err != nil || len(running) != 1 ||
				running[0].ID != operation.ID {
				t.Fatalf("transient recovery altered original receipt: %#v error=%v", running, err)
			}
			switch mode {
			case "shutdown":
				stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				var errs []error
				state.runtimeWorkers.shutdown(stopCtx, &errs)
				if err := errors.Join(errs...); err != nil {
					t.Fatal(err)
				}
			case "boot-failure":
				bootErr := errors.New("later boot step failed")
				cleanup.run(ctx, &bootErr)
			}
			if mode != "initial-sqlite-busy" {
				restoreInventory()
			}
			if busy != nil {
				select {
				case err := <-busy.contended:
					if contention, ok := errors.AsType[*store.WriteContentionError](
						err,
					); !ok || !store.IsSQLiteBusy(err) ||
						contention.Attempts < 1 {
						t.Fatalf("expected bounded real receipt SQLite contention: %v", err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("recovery never attempted the contended receipt")
				}
				if running, err := db.Worktrees.ListRunningExitOperations(
					ctx,
				); err != nil || len(running) != 1 ||
					running[0].ID != operation.ID {
					t.Fatalf("contention changed receipt: %#v error=%v", running, err)
				}
				unlock()
			}
			if mode != "restore" && mode != "sqlite-busy" && mode != "initial-sqlite-busy" {
				select {
				case <-worker.Stop():
				default:
					t.Fatal("shutdown returned before retry worker joined")
				}
				if running, err := db.Worktrees.ListRunningExitOperations(ctx); err != nil || len(running) != 1 {
					t.Fatalf("stopped worker recovered after cancellation: %#v error=%v", running, err)
				}
				return
			}
			waitForConditionWithin(t, "same receipt recovery after restored inventory", 5*time.Second, func() bool {
				running, err := db.Worktrees.ListRunningExitOperations(ctx)
				if err != nil {
					t.Error(err)
					return false
				}
				return len(running) == 0
			})
			summaries, err := db.ListEventSummaries(ctx, store.EventSummaryQuery{
				ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspaceID,
				WorktreeID: item.ID, Type: worktree.EventExitActionCompleted,
			})
			if err != nil || len(summaries) != 1 {
				t.Fatalf("restored receipt terminal event=%#v error=%v", summaries, err)
			}
			var payload worktree.ExitEventPayload
			if err := json.Unmarshal(summaries[0].Content, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.OperationID != operation.ID || payload.State != "completed" {
				t.Fatalf("recovery changed receipt identity: %#v", payload)
			}
		})
	}
}

// The real SQLite lock supplies the persistence-boundary failure without replacing recovery.
type managedDeliveryBusyEvents struct {
	*globaldb.GlobalDB
	contended chan error
	attempted bool
}

func (s *managedDeliveryBusyEvents) FinishExitOperationWithEvent(
	ctx context.Context, workspaceID, worktreeID, operationID, state string,
	finishedAt time.Time, event worktree.LifecycleEvent,
) (bool, error) {
	finished, err := s.GlobalDB.FinishExitOperationWithEvent(
		ctx, workspaceID, worktreeID, operationID, state, finishedAt, event,
	)
	if !s.attempted {
		s.attempted = true
		s.contended <- err
	}
	return finished, err
}

func newManagedDeliveryBusyEvents(t *testing.T, db *globaldb.GlobalDB) (*managedDeliveryBusyEvents, func()) {
	t.Helper()
	// Keep the real receipt connection deterministic while bounding only SQLite's per-attempt wait.
	db.DB().SetMaxOpenConns(1)
	db.DB().SetMaxIdleConns(1)
	if _, err := db.DB().ExecContext(t.Context(), "PRAGMA busy_timeout = 1"); err != nil {
		t.Fatal(err)
	}
	locker, err := sql.Open("sqlite", db.Path()+"?_pragma=busy_timeout(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := locker.Close(); err != nil {
			t.Error(err)
		}
	})
	conn, err := locker.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := conn.ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	unlock := sync.OnceFunc(func() {
		if _, err := conn.ExecContext(context.WithoutCancel(t.Context()), "ROLLBACK"); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(unlock)
	return &managedDeliveryBusyEvents{GlobalDB: db, contended: make(chan error, 1)}, unlock
}

// Invariant: the daemon adapter translates provisioning failures and only removes safe, unadmitted work.
// Owner: daemon worktree adapter; canonical worktree wiring suite (UT-038, UT-039).
func TestSubagentWorktreeAdapter(t *testing.T) {
	t.Parallel()
	t.Run("Should rollback failed setup and preserve hook denial identity", func(t *testing.T) {
		t.Parallel()
		service := &subagentWorktreeServiceStub{
			item:   worktree.Worktree{ID: "wt", SetupState: worktree.SetupFailed},
			status: worktree.Status{DirtyFiles: new(0)},
		}
		adapter := daemonSubagentWorktrees{lookup: func() subagentWorktreeService { return service }}
		req := session.SubagentWorktreeRequest{WorkspaceID: "ws", SubagentID: "sub", BaseRef: "main"}
		_, err := adapter.Provision(t.Context(), req)
		failure, ok := errors.AsType[*session.ErrSubagentIsolationFailed](err)
		if !ok || failure.Cause != "setup_failed" || service.rollbacks != 1 {
			t.Fatal(err, service.rollbacks)
		}
		service.provisionErr = worktree.ErrDeniedByHook
		_, err = adapter.Provision(t.Context(), req)
		if !errors.Is(err, worktree.ErrDeniedByHook) || !errors.Is(err, session.ErrSubagentCapabilityDenied) {
			t.Fatal(err)
		}
	})
	for _, tc := range []struct {
		name                string
		dirty, ahead        int
		readError           string
		retained, wantError bool
	}{
		{name: "Should remove a clean checkout with zero commits"},
		{name: "Should retain dirty work", dirty: 1, retained: true},
		{name: "Should retain committed work", ahead: 1, retained: true},
		{name: "Should fail closed when Git cannot be read", readError: "unreadable", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &subagentWorktreeServiceStub{
				status: worktree.Status{DirtyFiles: new(tc.dirty), ReadError: tc.readError},
				ahead:  tc.ahead,
			}
			adapter := daemonSubagentWorktrees{lookup: func() subagentWorktreeService { return svc }}
			retained, err := adapter.SafeRollback(t.Context(), "ws", "wt", "sub", "creation-sha")
			if retained != tc.retained || (err != nil) != tc.wantError {
				t.Fatal(retained, err)
			}
			if (retained || err != nil) && svc.rollbacks != 0 {
				t.Fatal("unsafe rollback", svc.rollbacks)
			}
			if !retained && err == nil && svc.rollbacks != 1 {
				t.Fatal("missing rollback")
			}
		})
	}
	t.Run("Should observe against creation SHA and distinguish unknown from no PR", func(t *testing.T) {
		t.Parallel()
		svc := &subagentWorktreeServiceStub{
			status: worktree.Status{
				HeadSHA:     new("head"),
				DirtyFiles:  new(0),
				RefreshedAt: new(time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)),
			},
			ahead:    2,
			forgeErr: worktree.ErrForgeUnavailable,
		}
		adapter := daemonSubagentWorktrees{lookup: func() subagentWorktreeService { return svc }}
		facts := adapter.Observe(t.Context(), "ws", "wt", "creation-sha")
		if svc.base != "creation-sha" || facts.PRStatus != "unknown" || facts.CommitsAhead == nil ||
			*facts.CommitsAhead != 2 {
			t.Fatal(facts, svc.base)
		}
		svc.forgeErr = nil
		svc.forge = &worktree.ForgeStatus{}
		if facts := adapter.Observe(
			t.Context(),
			"ws",
			"wt",
			"creation-sha",
		); facts.PRStatus != "none" ||
			facts.PRNumber != nil {
			t.Fatal(facts)
		}
		svc.forge = &worktree.ForgeStatus{
			PRNumber: new(7),
			PRState:  new("open"),
			PRURL:    "https://example.test/pull/7",
			Draft:    new(true),
		}
		if facts := adapter.Observe(
			t.Context(),
			"ws",
			"wt",
			"creation-sha",
		); facts.PRStatus != "draft" || facts.PRNumber == nil ||
			*facts.PRNumber != 7 {
			t.Fatal(facts)
		}
		svc.status.ReadError = "failed"
		if facts := adapter.Observe(
			t.Context(),
			"ws",
			"wt",
			"creation-sha",
		); facts.HeadSHA != "" || facts.CommitsAhead != nil ||
			!facts.ObservedAt.IsZero() {
			t.Fatal(facts)
		}
	})
}

type subagentWorktreeServiceStub struct {
	subagentWorktreeService
	item                   worktree.Worktree
	status                 worktree.Status
	forge                  *worktree.ForgeStatus
	provisionErr, forgeErr error
	ahead, rollbacks       int
	base                   string
}

var _ subagentWorktreeService = (*subagentWorktreeServiceStub)(nil)

func (s *subagentWorktreeServiceStub) Get(context.Context, string, string) (*worktree.Worktree, error) {
	return &s.item, nil
}

func (s *subagentWorktreeServiceStub) List(context.Context, string, bool) (*worktree.Listing, error) {
	return &worktree.Listing{}, nil
}

func (s *subagentWorktreeServiceStub) MaterializeForRun(
	context.Context,
	string,
	worktree.RunWorktreeRequest,
) (*worktree.Worktree, error) {
	return &s.item, s.provisionErr
}
func (s *subagentWorktreeServiceStub) RollbackRunMaterialization(context.Context, string, string, string) error {
	s.rollbacks++
	return nil
}
func (s *subagentWorktreeServiceStub) Status(context.Context, string, string, bool) (*worktree.Status, error) {
	return &s.status, nil
}
func (s *subagentWorktreeServiceStub) CommitsAheadOf(_ context.Context, _, _, base string) (int, error) {
	s.base = base
	return s.ahead, nil
}

func (s *subagentWorktreeServiceStub) StatusDetails(
	context.Context,
	string,
	string,
	bool,
	bool,
) (*worktree.StatusDetails, error) {
	return &worktree.StatusDetails{ForgeStatus: s.forge}, s.forgeErr
}
