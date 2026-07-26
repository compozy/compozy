package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	aghworkspace "github.com/compozy/agh/internal/workspace"
	"github.com/goccy/go-yaml"
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
	mustWriteMemory(t, store, memcontract.ScopeGlobal, "", "global.md", memcontract.TypeUser, "global memory")
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeWorkspace,
		workspace,
		"workspace.md",
		memcontract.TypeProject,
		"workspace memory",
	)
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeGlobal,
		"",
		"_system-hidden.md",
		memcontract.TypeReference,
		"system memory",
	)

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	t.Run("Should default list returns global scope", func(t *testing.T) {
		resp := performRequest(t, engine, http.MethodGet, "/api/memory", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 1 || payload.Memories[0].Filename != "global.md" {
			t.Fatalf("memories = %#v, want only global memory", payload.Memories)
		}
	})

	t.Run("Should scope global filters to global", func(t *testing.T) {
		resp := performRequest(t, engine, http.MethodGet, "/api/memory?scope=global", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 1 || payload.Memories[0].Filename != "global.md" {
			t.Fatalf("memories = %#v, want only global memory", payload.Memories)
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

	t.Run("Should include_system returns system-managed entries", func(t *testing.T) {
		resp := performRequest(t, engine, http.MethodGet, "/api/memory?scope=global&include_system=true", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload memoryListResponse
		decodeJSONResponse(t, resp, &payload)
		if len(payload.Memories) != 2 {
			t.Fatalf(
				"memories len = %d, want 2 with include_system; memories=%#v",
				len(payload.Memories),
				payload.Memories,
			)
		}
	})
}

func TestMemoryHandlersReadAndNotFound(t *testing.T) {
	t.Parallel()

	store, _ := newTestMemoryStore(t)
	mustWriteMemory(t, store, memcontract.ScopeGlobal, "", "readme.md", memcontract.TypeUser, "hello world")
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeGlobal,
		"",
		"_system-hidden.md",
		memcontract.TypeReference,
		"system body",
	)

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	resp := performRequest(t, engine, http.MethodGet, "/api/memory/readme.md?scope=global", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var payload memoryEntryResponse
	decodeJSONResponse(t, resp, &payload)
	if !strings.Contains(payload.Memory.Content, "hello world") {
		t.Fatalf("content = %q, want stored body", payload.Memory.Content)
	}

	missing := performRequest(t, engine, http.MethodGet, "/api/memory/missing.md?scope=global", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", missing.Code, http.StatusNotFound, missing.Body.String())
	}

	systemHidden := performRequest(t, engine, http.MethodGet, "/api/memory/_system-hidden.md?scope=global", nil)
	if systemHidden.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d for hidden system memory; body=%s",
			systemHidden.Code,
			http.StatusNotFound,
			systemHidden.Body.String(),
		)
	}

	systemVisible := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/memory/_system-hidden.md?scope=global&include_system=true",
		nil,
	)
	if systemVisible.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d for include_system read; body=%s",
			systemVisible.Code,
			http.StatusOK,
			systemVisible.Body.String(),
		)
	}
	var systemPayload memoryEntryResponse
	decodeJSONResponse(t, systemVisible, &systemPayload)
	if !strings.Contains(systemPayload.Memory.Content, "system body") {
		t.Fatalf("system content = %q, want stored body", systemPayload.Memory.Content)
	}
}

