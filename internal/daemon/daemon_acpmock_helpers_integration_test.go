//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/transcript"
)

func mockFixturePath(t testing.TB, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testutil", "acpmock", "testdata", name)
}

func createFixtureBackedSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
	name string,
) compozycontract.SessionPayload {
	t.Helper()

	session, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     agentName,
		Name:          name,
		WorkspacePath: harness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession(%q) error = %v", agentName, err)
	}
	active, err := harness.WaitForSessionActive(ctx, session.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v", session.ID, err)
	}
	return active
}

func createBoundFixtureBackedSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
	name string,
) compozycontract.SessionPayload {
	t.Helper()

	active := createFixtureBackedSession(t, ctx, harness, agentName, name)
	if _, err := harness.PromptSession(ctx, active.ID, "noop"); err != nil {
		t.Fatalf("PromptSession(%q bootstrap) error = %v", active.ID, err)
	}
	bound, err := harness.GetSession(ctx, active.ID)
	if err != nil {
		t.Fatalf("GetSession(%q bound) error = %v", active.ID, err)
	}
	return bound
}

func bindFixtureBackedSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	created compozycontract.SessionPayload,
	message string,
) compozycontract.SessionPayload {
	t.Helper()

	if _, err := harness.PromptSession(ctx, created.ID, message); err != nil {
		t.Fatalf("PromptSession(%q) error = %v", created.ID, err)
	}
	bound, err := harness.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSession(%q) after runtime bind error = %v", created.ID, err)
	}
	return bound
}
func createSessionHTTPFailure(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request compozycontract.CreateSessionRequest,
) (int, compozycontract.ErrorPayload) {
	t.Helper()

	response := postSessionHTTP(t, ctx, harness, request)
	return response.StatusCode, decodeSessionHTTPResponse[compozycontract.ErrorPayload](t, response)
}

func createSessionHTTPAccepted(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request compozycontract.CreateSessionRequest,
) (int, compozycontract.SessionPayload) {
	t.Helper()

	response := postSessionHTTP(t, ctx, harness, request)
	payload := decodeSessionHTTPResponse[compozycontract.SessionResponse](t, response)
	return response.StatusCode, payload.Session
}

func decodeSessionHTTPResponse[T any](t testing.TB, response *http.Response) T {
	t.Helper()
	var payload T
	decodeErr := json.NewDecoder(response.Body).Decode(&payload)
	closeErr := response.Body.Close()
	if err := errors.Join(decodeErr, closeErr); err != nil {
		t.Fatalf("decode/close HTTP create session response error = %v", err)
	}
	return payload
}

func postSessionHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request compozycontract.CreateSessionRequest,
) *http.Response {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(create session request) error = %v", err)
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		harness.HTTPURL("/api/sessions"),
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(create session) error = %v", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := harness.HTTPClient.Do(httpRequest)
	if err != nil {
		t.Fatalf("HTTP create session error = %v", err)
	}
	return response
}

func providerModelListHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	providerID string,
	view string,
) (int, compozycontract.ProviderModelListResponse) {
	t.Helper()

	path := "/api/model-catalog/providers/" + url.PathEscape(providerID) + "/models"
	if view != "" {
		path += "?view=" + url.QueryEscape(view)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(provider models) error = %v", err)
	}
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatalf("HTTP provider models error = %v", err)
	}
	var payload compozycontract.ProviderModelListResponse
	decodeErr := json.NewDecoder(response.Body).Decode(&payload)
	closeErr := response.Body.Close()
	if decodeErr != nil {
		t.Fatalf("decode HTTP provider models error = %v", decodeErr)
	}
	if closeErr != nil {
		t.Fatalf("close HTTP provider models body error = %v", closeErr)
	}
	return response.StatusCode, payload
}

func joinTranscriptContent(messages []transcript.UIMessage) string {
	return transcript.JoinUIMessageText(messages)
}

func sessionTranscriptMessages(response compozycontract.SessionTranscriptResponse) []transcript.UIMessage {
	return transcript.MessagesFromEntries(response.Entries)
}

func mustSessionTranscript(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) compozycontract.SessionTranscriptResponse {
	t.Helper()

	response, err := harness.SessionTranscript(ctx, sessionID)
	if err != nil {
		t.Fatalf("SessionTranscript(%q) error = %v", sessionID, err)
	}
	return response
}

func sessionTranscriptHasNeedle(
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	needle string,
) bool {
	response, err := harness.SessionTranscript(ctx, sessionID)
	if err != nil {
		return false
	}
	return strings.Contains(joinTranscriptContent(sessionTranscriptMessages(response)), needle)
}

func mustSessionEvents(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) compozycontract.SessionEventsResponse {
	t.Helper()

	events, err := harness.SessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("SessionEvents(%q) error = %v", sessionID, err)
	}
	return events
}

func seedNativeCompactionUpgrade(t *testing.T, ctx context.Context, h *e2etest.RuntimeHarness, id, canonical string) {
	t.Helper()
	stream := sessiondb.MigrationStream()
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{}
	directory := &atlasmigrate.MemDir{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") || name >= "00009_unarchive_compaction_spans.sql" {
			continue
		}
		content, err := fs.ReadFile(stream.FS, stream.Dir+"/"+name)
		if err != nil {
			t.Fatal(err)
		}
		if err := directory.WriteFile(name, content); err != nil {
			t.Fatal(err)
		}
		files[stream.Dir+"/"+name] = &fstest.MapFile{Data: content}
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := checksum.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	files[stream.Dir+"/"+atlasmigrate.HashFileName] = &fstest.MapFile{Data: hash}
	stream.FS = files
	stream.Bootstrap = nil
	path := store.SessionDBFile(filepath.Join(h.HomePaths.SessionsDir, id))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := store.Apply(ctx, db, stream); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO session_db_owner (singleton,session_id,workspace_id) VALUES (1,?,?)`,
		id,
		h.WorkspaceID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO events (id,turn_id,type,agent_name,content,timestamp,sequence,archived) VALUES ('legacy-fired','legacy-turn','session.compaction_fired','compaction-claude',?, ?,1,0)`,
		canonical,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}
}
