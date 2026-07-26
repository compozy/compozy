package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sessionledger "github.com/compozy/agh/internal/sessions/ledger"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
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
