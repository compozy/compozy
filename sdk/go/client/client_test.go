package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClientDiscovery(t *testing.T) {
	// not parallel: discovery cases temporarily change environment variables and the working directory.
	t.Setenv("COMPOZY_URL", "")
	t.Setenv("COMPOZY_APP_TOKEN", "")
	t.Run("Should prefer explicit URL over the environment and local home", func(t *testing.T) {
		explicit := identityServer(t, "0.4.1")
		environment := identityServer(t, "0.5.0")
		t.Setenv("COMPOZY_URL", environment.URL)
		c, err := New(t.Context(), WithBaseURL(explicit.URL), WithHome(t.TempDir()))
		if err != nil || c.DaemonVersion() != "0.4.1" {
			t.Fatalf("New() = %v, %v; want explicit daemon", c, err)
		}
	})
	t.Run("Should prefer the environment URL over a socket", func(t *testing.T) {
		environment := identityServer(t, "0.4.2")
		t.Setenv("COMPOZY_URL", environment.URL)
		c, err := New(t.Context(), WithSocket(filepath.Join(t.TempDir(), "missing.sock")))
		if err != nil || c.DaemonVersion() != "0.4.2" {
			t.Fatalf("New() = %v, %v; want environment daemon", c, err)
		}
	})
	t.Run("Should prefer a live home socket over the TCP record", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("unix socket discovery")
		}
		home := t.TempDir()
		t.Chdir(home)
		serveUnixIdentity(t, "daemon.sock", "0.4.3")
		writeDaemonRecord(t, home, identityServer(t, "0.4.4").URL, true)
		t.Setenv("COMPOZY_HOME", t.TempDir())
		c, err := New(t.Context(), WithHome("."))
		if err != nil || c.DaemonVersion() != "0.4.3" {
			t.Fatalf("New() = %v, %v; want unix daemon from explicit home", c, err)
		}
	})
	t.Run("Should try TCP after a stale socket exhausts its identity timeout", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("unix socket discovery")
		}
		home := t.TempDir()
		t.Chdir(home)
		serveUnixHandler(t, "daemon.sock", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		writeDaemonRecord(t, home, identityServer(t, "0.4.6").URL, true)
		c, err := New(t.Context(), WithHome("."))
		if err != nil || c.DaemonVersion() != "0.4.6" {
			t.Fatalf("New() = %v, %v; want TCP fallback after socket timeout", c, err)
		}
	})
	for _, explicitHost := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should fall back to the TCP record with explicit host %t", explicitHost),
			func(t *testing.T) {
				home := t.TempDir()
				writeDaemonRecord(t, home, identityServer(t, "0.4.5").URL, explicitHost)
				t.Setenv("COMPOZY_HOME", home)
				c, err := New(t.Context())
				if err != nil || c.DaemonVersion() != "0.4.5" {
					t.Fatalf("New() = %v, %v; want TCP daemon", c, err)
				}
			},
		)
	}
	t.Run("Should list both checked paths when no daemon exists", func(t *testing.T) {
		home := t.TempDir()
		_, err := New(t.Context(), WithHome(home))
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("New() error = %v, want ErrUnavailable", err)
		}
		for _, name := range []string{"daemon.sock", "daemon.json"} {
			if !strings.Contains(err.Error(), filepath.Join(home, name)) {
				t.Fatalf("New() error = %v, missing checked %s", err, name)
			}
		}
	})
	t.Run("Should retain caller cancellation during discovery", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := New(ctx, WithBaseURL(identityServer(t, "0.4.0").URL))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("New() error = %v, want context.Canceled", err)
		}
	})
	for _, recordHost := range []string{"0.0.0.0", "::", "[::]"} {
		t.Run("Should connect wildcard listener "+recordHost+" through loopback", func(t *testing.T) {
			home := t.TempDir()
			server := identityServer(t, "0.4.5")
			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(serverURL.Port())
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"http_host": recordHost, "port": port})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "daemon.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			wantHost := "::1"
			if recordHost == "0.0.0.0" {
				wantHost = "127.0.0.1"
			}
			want := "http://" + net.JoinHostPort(wantHost, serverURL.Port())
			if got, err := recordURL(home); err != nil || got != want {
				t.Fatalf("recordURL() = %q, %v; want %q", got, err, want)
			}
			if recordHost == "0.0.0.0" {
				if c, err := New(t.Context(), WithHome(home)); err != nil || c.DaemonVersion() != "0.4.5" {
					t.Fatalf("New(wildcard record) = %v, %v", c, err)
				}
			}
		})
	}
}

