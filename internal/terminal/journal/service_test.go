package journal

// Suite: durable terminal journal
// Invariant: accepted rows persist once, failures do not wedge later progress, and reads stay workspace/profile scoped.
// Boundary IN: journal assembly, workspacedb SQLite, retained artifact filesystem.
// Boundary OUT: HTTP/UDS projections and browser replay, owned by later terminal task suites.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

	t.Run("Should release a command identity rejected after its lane closes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		actor := terminalpkg.Actor{
			Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
		}
		info := terminalpkg.Info{
			ID: "term-close-race", WS: workspaceID, ProfileID: actor.ProfileID,
		}
		events := make(chan terminalpkg.Event, 1)
		service.RegisterTerminal(info, nil, func(event terminalpkg.Event) { events <- event })
		lane := service.lane(info)
		if lane == nil {
			t.Fatal("registered lane = nil")
		}
		lane.mu.Lock()
		lane.closed = true
		lane.mu.Unlock()
		const commandID = "cmd-close-race"
		if !service.claimCommandID(workspaceID, commandID) {
			t.Fatal("claimCommandID(first) = false, want reservation")
		}
		duration := int64(1)
		terminalID := info.ID
		lane.finishCommand(terminalpkg.CommandRow{
			ID: commandID, TerminalID: &terminalID, ProfileID: actor.ProfileID, Actor: actor,
			Command: "pwd", Cwd: "/workspace", StartedAt: time.UnixMilli(1), DurationMs: &duration,
			ExitCause: "unknown", DetectedBy: "idle", Approval: "human",
		}, time.UnixMilli(2))

		if !service.claimCommandID(workspaceID, commandID) {
			t.Fatal("claimCommandID(after rejection) = false, want released identity")
		}
		service.ReleaseCommandID(workspaceID, commandID)
		select {
		case event := <-events:
			t.Fatalf("rejected command emitted event %#v", event)
		default:
		}
		lane.wake <- struct{}{}
		err := service.CloseTerminal(ctx, info)
		if err == nil || !strings.Contains(err.Error(), "command completed after lane close") {
			t.Fatalf("CloseTerminal() error = %v, want rejected command persistence error", err)
		}
		if got := service.lane(info); got != nil {
			t.Fatalf("lane after terminal close = %#v, want removed stopped lane", got)
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
			OutputTail: []terminalpkg.OutputSegment{
				{Kind: terminalpkg.OutputSegmentBytes, Text: "failed: "},
				terminalpkg.RedactedInputMarker(6),
			},
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
		if len(got.OutputTail) != 2 || got.OutputTail[0].Kind != terminalpkg.OutputSegmentBytes ||
			got.OutputTail[0].Text != "failed: " ||
			got.OutputTail[1].Kind != terminalpkg.OutputSegmentRedactedInput ||
			got.OutputTail[1].Characters != 6 || got.OutputTail[1].Text != "" {
			t.Fatalf("Query() output tail = %#v, want typed output and redacted marker", got.OutputTail)
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
		lateRow := terminalpkg.CommandRow{
			ID: "cmd-late-recording-link", TerminalID: &terminalID, ProfileID: "profile-a",
			Actor: row.Actor, Command: "late insert", Cwd: row.Cwd, StartedAt: time.UnixMilli(1200),
			ExitCause: "exited", DetectedBy: "exact", Approval: "approved_once",
		}
		if err := service.Record(ctx, workspaceID, lateRow); err != nil {
			t.Fatalf("Record(late covered command) error = %v", err)
		}
		linked, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query(linked) error = %v", err)
		}
		linkedInside := commandRowByID(t, linked, row.ID)
		if linkedInside.RecordingID == nil || *linkedInside.RecordingID != "rec-1" {
			t.Fatalf("inside recording_id = %v, want rec-1", linkedInside.RecordingID)
		}
		if id := commandRowByID(t, linked, lateRow.ID).RecordingID; id == nil || *id != "rec-1" {
			t.Fatalf("late covered recording_id = %v, want rec-1", id)
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

	t.Run("Should flush a registered lane after workspace removal is staged", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		terminalID := terminalpkg.ID("term-staged-flush")
		actor := terminalpkg.Actor{
			Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
		}
		info := terminalpkg.Info{
			ID: terminalID, WS: workspaceID, ProfileID: "profile-a", Cwd: "/workspace",
		}
		service.RegisterTerminal(info, nil, nil)
		preparation, err := service.PrepareWorkspaceRemoval(ctx, workspaceID)
		if err != nil {
			t.Fatalf("PrepareWorkspaceRemoval() error = %v", err)
		}
		if err := preparation.BeforeDelete(ctx); err != nil {
			t.Fatalf("BeforeDelete() error = %v", err)
		}
		row := terminalpkg.CommandRow{
			ID: "cmd-staged-flush", TerminalID: &terminalID, ProfileID: info.ProfileID,
			Actor: actor, Command: "sleep 600", Cwd: info.Cwd, StartedAt: time.UnixMilli(1000),
			ExitCause: "signaled", DetectedBy: "exact", Approval: "human",
		}
		if err := service.RecordQueued(ctx, info, row); err != nil {
			t.Fatalf("RecordQueued() error = %v", err)
		}
		if err := service.CloseTerminal(ctx, info); err != nil {
			t.Fatalf("CloseTerminal() error = %v", err)
		}
		if err := preparation.Commit(ctx); err != nil {
			t.Fatalf("Commit() error = %v", err)
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
		entropy := bytes.Repeat([]byte{0x01}, 32)
		entropy = append(entropy, bytes.Repeat([]byte{0x02}, 16)...)
		service.entropy = bytes.NewReader(entropy)
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
		if first.ArtifactID != "art-01010101010101010101010101010101" ||
			second.ArtifactID != "art-02020202020202020202020202020202" {
			t.Fatalf("collision-safe artifact IDs = %q/%q", first.ArtifactID, second.ArtifactID)
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
				ID: "term-idle", WS: workspaceID, ProfileID: "profile-a", Cwd: "/workspace",
			}
			events := make(chan terminalpkg.Event, 6)
			service.RegisterTerminal(info, func(bool) {}, func(event terminalpkg.Event) { events <- event })
			service.ObserveInput(info, actor, []byte("echo approximate\n"))
			service.ObserveOutput(info, []byte("working"))
			idleRow := waitForJournalRows(ctx, t, service, workspaceID, 1).Entries[0]
			if idleRow.Command != "echo approximate" || idleRow.DetectedBy != "idle" ||
				idleRow.Actor != actor || idleRow.ExitCause != "unknown" {
				t.Fatalf("idle row = %#v, want actor %#v", idleRow, actor)
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
				switch event.Kind {
				case terminalpkg.EventKindCommandStarted:
					started++
				case terminalpkg.EventKindCommandFinished:
					finished++
				default:
					continue
				}
				if event.Actor != actor {
					t.Fatalf("%s actor = %#v, want observed input actor %#v", event.Kind, event.Actor, actor)
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
		info := terminalpkg.Info{
			ID: "term-close", WS: workspaceID, ProfileID: "profile-a", Cwd: "/workspace",
		}
		service.RegisterTerminal(info, func(bool) {}, func(terminalpkg.Event) {})
		if err := service.ConsumeMarkerFacts(ctx, info, []terminalpkg.MarkerFacts{
			{Kind: "S", Command: "pwd", Cwd: "/workspace"},
		}); err != nil {
			t.Fatalf("ConsumeMarkerFacts() error = %v", err)
		}
		lane := service.lane(info)
		if lane == nil {
			t.Fatal("registered lane = nil")
		}
		lane.appendOutputTailLocked([]byte("live tail"))
		if err := service.ConsumeMarkerFacts(
			ctx,
			info,
			[]terminalpkg.MarkerFacts{{Kind: "F", Exit: new(0)}},
		); err != nil {
			t.Fatalf("ConsumeMarkerFacts(finish) error = %v", err)
		}
		if err := service.CloseTerminal(ctx, info); err != nil {
			t.Fatalf("CloseTerminal() error = %v", err)
		}
		if lane := service.lane(info); lane != nil {
			t.Fatal("CloseTerminal() retained the terminal lane")
		}
		page, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: "profile-a"}, terminalpkg.Query{})
		if err != nil || len(page.Entries) != 1 || page.Entries[0].Command != "pwd" ||
			len(page.Entries[0].OutputTail) != 0 {
			t.Fatalf("Query(after CloseTerminal) = %#v, error=%v", page, err)
		}
	})

	t.Run("Should clear every live output tail on shutdown", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		terminalID := terminalpkg.ID("term-shutdown-tail")
		row := terminalpkg.CommandRow{
			ID: "cmd-shutdown-tail", TerminalID: &terminalID, ProfileID: "profile-a",
			Actor:   terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			Command: "pwd", Cwd: "/workspace", StartedAt: time.UnixMilli(1),
			ExitCause: "exited", DetectedBy: "exact", Approval: "human",
			OutputTail: []terminalpkg.OutputSegment{{Kind: terminalpkg.OutputSegmentBytes, Text: "live tail"}},
		}
		if err := service.Record(ctx, workspaceID, row); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		if got := service.liveOutputTail(workspaceID, row.ID); len(got) != 1 {
			t.Fatalf("liveOutputTail(before shutdown) = %#v, want one segment", got)
		}
		if err := service.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if got := service.liveOutputTail(workspaceID, row.ID); len(got) != 0 {
			t.Fatalf("liveOutputTail(after shutdown) = %#v, want empty", got)
		}
	})

	t.Run("Should cancel lanes and clear live tails after a lane close failure", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		pool, err := workspacedb.NewPool(func(context.Context, string) (workspacedb.ResolvedRoot, error) {
			return workspacedb.ResolvedRoot{}, errors.New("unexpected workspace open")
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		service, err := New(ctx, Options{Databases: pool, HomeDir: t.TempDir()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		laneErr := errors.New("lane close failed")
		laneDone := make(chan struct{})
		close(laneDone)
		service.lanes["failed"] = &terminalLane{closed: true, done: laneDone, err: laneErr}
		service.retainLiveOutputTail(
			"workspace-a",
			"terminal-a",
			"command-a",
			[]terminalpkg.OutputSegment{{Kind: terminalpkg.OutputSegmentBytes, Text: "tail"}},
		)
		shutdownErr := service.Shutdown(ctx)
		if !errors.Is(shutdownErr, laneErr) {
			t.Fatalf("Shutdown() error = %v, want lane close failure", shutdownErr)
		}
		if !errors.Is(context.Cause(service.laneCtx), context.Canceled) {
			t.Fatalf("lane context cause = %v, want cancellation", context.Cause(service.laneCtx))
		}
		if got := service.liveOutputTail("workspace-a", "command-a"); len(got) != 0 {
			t.Fatalf("liveOutputTail(after failed close) = %#v, want empty", got)
		}
		if _, err := pool.Open(ctx, "workspace-a"); err == nil || !strings.Contains(err.Error(), "pool is closed") {
			t.Fatalf("pool.Open(after failed close) error = %v, want closed pool", err)
		}
	})

	t.Run("Should retain every waiter and lane owner when bounded close expires", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			runCtx, cancelRun := context.WithCancelCause(ctx)
			workspaceRoot := t.TempDir()
			identity, err := workspacepkg.EnsureIdentity(ctx, workspaceRoot)
			if err != nil {
				t.Fatalf("EnsureIdentity() error = %v", err)
			}
			recordStarted := make(chan struct{})
			releaseStore := make(chan struct{})
			var recordStartedOnce sync.Once
			pool, err := workspacedb.NewPool(func(
				ctx context.Context,
				_ string,
			) (workspacedb.ResolvedRoot, error) {
				recordStartedOnce.Do(func() { close(recordStarted) })
				select {
				case <-releaseStore:
					return workspacedb.ResolvedRoot{
						RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID,
					}, nil
				case <-ctx.Done():
					return workspacedb.ResolvedRoot{}, context.Cause(ctx)
				}
			})
			if err != nil {
				t.Fatalf("NewPool() error = %v", err)
			}
			service, err := New(runCtx, Options{Databases: pool, HomeDir: t.TempDir()})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			actor := terminalpkg.Actor{
				Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
			}
			info := terminalpkg.Info{
				ID: "term-cancel-flush", WS: identity.WorkspaceID, ProfileID: "profile-a",
			}
			audit := make(chan bool, 2)
			service.RegisterTerminal(info, func(blocked bool) { audit <- blocked }, nil)
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
			assertJournalResultPending(t, "in-flight", firstResult)
			assertJournalResultPending(t, "queued", secondResult)
			select {
			case <-lane.done:
				t.Fatal("lane stopped after bounded close expired")
			default:
			}
			lane.mu.Lock()
			pending := lane.pending.Load()
			reservations := lane.reservations
			queued := len(lane.rows)
			sealed := lane.sealed
			lane.mu.Unlock()
			if pending != 2 || reservations != 1 || queued != 1 || !sealed {
				t.Fatalf(
					"retained lane state = pending %d reservations %d queued %d sealed %t",
					pending, reservations, queued, sealed,
				)
			}
			if retained := service.lane(info); retained != lane {
				t.Fatalf("lane ownership after timeout = %#v, want original lane", retained)
			}
			select {
			case <-audit:
				t.Fatal("bounded close timeout must not fabricate an audit state transition")
			default:
			}

			lane.release(1)
			close(releaseStore)
			synctest.Wait()
			if firstErr := receiveReadyJournalResult(
				t,
				"in-flight retry",
				firstResult,
			); firstErr != nil {
				t.Fatalf("in-flight retry error = %v", firstErr)
			}
			if secondErr := receiveReadyJournalResult(
				t,
				"queued retry",
				secondResult,
			); secondErr != nil {
				t.Fatalf("queued retry error = %v", secondErr)
			}
			if err := service.CloseTerminal(ctx, info); err != nil {
				t.Fatalf("CloseTerminal(retry) error = %v", err)
			}
			if retained := service.lane(info); retained != nil {
				t.Fatalf("CloseTerminal(retry) retained lane %#v", retained)
			}
			cancelRun(errors.New("test complete"))
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Pool.Close() error = %v", err)
			}
		})
	})

	t.Run("Should give each sequential lane close an independent bounded drain", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			runCtx, cancelRun := context.WithCancelCause(ctx)
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
			service, err := New(runCtx, Options{Databases: pool, HomeDir: t.TempDir()})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			actor := terminalpkg.Actor{
				Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a",
			}
			infos := []terminalpkg.Info{
				{
					ID: "term-first-drain", WS: firstIdentity.WorkspaceID,
					ProfileID: "profile-a",
				},
				{
					ID: "term-second-drain", WS: secondIdentity.WorkspaceID,
					ProfileID: "profile-a",
				},
			}
			results := make(map[terminalpkg.ID]<-chan error, len(infos))
			lanes := make(map[terminalpkg.ID]*terminalLane, len(infos))
			for _, info := range infos {
				service.RegisterTerminal(info, nil, nil)
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
				assertJournalResultPending(t, string(info.ID), results[info.ID])
				select {
				case <-lanes[info.ID].done:
					t.Fatalf("lane %q stopped after its bounded close expired", info.ID)
				default:
				}
				if pending := lanes[info.ID].pending.Load(); pending != 1 {
					t.Fatalf("lane %q pending = %d, want retained row", info.ID, pending)
				}
				if retained := service.lane(info); retained != lanes[info.ID] {
					t.Fatalf("service lane %q = %#v, want original owner", info.ID, retained)
				}
			}
			cleanupCause := errors.New("test producer cleanup")
			cancelRun(cleanupCause)
			synctest.Wait()
			for _, info := range infos {
				resultErr := receiveReadyJournalResult(t, string(info.ID)+" cleanup", results[info.ID])
				if !errors.Is(resultErr, cleanupCause) || !errors.Is(resultErr, storeErrors[info.WS]) {
					t.Fatalf("result for %q = %v, want producer and store failures", info.ID, resultErr)
				}
				<-lanes[info.ID].done
				service.removeStoppedLane(terminalLaneKey(info), lanes[info.ID])
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
		service.RegisterTerminal(info, nil, func(event terminalpkg.Event) {
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
		info := terminalpkg.Info{
			ID: "term-capacity", WS: workspaceID, ProfileID: "profile-a",
		}
		blocked := make(chan bool, 2)
		service.RegisterTerminal(info, func(value bool) { blocked <- value }, func(terminalpkg.Event) {})
		reservations := make([]terminalpkg.JournalInputReservation, 0, pendingLaneCapacity)
		for index := range pendingLaneCapacity {
			reservation, admitted := service.ReserveInput(
				info,
				terminalpkg.JournalInput{Content: []byte("echo reserved\n")},
			)
			if !admitted || reservation == nil {
				t.Fatalf("ReserveInput(%d) = %#v/%t, want reservation/true", index, reservation, admitted)
			}
			reservations = append(reservations, reservation)
		}
		if reservation, admitted := service.ReserveInput(
			info,
			terminalpkg.JournalInput{Content: []byte("echo rejected\n")},
		); admitted ||
			reservation != nil {
			t.Fatalf("ReserveInput(over capacity) = %#v/%t, want nil/false", reservation, admitted)
		}
		select {
		case value := <-blocked:
			if !value {
				t.Fatal("audit state = unblocked, want blocked at capacity")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for audit-blocked transition")
		}
		reservations[0].Release()
		select {
		case value := <-blocked:
			if value {
				t.Fatal("audit state = blocked, want unblocked after release")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for audit recovery")
		}
		for _, reservation := range reservations[1:] {
			reservation.Release()
		}
	})

	t.Run("Should keep a lane alive until reserved PTY delivery is journaled", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		info := terminalpkg.Info{
			ID: "term-delivery", WS: workspaceID, ProfileID: actor.ProfileID,
			Cwd: "/workspace",
		}
		service.RegisterTerminal(info, func(bool) {}, func(terminalpkg.Event) {})
		input := terminalpkg.JournalInput{Content: []byte("echo delivered\n")}
		reservation, admitted := service.ReserveInput(info, input)
		if !admitted || reservation == nil {
			t.Fatalf("ReserveInput() = %#v/%t, want reservation/true", reservation, admitted)
		}
		lane := service.lane(info)
		if lane == nil {
			t.Fatal("registered lane = nil")
		}

		closed := make(chan error, 1)
		go func() { closed <- service.CloseTerminal(ctx, info) }()
		deadline := time.Now().Add(time.Second)
		sealed := false
		for !sealed && time.Now().Before(deadline) {
			lane.mu.Lock()
			sealed = lane.sealed
			lane.mu.Unlock()
			runtime.Gosched()
		}
		if !sealed {
			t.Fatal("CloseTerminal() did not seal its lane")
		}
		if retained := service.lane(info); retained != lane {
			t.Fatalf("CloseTerminal() lane ownership = %#v, want sealed lane retained until quiescence", retained)
		}
		if extra, accepted := service.ReserveInput(info, input); accepted || extra != nil {
			t.Fatalf("ReserveInput() after seal = %#v/%t, want nil/false", extra, accepted)
		}
		select {
		case err := <-closed:
			t.Fatalf("CloseTerminal() returned before delivery commit: %v", err)
		default:
		}
		reservation.Commit(actor, input)
		select {
		case err := <-closed:
			if err != nil {
				t.Fatalf("CloseTerminal() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("CloseTerminal() did not finish after delivery commit")
		}
		if retained := service.lane(info); retained != nil {
			t.Fatalf("CloseTerminal() retained quiescent lane %#v", retained)
		}
		page, err := service.Query(ctx, workspaceID, store.ReadScope{ProfileID: actor.ProfileID}, terminalpkg.Query{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(page.Entries) != 1 || page.Entries[0].Command != "echo delivered" {
			t.Fatalf("Query() entries = %#v, want one delivered command", page.Entries)
		}
	})

	t.Run("Should regenerate a colliding marker command identity before publishing it", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		terminalID := terminalpkg.ID("term-collision")
		actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		info := terminalpkg.Info{ID: terminalID, WS: workspaceID, ProfileID: "profile-a"}
		const collisionID = "cmd-000102030405060708090a0b0c0d0e0f"
		seed := terminalpkg.CommandRow{
			ID: collisionID, TerminalID: &terminalID, ProfileID: info.ProfileID, Actor: actor,
			Command: "seed", Cwd: "/workspace", StartedAt: time.UnixMilli(1),
			ExitCause: "exited", DetectedBy: "exact", Approval: "human",
		}
		if err := service.Record(ctx, workspaceID, seed); err != nil {
			t.Fatalf("Record(seed) error = %v", err)
		}
		service.entropy = bytes.NewReader([]byte{
			0, 1, 2, 3, 4, 5, 6, 7,
			8, 9, 10, 11, 12, 13, 14, 15,
			16, 17, 18, 19, 20, 21, 22, 23,
			24, 25, 26, 27, 28, 29, 30, 31,
		})
		events := make(chan terminalpkg.Event, 2)
		service.RegisterTerminal(info, nil, func(event terminalpkg.Event) {
			if event.Kind == terminalpkg.EventKindCommandStarted || event.Kind == terminalpkg.EventKindCommandFinished {
				events <- event
			}
		})
		if err := service.ConsumeMarkerFacts(ctx, info, []terminalpkg.MarkerFacts{
			{Kind: "S", Command: "pwd", Cwd: "/workspace"}, {Kind: "F", Exit: new(0)},
		}); err != nil {
			t.Fatalf("ConsumeMarkerFacts() error = %v", err)
		}
		page := waitForJournalRows(ctx, t, service, workspaceID, 2)
		const regeneratedID = "cmd-101112131415161718191a1b1c1d1e1f"
		if commandRowByID(t, page, regeneratedID).Command != "pwd" {
			t.Fatalf("persisted rows = %#v, want regenerated marker command", page.Entries)
		}
		for range 2 {
			event := <-events
			if event.DetailValue().CommandID != regeneratedID {
				t.Fatalf("%s command_id = %q, want %q", event.Kind, event.DetailValue().CommandID, regeneratedID)
			}
		}
	})

	t.Run("Should regenerate a colliding recording identity before admission", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		service, workspaceID := newJournalTestService(ctx, t)
		terminalID := terminalpkg.ID("term-recording-collision")
		const collisionID = "rec-000102030405060708090a0b0c0d0e0f"
		stoppedAt := time.UnixMilli(2)
		if _, err := service.PersistRecording(ctx, workspaceID, terminalID, terminalpkg.RecordingRef{
			ID: collisionID, TerminalID: terminalID, ProfileID: "profile-a",
			StartedAt: time.UnixMilli(1), StoppedAt: &stoppedAt, ExpiresAt: time.UnixMilli(3),
		}, []byte("recording")); err != nil {
			t.Fatalf("PersistRecording(seed) error = %v", err)
		}
		service.entropy = bytes.NewReader([]byte{
			0, 1, 2, 3, 4, 5, 6, 7,
			8, 9, 10, 11, 12, 13, 14, 15,
			16, 17, 18, 19, 20, 21, 22, 23,
			24, 25, 26, 27, 28, 29, 30, 31,
		})
		id, err := service.ReserveRecordingID(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ReserveRecordingID() error = %v", err)
		}
		defer service.ReleaseRecordingID(workspaceID, id)
		if id != "rec-101112131415161718191a1b1c1d1e1f" {
			t.Fatalf("ReserveRecordingID() = %q, want regenerated identity", id)
		}
	})

	t.Run("Should retain an observed marker command while its store is unavailable", func(t *testing.T) {
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
				return workspacedb.ResolvedRoot{}, fmt.Errorf("store unavailable: %w", context.DeadlineExceeded)
			}
			return workspacedb.ResolvedRoot{
				RootDir:     workspaceRoot,
				WorkspaceID: identity.WorkspaceID,
			}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		ownerCtx, cancelOwner := context.WithCancelCause(context.WithoutCancel(t.Context()))
		service, err := New(ownerCtx, Options{Databases: pool, HomeDir: t.TempDir()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		t.Cleanup(func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := service.Shutdown(shutdownCtx); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
			cancelOwner(errors.New("journal retry test cleanup complete"))
		})
		info := terminalpkg.Info{ID: "term-retry", WS: identity.WorkspaceID, ProfileID: "profile-a"}
		blocked := make(chan bool, 4)
		service.RegisterTerminal(info, func(value bool) { blocked <- value }, nil)
		if err := service.ConsumeMarkerFacts(ctx, info, []terminalpkg.MarkerFacts{
			{Kind: "S", Command: "pwd", Cwd: "/workspace"},
			{Kind: "F", Exit: new(0)},
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
		if len(page.Entries) != 1 || page.Entries[0].DetectedBy != "marker" || page.Entries[0].Command != "pwd" {
			t.Fatalf("recovered entries = %#v, want one marker row", page.Entries)
		}
	})

	t.Run("Should retain an exhausted row and keep audit blocked until recovery", func(t *testing.T) {
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
				return workspacedb.ResolvedRoot{}, fmt.Errorf("store unavailable: %w", context.DeadlineExceeded)
			}
			return workspacedb.ResolvedRoot{RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		ownerCtx, cancelOwner := context.WithCancelCause(context.WithoutCancel(t.Context()))
		service, err := New(ownerCtx, Options{Databases: pool, HomeDir: t.TempDir()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		t.Cleanup(func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := service.Shutdown(shutdownCtx); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
			cancelOwner(errors.New("journal exhaustion test cleanup complete"))
		})
		terminalID := terminalpkg.ID("term-exhaustion")
		actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		info := terminalpkg.Info{ID: terminalID, WS: identity.WorkspaceID, ProfileID: "profile-a"}
		blocked := make(chan bool, 4)
		service.RegisterTerminal(info, func(value bool) { blocked <- value }, nil)
		row := func(id string) terminalpkg.CommandRow {
			return terminalpkg.CommandRow{
				ID: id, TerminalID: &terminalID, ProfileID: info.ProfileID, Actor: actor,
				Command: id, Cwd: "/workspace", StartedAt: time.UnixMilli(1),
				ExitCause: "exited", DetectedBy: "exact", Approval: "human",
			}
		}
		lane := service.lane(info)
		exhausted := lane.enqueue(row("cmd-exhausted"))
		waitForAuditState(t, blocked, true)
		deadline := time.Now().Add(3 * time.Second)
		for service.WriteFailureCount() <= 5 && time.Now().Before(deadline) {
			runtime.Gosched()
		}
		if failures := service.WriteFailureCount(); failures <= 5 {
			t.Fatalf("write failures = %d, want retries beyond the old exhaustion limit", failures)
		}
		assertJournalResultPending(t, "exhausted row", exhausted)
		if service.PendingCount(info) != 1 || !lane.blocked {
			t.Fatalf("retained lane = pending %d blocked %t, want 1/true", service.PendingCount(info), lane.blocked)
		}
		available.Store(true)
		select {
		case err := <-exhausted:
			if err != nil {
				t.Fatalf("recovered row error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("retained row did not persist after store recovery")
		}
		waitForAuditState(t, blocked, false)
		successor := lane.enqueue(row("cmd-after-recovery"))
		if err := <-successor; err != nil {
			t.Fatalf("successor row error = %v", err)
		}
		if service.PendingCount(info) != 0 || lane.blocked {
			t.Fatalf("lane after recovery = pending %d blocked %t", service.PendingCount(info), lane.blocked)
		}
		page, err := service.Query(
			ctx,
			identity.WorkspaceID,
			store.ReadScope{ProfileID: info.ProfileID},
			terminalpkg.Query{},
		)
		if err != nil || len(page.Entries) != 2 {
			t.Fatalf("Query() entries = %#v error = %v, want original and successor once", page, err)
		}
	})
}

func assertJournalResultPending(t *testing.T, label string, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		t.Fatalf("%s result completed while lane ownership was retained: %v", label, err)
	default:
	}
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
	ownerCtx, cancelOwner := context.WithCancelCause(context.WithoutCancel(t.Context()))
	service, err := New(ownerCtx, Options{Databases: pool, HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := service.Shutdown(shutdownCtx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
		cancelOwner(errors.New("journal test cleanup complete"))
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
