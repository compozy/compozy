//go:build integration

package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

func TestManagerIntegrationWorktreeBindingLifecycle(t *testing.T) {

	// Invariant: after admission, request cancellation cannot strand the durable
	// location journal. Owner: session lifecycle; this integration suite.
	for _, stopped := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should finish an admitted binding after caller cancellation stopped=%t", stopped),
			func(t *testing.T) {
				fixture := newSessionWorktreeRaceFixture(t, nil)
				h := fixture.h
				h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
				created, err := h.manager.Create(t.Context(), new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID}))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { stopActiveIntegrationSession(t, h, created.ID) })
				if stopped {
					if err := h.manager.Stop(t.Context(), created.ID); err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				h.manager.sessionCatalog = cancelBindingCatalog{GlobalDB: fixture.database, cancel: cancel}
				catalogEvents, closeCatalog, err := h.manager.SubscribeSessionCatalogEvents(t.Context(), CatalogScope{
					ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: h.workspaceID,
				})
				if err != nil {
					t.Fatal(err)
				}
				defer closeCatalog()
				changed, err := h.manager.ChangeWorktreeBinding(ctx, created.ID, fixture.item.ID, new(""))
				if err != nil {
					t.Fatal(err)
				}
				// The explicit wake must survive independent detail reads in clients.
				for found := false; !found; {
					select {
					case event := <-catalogEvents:
						found = event.ExecutionLocationChanged && event.SessionID == created.ID
					default:
						t.Fatal("accepted binding did not publish its location transition")
					}
				}
				meta, err := store.ReadSessionMeta(created.MetaPath())
				if err != nil {
					t.Fatal(err)
				}
				if changed.WorktreeID != fixture.item.ID || meta.WorktreeTransition == nil ||
					meta.WorktreeTransition.Pending {
					t.Fatalf("binding did not converge after cancellation: %#v", meta.WorktreeTransition)
				}
				events, err := h.manager.Prompt(t.Context(), created.ID, "Continue after the location change")
				if err != nil {
					t.Fatal(err)
				}
				collectEvents(t, events)
			},
		)
	}

	// Invariant: governed and archived conversations cannot change execution
	// ownership. Owner: session lifecycle; canonical binding integration suite.
	for _, kind := range []Type{SessionTypeSystem, SessionTypeCoordinator, SessionTypeSpawned} {
		t.Run("Should refuse a governed "+string(kind)+" session", func(t *testing.T) {
			fixture := newSessionWorktreeRaceFixture(t, nil)
			h := fixture.h
			h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
			opts := CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Type: kind}
			if kind == SessionTypeCoordinator || kind == SessionTypeSpawned {
				opts.Lineage = &store.SessionLineage{TTLExpiresAt: new(time.Now().Add(time.Hour))}
			}
			if kind == SessionTypeSpawned {
				parent, err := h.manager.Create(t.Context(), new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID}))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { stopActiveIntegrationSession(t, h, parent.ID) })
				opts.Lineage.ParentSessionID = parent.ID
				opts.Lineage.RootSessionID = parent.ID
				opts.Lineage.SpawnDepth = 1
			}
			created, err := h.manager.Create(t.Context(), &(opts))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { stopActiveIntegrationSession(t, h, created.ID) })
			before := created.Meta()
			for _, stopped := range []bool{false, true} {
				if stopped {
					if err := h.manager.Stop(t.Context(), created.ID); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := h.manager.ChangeWorktreeBinding(
					t.Context(),
					created.ID,
					fixture.item.ID,
					new(""),
				); !errors.Is(
					err,
					ErrValidation,
				) {
					t.Fatalf("governed binding error = %v (stopped=%t)", err, stopped)
				}
				meta, err := store.ReadSessionMeta(created.MetaPath())
				if err != nil {
					t.Fatal(err)
				}
				if meta.WorktreeIDValue() != before.WorktreeIDValue() || meta.CWDValue() != before.CWDValue() ||
					meta.WorktreeTransition != nil {
					t.Fatal("governed refusal changed the durable execution location")
				}
			}
		})
	}
	t.Run("Should preserve an archived session location", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)
		h := fixture.h
		h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
		created, err := h.manager.Create(t.Context(), new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID}))
		if err != nil {
			t.Fatal(err)
		}
		before := created.Meta()
		if _, err := h.manager.ArchiveWithStop(t.Context(), h.workspaceID, created.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.ChangeWorktreeBinding(
			t.Context(),
			created.ID,
			fixture.item.ID,
			new(""),
		); !errors.Is(
			err,
			ErrSessionArchived,
		) {
			t.Fatalf("archived binding error = %v", err)
		}
		meta, err := store.ReadSessionMeta(created.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		if meta.WorktreeIDValue() != before.WorktreeIDValue() || meta.CWDValue() != before.CWDValue() ||
			meta.WorktreeTransition != nil {
			t.Fatal("archived refusal changed the durable execution location")
		}
	})
	t.Run("Should preserve metadata when the catalog refuses a binding", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)
		h := fixture.h
		h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
		ctx := t.Context()
		created, err := h.manager.Create(ctx, new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, created.ID) })
		before := created.Meta()
		if err := fixture.database.UpdateSessionWorktreeBinding(ctx, store.SessionWorktreeBindingUpdate{
			SessionID: created.ID, WorkspaceID: h.workspaceID, ProfileID: before.ProfileID,
			WorktreeID: fixture.item.ID, UpdatedAt: before.UpdatedAt,
		}); err != nil {
			t.Fatal(err)
		}
		_, err = h.manager.ChangeWorktreeBinding(ctx, created.ID, fixture.item.ID, new(""))
		if !errors.Is(err, store.ErrSessionWorktreeConflict) {
			t.Fatalf("binding error = %v", err)
		}
		durable, err := store.ReadSessionMeta(created.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		for _, actual := range []store.SessionMeta{created.Meta(), durable} {
			if actual.WorktreeIDValue() != before.WorktreeIDValue() || actual.CWDValue() != before.CWDValue() ||
				actual.WorktreeTransition != nil {
				t.Fatalf("refused binding changed location: %#v", actual.SessionExecutionLocationState)
			}
		}
	})
	// Invariant: changing environments preserves conversation identity and restarts
	// execution in the selected root. Owner: session lifecycle, canonical suite.
	t.Run("Should change an idle session environment in place", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)
		h := fixture.h
		h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
		ctx := testutil.Context(t)
		subdirectory := filepath.Join(h.workspace, "nested")
		if err := os.Mkdir(subdirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		created, err := h.manager.Create(
			ctx,
			new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID, CWD: subdirectory}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, created.ID) })
		// Admission must refuse an occupied prompt slot without mutating identity.
		if _, err := created.beginExclusivePromptSetup(); err != nil {
			t.Fatal(err)
		}
		_, bindErr := h.manager.ChangeWorktreeBinding(ctx, created.ID, fixture.item.ID, new(""))
		created.finishPromptSetup()
		if !errors.Is(bindErr, ErrPromptInProgress) || created.Info().WorktreeID != "" {
			t.Fatalf("busy bind=%v", bindErr)
		}
		_, bindErr = h.manager.ChangeWorktreeBinding(ctx, created.ID, fixture.item.ID, new("wt-stale"))
		if !errors.Is(bindErr, store.ErrSessionWorktreeConflict) {
			t.Fatalf("stale bind=%v", bindErr)
		}
		first, err := h.manager.Prompt(ctx, created.ID, "Remember the unique fact: blue kiwi")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, first)
		originalACP := created.Info().ACPSessionID
		// Published state must already be eligible for the next operator action.
		h.manager.AddWorktreeBindingNotifier(func(ctx context.Context, info *Info, _ string) {
			if available := h.manager.WorktreeBindingAvailability(ctx, info.ID); !available.Available {
				t.Errorf("published binding remains reserved: %s", available.UnavailableReason)
			}
		})
		for _, destination := range []string{"", fixture.item.ID, ""} {
			expected := created.Info().WorktreeID
			changed, err := h.manager.ChangeWorktreeBinding(ctx, created.ID, destination, &expected)
			if err != nil {
				t.Fatal(err)
			}
			if changed.ID != created.ID || changed.WorktreeID != destination || changed.ACPSessionID != originalACP {
				t.Fatalf("changed identity = %#v", changed)
			}
			if destination != "" {
				releaseDelivery, err := h.manager.AcquireWorktreeDeliveryFence(
					ctx,
					h.workspaceID,
					destination,
					created.ID,
				)
				if err != nil {
					t.Fatal(err)
				}
				_, bindErr := h.manager.ChangeWorktreeBinding(ctx, created.ID, "", &destination)
				releaseDelivery()
				if !errors.Is(bindErr, worktreepkg.ErrOperationInProgress) {
					t.Fatalf("delivery unbind=%v", bindErr)
				}
			}
			root := h.workspace
			if destination != "" {
				root = fixture.item.Path
			}
			root = resolveIntegrationWorkspaceRoot(t, root)
			if got, err := h.manager.WorkingDir(ctx, created.ID); err != nil || got != root {
				t.Fatalf("WorkingDir = %q, %v; want %q", got, err, root)
			}
			if created.runtimeBindingSnapshot().process != nil {
				t.Fatal("old process remains bound")
			}
			next, err := h.manager.Prompt(ctx, created.ID, "Recall the unique fact")
			if err != nil {
				t.Fatal(err)
			}
			collectEvents(t, next)
			h.driver.mu.Lock()
			start := h.driver.startCalls[len(h.driver.startCalls)-1]
			stops := h.driver.stopCalls
			h.driver.mu.Unlock()
			if start.Cwd != root || stops == 0 {
				t.Fatalf("runtime start=%#v stops=%d", start, stops)
			}
			meta, err := store.ReadSessionMeta(created.MetaPath())
			if err != nil {
				t.Fatal(err)
			}
			if meta.WorktreeTransition == nil || meta.WorktreeTransition.Pending ||
				meta.WorktreeTransition.NoticePending {
				t.Fatalf("unfinished transition after prompt: %#v", meta.WorktreeTransition)
			}
		}
		rows, err := fixture.database.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: h.workspaceID,
		})
		if err != nil || len(rows) != 1 || rows[0].ID != created.ID {
			t.Fatalf("catalog=%#v error=%v", rows, err)
		}
	})
	// Invariant: an accepted binding publishes its location wake even if the
	// post-commit status projection is unavailable. Owner: session lifecycle.
	t.Run("Should publish accepted binding after a failed status read", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)
		h := fixture.h
		h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
		ctx := testutil.Context(t)
		created, err := h.manager.Create(ctx, new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID}))
		if err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Stop(ctx, created.ID); err != nil {
			t.Fatal(err)
		}
		path := created.MetaPath()
		h.manager.AddWorktreeBindingNotifier(func(ctx context.Context, _ *Info, _ string) {
			if err := os.Rename(path, path+".unavailable"); err != nil {
				t.Fatal(err)
			}
			if _, err := h.manager.Status(ctx, created.ID); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("status failure = %v", err)
			}
		})
		events, unsubscribe, err := h.manager.SubscribeSessionCatalogEvents(ctx, CatalogScope{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: h.workspaceID,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer unsubscribe()
		changed, bindErr := h.manager.ChangeWorktreeBinding(ctx, created.ID, fixture.item.ID, new(""))
		if err := os.Rename(path+".unavailable", path); err != nil {
			t.Fatal(err)
		}
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		for {
			select {
			case event := <-events:
				if event.SessionID == changed.ID && event.ExecutionLocationChanged {
					return
				}
			default:
				t.Fatal("accepted binding did not publish its location change")
			}
		}
	})
	// Invariant: stopped sessions can leave unavailable checkouts without starting
	// a provider. Owner: session lifecycle; same canonical integration suite.
	t.Run("Should bind stopped sessions and leave a removed checkout", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)
		h := fixture.h
		h.manager.SetWorkSignalSources(WorkSignalSource{Kind: WorkSignalHeldPermission, Source: SignalSourceFunc(h.manager.heldPermissionSignals)})
		h.manager.worktreeResolver = sessionBindingIntegrationResolver{service: fixture.service}
		ctx := testutil.Context(t)
		created, err := h.manager.Create(ctx, new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID}))
		if err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Stop(ctx, created.ID); err != nil {
			t.Fatal(err)
		}
		// Invariant: publication follows reservation release for stopped sessions too.
		h.manager.AddWorktreeBindingNotifier(func(_ context.Context, info *Info, _ string) {
			if _, err := h.manager.ChangeWorktreeBinding(
				t.Context(),
				info.ID,
				info.WorktreeID,
				&info.WorktreeID,
			); err != nil {
				t.Errorf("published stopped binding remains reserved: %v", err)
			}
		})
		changed, err := h.manager.ChangeWorktreeBinding(ctx, created.ID, fixture.item.ID, new(""))
		if err != nil {
			t.Fatal(err)
		}
		if changed.State != StateStopped || changed.ID != created.ID {
			t.Fatalf("changed=%#v", changed)
		}
		h.driver.mu.Lock()
		starts := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if starts != 1 {
			t.Fatalf("binding started a provider: %d starts", starts)
		}
		events, err := h.manager.Prompt(ctx, created.ID, "Continue in the worktree")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, events)
		h.driver.mu.Lock()
		cwd := h.driver.startCalls[len(h.driver.startCalls)-1].Cwd
		h.driver.mu.Unlock()
		if cwd != resolveIntegrationWorkspaceRoot(t, fixture.item.Path) {
			t.Fatalf("resume cwd=%q", cwd)
		}
		if err := h.manager.Stop(ctx, created.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.service.Remove(ctx, h.workspaceID, fixture.item.ID, true); err != nil {
			t.Fatal(err)
		}
		changed, err = h.manager.ChangeWorktreeBinding(ctx, created.ID, "", &fixture.item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if changed.State != StateStopped || changed.WorktreeID != "" {
			t.Fatalf("unbound=%#v", changed)
		}
	})

	// Invariant: an exact bound caller can hand off delivery while competing
	// starts/resumes remain fenced. Owner: session lifecycle; canonical suite.
	t.Run("Should fence only the bound managed delivery checkout", func(t *testing.T) {
		h := newHarness(t)
		h.manager = newManagerWithHarness(
			t,
			h,
			WithWorktreeResolver(
				&fakeSessionWorktreeResolver{
					resolve: func(_ context.Context, _ string, ref string) (string, string, error) { return ref, h.workspace, nil },
				},
			),
		)
		caller, err := h.manager.Create(
			testutil.Context(t),
			new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-delivery"}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, caller.ID) })
		competitor, err := h.manager.Create(
			testutil.Context(t),
			new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-delivery"}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, competitor.ID) })
		if release, err := h.manager.AcquireWorktreeDeliveryFence(
			testutil.Context(t),
			h.workspaceID,
			"wt-delivery",
			caller.ID,
		); err == nil {
			release()
			t.Fatal("delivery admitted another active bound session")
		}
		if err := h.manager.Stop(testutil.Context(t), competitor.ID); err != nil {
			t.Fatal(err)
		}
		release, err := h.manager.AcquireWorktreeDeliveryFence(
			testutil.Context(t),
			h.workspaceID,
			"wt-delivery",
			caller.ID,
		)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		if _, err := h.manager.Create(
			testutil.Context(t),
			new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-delivery"}),
		); !errors.Is(
			err,
			ErrValidation,
		) {
			t.Fatalf("competing start error=%v", err)
		}
		unrelated, err := h.manager.Create(
			testutil.Context(t),
			new(CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-other"}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, unrelated.ID) })
		if err := h.manager.Stop(testutil.Context(t), caller.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.Resume(testutil.Context(t), caller.ID); !errors.Is(err, ErrValidation) {
			t.Fatalf("resume under delivery fence=%v", err)
		}
		release()
		resumed, err := h.manager.Resume(testutil.Context(t), caller.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, resumed.ID) })
	})

	t.Run("Should bind lifecycle operations to the selected worktree", func(t *testing.T) {
		h := newHarness(t)
		initializeSessionIntegrationGitRepository(t, h.workspace)
		worktreesRoot := t.TempDir()
		worktreeA := filepath.Join(worktreesRoot, "worktree-a")
		worktreeB := filepath.Join(worktreesRoot, "worktree-b")
		runSessionIntegrationGit(t, h.workspace, "branch", "worktree-a", "main")
		runSessionIntegrationGit(t, h.workspace, "branch", "worktree-b", "main")
		runSessionIntegrationGit(t, h.workspace, "worktree", "add", worktreeA, "worktree-a")
		runSessionIntegrationGit(t, h.workspace, "worktree", "add", worktreeB, "worktree-b")

		missingErr := errors.New("worktree_missing")
		worktreeResolver := &fakeSessionWorktreeResolver{resolve: func(
			_ context.Context,
			_ string,
			ref string,
		) (string, string, error) {
			roots := map[string]string{"wt-a": worktreeA, "wt-b": worktreeB}
			root := roots[strings.TrimSpace(ref)]
			if root == "" {
				return "", "", errors.New("worktree_not_found")
			}
			if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
				return "", "", missingErr
			} else if err != nil {
				return "", "", err
			}
			return strings.TrimSpace(ref), root, nil
		}}
		var contextsMu sync.Mutex
		contexts := make([]StartupPromptContext, 0, 5)
		h.manager = newManagerWithHarness(
			t,
			h,
			WithWorktreeResolver(worktreeResolver),
			WithPromptAssembler(startupPromptAssemblerFunc(func(
				_ context.Context,
				startup StartupPromptContext,
				agent compozyconfig.AgentDef,
				_ *workspacepkg.ResolvedWorkspace,
			) (string, error) {
				contextsMu.Lock()
				contexts = append(contexts, startup)
				contextsMu.Unlock()
				return agent.Prompt, nil
			})),
		)

		parent, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-a",
			Lineage: &store.SessionLineage{
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
			},
		}))
		if err != nil {
			t.Fatalf("Create(bound parent) error = %v", err)
		}
		sameWorktree, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-a",
		}))
		if err != nil {
			t.Fatalf("Create(second same-worktree session) error = %v", err)
		}
		otherWorktree, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-b",
		}))
		if err != nil {
			t.Fatalf("Create(other-worktree session) error = %v", err)
		}
		rootSession, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID,
		}))
		if err != nil {
			t.Fatalf("Create(root session) error = %v", err)
		}
		for _, item := range []*Session{parent, sameWorktree, otherWorktree, rootSession} {
			t.Cleanup(func() { stopActiveIntegrationSession(t, h, item.ID) })
		}

		canonicalA := resolveIntegrationWorkspaceRoot(t, worktreeA)
		canonicalB := resolveIntegrationWorkspaceRoot(t, worktreeB)
		if got := h.driver.startCalls[0].Cwd; got != canonicalA {
			t.Fatalf("parent cmd.Dir = %q, want %q", got, canonicalA)
		}
		if got := h.driver.startCalls[1].Cwd; got != canonicalA {
			t.Fatalf("second cmd.Dir = %q, want %q", got, canonicalA)
		}
		if got := h.driver.startCalls[2].Cwd; got != canonicalB {
			t.Fatalf("other cmd.Dir = %q, want %q", got, canonicalB)
		}
		if err := os.WriteFile(
			filepath.Join(h.driver.startCalls[0].Cwd, "isolated.txt"),
			[]byte("worktree-a\n"),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(worktree A) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(h.driver.startCalls[2].Cwd, "isolated.txt"),
			[]byte("worktree-b\n"),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(worktree B) error = %v", err)
		}
		assertIntegrationFileContent(t, filepath.Join(worktreeA, "isolated.txt"), "worktree-a\n")
		assertIntegrationFileContent(t, filepath.Join(worktreeB, "isolated.txt"), "worktree-b\n")

		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn(bound child) error = %v", err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, child.ID) })
		if child.Info().WorktreeID != "wt-a" || h.driver.startCalls[len(h.driver.startCalls)-1].Cwd != canonicalA {
			t.Fatalf("bound child = %#v start=%#v, want wt-a at %q", child.Info(), h.driver.startCalls, canonicalA)
		}

		contextsMu.Lock()
		contextsSnapshot := append([]StartupPromptContext(nil), contexts...)
		contextsMu.Unlock()
		if len(contextsSnapshot) < 5 {
			t.Fatalf("startup contexts = %#v, want bound, root, and child contexts", contextsSnapshot)
		}
		for _, index := range []int{0, 1, 2, 4} {
			if contextsSnapshot[index].WorkspaceID != h.workspaceID ||
				contextsSnapshot[index].Workspace != h.workspace {
				t.Fatalf("startup context[%d] = %#v, want parent workspace", index, contextsSnapshot[index])
			}
		}
		if contextsSnapshot[0].WorktreeID != "wt-a" || contextsSnapshot[2].WorktreeID != "wt-b" ||
			contextsSnapshot[3].WorktreeID != "" {
			t.Fatalf("startup worktree contexts = %#v", contextsSnapshot)
		}

		if parent.Info().WorkspaceID != h.workspaceID || rootSession.Info().WorkspaceID != h.workspaceID {
			t.Fatalf(
				"session workspace ids = %q/%q, want %q",
				parent.Info().WorkspaceID,
				rootSession.Info().WorkspaceID,
				h.workspaceID,
			)
		}

		runSessionIntegrationGit(t, h.workspace, "worktree", "remove", "--force", worktreeA)
		startsBeforeRefusal := len(h.driver.startCalls)
		_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Minute,
		})
		if !errors.Is(err, missingErr) {
			t.Fatalf("Spawn(missing worktree) error = %v, want %v", err, missingErr)
		}
		if len(h.driver.startCalls) != startsBeforeRefusal {
			t.Fatalf("driver starts after missing spawn = %d, want %d", len(h.driver.startCalls), startsBeforeRefusal)
		}
		if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
			t.Fatalf("Stop(bound parent) error = %v", err)
		}
		_, err = h.manager.Resume(testutil.Context(t), parent.ID)
		if !errors.Is(err, missingErr) {
			t.Fatalf("Resume(missing worktree) error = %v, want %v", err, missingErr)
		}
	})
}