func TestClientRequests(t *testing.T) {
	t.Parallel()
	t.Run("Should report a lost daemon without classifying caller cancellation as unavailable", func(t *testing.T) {
		t.Parallel()
		server := identityServer(t, "0.4.0")
		c, err := New(t.Context(), WithBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		server.Close()
		if err := c.Do(t.Context(), http.MethodGet, "/api/test", nil, nil); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("Do() error = %v, want ErrUnavailable", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		cancel()
		err = c.Do(ctx, http.MethodGet, "/api/test", nil, nil)
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnavailable) {
			t.Fatalf("canceled Do() error = %v", err)
		}
	})
	t.Run("Should preserve escaped paths headers and daemon error fields", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.EscapedPath() != "/api/workspaces/a%2Fb" || r.URL.Query().Get("name") != "a b" {
				t.Errorf("request URL = %s", r.URL)
			}
			if r.Header.Get("Authorization") != "Bearer app-token" ||
				r.Header.Get("X-Compozy-Client-ID") != "test-app" {
				t.Errorf("request headers = %v", r.Header)
			}
			w.WriteHeader(http.StatusConflict)
			writeResponse(t, w, `{"error":"busy","code":"session_busy","current_turn_id":"t1","details":{"queue":2}}`)
		}, WithToken("app-token"), WithClientID("test-app"))
		err := c.Do(
			t.Context(),
			http.MethodPost,
			"/api/workspaces/a%2Fb?name=a+b",
			map[string]string{"message": "hello"},
			nil,
		)
		apiErr, ok := errors.AsType[*APIError](err)
		if !ok || apiErr.Status != 409 || apiErr.Code != "session_busy" || apiErr.Message != "busy" ||
			apiErr.CurrentTurnID != "t1" ||
			apiErr.Details["queue"] != float64(2) {
			t.Fatalf("Do() error = %#v, want preserved API error", err)
		}
		if !strings.Contains(apiErr.Error(), "session_busy") {
			t.Fatalf("Error() = %q", apiErr.Error())
		}
	})
	t.Run("Should preserve a non-JSON error response status", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			writeResponse(t, w, "upstream unavailable")
		})
		err := c.Do(t.Context(), http.MethodGet, "/api/test", nil, nil)
		apiErr, ok := errors.AsType[*APIError](err)
		if !ok || apiErr.Status != 502 || apiErr.Message != "Bad Gateway" {
			t.Fatalf("Do() error = %v", err)
		}
	})
	t.Run("Should decode JSON strings and preserve future enum values", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { writeResponse(t, w, `"future-value"`) })
		var value string
		if err := c.Do(t.Context(), http.MethodGet, "/api/test", nil, &value); err != nil || value != "future-value" {
			t.Fatalf("Do() = %q, %v", value, err)
		}
	})
	t.Run("Should upload a caller-owned reader and decode raw bytes", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("FormFile() error = %v", err)
				w.WriteHeader(400)
				return
			}
			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil || closeErr != nil || header.Filename != "hello.txt" ||
				header.Header.Get("Content-Type") != "text/plain" ||
				string(data) != "hello" {
				t.Errorf("upload = %v %q, errors = %v %v", header, data, readErr, closeErr)
			}
			writeResponse(t, w, "raw-result")
		})
		var out []byte
		err := c.doContent(
			t.Context(),
			http.MethodPost,
			"/api/test",
			nil,
			&UploadFile{Name: "hello.txt", ContentType: "text/plain", Reader: strings.NewReader("hello")},
			&out,
			"multipart/form-data",
			"*/*",
		)
		if err != nil || string(out) != "raw-result" {
			t.Fatalf("upload result = %q, %v", out, err)
		}
	})
	t.Run("Should honor an explicit deadline for an HTTP wait beyond thirty seconds", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			timer := time.NewTimer(31 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				writeResponse(t, w, `"completed"`)
			case <-r.Context().Done():
			}
		})
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		var result string
		if err := c.Do(ctx, http.MethodGet, "/api/wait", nil, &result); err != nil || result != "completed" {
			t.Fatalf("Do(long wait) = %q, %v", result, err)
		}
	})
}

