//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	_ "modernc.org/sqlite"
)

const (
	plantedRedactionSecret = "sk-ant-api03-aghredactionfixture1234567890"
	wantClaimTokenHash     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	wantEnvelopeSessionID  = "550e8400-e29b-41d4-a716-446655440000"
	wantEnvelopeRunID      = "62f82910-18ca-4f2e-aa4a-54dcde9fe761"
)

func TestDaemonE2ERedactsAgentOutputBeforeStreamingAndDurableAppend(t *testing.T) {
	t.Parallel()
	t.Run("Should redact before SSE history logs and both durable appends", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)
		testDaemonRedactionBoundary(t)
	})
}

func testDaemonRedactionBoundary(t *testing.T) {
	t.Helper()
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "redaction_fixture.json"),
			FixtureAgent: "redaction-probe",
			AgentName:    "redaction-probe",
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := createFixtureBackedSession(t, ctx, harness, "redaction-probe", "redaction-boundary")
	stream, err := harness.PromptSessionHTTP(ctx, session.ID, "emit redaction evidence")
	if err != nil {
		t.Fatalf("PromptSessionHTTP() error = %v", err)
	}
	assertRedactedBoundaryPayload(t, "HTTP SSE stream", joinedSSEPayload(stream), true)

	events, err := harness.SessionEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	eventsPayload, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("json.Marshal(events) error = %v", err)
	}
	assertRedactedBoundaryPayload(t, "history events", string(eventsPayload), true)

	transcript, err := harness.SessionTranscript(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionTranscript() error = %v", err)
	}
	transcriptPayload, err := json.Marshal(transcript)
	if err != nil {
		t.Fatalf("json.Marshal(transcript) error = %v", err)
	}
	assertRedactedBoundaryPayload(t, "history transcript", string(transcriptPayload), true)

	sessionDBPath := filepath.Join(harness.HomePaths.SessionsDir, session.ID, store.SessionDatabaseName)
	waitForRuntimeCondition(t, "redacted runtime ledger append", 5*time.Second, func() bool {
		payload, readErr := querySQLiteText(
			harness.HomePaths.DatabaseFile,
			"SELECT COALESCE(summary, '') || char(10) || content_json FROM event_summaries WHERE session_id = ?",
			session.ID,
		)
		return readErr == nil && strings.Contains(payload, redact.Marker)
	})

	runtimeRows, err := querySQLiteText(
		harness.HomePaths.DatabaseFile,
		"SELECT COALESCE(summary, '') || char(10) || content_json FROM event_summaries WHERE session_id = ?",
		session.ID,
	)
	if err != nil {
		t.Fatalf("query runtime event summaries error = %v", err)
	}
	assertRedactedBoundaryPayload(t, "agh.db event summaries", runtimeRows, true)

	sessionRows, err := querySQLiteText(sessionDBPath, "SELECT content FROM events ORDER BY sequence")
	if err != nil {
		t.Fatalf("query events.db rows error = %v", err)
	}
	assertRedactedBoundaryPayload(t, "events.db rows", sessionRows, true)
	for name, value := range map[string]string{
		"claim_token_hash": wantClaimTokenHash,
		"session_id":       wantEnvelopeSessionID,
		"run_id":           wantEnvelopeRunID,
	} {
		if !strings.Contains(sessionRows, value) {
			t.Fatalf("events.db rows missing intact %s %q: %s", name, value, sessionRows)
		}
	}

	logPayload, err := os.ReadFile(harness.HomePaths.LogFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", harness.HomePaths.LogFile, err)
	}
	assertRedactedBoundaryPayload(t, "structured log sink", string(logPayload), false)
	assertNoRawSecretInTree(t, harness.HomePaths.HomeDir)
}

func joinedSSEPayload(stream []e2etest.SSEEvent) string {
	var payload strings.Builder
	for _, event := range stream {
		payload.WriteString(event.Event)
		payload.WriteByte('\n')
		payload.Write(event.Data)
		payload.WriteByte('\n')
	}
	return payload.String()
}

func assertRedactedBoundaryPayload(t testing.TB, source string, payload string, requireMarker bool) {
	t.Helper()
	if strings.Contains(payload, plantedRedactionSecret) {
		t.Fatalf("%s leaked planted secret: %s", source, payload)
	}
	if requireMarker && !strings.Contains(payload, redact.Marker) {
		t.Fatalf("%s missing redaction marker: %s", source, payload)
	}
}

func assertNoRawSecretInTree(t testing.TB, root string) {
	t.Helper()
	secret := []byte(plantedRedactionSecret)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		if bytes.Contains(payload, secret) {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("resolve leaked artifact path %q: %w", path, err)
			}
			return fmt.Errorf("raw planted secret found in %q", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("harness artifact scan error = %v", err)
	}
}

func querySQLiteText(path string, query string, args ...any) (string, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", fmt.Errorf("open sqlite %q: %w", path, err)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		closeErr := db.Close()
		return "", errors.Join(fmt.Errorf("query sqlite %q: %w", path, err), closeErr)
	}

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			rowsCloseErr := rows.Close()
			dbCloseErr := db.Close()
			return "", errors.Join(fmt.Errorf("scan sqlite %q: %w", path, err), rowsCloseErr, dbCloseErr)
		}
		values = append(values, value)
	}
	iterationErr := rows.Err()
	rowsCloseErr := rows.Close()
	dbCloseErr := db.Close()
	if err := errors.Join(iterationErr, rowsCloseErr, dbCloseErr); err != nil {
		return "", fmt.Errorf("close sqlite query %q: %w", path, err)
	}
	return strings.Join(values, "\n"), nil
}
