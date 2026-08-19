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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	_ "modernc.org/sqlite"
)

const (
	plantedRedactionSecret = "sk-ant-api03-compozyredactionfixture1234567890"
	loopRedactionSecret    = "compozy_claim_LOOP_REDACTION_SECRET_1234567890"
	loopRedactionEnv       = "COMPOZY_E2E_LOOP_REDACTION_TOKEN"
	loopRedactionWorker    = "loop-redaction-worker"
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
	t.Run("Should redact Loop verdict diagnostics across every public and durable surface", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)
		testLoopVerdictRedactionBoundary(t)
	})
}

func testLoopVerdictRedactionBoundary(t *testing.T) {
	t.Helper()
	fixturePath := mockFixturePath(t, "loop_generation_feedback_fixture.json")
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Env: map[string]string{loopRedactionEnv: loopRedactionSecret},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath: fixturePath, FixtureAgent: "redaction_worker", AgentName: loopRedactionWorker,
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	definition := loopVerdictRedactionDefinition()
	createLoopViaHTTP(t, ctx, harness, definition)
	run := runFeedbackLoopViaHTTP(t, ctx, harness, definition)
	waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusExhausted)

	sqlRows, err := querySQLiteText(
		harness.HomePaths.DatabaseFile,
		"SELECT blocking_issues_json || char(10) || criteria_json FROM loop_gate_verdicts WHERE loop_run_id = ? ORDER BY generation",
		run.ID,
	)
	if err != nil {
		t.Fatalf("query Loop verdict rows error = %v", err)
	}
	assertLoopVerdictRedacted(t, "loop_gate_verdicts rows", sqlRows, true)

	statusPath := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(run.ID)
	var httpDetail compozycontract.LoopRunResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, statusPath, nil, &httpDetail); err != nil {
		t.Fatalf("HTTP Loop status error = %v", err)
	}
	assertLoopVerdictRedactedJSON(t, "HTTP Loop status", httpDetail, true)

	var udsDetail compozycontract.LoopRunResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, statusPath, nil, &udsDetail); err != nil {
		t.Fatalf("UDS Loop status error = %v", err)
	}
	assertLoopVerdictRedactedJSON(t, "UDS Loop status", udsDetail, true)

	stdout, stderr, err := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"loop", "status", "--workspace", harness.WorkspaceID, "--run-id", run.ID, "-o", "json",
	)
	if err != nil {
		t.Fatalf("CLI Loop status error = %v; stderr=%s", err, stderr)
	}
	assertLoopVerdictRedacted(t, "CLI Loop status", stdout+stderr, true)

	nativeDetail := invokeLoopRuntimeNativeStatus(t, ctx, harness, harness.WorkspaceID, run.ID)
	assertLoopVerdictRedactedJSON(t, "native Loop status", nativeDetail, true)

	events := readLoopRunSSEUntil(
		t,
		ctx,
		harness,
		loopRunEventsPath(harness.WorkspaceID, run.ID, 0),
		func(events []loopRunSSEEvent) bool {
			count := 0
			for _, event := range events {
				if event.Kind == compozycontract.LoopRunEventGateVerdict {
					count++
				}
			}
			return count >= 2
		},
	)
	assertLoopVerdictRedactedJSON(t, "Loop SSE replay", events, true)

	registration, ok := harness.MockAgentRegistration(loopRedactionWorker)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) missing", loopRedactionWorker)
	}
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	assertLoopVerdictRedactedJSON(t, "previous namespace prompt", acpmock.PromptDiagnostics(diagnostics), true)

	logPayload, err := os.ReadFile(harness.HomePaths.LogFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", harness.HomePaths.LogFile, err)
	}
	assertLoopVerdictRedacted(t, "structured Loop logs", string(logPayload), false)
	assertNoSecretInTree(t, harness.HomePaths.HomeDir, loopRedactionSecret, "Loop claim token")
}

func loopVerdictRedactionDefinition() compozycontract.LoopDefinitionDocument {
	definition := feedbackBaseDefinition("loop-verdict-redaction", "redaction draft", 2)
	definition.Graph.Nodes[0].Params["agent"] = loopRedactionWorker
	shouldRepair := dsl.Node{
		ID: "should_repair", Class: dsl.NodeClassControl, Kind: "branch",
		Condition: "generation > 1",
	}
	repair := definition.Graph.Nodes[0]
	repair.ID = "repair"
	repair.Params = map[string]any{
		"agent":  loopRedactionWorker,
		"prompt": `repair {{ toJson .previous.verdicts.contract.criteria }}`,
		"output_schema": map[string]any{
			"type": "object", "required": []any{"summary", "value"},
			"properties": map[string]any{
				"summary": map[string]any{"type": "string"},
				"value":   map[string]any{"type": "string"},
			},
		},
	}
	definition.Graph.Nodes = append(definition.Graph.Nodes, shouldRepair, repair)
	definition.Graph.Edges = []dsl.Edge{{From: "should_repair", To: "repair"}}
	// This gate intentionally always rejects so every public surface carries redacted diagnostics.
	definition.Contract.Verification = []dsl.GateCriterion{{
		ID:       "claim_guard",
		Type:     "command",
		Check:    `printf '%s' "$` + loopRedactionEnv + `"`,
		Expect:   "stdout_contains",
		Contains: "expected-redaction-sentinel",
	}}
	return definition
}

func assertLoopVerdictRedactedJSON(t testing.TB, source string, value any, requireMarker bool) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", source, err)
	}
	assertLoopVerdictRedacted(t, source, string(payload), requireMarker)
}

func assertLoopVerdictRedacted(t testing.TB, source string, payload string, requireMarker bool) {
	t.Helper()
	if strings.Contains(payload, loopRedactionSecret) {
		t.Fatalf("%s leaked Loop claim token: %s", source, payload)
	}
	if requireMarker && !strings.Contains(payload, "compozy_claim_[REDACTED]") {
		t.Fatalf("%s missing Loop claim-token redaction marker: %s", source, payload)
	}
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
	assertRedactedBoundaryPayload(t, "compozy.db event summaries", runtimeRows, true)

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
	assertNoSecretInTree(t, root, plantedRedactionSecret, "planted secret")
}

func assertNoSecretInTree(t testing.TB, root string, rawSecret string, secretLabel string) {
	t.Helper()
	secret := []byte(rawSecret)
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
			return fmt.Errorf("raw %s found in %q", secretLabel, relative)
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
