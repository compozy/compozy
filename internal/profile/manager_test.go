package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestManagerProfileLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve identity through rename archive unarchive and delete", func(t *testing.T) {
		t.Parallel()

		manager, database, home := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{
			Name: "marketing", Color: "#FF7F3A", Icon: "megaphone",
			Activate: &Lens{Kind: SelectionLensGlobal},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if len(created.ID) != 26 || created.State != StateActive || created.Color != "#ff7f3a" {
			t.Fatalf("Create() = %#v, want active persisted identity with ULID", created)
		}
		if _, err := os.Stat(filepath.Join(home.ProfilesDir, "marketing")); err != nil {
			t.Fatalf("created profile directory error = %v", err)
		}

		emoji := "🚀"
		updated, err := manager.UpdateIdentity(ctx, created.Name, IdentityPatch{Emoji: &emoji})
		if err != nil {
			t.Fatalf("UpdateIdentity() error = %v", err)
		}
		if updated.Emoji != emoji || updated.Icon != "" || updated.ID != created.ID {
			t.Fatalf("UpdateIdentity() = %#v, want emoji replacement and stable id", updated)
		}

		renamePlan, err := manager.PrepareRename(ctx, "marketing", "growth")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		if _, err := manager.Rename(ctx, "marketing", RenameOptions{
			NewName: "growth", Repos: RepoChoice{None: true}, PlanRevision: renamePlan.Revision,
		}); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		renamed, err := manager.GetByName(ctx, "growth")
		if err != nil {
			t.Fatalf("GetByName(renamed) error = %v", err)
		}
		if renamed.ID != created.ID {
			t.Fatalf("renamed ID = %q, want %q", renamed.ID, created.ID)
		}
		if _, err := os.Stat(filepath.Join(home.ProfilesDir, "growth")); err != nil {
			t.Fatalf("renamed profile directory error = %v", err)
		}

		archivePlan, err := manager.PrepareArchive(ctx, "growth")
		if err != nil {
			t.Fatalf("PrepareArchive() error = %v", err)
		}
		if _, err := manager.Archive(ctx, "growth", archivePlan.Revision); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if _, err := manager.Archive(ctx, "growth", archivePlan.Revision); err != nil {
			t.Fatalf("Archive(idempotent) error = %v", err)
		}
		fallback, err := manager.Resolve(ctx, ResolveInput{Lens: Lens{Kind: SelectionLensGlobal}})
		if err != nil {
			t.Fatalf("Resolve(archived remembered) error = %v", err)
		}
		if fallback.Profile.Name != "default" || fallback.Note != ResolutionNoteArchivedRememberedFallback {
			t.Fatalf("Resolve(archived remembered) = %#v", fallback)
		}

		unarchived, err := manager.Unarchive(ctx, "growth")
		if err != nil {
			t.Fatalf("Unarchive() error = %v", err)
		}
		if unarchived.Profile.State != StateActive {
			t.Fatalf("Unarchive().Profile.State = %q, want active", unarchived.Profile.State)
		}
		if _, err := manager.Unarchive(ctx, "growth"); err != nil {
			t.Fatalf("Unarchive(idempotent) error = %v", err)
		}

		deletePlan, err := manager.PrepareDelete(ctx, "growth")
		if err != nil {
			t.Fatalf("PrepareDelete() error = %v", err)
		}
		deleted, err := manager.Delete(ctx, "growth", deletePlan.Revision)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.SweptSelections != 1 {
			t.Fatalf("Delete().SweptSelections = %d, want 1", deleted.SweptSelections)
		}
		if _, err := manager.GetByName(ctx, "growth"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByName(deleted) error = %v, want ErrNotFound", err)
		}
		var selections int
		if err := database.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM profile_selections WHERE profile_id = ?`, created.ID).Scan(&selections); err != nil {
			t.Fatalf("count swept selections error = %v", err)
		}
		if selections != 0 {
			t.Fatalf("selection rows = %d, want 0", selections)
		}
	})

	t.Run("Should record lifecycle events with snake_case payload keys", func(t *testing.T) {
		t.Parallel()

		recorder := &recordingEventRecorder{}
		manager := newTestManagerWithRecorder(t, recorder)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{
			Name: "marketing", Color: "#ff7f3a", Icon: "megaphone",
			Activate: &Lens{Kind: SelectionLensGlobal},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		names := recorder.names()
		if !slices.Contains(names, "profile.created") || !slices.Contains(names, "profile.selection_changed") {
			t.Fatalf("recorded events = %v, want profile.created and profile.selection_changed", names)
		}

		selection, ok := recorder.find("profile.selection_changed")
		if !ok {
			t.Fatalf("recorded events = %v, want profile.selection_changed", names)
		}
		// The marshaled form is what reaches clients as the `content` field of a
		// log event, so the wire keys are the contract — not the Go field names.
		payload, err := json.Marshal(selection)
		if err != nil {
			t.Fatalf("Marshal(event) error = %v", err)
		}
		var wire map[string]string
		if err := json.Unmarshal(payload, &wire); err != nil {
			t.Fatalf("Unmarshal(event) error = %v", err)
		}
		if wire["name"] != "profile.selection_changed" {
			t.Fatalf("event name = %q, want %q", wire["name"], "profile.selection_changed")
		}
		if wire["profile_id"] != created.ID {
			t.Fatalf("event profile_id = %q, want %q", wire["profile_id"], created.ID)
		}
		if wire["profile_name"] != "marketing" {
			t.Fatalf("event profile_name = %q, want %q", wire["profile_name"], "marketing")
		}
		// An empty error must not reach the wire at all — clients branch on presence.
		if _, present := wire["error"]; present {
			t.Fatalf("event payload = %v, want no error key when the event succeeded", wire)
		}
		for key := range wire {
			if strings.ToLower(key) != key {
				t.Fatalf("event payload key %q is not snake_case: %v", key, wire)
			}
		}
	})

	t.Run("Should reject invalid reserved duplicate and conflicting identity inputs", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		for _, name := range []string{"Marketing", "mkt space", "-x", strings.Repeat("a", 33)} {
			if _, err := manager.Create(ctx, CreateInput{Name: name}); !errors.Is(err, ErrNameInvalid) {
				t.Fatalf("Create(%q) error = %v, want ErrNameInvalid", name, err)
			}
		}
		for _, name := range []string{"default", "all", "global"} {
			if _, err := manager.Create(ctx, CreateInput{Name: name}); !errors.Is(err, ErrNameReserved) {
				t.Fatalf("Create(%q) error = %v, want ErrNameReserved", name, err)
			}
		}
		if _, err := manager.Create(ctx, CreateInput{Name: "dev", Icon: "code", Emoji: "🧑‍💻"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Create(conflicting symbol) error = %v, want ErrInvalidInput", err)
		}
		if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); err != nil {
			t.Fatalf("Create(dev) error = %v", err)
		}
		if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); !errors.Is(err, ErrNameTaken) {
			t.Fatalf("Create(duplicate) error = %v, want ErrNameTaken", err)
		}
		invalidColor := "red"
		if _, err := manager.UpdateIdentity(ctx, "dev", IdentityPatch{Color: &invalidColor}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("UpdateIdentity(invalid color) error = %v, want ErrInvalidInput", err)
		}
		if _, err := manager.PrepareArchive(ctx, "default"); !errors.Is(err, ErrPermanent) {
			t.Fatalf("PrepareArchive(default) error = %v, want ErrPermanent", err)
		}
	})

	t.Run("Should reject stale plans without committing", func(t *testing.T) {
		t.Parallel()

		manager, _, home := newTestManager(t)
		ctx := testutil.Context(t)
		if _, err := manager.Create(ctx, CreateInput{Name: "dev"}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		plan, err := manager.PrepareRename(ctx, "dev", "engineering")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		changed := filepath.Join(home.ProfilesDir, "dev", "changed.txt")
		if err := os.WriteFile(changed, []byte("revision changed"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err = manager.Rename(ctx, "dev", RenameOptions{
			NewName: "engineering", Repos: RepoChoice{None: true}, PlanRevision: plan.Revision,
		})
		if !errors.Is(err, ErrPlanStale) {
			t.Fatalf("Rename(stale plan) error = %v, want ErrPlanStale", err)
		}
		if _, err := manager.GetByName(ctx, "dev"); err != nil {
			t.Fatalf("GetByName(dev after stale plan) error = %v", err)
		}
	})

	t.Run("Should refuse archive while a notification delivery permit is held", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		created, err := manager.Create(ctx, CreateInput{Name: "alerts"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		plan, err := manager.PrepareArchive(ctx, created.Name)
		if err != nil {
			t.Fatalf("PrepareArchive() error = %v", err)
		}
		now := formatTimestamp(time.Now())
		if _, err := database.DB().ExecContext(ctx, `
			INSERT INTO notification_delivery_permits
			(scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id, delivery_id, acquired_at)
			VALUES ('global', ?, '', 'terminal', 'task-events', 'task-1', 'delivery-held', ?)`,
			created.ID, now,
		); err != nil {
			t.Fatalf("insert delivery permit error = %v", err)
		}
		if _, err := manager.Archive(ctx, created.Name, plan.Revision); !errors.Is(err, ErrDeliveriesInFlight) {
			t.Fatalf("Archive(with permit) error = %v, want ErrDeliveriesInFlight", err)
		}
		stored, err := manager.GetByName(ctx, created.Name)
		if err != nil {
			t.Fatalf("GetByName() error = %v", err)
		}
		if stored.State != StateActive {
			t.Fatalf("profile state after refused archive = %q, want active", stored.State)
		}
	})
}

func TestManagerSelectionResolutionAndAvailability(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve flag env remembered default and session in priority order", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		dev, err := manager.Create(ctx, CreateInput{Name: "dev"})
		if err != nil {
			t.Fatalf("Create(dev) error = %v", err)
		}
		marketing, err := manager.Create(ctx, CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatalf("Create(marketing) error = %v", err)
		}
		lens := Lens{Kind: SelectionLensWorkspace, WorkspaceID: "ws-1"}
		if err := manager.PutSelection(ctx, Selection{Lens: lens.Kind, WorkspaceID: lens.WorkspaceID, ProfileID: marketing.ID}); err != nil {
			t.Fatalf("PutSelection() error = %v", err)
		}

		cases := []struct {
			name string
			in   ResolveInput
			want string
			src  ResolutionSource
		}{
			{name: "flag", in: ResolveInput{Flag: "dev", Env: "marketing", Lens: lens}, want: "dev", src: ResolutionSourceFlag},
			{name: "env", in: ResolveInput{Env: "dev", Lens: lens}, want: "dev", src: ResolutionSourceEnv},
			{name: "remembered", in: ResolveInput{Lens: lens}, want: "marketing", src: ResolutionSourceRemembered},
			{name: "default", in: ResolveInput{Lens: Lens{Kind: SelectionLensGlobal}}, want: "default", src: ResolutionSourceDefault},
			{name: "session", in: ResolveInput{Flag: "dev", Env: "dev", SessionProfileID: dev.ID, Lens: lens}, want: "dev", src: ResolutionSourceSession},
		}
		for _, testCase := range cases {
			testCase := testCase
			t.Run("Should resolve "+testCase.name, func(t *testing.T) {
				t.Parallel()

				got, err := manager.Resolve(ctx, testCase.in)
				if err != nil {
					t.Fatalf("Resolve() error = %v", err)
				}
				if got.Profile.Name != testCase.want || got.Source != testCase.src {
					t.Fatalf("Resolve() = %#v, want name %q source %q", got, testCase.want, testCase.src)
				}
			})
		}
		if _, err := manager.Resolve(ctx, ResolveInput{Flag: "marketing", SessionProfileID: dev.ID, Lens: lens}); !errors.Is(err, ErrSessionConflict) {
			t.Fatalf("Resolve(session conflict) error = %v, want ErrSessionConflict", err)
		}
		if _, err := manager.Resolve(ctx, ResolveInput{Flag: "missing", Lens: lens}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Resolve(missing flag) error = %v, want ErrNotFound", err)
		}
		if err := manager.PutSelection(ctx, Selection{Lens: SelectionLensGlobal, ProfileID: "@all"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("PutSelection(@all) error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("Should make pending and failed lifecycle owners unavailable until done", func(t *testing.T) {
		t.Parallel()

		manager, database, _ := newTestManager(t)
		ctx := testutil.Context(t)
		dev, err := manager.Create(ctx, CreateInput{Name: "dev"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		insertLifecycleOperation(t, ctx, database, "op_availability", dev.ID, "rename", "dev", "engineering", "failed")
		if _, err := manager.Resolve(ctx, ResolveInput{Flag: "dev", Lens: Lens{Kind: SelectionLensGlobal}}); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("Resolve(unavailable) error = %v, want ErrUnavailable", err)
		}
		if err := manager.PutSelection(ctx, Selection{Lens: SelectionLensGlobal, ProfileID: dev.ID}); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("PutSelection(unavailable) error = %v, want ErrUnavailable", err)
		}
		if _, err := database.DB().ExecContext(ctx, `UPDATE profile_lifecycle_ops SET status = 'done', completed_at = ? WHERE id = ?`, formatTimestamp(time.Now()), "op_availability"); err != nil {
			t.Fatalf("complete lifecycle operation error = %v", err)
		}
		if _, err := manager.Resolve(ctx, ResolveInput{Flag: "dev", Lens: Lens{Kind: SelectionLensGlobal}}); err != nil {
			t.Fatalf("Resolve(done operation) error = %v", err)
		}
	})
}

func TestManagerRecoveryAndReservation(t *testing.T) {
	t.Parallel()

	t.Run("Should converge an applied filesystem step exactly once", func(t *testing.T) {
		t.Parallel()

		manager, database, home := newTestManager(t)
		ctx := testutil.Context(t)
		profileID := strings.Repeat("R", 26)
		now := formatTimestamp(time.Now())
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO profiles (id, name, color, icon, state, created_at) VALUES (?, 'recovered', '#8e8eb5', 'circle', 'active', ?)`, profileID, now); err != nil {
			t.Fatalf("insert recovery profile error = %v", err)
		}
		insertLifecycleOperation(t, ctx, database, "op_recovery", profileID, "create", "", "recovered", "applied")
		path := filepath.Join(home.ProfilesDir, "recovered")
		if _, err := database.DB().ExecContext(ctx, `INSERT INTO profile_lifecycle_op_steps (op_id, seq, action, path_new, status, updated_at) VALUES ('op_recovery', 0, 'mkdir_profile', ?, 'pending', ?)`, path, now); err != nil {
			t.Fatalf("insert recovery step error = %v", err)
		}
		if err := manager.Recover(ctx); err != nil {
			t.Fatalf("Recover() error = %v", err)
		}
		if err := manager.Recover(ctx); err != nil {
			t.Fatalf("Recover(idempotent) error = %v", err)
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("recovered path stat = %#v, %v", info, err)
		}
		var status string
		if err := database.DB().QueryRowContext(ctx, `SELECT status FROM profile_lifecycle_ops WHERE id = 'op_recovery'`).Scan(&status); err != nil {
			t.Fatalf("read recovery status error = %v", err)
		}
		if status != opStatusDone {
			t.Fatalf("recovery status = %q, want %q", status, opStatusDone)
		}
	})

	t.Run("Should allow exactly one concurrent creator for a name", func(t *testing.T) {
		t.Parallel()

		manager, _, _ := newTestManager(t)
		ctx := testutil.Context(t)
		start := make(chan struct{})
		errs := make([]error, 2)
		var wait sync.WaitGroup
		wait.Add(2)
		for index := range errs {
			index := index
			go func() {
				defer wait.Done()
				<-start
				_, errs[index] = manager.Create(ctx, CreateInput{Name: "racing"})
			}()
		}
		close(start)
		wait.Wait()
		successes, conflicts := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrNameTaken):
				conflicts++
			default:
				t.Fatalf("Create(racing) error = %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent creates successes=%d conflicts=%d, want 1/1", successes, conflicts)
		}
	})
}

