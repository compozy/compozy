package session

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

type reparentHarness struct {
	*harness
	db *globaldb.GlobalDB
}

func newReparentHarness(t *testing.T) *reparentHarness {
	t.Helper()
	return newReparentHarnessFrom(t, newHarness(t))
}

// newSpawnReparentHarness supports governed spawns, whose tool narrowing needs hosted MCP.
func newSpawnReparentHarness(t *testing.T) *reparentHarness {
	t.Helper()
	return newReparentHarnessFrom(t, newHostedMCPHarness(t))
}

// failingTakeOverCatalog refuses to register a user-typed row while armed, the
// projection a take-over writes before its lineage transaction.
type failingTakeOverCatalog struct {
	*globaldb.GlobalDB
	armed atomic.Bool
}

func (c *failingTakeOverCatalog) RegisterSession(ctx context.Context, info store.SessionInfo) error {
	if c.armed.Load() && info.SessionType == string(SessionTypeUser) {
		return errors.New("catalog unavailable")
	}
	return c.GlobalDB.RegisterSession(ctx, info)
}

func (c *failingTakeOverCatalog) RegisterSessionWithCreationIdentity(
	ctx context.Context,
	info store.SessionInfo,
	identity store.SessionCreationIdentity,
) (store.SessionCreationRegistration, error) {
	if c.armed.Load() && info.SessionType == string(SessionTypeUser) {
		return store.SessionCreationRegistration{}, errors.New("catalog unavailable")
	}
	return c.GlobalDB.RegisterSessionWithCreationIdentity(ctx, info, identity)
}

// failingLineageCatalog cancels the request and refuses the lineage transaction,
// the step after a take-over has already projected its catalog row.
type failingLineageCatalog struct {
	*globaldb.GlobalDB
	cancel context.CancelFunc
}

func (c *failingLineageCatalog) UpdateSessionLineages(
	context.Context, string, []store.SessionLineageUpdate,
) error {
	c.cancel()
	return errors.New("lineage transaction unavailable")
}

func newReparentHarnessFrom(t *testing.T, h *harness, catalog ...store.SessionCatalog) *reparentHarness {
	t.Helper()
	db := openManagerInputQueueStore(t)
	registerManagerInputQueueWorkspace(t, db, h)
	var sessionCatalog store.SessionCatalog = db
	if len(catalog) > 0 {
		sessionCatalog = catalog[0]
	}
	h.manager = newManagerWithHarness(
		t, h, WithSessionCatalog(sessionCatalog), WithHostedMCPLauncher(h.manager.hostedMCP),
	)
	cleanupTestManager(t, h.manager)
	return &reparentHarness{harness: h, db: db}
}

func (h *reparentHarness) create(t *testing.T, parentID string) *Session {
	t.Helper()
	opts := CreateOpts{AgentName: "coder", Name: "reparent", Workspace: h.workspaceID}
	if parentID != "" {
		opts.Lineage = &store.SessionLineage{ParentSessionID: parentID}
	}
	created, err := h.manager.Create(testutil.Context(t), opts)
	if err != nil {
		t.Fatalf("Create(parent=%q) error = %v", parentID, err)
	}
	return created
}

func (h *reparentHarness) createStopped(t *testing.T, parentID string) *Session {
	t.Helper()
	created := h.create(t, parentID)
	if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
		t.Fatalf("Stop(%q) error = %v", created.ID, err)
	}
	return created
}

func (h *reparentHarness) spawnParent(t *testing.T) *Session {
	t.Helper()
	parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Name: "orchestrator", Workspace: h.workspaceID, Type: SessionTypeUser,
		Lineage: &store.SessionLineage{
			SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
			PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
		},
	})
	if err != nil {
		t.Fatalf("Create(spawn parent) error = %v", err)
	}
	return parent
}

func (h *reparentHarness) catalogLineage(t *testing.T, sessionID string) store.SessionLineage {
	t.Helper()
	rows, err := h.db.ListSessions(testutil.Context(t), store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true},
		ID:        sessionID,
	})
	if err != nil || len(rows) != 1 || rows[0].Lineage == nil {
		t.Fatalf("ListSessions(%q) = %#v, %v; want one row with lineage", sessionID, rows, err)
	}
	return *store.NormalizeSessionLineage(sessionID, rows[0].Lineage)
}

type wantLineage struct {
	parent string
	root   string
	depth  int
	kind   store.LineageKind
}

func assertReparentLineage(t *testing.T, h *reparentHarness, session *Session, want wantLineage) {
	t.Helper()
	meta := readMeta(t, session.MetaPath())
	sources := map[string]store.SessionLineage{
		"meta":    *store.NormalizeSessionLineage(session.ID, meta.Lineage),
		"catalog": h.catalogLineage(t, session.ID),
	}
	if live, ok := h.manager.Get(session.ID); ok {
		sources["memory"] = *live.Info().Lineage
	}
	for source, got := range sources {
		if got.ParentSessionID != want.parent || got.RootSessionID != want.root ||
			got.SpawnDepth != want.depth || got.Kind != want.kind {
			t.Fatalf("%s lineage of %s = %#v, want %+v", source, session.ID, got, want)
		}
	}
}