func TestClientGeneratedOperations(t *testing.T) {
	t.Parallel()
	// Invariant: the original no-options workspace list call remains source-compatible.
	t.Run("Should list workspaces with or without the archived filter", func(t *testing.T) {
		t.Parallel()
		archived := true
		for _, options := range [][]ListWorkspacesParams{nil, {{Archived: &archived}}} {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				want := ""
				if len(options) > 0 {
					want = "true"
				}
				if r.URL.Path != "/api/workspaces" || r.URL.Query().Get("archived") != want {
					t.Errorf("workspace list request = %s", r.URL.String())
				}
				writeResponse(t, w, `{"workspaces":[]}`)
			})
			var err error
			if options == nil {
				var legacy interface {
					ListWorkspaces(context.Context) (*WorkspacesResponse, error)
				} = c
				_, err = legacy.ListWorkspaces(t.Context())
			} else {
				_, err = c.ListWorkspacesWithParams(t.Context(), options[0])
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("Should preserve workspace configuration error details for callers", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/workspaces" {
				t.Errorf("workspace request = %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			writeResponse(t, w, `{"error":"Remove unknown keys","code":"workspace_config_invalid",`+
				`"config_path":"/project/.compozy/config.toml","unknown_keys":["defaults.ide","tasks.run"]}`)
		})
		_, err := c.CreateWorkspace(t.Context(), CreateWorkspaceRequest{RootDir: "/project"})
		apiErr, ok := errors.AsType[*APIError](err)
		if !ok || apiErr.Status != http.StatusUnprocessableEntity || apiErr.Code != "workspace_config_invalid" ||
			apiErr.ConfigPath != "/project/.compozy/config.toml" ||
			!slices.Equal(apiErr.UnknownKeys, []string{"defaults.ide", "tasks.run"}) {
			t.Fatalf("CreateWorkspace() error = %#v, want complete config error details", err)
		}
	})

	// Invariant: distinct successful tool responses retain their HTTP discriminator and complete body.
	// Owner: generated Go client transport; canonical suite: generated operations over real HTTP.
	for _, test := range []struct {
		status int
		wire   string
	}{
		{http.StatusOK, `{"tool_id":"compozy__session_list","status":"completed","result":{"content":[]}}`},
		{http.StatusAccepted, `{"error":{"code":"tool_approval_required","message":"Approval required","details":{"approval_token":"approve-1"}}}`},
	} {
		t.Run(fmt.Sprintf("Should preserve tool response status %d and its payload", test.status), func(t *testing.T) {
			t.Parallel()
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/tools/compozy__session_list/invoke" {
					t.Errorf("tool request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(test.status)
				writeResponse(t, w, test.wire)
			})
			result, err := c.InvokeTool(t.Context(), InvokeToolParams{ID: "compozy__session_list"}, ToolInvokeRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if result.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", result.StatusCode, test.status)
			}
			if test.status == http.StatusAccepted {
				approval, err := result.AsToolErrorResponse()
				if err != nil || approval.Error.Code != "tool_approval_required" ||
					string(approval.Error.Details["approval_token"]) != `"approve-1"` {
					t.Fatalf("approval payload = %#v, %v", approval, err)
				}
			} else {
				completed, err := result.AsToolInvokeResponse()
				if err != nil || completed.ToolID != "compozy__session_list" || completed.Status != "completed" {
					t.Fatalf("completed payload = %#v, %v", completed, err)
				}
			}
		})
	}
	for _, test := range []struct {
		name     string
		params   ListSessionsParams
		explicit bool
	}{
		{"Should retain explicit zero and false query values", ListSessionsParams{IncludeHealth: new(false), Limit: new(0)}, true},
		{"Should omit nil query values", ListSessionsParams{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				if test.explicit {
					if query.Get("limit") != "0" || query.Get("include_health") != "false" {
						t.Errorf("explicit query = %v", query)
					}
				} else if query.Has("limit") || query.Has("include_health") {
					t.Errorf("nil parameters were sent: %v", query)
				}
				writeResponse(t, w, `{"sessions":[]}`)
			})
			if _, err := c.ListSessions(t.Context(), test.params); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("Should escape path components and preserve repeated query arrays", func(t *testing.T) {
		t.Parallel()
		include := []string{"a b.go", "src/a&b.go"}
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.EscapedPath() != "/api/workspaces/work%2Fspace%25value/worktrees/tree%2Fa%252Fb/exit" {
				t.Errorf("escaped path = %s", r.URL.EscapedPath())
			}
			if values := r.URL.Query()["include"]; !slices.Equal(values, include) {
				t.Errorf("include query = %v, want %v", values, include)
			}
			writeResponse(t, w, `{}`)
		})
		_, err := c.GetWorktreeExitPlan(
			t.Context(),
			GetWorktreeExitPlanParams{WorkspaceID: "work/space%value", WorktreeID: "tree/a%2Fb", Include: &include},
		)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should send stream cursors in headers and retain raw typed frame payloads", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.EscapedPath() != "/api/workspaces/ws%2Fa%25/sessions/session%2Fb%25/stream" {
				t.Errorf("escaped stream path = %s", r.URL.EscapedPath())
			}
			if r.Header.Get("Last-Event-ID") != "42" || r.URL.Query().Get("after_sequence") != "0" ||
				r.URL.Query().Get("limit") != "0" {
				t.Errorf("stream cursor headers=%v query=%v", r.Header, r.URL.Query())
			}
			w.Header().Set("Content-Type", "text/event-stream")
			writeResponse(t, w, "id: 43\nevent: session_event\ndata: {\"session_id\":\"session\",\"sequence\":43}\n\n")
		})
		s, err := c.StreamSession(t.Context(), StreamSessionParams{
			WorkspaceID:   "ws/a%",
			SessionID:     "session/b%",
			LastEventID:   new("42"),
			AfterSequence: new(int64(0)),
			Limit:         new(0),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
		frame, err := s.Next()
		if err != nil || frame.Event != "session_event" || frame.ID != "43" {
			t.Fatalf("Next() = %#v, %v", frame, err)
		}
		data, err := json.Marshal(frame.Data)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			SessionID string `json:"session_id"`
			Sequence  int64  `json:"sequence"`
		}
		if err := json.Unmarshal(
			data,
			&payload,
		); err != nil || payload.SessionID != "session" ||
			payload.Sequence != 43 {
			t.Fatalf("frame payload = %s, %v", data, err)
		}
	})
	// Invariant: public DTOs preserve the daemon's scalar, union, list and null JSON forms.
	// Owner: generated Go client codec; canonical generated-operations suite.
	for _, test := range []struct {
		name, wire string
		target     any
	}{
		{"task status", `"running"`, new(TaskRunStatus)},
		{"run kind", `"worker"`, new(RunKind)},
		{"percent threshold", `"66%"`, new(StrategyThreshold)},
		{"count threshold", `{"count":2}`, new(StrategyThreshold)},
		{"strategy shorthand", `"all"`, new(StrategySpec)},
		{"strategy object", `{"kind":"quorum","threshold":"66%"}`, new(StrategySpec)},
		{"stop expression", `"score >= 0.8"`, new(StopWhenSpec)},
		{"stop policy", `{"expr":"score >= 0.8","on_eval_error":"fail"}`, new(StopWhenSpec)},
		{"source list", `["~/.agents/skills"]`, new(OptionalStringList)},
		{"empty source list", `[]`, new(OptionalStringList)},
		{"null source list", `null`, new(OptionalStringList)},
		{"explicit null override", `{"override":{"sources":null}}`, new(UpdateSettingsSkillsRequest)},
		{"empty override", `{"override":{}}`, new(UpdateSettingsSkillsRequest)},
		{"omitted provider environment", `{}`, new(SettingsProviderWritePayload)},
		{"cleared provider environment", `{"session_env":{}}`, new(SettingsProviderWritePayload)},
	} {
		t.Run("Should round trip "+test.name+" through its public JSON form", func(t *testing.T) {
			t.Parallel()
			if err := json.Unmarshal([]byte(test.wire), test.target); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(test.target)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(test.wire), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Marshal() = %s, want equivalent JSON %s", encoded, test.wire)
			}
		})
	}
}