func TestMemoryHandlersWriteValidationAndScopeResolution(t *testing.T) {
	t.Parallel()

	store, workspace := newTestMemoryStore(t)
	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	valid := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory",
		[]byte(`{"scope":"global","type":"user","name":"Valid","description":"desc","content":"hello"}`),
	)
	if valid.Code != http.StatusOK {
		t.Fatalf("valid status = %d, want %d; body=%s", valid.Code, http.StatusOK, valid.Body.String())
	}
	var validPayload memoryMutationDecisionResponse
	decodeJSONResponse(t, valid, &validPayload)
	if !validPayload.Applied || validPayload.Decision.TargetFilename == "" {
		t.Fatalf("valid payload = %#v, want applied decision with target filename", validPayload)
	}
	if _, err := store.Read(memcontract.ScopeGlobal, validPayload.Decision.TargetFilename); err != nil {
		t.Fatalf("store.Read(valid) error = %v", err)
	}

	invalid := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory",
		[]byte(`{"scope":"global","type":"user","name":"Invalid"}`),
	)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want %d; body=%s", invalid.Code, http.StatusBadRequest, invalid.Body.String())
	}

	missing := performRequest(t, engine, http.MethodPost, "/api/memory", []byte(`{"scope":"global"}`))
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing status = %d, want %d; body=%s", missing.Code, http.StatusBadRequest, missing.Body.String())
	}

	userDefault := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory",
		[]byte(`{"type":"user","name":"User Default","description":"desc","content":"global body"}`),
	)
	if userDefault.Code != http.StatusOK {
		t.Fatalf(
			"userDefault status = %d, want %d; body=%s",
			userDefault.Code,
			http.StatusOK,
			userDefault.Body.String(),
		)
	}
	var userDefaultPayload memoryMutationDecisionResponse
	decodeJSONResponse(t, userDefault, &userDefaultPayload)
	if _, err := store.Read(memcontract.ScopeGlobal, userDefaultPayload.Decision.TargetFilename); err != nil {
		t.Fatalf("store.Read(global inferred) error = %v", err)
	}

	projectDefault := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory",
		[]byte(`{"workspace_id":"`+escapeJSON(
			workspace,
		)+`","type":"project","name":"Project Default","description":"desc","content":"workspace body"}`),
	)
	if projectDefault.Code != http.StatusOK {
		t.Fatalf(
			"projectDefault status = %d, want %d; body=%s",
			projectDefault.Code,
			http.StatusOK,
			projectDefault.Body.String(),
		)
	}
	var projectDefaultPayload memoryMutationDecisionResponse
	decodeJSONResponse(t, projectDefault, &projectDefaultPayload)
	if _, err := store.ForWorkspace(workspace).Read(
		memcontract.ScopeWorkspace,
		projectDefaultPayload.Decision.TargetFilename,
	); err != nil {
		t.Fatalf("store.Read(workspace inferred) error = %v", err)
	}
}

func TestMemoryHandlersDeleteAndNotFound(t *testing.T) {
	t.Parallel()

	store, _ := newTestMemoryStore(t)
	mustWriteMemory(t, store, memcontract.ScopeGlobal, "", "delete-me.md", memcontract.TypeUser, "bye")

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})
	engine := newTestRouter(t, handlers)

	resp := performRequest(t, engine, http.MethodDelete, "/api/memory/delete-me.md?scope=global", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if _, err := store.Read(memcontract.ScopeGlobal, "delete-me.md"); err == nil {
		t.Fatal("expected file to be deleted")
	}

	missing := performRequest(t, engine, http.MethodDelete, "/api/memory/missing.md?scope=global", nil)
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
		memcontract.ScopeGlobal,
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
		[]byte(`{"query_text":"auth migration sessions","workspace_id":"`+escapeJSON(workspace)+`"}`),
	)
	if search.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d; body=%s", search.Code, http.StatusOK, search.Body.String())
	}

	var searchPayload memorySearchResponse
	decodeJSONResponse(t, search, &searchPayload)
	if len(searchPayload.Results) == 0 || searchPayload.Results[0].Memory.Scope != memcontract.ScopeWorkspace {
		t.Fatalf("search results = %#v, want workspace hit first", searchPayload.Results)
	}

	globalOnly := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory/search",
		[]byte(`{"scope":"global","workspace_id":"`+escapeJSON(workspace)+`","query_text":"sessions"}`),
	)
	if globalOnly.Code != http.StatusOK {
		t.Fatalf(
			"global-only search status = %d, want %d; body=%s",
			globalOnly.Code,
			http.StatusOK,
			globalOnly.Body.String(),
		)
	}
	var globalOnlyPayload memorySearchResponse
	decodeJSONResponse(t, globalOnly, &globalOnlyPayload)
	if len(globalOnlyPayload.Results) != 0 {
		t.Fatalf("global-only search results = %#v, want none for workspace-only content", globalOnlyPayload.Results)
	}

	reindex := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/memory/reindex",
		[]byte(`{"workspace_id":"`+escapeJSON(workspace)+`"}`),
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

	trigger.enabled = true
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

func TestMemoryHandlersDreamTriggerDisabledAndBadJSON(t *testing.T) {
	t.Parallel()

	store, _ := newTestMemoryStore(t)
	engine := newTestRouter(t, newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, nil))

	badRequest := performRequest(t, engine, http.MethodPost, "/api/memory/dreams/trigger", []byte(`{`))
	if badRequest.Code != http.StatusBadRequest {
		t.Fatalf(
			"badRequest status = %d, want %d; body=%s",
			badRequest.Code,
			http.StatusBadRequest,
			badRequest.Body.String(),
		)
	}

	disabled := performRequest(t, engine, http.MethodPost, "/api/memory/dreams/trigger", nil)
	if disabled.Code != http.StatusOK {
		t.Fatalf("disabled status = %d, want %d; body=%s", disabled.Code, http.StatusOK, disabled.Body.String())
	}

	var payload memoryDreamTriggerResponse
	decodeJSONResponse(t, disabled, &payload)
	if payload.Triggered || !strings.Contains(payload.Reason, "disabled") {
		t.Fatalf("payload = %#v, want disabled response", payload)
	}
}

