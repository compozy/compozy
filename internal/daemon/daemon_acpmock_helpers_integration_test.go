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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"

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

func seedNativeCompactionUpgrade(ctx context.Context, t *testing.T, h *e2etest.RuntimeHarness, id, canonical string) {
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
	// The first daemon is stopped; replace only this test-owned current database
	// with the preceding-release fixture before the second real daemon boots.
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
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

// History retains grouped raw ledger snapshots; folding belongs to transcript.
func assertNativeCompactionHistory(ctx context.Context, t *testing.T, h *e2etest.RuntimeHarness, base string) {
	t.Helper()
	var history compozycontract.SessionHistoryResponse
	if err := h.HTTPJSON(ctx, http.MethodGet, base+"/history?limit=1000", nil, &history); err != nil {
		t.Fatal(err)
	}
	snapshots := 0
	lastStatus := ""
	for _, turn := range history.History {
		for _, event := range turn.Events {
			if event.Type != "compaction" {
				continue
			}
			decoded, err := transcript.UnmarshalAgentEvent(string(event.Content))
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Compaction == nil || decoded.Compaction.CompactionID != "native-1" {
				t.Fatalf("raw history snapshot = %#v", decoded)
			}
			snapshots++
			lastStatus = decoded.Compaction.Status
		}
	}
	if snapshots < 2 || lastStatus != "completed" {
		t.Fatalf("raw snapshots = %d, latest = %q, want lifecycle rows ending completed", snapshots, lastStatus)
	}
}

func nativeCompactionHookOverlay(t *testing.T, capture string) string {
	t.Helper()
	var overlay strings.Builder
	command := `payload=$(cat); printf '%s\n' "$payload" >> "$HOOK_CAPTURE"; printf '{}'`
	for _, event := range []string{"context.pre_compact", "context.post_compact"} {
		_, err := fmt.Fprintf(
			&overlay,
			"[[hooks.declarations]]\nname = %s\nevent = %s\nmode = \"sync\"\n[hooks.declarations.executor]\ncommand = \"/bin/sh\"\nargs = [\"-c\", %s]\nenv = { HOOK_CAPTURE = %s }\n",
			strconv.Quote(event),
			strconv.Quote(event),
			strconv.Quote(command),
			strconv.Quote(capture),
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	return overlay.String()
}

// Twenty real turns exceed the replay budget while keeping integration setup small.
func seedMaintenanceCompactionReplay(t *testing.T, home compozyconfig.HomePaths, active *session.Session) {
	t.Helper()
	meta := active.Meta()
	owner, err := meta.DatabaseOwner()
	if err != nil {
		t.Fatal(err)
	}
	db, err := sessiondb.OpenSessionDB(
		t.Context(),
		owner,
		store.SessionDBFile(filepath.Join(home.SessionsDir, active.ID)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	rows := make([]store.SessionEvent, 0, 40)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for turn := range 20 {
		user := fmt.Sprintf("persisted user turn %d", turn)
		if turn == 0 {
			user = rebuildFirstUser
		}
		for index, message := range []struct{ kind, text string }{{"user_message", user}, {"agent_message", strings.Repeat("x", 10000)}} {
			rows = append(
				rows,
				store.SessionEvent{
					ID:        fmt.Sprintf("maintenance-%d-%d", turn, index),
					SessionID: active.ID,
					TurnID:    fmt.Sprintf("maintenance-turn-%d", turn),
					Type:      message.kind,
					AgentName: "compaction-claude",
					Content:   message.text,
					Timestamp: at.Add(time.Duration(turn) * time.Second),
				},
			)
		}
	}
	if _, err := db.RecordPersistedBatch(t.Context(), rows); err != nil {
		t.Fatal(err)
	}
}

// The first process has already loaded the blocking fixture. Its replacement
// receives the same native frames without the fault gate after SIGKILL.
func restoreRecoveryCompactionFixture(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(mockFixturePath(t, "native_compaction_fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