func TestClientPrompt(t *testing.T) {
	t.Parallel()
	t.Run("Should expose accepted input and retain retry identity and unknown modes", func(t *testing.T) {
		t.Parallel()
		messageIDs := make(chan string, 2)
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode prompt: %v", err)
			}
			if body["idempotency_key"] != "retry-key" || body["message_id"] == "" || body["message"] != "hello" ||
				body["expected_turn_id"] != "turn-1" {
				t.Errorf("prompt body = %v", body)
			}
			messageIDs <- body["message_id"]
			w.WriteHeader(http.StatusAccepted)
			writeResponse(
				t,
				w,
				`{"prompt":{"status":"queued","mode":"future-mode","delivery":"after_turn","queue_position":1}}`,
			)
		})
		for range 2 {
			s, err := c.Prompt(
				t.Context(),
				PromptInput{
					WorkspaceID:    "ws",
					SessionID:      "session",
					Message:        "hello",
					IdempotencyKey: "retry-key",
					ExpectedTurnID: "turn-1",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			accepted, ok := s.Accepted()
			if !ok || accepted.Status != "queued" || accepted.Mode != "future-mode" || accepted.QueuePosition != 1 {
				t.Fatalf("Accepted() = %#v, %t", accepted, ok)
			}
			if _, err := s.Next(); !errors.Is(err, io.EOF) {
				t.Fatalf("Next() error = %v, want EOF", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if first, second := <-messageIDs, <-messageIDs; first != second {
			t.Fatalf("retry changed message identity: %q then %q", first, second)
		}
	})
	t.Run("Should yield all prompt chunks and generated command identity", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			if err := json.NewDecoder(r.Body).
				Decode(&body); err != nil || body["idempotency_key"] == "" ||
				body["message_id"] == "" {
				t.Errorf("prompt body = %v, %v", body, err)
			}
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			writeResponse(
				t,
				w,
				"data: {\"type\":\"start\"}\n\ndata: {\"type\":\"text-delta\",\"delta\":\"hello\"}\n\ndata: {\"type\":\"text-delta\",\"delta\":\" world\"}\n\ndata: {\"type\":\"finish\"}\n\ndata: [DONE]\n\n",
			)
		})
		s := testPrompt(t.Context(), t, c)
		if _, ok := s.Accepted(); ok {
			t.Fatal("stream unexpectedly accepted")
		}
		var text strings.Builder
		for _, expected := range []string{"start", "text-delta", "text-delta", "finish"} {
			chunk, err := s.Next()
			if err != nil || chunk.Type != expected || !json.Valid(chunk.Raw) {
				t.Fatalf("Next() = %#v, %v; want %s", chunk, err, expected)
			}
			text.WriteString(chunk.TextDelta())
		}
		if _, err := s.Next(); !errors.Is(err, io.EOF) || text.String() != "hello world" {
			t.Fatalf("stream result = %q, %v", text.String(), err)
		}
	})
	t.Run("Should preserve context cancellation after the first chunk", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			writeResponse(t, w, "data: {\"type\":\"start\"}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s := testPrompt(ctx, t, c)
		if _, err := s.Next(); err != nil {
			t.Fatal(err)
		}
		cancel()
		_, err := s.Next()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Next() error = %v, want context.Canceled", err)
		}
	})
	for _, test := range []struct{ name, payload string }{
		{"Should reject malformed prompt JSON", "data: {broken}\n\n"},
		{"Should reject missing prompt chunk type", "data: {}\n\n"},
		{"Should reject a prompt body that ends before completion", "data: {\"type\":\"start\"}\n\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				writeResponse(t, w, test.payload)
			})
			s := testPrompt(t.Context(), t, c)
			_, err := s.Next()
			if err == nil {
				_, err = s.Next()
			}
			streamErr, ok := errors.AsType[*StreamError](err)
			if !ok || streamErr.Op != "sendSessionPrompt" {
				t.Fatalf("Next() error = %v, want StreamError", err)
			}
		})
	}
}