func TestManagerAdoptSession(t *testing.T) {
	t.Parallel()

	t.Run("Should adopt a parentless session and re-root its subtree", func(t *testing.T) {
		t.Parallel()

		h := newReparentHarness(t)
		top := h.createStopped(t, "")
		target := h.create(t, top.ID)
		t.Cleanup(func() { reportSessionStop(t, h.harness, target.ID) })
		source := h.createStopped(t, "")
		child := h.create(t, source.ID)
		t.Cleanup(func() { reportSessionStop(t, h.harness, child.ID) })
		grandchild := h.createStopped(t, child.ID)

		events, cancel, err := h.manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		info, err := h.manager.AdoptSession(testutil.Context(t), h.workspaceID, source.ID, target.ID)
		if err != nil {
			t.Fatalf("AdoptSession() error = %v", err)
		}
		if info.ID != source.ID || info.Lineage == nil || info.Lineage.ParentSessionID != target.ID {
			t.Fatalf("AdoptSession() info = %#v, want source adopted by %s", info, target.ID)
		}
		assertReparentLineage(t, h, source, wantLineage{
			parent: target.ID, root: top.ID, depth: 2, kind: store.LineageKindProvenance,
		})
		assertReparentLineage(t, h, child, wantLineage{
			parent: source.ID, root: top.ID, depth: 3, kind: store.LineageKindProvenance,
		})
		assertReparentLineage(t, h, grandchild, wantLineage{
			parent: child.ID, root: top.ID, depth: 4, kind: store.LineageKindProvenance,
		})

		seen := map[string]bool{}
		deadline := time.After(2 * time.Second)
		for len(seen) < 3 {
			select {
			case event := <-events:
				if event.Kind == CatalogEventUpserted {
					seen[event.SessionID] = true
				}
			case <-deadline:
				t.Fatalf("catalog upserts = %v, want source, child, and grandchild", seen)
			}
		}
		if !seen[source.ID] || !seen[child.ID] || !seen[grandchild.ID] {
			t.Fatalf("catalog upserts = %v, want source, child, and grandchild", seen)
		}
	})

	t.Run("Should detach an adopted session and restore its subtree root", func(t *testing.T) {
		t.Parallel()

		h := newReparentHarness(t)
		target := h.createStopped(t, "")
		source := h.create(t, "")
		t.Cleanup(func() { reportSessionStop(t, h.harness, source.ID) })
		child := h.createStopped(t, source.ID)
		if _, err := h.manager.AdoptSession(testutil.Context(t), h.workspaceID, source.ID, target.ID); err != nil {
			t.Fatalf("AdoptSession() error = %v", err)
		}

		info, err := h.manager.DetachSession(testutil.Context(t), h.workspaceID, source.ID)
		if err != nil {
			t.Fatalf("DetachSession() error = %v", err)
		}
		if info.Lineage == nil || info.Lineage.ParentSessionID != "" {
			t.Fatalf("DetachSession() info = %#v, want top-level", info)
		}
		assertReparentLineage(t, h, source, wantLineage{root: source.ID, kind: store.LineageKindRoot})
		assertReparentLineage(t, h, child, wantLineage{
			parent: source.ID, root: source.ID, depth: 1, kind: store.LineageKindProvenance,
		})
	})

	t.Run("Should take over a stopped spawned worker as a resumable root user session", func(t *testing.T) {
		t.Parallel()

		h := newSpawnReparentHarness(t)
		ctx := testutil.Context(t)
		parent := h.spawnParent(t)
		worker, err := h.manager.Spawn(ctx, SpawnOpts{
			ParentSessionID:  parent.ID,
			AgentName:        "coder",
			Name:             "worker",
			TTL:              time.Hour,
			AutoStopOnParent: true,
			NotifyCreator:    true,
			PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
		})
		if err != nil {
			t.Fatalf("Spawn(worker) error = %v", err)
		}
		for _, id := range []string{worker.ID, parent.ID} {
			if err := h.manager.Stop(ctx, id); err != nil {
				t.Fatalf("Stop(%q) error = %v", id, err)
			}
		}
		// Releases before the canceled-start fix recorded a reaper-canceled resume
		// as a process failure, which marks the runtime dead.
		metaPath := worker.MetaPath()
		meta := readMeta(t, metaPath)
		meta.Failure = &store.SessionFailure{
			Kind:    store.FailureProcess,
			Summary: `session: startup canceled for "` + worker.ID + `": spawn_reaper:parent_stopped`,
		}
		if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
			t.Fatal(err)
		}

		info, err := h.manager.DetachSession(ctx, h.workspaceID, worker.ID)
		if err != nil {
			t.Fatalf("DetachSession(spawned worker) error = %v", err)
		}
		if info.Type != SessionTypeUser {
			t.Fatalf("DetachSession() type = %q, want user", info.Type)
		}
		assertReparentLineage(t, h, worker, wantLineage{root: worker.ID, kind: store.LineageKindRoot})
		meta = readMeta(t, metaPath)
		if meta.SessionType != string(SessionTypeUser) || meta.Failure == nil ||
			meta.Failure.Kind != store.FailureCanceled {
			t.Fatalf(
				"taken-over meta type=%q failure=%#v, want user with a canceled stop",
				meta.SessionType, meta.Failure,
			)
		}
		lineage := store.NormalizeSessionLineage(worker.ID, meta.Lineage)
		if lineage.SpawnRole != "" || lineage.TTLExpiresAt != nil || lineage.AutoStopOnParent ||
			lineage.NotifyCreator || lineage.SpawnBudget != (store.SessionSpawnBudget{}) ||
			len(lineage.PermissionPolicy.Tools) != 0 {
			t.Fatalf("taken-over lineage kept spawn governance: %#v", lineage)
		}
		rows, err := h.db.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, ID: worker.ID,
		})
		if err != nil || len(rows) != 1 || rows[0].SessionType != string(SessionTypeUser) ||
			rows[0].Lineage == nil || rows[0].Lineage.TTLExpiresAt != nil || rows[0].Lineage.SpawnRole != "" {
			t.Fatalf("catalog row after take-over = %#v, %v", rows, err)
		}

		resumed, err := h.manager.Resume(ctx, worker.ID)
		if err != nil {
			t.Fatalf("Resume(taken-over worker) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h.harness, resumed.ID) })
		if got := resumed.Info().Type; got != SessionTypeUser {
			t.Fatalf("resumed type = %q, want user", got)
		}
	})

	t.Run("Should leave a worker unchanged when the take-over projection fails", func(t *testing.T) {
		t.Parallel()

		hostedHarness := newHostedMCPHarness(t)
		catalog := &failingTakeOverCatalog{}
		h := newReparentHarnessFrom(t, hostedHarness, catalog)
		catalog.GlobalDB = h.db
		ctx := testutil.Context(t)
		parent := h.spawnParent(t)
		worker, err := h.manager.Spawn(ctx, SpawnOpts{
			ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour, AutoStopOnParent: true,
		})
		if err != nil {
			t.Fatalf("Spawn(worker) error = %v", err)
		}
		for _, id := range []string{worker.ID, parent.ID} {
			if err := h.manager.Stop(ctx, id); err != nil {
				t.Fatalf("Stop(%q) error = %v", id, err)
			}
		}

		catalog.armed.Store(true)
		if _, err := h.manager.DetachSession(ctx, h.workspaceID, worker.ID); err == nil ||
			!strings.Contains(err.Error(), "catalog unavailable") {
			t.Fatalf("DetachSession() error = %v, want the injected projection failure", err)
		}
		meta := readMeta(t, worker.MetaPath())
		if meta.SessionType != string(SessionTypeSpawned) || meta.Lineage == nil ||
			meta.Lineage.ParentSessionID != parent.ID || !meta.Lineage.AutoStopOnParent {
			t.Fatalf(
				"meta after failed take-over = type %q lineage %#v, want unchanged",
				meta.SessionType, meta.Lineage,
			)
		}
		if got := h.catalogLineage(t, worker.ID); got.ParentSessionID != parent.ID {
			t.Fatalf("catalog lineage after failed take-over = %#v, want unchanged", got)
		}

		catalog.armed.Store(false)
		info, err := h.manager.DetachSession(ctx, h.workspaceID, worker.ID)
		if err != nil || info.Type != SessionTypeUser {
			t.Fatalf("retry DetachSession() = %#v, %v; want the take-over", info, err)
		}
	})

	t.Run("Should restore the catalog row when a canceled take-over fails its lineage update", func(t *testing.T) {
		t.Parallel()

		hostedHarness := newHostedMCPHarness(t)
		catalog := &failingLineageCatalog{}
		h := newReparentHarnessFrom(t, hostedHarness, catalog)
		catalog.GlobalDB = h.db
		ctx := testutil.Context(t)
		parent := h.spawnParent(t)
		worker, err := h.manager.Spawn(ctx, SpawnOpts{
			ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour, AutoStopOnParent: true,
		})
		if err != nil {
			t.Fatalf("Spawn(worker) error = %v", err)
		}
		for _, id := range []string{worker.ID, parent.ID} {
			if err := h.manager.Stop(ctx, id); err != nil {
				t.Fatalf("Stop(%q) error = %v", id, err)
			}
		}

		requestCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		catalog.cancel = cancel
		if _, err := h.manager.DetachSession(requestCtx, h.workspaceID, worker.ID); err == nil ||
			!strings.Contains(err.Error(), "lineage transaction unavailable") {
			t.Fatalf("DetachSession() error = %v, want the lineage failure", err)
		}
		if meta := readMeta(t, worker.MetaPath()); meta.SessionType != string(SessionTypeSpawned) {
			t.Fatalf("meta type after failed take-over = %q, want spawned", meta.SessionType)
		}
		rows, err := h.db.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, ID: worker.ID,
		})
		if err != nil || len(rows) != 1 || rows[0].SessionType != string(SessionTypeSpawned) ||
			rows[0].Lineage == nil || rows[0].Lineage.ParentSessionID != parent.ID {
			t.Fatalf("catalog row after failed take-over = %#v, %v; want the spawned worker restored", rows, err)
		}
	})

	t.Run("Should refuse taking over a running spawned worker without changing it", func(t *testing.T) {
		t.Parallel()

		h := newSpawnReparentHarness(t)
		ctx := testutil.Context(t)
		parent := h.spawnParent(t)
		t.Cleanup(func() { reportSessionStop(t, h.harness, parent.ID) })
		worker, err := h.manager.Spawn(ctx, SpawnOpts{
			ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour, AutoStopOnParent: true,
		})
		if err != nil {
			t.Fatalf("Spawn(worker) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h.harness, worker.ID) })

		_, err = h.manager.DetachSession(ctx, h.workspaceID, worker.ID)
		refusal, ok := errors.AsType[*ReparentError](err)
		if !ok || refusal.Reason != ReparentRefusalSourceRunning {
			t.Fatalf("DetachSession(running worker) error = %v, want %q", err, ReparentRefusalSourceRunning)
		}
		if got := worker.Info(); got.Type != SessionTypeSpawned || got.Lineage.ParentSessionID != parent.ID {
			t.Fatalf("running worker changed after refusal: %#v", got)
		}
	})

	t.Run("Should refuse a subtree crossing workspaces without changing any lineage", func(t *testing.T) {
		t.Parallel()
		h := newReparentHarness(t)
		source := h.createStopped(t, "")
		target := h.createStopped(t, "")
		child := store.SessionInfo{
			ID: "sess-user-child", ProfileID: store.DefaultProfileID, AgentName: "coder",
			SessionType: "user", State: "stopped", RuntimeStatus: store.SessionRuntimeUnbound,
			CreatedAt: source.CreatedAt, UpdatedAt: source.CreatedAt,
			Lineage: &store.SessionLineage{ParentSessionID: source.ID, RootSessionID: source.ID,
				SpawnDepth: 1, Kind: store.LineageKindProvenance},
		}
		if err := h.db.RegisterSession(t.Context(), child); err != nil {
			t.Fatal(err)
		}
		_, err := h.manager.AdoptSession(t.Context(), h.workspaceID, source.ID, target.ID)
		refusal, ok := errors.AsType[*ReparentError](err)
		if !ok || refusal.Reason != ReparentRefusalCrossWorkspace {
			t.Fatalf("AdoptSession() = %v", err)
		}
		assertReparentLineage(t, h, source, wantLineage{root: source.ID, kind: store.LineageKindRoot})
		if got := h.catalogLineage(t, child.ID); got.RootSessionID != source.ID || got.SpawnDepth != 1 {
			t.Fatalf("foreign descendant lineage changed: %+v", got)
		}
	})

	t.Run("Should refuse invalid reparent requests without changing lineage", func(t *testing.T) {
		t.Parallel()

		h := newReparentHarness(t)
		root := h.createStopped(t, "")
		child := h.createStopped(t, root.ID)
		archived := h.createStopped(t, "")
		if _, err := h.manager.Archive(testutil.Context(t), h.workspaceID, archived.ID); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		system, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("Create(system) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h.harness, system.ID) })

		for _, test := range []struct {
			name        string
			workspaceID string
			sessionID   string
			parentID    string
			detach      bool
			wantErr     error
			wantReason  ReparentRefusal
		}{
			{name: "self", sessionID: root.ID, parentID: root.ID, wantReason: ReparentRefusalCycle},
			{name: "descendant parent", sessionID: root.ID, parentID: child.ID, wantReason: ReparentRefusalCycle},
			{name: "source with parent", sessionID: child.ID, parentID: archived.ID, wantReason: ReparentRefusalSourceHasParent},
			{name: "archived parent", sessionID: root.ID, parentID: archived.ID, wantReason: ReparentRefusalArchived},
			{name: "archived source", sessionID: archived.ID, parentID: root.ID, wantReason: ReparentRefusalArchived},
			{name: "managed parent", sessionID: root.ID, parentID: system.ID, wantReason: ReparentRefusalParentIneligible},
			{name: "managed source", sessionID: system.ID, parentID: root.ID, wantReason: ReparentRefusalSourceNotUser},
			{name: "missing parent id", sessionID: root.ID, wantErr: ErrValidation},
			{name: "unknown parent", sessionID: root.ID, parentID: "sess-missing", wantErr: ErrSessionNotFound},
			{
				name: "foreign workspace", workspaceID: "ws-foreign", sessionID: root.ID, parentID: archived.ID,
				wantErr: ErrSessionNotFound,
			},
			{name: "detach root", sessionID: root.ID, detach: true, wantReason: ReparentRefusalNotDetachable},
		} {
			t.Run("Should refuse "+test.name, func(t *testing.T) {
				workspaceID := test.workspaceID
				if workspaceID == "" {
					workspaceID = h.workspaceID
				}
				var err error
				if test.detach {
					_, err = h.manager.DetachSession(testutil.Context(t), workspaceID, test.sessionID)
				} else {
					_, err = h.manager.AdoptSession(testutil.Context(t), workspaceID, test.sessionID, test.parentID)
				}
				if test.wantReason != "" {
					refusal, ok := errors.AsType[*ReparentError](err)
					if !ok || refusal.Reason != test.wantReason || !errors.Is(err, ErrSessionReparentRefused) {
						t.Fatalf("reparent error = %v, want refusal %q", err, test.wantReason)
					}
					return
				}
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("reparent error = %v, want %v", err, test.wantErr)
				}
			})
		}
		assertReparentLineage(t, h, root, wantLineage{root: root.ID, kind: store.LineageKindRoot})
		assertReparentLineage(t, h, child, wantLineage{
			parent: root.ID, root: root.ID, depth: 1, kind: store.LineageKindProvenance,
		})
	})
}

func TestReparentDuringStartPreparation(t *testing.T) {
	t.Parallel()
	for _, resume := range []bool{false, true} {
		name := "Should refresh a prepared child after its parent is adopted"
		if resume {
			name = "Should refresh a prepared resume after its session is adopted"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newReparentHarness(t)
			target := h.createStopped(t, "")
			source := h.createStopped(t, "")
			var spec sessionStartSpec
			var err error
			if resume {
				meta := readMeta(t, source.MetaPath())
				spec, err = h.manager.prepareResumeStart(t.Context(), &meta)
			} else {
				spec, err = h.manager.prepareCreateStart(t.Context(), CreateOpts{
					AgentName: "coder", Workspace: h.workspaceID,
					Lineage: &store.SessionLineage{ParentSessionID: source.ID},
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.manager.AdoptSession(t.Context(), h.workspaceID, source.ID, target.ID); err != nil {
				t.Fatal(err)
			}
			started, err := h.manager.startSession(t.Context(), &spec)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { reportSessionStop(t, h.harness, started.ID) })
			want := wantLineage{parent: source.ID, root: target.ID, depth: 2, kind: store.LineageKindProvenance}
			if resume {
				want.parent = target.ID
				want.depth = 1
			}
			assertReparentLineage(t, h, started, want)
		})
	}
}

func TestValidateReparentNodes(t *testing.T) {
	t.Parallel()

	user := func(lineage store.SessionLineage) reparentNode {
		return reparentNode{id: "sess-a", profileID: "default", sessionType: SessionTypeUser, lineage: lineage}
	}
	t.Run("Should refuse adopting sessions that already carry lineage or are not user sessions", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name string
			node reparentNode
			want ReparentRefusal
		}{
			{
				name: "subagent",
				node: reparentNode{id: "sess-a", sessionType: SessionTypeSpawned, lineage: store.SessionLineage{
					ParentSessionID: "sess-p", SpawnRole: store.SubagentSpawnRole, Kind: store.LineageKindSpawn,
				}},
				want: ReparentRefusalSourceNotUser,
			},
			{
				name: "spawn role user session",
				node: user(store.SessionLineage{SpawnRole: "worker", Kind: store.LineageKindSpawn}),
				want: ReparentRefusalSourceNotUser,
			},
			{
				name: "continued session",
				node: user(store.SessionLineage{ParentSessionID: "sess-p", Kind: store.LineageKindContinue}),
				want: ReparentRefusalSourceHasParent,
			},
			{
				name: "forked session",
				node: user(store.SessionLineage{ParentSessionID: "sess-p", Kind: store.LineageKindFork}),
				want: ReparentRefusalSourceHasParent,
			},
			{
				name: "recovery session",
				node: user(store.SessionLineage{ParentSessionID: "sess-p", Kind: store.LineageKindRecovery}),
				want: ReparentRefusalSourceHasParent,
			},
		} {
			t.Run("Should refuse "+test.name, func(t *testing.T) {
				t.Parallel()
				refusal, ok := errors.AsType[*ReparentError](validateAdoptSource(&test.node))
				if !ok || refusal.Reason != test.want {
					t.Fatalf("validateAdoptSource(%s) refusal = %#v, want %q", test.name, refusal, test.want)
				}
			})
		}
	})

	t.Run("Should refuse a parent from another profile as not found", func(t *testing.T) {
		t.Parallel()
		source := user(store.SessionLineage{})
		parent := reparentNode{id: "sess-p", profileID: "work", sessionType: SessionTypeUser}
		if err := validateAdoptParent(&source, &parent); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("validateAdoptParent(cross-profile) error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("Should admit only stopped, exited spawned workers for take-over", func(t *testing.T) {
		t.Parallel()
		spawned := func(role string, state State) reparentNode {
			return reparentNode{
				id: "sess-a", sessionType: SessionTypeSpawned,
				lineage: store.SessionLineage{ParentSessionID: "sess-p", SpawnRole: role, Kind: store.LineageKindSpawn},
				meta:    store.SessionMeta{ID: "sess-a", State: string(state)},
			}
		}
		for _, test := range []struct {
			name string
			node reparentNode
			want ReparentRefusal
		}{
			{name: "subagent", node: spawned(store.SubagentSpawnRole, StateStopped), want: ReparentRefusalSourceNotUser},
			{name: "daemon auto-title role", node: spawned(SpawnRoleAutoTitle, StateStopped), want: ReparentRefusalSourceNotUser},
			// After a daemon restart a stopping worker is absent from the manager.
			{name: "stopping worker without a run", node: spawned("worker", StateStopping), want: ReparentRefusalSourceRunning},
			{name: "stopped worker with an unverifiable process", node: func() reparentNode {
				node := spawned("worker", StateStopped)
				// A recorded PID without its start time cannot prove the process exited.
				node.meta.Liveness = &store.SessionLivenessMeta{SubprocessPID: 4242}
				return node
			}(), want: ReparentRefusalSourceRunning},
			{name: "stopped worker whose process still runs", node: func() reparentNode {
				node := spawned("worker", StateStopped)
				started, err := procutil.StartedAt(os.Getpid())
				if err != nil {
					t.Fatal(err)
				}
				node.meta.Liveness = &store.SessionLivenessMeta{
					SubprocessPID: os.Getpid(), SubprocessStartedAt: &started,
				}
				return node
			}(), want: ReparentRefusalSourceRunning},
			{name: "stopped worker", node: spawned("worker", StateStopped)},
			{name: "stopped custom role", node: spawned("reviewer", StateStopped)},
		} {
			t.Run("Should classify "+test.name, func(t *testing.T) {
				t.Parallel()
				takeOver, err := validateDetachSource(&test.node)
				if !takeOver {
					t.Fatalf("validateDetachSource(%s) takeOver = false, want a take-over request", test.name)
				}
				if test.want == "" {
					if err != nil {
						t.Fatalf("validateDetachSource(%s) error = %v, want admitted", test.name, err)
					}
					return
				}
				if refusal, ok := errors.AsType[*ReparentError](err); !ok || refusal.Reason != test.want {
					t.Fatalf("validateDetachSource(%s) error = %v, want %q", test.name, err, test.want)
				}
			})
		}
	})

	t.Run("Should refuse detaching continued, forked, and spawned sessions", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []store.LineageKind{
			store.LineageKindContinue, store.LineageKindFork, store.LineageKindRecovery, store.LineageKindSpawn,
		} {
			node := user(store.SessionLineage{ParentSessionID: "sess-p", Kind: kind})
			if _, err := validateDetachSource(&node); !errors.Is(err, ErrSessionReparentRefused) {
				t.Fatalf("validateDetachSource(%s) error = %v, want refusal", kind, err)
			}
		}
	})
}