func TestManagerIntegrationWorktreeRemovalBindingRace(t *testing.T) {
	t.Run("Should let an active session binding win before removal", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)

		bound, err := fixture.h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: fixture.h.workspaceID, Worktree: fixture.item.ID,
		}))
		if err != nil {
			t.Fatalf("Create(bound winner) error = %v", err)
		}
		if got := bound.Info().WorktreeID; got != fixture.item.ID {
			t.Fatalf("bound winner worktree_id = %q, want %q", got, fixture.item.ID)
		}

		if _, err := fixture.service.Remove(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID, false,
		); !errors.Is(err, worktreepkg.ErrSessionActive) {
			t.Fatalf("Remove(after binding) error = %v, want %v", err, worktreepkg.ErrSessionActive)
		}
		stored, err := fixture.database.Worktrees.Get(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID,
		)
		if err != nil || stored == nil || stored.State != worktreepkg.StateReady {
			t.Fatalf("worktree after binding winner = %#v, %v, want ready", stored, err)
		}

		if err := fixture.h.manager.Stop(testutil.Context(t), bound.ID); err != nil {
			t.Fatalf("Stop(bound winner) error = %v", err)
		}
		if _, err := fixture.service.Remove(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID, true,
		); err != nil {
			t.Fatalf("Remove(cleanup) error = %v", err)
		}
	})

	t.Run("Should let the removal fence reject a stale new session binding", func(t *testing.T) {
		hook := newBlockingSessionWorktreeRemovalHook()
		fixture := newSessionWorktreeRaceFixture(t, hook)
		removeDone := make(chan error, 1)
		go func() {
			_, err := fixture.service.Remove(
				t.Context(), fixture.h.workspaceID, fixture.item.ID, false,
			)
			removeDone <- err
		}()

		select {
		case <-hook.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for removal fence")
		}

		bound, err := fixture.h.manager.Create(testutil.Context(t), new(CreateOpts{
			AgentName: "coder", Workspace: fixture.h.workspaceID, Worktree: fixture.item.ID,
		}))
		if bound != nil || !errors.Is(err, worktreepkg.ErrNotReady) {
			t.Fatalf("Create(after removal fence) = (%#v, %v), want nil and %v", bound, err, worktreepkg.ErrNotReady)
		}
		if got := len(fixture.h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls after rejected binding = %d, want 0", got)
		}
		sessions, listErr := fixture.database.ListSessions(testutil.Context(t), store.SessionListQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: fixture.h.workspaceID,
			WorktreeID:  fixture.item.ID,
		})
		if listErr != nil {
			t.Fatalf("ListSessions(rejected binding) error = %v", listErr)
		}
		if len(sessions) != 0 {
			t.Fatalf("sessions after rejected binding = %#v, want none", sessions)
		}

		close(hook.release)
		select {
		case removeErr := <-removeDone:
			if removeErr != nil {
				t.Fatalf("Remove(fenced winner) error = %v", removeErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for fenced removal")
		}
		stored, getErr := fixture.database.Worktrees.Get(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID,
		)
		if getErr != nil || stored == nil || stored.State != worktreepkg.StateRemoved {
			t.Fatalf("worktree after removal winner = %#v, %v, want removed", stored, getErr)
		}
	})
}