func TestClientStreams(t *testing.T) {
	t.Parallel()
	t.Run("Should parse SSE metadata multiline data and all line endings", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			writeResponse(
				t,
				w,
				"\ufeff: heartbeat\r\nid: 123\revent: update\rdata: {\ndata: \"value\": \"hello\"}\n\ndata: {\"value\":\"again\"}\r\n\r\ndata: [DONE]\n\n",
			)
		})
		type payload struct {
			Value string `json:"value"`
		}
		s, err := newStream[payload](t.Context(), c, http.MethodGet, "/api/events", nil, nil, "streamTest")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
		frame, err := s.Next()
		if err != nil || frame.Event != "update" || frame.ID != "123" || frame.Data.Value != "hello" {
			t.Fatalf("Next() = %#v, %v", frame, err)
		}
		frame, err = s.Next()
		if err != nil || frame.Event != "" || frame.ID != "123" || frame.Data.Value != "again" {
			t.Fatalf("Next() = %#v, %v", frame, err)
		}
		if _, err := s.Next(); !errors.Is(err, io.EOF) {
			t.Fatalf("Next() error = %v", err)
		}
	})
	t.Run("Should stop with a typed API error before stream parsing", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			writeResponse(t, w, `{"error":"revoked","code":"app_token_revoked"}`)
		})
		_, err := newStream[json.RawMessage](t.Context(), c, http.MethodGet, "/api/events", nil, nil, "streamTest")
		apiErr, ok := errors.AsType[*APIError](err)
		if !ok || apiErr.Status != 401 || apiErr.Code != "app_token_revoked" {
			t.Fatalf("newStream() error = %v", err)
		}
	})
	t.Run("Should expose settings log tail as a typed SSE operation", func(t *testing.T) {
		t.Parallel()
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/settings/observability/log-tail" || r.Header.Get("Accept") != "text/event-stream" {
				t.Errorf("stream request = %s, accept = %q", r.URL.Path, r.Header.Get("Accept"))
			}
			w.Header().Set("Content-Type", "text/event-stream")
			writeResponse(t, w, "event: log\ndata: {\"line\":\"ready\"}\n\n")
		})
		stream, err := c.StreamSettingsObservabilityLogTail(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := stream.Close(); err != nil {
				t.Error(err)
			}
		}()
		frame, err := stream.Next()
		if err != nil || frame.Event != "log" || frame.Data.Line != "ready" {
			t.Fatalf("log frame = %+v, %v", frame, err)
		}
	})
}

