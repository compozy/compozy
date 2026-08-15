package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestManagerDelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "Should remove a stopped user session from durable counts",
			run: func(t *testing.T) {
				catalog := newRecordingSessionCatalog()
				h := newHarness(t, WithSessionCatalog(catalog))
				session := createSession(t, h)
				attachmentPath := writeSessionAttachmentFixture(t, h, session.ID, "delete-me")

				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}

				if _, err := os.Stat(session.SessionDir()); err != nil {
					t.Fatalf("Stat(session dir before delete) error = %v", err)
				}
				events, cancel, err := h.manager.SubscribeSessionCatalogEvents(
					testutil.Context(t),
				)
				if err != nil {
					t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
				}
				defer cancel()

				if err := h.manager.Delete(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}

				if _, err := os.Stat(session.SessionDir()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Stat(session dir after delete) error = %v, want os.ErrNotExist", err)
				}
				if _, err := os.Stat(attachmentPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Stat(attachment after delete) error = %v, want os.ErrNotExist", err)
				}
				if _, err := h.manager.Status(testutil.Context(t), session.ID); !errors.Is(err, ErrSessionNotFound) {
					t.Fatalf("Status(after delete) error = %v, want %v", err, ErrSessionNotFound)
				}

				infos, err := h.manager.ListAll(testutil.Context(t))
				if err != nil {
					t.Fatalf("ListAll() error = %v", err)
				}
				for _, info := range infos {
					if info != nil && info.ID == session.ID {
						t.Fatalf("ListAll() still returned deleted session %q", session.ID)
					}
				}
				assertDeletedUserSessionCatalogTruth(t, catalog, events, h.workspaceID, session.ID)
			},
		},
		{
			name: "Should stop and delete an active user session from durable counts",
			run: func(t *testing.T) {
				catalog := newRecordingSessionCatalog()
				h := newHarness(t, WithSessionCatalog(catalog))
				session := createSession(t, h)
				events, cancel, err := h.manager.SubscribeSessionCatalogEvents(
					testutil.Context(t),
				)
				if err != nil {
					t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
				}
				defer cancel()

				if got := h.driver.stopCalls; got != 0 {
					t.Fatalf("driver stop calls before delete = %d, want 0", got)
				}

				if err := h.manager.Delete(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Delete(active) error = %v", err)
				}

				if got := h.driver.stopCalls; got != 1 {
					t.Fatalf("driver stop calls after delete = %d, want 1", got)
				}
				if _, ok := h.manager.Get(session.ID); ok {
					t.Fatalf("Get(%q) after delete = found, want missing", session.ID)
				}
				if _, err := os.Stat(session.SessionDir()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Stat(session dir after delete) error = %v, want os.ErrNotExist", err)
				}
				assertDeletedUserSessionCatalogTruth(t, catalog, events, h.workspaceID, session.ID)
			},
		},
		{
			name: "Should wait for active stored readers before committing deletion",
			run: func(t *testing.T) {
				catalog := newRecordingSessionCatalog()
				catalog.requireExistingUpdates()
				h := newHarness(
					t,
					WithSessionCatalog(catalog),
					withDefaultQueryStoreRuntime(),
				)
				shutdownQueryStoreRuntimeForTest(t, h.manager)
				session := createSession(t, h)
				ctx := testutil.Context(t)

				reader, err := h.manager.queryStoreRuntime.Open(
					ctx,
					testSessionDBOwner(session.ID, session.WorkspaceID),
					session.DBPath(),
				)
				if err != nil {
					t.Fatalf("Open(stored reader) error = %v", err)
				}
				t.Cleanup(func() {
					if closeErr := reader.Close(testutil.Context(t)); closeErr != nil {
						t.Errorf("Close(stored reader cleanup) error = %v", closeErr)
					}
				})

				deleteDone := make(chan error, 1)
				go func() {
					deleteDone <- h.manager.Delete(ctx, session.ID)
				}()

				for {
					probe, openErr := h.manager.queryStoreRuntime.Open(
						ctx,
						testSessionDBOwner(session.ID, session.WorkspaceID),
						session.DBPath(),
					)
					if errors.Is(openErr, sessiondb.ErrReadOnlyPoolQuiescing) {
						break
					}
					if openErr != nil {
						t.Fatalf("Open(quiescence probe) error = %v", openErr)
					}
					if closeErr := probe.Close(ctx); closeErr != nil {
						t.Fatalf("Close(quiescence probe) error = %v", closeErr)
					}
					select {
					case deleteErr := <-deleteDone:
						t.Fatalf("Delete() returned before stored reader closed: %v", deleteErr)
					default:
						runtime.Gosched()
					}
				}

				select {
				case deleteErr := <-deleteDone:
					t.Fatalf("Delete() returned while stored reader was active: %v", deleteErr)
				default:
				}
				if _, err := os.Stat(session.SessionDir()); err != nil {
					t.Fatalf("Stat(session dir while deletion quiesced) error = %v", err)
				}
				if _, ok := catalog.get(session.ID); !ok {
					t.Fatalf("catalog lost session %q while deletion quiesced", session.ID)
				}
				if err := reader.Close(ctx); err != nil {
					t.Fatalf("Close(stored reader) error = %v", err)
				}
				select {
				case deleteErr := <-deleteDone:
					if deleteErr != nil {
						t.Fatalf("Delete() error = %v", deleteErr)
					}
				case <-ctx.Done():
					t.Fatalf("Delete() did not finish after stored reader closed: %v", ctx.Err())
				}

				if _, err := os.Stat(session.SessionDir()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Stat(session dir after delete) error = %v, want os.ErrNotExist", err)
				}
				if _, ok := catalog.get(session.ID); ok {
					t.Fatalf("catalog still returned deleted session %q", session.ID)
				}
			},
		},
		{
			name: "Should refuse a foreign database family before staging deletion",
			run: func(t *testing.T) {
				h := newHarness(t)
				target := createSession(t, h)
				foreign := createSession(t, h)
				if err := h.manager.Stop(testutil.Context(t), target.ID); err != nil {
					t.Fatalf("Stop(target) error = %v", err)
				}
				if err := h.manager.Stop(testutil.Context(t), foreign.ID); err != nil {
					t.Fatalf("Stop(foreign) error = %v", err)
				}

				substituteManagerSessionDBFamily(t, target.DBPath(), foreign.DBPath())
				before := readManagerSessionDBFamilyDigest(t, target.DBPath())
				err := h.manager.Delete(testutil.Context(t), target.ID)
				if !errors.Is(err, sessiondb.ErrSessionDBOwnerMismatch) {
					t.Fatalf("Delete(foreign family) error = %v, want owner mismatch", err)
				}
				assertManagerSessionDBFamilyUnchanged(t, target.DBPath(), before)
				if _, statErr := os.Stat(target.SessionDir()); statErr != nil {
					t.Fatalf("Stat(target session directory after refusal) error = %v", statErr)
				}
			},
		},
		{
			name: "Should refuse metadata relabeled as another session without touching either directory",
			run: func(t *testing.T) {
				h := newHarness(t)
				target := createSession(t, h)
				foreign := createSession(t, h)
				if err := h.manager.Stop(testutil.Context(t), target.ID); err != nil {
					t.Fatalf("Stop(target) error = %v", err)
				}
				if err := h.manager.Stop(testutil.Context(t), foreign.ID); err != nil {
					t.Fatalf("Stop(foreign) error = %v", err)
				}

				metaPath := store.SessionMetaFile(target.SessionDir())
				meta, err := store.ReadSessionMeta(metaPath)
				if err != nil {
					t.Fatalf("ReadSessionMeta(target) error = %v", err)
				}
				meta.ID = foreign.ID
				meta.WorkspaceID = foreign.WorkspaceID
				if meta.Lineage != nil {
					meta.Lineage.RootSessionID = foreign.ID
				}
				encodedMeta, err := json.Marshal(meta)
				if err != nil {
					t.Fatalf("json.Marshal(relabeled target metadata) error = %v", err)
				}
				if err := os.WriteFile(metaPath, encodedMeta, 0o600); err != nil {
					t.Fatalf("os.WriteFile(relabeled target metadata) error = %v", err)
				}
				targetBefore := readManagerSessionDBFamilyDigest(t, target.DBPath())
				foreignBefore := readManagerSessionDBFamilyDigest(t, foreign.DBPath())

				err = h.manager.Delete(testutil.Context(t), target.ID)
				if !errors.Is(err, ErrSessionNotFound) {
					t.Fatalf("Delete(relabeled metadata) error = %v, want ErrSessionNotFound", err)
				}
				assertManagerSessionDBFamilyUnchanged(t, target.DBPath(), targetBefore)
				assertManagerSessionDBFamilyUnchanged(t, foreign.DBPath(), foreignBefore)
				for _, sessionDir := range []string{target.SessionDir(), foreign.SessionDir()} {
					if _, statErr := os.Stat(sessionDir); statErr != nil {
						t.Fatalf("Stat(%q after refusal) error = %v", sessionDir, statErr)
					}
				}
			},
		},
		{
			name: "Should return session not found when catalog and artifacts are absent",
			run: func(t *testing.T) {
				catalog := newRecordingSessionCatalog()
				h := newHarness(t, WithSessionCatalog(catalog))

				err := h.manager.Delete(testutil.Context(t), "sess-missing-delete")
				if !errors.Is(err, ErrSessionNotFound) {
					t.Fatalf("Delete(missing) error = %v, want ErrSessionNotFound", err)
				}
			},
		},
		{
			name: "Should ignore concurrent stop races that report session not found",
			run: func(t *testing.T) {
				called := false

				err := stopSessionBeforeDelete(
					testutil.Context(t),
					"sess-race",
					func(context.Context, string, StopCause, string) error {
						called = true
						return ErrSessionNotFound
					},
				)
				if err != nil {
					t.Fatalf("stopSessionBeforeDelete() error = %v, want nil", err)
				}
				if !called {
					t.Fatal("stopSessionBeforeDelete() did not call the stop function")
				}
			},
		},
		{
			name: "Should wrap stop errors with delete context",
			run: func(t *testing.T) {
				h := newHarness(t)
				session := createSession(t, h)
				stopErr := errors.New("driver stop failed")
				h.driver.stopHook = func(*fakeProcess) error {
					return stopErr
				}

				err := h.manager.Delete(testutil.Context(t), session.ID)
				if !errors.Is(err, stopErr) {
					t.Fatalf("Delete() error = %v, want wrapped stop error", err)
				}
				if !strings.Contains(err.Error(), `session: stop "`) {
					t.Fatalf("Delete() error = %q, want stop context", err.Error())
				}
			},
		},
		{
			name: "Should restore the staged directory when catalog deletion fails",
			run: func(t *testing.T) {
				catalog := newRecordingSessionCatalog()
				h := newHarness(
					t,
					WithSessionCatalog(catalog),
					withDefaultQueryStoreRuntime(),
				)
				shutdownQueryStoreRuntimeForTest(t, h.manager)
				session := createSession(t, h)
				attachmentPath := writeSessionAttachmentFixture(t, h, session.ID, "restore-me")
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}
				deleteErr := errors.New("catalog delete failed")
				catalog.deleteErr = deleteErr

				err := h.manager.Delete(testutil.Context(t), session.ID)
				if !errors.Is(err, deleteErr) {
					t.Fatalf("Delete() error = %v, want %v", err, deleteErr)
				}
				if _, err := os.Stat(session.SessionDir()); err != nil {
					t.Fatalf("Stat(restored session dir) error = %v", err)
				}
				if payload, err := os.ReadFile(attachmentPath); err != nil || string(payload) != "restore-me" {
					t.Fatalf("ReadFile(restored attachment) = %q, %v", payload, err)
				}
				listed, err := catalog.ListSessions(testutil.Context(t), store.SessionListQuery{
					WorkspaceID: h.workspaceID,
					SessionType: string(SessionTypeUser),
				})
				if err != nil {
					t.Fatalf("ListSessions() error = %v", err)
				}
				if len(listed) != 1 || listed[0].ID != session.ID {
					t.Fatalf("ListSessions() = %#v, want preserved session %q", listed, session.ID)
				}
				entries, err := os.ReadDir(h.homePaths.SessionsDir)
				if err != nil {
					t.Fatalf("ReadDir(sessions) error = %v", err)
				}
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
						t.Fatalf("rollback left deletion tombstone %q", entry.Name())
					}
				}
				reopened, err := h.manager.queryStoreRuntime.Open(
					testutil.Context(t),
					testSessionDBOwner(session.ID, session.WorkspaceID),
					session.DBPath(),
				)
				if err != nil {
					t.Fatalf("Open(stored reader after catalog failure) error = %v", err)
				}
				if err := reopened.Close(testutil.Context(t)); err != nil {
					t.Fatalf("Close(reopened stored reader) error = %v", err)
				}
			},
		},
		{
			name: "Should keep a staged tombstone leased until catalog rollback restores it",
			run: func(t *testing.T) {
				ctx := testutil.Context(t)
				catalog := newRecordingSessionCatalog()
				h := newHarness(t, WithSessionCatalog(catalog), withDefaultQueryStoreRuntime())
				shutdownQueryStoreRuntimeForTest(t, h.manager)
				session := createSession(t, h)
				if err := h.manager.Stop(ctx, session.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}

				deleteEntered := make(chan struct{})
				releaseDelete := make(chan struct{})
				catalogErr := errors.New("catalog delete interrupted")
				catalog.deleteHook = func(hookCtx context.Context, id string) error {
					if id != session.ID {
						return fmt.Errorf("unexpected catalog delete id %q", id)
					}
					close(deleteEntered)
					select {
					case <-releaseDelete:
						return catalogErr
					case <-hookCtx.Done():
						return hookCtx.Err()
					}
				}

				deleteDone := make(chan error, 1)
				go func() {
					deleteDone <- h.manager.Delete(ctx, session.ID)
				}()
				select {
				case <-deleteEntered:
				case <-ctx.Done():
					t.Fatalf("Delete() did not reach catalog mutation: %v", ctx.Err())
				}

				tombstonePath, tombstoneName := findSessionDeleteTombstone(t, h.homePaths.SessionsDir)
				cleanupAttempted := make(chan struct{})
				cleanupManager := &Manager{
					homePaths:      h.homePaths,
					sessionCatalog: catalog,
					removeAllPath:  nil,
					lifecycleCtx:   ctx,
					acquireSessionDBFamilyLease: func(
						acquireCtx context.Context,
						path string,
					) (*sessiondb.FamilyLease, error) {
						close(cleanupAttempted)
						return sessiondb.AcquireFamilyLease(acquireCtx, path)
					},
				}
				cleanupDone := make(chan error, 1)
				go func() {
					cleanupDone <- cleanupManager.cleanupSessionDeleteTombstone(tombstonePath, tombstoneName)
				}()
				select {
				case <-cleanupAttempted:
				case <-ctx.Done():
					t.Fatalf("concurrent cleanup did not attempt the family lease: %v", ctx.Err())
				}
				select {
				case cleanupErr := <-cleanupDone:
					t.Fatalf("concurrent cleanup crossed the held family lease: %v", cleanupErr)
				default:
				}

				close(releaseDelete)
				select {
				case deleteErr := <-deleteDone:
					if !errors.Is(deleteErr, catalogErr) {
						t.Fatalf("Delete() error = %v, want %v", deleteErr, catalogErr)
					}
				case <-ctx.Done():
					t.Fatalf("Delete() did not roll back after catalog failure: %v", ctx.Err())
				}
				select {
				case cleanupErr := <-cleanupDone:
					if cleanupErr == nil {
						t.Fatal("concurrent cleanup error = nil after rollback removed its tombstone")
					}
				case <-ctx.Done():
					t.Fatalf("concurrent cleanup did not finish after rollback released the lease: %v", ctx.Err())
				}

				if _, err := os.Stat(session.SessionDir()); err != nil {
					t.Fatalf("Stat(restored session directory) error = %v", err)
				}
				assertNoSessionDeleteTombstones(t, h.homePaths.SessionsDir)
				reader, err := sessiondb.OpenSessionDBReadOnly(
					ctx,
					testSessionDBOwner(session.ID, session.WorkspaceID),
					session.DBPath(),
				)
				if err != nil {
					t.Fatalf("OpenSessionDBReadOnly(restored) error = %v", err)
				}
				if err := reader.Close(ctx); err != nil {
					t.Fatalf("Close(restored reader) error = %v", err)
				}
			},
		},
		{
			name: "Should recover an interrupted staged tombstone from every catalog state",
			run: func(t *testing.T) {
				tests := []struct {
					name           string
					catalogKnown   bool
					catalogPresent bool
					wantRestored   bool
				}{
					{
						name:         "Should restore conservatively when catalog ownership is unknown",
						wantRestored: true,
					},
					{
						name:           "Should restore when the catalog still owns the session",
						catalogKnown:   true,
						catalogPresent: true,
						wantRestored:   true,
					},
					{
						name:         "Should finish deletion when the catalog no longer owns the session",
						catalogKnown: true,
					},
				}
				for _, test := range tests {
					t.Run(test.name, func(t *testing.T) {
						t.Parallel()

						ctx := testutil.Context(t)
						catalog := newRecordingSessionCatalog()
						managerOptions := []Option(nil)
						if test.catalogKnown {
							managerOptions = append(managerOptions, WithSessionCatalog(catalog))
						}
						h := newHarness(t, managerOptions...)
						session := createSession(t, h)
						attachmentPath := writeSessionAttachmentFixture(t, h, session.ID, "recover-me")
						if err := h.manager.Stop(ctx, session.ID); err != nil {
							t.Fatalf("Stop() error = %v", err)
						}

						staged, err := h.manager.stageSessionDelete(ctx, session.ID, false)
						if err != nil {
							t.Fatalf("stageSessionDelete() error = %v", err)
						}
						if err := staged.capabilities.Release(); err != nil {
							t.Fatalf("release(staged crash simulation) error = %v", err)
						}
						if _, err := os.Stat(session.SessionDir()); !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("Stat(original before recovery) error = %v, want os.ErrNotExist", err)
						}
						if test.catalogKnown && !test.catalogPresent {
							if err := catalog.DeleteSession(ctx, session.ID); err != nil {
								t.Fatalf("DeleteSession(catalog crash state) error = %v", err)
							}
						}

						recoveryManager := newManagerWithHarness(t, h, managerOptions...)
						shutdownQueryStoreRuntimeForTest(t, recoveryManager)
						_, statErr := os.Stat(session.SessionDir())
						if test.wantRestored {
							if statErr != nil {
								t.Fatalf("Stat(restored staged session) error = %v", statErr)
							}
							reader, err := sessiondb.OpenSessionDBReadOnly(
								ctx,
								testSessionDBOwner(session.ID, session.WorkspaceID),
								session.DBPath(),
							)
							if err != nil {
								t.Fatalf("OpenSessionDBReadOnly(restored staged session) error = %v", err)
							}
							if err := reader.Close(ctx); err != nil {
								t.Fatalf("Close(restored staged reader) error = %v", err)
							}
							if payload, err := os.ReadFile(attachmentPath); err != nil || string(payload) != "recover-me" {
								t.Fatalf("ReadFile(restored staged attachment) = %q, %v", payload, err)
							}
						} else if !errors.Is(statErr, os.ErrNotExist) {
							t.Fatalf("Stat(deleted staged session) error = %v, want os.ErrNotExist", statErr)
						}
						if !test.wantRestored {
							if _, err := os.Stat(attachmentPath); !errors.Is(err, os.ErrNotExist) {
								t.Fatalf("Stat(deleted staged attachment) error = %v, want os.ErrNotExist", err)
							}
						}
						assertNoSessionDeleteTombstones(t, h.homePaths.SessionsDir)
					})
				}
			},
		},
		{
			name: "Should retain cleanup tombstones without reporting a partial deletion",
			run: func(t *testing.T) {
				catalog := newRecordingSessionCatalog()
				h := newHarness(
					t,
					WithSessionCatalog(catalog),
					withDefaultQueryStoreRuntime(),
				)
				shutdownQueryStoreRuntimeForTest(t, h.manager)
				session := createSession(t, h)
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}
				removeErr := errors.New("cleanup unavailable")
				h.manager.removeAllPath = func(string) error { return removeErr }

				if err := h.manager.Delete(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Delete() error = %v, want logical deletion with deferred cleanup", err)
				}
				if _, err := h.manager.Status(testutil.Context(t), session.ID); !errors.Is(err, ErrSessionNotFound) {
					t.Fatalf("Status(after logical delete) error = %v, want %v", err, ErrSessionNotFound)
				}
				entries, err := os.ReadDir(h.homePaths.SessionsDir)
				if err != nil {
					t.Fatalf("ReadDir(sessions) error = %v", err)
				}
				foundTombstone := false
				for _, entry := range entries {
					foundTombstone = foundTombstone || strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix)
				}
				if !foundTombstone {
					t.Fatal("deferred cleanup did not retain a deletion tombstone")
				}
				recoveryManager := newManagerWithHarness(t, h, WithSessionCatalog(catalog))
				shutdownQueryStoreRuntimeForTest(t, recoveryManager)
				assertNoSessionDeleteTombstones(t, h.homePaths.SessionsDir)
				reader, openErr := h.manager.queryStoreRuntime.Open(
					testutil.Context(t),
					testSessionDBOwner(session.ID, session.WorkspaceID),
					session.DBPath(),
				)
				if errors.Is(openErr, sessiondb.ErrReadOnlyPoolQuiescing) {
					t.Fatal("stored reader pool remained quiesced after logical deletion")
				}
				if openErr == nil {
					if err := reader.Close(testutil.Context(t)); err != nil {
						t.Fatalf("Close(stored reader after logical delete) error = %v", err)
					}
				}
			},
		},
		{
			name: "Should reject workspace removal while a session start is reserved",
			run: func(t *testing.T) {
				ctx := testutil.Context(t)
				db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "global.db"))
				if err != nil {
					t.Fatalf("OpenGlobalDB() error = %v", err)
				}
				t.Cleanup(func() {
					if err := db.Close(testutil.Context(t)); err != nil {
						t.Errorf("Close() error = %v", err)
					}
				})
				h := newHarness(t, WithSessionCatalog(db))
				now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
				if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
					ID: h.workspaceID, RootDir: h.workspace, Name: h.workspaceName,
					CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("InsertWorkspace() error = %v", err)
				}
				resolver, err := workspacepkg.NewResolver(
					db,
					workspacepkg.WithHomePaths(h.homePaths),
					workspacepkg.WithConfigLoader(func(string) (compozyconfig.Config, error) { return h.cfg, nil }),
				)
				if err != nil {
					t.Fatalf("NewResolver() error = %v", err)
				}
				resolver.SetUnregisterPreparer(
					func(ctx context.Context, workspace workspacepkg.Workspace) (workspacepkg.UnregisterPreparation, error) {
						return h.manager.PrepareWorkspaceRemoval(ctx, workspace.ID)
					},
				)

				startEntered := make(chan struct{})
				releaseStart := make(chan struct{})
				h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
					close(startEntered)
					<-releaseStart
					return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-starting"), nil
				}
				type createResult struct {
					session *Session
					err     error
				}
				created := make(chan createResult, 1)
				go func() {
					session, createErr := h.manager.Create(ctx, CreateOpts{
						AgentName: "coder", Workspace: h.workspaceID,
					})
					created <- createResult{session: session, err: createErr}
				}()
				select {
				case <-startEntered:
				case <-ctx.Done():
					t.Fatalf("Create() did not reach provider start: %v", ctx.Err())
				}

				unregisterErr := resolver.Unregister(ctx, h.workspaceID)
				if !errors.Is(unregisterErr, workspacepkg.ErrWorkspaceHasActiveSessions) {
					t.Fatalf(
						"Unregister(starting workspace) error = %v, want %v",
						unregisterErr,
						workspacepkg.ErrWorkspaceHasActiveSessions,
					)
				}
				if _, err := db.GetWorkspace(ctx, h.workspaceID); err != nil {
					t.Fatalf("GetWorkspace(after rejected unregister) error = %v", err)
				}
				metaPath := store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "sess-1"))
				meta, err := store.ReadSessionMeta(metaPath)
				if err != nil {
					t.Fatalf("ReadSessionMeta(starting) error = %v", err)
				}
				if meta.State != string(StateStarting) || meta.WorkspaceID != h.workspaceID {
					t.Fatalf("starting session metadata = %#v", meta)
				}

				close(releaseStart)
				var result createResult
				select {
				case result = <-created:
				case <-ctx.Done():
					t.Fatalf("Create() did not finish after release: %v", ctx.Err())
				}
				if result.err != nil || result.session == nil || result.session.Info().State != StateActive {
					t.Fatalf("Create() result = session:%#v error:%v", result.session, result.err)
				}
				t.Cleanup(func() {
					if err := h.manager.Stop(testutil.Context(t), result.session.ID); err != nil {
						t.Errorf("Stop() cleanup error = %v", err)
					}
				})
			},
		},
		{
			name: "Should prune a missing workspace through the staged session owner",
			run: func(t *testing.T) {
				ctx := testutil.Context(t)
				db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "global.db"))
				if err != nil {
					t.Fatalf("OpenGlobalDB() error = %v", err)
				}
				t.Cleanup(func() {
					if err := db.Close(testutil.Context(t)); err != nil {
						t.Errorf("Close() error = %v", err)
					}
				})
				h := newHarness(t, WithSessionCatalog(db))
				now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
				if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
					ID:        h.workspaceID,
					RootDir:   h.workspace,
					Name:      h.workspaceName,
					CreatedAt: now,
					UpdatedAt: now,
				}); err != nil {
					t.Fatalf("InsertWorkspace() error = %v", err)
				}
				session := createSession(t, h)
				attachmentPath := writeSessionAttachmentFixture(t, h, session.ID, "workspace-session")
				orphanPath := filepath.Join(
					h.homePaths.SessionAttachmentsDir,
					h.workspaceID,
					"orphan-session",
					"orphan.bin",
				)
				if err := os.MkdirAll(filepath.Dir(orphanPath), 0o700); err != nil {
					t.Fatalf("MkdirAll(orphan attachment) error = %v", err)
				}
				if err := os.WriteFile(orphanPath, []byte("orphan"), 0o600); err != nil {
					t.Fatalf("WriteFile(orphan attachment) error = %v", err)
				}
				if err := h.manager.Stop(ctx, session.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}
				if _, err := db.DB().ExecContext(
					ctx,
					`INSERT INTO permission_log (
						id, session_id, agent_name, action, resource, decision, policy_used, timestamp
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					"perm-workspace-prune",
					session.ID,
					"coder",
					"invoke",
					"compozy__task_run_complete",
					"allow",
					"test",
					now.Format(time.RFC3339Nano),
				); err != nil {
					t.Fatalf("Insert permission_log error = %v", err)
				}
				if _, err := db.DB().ExecContext(
					ctx,
					`INSERT INTO token_stats (
						id, session_id, agent_name, turn_count, updated_at
					) VALUES (?, ?, ?, ?, ?)`,
					"tokens-workspace-prune",
					session.ID,
					"coder",
					1,
					now.Format(time.RFC3339Nano),
				); err != nil {
					t.Fatalf("Insert token_stats error = %v", err)
				}

				resolver, err := workspacepkg.NewResolver(
					db,
					workspacepkg.WithHomePaths(h.homePaths),
					workspacepkg.WithConfigLoader(func(string) (compozyconfig.Config, error) { return h.cfg, nil }),
				)
				if err != nil {
					t.Fatalf("NewResolver() error = %v", err)
				}
				resolver.SetUnregisterPreparer(
					func(ctx context.Context, workspace workspacepkg.Workspace) (workspacepkg.UnregisterPreparation, error) {
						return h.manager.PrepareWorkspaceRemoval(ctx, workspace.ID)
					},
				)
				if err := os.RemoveAll(h.workspace); err != nil {
					t.Fatalf("RemoveAll(workspace) error = %v", err)
				}

				workspaces, err := resolver.List(ctx)
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if len(workspaces) != 0 {
					t.Fatalf("List() = %#v, want missing workspace pruned", workspaces)
				}
				if _, err := db.GetWorkspace(ctx, h.workspaceID); !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
					t.Fatalf("GetWorkspace() error = %v, want %v", err, workspacepkg.ErrWorkspaceNotFound)
				}
				if _, err := os.Stat(session.SessionDir()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Stat(session dir after prune) error = %v, want os.ErrNotExist", err)
				}
				for _, path := range []string{attachmentPath, orphanPath} {
					if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("Stat(workspace attachment %q) error = %v, want os.ErrNotExist", path, err)
					}
				}
				for _, table := range []string{"permission_log", "token_stats"} {
					var count int
					if err := db.DB().QueryRowContext(
						ctx,
						"SELECT COUNT(*) FROM "+table+" WHERE session_id = ?",
						session.ID,
					).Scan(&count); err != nil {
						t.Fatalf("Count %s error = %v", table, err)
					}
					if count != 0 {
						t.Fatalf("%s rows after workspace prune = %d, want 0", table, count)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func withDefaultQueryStoreRuntime() Option {
	return func(manager *Manager) {
		manager.openQueryStore = nil
		manager.queryStoreExplicit = false
	}
}

func shutdownQueryStoreRuntimeForTest(t *testing.T, manager *Manager) {
	t.Helper()
	t.Cleanup(func() {
		if err := manager.shutdownQueryStoreRuntime(testutil.Context(t)); err != nil {
			t.Errorf("shutdownQueryStoreRuntime() error = %v", err)
		}
	})
}

func findSessionDeleteTombstone(t *testing.T, sessionsDir string) (string, string) {
	t.Helper()
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatalf("ReadDir(sessions) error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
			return filepath.Join(sessionsDir, entry.Name()), entry.Name()
		}
	}
	t.Fatal("session deletion tombstone was not found")
	return "", ""
}

func assertNoSessionDeleteTombstones(t *testing.T, sessionsDir string) {
	t.Helper()
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatalf("ReadDir(sessions) error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
			t.Fatalf("unexpected session deletion tombstone %q", entry.Name())
		}
	}
}

func writeSessionAttachmentFixture(t *testing.T, h *harness, sessionID string, contents string) string {
	t.Helper()
	path := filepath.Join(h.homePaths.SessionAttachmentsDir, h.workspaceID, sessionID, "attachment.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(attachment fixture) error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(attachment fixture) error = %v", err)
	}
	return path
}

func assertDeletedUserSessionCatalogTruth(
	t *testing.T,
	catalog *recordingSessionCatalog,
	events <-chan CatalogEvent,
	workspaceID string,
	sessionID string,
) {
	t.Helper()

	listed, err := catalog.ListSessions(testutil.Context(t), store.SessionListQuery{
		WorkspaceID: workspaceID,
		SessionType: string(SessionTypeUser),
	})
	if err != nil {
		t.Fatalf("ListSessions(after delete) error = %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("ListSessions(after delete) = %#v, want exact count 0", listed)
	}
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("catalog event stream closed before delete event")
			}
			if event.Kind != CatalogEventDeleted {
				continue
			}
			if event.WorkspaceID != workspaceID || event.SessionID != sessionID {
				t.Fatalf("deleted catalog event = %#v, want workspace %q session %q", event, workspaceID, sessionID)
			}
			return
		default:
			t.Fatalf("deleted catalog event missing for session %q", sessionID)
		}
	}
}