type sessionWorktreeRaceFixture struct {
	h        *harness
	database *globaldb.GlobalDB
	service  *worktreepkg.Service
	item     *worktreepkg.Worktree
}

func newSessionWorktreeRaceFixture(
	t *testing.T,
	hooks worktreepkg.HookDispatcher,
) *sessionWorktreeRaceFixture {
	t.Helper()
	ctx := testutil.Context(t)
	h := newHarness(t)
	if err := h.manager.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown(initial manager) error = %v", err)
	}
	initializeSessionIntegrationGitRepository(t, h.workspace)

	database, err := openSessionTestGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(testutil.Context(t)); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	})
	if err := database.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: h.workspaceID, Name: h.workspaceName, RootDir: h.workspace,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	runner, err := worktreepkg.NewRealGitRunner(10 * time.Second)
	if err != nil {
		t.Fatalf("NewRealGitRunner() error = %v", err)
	}
	settings := compozyconfig.DefaultWorktreesConfig()
	service := worktreepkg.NewService(
		database.Worktrees,
		runner,
		worktreepkg.WithWorkspaceResolver(sessionWorktreeWorkspaceResolver{workspace: worktreepkg.Workspace{
			ID: h.workspaceID, Name: h.workspaceName, Root: h.workspace,
		}}),
		worktreepkg.WithSessionGuard(sessionWorktreeGlobalDBGuard{database: database}),
		worktreepkg.WithConfig(settings, t.TempDir()),
		worktreepkg.WithHooks(hooks),
		worktreepkg.WithIDGenerator(func(string) (string, error) { return "wt-session-race", nil }),
	)
	item, err := service.Create(ctx, h.workspaceID, worktreepkg.CreateOptions{
		ProfileID: store.DefaultProfileID,
		Name:      "Session Race",
	})
	if err != nil {
		t.Fatalf("worktree Create() error = %v", err)
	}
	resolver := &fakeSessionWorktreeResolver{resolve: func(
		_ context.Context,
		_ string,
		_ string,
	) (string, string, error) {
		// Deliberately return the admitted path even after the SQL removal fence.
		// The catalog must still reject the stale new binding atomically.
		return item.ID, item.Path, nil
	}}
	h.manager = newManagerWithHarness(
		t,
		h,
		WithSessionCatalog(database),
		WithWorktreeResolver(resolver),
	)
	cleanupTestManager(t, h.manager)

	return &sessionWorktreeRaceFixture{h: h, database: database, service: service, item: item}
}

