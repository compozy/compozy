package testutil

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestNewDisabledNetworkHomeConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should create one home layout and derive config from it", func(t *testing.T) {
		t.Parallel()

		homePaths, cfg := NewDisabledNetworkHomeConfig(t)
		want := ConfigWithDisabledNetwork(homePaths)

		if cfg.Network.Enabled {
			t.Fatal("config network enabled = true, want false")
		}
		if cfg.Daemon != want.Daemon {
			t.Fatalf("daemon config = %#v, want %#v", cfg.Daemon, want.Daemon)
		}
		if cfg.Daemon.Socket != homePaths.DaemonSocket {
			t.Fatalf("daemon socket = %q, want %q", cfg.Daemon.Socket, homePaths.DaemonSocket)
		}
		if cfg.Memory.GlobalDir != homePaths.MemoryDir {
			t.Fatalf("memory global dir = %q, want %q", cfg.Memory.GlobalDir, homePaths.MemoryDir)
		}
		if cfg.Network != want.Network {
			t.Fatalf("network config = %#v, want %#v", cfg.Network, want.Network)
		}
	})
}

func TestStubSessionManagerList(t *testing.T) {
	t.Parallel()

	t.Run("Should return empty slice on fallback error", func(t *testing.T) {
		t.Parallel()

		manager := StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, errors.New("boom")
			},
		}

		got := manager.List()
		if got == nil {
			t.Fatal("List() = nil, want empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("len(List()) = %d, want 0", len(got))
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

func TestStubNetworkServiceWaitInboxFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should return the sentinel wait-inbox error", func(t *testing.T) {
		t.Parallel()

		_, err := StubNetworkService{}.WaitInbox(context.Background(), "sess-1", "builders")
		if !errors.Is(err, ErrStubNetworkServiceWaitInboxNotImplemented) {
			t.Fatalf("WaitInbox() error = %v, want %v", err, ErrStubNetworkServiceWaitInboxNotImplemented)
		}
	})
}

func TestStubTaskManagerFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("Should report a missing task before any run exists", func(t *testing.T) {
		t.Parallel()

		_, err := (&StubTaskManager{}).EnqueueRun(
			context.Background(),
			taskpkg.EnqueueRun{},
			taskpkg.ActorContext{},
		)
		if !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("EnqueueRun() error = %v, want %v", err, taskpkg.ErrTaskNotFound)
		}
	})

	t.Run("Should preserve filtered catalog continuation metadata", func(t *testing.T) {
		t.Parallel()

		liveSpec := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-catalog",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "builders",
			Source:          participation.SourceExplicitRequest,
			Bounds: participation.Bounds{
				MaxWakes:         1,
				MaxWakeWallTime:  "30s",
				MaxTotalWallTime: "1m",
				MaxInputTokens:   1024,
				MaxOutputTokens:  512,
				MaxWakeDepth:     1,
				CoalesceWindow:   "250ms",
			},
		}
		now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
		stub := &StubTaskManager{ListTasksFn: func(
			context.Context,
			taskpkg.Query,
			taskpkg.ActorContext,
		) ([]taskpkg.Summary, error) {
			return []taskpkg.Summary{
				{
					ID: "task-1", Priority: taskpkg.PriorityHigh, LastActivityAt: now,
					ActiveRun: &taskpkg.RunSummary{ResolvedNetworkParticipation: &liveSpec},
				},
				{
					ID: "task-2", Priority: taskpkg.PriorityMedium, LastActivityAt: now.Add(-time.Minute),
					ActiveRun: &taskpkg.RunSummary{ResolvedNetworkParticipation: &liveSpec},
				},
			}, nil
		}}
		query := taskpkg.CatalogQuery{
			Scope:                taskpkg.CatalogScopeGlobal,
			Sort:                 taskpkg.CatalogSortRecent,
			Limit:                1,
			ParticipationChannel: "builders",
		}
		page, err := stub.ListTaskCatalog(context.Background(), query, taskpkg.ActorContext{})
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

func TestNewSessionInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should return stable API fixture values", func(t *testing.T) {
		t.Parallel()

		got := NewSessionInfo("sess-1")
		wantTime := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
		if got.ID != "sess-1" || got.Name != "demo" || got.AgentName != "coder" {
			t.Fatalf("session identity = %#v, want stable demo coder session", got)
		}
		if got.WorkspaceID != "ws-workspace" || got.Workspace != "/workspace" {
			t.Fatalf("workspace fields = %#v, want stable workspace fixture", got)
		}
		if got.State != session.StateActive {
			t.Fatalf("state = %q, want %q", got.State, session.StateActive)
		}
		if !got.CreatedAt.Equal(wantTime) || !got.UpdatedAt.Equal(wantTime) {
			t.Fatalf("timestamps = %s/%s, want %s", got.CreatedAt, got.UpdatedAt, wantTime)
		}
	})
}

func TestStubResourceServicePut(t *testing.T) {
	t.Parallel()

	t.Run("Should clone spec JSON and populate deterministic metadata", func(t *testing.T) {
		t.Parallel()

		specJSON := []byte(`{"name":"demo"}`)
		draft := resources.RawDraft{
			Kind:     resources.ResourceKind("agent"),
			ID:       "agent.demo",
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: specJSON,
		}

		got, err := StubResourceService{}.Put(context.Background(), draft)
		if err != nil {
			t.Fatalf("StubResourceService.Put() error = %v", err)
		}
		specJSON[0] = '['

		if got.Kind != draft.Kind || got.ID != draft.ID || got.Scope != draft.Scope {
			t.Fatalf("record identity = %#v, want draft identity", got)
		}
		if got.Version != 1 {
			t.Fatalf("version = %d, want 1", got.Version)
		}
		if got.Owner.Kind != "daemon" || got.Owner.ID != "daemon-control" {
			t.Fatalf("owner = %#v, want daemon owner", got.Owner)
		}
		if got.Source.Kind != "daemon" || got.Source.ID != "system" {
			t.Fatalf("source = %#v, want daemon system source", got.Source)
		}
		if string(got.SpecJSON) != `{"name":"demo"}` {
			t.Fatalf("spec JSON = %s, want cloned original JSON", string(got.SpecJSON))
		}
		if got.CreatedAt != got.UpdatedAt || got.CreatedAt.IsZero() {
			t.Fatalf("timestamps = %s/%s, want deterministic non-zero timestamps", got.CreatedAt, got.UpdatedAt)
		}
	})
}

func TestStubWorkspaceServiceDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should report unconfigured register and resolve-or-register methods", func(t *testing.T) {
		t.Parallel()

		service := StubWorkspaceService{}
		if _, err := service.Register(
			context.Background(),
			workspacepkg.RegisterOptions{},
		); !errors.Is(err, ErrStubWorkspaceServiceNotImplemented) {
			t.Fatalf("Register() error = %v, want ErrStubWorkspaceServiceNotImplemented", err)
		}
		if _, err := service.ResolveOrRegister(context.Background(), "/workspace"); !errors.Is(
			err,
			ErrStubWorkspaceServiceNotImplemented,
		) {
			t.Fatalf("ResolveOrRegister() error = %v, want ErrStubWorkspaceServiceNotImplemented", err)
		}
	})
}