func newTestManager(t *testing.T) (*Manager, *globaldb.GlobalDB, compozyconfig.HomePaths) {
	t.Helper()

	home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	database, err := globaldb.OpenGlobalDB(testutil.Context(t), home.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	manager, err := NewManager(WithStore(database), WithHomePaths(home))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, database, home
}

// recordingEventRecorder captures emitted lifecycle events so a test can assert
// the payload that reaches the durable event store and, from there, the logs
// stream.
type recordingEventRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingEventRecorder) RecordProfileEvent(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingEventRecorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.events))
	for _, event := range r.events {
		names = append(names, event.Name)
	}
	return names
}

func (r *recordingEventRecorder) find(name string) (Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range r.events {
		if event.Name == name {
			return event, true
		}
	}
	return Event{}, false
}

func newTestManagerWithRecorder(t *testing.T, recorder EventRecorder) *Manager {
	t.Helper()

	home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	database, err := globaldb.OpenGlobalDB(testutil.Context(t), home.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	manager, err := NewManager(WithStore(database), WithHomePaths(home), WithEventRecorder(recorder))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func insertLifecycleOperation(
	t *testing.T,
	ctx context.Context,
	database *globaldb.GlobalDB,
	opID, profileID, kind, oldName, newName, status string,
) {
	t.Helper()

	now := formatTimestamp(time.Now())
	completedAt := any(nil)
	if status == opStatusDone {
		completedAt = now
	}
	if _, err := database.DB().ExecContext(ctx, `
		INSERT INTO profile_lifecycle_ops
		(id, kind, profile_id, old_name, new_name, plan_revision, status, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), 'test-revision', ?, ?, ?, ?)`,
		opID, kind, profileID, oldName, newName, status, now, now, completedAt,
	); err != nil {
		t.Fatalf("insert lifecycle operation %q error = %v", opID, err)
	}
}