type sessionWorktreeWorkspaceResolver struct {
	workspace worktreepkg.Workspace
}

func (r sessionWorktreeWorkspaceResolver) ResolveWorktreeWorkspace(
	_ context.Context,
	workspaceID string,
	_ string,
) (worktreepkg.Workspace, error) {
	if workspaceID != r.workspace.ID {
		return worktreepkg.Workspace{}, worktreepkg.ErrNotFound
	}
	return r.workspace, nil
}

func (r sessionWorktreeWorkspaceResolver) ListWorktreeWorkspaces(
	context.Context,
) ([]worktreepkg.Workspace, error) {
	return []worktreepkg.Workspace{r.workspace}, nil
}

type sessionWorktreeGlobalDBGuard struct {
	database *globaldb.GlobalDB
}

func (g sessionWorktreeGlobalDBGuard) HasActiveSession(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
) (bool, error) {
	return g.database.HasActiveWorktreeSession(ctx, workspaceID, worktreeID)
}

type blockingSessionWorktreeRemovalHook struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingSessionWorktreeRemovalHook() *blockingSessionWorktreeRemovalHook {
	return &blockingSessionWorktreeRemovalHook{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *blockingSessionWorktreeRemovalHook) DispatchWorktreeHook(
	ctx context.Context,
	request worktreepkg.HookRequest,
) (worktreepkg.HookVerdict, error) {
	if request.Event != worktreepkg.EventPreRemove {
		return worktreepkg.HookVerdict{}, nil
	}
	h.once.Do(func() { close(h.entered) })
	select {
	case <-h.release:
		return worktreepkg.HookVerdict{}, nil
	case <-ctx.Done():
		return worktreepkg.HookVerdict{}, ctx.Err()
	}
}

func initializeSessionIntegrationGitRepository(t *testing.T, root string) {
	t.Helper()
	runSessionIntegrationGit(t, root, "init", "-b", "main")
	runSessionIntegrationGit(t, root, "config", "user.name", "Compozy Integration")
	runSessionIntegrationGit(t, root, "config", "user.email", "integration@compozy.test")
	if err := os.WriteFile(filepath.Join(root, "shared.txt"), []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(shared.txt) error = %v", err)
	}
	runSessionIntegrationGit(t, root, "add", "shared.txt")
	runSessionIntegrationGit(t, root, "commit", "-m", "initial")
}

func runSessionIntegrationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s error = %v; output=%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func stopActiveIntegrationSession(t *testing.T, h *harness, sessionID string) {
	t.Helper()
	current, ok := h.manager.Get(sessionID)
	if !ok || current.Info().State == StateStopped {
		return
	}
	if err := h.manager.Stop(testutil.Context(t), sessionID); err != nil {
		t.Errorf("Stop(%q) cleanup error = %v", sessionID, err)
	}
}

func assertIntegrationFileContent(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, content, want)
	}
}

