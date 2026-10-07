package testutil

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestNewHomeConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should create one home layout and derive config from it", func(t *testing.T) {
		t.Parallel()

		homePaths, cfg := NewHomeConfig(t)
		want := ConfigForTest(homePaths)

		if cfg.Daemon != want.Daemon {
			t.Fatalf("daemon config = %#v, want %#v", cfg.Daemon, want.Daemon)
		}
		if cfg.Daemon.Socket != homePaths.DaemonSocket {
			t.Fatalf("daemon socket = %q, want %q", cfg.Daemon.Socket, homePaths.DaemonSocket)
		}
		if cfg.Memory.GlobalDir != homePaths.MemoryDir {
			t.Fatalf("memory global dir = %q, want %q", cfg.Memory.GlobalDir, homePaths.MemoryDir)
		}
	})
}

func TestPerformRequestWithHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should set JSON content type only when body is present and preserve headers", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			method       string
			body         []byte
			headers      map[string]string
			wantResponse string
		}{
			{
				name:         "Should set JSON content type when body is present",
				method:       http.MethodPost,
				body:         []byte(`{"ok":true}`),
				headers:      map[string]string{"X-Trace": "trace-1"},
				wantResponse: "application/json|trace-1",
			},
			{
				name:         "Should preserve headers without forcing content type when body is absent",
				method:       http.MethodGet,
				headers:      map[string]string{"X-Trace": "trace-2"},
				wantResponse: "|trace-2",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
					_, err := w.Write([]byte(r.Header.Get("Content-Type") + "|" + r.Header.Get("X-Trace")))
					if err != nil {
						t.Fatalf("ResponseWriter.Write() error = %v", err)
					}
				})

				response := PerformRequestWithHeaders(
					t,
					handler,
					tt.method,
					"/demo",
					tt.body,
					tt.headers,
				)
				if response.Code != http.StatusCreated {
					t.Fatalf("response status = %d, want %d", response.Code, http.StatusCreated)
				}
				if got := response.Body.String(); got != tt.wantResponse {
					t.Fatalf("response body = %q, want %q", got, tt.wantResponse)
				}
			})
		}
	})
}

func TestParseSSE(t *testing.T) {
	t.Parallel()

	t.Run("Should support multiple data lines and final record without blank line", func(t *testing.T) {
		t.Parallel()

		records := ParseSSE(t, strings.Join([]string{
			"id: 1",
			"event: chunk",
			`data: {"delta":"a"}`,
			`data: {"delta":"b"}`,
			"",
			"id: 2",
			"event: done",
			`data: {"ok":true}`,
		}, "\n"))

		if got, want := len(records), 2; got != want {
			t.Fatalf("len(records) = %d, want %d", got, want)
		}
		if records[0].ID != "1" || records[0].Event != "chunk" {
			t.Fatalf("first record = %#v, want id=1 event=chunk", records[0])
		}
		if got, want := string(records[0].Data), "{\"delta\":\"a\"}\n{\"delta\":\"b\"}"; got != want {
			t.Fatalf("first record data = %q, want %q", got, want)
		}
		if records[1].ID != "2" || records[1].Event != "done" || string(records[1].Data) != `{"ok":true}` {
			t.Fatalf("final record = %#v, want done record", records[1])
		}
	})

	t.Run("Should ignore empty frames and accept field lines without a space", func(t *testing.T) {
		t.Parallel()

		records := ParseSSE(t, strings.Join([]string{
			"",
			"id:3",
			"event:chunk",
			`data:{"delta":"a"}`,
			"",
			"",
			"id: 4",
			"event: done",
			`data: {"ok":true}`,
		}, "\n"))

		if got, want := len(records), 2; got != want {
			t.Fatalf("len(records) = %d, want %d", got, want)
		}
		if records[0].ID != "3" || records[0].Event != "chunk" || string(records[0].Data) != `{"delta":"a"}` {
			t.Fatalf("first record = %#v, want compact-field frame", records[0])
		}
		if records[1].ID != "4" || records[1].Event != "done" || string(records[1].Data) != `{"ok":true}` {
			t.Fatalf("second record = %#v, want spaced-field frame", records[1])
		}
	})

	t.Run("Should parse a single data line larger than the scanner default token limit", func(t *testing.T) {
		t.Parallel()

		largeData := strings.Repeat("x", 70*1024)
		records := ParseSSE(t, strings.Join([]string{
			"id: large",
			"event: chunk",
			"data: " + largeData,
			"",
		}, "\n"))

		if got, want := len(records), 1; got != want {
			t.Fatalf("len(records) = %d, want %d", got, want)
		}
		if records[0].ID != "large" || records[0].Event != "chunk" {
			t.Fatalf("record metadata = %#v, want large chunk", records[0])
		}
		if got := string(records[0].Data); got != largeData {
			t.Fatalf("record data length = %d, want %d", len(got), len(largeData))
		}
	})
}

func TestStubTaskManagerFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve filtered catalog continuation metadata", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
		stub := &StubTaskManager{ListTasksFn: func(
			_ context.Context,
			gotQuery taskpkg.Query,
			_ taskpkg.ActorContext,
		) ([]taskpkg.Summary, error) {
			if gotQuery.ReadScope != (storepkg.ReadScope{ProfileID: storepkg.DefaultProfileID}) {
				t.Fatalf("ListTasks() read scope = %#v, want default profile", gotQuery.ReadScope)
			}
			return []taskpkg.Summary{
				{
					ID: "task-1", Priority: taskpkg.PriorityHigh, LastActivityAt: now,
					ActiveRun: &taskpkg.RunSummary{},
				},
				{
					ID: "task-2", Priority: taskpkg.PriorityMedium, LastActivityAt: now.Add(-time.Minute),
					ActiveRun: &taskpkg.RunSummary{},
				},
			}, nil
		}}
		query := taskpkg.CatalogQuery{
			ReadScope: storepkg.ReadScope{ProfileID: storepkg.DefaultProfileID},
			Scope:     taskpkg.CatalogScopeGlobal,
			Sort:      taskpkg.CatalogSortRecent,
			Limit:     1,
		}
		page, err := stub.ListTaskCatalog(t.Context(), query, taskpkg.ActorContext{})
		if err != nil {
			t.Fatalf("ListTaskCatalog() error = %v", err)
		}
		if got, want := len(page.Tasks), 1; got != want {
			t.Fatalf("len(page.Tasks) = %d, want %d", got, want)
		}
		if page.Total != 2 || !page.HasMore || page.NextCursor == "" {
			t.Fatalf("page metadata = %#v, want total two with continuation", page)
		}
		cursorQuery := query
		cursorQuery.Cursor = page.NextCursor
		if _, err := taskpkg.DecodeCatalogCursor(cursorQuery); err != nil {
			t.Fatalf("DecodeCatalogCursor() error = %v", err)
		}
	})
}
