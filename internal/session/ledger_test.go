package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	sessionledger "github.com/compozy/compozy/internal/sessions/ledger"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestManagerSessionLedger(t *testing.T) {
	t.Parallel()

	t.Run("Should materialize forensic ledger on session end", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		h.manager.ledgerMaterializer = materializer
		parent := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				parent.ID,
			); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop(parent) error = %v", err)
			}
		})
		session := createChildSession(t, h, parent.ID)

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		lines := readSessionLedgerLines(t, ledgerPath)
		if len(lines) < 2 {
			t.Fatalf("ledger line count = %d, want at least 2", len(lines))
		}
		meta := decodeSessionLedgerLine(t, lines[0])
		if got := meta["type"]; got != "ledger_meta" {
			t.Fatalf("ledger meta type = %v, want ledger_meta", got)
		}
		if got := meta["workspace_id"]; got != h.workspaceID {
			t.Fatalf("ledger workspace_id = %v, want %q", got, h.workspaceID)
		}
		if got := meta["spawn_parent_id"]; got != parent.ID {
			t.Fatalf("ledger spawn_parent_id = %v, want %q", got, parent.ID)
		}

		foundStopEvent := false
		for _, line := range lines[1:] {
			event := decodeSessionLedgerLine(t, line)
			if event["event_type"] == EventTypeSessionStopped {
				foundStopEvent = true
				break
			}
		}
		if !foundStopEvent {
			t.Fatalf("ledger %q does not contain %s event", ledgerPath, EventTypeSessionStopped)
		}

		events := readStoredEvents(t, session)
		if len(events) == 0 {
			t.Fatal("live event store has no events after ledger materialization")
		}
	})

	// Invariant: resuming a stopped session retires its forensic projection before
	// new events, and the next stop atomically materializes the complete history.
	t.Run("Should replace the stopped ledger across resume and stop", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		h.manager.ledgerMaterializer = materializer
		session := createSession(t, h)
		first, err := h.manager.Prompt(testutil.Context(t), session.ID, "before resume")
		if err != nil {
			t.Fatalf("Prompt(before resume) error = %v", err)
		}
		collectEvents(t, first)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop(first) error = %v", err)
		}

		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		if content, err := os.ReadFile(ledgerPath); err != nil {
			t.Fatalf("ReadFile(first ledger) error = %v", err)
		} else if !strings.Contains(string(content), "before resume") {
			t.Fatalf("first ledger does not contain the first prompt: %s", content)
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		if _, err := os.Stat(ledgerPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(ledger after resume) error = %v, want os.ErrNotExist", err)
		}
		second, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "after resume")
		if err != nil {
			t.Fatalf("Prompt(after resume) error = %v", err)
		}
		collectEvents(t, second)
		if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Fatalf("Stop(second) error = %v", err)
		}

		content, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(replaced ledger) error = %v", err)
		}
		if !strings.Contains(string(content), "before resume") ||
			!strings.Contains(string(content), "after resume") {
			t.Fatalf("replaced ledger does not contain complete history: %s", content)
		}
	})

	t.Run("Should preserve the stopped ledger when resume cannot discard it", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		h.manager.ledgerMaterializer = materializer
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		before, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(ledger before failed discard) error = %v", err)
		}
		discardErr := errors.New("ledger storage unavailable")
		h.manager.ledgerMaterializer = &testLedgerMaterializer{
			delegate: materializer,
			discard: func(context.Context, store.SessionLedgerRecord) error {
				return discardErr
			},
		}
		startCalls := len(h.driver.startCalls)

		if _, err := h.manager.Resume(testutil.Context(t), session.ID); !errors.Is(err, discardErr) {
			t.Fatalf("Resume() error = %v, want %v", err, discardErr)
		}
		if len(h.driver.startCalls) != startCalls {
			t.Fatalf("driver start calls = %d, want unchanged %d", len(h.driver.startCalls), startCalls)
		}
		after, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(ledger after failed discard) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatal("failed resume changed the existing ledger")
		}
	})

	t.Run("Should rematerialize the ledger when provider resume fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		h.manager.ledgerMaterializer = materializer
		session := createSession(t, h)
		const originalPrompt = "before failed resume"
		events, err := h.manager.Prompt(testutil.Context(t), session.ID, originalPrompt)
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		startErr := errors.New("provider resume unavailable")
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
			return nil, startErr
		}

		if _, err := h.manager.Resume(testutil.Context(t), session.ID); !errors.Is(err, startErr) {
			t.Fatalf("Resume() error = %v, want %v", err, startErr)
		}
		ledger, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(rematerialized ledger) error = %v", err)
		}
		if !strings.Contains(string(ledger), originalPrompt) {
			t.Fatalf("rematerialized ledger does not contain original prompt: %s", ledger)
		}
		if !strings.Contains(string(ledger), EventTypeSessionStopped) {
			t.Fatalf("rematerialized ledger does not contain %s: %s", EventTypeSessionStopped, ledger)
		}
		if meta := readMeta(t, session.MetaPath()); meta.State != string(StateStopped) {
			t.Fatalf("meta state after failed resume = %q, want %q", meta.State, StateStopped)
		}
	})
}

func createChildSession(t *testing.T, h *harness, parentID string) *Session {
	t.Helper()

	expiresAt := time.Now().UTC().Add(time.Hour)
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "child",
		Workspace: h.workspaceID,
		Type:      SessionTypeSpawned,
		Lineage: &store.SessionLineage{
			ParentSessionID: parentID,
			RootSessionID:   parentID,
			SpawnDepth:      1,
			SpawnRole:       "reviewer",
			TTLExpiresAt:    &expiresAt,
		},
	})
	if err != nil {
		t.Fatalf("Create(child) error = %v", err)
	}
	return session
}

func readSessionLedgerLines(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("ledger %q is empty", path)
	}
	return lines
}

func decodeSessionLedgerLine(t *testing.T, line string) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", line, err)
	}
	return payload
}