func TestManagerIntegrationFullLifecycle(t *testing.T) {
	h := newHarness(t)

	sessionCWD := filepath.Join(h.workspace, "nested-session-cwd")
	if err := os.MkdirAll(sessionCWD, 0o755); err != nil {
		t.Fatalf("MkdirAll(session CWD) error = %v", err)
	}
	canonicalSessionCWD := resolveIntegrationWorkspaceRoot(t, sessionCWD)
	session, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
		AgentName: "coder",
		Name:      "session",
		Workspace: h.workspaceID,
		CWD:       sessionCWD,
	}))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := h.driver.startCalls[0].Cwd; got != canonicalSessionCWD {
		t.Fatalf("Create() CWD = %q, want %q", got, canonicalSessionCWD)
	}
	recorder, ok := session.recorderHandle().(*sessiondb.SessionDB)
	if !ok {
		t.Fatalf("recorder = %T, want *sessiondb.SessionDB", session.recorderHandle())
	}
	if got, want := recorder.Path(), session.DBPath(); got != want {
		t.Fatalf("SessionDB.Path() = %q, want %q", got, want)
	}

	firstPrompt, err := h.manager.Prompt(testutil.Context(t), session.ID, "first")
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	firstEvents := collectEvents(t, firstPrompt)
	if len(firstEvents) != 2 {
		t.Fatalf("first prompt events = %d, want 2", len(firstEvents))
	}
	acpSessionIDBeforeStop := session.Info().ACPSessionID
	if strings.TrimSpace(acpSessionIDBeforeStop) == "" {
		t.Fatal("ACP session ID before stop is empty")
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	secondPrompt, err := h.manager.Prompt(testutil.Context(t), session.ID, "second")
	if err != nil {
		t.Fatalf("Prompt(stopped, second) error = %v", err)
	}
	secondEvents := collectEvents(t, secondPrompt)
	if len(secondEvents) != 2 {
		t.Fatalf("second prompt events = %d, want 2", len(secondEvents))
	}
	resumed, ok := h.manager.Get(session.ID)
	if !ok {
		t.Fatalf("Get(%q) found = false, want resumed session", session.ID)
	}
	h.driver.mu.Lock()
	resumeStart := h.driver.startCalls[1]
	h.driver.mu.Unlock()
	if got := resumeStart.Cwd; got != canonicalSessionCWD {
		t.Fatalf("prompt resume CWD = %q, want persisted %q", got, canonicalSessionCWD)
	}
	if got := resumeStart.ResumeSessionID; got != acpSessionIDBeforeStop {
		t.Fatalf("prompt resume ACP session ID = %q, want persisted %q", got, acpSessionIDBeforeStop)
	}

	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("final Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(resumed.ID, resumed.WorkspaceID),
		resumed.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if len(events) != 12 {
		t.Fatalf("stored events = %d, want 12", len(events))
	}
	if got := countEventType(events, eventspkg.SessionStopEscalated); got != 2 {
		t.Fatalf("stored stop escalation events = %d, want 2", got)
	}
	if !containsEventType(events, acp.EventTypeAgentMessage) || !containsEventType(events, acp.EventTypeDone) {
		t.Fatalf("stored events missing expected types: %#v", events)
	}
	if got := countEventType(events, EventTypeSessionStopped); got != 2 {
		t.Fatalf("stored %q events = %d, want 2", EventTypeSessionStopped, got)
	}
	if got := countEventType(events, eventspkg.TranscriptMarkerCreated); got != 2 {
		t.Fatalf("stored %q events = %d, want 2", eventspkg.TranscriptMarkerCreated, got)
	}

	meta := readMeta(t, resumed.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
}

func TestManagerIntegrationSyntheticPromptPersistsDedicatedEventsWithMixedHistory(t *testing.T) {
	h := newHarness(t)

	session := createSession(t, h)
	enableSyntheticQueue(t, h, session)
	userEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
	if err != nil {
		t.Fatalf("Prompt(user) error = %v", err)
	}
	_ = collectEvents(t, userEvents)

	syntheticEvents, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
			Reason:    "task_run_completed",
			Summary:   "background task finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, syntheticEvents)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(session.ID, session.WorkspaceID),
		session.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if got := countEventType(events, acp.EventTypeUserMessage); got != 1 {
		t.Fatalf("countEventType(user_message) = %d, want 1 user input", got)
	}
	if got := countEventType(events, acp.EventTypeSyntheticReentry); got != 1 {
		t.Fatalf("countEventType(synthetic_reentry) = %d, want 1", got)
	}
	if !containsEventType(events, acp.EventTypeAgentMessage) || !containsEventType(events, acp.EventTypeDone) {
		t.Fatalf("mixed history missing runtime events: %#v", events)
	}
}

func TestManagerIntegrationSyntheticQueueSurvivesRestart(t *testing.T) {
	t.Run(
		"Should resume accepted synthetic input once with its metadata after the manager and database restart",
		func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			h := newHarness(t)
			sess := createSession(t, h)
			enableSyntheticQueue(t, h, sess)
			database, ok := h.manager.inputQueueStore.(*globaldb.GlobalDB)
			if !ok {
				t.Fatal("expected durable queue fixture")
			}
			if _, _, err := database.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: "cleared-before-restart", SessionID: sess.ID, Text: "removed before crash", QueueCap: 3,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := database.ClearSessionInputs(ctx, store.SessionInputClearRequest{
				SessionID: sess.ID, TurnID: "clear-before-restart", ActorKind: "human", ActorID: "restarting-operator",
			}); err != nil {
				t.Fatal(err)
			}
			release := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			h.driver.cancelHook = func(*fakeProcess) error {
				releaseOnce.Do(func() { close(release) })
				return nil
			}
			dispatched := make(chan acp.PromptRequest, 2)
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				if req.Message != "hold the active turn" {
					dispatched <- req
					return completedSyntheticPromptEvents(req.TurnID), nil
				}
				events := make(chan acp.AgentEvent, 1)
				go func() {
					<-release
					events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
					close(events)
				}()
				return events, nil
			}
			active, err := h.manager.Prompt(ctx, sess.ID, "hold the active turn")
			if err != nil {
				t.Fatal(err)
			}
			queuedOpts := SendPromptOpts{Message: "human follow-up", Mode: BusyInputModeQueue,
				MessageID: "restart-human", IdempotencyKey: "restart-human-key"}
			human, err := h.manager.SendPrompt(ctx, sess.ID, queuedOpts)
			if err != nil {
				t.Fatal(err)
			}
			observed, err := h.manager.PromptSynthetic(ctx, sess.ID, SyntheticPromptOpts{
				Message: "resume detached completion", TurnID: "synthetic-resume-turn",
				Metadata: acp.PromptSyntheticMeta{
					TaskRunID: "run-detached", CoordinatorSessionID: "coordinator", Reason: "task_run_completed",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			pending, err := h.manager.inputQueue.List(ctx, sess.ID)
			if err != nil || len(pending) != 2 {
				t.Fatalf("pending = %#v, %v", pending, err)
			}
			if pending[1].OwnerKind != store.SessionInputOwnerSynthetic ||
				pending[1].Status != store.SessionInputQueueStatusQueued {
				t.Fatalf("synthetic admission = %#v", pending[1])
			}
			if err := h.manager.Stop(ctx, sess.ID); err != nil {
				t.Fatal(err)
			}
			if err := h.manager.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			collectEvents(t, active)
			collectEvents(t, observed)
			if err := database.Close(ctx); err != nil {
				t.Fatal(err)
			}
			reopened, err := openSessionTestGlobalDB(ctx, database.Path())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reopened.Close(testutil.Context(t)); err != nil {
					t.Error(err)
				}
			})
			h.manager = newManagerWithHarness(t, h, WithSessionInputQueueStore(reopened))
			cleanupTestManager(t, h.manager)
			replayed, err := h.manager.SendPrompt(ctx, sess.ID, queuedOpts)
			if err != nil || !replayed.Replayed || replayed.QueueEntryID != human.QueueEntryID {
				t.Fatalf("replay after database restart = %#v, %v", replayed, err)
			}
			if _, err := h.manager.Resume(ctx, sess.ID); err != nil {
				t.Fatal(err)
			}
			select {
			case req := <-dispatched:
				if req.Message != "human follow-up" || req.Meta.Synthetic != nil {
					t.Fatalf("first resumed input = %#v, want human FIFO head", req)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("human input did not dispatch after restart")
			}
			select {
			case req := <-dispatched:
				if req.TurnID != "synthetic-resume-turn" || req.Meta.Synthetic == nil ||
					req.Meta.Synthetic.TaskRunID != "run-detached" || req.Meta.Synthetic.CoordinatorSessionID != "coordinator" {
					t.Fatalf("resumed prompt = %#v", req)
				}
			case <-time.After(5 * time.Second):
				entry, readErr := reopened.GetSessionInputQueueEntry(ctx, sess.ID, pending[1].ID)
				t.Fatalf(
					"resumed session did not dispatch durable synthetic input: entry=%#v, readErr=%v",
					entry,
					readErr,
				)
			}
			if err := h.manager.WaitForPromptDrains(ctx); err != nil {
				t.Fatal(err)
			}
			entry, err := reopened.GetSessionInputQueueEntry(ctx, sess.ID, pending[1].ID)
			if err != nil || entry.Status != store.SessionInputQueueStatusSent || entry.AttemptCount != 1 {
				t.Fatalf("resumed queue receipt = %#v, %v", entry, err)
			}
			recoveredEvents, err := h.manager.Events(ctx, sess.ID, store.EventQuery{TurnID: "clear-before-restart"})
			if err != nil {
				t.Fatal(err)
			}
			if countEventType(recoveredEvents, eventspkg.TranscriptMarkerCreated) != 1 ||
				countEventType(recoveredEvents, eventspkg.SessionQueueCleared) != 1 {
				t.Fatalf("recovered clear footprint = %#v", recoveredEvents)
			}
			marker := requireTranscriptMarker(t, h.manager, sess.ID, transcript.MarkerQueueCleared)
			if marker.Evidence["actor_id"] != "restarting-operator" {
				t.Fatalf("recovered clear actor = %#v", marker.Evidence)
			}
			traces, err := reopened.ListPendingSessionInputClearTraces(ctx, sess.ID)
			if err != nil || len(traces) != 0 {
				t.Fatalf("pending clear traces after resume = %#v, %v", traces, err)
			}
			if got := len(managerPromptCalls(h)); got != 3 {
				t.Fatalf("prompt calls = %d, want active user, queued human, then synthetic", got)
			}
		},
	)
}

func TestManagerIntegrationFullLifecycleHooksFireInOrder(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(entry string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, entry)
	}

	dispatcher := &spyHookDispatcher{
		dispatchSessionPreCreateFn: func(_ context.Context, payload hookspkg.SessionPreCreatePayload) (hookspkg.SessionPreCreatePayload, error) {
			record("session.pre_create")
			return payload, nil
		},
		dispatchPromptPostAssembleFn: func(_ context.Context, payload hookspkg.PromptPayload) (hookspkg.PromptPayload, error) {
			record("prompt.post_assemble")
			return payload, nil
		},
		dispatchAgentPreStartFn: func(_ context.Context, payload hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error) {
			record("agent.pre_start")
			return payload, nil
		},
		dispatchAgentSpawnedFn: func(_ context.Context, payload hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error) {
			record("agent.spawned")
			return payload, nil
		},
		dispatchSessionPostCreateFn: func(_ context.Context, payload hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePayload, error) {
			record("session.post_create")
			return payload, nil
		},
		dispatchInputPreSubmitFn: func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
			record("input.pre_submit")
			return payload, nil
		},
		dispatchTurnStartFn: func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			record("turn.start")
			return payload, nil
		},
		dispatchTurnEndFn: func(_ context.Context, payload hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error) {
			record("turn.end")
			return payload, nil
		},
		dispatchMessageStartFn: func(_ context.Context, payload hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error) {
			record("message.start")
			return payload, nil
		},
		dispatchMessageDeltaFn: func(_ context.Context, payload hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error) {
			record("message.delta:" + payload.DeltaType)
			return payload, nil
		},
		dispatchMessageEndFn: func(_ context.Context, payload hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error) {
			record("message.end")
			return payload, nil
		},
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			record("event.pre_record:" + payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			record("event.post_record:" + payload.RecordType)
			return payload, nil
		},
		dispatchSessionPreStopFn: func(_ context.Context, payload hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error) {
			record("session.pre_stop")
			return payload, nil
		},
		dispatchAgentStoppedFn: func(_ context.Context, payload hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error) {
			record("agent.stopped")
			return payload, nil
		},
		dispatchSessionPostStopFn: func(_ context.Context, payload hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error) {
			record("session.post_stop")
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	session := createSession(t, h)
	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	want := []string{
		"session.pre_create",
		"prompt.post_assemble",
		"agent.pre_start",
		"agent.spawned",
		"session.post_create",
		"input.pre_submit",
		"turn.start",
		"event.pre_record:user_message",
		"event.post_record:user_message",
		"message.start",
		"message.delta:text",
		"event.pre_record:agent_message",
		"event.post_record:agent_message",
		"message.end",
		"event.pre_record:done",
		"event.post_record:done",
		"turn.end",
		"session.pre_stop",
		"event.pre_record:session.stop_escalated",
		"event.post_record:session.stop_escalated",
		"event.pre_record:session_stopped",
		"event.post_record:session_stopped",
		"event.pre_record:transcript_marker.created",
		"event.post_record:transcript_marker.created",
		"agent.stopped",
		"session.post_stop",
	}

	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("hook order = %#v, want %#v", got, want)
	}
}

