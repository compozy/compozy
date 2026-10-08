package udsapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v3"

	memcontract "github.com/compozy/compozy/internal/memory/contract"

	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

type stubDreamTrigger struct {
	triggered bool
	reason    string
	err       error
	last      time.Time
	lastErr   error
	enabled   bool
	calls     int
}

func (s *stubDreamTrigger) Trigger(context.Context, string) (bool, string, error) {
	s.calls++
	return s.triggered, s.reason, s.err
}

func (s *stubDreamTrigger) LastConsolidatedAt() (time.Time, error) {
	return s.last, s.lastErr
}

func (s *stubDreamTrigger) Enabled() bool {
	return s.enabled
}

func TestMemoryHandlersListAndFilters(t *testing.T) {
	t.Parallel()

	store, workspace := newTestMemoryStore(t)
	mustWriteMemory(t, store, memcontract.ScopeProfile, "", "profile.md", memcontract.TypeUser, "profile memory")
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeWorkspace,
		workspace,
		"workspace.md",
		memcontract.TypeProject,
		"workspace memory",
	)

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	t.Run("Should default list returns profile scope", func(t *testing.T) {
		resp := performRequest(t, engine, http.MethodGet, "/api/memory", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 1 || payload.Memories[0].Filename != "profile.md" {
			t.Fatalf("memories = %#v, want only profile memory", payload.Memories)
		}
	})

	t.Run("Should scope profile filters to the active profile", func(t *testing.T) {
		resp := performRequest(t, engine, http.MethodGet, "/api/memory?scope=profile", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 1 || payload.Memories[0].Filename != "profile.md" {
			t.Fatalf("memories = %#v, want only profile memory", payload.Memories)
		}
	})

	t.Run("Should scope workspace filters to workspace", func(t *testing.T) {
		resp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/memory?scope=workspace&workspace_id="+url.QueryEscape(workspace),
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 1 || payload.Memories[0].Filename != "workspace.md" {
			t.Fatalf("memories = %#v, want only workspace memory", payload.Memories)
		}
	})

	t.Run("Should workspace query without scope includes both scopes", func(t *testing.T) {
		resp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/memory?workspace_id="+url.QueryEscape(workspace),
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 2 {
			t.Fatalf("memories len = %d, want 2; memories=%#v", len(payload.Memories), payload.Memories)
		}
	})
}

func TestMemoryHandlersReadAndNotFound(t *testing.T) {
	t.Parallel()

	store, _ := newTestMemoryStore(t)
	mustWriteMemory(t, store, memcontract.ScopeProfile, "", "readme.md", memcontract.TypeUser, "hello world")

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	resp := performRequest(t, engine, http.MethodGet, "/api/memory/readme.md?scope=profile", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var payload memoryEntryResponse
	decodeJSONResponse(t, resp, &payload)
	if !strings.Contains(payload.Memory.Content, "hello world") {
		t.Fatalf("content = %q, want stored body", payload.Memory.Content)
	}

	missing := performRequest(t, engine, http.MethodGet, "/api/memory/missing.md?scope=profile", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", missing.Code, http.StatusNotFound, missing.Body.String())
	}
}

func TestMemoryHandlersSearchAndReindex(t *testing.T) {
	t.Parallel()

	store, workspace := newTestMemoryStore(t)
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeProfile,
		"",
		"prefs.md",
		memcontract.TypeUser,
		"User prefers concise answers",
	)
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeWorkspace,
		workspace,
		"auth.md",
		memcontract.TypeProject,
		"Auth migration uses sessions",
	)

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	search := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory/search",
		[]byte(`{"query_text":"auth migration sessions","workspace_id":"`+escapeJSON(t, workspace)+`"}`),
	)
	if search.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d; body=%s", search.Code, http.StatusOK, search.Body.String())
	}

	var searchPayload memorySearchResponse
	decodeJSONResponse(t, search, &searchPayload)
	if len(searchPayload.Results) == 0 || searchPayload.Results[0].Memory.Scope != memcontract.ScopeWorkspace {
		t.Fatalf("search results = %#v, want workspace hit first", searchPayload.Results)
	}

	reindex := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory/reindex",
		[]byte(`{"workspace_id":"`+escapeJSON(t, workspace)+`"}`),
	)
	if reindex.Code != http.StatusOK {
		t.Fatalf("reindex status = %d, want %d; body=%s", reindex.Code, http.StatusOK, reindex.Body.String())
	}

	var payload memoryReindexResponse
	decodeJSONResponse(t, reindex, &payload)
	if payload.IndexedFiles != 2 {
		t.Fatalf("reindex payload = %#v, want indexed_files=2", payload)
	}
}

