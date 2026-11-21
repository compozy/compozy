package session

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/store"
)

// archiveStoppedFixture is one stopped thread (source, side chat, child) and
// one thread whose stopped root still has a running child.
type archiveStoppedFixture struct {
	h         *deriveHarness
	profileID string
	source    string
	side      string
	child     string
	busy      string
}

func newArchiveStoppedFixture(t *testing.T) archiveStoppedFixture {
	t.Helper()
	h := newDeriveHarness(t)
	ctx := t.Context()
	source := h.newDeriveSource(t)
	side := h.openSide(t, source, "archive-stopped-side")
	child, err := h.manager.Create(ctx, &CreateOpts{AgentName: "coder", Workspace: h.workspaceID,
		Lineage: &store.SessionLineage{ParentSessionID: source.ID}})
	if err != nil {
		t.Fatal(err)
	}
	busy, err := h.manager.Create(ctx, &CreateOpts{AgentName: "coder", Workspace: h.workspaceID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.manager.Create(ctx, &CreateOpts{AgentName: "coder", Workspace: h.workspaceID,
		Lineage: &store.SessionLineage{ParentSessionID: busy.ID}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{side.ID, child.ID, source.ID, busy.ID} {
		if err := h.manager.Stop(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	info, err := h.manager.Status(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	profileID := info.ProfileID
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	return archiveStoppedFixture{
		h: h, profileID: profileID, source: source.ID, side: side.ID, child: child.ID, busy: busy.ID,
	}
}

func (f *archiveStoppedFixture) run(t *testing.T, dryRun bool, caller string) ArchiveStoppedReport {
	t.Helper()
	report, err := f.h.manager.ArchiveStopped(t.Context(), ArchiveStoppedRequest{
		ProfileID: f.profileID, WorkspaceID: f.h.workspaceID, DryRun: dryRun, CallerSessionID: caller,
	})
	if err != nil {
		t.Fatalf("ArchiveStopped() error = %v", err)
	}
	return report
}

func (f *archiveStoppedFixture) archivedAt(t *testing.T, id string) bool {
	t.Helper()
	at, err := f.h.db.SessionArchivedAt(t.Context(), f.h.workspaceID, id)
	if err != nil {
		t.Fatal(err)
	}
	return at != nil
}

func TestArchiveStopped(t *testing.T) {
	t.Parallel()

	// Invariant: the thread is the unit — a stopped thread archives whole, root
	// first, and a thread with any running member is skipped whole.
	t.Run("Should archive a stopped thread whole and skip a thread with a running child", func(t *testing.T) {
		t.Parallel()
		f := newArchiveStoppedFixture(t)
		report := f.run(t, false, "")

		if len(report.Archived) != 1 || report.Archived[0].RootID != f.source {
			t.Fatalf("Archived = %+v, want the source thread only", report.Archived)
		}
		ids := report.Archived[0].SessionIDs
		if len(ids) == 0 || ids[0] != f.source || !slices.Contains(ids, f.side) || !slices.Contains(ids, f.child) {
			t.Fatalf("SessionIDs = %v, want source first with side and child", ids)
		}
		for _, id := range ids {
			if !f.archivedAt(t, id) {
				t.Fatalf("member %s stayed unarchived", id)
			}
		}
		if want := map[ArchiveStoppedSkipReason]int{ArchiveStoppedSkipRunning: 1}; !reflect.DeepEqual(
			report.SkipCounts, want,
		) {
			t.Fatalf("SkipCounts = %v, want %v", report.SkipCounts, want)
		}
		if len(report.Skipped) != 1 || report.Skipped[0].RootID != f.busy || f.archivedAt(t, f.busy) {
			t.Fatalf("Skipped = %+v, want the busy thread left unarchived", report.Skipped)
		}
		if len(report.Failures) != 0 {
			t.Fatalf("Failures = %+v, want none", report.Failures)
		}
	})

	// Invariant: a dry run reports exactly what the real run then does, with no side effects.
	t.Run("Should report in a dry run what the real run archives without archiving", func(t *testing.T) {
		t.Parallel()
		f := newArchiveStoppedFixture(t)
		events, cancel, err := f.h.manager.SubscribeSessionCatalogEvents(t.Context(),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true})
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()

		dry := f.run(t, true, "")
		if !dry.DryRun {
			t.Fatal("dry run report DryRun = false")
		}
		for _, id := range []string{f.source, f.side, f.child} {
			if f.archivedAt(t, id) {
				t.Fatalf("dry run archived %s", id)
			}
		}
		select {
		case event := <-events:
			t.Fatalf("dry run published a catalog event: %+v", event)
		default:
		}

		actual := f.run(t, false, "")
		dry.DryRun = false
		if len(actual.Archived) == 0 || !reflect.DeepEqual(dry, actual) {
			t.Fatalf("dry run report = %+v, real run report = %+v", dry, actual)
		}
	})

	// Invariant: the operation never archives the calling session's thread.
	t.Run("Should skip the caller's own thread", func(t *testing.T) {
		t.Parallel()
		f := newArchiveStoppedFixture(t)
		report := f.run(t, false, f.child)

		if len(report.Archived) != 0 || report.SkipCounts[ArchiveStoppedSkipCaller] != 1 {
			t.Fatalf("report = %+v, want the caller thread skipped as caller", report)
		}
		if f.archivedAt(t, f.source) || f.archivedAt(t, f.child) {
			t.Fatal("the caller's thread was archived")
		}
	})

	t.Run("Should refuse a request without exactly one scope", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		for _, req := range []ArchiveStoppedRequest{
			{ProfileID: store.DefaultProfileID},
			{ProfileID: store.DefaultProfileID, WorkspaceID: "ws", AllWorkspaces: true},
			{WorkspaceID: "ws"},
		} {
			if _, err := h.manager.ArchiveStopped(t.Context(), req); !errors.Is(err, ErrArchiveStoppedInvalidRequest) {
				t.Fatalf("ArchiveStopped(%+v) error = %v, want ErrArchiveStoppedInvalidRequest", req, err)
			}
		}
	})
}