func (h *harness) spawnBoundWorker(t *testing.T, parent *Session) *Session {
	t.Helper()
	worker, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
		ParentSessionID:  parent.ID,
		AgentName:        "coder",
		Name:             "worker",
		TTL:              time.Hour,
		AutoStopOnParent: true,
		PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
	})
	if err != nil {
		t.Fatalf("Spawn(worker) error = %v", err)
	}
	return worker
}

func (h *harness) mustStop(t *testing.T, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := h.manager.Stop(testutil.Context(t), id); err != nil {
			t.Fatalf("Stop(%q) error = %v", id, err)
		}
	}
}

func (h *harness) assertRunning(t *testing.T, id string) {
	t.Helper()
	info, err := h.manager.Status(testutil.Context(t), id)
	if err != nil || info.State != StateActive {
		t.Fatalf("Status(%q) = %#v, %v; want a running session", id, info, err)
	}
}

// Invariant: a parent-stop reap commits only while its condition holds, decided
// atomically with the move to stopping; owner: the session manager's stop path.
func TestStopIfParentStopped(t *testing.T) {
	t.Parallel()

	const detail = "spawn_reaper:parent_stopped"

	t.Run("Should stop the child resumably, running effects before termination", func(t *testing.T) {
		t.Parallel()
		h := newSpawnReparentHarness(t)
		ctx := testutil.Context(t)
		parent := h.spawnParent(t)
		worker := h.spawnBoundWorker(t, parent)
		h.mustStop(t, parent.ID)

		stateAtCommit := State("")
		outcome, err := h.manager.StopIfParentStopped(ctx, worker.ID, detail, func() {
			stateAtCommit = worker.Info().State
			// The commit already happened, and no lock is held: this parent resume
			// must finish, and must not save the child.
			if _, resumeErr := h.manager.Resume(ctx, parent.ID); resumeErr != nil {
				t.Errorf("Resume(parent) during commit error = %v", resumeErr)
			}
		})
		if err != nil || outcome != ParentStopCommitted {
			t.Fatalf("StopIfParentStopped() = %v, %v; want committed", outcome, err)
		}
		if stateAtCommit != StateStopping {
			t.Fatalf("state when effects ran = %q, want stopping", stateAtCommit)
		}
		meta := readMeta(t, worker.MetaPath())
		if meta.State != string(StateStopped) || (meta.Failure != nil && meta.Failure.Kind != store.FailureCanceled) ||
			meta.StopReason == nil || *meta.StopReason != store.StopUserCanceled || meta.StopDetail != detail {
			t.Fatalf("stopped meta state=%s failure=%#v reason=%v detail=%q, want a resumable reap",
				meta.State, meta.Failure, meta.StopReason, meta.StopDetail)
		}
		h.assertRunning(t, parent.ID)
		if _, err := h.manager.Resume(ctx, worker.ID); err != nil {
			t.Fatalf("Resume(reaped worker) error = %v", err)
		}
	})

	t.Run("Should keep the child when the parent resumes during the stop", func(t *testing.T) {
		t.Parallel()
		dispatcher := &spyHookDispatcher{}
		h := newHostedMCPHarness(t, WithHookSet(fullHookSet(dispatcher)))
		ctx := testutil.Context(t)
		parent, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeUser,
			Lineage: &store.SessionLineage{
				SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
				PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
			},
		})
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		worker := h.spawnBoundWorker(t, parent)
		h.mustStop(t, parent.ID)

		// session.pre_stop runs past every check and before the commit.
		dispatcher.dispatchSessionPreStopFn = func(
			_ context.Context,
			payload hookspkg.SessionPreStopPayload,
		) (hookspkg.SessionPreStopPayload, error) {
			if payload.SessionID == worker.ID {
				if _, resumeErr := h.manager.Resume(ctx, parent.ID); resumeErr != nil {
					t.Errorf("Resume(parent) error = %v", resumeErr)
				}
			}
			return payload, nil
		}
		commits := 0
		outcome, err := h.manager.StopIfParentStopped(ctx, worker.ID, detail, func() { commits++ })
		if err != nil || outcome != ParentStopSuperseded || commits != 0 {
			t.Fatalf("StopIfParentStopped() = %v, %v, commits %d; want quietly superseded", outcome, err, commits)
		}
		h.assertRunning(t, worker.ID)
		if readMeta(t, worker.MetaPath()).StopReason != nil {
			t.Fatal("a declined stop recorded a stop reason")
		}
		dispatcher.dispatchSessionPreStopFn = nil
		h.mustStop(t, worker.ID)
	})

	t.Run("Should let a stop that joined a declined one stop the child", func(t *testing.T) {
		t.Parallel()
		dispatcher := &spyHookDispatcher{}
		h := newHostedMCPHarness(t, WithHookSet(fullHookSet(dispatcher)))
		ctx := testutil.Context(t)
		parent, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeUser,
			Lineage: &store.SessionLineage{
				SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
				PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{testToolRead}},
			},
		})
		if err != nil {
			t.Fatalf("Create(parent) error = %v", err)
		}
		worker := h.spawnBoundWorker(t, parent)

		// The child's parent stays live, so the guard declines after this request
		// joined or, at the latest, right before it claims its own run.
		joined := make(chan error, 1)
		var once sync.Once
		dispatcher.dispatchSessionPreStopFn = func(
			_ context.Context,
			payload hookspkg.SessionPreStopPayload,
		) (hookspkg.SessionPreStopPayload, error) {
			once.Do(func() { go func() { joined <- h.manager.Stop(ctx, worker.ID) }() })
			return payload, nil
		}
		outcome, err := h.manager.StopIfParentStopped(ctx, worker.ID, detail, nil)
		if err != nil || outcome == ParentStopCommitted {
			t.Fatalf("StopIfParentStopped() = %v, %v; want no commit", outcome, err)
		}
		if err := <-joined; err != nil {
			t.Fatalf("Stop() after a declined stop error = %v", err)
		}
		if got := readMeta(t, worker.MetaPath()).State; got != string(StateStopped) {
			t.Fatalf("child state = %q, want stopped by the operator request", got)
		}
	})

	for _, tc := range []struct {
		name   string
		change func(t *testing.T, h *reparentHarness, worker *Session)
	}{
		{name: "taken over", change: func(t *testing.T, h *reparentHarness, worker *Session) {
			t.Helper()
			ctx := testutil.Context(t)
			h.mustStop(t, worker.ID)
			if _, err := h.manager.DetachSession(ctx, h.workspaceID, worker.ID); err != nil {
				t.Fatalf("DetachSession() error = %v", err)
			}
			if _, err := h.manager.Resume(ctx, worker.ID); err != nil {
				t.Fatalf("Resume(taken over) error = %v", err)
			}
		}},
		{name: "resumed by the user", change: func(t *testing.T, h *reparentHarness, worker *Session) {
			t.Helper()
			h.mustStop(t, worker.ID)
			if _, err := h.manager.Resume(testutil.Context(t), worker.ID); err != nil {
				t.Fatalf("Resume(worker) error = %v", err)
			}
		}},
	} {
		t.Run("Should keep a child "+tc.name+" after the reaper reread it", func(t *testing.T) {
			t.Parallel()
			h := newSpawnReparentHarness(t)
			ctx := testutil.Context(t)
			parent := h.spawnParent(t)
			worker := h.spawnBoundWorker(t, parent)
			h.mustStop(t, parent.ID)

			// The reread: this is everything the reaper knows when it asks for the stop.
			reread, err := h.manager.Status(ctx, worker.ID)
			if err != nil || reread.State != StateActive || !reread.Lineage.AutoStopOnParent {
				t.Fatalf("Status(worker) = %#v, %v; want a reapable child", reread, err)
			}
			tc.change(t, h, worker)

			commits := 0
			outcome, err := h.manager.StopIfParentStopped(ctx, worker.ID, detail, func() { commits++ })
			if err != nil || outcome != ParentStopSuperseded || commits != 0 {
				t.Fatalf("StopIfParentStopped() = %v, %v, commits %d; want quietly superseded", outcome, err, commits)
			}
			h.assertRunning(t, worker.ID)
		})
	}

	t.Run("Should report a child that already stopped as ended", func(t *testing.T) {
		t.Parallel()
		h := newSpawnReparentHarness(t)
		parent := h.spawnParent(t)
		worker := h.spawnBoundWorker(t, parent)
		h.mustStop(t, worker.ID, parent.ID)

		commits := 0
		outcome, err := h.manager.StopIfParentStopped(testutil.Context(t), worker.ID, detail, func() { commits++ })
		if err != nil || outcome != ParentStopEnded || commits != 0 {
			t.Fatalf("StopIfParentStopped() = %v, %v, commits %d; want ended", outcome, err, commits)
		}
	})

	t.Run("Should guard a child still starting", func(t *testing.T) {
		t.Parallel()
		h := newSpawnReparentHarness(t)
		ctx := testutil.Context(t)
		parent := h.spawnParent(t)
		worker := h.spawnBoundWorker(t, parent)
		// The parent is live, so this resume keeps the binding.
		h.mustStop(t, worker.ID)
		entered := make(chan struct{})
		release := make(chan struct{})
		h.driver.mu.Lock()
		h.driver.startContextHook = func(ctx context.Context, _ acp.StartOpts, _ int) (*fakeProcess, error) {
			close(entered)
			// The fake driver holds its lock across this hook; stopping the parent needs it.
			h.driver.mu.Unlock()
			defer h.driver.mu.Lock()
			select {
			case <-release:
				return nil, errors.New("launch released")
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		h.driver.mu.Unlock()
		resumed := make(chan error, 1)
		go func() {
			_, err := h.manager.Resume(ctx, worker.ID)
			resumed <- err
		}()
		<-entered

		commits := 0
		outcome, err := h.manager.StopIfParentStopped(ctx, worker.ID, detail, func() { commits++ })
		if err != nil || outcome != ParentStopSuperseded || commits != 0 {
			t.Fatalf("StopIfParentStopped(live parent) = %v, %v; want superseded", outcome, err)
		}
		if info, err := h.manager.Status(ctx, worker.ID); err != nil || info.State != StateStarting {
			t.Fatalf("Status after a declined stop = %#v, %v; want the launch undisturbed", info, err)
		}
		h.mustStop(t, parent.ID)
		outcome, err = h.manager.StopIfParentStopped(ctx, worker.ID, detail, func() { commits++ })
		if err != nil || outcome != ParentStopCommitted || commits != 1 {
			t.Fatalf("StopIfParentStopped(stopped parent) = %v, %v; want committed", outcome, err)
		}
		close(release)
		<-resumed
		if got := readMeta(t, worker.MetaPath()).State; got != string(StateStopped) {
			t.Fatalf("child state = %q, want stopped", got)
		}
	})
}