func TestManagerIntegrationPreStopRequiredHookErrorPreventsCleanStop(t *testing.T) {
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "required-pre-stop",
			Event:        hookspkg.HookSessionPreStop,
			Mode:         hookspkg.HookModeSync,
			Required:     true,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"required-pre-stop": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPatch, error) {
					return hookspkg.SessionPreStopPatch{}, errors.New("required hook failed")
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)

	err := h.manager.Stop(testutil.Context(t), session.ID)
	if err == nil {
		t.Fatal("Stop() error = nil, want required pre-stop hook failure")
	}
	if got := session.Info().State; got != StateActive {
		t.Fatalf("session state after failed Stop() = %q, want %q", got, StateActive)
	}
	if _, ok := h.manager.Get(session.ID); !ok {
		t.Fatalf("Get(%q) = missing, want active session after failed stop", session.ID)
	}

	h.manager.hooks = HookSet{}
	if cleanupErr := h.manager.Stop(testutil.Context(t), session.ID); cleanupErr != nil {
		t.Fatalf("cleanup Stop() error = %v", cleanupErr)
	}
}

func TestManagerIntegrationUserLifecycle(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(
			fmt.Sprintf(
				"Should create prompt reopen and resume User without a registered project accepted=%t",
				accepted,
			),
			func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				ctx := testutil.Context(t)
				db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "global.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.Close(testutil.Context(t)); err != nil {
						t.Error(err)
					}
				})
				resolved, err := h.resolver.Resolve(ctx, h.workspaceID)
				if err != nil {
					t.Fatal(err)
				}
				resolved.RootDir, err = filepath.EvalSymlinks(resolved.RootDir)
				if err != nil {
					t.Fatal(err)
				}
				resolved.ID, resolved.WorkspaceID, resolved.Scope = "", "", workspacepkg.ScopeUser
				userResolver := userLifecycleResolver{WorkspaceResolver: h.resolver, resolved: resolved}
				h.manager = newManagerWithHarness(t, h, WithWorkspaceResolver(userResolver), WithSessionCatalog(db))
				cleanupTestManager(t, h.manager)
				opts := CreateOpts{Scope: workspacepkg.ScopeUser, AgentName: "coder"}
				var created *Session
				if accepted {
					info, createErr := h.manager.CreateAccepted(ctx, new(CreateAcceptedOpts{Session: opts}))
					if createErr != nil {
						t.Fatal(createErr)
					}
					created, _ = h.manager.Get(info.ID)
				} else {
					created, err = h.manager.Create(ctx, &(opts))
					if err != nil {
						t.Fatal(err)
					}
				}
				if created.Info().WorkspaceID != "" {
					t.Fatalf("workspace = %q", created.Info().WorkspaceID)
				}
				meta := created.Meta()
				if meta.CreationProfile.Version != store.UserSessionCreationProfileVersion ||
					meta.CreationProfile.WorkspaceID != workspaceaccess.UserSelector ||
					meta.CreationProfile.ProfileID != store.DefaultProfileID {
					t.Fatalf("profile = %+v", meta.CreationProfile)
				}
				events, err := h.manager.Prompt(ctx, created.ID, "first")
				if err != nil {
					t.Fatal(err)
				}
				collectEvents(t, events)
				if err := h.manager.Stop(ctx, created.ID); err != nil {
					t.Fatal(err)
				}
				h.manager = newManagerWithHarness(
					t,
					h,
					WithWorkspaceResolver(userResolver),
					WithSessionCatalog(db),
					WithSessionPromptAdmissionStore(db),
					WithSessionIDGenerator(sequentialIDGenerator("resumed")),
				)
				cleanupTestManager(t, h.manager)
				resumed, err := h.manager.Resume(ctx, created.ID)
				if err != nil {
					t.Fatal(err)
				}
				if resumed.Info().WorkspaceID != "" {
					t.Fatal("resume registered a workspace")
				}
				events, err = h.manager.Prompt(ctx, created.ID, "second")
				if err != nil {
					t.Fatal(err)
				}
				collectEvents(t, events)
				page, err := h.manager.ListPage(
					ctx,
					ListQuery{
						ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
						WorkspaceID: workspaceaccess.UserSelector,
						Limit:       10,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				if len(page.Sessions) != 1 || page.Sessions[0].ID != created.ID {
					t.Fatalf("page = %+v", page)
				}

				fork, err := h.manager.ForkSession(
					ctx,
					ForkSessionOpts{
						SourceSessionID: created.ID,
						ProfileID:       store.DefaultProfileID,
						IdempotencyKey:  "user-fork",
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				if fork.Child == nil || fork.Child.WorkspaceID != "" {
					t.Fatalf("fork=%+v", fork)
				}
				child, err := h.manager.Spawn(
					ctx,
					SpawnOpts{ParentSessionID: created.ID, AgentName: "coder", TTL: time.Minute},
				)
				if err != nil {
					t.Fatal(err)
				}
				if child.Info().WorkspaceID != "" {
					t.Fatal("spawn left User scope")
				}
				if len(h.resolver.resolveOrRegisterCalls) != 0 {
					t.Fatal("User auto-registered a project")
				}
				workspaces, err := db.ListWorkspaces(ctx)
				if err != nil || len(workspaces) != 0 {
					t.Fatalf("workspaces=%+v err=%v", workspaces, err)
				}
				for _, start := range h.driver.startCalls {
					if start.Cwd != resolved.RootDir {
						t.Fatalf("User cwd=%q want=%q", start.Cwd, resolved.RootDir)
					}
				}
				admitted, err := h.manager.SendPrompt(ctx, created.ID, SendPromptOpts{
					Message: "third", MessageID: "msg-user-admitted", IdempotencyKey: "idem-user-admitted",
				})
				if err != nil || admitted.TargetWorkspaceID != "" {
					t.Fatalf("admitted User prompt = %+v, %v", admitted, err)
				}
			},
		)
	}
}