func TestMemoryHandlersDreamTrigger(t *testing.T) {
	t.Parallel()

	store, _ := newTestMemoryStore(t)
	trigger := &stubDreamTrigger{enabled: true, triggered: true}
	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, trigger)
	engine := newTestRouter(t, handlers)

	triggered := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory/dreams/trigger",
		[]byte(`{"workspace_id":"ws-project"}`),
	)
	if triggered.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", triggered.Code, http.StatusOK, triggered.Body.String())
	}

	var triggeredPayload memoryDreamTriggerResponse
	decodeJSONResponse(t, triggered, &triggeredPayload)
	if !triggeredPayload.Triggered {
		t.Fatalf("payload = %#v, want triggered", triggeredPayload)
	}

	trigger.triggered = false
	trigger.reason = "gates not satisfied"

	notTriggered := performRequest(t, engine, http.MethodPost, "/api/memory/dreams/trigger", []byte(`{}`))
	if notTriggered.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", notTriggered.Code, http.StatusOK, notTriggered.Body.String())
	}

	var notTriggeredPayload memoryDreamTriggerResponse
	decodeJSONResponse(t, notTriggered, &notTriggeredPayload)
	if notTriggeredPayload.Triggered || notTriggeredPayload.Reason != "gates not satisfied" {
		t.Fatalf("payload = %#v, want gates-failed response", notTriggeredPayload)
	}
}

func TestMemoryHandlersReturnInternalErrorWithoutConfiguredStore(t *testing.T) {
	t.Parallel()

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, nil, &stubDreamTrigger{enabled: true})
	engine := newTestRouter(t, handlers)
	document := escapeJSON(t, memoryDocument(t, "Valid", "desc", memcontract.TypeUser, "hello"))

	requests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/api/memory"},
		{method: http.MethodGet, path: "/api/memory/valid.md?scope=profile"},
		{
			method: http.MethodPost,
			path:   "/api/memory",
			body: []byte(
				`{"scope":"profile","type":"user","name":"Valid","content":"` + document + `"}`,
			),
		},
		{method: http.MethodDelete, path: "/api/memory/valid.md?scope=profile"},
	}

	for _, request := range requests {
		resp := performRequest(t, engine, request.method, request.path, request.body)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"%s %s status = %d, want %d; body=%s",
				request.method,
				request.path,
				resp.Code,
				http.StatusInternalServerError,
				resp.Body.String(),
			)
		}
	}

	if err := newMemoryValidationError(nil); err != nil {
		t.Fatalf("newMemoryValidationError(nil) = %v, want nil", err)
	}
}

func newTestMemoryHandlers(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	store *memory.Store,
	trigger core.DreamTrigger,
) *Handlers {
	t.Helper()

	homePaths := newTestHomePaths(t)
	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.Memory.Enabled = true

	return newHandlers(&handlerConfig{
		sessions:     manager,
		observer:     observer,
		memoryStore:  store,
		dreamTrigger: trigger,
		homePaths:    homePaths,
		config:       cfg,
		logger:       discardLogger(),
		startedAt:    time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		now:          func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		pollInterval: 5 * time.Millisecond,
		agentLoader:  compozyconfig.LoadAgentDef,
	})
}

func newTestMemoryStore(t *testing.T) (*memory.Store, string) {
	t.Helper()

	baseDir := t.TempDir()
	globalDir := filepath.Join(baseDir, "global-memory")
	store := memory.NewStore(globalDir, memory.WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")))
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	if err := store.OpenCatalog(t.Context()); err != nil {
		t.Fatalf("OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(context.Background()); err != nil {
			t.Errorf("CloseCatalog() error = %v", err)
		}
	})
	workspace := t.TempDir()
	if _, err := compozyworkspace.EnsureIdentity(t.Context(), workspace); err != nil {
		t.Fatalf("EnsureIdentity(%q) error = %v", workspace, err)
	}
	return store, workspace
}

func mustWriteMemory(
	t *testing.T,
	store *memory.Store,
	scope memcontract.Scope,
	workspace string,
	filename string,
	typ memcontract.Type,
	body string,
) {
	t.Helper()

	target := store
	if scope == memcontract.ScopeWorkspace {
		target = store.ForWorkspace(workspace)
	}
	if err := target.Write(
		t.Context(), scope, filename, []byte(memoryDocument(t, filename, "desc", typ, body)),
	); err != nil {
		t.Fatalf("Write(%s) error = %v", filename, err)
	}
}

func memoryDocument(t *testing.T, name string, description string, typ memcontract.Type, body string) string {
	t.Helper()

	header := memcontract.Header{
		Name:        name,
		Description: description,
		Type:        typ,
	}
	metadata, err := yaml.Marshal(header)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	return "---\n" + string(metadata) + "---\n\n" + body
}

func escapeJSON(t *testing.T, value string) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return strings.Trim(string(payload), "\"")
}
