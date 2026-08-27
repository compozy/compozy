package journal

// Suite: durable terminal journal
// Invariant: accepted command rows persist once and reads never escape their workspace/profile scope.
// Boundary IN: journal assembly, workspacedb SQLite, retained artifact filesystem.
// Boundary OUT: HTTP/UDS projections and browser replay, owned by later terminal task suites.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/workspacedb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestService(t *testing.T) {
	t.Run("Should fail closed when authenticated marker facts have no terminal lane", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		err := service.ConsumeMarkerFacts(ctx, terminalpkg.Info{
			ID: "term-missing", WS: workspaceID, ProfileID: "profile-a",
		}, []terminalpkg.MarkerFacts{{Kind: "S", Command: "pwd", Cwd: "/workspace"}})
		if !errors.Is(err, terminalpkg.ErrJournalUnavailable) {
			t.Fatalf("ConsumeMarkerFacts(without lane) error = %v, want ErrJournalUnavailable", err)
		}
	})

	t.Run("Should append exact rows with secret scrubbing and recording linkage", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		exitCode := 1
		duration := int64(84)
		terminalID := terminalpkg.ID("term-exact")
		row := terminalpkg.CommandRow{
			ID: "cmd-exact", TerminalID: &terminalID, ProfileID: "profile-a",
			Actor:   terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent, ID: "agent-a", ProfileID: "profile-a"},
			Command: "mysql -pHUNTER2 --api-key=secret-value", Cwd: "/workspace",
			StartedAt: time.UnixMilli(1000), DurationMs: &duration, ExitCode: &exitCode,
			ExitCause: "exited", DetectedBy: "exact", Approval: "approved_once",
		}
		if err := service.Record(ctx, workspaceID, row); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		for _, outside := range []terminalpkg.CommandRow{
			{
				ID: "cmd-before-recording", TerminalID: &terminalID, ProfileID: "profile-a",
				Actor: row.Actor, Command: "before", Cwd: row.Cwd, StartedAt: time.UnixMilli(800),
				ExitCause: "exited", DetectedBy: "exact", Approval: "approved_once",
			},
			{
				ID: "cmd-after-recording", TerminalID: &terminalID, ProfileID: "profile-a",
				Actor: row.Actor, Command: "after", Cwd: row.Cwd, StartedAt: time.UnixMilli(2100),
				ExitCause: "exited", DetectedBy: "exact", Approval: "approved_once",
			},
		} {
			if err := service.Record(ctx, workspaceID, outside); err != nil {
				t.Fatalf("Record(%q) error = %v", outside.ID, err)
			}
		}
		page, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(page.Entries) != 3 {
			t.Fatalf("len(entries) = %d, want 3", len(page.Entries))
		}
		got := commandRowByID(t, page, row.ID)
		if got.ID != row.ID || got.ExitCode == nil || *got.ExitCode != 1 || got.DetectedBy != "exact" ||
			got.Approval != "approved_once" || got.Actor.Kind != terminalpkg.ActorKindAgent {
			t.Fatalf("Query() row = %#v, want exact persisted fields", got)
		}
		if strings.Contains(got.Command, "HUNTER2") || strings.Contains(got.Command, "secret-value") {
			t.Fatalf("Query() command leaked secret: %q", got.Command)
		}

		recordingData := []byte("{\"version\":2,\"width\":80,\"height\":24}\n")
		stoppedAt := time.UnixMilli(2000)
		recording := terminalpkg.RecordingRef{
			ID: "rec-1", TerminalID: terminalID, ProfileID: "profile-a",
			StartedAt: time.UnixMilli(900), StoppedAt: &stoppedAt, ExpiresAt: time.UnixMilli(9000),
		}
		persisted, err := service.PersistRecording(ctx, workspaceID, terminalID, recording, recordingData)
		if err != nil {
			t.Fatalf("PersistRecording() error = %v", err)
		}
		if persisted.Path == "" || persisted.Digest == "" || persisted.Bytes != int64(len(recordingData)) {
			t.Fatalf("PersistRecording() = %#v, want retained metadata", persisted)
		}
		linked, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query(linked) error = %v", err)
		}
		linkedInside := commandRowByID(t, linked, row.ID)
		if linkedInside.RecordingID == nil || *linkedInside.RecordingID != "rec-1" {
			t.Fatalf("inside recording_id = %v, want rec-1", linkedInside.RecordingID)
		}
		if before := commandRowByID(t, linked, "cmd-before-recording"); before.RecordingID != nil {
			t.Fatalf("before recording_id = %v, want nil", before.RecordingID)
		}
		if after := commandRowByID(t, linked, "cmd-after-recording"); after.RecordingID != nil {
			t.Fatalf("after recording_id = %v, want nil", after.RecordingID)
		}

		secondStoppedAt := time.UnixMilli(2200)
		if _, err := service.PersistRecording(ctx, workspaceID, terminalID, terminalpkg.RecordingRef{
			ID: "rec-2", TerminalID: terminalID, ProfileID: "profile-a",
			StartedAt: time.UnixMilli(1500), StoppedAt: &secondStoppedAt, ExpiresAt: time.UnixMilli(9000),
		}, recordingData); err != nil {
			t.Fatalf("PersistRecording(second) error = %v", err)
		}
		relinked, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query(relinked) error = %v", err)
		}
		if id := commandRowByID(t, relinked, row.ID).RecordingID; id == nil || *id != "rec-1" {
			t.Fatalf("overlap recording_id = %v, want original rec-1", id)
		}
		if id := commandRowByID(t, relinked, "cmd-after-recording").RecordingID; id == nil || *id != "rec-2" {
			t.Fatalf("later recording_id = %v, want rec-2", id)
		}
	})

	t.Run("Should remove the workspace database and retained terminal files together", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		terminalID := terminalpkg.ID("term-remove")
		row := terminalpkg.CommandRow{
			ID: "cmd-remove", TerminalID: &terminalID, ProfileID: "profile-a",
			Actor:   terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			Command: "pwd", Cwd: "/workspace", StartedAt: time.UnixMilli(1000),
			ExitCause: "exited", DetectedBy: "exact", Approval: "human",
		}
		if err := service.Record(ctx, workspaceID, row); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		artifact, err := service.WriteArtifact(
			ctx, workspaceID, "profile-a", row.ID, &terminalID, []byte("artifact"), time.UnixMilli(9000),
		)
		if err != nil {
			t.Fatalf("WriteArtifact() error = %v", err)
		}
		stoppedAt := time.UnixMilli(2000)
		recording, err := service.PersistRecording(ctx, workspaceID, terminalID, terminalpkg.RecordingRef{
			ID: "rec-remove", TerminalID: terminalID, ProfileID: "profile-a",
			StartedAt: time.UnixMilli(900), StoppedAt: &stoppedAt, ExpiresAt: time.UnixMilli(9000),
		}, []byte("recording"))
		if err != nil {
			t.Fatalf("PersistRecording() error = %v", err)
		}
		db, err := service.databases.Open(ctx, workspaceID)
		if err != nil {
			t.Fatalf("databases.Open() error = %v", err)
		}
		dbPath := db.Path()
		if err := service.RemoveWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("RemoveWorkspace() error = %v", err)
		}
		for _, path := range []string{artifact.Path, recording.Path, dbPath} {
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want %v", path, err, os.ErrNotExist)
			}
		}
	})

	t.Run("Should preserve signaled and unknown exit causes without fabricating a code", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		signal := "TERM"
		for index, row := range []terminalpkg.CommandRow{
			{
				ID: "cmd-signaled", ProfileID: "profile-a",
				Actor: terminalpkg.Actor{
					Kind: terminalpkg.ActorKindAgent, ID: "agent-a", ProfileID: "profile-a",
				},
				Command: "server", Cwd: "/workspace", StartedAt: time.UnixMilli(2),
				ExitSignal: &signal, ExitCause: "signaled", DetectedBy: "exact", Approval: "approved_once",
			},
			{
				ID: "cmd-unknown", ProfileID: "profile-a",
				Actor: terminalpkg.Actor{
					Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
				},
				Command: "interactive", Cwd: "/workspace", StartedAt: time.UnixMilli(1),
				ExitCause: "unknown", DetectedBy: "idle", Approval: "human",
			},
		} {
			if err := service.Record(ctx, workspaceID, row); err != nil {
				t.Fatalf("Record(%d) error = %v", index, err)
			}
		}
		page, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(page.Entries) != 2 || page.Entries[0].ExitCause != "signaled" ||
			page.Entries[0].ExitSignal == nil || *page.Entries[0].ExitSignal != signal ||
			page.Entries[0].ExitCode != nil || page.Entries[1].ExitCause != "unknown" ||
			page.Entries[1].ExitCode != nil {
			t.Fatalf("exit-cause rows = %#v, want signaled then unknown without codes", page.Entries)
		}
	})

	t.Run("Should page by start time and id without overlap", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		for index := range 55 {
			row := terminalpkg.CommandRow{
				ID: commandIDForIndex(index), ProfileID: "profile-a",
				Actor:   terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
				Command: "pwd", Cwd: "/workspace", StartedAt: time.UnixMilli(int64(index / 2)),
				ExitCause: "exited", DetectedBy: "exact", Approval: "human",
			}
			if err := service.Record(ctx, workspaceID, row); err != nil {
				t.Fatalf("Record(%d) error = %v", index, err)
			}
		}
		first, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query(first) error = %v", err)
		}
		if len(first.Entries) != defaultPageLimit || first.Next == "" {
			t.Fatalf("first page len/next = %d/%q, want 50/non-empty", len(first.Entries), first.Next)
		}
		for index := 1; index < len(first.Entries); index++ {
			previous, current := first.Entries[index-1], first.Entries[index]
			if previous.StartedAt.Before(current.StartedAt) ||
				previous.StartedAt.Equal(current.StartedAt) && previous.ID < current.ID {
				t.Fatalf("page order at %d = %s/%s then %s/%s", index,
					previous.StartedAt, previous.ID, current.StartedAt, current.ID)
			}
		}
		second, err := service.Query(
			ctx,
			workspaceID,
			store.ReadScope{ProfileID: "profile-a"},
			terminalpkg.Query{Cursor: first.Next},
		)
		if err != nil {
			t.Fatalf("Query(second) error = %v", err)
		}
		if len(second.Entries) != 5 || second.Next != "" {
			t.Fatalf("second page len/next = %d/%q, want 5/empty", len(second.Entries), second.Next)
		}
		seen := make(map[string]struct{}, 55)
		for _, row := range append(first.Entries, second.Entries...) {
			if _, exists := seen[row.ID]; exists {
				t.Fatalf("duplicate paged id %q", row.ID)
			}
			seen[row.ID] = struct{}{}
		}
		if _, err := service.Query(
			ctx,
			workspaceID,
			store.ReadScope{ProfileID: "profile-b"},
			terminalpkg.Query{Cursor: first.Next},
		); err == nil {
			t.Fatal("Query(cross-profile cursor) error = nil, want invalid cursor")
		}
		if _, err := service.Query(
			ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{Limit: maximumPageLimit + 1},
		); err == nil {
			t.Fatal("Query(over maximum limit) error = nil, want validation error")
		}
		if page, err := service.Query(
			ctx,
			workspaceID,
			store.ReadScope{},
			terminalpkg.Query{},
		); err == nil ||
			page != nil {
			t.Fatalf("Query(invalid scope) = %#v error=%v, want validation error", page, err)
		}
	})

	t.Run("Should retain a contained artifact and refuse another profile", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		row := terminalpkg.CommandRow{
			ID: "cmd-artifact", ProfileID: "profile-a",
			Actor:   terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			Command: "cat output", Cwd: "/workspace", StartedAt: time.UnixMilli(1),
			ExitCause: "exited", DetectedBy: "exact", Approval: "human",
		}
		if err := service.Record(ctx, workspaceID, row); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		contents := []byte("byte-exact artifact")
		ref, err := service.WriteArtifact(
			ctx, workspaceID, "profile-a", row.ID, nil, contents, time.Unix(3600, 0),
		)
		if err != nil {
			t.Fatalf("WriteArtifact() error = %v", err)
		}
		info, err := os.Stat(ref.Path)
		if err != nil {
			t.Fatalf("Stat(artifact) error = %v", err)
		}
		if info.Mode().Perm() != privateFileMode {
			t.Fatalf("artifact mode = %o, want %o", info.Mode().Perm(), privateFileMode)
		}
		digest := sha256.Sum256(contents)
		if filepath.Base(ref.Path) != hex.EncodeToString(digest[:])+".bin" {
			t.Fatalf("artifact path = %q, want content-addressed digest", ref.Path)
		}
		reader, err := service.Artifact(
			ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, ref.ArtifactID,
		)
		if err != nil {
			t.Fatalf("Artifact() error = %v", err)
		}
		got, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("Read/Close artifact errors = %v / %v", err, closeErr)
		}
		if !bytes.Equal(got, contents) {
			t.Fatalf("Artifact() = %q, want %q", got, contents)
		}
		if _, err := service.Artifact(
			ctx, workspaceID, store.ReadScope{ProfileID: "profile-b"}, ref.ArtifactID,
		); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Artifact(cross-profile) error = %v, want not-exist", err)
		}
	})

	t.Run("Should sweep shared content only after its last retained row expires", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		for _, commandID := range []string{"cmd-first", "cmd-second"} {
			if err := service.Record(ctx, workspaceID, terminalpkg.CommandRow{
				ID: commandID, ProfileID: "profile-a",
				Actor: terminalpkg.Actor{
					Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
				},
				Command: commandID, Cwd: "/workspace", StartedAt: time.UnixMilli(1),
				ExitCause: "exited", DetectedBy: "exact", Approval: "human",
			}); err != nil {
				t.Fatalf("Record(%q) error = %v", commandID, err)
			}
		}
		contents := []byte("shared retained bytes")
		first, err := service.WriteArtifact(
			ctx, workspaceID, "profile-a", "cmd-first", nil, contents, time.UnixMilli(1000),
		)
		if err != nil {
			t.Fatalf("WriteArtifact(first) error = %v", err)
		}
		second, err := service.WriteArtifact(
			ctx, workspaceID, "profile-a", "cmd-second", nil, contents, time.UnixMilli(3000),
		)
		if err != nil {
			t.Fatalf("WriteArtifact(second) error = %v", err)
		}
		if first.Path != second.Path {
			t.Fatalf("shared artifact paths = %q/%q, want equal", first.Path, second.Path)
		}
		if err := service.SweepExpired(ctx, workspaceID, time.UnixMilli(2000)); err != nil {
			t.Fatalf("SweepExpired(first) error = %v", err)
		}
		reader, err := service.Artifact(
			ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, second.ArtifactID,
		)
		if err != nil {
			t.Fatalf("Artifact(second after first sweep) error = %v", err)
		}
		if err := reader.Close(); err != nil {
			t.Fatalf("Close(second artifact) error = %v", err)
		}
		if err := service.SweepExpired(ctx, workspaceID, time.UnixMilli(4000)); err != nil {
			t.Fatalf("SweepExpired(second) error = %v", err)
		}
		if _, err := os.Stat(second.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(shared artifact) error = %v, want %v", err, os.ErrNotExist)
		}
	})

	t.Run(
		"Should degrade accepted input to one idle row unless an authenticated marker claims it [UT-086][UT-088]",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			service, workspaceID := newJournalTestService(ctx, t)
			actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
			info := terminalpkg.Info{
				ID: "term-idle", WS: workspaceID, ProfileID: "profile-a", Cwd: "/workspace", Controller: &actor,
			}
			events := make(chan terminalpkg.Event, 6)
			service.RegisterTerminal(ctx, info, func(bool) {}, func(event terminalpkg.Event) { events <- event })
			service.ObserveInput(info, actor, []byte("echo approximate\n"))
			service.ObserveOutput(info)
			idleRow := waitForJournalRows(ctx, t, service, workspaceID, 1).Entries[0]
			if idleRow.Command != "echo approximate" || idleRow.DetectedBy != "idle" ||
				idleRow.Actor.Kind != terminalpkg.ActorKindHuman || idleRow.ExitCause != "unknown" {
				t.Fatalf("idle row = %#v", idleRow)
			}

			service.ObserveInput(info, actor, []byte("echo authenticated\n"))
			if err := service.ConsumeMarkerFacts(ctx, info, []terminalpkg.MarkerFacts{
				{Kind: "S", Command: "echo authenticated", Cwd: "/workspace"},
				{Kind: "F", Exit: new(0)},
			}); err != nil {
				t.Fatalf("ConsumeMarkerFacts() error = %v", err)
			}
			page := waitForJournalRows(ctx, t, service, workspaceID, 2)
			markerRows := 0
			idleRows := 0
			for _, row := range page.Entries {
				switch row.DetectedBy {
				case "marker":
					markerRows++
				case "idle":
					idleRows++
				}
			}
			if markerRows != 1 || idleRows != 1 {
				t.Fatalf("detection rows marker/idle = %d/%d, want 1/1", markerRows, idleRows)
			}
			stability := time.NewTimer(idleCommandDelay + 50*time.Millisecond)
			select {
			case <-ctx.Done():
				if !stability.Stop() {
					<-stability.C
				}
				t.Fatalf("idle duplicate stability window canceled: %v", context.Cause(ctx))
			case <-stability.C:
			}
			page = waitForJournalRows(ctx, t, service, workspaceID, 2)
			if len(page.Entries) != 2 {
				t.Fatalf("authenticated marker left a duplicate idle row: %#v", page.Entries)
			}

			started, finished := 0, 0
			for len(events) > 0 {
				event := <-events
				if event.Kind == terminalpkg.EventKindCommandStarted {
					started++
				}
				if event.Kind == terminalpkg.EventKindCommandFinished {
					finished++
				}
			}
			if started != 2 || finished != 2 {
				t.Fatalf("command event counts start/finish = %d/%d, want 2/2", started, finished)
			}
		},
	)

	t.Run("Should drain and remove a registered terminal lane when it closes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		info := terminalpkg.Info{
			ID: "term-close", WS: workspaceID, ProfileID: "profile-a", Cwd: "/workspace", Controller: &actor,
		}
		service.RegisterTerminal(ctx, info, func(bool) {}, func(terminalpkg.Event) {})
		if err := service.ConsumeMarkerFacts(ctx, info, []terminalpkg.MarkerFacts{
			{Kind: "S", Command: "pwd", Cwd: "/workspace"}, {Kind: "F", Exit: new(0)},
		}); err != nil {
			t.Fatalf("ConsumeMarkerFacts() error = %v", err)
		}
		if err := service.CloseTerminal(ctx, info); err != nil {
			t.Fatalf("CloseTerminal() error = %v", err)
		}
		if lane := service.lane(info); lane != nil {
			t.Fatal("CloseTerminal() retained the terminal lane")
		}
		page, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil || len(page.Entries) != 1 || page.Entries[0].Command != "pwd" {
			t.Fatalf("Query(after CloseTerminal) = %#v, error=%v", page, err)
		}
	})

	t.Run("Should fail every queued waiter when bounded lane close cancels an in-flight record", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			workspaceRoot := t.TempDir()
			identity, err := workspacepkg.EnsureIdentity(ctx, workspaceRoot)
			if err != nil {
				t.Fatalf("EnsureIdentity() error = %v", err)
			}
			recordStarted := make(chan struct{})
			storeErr := errors.New("store write aborted")
			var recordStartedOnce sync.Once
			pool, err := workspacedb.NewPool(func(
				ctx context.Context,
				_ string,
			) (workspacedb.ResolvedRoot, error) {
				recordStartedOnce.Do(func() { close(recordStarted) })
				<-ctx.Done()
				return workspacedb.ResolvedRoot{}, errors.Join(storeErr, context.Cause(ctx))
			})
			if err != nil {
				t.Fatalf("NewPool() error = %v", err)
			}
			service, err := New(Options{Databases: pool, HomeDir: t.TempDir()})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			actor := terminalpkg.Actor{
				Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
			}
			info := terminalpkg.Info{
				ID: "term-cancel-flush", WS: identity.WorkspaceID, ProfileID: "profile-a", Controller: &actor,
			}
			audit := make(chan bool, 2)
			service.RegisterTerminal(ctx, info, func(blocked bool) { audit <- blocked }, nil)
			lane := service.lane(info)
			if lane == nil {
				t.Fatal("registered lane = nil")
			}
			terminalID := info.ID
			row := func(id string) terminalpkg.CommandRow {
				return terminalpkg.CommandRow{
					ID: id, TerminalID: &terminalID, ProfileID: info.ProfileID, Actor: actor,
					Command: "pwd", Cwd: "/workspace", StartedAt: time.UnixMilli(1),
					ExitCause: "exited", DetectedBy: "marker", Approval: "human",
				}
			}
			firstResult := lane.enqueue(row("cmd-first"))
			<-recordStarted
			secondResult := lane.enqueue(row("cmd-second"))
			if !lane.reserve(1) {
				t.Fatal("reserve() = false, want retained reservation before cancellation")
			}
			closeCtx, cancelClose := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancelClose()
			closeDone := make(chan error, 1)
			go func() { closeDone <- lane.close(closeCtx) }()
			synctest.Wait()
			closeErr := <-closeDone
			if !errors.Is(closeErr, context.DeadlineExceeded) {
				t.Fatalf("lane.close() error = %v, want deadline exceeded", closeErr)
			}
			firstErr := receiveReadyJournalResult(t, "in-flight", firstResult)
			if !errors.Is(firstErr, context.DeadlineExceeded) || !errors.Is(firstErr, storeErr) ||
				!strings.Contains(firstErr.Error(), "cmd-first") || strings.Contains(firstErr.Error(), "cmd-second") {
				t.Fatalf("in-flight result error = %v, want cmd-first plus store and close failures only", firstErr)
			}
			secondErr := receiveReadyJournalResult(t, "queued", secondResult)
			if !errors.Is(secondErr, context.DeadlineExceeded) || errors.Is(secondErr, storeErr) ||
				!strings.Contains(secondErr.Error(), "cmd-second") || strings.Contains(secondErr.Error(), "cmd-first") {
				t.Fatalf("queued result error = %v, want cmd-second close failure only", secondErr)
			}
			select {
			case <-lane.done:
			default:
				t.Fatal("lane goroutine remained live after canceled close")
			}
			lane.mu.Lock()
			pending := lane.pending.Load()
			reservations := lane.reservations
			queued := len(lane.rows)
			blocked := lane.blocked
			lane.mu.Unlock()
			if pending != 0 || reservations != 0 || queued != 0 || !blocked {
				t.Fatalf(
					"lane teardown state = pending %d reservations %d queued %d blocked %t",
					pending, reservations, queued, blocked,
				)
			}
			select {
			case blockedState := <-audit:
				if !blockedState {
					t.Fatal("audit state = unblocked, want fail-closed")
				}
			default:
				t.Fatal("missing fail-closed audit transition")
			}
			shutdownCtx, cancelShutdown := context.WithTimeout(ctx, time.Second)
			defer cancelShutdown()
			if err := service.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})
	})

	t.Run("Should give each sequential lane close an independent bounded drain", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			firstIdentity, err := workspacepkg.EnsureIdentity(ctx, t.TempDir())
			if err != nil {
				t.Fatalf("EnsureIdentity(first) error = %v", err)
			}
			secondIdentity, err := workspacepkg.EnsureIdentity(ctx, t.TempDir())
			if err != nil {
				t.Fatalf("EnsureIdentity(second) error = %v", err)
			}
			storeErrors := map[string]error{
				firstIdentity.WorkspaceID:  errors.New("first store write aborted"),
				secondIdentity.WorkspaceID: errors.New("second store write aborted"),
			}
			recordStarted := make(chan string, len(storeErrors))
			pool, err := workspacedb.NewPool(func(
				ctx context.Context,
				workspaceID string,
			) (workspacedb.ResolvedRoot, error) {
				storeErr, ok := storeErrors[workspaceID]
				if !ok {
					return workspacedb.ResolvedRoot{}, errors.New("unexpected workspace")
				}
				recordStarted <- workspaceID
				<-ctx.Done()
				return workspacedb.ResolvedRoot{}, errors.Join(storeErr, context.Cause(ctx))
			})
			if err != nil {
				t.Fatalf("NewPool() error = %v", err)
			}
			service, err := New(Options{Databases: pool, HomeDir: t.TempDir()})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			actor := terminalpkg.Actor{
				Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
			}
			infos := []terminalpkg.Info{
				{
					ID: "term-first-drain", WS: firstIdentity.WorkspaceID,
					ProfileID: "profile-a", Controller: &actor,
				},
				{
					ID: "term-second-drain", WS: secondIdentity.WorkspaceID,
					ProfileID: "profile-a", Controller: &actor,
				},
			}
			results := make(map[terminalpkg.ID]<-chan error, len(infos))
			lanes := make(map[terminalpkg.ID]*terminalLane, len(infos))
			for _, info := range infos {
				service.RegisterTerminal(ctx, info, nil, nil)
				lane := service.lane(info)
				if lane == nil {
					t.Fatalf("registered lane %q = nil", info.ID)
				}
				lanes[info.ID] = lane
				terminalID := info.ID
				results[info.ID] = lane.enqueue(terminalpkg.CommandRow{
					ID: "cmd-" + string(info.ID), TerminalID: &terminalID,
					ProfileID: info.ProfileID, Actor: actor, Command: "pwd", Cwd: "/workspace",
					StartedAt: time.UnixMilli(1), ExitCause: "exited", DetectedBy: "marker", Approval: "human",
				})
			}
			started := make(map[string]bool, len(infos))
			for range infos {
				started[<-recordStarted] = true
			}
			for _, info := range infos {
				if !started[info.WS] {
					t.Fatalf("record for workspace %q did not start", info.WS)
				}
			}

			closeErr := service.closeLanes(ctx, func(*terminalLane) bool { return true })
			if !errors.Is(closeErr, context.DeadlineExceeded) {
				t.Fatalf("closeLanes() error = %v, want deadline exceeded", closeErr)
			}
			for _, info := range infos {
				rowID := "cmd-" + string(info.ID)
				resultErr := receiveReadyJournalResult(t, string(info.ID), results[info.ID])
				if !errors.Is(resultErr, context.DeadlineExceeded) ||
					!errors.Is(resultErr, storeErrors[info.WS]) || !strings.Contains(resultErr.Error(), rowID) {
					t.Fatalf("result for %q = %v, want its row, store, and deadline errors", info.ID, resultErr)
				}
				for _, other := range infos {
					otherRowID := "cmd-" + string(other.ID)
					if other.ID != info.ID && strings.Contains(resultErr.Error(), otherRowID) {
						t.Fatalf("result for %q contains unrelated row %q: %v", info.ID, otherRowID, resultErr)
					}
				}
				select {
				case <-lanes[info.ID].done:
				default:
					t.Fatalf("lane %q remained live after closeLanes returned", info.ID)
				}
				if pending := lanes[info.ID].pending.Load(); pending != 0 {
					t.Fatalf("lane %q pending = %d, want 0", info.ID, pending)
				}
				if retained := service.lane(info); retained != nil {
					t.Fatalf("service retained closed lane %q", info.ID)
				}
			}
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Pool.Close() error = %v", err)
			}
		})
	})

	t.Run("Should publish audit transitions outside the lane lock", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		service, workspaceID := newJournalTestService(ctx, t)
		info := terminalpkg.Info{
			ID: "term-reentrant-audit", WS: workspaceID, ProfileID: "profile-a",
		}
		var lane *terminalLane
		reentered := make(chan bool, 1)
		transitions := make([]bool, 0, 2)
		service.RegisterTerminal(ctx, info, nil, func(event terminalpkg.Event) {
			if event.Kind != terminalpkg.EventKindAuditChanged {
				return
			}
			blocked := event.DetailValue().AuditBlocked
			transitions = append(transitions, blocked)
			if blocked {
				reentered <- lane.reserve(1)
			}
		})
		lane = service.lane(info)
		if lane == nil {
			t.Fatal("registered lane = nil")
		}

		lane.setAuditBlocked()
		select {
		case admitted := <-reentered:
			if !admitted {
				t.Fatal("re-entrant reserve() = false, want true")
			}
		default:
			t.Fatal("audit observer did not finish re-entering the lane before transition returned")
		}
		if len(transitions) != 1 || !transitions[0] {
			t.Fatalf("audit transitions = %#v, want one blocked transition", transitions)
		}
		lane.release(1)
		if err := service.CloseTerminal(ctx, info); err != nil {
			t.Fatalf("CloseTerminal() error = %v", err)
		}
	})

	t.Run("Should reserve at most 64 pending command rows before PTY delivery", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		info := terminalpkg.Info{
			ID: "term-capacity", WS: workspaceID, ProfileID: "profile-a", Controller: &actor,
		}
		blocked := make(chan bool, 2)
		service.RegisterTerminal(ctx, info, func(value bool) { blocked <- value }, func(terminalpkg.Event) {})
		for index := range pendingLaneCapacity {
			reservation, admitted := service.ReserveInput(info, []byte("echo reserved\n"))
			if !admitted || reservation != 1 {
				t.Fatalf("ReserveInput(%d) = %d/%t, want 1/true", index, reservation, admitted)
			}
		}
		if reservation, admitted := service.ReserveInput(
			info,
			[]byte("echo rejected\n"),
		); admitted ||
			reservation != 1 {
			t.Fatalf("ReserveInput(over capacity) = %d/%t, want 1/false", reservation, admitted)
		}
		select {
		case value := <-blocked:
			if !value {
				t.Fatal("audit state = unblocked, want blocked at capacity")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for audit-blocked transition")
		}
		service.ReleaseInput(info, 1)
		select {
		case value := <-blocked:
			if value {
				t.Fatal("audit state = blocked, want unblocked after release")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for audit recovery")
		}
		service.ReleaseInput(info, pendingLaneCapacity-1)
	})

	t.Run("Should block on durable failure and recover one row exactly once", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		identity, err := workspacepkg.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		var available atomic.Bool
		pool, err := workspacedb.NewPool(func(context.Context, string) (workspacedb.ResolvedRoot, error) {
			if !available.Load() {
				return workspacedb.ResolvedRoot{}, errors.New("store unavailable")
			}
			return workspacedb.ResolvedRoot{
				RootDir:     workspaceRoot,
				WorkspaceID: identity.WorkspaceID,
			}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		service, err := New(Options{Databases: pool, HomeDir: t.TempDir()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		t.Cleanup(func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := service.Shutdown(shutdownCtx); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		info := terminalpkg.Info{
			ID: "term-retry", WS: identity.WorkspaceID, ProfileID: "profile-a",
			Controller: &terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
		}
		blocked := make(chan bool, 4)
		service.RegisterTerminal(ctx, info, func(value bool) { blocked <- value }, nil)
		if err := service.ConsumeMarkerFacts(ctx, info, []terminalpkg.MarkerFacts{
			{Kind: "S", Command: "pwd", Cwd: "/workspace"}, {Kind: "F", Exit: new(0)},
		}); err != nil {
			t.Fatalf("ConsumeMarkerFacts() error = %v", err)
		}
		waitForAuditState(t, blocked, true)
		if pending := service.PendingCount(info); pending != 1 || service.WriteFailureCount() < retryBlockAttempt {
			t.Fatalf("pending/failures = %d/%d, want 1/>=%d", pending, service.WriteFailureCount(), retryBlockAttempt)
		}
		available.Store(true)
		waitForAuditState(t, blocked, false)
		page, err := service.Query(
			ctx, identity.WorkspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{},
		)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(page.Entries) != 1 || page.Entries[0].DetectedBy != "marker" {
			t.Fatalf("recovered entries = %#v, want one marker row", page.Entries)
		}
	})
}

func newJournalTestService(ctx context.Context, t *testing.T) (*Service, string) {
	t.Helper()
	workspaceRoot := t.TempDir()
	identity, err := workspacepkg.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	pool, err := workspacedb.NewPool(func(
		_ context.Context,
		workspaceID string,
	) (workspacedb.ResolvedRoot, error) {
		if workspaceID != identity.WorkspaceID {
			return workspacedb.ResolvedRoot{}, errors.New("unknown workspace")
		}
		return workspacedb.ResolvedRoot{
			RootDir:     workspaceRoot,
			WorkspaceID: identity.WorkspaceID,
		}, nil
	})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	service, err := New(Options{Databases: pool, HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := service.Shutdown(shutdownCtx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	return service, identity.WorkspaceID
}

func commandIDForIndex(index int) string {
	return "cmd-" + string(rune('a'+index/26)) + string(rune('a'+index%26))
}

func commandRowByID(t *testing.T, page *terminalpkg.Page, id string) terminalpkg.CommandRow {
	t.Helper()
	for _, row := range page.Entries {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("command row %q not found", id)
	return terminalpkg.CommandRow{}
}

func receiveReadyJournalResult(t *testing.T, name string, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	default:
		t.Fatalf("%s result waiter remained live after close returned", name)
		return nil
	}
}

func waitForAuditState(t *testing.T, states <-chan bool, want bool) {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case got := <-states:
			if got == want {
				return
			}
		case <-timer.C:
			t.Fatalf("audit state did not become %t", want)
		}
	}
}

func waitForJournalRows(
	ctx context.Context,
	t *testing.T,
	service *Service,
	workspaceID string,
	want int,
) *terminalpkg.Page {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		page, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err == nil && len(page.Entries) >= want {
			return page
		}
		select {
		case <-deadline.C:
			got := 0
			if page != nil {
				got = len(page.Entries)
			}
			t.Fatalf("journal rows = %d error=%v, want >= %d", got, err, want)
		case <-ctx.Done():
			t.Fatalf("journal row wait canceled: %v", context.Cause(ctx))
		case <-ticker.C:
		}
	}
}
