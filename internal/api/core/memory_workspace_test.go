package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/goccy/go-yaml"
)

func TestMemoryHandlersAndHelpers(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (handlerFixture, string, *stubDreamTrigger) {
		t.Helper()

		store := memory.NewStore(
			filepath.Join(t.TempDir(), "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(t.TempDir(), "agh.db")),
		)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := store.CloseRecallSignalRecorders(ctx); err != nil {
				t.Fatalf("CloseRecallSignalRecorders() error = %v", err)
			}
		})
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		openCoreTestMemoryCatalog(t, store)

		workspace := filepath.Join(t.TempDir(), "workspace with space")
		if err := os.MkdirAll(workspace, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspace) error = %v", err)
		}
		if err := store.Write(
			memcontract.ScopeGlobal,
			"global.md",
			[]byte(memoryDocument(t, "Global", memcontract.TypeUser, "hello")),
		); err != nil {
			t.Fatalf("Write(global) error = %v", err)
		}
		if err := store.ForWorkspace(workspace).
			Write(memcontract.ScopeWorkspace, "workspace.md", []byte(memoryDocument(t, "Workspace", memcontract.TypeProject, "world"))); err != nil {
			t.Fatalf("Write(workspace) error = %v", err)
		}

		trigger := &stubDreamTrigger{
			EnabledFn: true,
			Triggered: true,
			Reason:    "queued",
			Last:      time.Date(2026, 4, 4, 3, 30, 0, 0, time.UTC),
		}
		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				info := testutil.NewSessionInfo("sess-a")
				info.Workspace = workspace
				return []*session.Info{info}, nil
			},
		}
		observer := testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", ActiveSessions: 1}, nil
			},
		}

		return newHandlerFixture(
			t,
			manager,
			observer,
			testutil.StubWorkspaceService{},
			store,
			trigger,
		), workspace, trigger
	}

	t.Run("Should list memory for a workspace", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		query := url.Values{}
		query.Set("workspace_id", workspace)
		query.Set("sort", memory.HeaderListSortName)
		query.Set("limit", "1")
		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory?"+query.Encode(), nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list memory status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
		}

		var payload contract.MemoryListResponse
		testutil.DecodeJSONResponse(t, listResp, &payload)
		if len(payload.Memories) != 1 || payload.Memories[0].Filename != "global.md" {
			t.Fatalf("first memory page = %#v, want global.md", payload.Memories)
		}
		if payload.Page.Total != 2 || payload.Page.Limit != 1 || !payload.Page.HasMore ||
			payload.Page.NextCursor == "" {
			t.Fatalf("first memory page metadata = %#v", payload.Page)
		}

		query.Set("cursor", payload.Page.NextCursor)
		secondResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory?"+query.Encode(), nil)
		if secondResp.Code != http.StatusOK {
			t.Fatalf(
				"second memory page status = %d, want %d; body=%s",
				secondResp.Code,
				http.StatusOK,
				secondResp.Body.String(),
			)
		}
		var second contract.MemoryListResponse
		testutil.DecodeJSONResponse(t, secondResp, &second)
		if len(second.Memories) != 1 || second.Memories[0].Filename != "workspace.md" {
			t.Fatalf("second memory page = %#v, want workspace.md", second.Memories)
		}
		if second.Memories[0].WorkspaceID == "" {
			t.Fatalf("second memory page workspace_id = empty; payload=%#v", second.Memories[0])
		}
		if second.Page.Total != 2 || second.Page.Limit != 1 || second.Page.HasMore || second.Page.NextCursor != "" {
			t.Fatalf("second memory page metadata = %#v", second.Page)
		}
	})

	t.Run("Should reach an oldest matching memory beyond the prompt scan cap", func(t *testing.T) {
		t.Parallel()

		globalDir := filepath.Join(t.TempDir(), "memory")
		store := memory.NewStore(
			globalDir,
			memory.WithCatalogDatabasePath(filepath.Join(t.TempDir(), "agh.db")),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		openCoreTestMemoryCatalog(t, store)
		seedCoreMemoryCatalogDocuments(t, globalDir, 205)
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			store,
			&stubDreamTrigger{},
		)

		query := url.Values{}
		query.Set("scope", string(memcontract.ScopeGlobal))
		query.Set("type", string(memcontract.TypeReference))
		query.Set("limit", "1")
		response := performRequest(t, fixture.Engine, http.MethodGet, "/memory?"+query.Encode(), nil)
		if response.Code != http.StatusOK {
			t.Fatalf("list memory status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}

		var payload contract.MemoryListResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.Page.Total != 1 || len(payload.Memories) != 1 ||
			payload.Memories[0].Filename != "memory-000.md" {
			t.Fatalf("list memory payload = %#v, want oldest matching memory", payload)
		}
	})

	t.Run("Should isolate workspace agent memory identities", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		identity, err := workspacepkg.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		store := memory.NewStore(
			filepath.Join(baseDir, "global-memory"),
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		)
		openCoreTestMemoryCatalog(t, store)
		base := store.ForWorkspace(workspaceRoot)
		for _, agentName := range []string{"reviewer-a", "reviewer-b"} {
			agentStore := base.ForAgent(identity.WorkspaceID, agentName, memcontract.AgentTierWorkspace)
			if err := agentStore.EnsureDirs(); err != nil {
				t.Fatalf("Store.EnsureDirs(%s) error = %v", agentName, err)
			}
			if err := agentStore.Write(
				memcontract.ScopeAgent,
				agentName+".md",
				[]byte(memoryDocument(t, agentName, memcontract.TypeFeedback, "agent memory")),
			); err != nil {
				t.Fatalf("Store.Write(%s) error = %v", agentName, err)
			}
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: identity.WorkspaceID, RootDir: workspaceRoot},
					WorkspaceID: identity.WorkspaceID,
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			workspaces,
			store,
			&stubDreamTrigger{},
		)

		query := url.Values{}
		query.Set("workspace_id", identity.WorkspaceID)
		query.Set("agent_name", "reviewer-a")
		query.Set("agent_tier", string(memcontract.AgentTierWorkspace))
		response := performRequest(t, fixture.Engine, http.MethodGet, "/memory?"+query.Encode(), nil)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"list agent memory status = %d, want %d; body=%s",
				response.Code,
				http.StatusOK,
				response.Body.String(),
			)
		}

		var payload contract.MemoryListResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.Page.Total != 1 || len(payload.Memories) != 1 {
			t.Fatalf("list agent memory payload = %#v, want one reviewer-a memory", payload)
		}
		got := payload.Memories[0]
		if got.Filename != "reviewer-a.md" || got.WorkspaceID != identity.WorkspaceID ||
			got.AgentName != "reviewer-a" || got.AgentTier != memcontract.AgentTierWorkspace {
			t.Fatalf("list agent memory item = %#v, want reviewer-a workspace identity", got)
		}
	})

	t.Run("Should search explicit single-token workspace memory", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		wantModTime := time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)
		workspaceMemoryPath := filepath.Join(workspace, ".agh", "memory", "workspace.md")
		if err := os.Chtimes(workspaceMemoryPath, wantModTime, wantModTime); err != nil {
			t.Fatalf("os.Chtimes(workspace memory) error = %v", err)
		}
		reindexBody, err := json.Marshal(contract.MemoryReindexV2Request{
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: workspace,
		})
		if err != nil {
			t.Fatalf("json.Marshal(reindex) error = %v", err)
		}
		reindexResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory/reindex", reindexBody)
		if reindexResp.Code != http.StatusOK {
			t.Fatalf(
				"reindex memory status = %d, want %d; body=%s",
				reindexResp.Code,
				http.StatusOK,
				reindexResp.Body.String(),
			)
		}

		body, err := json.Marshal(contract.MemorySearchRequest{
			QueryText:   "workspace",
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: workspace,
			TopK:        3,
		})
		if err != nil {
			t.Fatalf("json.Marshal(search) error = %v", err)
		}
		searchResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory/search", body)
		if searchResp.Code != http.StatusOK {
			t.Fatalf(
				"search memory status = %d, want %d; body=%s",
				searchResp.Code,
				http.StatusOK,
				searchResp.Body.String(),
			)
		}

		var payload contract.MemorySearchResponse
		testutil.DecodeJSONResponse(t, searchResp, &payload)
		if len(payload.Results) == 0 || payload.Results[0].Memory.Filename != "workspace.md" {
			t.Fatalf("search results = %#v, want workspace.md", payload.Results)
		}
		if got := payload.Results[0].Memory.ModTime; !got.Equal(wantModTime) {
			t.Fatalf("search result mod time = %v, want %v", got, wantModTime)
		}
	})

	t.Run("Should read global memory", func(t *testing.T) {
		t.Parallel()

		fixture, _, _ := setup(t)
		readResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/global.md?scope=global", nil)
		if readResp.Code != http.StatusOK {
			t.Fatalf("read memory status = %d, want %d", readResp.Code, http.StatusOK)
		}

		var payload contract.MemoryEntryResponse
		testutil.DecodeJSONResponse(t, readResp, &payload)
		if payload.Memory.Content == "" {
			t.Fatalf("read payload = %#v, want non-empty content", payload)
		}
	})

	t.Run("Should write workspace memory", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		const rawContent = "durable response body sentinel"
		writeBody, err := json.Marshal(contract.MemoryCreateRequest{
			WorkspaceID: workspace,
			Type:        memcontract.TypeProject,
			Name:        "Project",
			Content:     rawContent,
		})
		if err != nil {
			t.Fatalf("json.Marshal(write request) error = %v", err)
		}
		writeResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", writeBody)
		if writeResp.Code != http.StatusOK {
			t.Fatalf(
				"write memory status = %d, want %d; body=%s",
				writeResp.Code,
				http.StatusOK,
				writeResp.Body.String(),
			)
		}

		var payload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, writeResp, &payload)
		if !payload.Applied || payload.Decision.TargetFilename == "" {
			t.Fatalf("write payload = %#v, want applied decision", payload)
		}
		if payload.Decision.PostContentHash == "" {
			t.Fatalf("write payload = %#v, want content hash without raw content", payload)
		}
		for _, leaked := range []string{rawContent, `"post_content":`, `"prior_content":`, `"raw_response":`} {
			if strings.Contains(writeResp.Body.String(), leaked) {
				t.Fatalf("write response leaked %q in body %s", leaked, writeResp.Body.String())
			}
		}
	})

	t.Run("Should delete workspace memory", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		writeBody, err := json.Marshal(contract.MemoryCreateRequest{
			WorkspaceID: workspace,
			Type:        memcontract.TypeProject,
			Name:        "Delete Me",
			Content:     "updated",
		})
		if err != nil {
			t.Fatalf("json.Marshal(write request) error = %v", err)
		}
		writeResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", writeBody)
		if writeResp.Code != http.StatusOK {
			t.Fatalf(
				"write memory status = %d, want %d; body=%s",
				writeResp.Code,
				http.StatusOK,
				writeResp.Body.String(),
			)
		}
		var writePayload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, writeResp, &writePayload)

		query := url.Values{}
		query.Set("scope", "workspace")
		query.Set("workspace_id", workspace)
		deleteResp := performRequest(
			t,
			fixture.Engine,
			http.MethodDelete,
			"/memory/"+writePayload.Decision.TargetFilename+"?"+query.Encode(),
			nil,
		)
		if deleteResp.Code != http.StatusOK {
			t.Fatalf("delete memory status = %d, want %d", deleteResp.Code, http.StatusOK)
		}

		var payload contract.MemoryDeleteResponse
		testutil.DecodeJSONResponse(t, deleteResp, &payload)
		if !payload.Applied {
			t.Fatalf("delete payload = %#v, want applied=true", payload)
		}
	})

	t.Run("Should trigger dream consolidation", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, trigger := setup(t)
		identity, err := workspacepkg.EnsureIdentity(context.Background(), workspace)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		body, err := json.Marshal(contract.MemoryDreamTriggerRequest{WorkspaceID: identity.WorkspaceID})
		if err != nil {
			t.Fatalf("json.Marshal(consolidate request) error = %v", err)
		}
		consolidateResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory/dreams/trigger", body)
		if consolidateResp.Code != http.StatusOK {
			t.Fatalf("consolidate status=%d want=%d", consolidateResp.Code, http.StatusOK)
		}
		if trigger.Calls != 1 || trigger.Workspace != identity.WorkspaceID {
			t.Fatalf("trigger calls=%d workspace=%q", trigger.Calls, trigger.Workspace)
		}

		var payload contract.MemoryDreamTriggerResponse
		testutil.DecodeJSONResponse(t, consolidateResp, &payload)
		if !payload.Triggered || payload.Reason != "queued" {
			t.Fatalf("consolidate payload = %#v", payload)
		}
	})

	t.Run("Should report observe health", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		query := url.Values{}
		query.Set("workspace_id", workspace)
		healthResp := performRequest(t, fixture.Engine, http.MethodGet, "/status?"+query.Encode(), nil)
		if healthResp.Code != http.StatusOK {
			t.Fatalf("health status = %d, want %d", healthResp.Code, http.StatusOK)
		}

		var payload struct {
			Health observe.Health               `json:"health"`
			Memory contract.MemoryHealthPayload `json:"memory"`
		}
		testutil.DecodeJSONResponse(t, healthResp, &payload)
		if payload.Health.Status != "ok" || payload.Health.ActiveSessions != 1 {
			t.Fatalf("health payload = %#v", payload.Health)
		}
		if payload.Memory.WorkspaceFiles != 1 || !payload.Memory.DreamEnabled {
			t.Fatalf("memory payload = %#v", payload.Memory)
		}
	})

	t.Run("Should report memory health directly", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		workspaceStore := fixture.Handlers.MemoryStore.ForWorkspace(workspace)
		for idx := range 205 {
			filename := fmt.Sprintf("health-%03d.md", idx)
			if err := workspaceStore.Write(
				memcontract.ScopeWorkspace,
				filename,
				[]byte(memoryDocument(t, fmt.Sprintf("Health %03d", idx), memcontract.TypeReference, "health count")),
			); err != nil {
				t.Fatalf("Write(%q) error = %v", filename, err)
			}
		}
		query := url.Values{}
		query.Set("workspace_id", workspace)
		healthResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/health?"+query.Encode(), nil)
		if healthResp.Code != http.StatusOK {
			t.Fatalf("memory health status = %d, want %d", healthResp.Code, http.StatusOK)
		}

		var payload contract.MemoryHealthPayload
		testutil.DecodeJSONResponse(t, healthResp, &payload)
		if payload.Status != "ok" ||
			!payload.Configured ||
			payload.WorkspaceCount != 1 ||
			payload.WorkspaceFiles != 206 {
			t.Fatalf("memory health payload = %#v", payload)
		}
	})

	t.Run("Should report degraded memory health when dream status fails", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, trigger := setup(t)
		trigger.LastErr = errors.New("dream status failed")
		query := url.Values{}
		query.Set("workspace_id", workspace)
		healthResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/health?"+query.Encode(), nil)
		if healthResp.Code != http.StatusOK {
			t.Fatalf(
				"memory health status = %d, want %d; body=%s",
				healthResp.Code,
				http.StatusOK,
				healthResp.Body.String(),
			)
		}

		var payload contract.MemoryHealthPayload
		testutil.DecodeJSONResponse(t, healthResp, &payload)
		if payload.Status != "degraded" || !strings.Contains(payload.Reason, "dream status failed") {
			t.Fatalf("memory health payload = %#v, want degraded dream status failure", payload)
		}
	})

	t.Run("Should report unavailable memory health when store is missing", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		healthResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/health", nil)
		if healthResp.Code != http.StatusOK {
			t.Fatalf("memory health status = %d, want %d", healthResp.Code, http.StatusOK)
		}

		var payload contract.MemoryHealthPayload
		testutil.DecodeJSONResponse(t, healthResp, &payload)
		if payload.Status != "unavailable" || payload.Configured || payload.Reason == "" {
			t.Fatalf("memory health payload = %#v, want unavailable and unconfigured", payload)
		}
	})

	t.Run("Should report disabled memory health before missing store", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Memory.Enabled = false
		healthResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/health", nil)
		if healthResp.Code != http.StatusOK {
			t.Fatalf("memory health status = %d, want %d", healthResp.Code, http.StatusOK)
		}

		var payload contract.MemoryHealthPayload
		testutil.DecodeJSONResponse(t, healthResp, &payload)
		if payload.Status != "disabled" || payload.Reason == "" || payload.Enabled {
			t.Fatalf("memory health payload = %#v, want disabled", payload)
		}
	})

	t.Run("Should report degraded memory health for orphaned catalog rows", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspace := filepath.Join(baseDir, "workspace")
		store := memory.NewStore(
			filepath.Join(baseDir, "global"),
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		).ForWorkspace(workspace)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		openCoreTestMemoryCatalog(t, store)
		if err := store.Write(
			memcontract.ScopeWorkspace,
			"orphan.md",
			[]byte(memoryDocument(t, "Orphan", memcontract.TypeProject, "orphan signal")),
		); err != nil {
			t.Fatalf("Write(workspace) error = %v", err)
		}
		if _, err := store.Search(context.Background(), "orphan signal", memcontract.SearchOptions{
			Workspace: workspace,
			Limit:     5,
		}); err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if err := os.Remove(filepath.Join(workspace, aghconfig.DirName, "memory", "orphan.md")); err != nil {
			t.Fatalf("os.Remove(orphan memory) error = %v", err)
		}

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			store,
			nil,
		)
		query := url.Values{}
		query.Set("workspace_id", workspace)
		healthResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/health?"+query.Encode(), nil)
		if healthResp.Code != http.StatusOK {
			t.Fatalf("memory health status = %d, want %d", healthResp.Code, http.StatusOK)
		}

		var payload contract.MemoryHealthPayload
		testutil.DecodeJSONResponse(t, healthResp, &payload)
		if payload.Status != "degraded" || payload.WorkspaceFiles != 0 || payload.IndexedFiles != 1 ||
			payload.OrphanedFiles != 1 || !strings.Contains(payload.Reason, "orphaned") {
			t.Fatalf("memory health payload = %#v, want degraded orphan report", payload)
		}
	})

	t.Run("Should list filtered redacted memory history", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspace := filepath.Join(baseDir, "workspace")
		store := memory.NewStore(
			filepath.Join(baseDir, "global"),
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		).ForWorkspace(workspace)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		openCoreTestMemoryCatalog(t, store)
		if err := store.Write(
			memcontract.ScopeWorkspace,
			"project.md",
			[]byte(memoryDocument(t, "Project", memcontract.TypeProject, "common signal")),
		); err != nil {
			t.Fatalf("Write(workspace) error = %v", err)
		}
		since := time.Now().Add(-time.Second).UTC()
		if _, err := store.Search(context.Background(), "common token=super-secret", memcontract.SearchOptions{
			Workspace: workspace,
			Limit:     5,
		}); err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			store,
			nil,
		)
		query := url.Values{}
		query.Set("workspace_id", workspace)
		query.Set("operation", "memory.search")
		query.Set("since", since.Format(time.RFC3339Nano))
		query.Set("limit", "2")
		historyResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/history?"+query.Encode(), nil)
		if historyResp.Code != http.StatusOK {
			t.Fatalf(
				"memory history status = %d, want %d; body=%s",
				historyResp.Code,
				http.StatusOK,
				historyResp.Body.String(),
			)
		}

		var payload contract.MemoryOperationHistoryResponse
		testutil.DecodeJSONResponse(t, historyResp, &payload)
		if len(payload.Operations) != 1 {
			t.Fatalf("len(payload.Operations) = %d, want 1; payload=%#v", len(payload.Operations), payload)
		}
		identity, err := workspacepkg.EnsureIdentity(context.Background(), workspace)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		got := payload.Operations[0]
		if got.Operation != "memory.search" || got.WorkspaceID != identity.WorkspaceID {
			t.Fatalf("operation payload = %#v, want workspace memory.search", got)
		}
		if strings.Contains(got.Summary, "super-secret") || !strings.Contains(got.Summary, "token=[REDACTED]") {
			t.Fatalf("operation summary = %q, want redacted token", got.Summary)
		}
	})

	t.Run("Should map validation errors to bad requests", func(t *testing.T) {
		t.Parallel()

		if status := core.StatusForMemoryError(
			core.NewMemoryValidationError(errors.New("bad")),
		); status != http.StatusBadRequest {
			t.Fatalf("StatusForMemoryError(validation) = %d, want %d", status, http.StatusBadRequest)
		}
	})

	t.Run("Should promote memory across scopes", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _ := setup(t)
		body, err := json.Marshal(contract.MemoryPromoteRequest{
			Filename: "global.md",
			From: contract.MemoryScopeSelectorPayload{
				Scope: memcontract.ScopeGlobal,
			},
			To: contract.MemoryScopeSelectorPayload{
				Scope:       memcontract.ScopeWorkspace,
				WorkspaceID: workspace,
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal(promote request) error = %v", err)
		}

		resp := performRequest(t, fixture.Engine, http.MethodPost, "/memory/promote", body)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"promote memory status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}

		var payload contract.MemoryPromoteResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !payload.Applied || payload.Decision.Scope != memcontract.ScopeWorkspace {
			t.Fatalf("promote payload = %#v, want applied workspace decision", payload)
		}
		readResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/memory/global.md?scope=workspace&workspace_id="+url.QueryEscape(workspace),
			nil,
		)
		if readResp.Code != http.StatusOK {
			t.Fatalf(
				"promoted read status = %d, want %d; body=%s",
				readResp.Code,
				http.StatusOK,
				readResp.Body.String(),
			)
		}
	})

	t.Run("Should expose decision list show and revert", func(t *testing.T) {
		t.Parallel()

		fixture, _, _ := setup(t)
		writeBody, err := json.Marshal(contract.MemoryCreateRequest{
			Scope:   memcontract.ScopeGlobal,
			Type:    memcontract.TypeUser,
			Name:    "Decision API",
			Content: "Decision API body for revert.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(write request) error = %v", err)
		}
		writeResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", writeBody)
		if writeResp.Code != http.StatusOK {
			t.Fatalf(
				"write memory status = %d, want %d; body=%s",
				writeResp.Code,
				http.StatusOK,
				writeResp.Body.String(),
			)
		}
		var writePayload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, writeResp, &writePayload)

		newerBody, err := json.Marshal(contract.MemoryCreateRequest{
			Scope:   memcontract.ScopeGlobal,
			Type:    memcontract.TypeProject,
			Name:    "Newer Decision API",
			Content: "A different decision that must not consume the filtered limit.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(newer write request) error = %v", err)
		}
		newerResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", newerBody)
		if newerResp.Code != http.StatusOK {
			t.Fatalf(
				"newer write status = %d, want %d; body=%s",
				newerResp.Code,
				http.StatusOK,
				newerResp.Body.String(),
			)
		}

		listResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/memory/decisions?scope=global&filename="+
				url.QueryEscape(writePayload.Decision.TargetFilename)+"&limit=1",
			nil,
		)
		if listResp.Code != http.StatusOK {
			t.Fatalf(
				"list decisions status = %d, want %d; body=%s",
				listResp.Code,
				http.StatusOK,
				listResp.Body.String(),
			)
		}
		var listPayload contract.MemoryDecisionListResponse
		testutil.DecodeJSONResponse(t, listResp, &listPayload)
		if len(listPayload.Decisions) != 1 || listPayload.Decisions[0].ID != writePayload.Decision.ID {
			t.Fatalf("decision list = %#v, want only decision %q", listPayload.Decisions, writePayload.Decision.ID)
		}

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/decisions/"+writePayload.Decision.ID, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get decision status = %d, want %d; body=%s", getResp.Code, http.StatusOK, getResp.Body.String())
		}
		var getPayload contract.MemoryDecisionResponse
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if getPayload.Decision.ID != writePayload.Decision.ID || getPayload.Decision.AppliedAt == nil {
			t.Fatalf("get decision payload = %#v, want applied decision", getPayload.Decision)
		}

		revertResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/memory/decisions/"+writePayload.Decision.ID+"/revert",
			[]byte(`{"reason":"test cleanup"}`),
		)
		if revertResp.Code != http.StatusOK {
			t.Fatalf(
				"revert decision status = %d, want %d; body=%s",
				revertResp.Code,
				http.StatusOK,
				revertResp.Body.String(),
			)
		}
		var revertPayload contract.MemoryDecisionRevertResponse
		testutil.DecodeJSONResponse(t, revertResp, &revertPayload)
		if !revertPayload.Reverted || revertPayload.Decision.ID != writePayload.Decision.ID {
			t.Fatalf("revert payload = %#v, want reverted decision", revertPayload)
		}
		readResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/memory/"+writePayload.Decision.TargetFilename+"?scope=global",
			nil,
		)
		if readResp.Code != http.StatusNotFound {
			t.Fatalf("read reverted memory status = %d, want %d", readResp.Code, http.StatusNotFound)
		}
	})

	t.Run("Should revert workspace decisions through the workspace-bound store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewStore(
			filepath.Join(t.TempDir(), "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(t.TempDir(), "agh.db")),
		)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := store.CloseRecallSignalRecorders(ctx); err != nil {
				t.Fatalf("CloseRecallSignalRecorders() error = %v", err)
			}
		})
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		openCoreTestMemoryCatalog(t, store)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		identity, err := workspacepkg.EnsureIdentity(context.Background(), workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != identity.WorkspaceID {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      identity.WorkspaceID,
						Name:    "workspace",
						RootDir: workspaceRoot,
					},
					WorkspaceID: identity.WorkspaceID,
				}, nil
			},
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				if ref != identity.WorkspaceID {
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.Workspace{
					ID:      identity.WorkspaceID,
					Name:    "workspace",
					RootDir: workspaceRoot,
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			workspaces,
			store,
			nil,
		)
		writeBody, err := json.Marshal(contract.MemoryCreateRequest{
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: identity.WorkspaceID,
			Type:        memcontract.TypeProject,
			Name:        "Workspace Revert",
			Content:     "Original workspace body.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(write request) error = %v", err)
		}
		writeResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", writeBody)
		if writeResp.Code != http.StatusOK {
			t.Fatalf("write status = %d, want %d; body=%s", writeResp.Code, http.StatusOK, writeResp.Body.String())
		}
		var writePayload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, writeResp, &writePayload)

		editBody, err := json.Marshal(contract.MemoryEditRequest{
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: identity.WorkspaceID,
			Type:        memcontract.TypeProject,
			Name:        "Workspace Revert",
			Content:     "Edited workspace body.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(edit request) error = %v", err)
		}
		editResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/memory/"+writePayload.Decision.TargetFilename,
			editBody,
		)
		if editResp.Code != http.StatusOK {
			t.Fatalf("edit status = %d, want %d; body=%s", editResp.Code, http.StatusOK, editResp.Body.String())
		}
		var editPayload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, editResp, &editPayload)
		if editPayload.Decision.Scope != memcontract.ScopeWorkspace {
			t.Fatalf("edit decision scope = %q, want workspace", editPayload.Decision.Scope)
		}

		revertResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/memory/decisions/"+editPayload.Decision.ID+"/revert",
			[]byte(`{"reason":"operator revert"}`),
		)
		if revertResp.Code != http.StatusOK {
			t.Fatalf(
				"revert status = %d, want %d; body=%s",
				revertResp.Code,
				http.StatusOK,
				revertResp.Body.String(),
			)
		}
		var revertPayload contract.MemoryDecisionRevertResponse
		testutil.DecodeJSONResponse(t, revertResp, &revertPayload)
		if !revertPayload.Reverted || revertPayload.Decision.ID != editPayload.Decision.ID {
			t.Fatalf("revert payload = %#v, want reverted edit decision", revertPayload)
		}

		readResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/memory/"+editPayload.Decision.TargetFilename+"?scope=workspace&workspace_id="+url.QueryEscape(
				identity.WorkspaceID,
			),
			nil,
		)
		if readResp.Code != http.StatusOK {
			t.Fatalf("read status = %d, want %d; body=%s", readResp.Code, http.StatusOK, readResp.Body.String())
		}
		var readPayload contract.MemoryEntryResponse
		testutil.DecodeJSONResponse(t, readResp, &readPayload)
		if !strings.Contains(readPayload.Memory.Content, "Original workspace body.") {
			t.Fatalf("reverted content = %q, want original workspace body", readPayload.Memory.Content)
		}
	})

	t.Run("Should revert agent workspace-tier decisions through the workspace-bound store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewStore(
			filepath.Join(t.TempDir(), "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(t.TempDir(), "agh.db")),
		)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := store.CloseRecallSignalRecorders(ctx); err != nil {
				t.Fatalf("CloseRecallSignalRecorders() error = %v", err)
			}
		})
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		openCoreTestMemoryCatalog(t, store)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		identity, err := workspacepkg.EnsureIdentity(context.Background(), workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != identity.WorkspaceID {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      identity.WorkspaceID,
						Name:    "workspace",
						RootDir: workspaceRoot,
					},
					WorkspaceID: identity.WorkspaceID,
				}, nil
			},
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				if ref != identity.WorkspaceID {
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.Workspace{
					ID:      identity.WorkspaceID,
					Name:    "workspace",
					RootDir: workspaceRoot,
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			workspaces,
			store,
			nil,
		)
		agentName := "coder"
		writeBody, err := json.Marshal(contract.MemoryCreateRequest{
			Scope:       memcontract.ScopeAgent,
			WorkspaceID: identity.WorkspaceID,
			AgentName:   agentName,
			AgentTier:   memcontract.AgentTierWorkspace,
			Type:        memcontract.TypeProject,
			Name:        "Agent Workspace Revert",
			Content:     "Original agent workspace body.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(write request) error = %v", err)
		}
		writeResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", writeBody)
		if writeResp.Code != http.StatusOK {
			t.Fatalf("write status = %d, want %d; body=%s", writeResp.Code, http.StatusOK, writeResp.Body.String())
		}
		var writePayload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, writeResp, &writePayload)

		editBody, err := json.Marshal(contract.MemoryEditRequest{
			Scope:       memcontract.ScopeAgent,
			WorkspaceID: identity.WorkspaceID,
			AgentName:   agentName,
			AgentTier:   memcontract.AgentTierWorkspace,
			Type:        memcontract.TypeProject,
			Name:        "Agent Workspace Revert",
			Content:     "Edited agent workspace body.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(edit request) error = %v", err)
		}
		editResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/memory/"+writePayload.Decision.TargetFilename,
			editBody,
		)
		if editResp.Code != http.StatusOK {
			t.Fatalf("edit status = %d, want %d; body=%s", editResp.Code, http.StatusOK, editResp.Body.String())
		}
		var editPayload contract.MemoryMutationDecisionResponse
		testutil.DecodeJSONResponse(t, editResp, &editPayload)
		if editPayload.Decision.Scope != memcontract.ScopeAgent ||
			editPayload.Decision.AgentTier != memcontract.AgentTierWorkspace {
			t.Fatalf("edit decision = %#v, want agent workspace-tier decision", editPayload.Decision)
		}

		revertResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/memory/decisions/"+editPayload.Decision.ID+"/revert",
			[]byte(`{"reason":"operator revert"}`),
		)
		if revertResp.Code != http.StatusOK {
			t.Fatalf(
				"revert status = %d, want %d; body=%s",
				revertResp.Code,
				http.StatusOK,
				revertResp.Body.String(),
			)
		}
		var revertPayload contract.MemoryDecisionRevertResponse
		testutil.DecodeJSONResponse(t, revertResp, &revertPayload)
		if !revertPayload.Reverted || revertPayload.Decision.ID != editPayload.Decision.ID {
			t.Fatalf("revert payload = %#v, want reverted edit decision", revertPayload)
		}

		readResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/memory/"+editPayload.Decision.TargetFilename+"?scope=agent&agent_name="+
				url.QueryEscape(agentName)+
				"&agent_tier=workspace&workspace_id="+url.QueryEscape(identity.WorkspaceID),
			nil,
		)
		if readResp.Code != http.StatusOK {
			t.Fatalf("read status = %d, want %d; body=%s", readResp.Code, http.StatusOK, readResp.Body.String())
		}
		var readPayload contract.MemoryEntryResponse
		testutil.DecodeJSONResponse(t, readResp, &readPayload)
		if !strings.Contains(readPayload.Memory.Content, "Original agent workspace body.") {
			t.Fatalf("reverted content = %q, want original agent workspace body", readPayload.Memory.Content)
		}
	})

	t.Run("Should reset reload list daily logs and create ad-hoc notes", func(t *testing.T) {
		t.Parallel()

		fixture, _, _ := setup(t)
		writeBody, err := json.Marshal(contract.MemoryCreateRequest{
			Scope:   memcontract.ScopeGlobal,
			Type:    memcontract.TypeUser,
			Name:    "Daily Event",
			Content: "Daily API event body.",
		})
		if err != nil {
			t.Fatalf("json.Marshal(write request) error = %v", err)
		}
		writeResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory", writeBody)
		if writeResp.Code != http.StatusOK {
			t.Fatalf(
				"write memory status = %d, want %d; body=%s",
				writeResp.Code,
				http.StatusOK,
				writeResp.Body.String(),
			)
		}

		dailyResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/daily?limit=5", nil)
		if dailyResp.Code != http.StatusOK {
			t.Fatalf("daily status = %d, want %d; body=%s", dailyResp.Code, http.StatusOK, dailyResp.Body.String())
		}
		var dailyPayload contract.MemoryDailyLogListResponse
		testutil.DecodeJSONResponse(t, dailyResp, &dailyPayload)
		if len(dailyPayload.Logs) == 0 || dailyPayload.Logs[0].OperationCount == 0 || dailyPayload.Logs[0].Path == "" {
			t.Fatalf("daily payload = %#v, want at least one operation log", dailyPayload)
		}

		resetResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/memory/reset",
			[]byte(`{"scope":"global","derived_only":true,"confirm":true}`),
		)
		if resetResp.Code != http.StatusOK {
			t.Fatalf("reset status = %d, want %d; body=%s", resetResp.Code, http.StatusOK, resetResp.Body.String())
		}
		var resetPayload contract.MemoryResetResponse
		testutil.DecodeJSONResponse(t, resetResp, &resetPayload)
		if !resetPayload.DerivedOnly || resetPayload.ResetAt.IsZero() {
			t.Fatalf("reset payload = %#v, want derived reset timestamp", resetPayload)
		}

		reloadResp := performRequest(t, fixture.Engine, http.MethodPost, "/memory/reload?scope=global", nil)
		if reloadResp.Code != http.StatusOK {
			t.Fatalf("reload status = %d, want %d; body=%s", reloadResp.Code, http.StatusOK, reloadResp.Body.String())
		}
		var reloadPayload contract.MemoryReloadResponse
		testutil.DecodeJSONResponse(t, reloadResp, &reloadPayload)
		if reloadPayload.Generation == 0 || reloadPayload.ReloadedAt.IsZero() {
			t.Fatalf("reload payload = %#v, want generation timestamp", reloadPayload)
		}

		adhocResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/memory/ad-hoc",
			[]byte(`{"scope":"global","content":"Remember ad-hoc API notes.","slug":"api-note"}`),
		)
		if adhocResp.Code != http.StatusOK {
			t.Fatalf("ad-hoc status = %d, want %d; body=%s", adhocResp.Code, http.StatusOK, adhocResp.Body.String())
		}
		var adhocPayload contract.MemoryAdhocNoteResponse
		testutil.DecodeJSONResponse(t, adhocResp, &adhocPayload)
		if !adhocPayload.Accepted || !strings.Contains(adhocPayload.Path, "api-note") {
			t.Fatalf("ad-hoc payload = %#v, want accepted note path", adhocPayload)
		}
	})

	t.Run("Should return truthful dream and recall trace responses", func(t *testing.T) {
		t.Parallel()

		fixture, _, _ := setup(t)
		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/dreams", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("dream list status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
		}
		var listPayload contract.MemoryDreamListResponse
		testutil.DecodeJSONResponse(t, listResp, &listPayload)
		if listPayload.Dreams == nil {
			t.Fatalf("dream list payload = %#v, want non-nil list", listPayload)
		}

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/dreams/missing-run", nil)
		if getResp.Code != http.StatusNotFound {
			t.Fatalf("dream get status = %d, want %d", getResp.Code, http.StatusNotFound)
		}

		traceResp := performRequest(t, fixture.Engine, http.MethodGet, "/memory/recall-traces/sess-1/7", nil)
		if traceResp.Code != http.StatusNotFound {
			t.Fatalf("recall trace status = %d, want %d", traceResp.Code, http.StatusNotFound)
		}
	})
}