func TestHealthIncludesMemoryStats(t *testing.T) {
	t.Parallel()

	store, workspace := newTestMemoryStore(t)
	mustWriteMemory(t, store, memcontract.ScopeGlobal, "", "health-global.md", memcontract.TypeUser, "global")
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeWorkspace,
		workspace,
		"health-workspace.md",
		memcontract.TypeProject,
		"workspace",
	)

	last := time.Date(2026, 4, 4, 3, 30, 0, 0, time.UTC)
	trigger := &stubDreamTrigger{enabled: true, last: last}
	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			info := newSessionInfo("sess-1")
			info.Workspace = workspace
			return []*session.Info{info}, nil
		},
	}
	observer := stubObserver{
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{Status: "ok", ActiveSessions: 1}, nil
		},
	}

	handlers := newTestMemoryHandlers(t, manager, observer, store, trigger)
	engine := newTestRouter(t, handlers)

	resp := performRequest(t, engine, http.MethodGet, "/api/status", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var payload struct {
		Memory memoryHealthPayload `json:"memory"`
	}
	decodeJSONResponse(t, resp, &payload)
	if payload.Memory.GlobalFiles != 1 || payload.Memory.WorkspaceFiles != 1 || !payload.Memory.DreamEnabled {
		t.Fatalf("memory health = %#v", payload.Memory)
	}
	if payload.Memory.LastConsolidation == nil || !payload.Memory.LastConsolidation.Equal(last) {
		t.Fatalf("last consolidation = %#v, want %s", payload.Memory.LastConsolidation, last)
	}
	if !payload.Memory.Enabled || payload.Memory.IndexedFiles != 2 || payload.Memory.OrphanedFiles != 0 {
		t.Fatalf("memory health catalog stats = %#v, want enabled+indexed stats", payload.Memory)
	}
	if payload.Memory.LastReindex == nil {
		t.Fatalf("last reindex = %#v, want non-nil", payload.Memory.LastReindex)
	}
}

func TestMemoryHelpersResolveLocationAndScope(t *testing.T) {
	t.Parallel()

	store, workspace := newTestMemoryStore(t)
	mustWriteMemory(t, store, memcontract.ScopeGlobal, "", "shared.md", memcontract.TypeUser, "global")
	mustWriteMemory(t, store, memcontract.ScopeWorkspace, workspace, "shared.md", memcontract.TypeProject, "workspace")
	mustWriteMemory(
		t,
		store,
		memcontract.ScopeWorkspace,
		workspace,
		"workspace-only.md",
		memcontract.TypeProject,
		"workspace only",
	)

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, store, &stubDreamTrigger{})

	location, err := handlers.resolveMemoryLocation("workspace-only.md", "", workspace)
	if err != nil {
		t.Fatalf("resolveMemoryLocation(workspace-only) error = %v", err)
	}
	if location.Scope != memcontract.ScopeWorkspace || location.Workspace != workspace {
		t.Fatalf("location = %#v, want workspace match", location)
	}

	_, err = handlers.resolveMemoryLocation("shared.md", "", workspace)
	if !errors.Is(err, memory.ErrValidation) {
		t.Fatalf("resolveMemoryLocation(shared) error = %v, want validation error", err)
	}

	_, err = handlers.resolveMemoryLocation("shared.md", "workspace", "")
	if !errors.Is(err, memory.ErrValidation) {
		t.Fatalf("resolveMemoryLocation(workspace without workspace) error = %v, want validation error", err)
	}

	_, err = handlers.resolveMemoryLocation("missing.md", "", workspace)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolveMemoryLocation(missing) error = %v, want os.ErrNotExist", err)
	}
}