func TestClientVersions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, client, daemon string
		warning              bool
	}{
		{"Should warn once when major or minor differs", "0.4.0", "0.5.1", true},
		{"Should ignore prerelease and patch differences", "0.4.0", "0.4.0-beta.3", false},
		{"Should skip development versions", "(devel)", "0.5.1", false},
		{"Should skip the unpublished client placeholder", "0.0.0-dev", "0.5.1", false},
		{"Should skip the unpublished daemon placeholder", "0.4.0", "0.0.0-dev", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			warnVersionMismatch(slog.New(slog.NewTextHandler(&out, nil)), test.client, test.daemon)
			if test.warning {
				if strings.Count(out.String(), "level=WARN") != 1 || !strings.Contains(out.String(), test.client) ||
					!strings.Contains(out.String(), test.daemon) {
					t.Fatalf("warning = %q", out.String())
				}
			} else if out.Len() != 0 {
				t.Fatalf("unexpected warning = %q", out.String())
			}
		})
	}
}

func testClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status/identity" {
			writeResponse(t, w, `{"daemon":{"version":"0.4.0"}}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	options := []Option{
		WithBaseURL(server.URL),
		WithToken(""),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	}
	c, err := New(t.Context(), append(options, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testPrompt(ctx context.Context, t *testing.T, c *Client) *PromptStream {
	t.Helper()
	s, err := c.Prompt(ctx, PromptInput{WorkspaceID: "workspace", SessionID: "session", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func identityServer(t *testing.T, version string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, fmt.Sprintf(`{"daemon":{"version":%q}}`, version))
	}))
	t.Cleanup(server.Close)
	return server
}

func serveUnixIdentity(t *testing.T, path, version string) {
	t.Helper()
	serveUnixHandler(t, path, func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, fmt.Sprintf(`{"daemon":{"version":%q}}`, version))
	})
}

func serveUnixHandler(t *testing.T, path string, handler http.HandlerFunc) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	if err := server.Listener.Close(); err != nil {
		t.Fatal(err)
	}
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
}

func writeDaemonRecord(t *testing.T, home, serverURL string, explicitHost bool) {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	record := map[string]any{"port": port}
	if explicitHost {
		record["http_host"] = u.Hostname()
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "daemon.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write response: %v", err)
	}
}