func openCoreTestMemoryCatalog(t *testing.T, store *memory.Store) {
	t.Helper()
	if err := store.OpenCatalog(t.Context()); err != nil {
		t.Fatalf("Store.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(context.Background()); err != nil {
			t.Errorf("Store.CloseCatalog() error = %v", err)
		}
	})
}

func TestWorkspaceHandlersDelegateToService(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (handlerFixture, workspacepkg.Workspace, workspacepkg.ResolvedWorkspace, *bool, *bool, *bool, string, string) {
		t.Helper()

		rootDir := filepath.Join(t.TempDir(), "root dir")
		addDir := filepath.Join(t.TempDir(), "add dir")
		if err := os.MkdirAll(rootDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(rootDir) error = %v", err)
		}
		if err := os.MkdirAll(addDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(addDir) error = %v", err)
		}
		workspace := workspacepkg.Workspace{
			ID:             "ws_alpha",
			RootDir:        rootDir,
			AdditionalDirs: []string{addDir},
			Name:           "alpha",
			DefaultAgent:   "coder",
			SandboxRef:     "daytona-dev",
			CreatedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt:      time.Date(2026, 4, 3, 12, 1, 0, 0, time.UTC),
		}
		resolved := workspacepkg.ResolvedWorkspace{
			Workspace: workspace,
			Config: aghconfig.Config{
				Providers: map[string]aghconfig.ProviderConfig{
					"alpha": {Command: "alpha --acp"},
				},
			},
			Agents: []aghconfig.AgentDef{{
				Name:     "coder",
				Provider: "fake",
				Prompt:   "hello",
			}},
			Skills: []workspacepkg.SkillPath{{
				Dir:    filepath.Join(rootDir, ".skills", "build"),
				Source: "workspace",
			}},
		}
		updateCalled := false
		deleteCalled := false
		resolveCalled := false
		workspaces := testutil.StubWorkspaceService{
			RegisterFn: func(_ context.Context, opts workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
				if opts.RootDir != rootDir || len(opts.AdditionalDirs) != 1 ||
					opts.DefaultAgent != "coder" ||
					opts.SandboxRef != "daytona-dev" {
					t.Fatalf("Register opts = %#v", opts)
				}
				return workspace, nil
			},
			ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
				return []workspacepkg.Workspace{workspace}, nil
			},
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return resolved, nil
			},
			UpdateFn: func(_ context.Context, id string, opts workspacepkg.UpdateOptions) error {
				updateCalled = true
				if id != workspace.ID || opts.Name == nil || *opts.Name != "beta" {
					t.Fatalf("Update call = %q %#v", id, opts)
				}
				if opts.SandboxRef != nil && *opts.SandboxRef != "local-dev" {
					t.Fatalf("Update sandbox ref = %#v, want local-dev", opts.SandboxRef)
				}
				return nil
			},
			UnregisterFn: func(_ context.Context, id string) error {
				deleteCalled = true
				if id != workspace.ID {
					t.Fatalf("Unregister id = %q, want %q", id, workspace.ID)
				}
				return nil
			},
			ResolveOrRegisterFn: func(_ context.Context, path string) (workspacepkg.ResolvedWorkspace, error) {
				resolveCalled = true
				if path != rootDir {
					t.Fatalf("ResolveOrRegister path = %q, want %q", path, rootDir)
				}
				return resolved, nil
			},
		}
		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				info := testutil.NewSessionInfo("sess-a")
				info.WorkspaceID = workspace.ID
				info.State = session.StateStopped
				return []*session.Info{info}, nil
			},
		}

		return newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			workspaces,
			nil,
			nil,
		), workspace, resolved, &updateCalled, &deleteCalled, &resolveCalled, rootDir, addDir
	}

	t.Run("Should create a workspace", func(t *testing.T) {
		t.Parallel()

		fixture, _, _, _, _, _, rootDir, addDir := setup(t)
		createBody, err := json.Marshal(contract.CreateWorkspaceRequest{
			RootDir:      rootDir,
			AddDirs:      []string{addDir},
			Name:         "alpha",
			DefaultAgent: "coder",
			SandboxRef:   "daytona-dev",
		})
		if err != nil {
			t.Fatalf("json.Marshal(create workspace request) error = %v", err)
		}
		createResp := performRequest(t, fixture.Engine, http.MethodPost, "/workspaces", createBody)
		if createResp.Code != http.StatusCreated {
			t.Fatalf("create workspace status = %d, want %d", createResp.Code, http.StatusCreated)
		}

		var payload struct {
			Workspace contract.WorkspacePayload `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, createResp, &payload)
		if payload.Workspace.RootDir != rootDir || len(payload.Workspace.AddDirs) != 1 ||
			payload.Workspace.AddDirs[0] != addDir {
			t.Fatalf("create workspace payload = %#v", payload.Workspace)
		}
	})

	t.Run("Should list workspaces", func(t *testing.T) {
		t.Parallel()

		fixture, _, _, _, _, _, _, _ := setup(t)
		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list workspaces status = %d, want %d", listResp.Code, http.StatusOK)
		}

		var payload struct {
			Workspaces []contract.WorkspacePayload `json:"workspaces"`
		}
		testutil.DecodeJSONResponse(t, listResp, &payload)
		if len(payload.Workspaces) != 1 || payload.Workspaces[0].ID != "ws_alpha" {
			t.Fatalf("list workspaces payload = %#v", payload.Workspaces)
		}
	})

	t.Run("Should get a workspace with sessions", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, resolved, _, _, _, _, _ := setup(t)
		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces/"+workspace.ID, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get workspace status = %d, want %d", getResp.Code, http.StatusOK)
		}

		var getPayload contract.WorkspaceDetailPayload
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if len(getPayload.Sessions) != 1 || getPayload.Sessions[0].WorkspaceID != workspace.ID {
			t.Fatalf("sessions payload = %#v", getPayload.Sessions)
		}
		expectedProviders := core.SessionProviderOptionPayloadsFromConfig(&resolved.Config)
		if got, want := len(getPayload.Providers), len(expectedProviders); got != want {
			t.Fatalf("len(providers) = %d, want %d (%#v)", got, want, getPayload.Providers)
		}
		for i, want := range expectedProviders {
			if got := getPayload.Providers[i]; !reflect.DeepEqual(got, want) {
				t.Fatalf("providers[%d] = %#v, want %#v", i, got, want)
			}
		}
	})

	t.Run("Should merge projected catalog agents into workspace detail", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _, _, _, _, _, _ := setup(t)
		fixture.Handlers.AgentCatalog = stubAgentCatalog{
			agents: []aghconfig.AgentDef{
				{
					Name:     "coder",
					Provider: "catalog-should-not-win",
					Prompt:   "global duplicate",
				},
				{
					Name:     "qa-extension-agent",
					Provider: "codex",
					Prompt:   "extension agent",
				},
				{
					Name:     "onboarding",
					Provider: "codex",
					Prompt:   "internal onboarding",
				},
			},
		}

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces/"+workspace.ID, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get workspace status = %d, want %d", getResp.Code, http.StatusOK)
		}

		var getPayload contract.WorkspaceDetailPayload
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if got, want := len(getPayload.Agents), 3; got != want {
			t.Fatalf("len(agents) = %d, want %d: %#v", got, want, getPayload.Agents)
		}
		if got, want := getPayload.Agents[0].Name, "coder"; got != want {
			t.Fatalf("agents[0].name = %q, want %q", got, want)
		}
		if got, want := getPayload.Agents[0].Provider, "fake"; got != want {
			t.Fatalf("agents[0].provider = %q, want workspace-scoped provider %q", got, want)
		}
		if got, want := getPayload.Agents[1].Name, "onboarding"; got != want {
			t.Fatalf("agents[1].name = %q, want %q", got, want)
		}
		if got, want := getPayload.Agents[2].Name, "qa-extension-agent"; got != want {
			t.Fatalf("agents[2].name = %q, want %q", got, want)
		}
	})

	t.Run("Should update a workspace via the service", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _, updateCalled, _, _, _, _ := setup(t)
		updateResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/"+workspace.ID,
			[]byte(`{"name":"beta"}`),
		)
		if updateResp.Code != http.StatusOK || !*updateCalled {
			t.Fatalf("update status=%d called=%v", updateResp.Code, *updateCalled)
		}

		var payload struct {
			Workspace contract.WorkspacePayload `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, updateResp, &payload)
		if payload.Workspace.Name != "alpha" {
			t.Fatalf("update workspace payload = %#v", payload.Workspace)
		}
	})

	t.Run("Should delete a workspace via the service", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _, _, deleteCalled, _, _, _ := setup(t)
		fixture.Handlers.Sessions = testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				t.Fatal("DeleteWorkspace() called Sessions.ListAll")
				return nil, nil
			},
			DeleteFn: func(context.Context, string) error {
				t.Fatal("DeleteWorkspace() called Sessions.Delete")
				return nil
			},
		}
		deleteResp := performRequest(t, fixture.Engine, http.MethodDelete, "/workspaces/"+workspace.ID, nil)
		if deleteResp.Code != http.StatusNoContent || !*deleteCalled {
			t.Fatalf("delete status=%d called=%v", deleteResp.Code, *deleteCalled)
		}
	})

	t.Run("Should resolve a workspace path via the service", func(t *testing.T) {
		t.Parallel()

		fixture, _, _, _, _, resolveCalled, rootDir, _ := setup(t)
		resolveBody, err := json.Marshal(contract.ResolveWorkspaceRequest{Path: rootDir})
		if err != nil {
			t.Fatalf("json.Marshal(resolve workspace request) error = %v", err)
		}
		resolveResp := performRequest(t, fixture.Engine, http.MethodPost, "/workspaces/resolve", resolveBody)
		if resolveResp.Code != http.StatusOK || !*resolveCalled {
			t.Fatalf("resolve status=%d called=%v", resolveResp.Code, *resolveCalled)
		}

		var payload struct {
			Workspace contract.WorkspacePayload `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, resolveResp, &payload)
		if payload.Workspace.RootDir != rootDir {
			t.Fatalf("resolve workspace payload = %#v", payload.Workspace)
		}
	})
}

func TestWorkspaceUpdateSupportsAddDirsAndDefaultAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should update add_dirs and default_agent", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		addDir := t.TempDir()
		workspace := workspacepkg.Workspace{ID: "ws_alpha", RootDir: rootDir, Name: "alpha"}
		var captured workspacepkg.UpdateOptions
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			UpdateFn: func(_ context.Context, _ string, opts workspacepkg.UpdateOptions) error {
				captured = opts
				return nil
			},
		}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, workspaces, nil, nil)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/ws_alpha",
			[]byte(`{"add_dirs":["`+addDir+`"],"default_agent":"coder","sandbox_ref":"local-dev"}`),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("update add_dirs/default_agent status = %d, want %d", resp.Code, http.StatusOK)
		}
		if captured.AdditionalDirs == nil || len(*captured.AdditionalDirs) != 1 ||
			(*captured.AdditionalDirs)[0] != addDir {
			t.Fatalf("captured add dirs = %#v", captured.AdditionalDirs)
		}
		if captured.DefaultAgent == nil || *captured.DefaultAgent != "coder" {
			t.Fatalf("captured default agent = %#v", captured.DefaultAgent)
		}
		if captured.SandboxRef == nil || *captured.SandboxRef != "local-dev" {
			t.Fatalf("captured sandbox ref = %#v", captured.SandboxRef)
		}
	})
}

func memoryDocument(t *testing.T, name string, typ memcontract.Type, body string) string {
	t.Helper()

	header := memcontract.Header{
		Name:        name,
		Description: "desc",
		Type:        typ,
	}
	metadata, err := yaml.Marshal(header)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	return "---\n" + string(metadata) + "---\n\n" + body
}

func seedCoreMemoryCatalogDocuments(t *testing.T, dir string, count int) {
	t.Helper()

	baseTime := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	for index := range count {
		typ := memcontract.TypeProject
		if index == 0 {
			typ = memcontract.TypeReference
		}
		filename := fmt.Sprintf("memory-%03d.md", index)
		content := memoryDocument(t, fmt.Sprintf("Memory %03d", index), typ, "body")
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", filename, err)
		}
		modTime := baseTime.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("Chtimes(%s) error = %v", filename, err)
		}
	}
}