func TestMemoryHelpersWriteScopeStatusAndWorkspaces(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(t.TempDir(), "..", "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", workspace, err)
	}
	content := memoryDocument(t, "Project Default", "desc", memcontract.TypeProject, "workspace body")

	scope, resolvedWorkspace, err := resolveMemoryWriteScope(memoryWriteRequest{
		Scope:     "workspace",
		Workspace: workspace,
		Content:   content,
	})
	if err != nil {
		t.Fatalf("resolveMemoryWriteScope() error = %v", err)
	}
	if scope != memcontract.ScopeWorkspace {
		t.Fatalf("scope = %q, want workspace", scope)
	}
	if resolvedWorkspace == "" || !filepath.IsAbs(resolvedWorkspace) {
		t.Fatalf("resolvedWorkspace = %q, want absolute path", resolvedWorkspace)
	}

	if _, _, err := resolveMemoryWriteScope(memoryWriteRequest{}); !errors.Is(err, memory.ErrValidation) {
		t.Fatalf("resolveMemoryWriteScope(empty) error = %v, want validation", err)
	}
	if _, _, err := resolveMemoryWriteScope(
		memoryWriteRequest{Content: "not frontmatter"},
	); !errors.Is(
		err,
		memory.ErrValidation,
	) {
		t.Fatalf("resolveMemoryWriteScope(invalid content) error = %v, want validation", err)
	}
	if _, err := parseOptionalMemoryScope("bogus"); !errors.Is(err, memory.ErrValidation) {
		t.Fatalf("parseOptionalMemoryScope(bogus) error = %v, want validation", err)
	}
	if _, err := resolveMemoryWorkspace(""); !errors.Is(err, memory.ErrValidation) {
		t.Fatalf("resolveMemoryWorkspace(\"\") error = %v, want validation", err)
	}

	statuses := map[string]int{
		"nil":        statusForMemoryError(nil),
		"not_found":  statusForMemoryError(fmt.Errorf("%w: missing", os.ErrNotExist)),
		"validation": statusForMemoryError(newMemoryValidationError(errors.New("bad request"))),
		"internal":   statusForMemoryError(errors.New("boom")),
	}
	if statuses["nil"] != http.StatusOK || statuses["not_found"] != http.StatusNotFound ||
		statuses["validation"] != http.StatusBadRequest ||
		statuses["internal"] != http.StatusInternalServerError {
		t.Fatalf("statuses = %#v", statuses)
	}

	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			first := newSessionInfo("sess-1")
			first.Workspace = workspace
			second := newSessionInfo("sess-2")
			second.Workspace = filepath.Clean(workspace)
			empty := newSessionInfo("sess-3")
			empty.Workspace = ""
			return []*session.Info{first, second, empty}, nil
		},
	}
	handlers := newTestMemoryHandlers(t, manager, stubObserver{}, nil, &stubDreamTrigger{})
	workspaces, err := handlers.memoryHealthWorkspaces(context.Background(), "")
	if err != nil {
		t.Fatalf("memoryHealthWorkspaces() error = %v", err)
	}
	if len(workspaces) != 1 || !filepath.IsAbs(workspaces[0]) {
		t.Fatalf("workspaces = %#v, want one absolute path", workspaces)
	}

	explicitWorkspace := t.TempDir()
	explicit, err := handlers.memoryHealthWorkspaces(context.Background(), explicitWorkspace)
	if err != nil {
		t.Fatalf("memoryHealthWorkspaces(explicit) error = %v", err)
	}
	if len(explicit) != 1 || !filepath.IsAbs(explicit[0]) {
		t.Fatalf("explicit workspaces = %#v, want one absolute path", explicit)
	}
}

func TestMemoryHandlersReturnInternalErrorWithoutConfiguredStore(t *testing.T) {
	t.Parallel()

	handlers := newTestMemoryHandlers(t, stubSessionManager{}, stubObserver{}, nil, &stubDreamTrigger{enabled: true})
	engine := newTestRouter(t, handlers)
	document := escapeJSON(memoryDocument(t, "Valid", "desc", memcontract.TypeUser, "hello"))

	requests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/api/memory"},
		{method: http.MethodGet, path: "/api/memory/valid.md?scope=global"},
		{
			method: http.MethodPost,
			path:   "/api/memory",
			body: []byte(
				`{"scope":"global","type":"user","name":"Valid","content":"` + document + `"}`,
			),
		},
		{method: http.MethodDelete, path: "/api/memory/valid.md?scope=global"},
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
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123

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
		agentLoader:  aghconfig.LoadAgentDef,
		httpPort:     cfg.HTTP.Port,
	})
}

func newTestMemoryStore(t *testing.T) (*memory.Store, string) {
	t.Helper()

	baseDir := t.TempDir()
	globalDir := filepath.Join(baseDir, "global-memory")
	store := memory.NewStore(globalDir, memory.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")))
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
	if _, err := aghworkspace.EnsureIdentity(context.Background(), workspace); err != nil {
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
	if err := target.Write(scope, filename, []byte(memoryDocument(t, filename, "desc", typ, body))); err != nil {
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

func escapeJSON(value string) string {
	payload, _ := json.Marshal(value)
	return strings.Trim(string(payload), "\"")
}