type userLifecycleResolver struct {
	WorkspaceResolver
	resolved workspacepkg.ResolvedWorkspace
}

var _ workspacepkg.UserRuntimeResolver = userLifecycleResolver{}

func (r userLifecycleResolver) ResolveUser(
	_ context.Context,
	profileID string,
) (workspacepkg.ResolvedWorkspace, error) {
	resolved := r.resolved
	resolved.ProfileID = profileID
	resolved.ProfileName = "default"
	return resolved, nil
}

// sessionBindingIntegrationResolver uses real worktree usage leases and SQLite.
type sessionBindingIntegrationResolver struct{ service *worktreepkg.Service }

func (r sessionBindingIntegrationResolver) ResolveSessionWorktree(
	ctx context.Context,
	ws, ref string,
) (string, string, error) {
	item, err := r.service.Get(ctx, ws, ref)
	if err != nil {
		return "", "", err
	}
	return item.ID, item.Path, nil
}

func (r sessionBindingIntegrationResolver) AcquireSessionWorktreeBinding(
	ctx context.Context,
	ws, profile, ref string,
) (string, string, func(), error) {
	item, release, err := r.service.AcquireBindingUsage(ctx, ws, ref)
	if err != nil {
		return "", "", nil, err
	}
	if item.ProfileID != profile || item.State != worktreepkg.StateReady {
		release()
		return "", "", nil, worktreepkg.ErrNotReady
	}
	return item.ID, item.Path, release, nil
}

func (r sessionBindingIntegrationResolver) AcquireSessionWorktreeUnbinding(
	ctx context.Context,
	ws, profile, ref string,
) (func(), error) {
	item, release, err := r.service.AcquireBindingUsage(ctx, ws, ref)
	if err != nil {
		return nil, err
	}
	if item.ProfileID != profile {
		release()
		return nil, worktreepkg.ErrNotFound
	}
	return release, nil
}

// Injects request cancellation at the catalog I/O boundary while using real SQLite.
type cancelBindingCatalog struct {
	*globaldb.GlobalDB
	cancel context.CancelFunc
}

func (c cancelBindingCatalog) UpdateSessionWorktreeBinding(
	ctx context.Context,
	update store.SessionWorktreeBindingUpdate,
) error {
	c.cancel()
	return c.GlobalDB.UpdateSessionWorktreeBinding(ctx, update)
}
