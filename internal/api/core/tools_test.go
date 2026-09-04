package core_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestToolHandlersExposeOperatorSessionInvokeAndToolsets(t *testing.T) {
	t.Parallel()

	t.Run("Should expose operator session invoke and toolset handlers", func(t *testing.T) {
		t.Parallel()

		registry := newAPITestToolRegistry(t, false)
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			Sessions:           networkTestSessionManager("ws-workspace", "sess-1"),
			Observer:           testutil.StubObserver{},
			Tasks:              &testutil.StubTaskManager{},
			Workspaces:         defaultCoreWorkspaceService(testutil.StubWorkspaceService{}),
			Tools:              registry,
			Toolsets:           registry,
			ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
			HomePaths:          homePaths,
			Config:             cfg,
			Logger:             testutil.DiscardLogger(),
			StartedAt:          time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			Now:                func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
			PollInterval:       time.Millisecond,
			StreamDone:         make(chan struct{}),
			MaskInternalErrors: false,
		})
		engine := newToolCoreEngine(t, handlers)

		listResp := performRequest(t, engine, http.MethodGet, "/tools", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf(
				"operator list status = %d, want %d; body=%s",
				listResp.Code,
				http.StatusOK,
				listResp.Body.String(),
			)
		}
		var list contract.ToolsResponse
		decodeToolJSON(t, listResp.Body.Bytes(), &list)
		if got, want := len(list.Tools), 2; got != want {
			t.Fatalf("operator tool count = %d, want %d", got, want)
		}

		sessionResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-workspace/sessions/sess-1/tools", nil)
		if sessionResp.Code != http.StatusOK {
			t.Fatalf("session list status = %d, want %d", sessionResp.Code, http.StatusOK)
		}
		var sessionTools contract.ToolsResponse
		decodeToolJSON(t, sessionResp.Body.Bytes(), &sessionTools)
		if got, want := len(sessionTools.Tools), 1; got != want {
			t.Fatalf("session tool count = %d, want %d", got, want)
		}
		if sessionTools.Tools[0].Descriptor.ToolID != toolspkg.ToolIDSkillView {
			t.Fatalf("session tool = %q, want %q", sessionTools.Tools[0].Descriptor.ToolID, toolspkg.ToolIDSkillView)
		}

		searchResp := performRequest(t, engine, http.MethodPost, "/tools/search", []byte(`{"query":"skill","limit":1}`))
		if searchResp.Code != http.StatusOK {
			t.Fatalf("search status = %d, want %d", searchResp.Code, http.StatusOK)
		}
		var search contract.ToolsResponse
		decodeToolJSON(t, searchResp.Body.Bytes(), &search)
		if got, want := len(search.Tools), 1; got != want {
			t.Fatalf("search tool count = %d, want %d", got, want)
		}

		sessionSearchResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-1/tools/search",
			[]byte(`{"query":"skill","limit":1,"workspace_id":"ws-workspace","session_id":"sess-1"}`),
		)
		if sessionSearchResp.Code != http.StatusOK {
			t.Fatalf("session search status = %d, want %d", sessionSearchResp.Code, http.StatusOK)
		}
		var sessionSearch contract.ToolsResponse
		decodeToolJSON(t, sessionSearchResp.Body.Bytes(), &sessionSearch)
		if got, want := len(sessionSearch.Tools), 1; got != want {
			t.Fatalf("session search tool count = %d, want %d", got, want)
		}
		searchScope, searchQuery := registry.lastSearch()
		if searchScope.SessionID != "sess-1" || searchScope.WorkspaceID != "ws-workspace" || searchScope.Operator {
			t.Fatalf("session search scope = %#v, want session workspace scope", searchScope)
		}
		if searchQuery.Query != "skill" || searchQuery.Limit != 1 {
			t.Fatalf("session search query = %#v, want skill limit", searchQuery)
		}

		getResp := performRequest(t, engine, http.MethodGet, "/tools/compozy__skill_view", nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d", getResp.Code, http.StatusOK)
		}
		var gotTool contract.ToolResponse
		decodeToolJSON(t, getResp.Body.Bytes(), &gotTool)
		if gotTool.Tool.Descriptor.Backend.Kind != toolspkg.BackendNativeGo {
			t.Fatalf("backend kind = %q, want %q", gotTool.Tool.Descriptor.Backend.Kind, toolspkg.BackendNativeGo)
		}

		invokeResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/compozy__skill_view/invoke",
			[]byte(`{"session_id":"sess-1","workspace_id":"ws-1","input":{"message":"hello"}}`),
		)
		if invokeResp.Code != http.StatusOK {
			t.Fatalf("invoke status = %d, want %d; body=%s", invokeResp.Code, http.StatusOK, invokeResp.Body.String())
		}
		var invoke contract.ToolInvokeResponse
		decodeToolJSON(t, invokeResp.Body.Bytes(), &invoke)
		if invoke.ToolID != toolspkg.ToolIDSkillView || invoke.Status != "completed" {
			t.Fatalf("invoke response = %#v, want completed skill_view", invoke)
		}
		if registry.callCount(toolspkg.ToolIDSkillView) != 1 {
			t.Fatalf("registry call count = %d, want 1", registry.callCount(toolspkg.ToolIDSkillView))
		}

		toolsetsResp := performRequest(t, engine, http.MethodGet, "/toolsets", nil)
		if toolsetsResp.Code != http.StatusOK {
			t.Fatalf("toolsets status = %d, want %d", toolsetsResp.Code, http.StatusOK)
		}
		var toolsets contract.ToolsetsResponse
		decodeToolJSON(t, toolsetsResp.Body.Bytes(), &toolsets)
		if got, want := len(toolsets.Toolsets), 1; got != want {
			t.Fatalf("toolset count = %d, want %d", got, want)
		}
		if toolsets.Toolsets[0].Status != "expanded" ||
			len(toolsets.Toolsets[0].ExpandedTools) != 1 ||
			toolsets.Toolsets[0].ExpandedTools[0] != toolspkg.ToolIDSkillView {
			t.Fatalf("toolset payload = %#v, want expanded skill_view", toolsets.Toolsets[0])
		}

		toolsetResp := performRequest(t, engine, http.MethodGet, "/toolsets/compozy__catalog", nil)
		if toolsetResp.Code != http.StatusOK {
			t.Fatalf("toolset get status = %d, want %d", toolsetResp.Code, http.StatusOK)
		}
		var toolset contract.ToolsetResponse
		decodeToolJSON(t, toolsetResp.Body.Bytes(), &toolset)
		if toolset.Toolset.ID != toolspkg.ToolsetIDCatalog || registry.lastToolsetID() != toolspkg.ToolsetIDCatalog {
			t.Fatalf("toolset get = %#v id=%q, want catalog", toolset.Toolset, registry.lastToolsetID())
		}
	})
}

func TestToolHandlersProjectSelectedProfileCatalog(t *testing.T) {
	t.Parallel()

	registry := newAPITestToolRegistry(t, false)
	registry.restrictListTo("profile-marketing", "ws-owner")
	homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:      "api-core-test",
		Profiles:           sessionProfileServiceStub{},
		Tools:              registry,
		Toolsets:           registry,
		ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
		HomePaths:          homePaths,
		Config:             cfg,
		Logger:             testutil.DiscardLogger(),
		StartedAt:          time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		Now:                func() time.Time { return time.Date(2026, 9, 4, 12, 0, 1, 0, time.UTC) },
		PollInterval:       time.Millisecond,
		StreamDone:         make(chan struct{}),
		MaskInternalErrors: false,
	})
	engine := newToolCoreEngine(t, handlers)

	t.Run("Should expose tools only in the owning profile and workspace", func(t *testing.T) {
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/tools?profile=marketing&workspace_id=ws-owner",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("owning scope status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		var payload contract.ToolsResponse
		decodeToolJSON(t, response.Body.Bytes(), &payload)
		if got, want := len(payload.Tools), 2; got != want {
			t.Fatalf("owning scope tool count = %d, want %d", got, want)
		}
		scope := registry.lastList()
		if scope.ProfileID != "profile-marketing" || scope.WorkspaceID != "ws-owner" || !scope.Operator {
			t.Fatalf("owning scope = %#v, want selected profile and workspace operator scope", scope)
		}
	})

	t.Run("Should isolate the default profile", func(t *testing.T) {
		response := performRequest(t, engine, http.MethodGet, "/tools?workspace_id=ws-owner", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("default scope status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		var payload contract.ToolsResponse
		decodeToolJSON(t, response.Body.Bytes(), &payload)
		if got := len(payload.Tools); got != 0 {
			t.Fatalf("default scope tool count = %d, want 0", got)
		}
		if scope := registry.lastList(); scope.ProfileID != store.DefaultProfileID {
			t.Fatalf("default scope profile = %q, want %q", scope.ProfileID, store.DefaultProfileID)
		}
	})

	t.Run("Should isolate a peer workspace", func(t *testing.T) {
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/tools?profile=marketing&workspace_id=ws-peer",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("peer scope status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		var payload contract.ToolsResponse
		decodeToolJSON(t, response.Body.Bytes(), &payload)
		if got := len(payload.Tools); got != 0 {
			t.Fatalf("peer scope tool count = %d, want 0", got)
		}
	})

	t.Run("Should reject an all-profile tool projection", func(t *testing.T) {
		response := performRequest(t, engine, http.MethodGet, "/tools?all_profiles=true", nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("all-profile status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body)
		}
		if !strings.Contains(response.Body.String(), "tools require exactly one profile") {
			t.Fatalf("all-profile body = %s, want profile selection error", response.Body)
		}
	})

	t.Run("Should preserve the selected profile across operator tool paths", func(t *testing.T) {
		search := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/search?profile=marketing",
			[]byte(`{"query":"skill","workspace_id":"ws-owner"}`),
		)
		if search.Code != http.StatusOK {
			t.Fatalf("search status = %d, want %d; body=%s", search.Code, http.StatusOK, search.Body)
		}
		searchScope, _ := registry.lastSearch()
		if searchScope.ProfileID != "profile-marketing" || searchScope.WorkspaceID != "ws-owner" {
			t.Fatalf("search scope = %#v, want selected profile and workspace", searchScope)
		}

		get := performRequest(
			t,
			engine,
			http.MethodGet,
			"/tools/compozy__skill_view?profile=marketing&workspace_id=ws-owner",
			nil,
		)
		if get.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d; body=%s", get.Code, http.StatusOK, get.Body)
		}
		if scope := registry.lastGet(); scope.ProfileID != "profile-marketing" || scope.WorkspaceID != "ws-owner" {
			t.Fatalf("get scope = %#v, want selected profile and workspace", scope)
		}

		approval := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/compozy__skill_view/approvals?profile=marketing",
			[]byte(`{"session_id":"sess-1","workspace_id":"ws-owner","input":{"message":"hello"}}`),
		)
		if approval.Code != http.StatusCreated {
			t.Fatalf("approval status = %d, want %d; body=%s", approval.Code, http.StatusCreated, approval.Body)
		}
		if scope := registry.lastGet(); scope.ProfileID != "profile-marketing" || scope.WorkspaceID != "ws-owner" {
			t.Fatalf("approval scope = %#v, want selected profile and workspace", scope)
		}

		registry.setCallError(toolspkg.ToolIDSkillView, toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			toolspkg.ToolIDSkillView,
			"peer workspace denied",
			toolspkg.ErrToolDenied,
			toolspkg.ReasonWorkspaceAccessDenied,
		))
		invoke := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/compozy__skill_view/invoke?profile=marketing",
			[]byte(`{"session_id":"sess-1","workspace_id":"ws-peer","input":{"message":"hello"}}`),
		)
		if invoke.Code != http.StatusForbidden {
			t.Fatalf("invoke status = %d, want %d; body=%s", invoke.Code, http.StatusForbidden, invoke.Body)
		}
		callScope, _ := registry.lastCall()
		if callScope.ProfileID != "profile-marketing" || callScope.WorkspaceID != "ws-peer" {
			t.Fatalf("invoke scope = %#v, want selected profile and peer workspace", callScope)
		}

		listToolsets := performRequest(
			t,
			engine,
			http.MethodGet,
			"/toolsets?profile=marketing&workspace_id=ws-owner",
			nil,
		)
		if listToolsets.Code != http.StatusOK {
			t.Fatalf("list toolsets status = %d, want %d; body=%s", listToolsets.Code, http.StatusOK, listToolsets.Body)
		}
		if scope := registry.lastToolsetList(); scope.ProfileID != "profile-marketing" ||
			scope.WorkspaceID != "ws-owner" {
			t.Fatalf("list toolsets scope = %#v, want selected profile and workspace", scope)
		}

		getToolset := performRequest(
			t,
			engine,
			http.MethodGet,
			"/toolsets/compozy__catalog?profile=marketing&workspace_id=ws-owner",
			nil,
		)
		if getToolset.Code != http.StatusOK {
			t.Fatalf("get toolset status = %d, want %d; body=%s", getToolset.Code, http.StatusOK, getToolset.Body)
		}
		if scope := registry.lastToolsetGet(); scope.ProfileID != "profile-marketing" ||
			scope.WorkspaceID != "ws-owner" {
			t.Fatalf("get toolset scope = %#v, want selected profile and workspace", scope)
		}
	})
}

func TestToolArtifactHandlersPreserveWorkspaceScopeAndExactPages(t *testing.T) {
	t.Parallel()

	t.Run("Should page exact bytes through the durable workspace identity", func(t *testing.T) {
		t.Parallel()

		store, err := toolspkg.OpenFilesystemToolArtifactStore(
			t.Context(),
			t.TempDir(),
			toolspkg.ToolArtifactRetention{MaxCount: 10, MaxBytes: 1 << 20, MaxAge: time.Hour},
		)
		if err != nil {
			t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
		}
		content := []byte(`{"version":"compozy.tool-result/v1","tail":"D6-TAIL"}`)
		ref, err := store.Put(t.Context(), "workspace-durable", content)
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		artifactID, err := toolspkg.ParseToolArtifactURI(ref.URI)
		if err != nil {
			t.Fatalf("ParseToolArtifactURI() error = %v", err)
		}

		workspaces := defaultCoreWorkspaceService(testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref == "workspace-alias" {
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "registry-workspace"},
						WorkspaceID: "workspace-durable",
					}, nil
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "registry-other"},
					WorkspaceID: "workspace-other",
				}, nil
			},
		})
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			Workspaces:         workspaces,
			ToolArtifacts:      store,
			HomePaths:          homePaths,
			Config:             cfg,
			Logger:             testutil.DiscardLogger(),
			MaskInternalErrors: false,
		})
		engine := newToolCoreEngine(t, handlers)

		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/workspace-alias/tool-artifacts/"+artifactID+"?offset=2&limit=7",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("artifact read status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		var page contract.ToolArtifactPageResponse
		decodeToolJSON(t, response.Body.Bytes(), &page)
		decoded, err := base64.StdEncoding.DecodeString(page.DataBase64)
		if err != nil {
			t.Fatalf("DecodeString() error = %v", err)
		}
		if got, want := string(decoded), string(content[2:9]); got != want {
			t.Fatalf("artifact page = %q, want %q", got, want)
		}
		if page.Artifact != ref || page.Offset != 2 || page.Bytes != 7 || page.NextOffset != 9 || page.EOF {
			t.Fatalf("artifact page metadata = %#v, want exact non-terminal page", page)
		}

		foreign := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/workspace-other/tool-artifacts/"+artifactID,
			nil,
		)
		if foreign.Code != http.StatusNotFound {
			t.Fatalf("foreign artifact status = %d, want %d; body=%s", foreign.Code, http.StatusNotFound, foreign.Body)
		}

		invalid := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/workspace-alias/tool-artifacts/"+artifactID+"?limit=65537",
			nil,
		)
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf(
				"invalid artifact status = %d, want %d; body=%s",
				invalid.Code,
				http.StatusBadRequest,
				invalid.Body,
			)
		}
		var invalidPayload contract.ToolErrorResponse
		decodeToolJSON(t, invalid.Body.Bytes(), &invalidPayload)
		if invalidPayload.Error.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("invalid artifact error = %#v, want invalid_input", invalidPayload.Error)
		}

		beyondEnd := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/workspace-alias/tool-artifacts/"+artifactID+"?offset=999",
			nil,
		)
		if beyondEnd.Code != http.StatusBadRequest {
			t.Fatalf(
				"beyond-end artifact status = %d, want %d; body=%s",
				beyondEnd.Code,
				http.StatusBadRequest,
				beyondEnd.Body,
			)
		}
		var beyondEndPayload contract.ToolErrorResponse
		decodeToolJSON(t, beyondEnd.Body.Bytes(), &beyondEndPayload)
		if beyondEndPayload.Error.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("beyond-end artifact error = %#v, want invalid_input", beyondEndPayload.Error)
		}
	})
}

func TestToolErrorResponses(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve typed terminal details without parsing the message", func(t *testing.T) {
		t.Parallel()

		controller := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "client:web"}
		domainErr := &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeLimitReached, Message: "opaque terminal refusal",
			Current: 8, Max: 10, Controller: &controller, Path: "/workspace",
			Mode: terminalpkg.ModePTY, Platform: "windows", Err: terminalpkg.ErrLimitReached,
		}
		err := toolspkg.NewToolError(
			toolspkg.ErrorCode(terminalpkg.ErrorCodeLimitReached),
			toolspkg.ToolIDTerminalOpen,
			domainErr.Error(),
			domainErr,
			toolspkg.ReasonCode(terminalpkg.ErrorCodeLimitReached),
		)
		payload := core.ToolErrorResponseForError(err, http.StatusConflict, true)
		want := map[string]string{
			"current": "8", "max": "10",
			"controller": `{"kind":"human","id":"client:web"}`,
			"path":       `"/workspace"`, "mode": `"pty"`, "platform": `"windows"`,
		}
		for key, value := range want {
			if got := string(payload.Error.Details[key]); got != value {
				t.Fatalf("terminal tool detail %q = %q, want %q", key, got, value)
			}
		}
	})

	t.Run("Should describe a missing config path without reporting the tool missing", func(t *testing.T) {
		t.Parallel()

		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			toolspkg.ToolIDConfigGet,
			`config path "loops.inputs.extension-probe.preference" not found`,
			toolspkg.ErrToolNotFound,
			toolspkg.ReasonConfigPathNotFound,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)

		if status != http.StatusNotFound ||
			payload.Error.Code != toolspkg.ErrorCodeNotFound ||
			payload.Error.ToolID != toolspkg.ToolIDConfigGet ||
			payload.Error.Message != "config path not found" ||
			!slices.Equal(payload.Error.ReasonCodes, []toolspkg.ReasonCode{toolspkg.ReasonConfigPathNotFound}) {
			t.Fatalf("missing config payload = %#v status=%d, want reason-aware 404", payload.Error, status)
		}
		if strings.Contains(payload.Error.Message, "tool not found") {
			t.Fatalf("missing config message = %q, must not identify the tool as missing", payload.Error.Message)
		}
	})

	t.Run("Should describe a missing skill resource without reporting the tool missing", func(t *testing.T) {
		t.Parallel()

		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			toolspkg.ToolIDSkillView,
			`skill resource "references/terminal.md" not found`,
			toolspkg.ErrToolNotFound,
			toolspkg.ReasonSkillResourceNotFound,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)

		if status != http.StatusNotFound ||
			payload.Error.Code != toolspkg.ErrorCodeNotFound ||
			payload.Error.ToolID != toolspkg.ToolIDSkillView ||
			payload.Error.Message != "skill resource not found" ||
			!slices.Equal(
				payload.Error.ReasonCodes,
				[]toolspkg.ReasonCode{toolspkg.ReasonSkillResourceNotFound},
			) {
			t.Fatalf("missing skill resource payload = %#v status=%d, want reason-aware 404", payload.Error, status)
		}
		if strings.Contains(payload.Error.Message, "tool not found") {
			t.Fatalf(
				"missing skill resource message = %q, must not identify the tool as missing",
				payload.Error.Message,
			)
		}
	})

	t.Run("Should describe malformed skill frontmatter as an invalid definition", func(t *testing.T) {
		t.Parallel()

		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolspkg.ToolIDSkillView,
			"internal parser detail",
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonSkillDefinitionInvalid,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)

		if status != http.StatusBadRequest ||
			payload.Error.Message != "skill definition is invalid" ||
			!slices.Equal(
				payload.Error.ReasonCodes,
				[]toolspkg.ReasonCode{toolspkg.ReasonSkillDefinitionInvalid},
			) {
			t.Fatalf("invalid skill definition payload = %#v status=%d, want reason-aware 400", payload.Error, status)
		}
	})

	t.Run("Should let the error code control the message before a mismatched reason", func(t *testing.T) {
		t.Parallel()

		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			toolspkg.ToolIDConfigGet,
			"backend failed",
			toolspkg.ErrToolBackendFailed,
			toolspkg.ReasonConfigPathNotFound,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, false)

		if status != http.StatusBadGateway || payload.Error.Message != "tool backend failed" {
			t.Fatalf(
				"mismatched reason payload = %#v status=%d, want code-specific backend failure",
				payload.Error,
				status,
			)
		}
	})

	t.Run("Should sanitize cross workspace denial details", func(t *testing.T) {
		t.Parallel()

		const secret = "provider-token=workspace-secret"
		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			toolspkg.ToolIDWorkspaceInfo,
			"workspace info denied: "+secret,
			toolspkg.ErrToolDenied,
			toolspkg.ReasonWorkspaceAccessDenied,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)
		if status != http.StatusForbidden ||
			payload.Error.Message != "tool invocation denied" ||
			!slices.Contains(payload.Error.ReasonCodes, toolspkg.ReasonWorkspaceAccessDenied) {
			t.Fatalf("workspace denial payload = %#v, want canonical message and reason", payload.Error)
		}
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			t.Fatalf("json.Marshal() error = %v", encodeErr)
		}
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("workspace denial leaked internal detail: %s", encoded)
		}
	})

	t.Run("Should return HTTP 422 for reserved agent names", func(t *testing.T) {
		t.Parallel()

		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeAgentNameReserved,
			toolspkg.ToolIDAgentCreate,
			"agent name is reserved",
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonSchemaInvalid,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)

		if status != http.StatusUnprocessableEntity {
			t.Fatalf("tool error status = %d, want %d", status, http.StatusUnprocessableEntity)
		}
		if payload.Error.Code != toolspkg.ErrorCodeAgentNameReserved {
			t.Fatalf("tool error code = %q, want %q", payload.Error.Code, toolspkg.ErrorCodeAgentNameReserved)
		}
	})

	t.Run("Should return HTTP 507 with the safe partial result", func(t *testing.T) {
		t.Parallel()

		partial := toolspkg.ToolResult{
			Content: []toolspkg.ToolContent{{Type: "text", Text: "bounded preview"}},
		}
		err := toolspkg.NewToolError(
			toolspkg.ErrorCodeResultPersistenceFailed,
			toolspkg.ToolIDSkillView,
			"internal persistence detail compozy_claim_secret",
			toolspkg.ErrToolResultPersistence,
		).WithPartialResult(partial)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)

		if status != http.StatusInsufficientStorage {
			t.Fatalf("tool error status = %d, want %d", status, http.StatusInsufficientStorage)
		}
		if payload.Error.Code != toolspkg.ErrorCodeResultPersistenceFailed ||
			payload.Error.Message != "tool result could not be retained" ||
			payload.Error.PartialResult == nil ||
			len(payload.Error.PartialResult.Content) != 1 ||
			payload.Error.PartialResult.Content[0].Type != "text" ||
			payload.Error.PartialResult.Content[0].Text != "bounded preview" {
			t.Fatalf("tool partial error payload = %#v, want safe 507 partial result", payload.Error)
		}
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			t.Fatalf("json.Marshal() error = %v", encodeErr)
		}
		if strings.Contains(string(encoded), "compozy_claim_secret") {
			t.Fatalf("tool partial error leaked internal detail: %s", encoded)
		}
	})

	t.Run("Should expose only explicitly authored operator remediation", func(t *testing.T) {
		t.Parallel()

		err := toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeUnavailable,
			toolspkg.ToolIDExtensionsInstall,
			"internal dependency detail token=super-secret",
			toolspkg.ErrToolUnavailable,
			"Git is too old",
			"Install Git 2.37 or newer. Run `git --version`.",
			toolspkg.ReasonExtensionGitVersionUnsupported,
		)
		status := core.StatusForToolError(err)
		payload := core.ToolErrorResponseForError(err, status, true)

		if status != http.StatusUnprocessableEntity || payload.Error.Message != "tool unavailable" {
			t.Fatalf("operator failure status/payload = %d/%#v", status, payload.Error)
		}
		cause := string(payload.Error.Details[contract.ToolOperatorCauseDetailKey])
		recovery := string(payload.Error.Details[contract.ToolOperatorRecoveryDetailKey])
		if cause != `"Git is too old"` ||
			!strings.Contains(recovery, "Git 2.37 or newer") ||
			!strings.Contains(recovery, "git --version") {
			t.Fatalf("operator failure details = %#v, want authored remediation", payload.Error.Details)
		}
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			t.Fatalf("json.Marshal() error = %v", encodeErr)
		}
		if strings.Contains(string(encoded), "super-secret") {
			t.Fatalf("operator failure leaked internal detail: %s", encoded)
		}
	})
}

func TestToolApprovalHandlersMintAndConsumeSingleUseTokens(t *testing.T) {
	t.Parallel()

	t.Run("Should mint and consume single-use tokens", func(t *testing.T) {
		t.Parallel()

		approvals := toolspkg.NewApprovalTokenStore(
			time.Minute,
			toolspkg.WithApprovalTokenClock(func() time.Time {
				return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
			}),
		)
		registry := newAPITestToolRegistry(t, true, approvals)
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "api-core-test",
			Sessions:      testutil.StubSessionManager{},
			Observer:      testutil.StubObserver{},
			Tasks:         &testutil.StubTaskManager{},
			Workspaces:    testutil.StubWorkspaceService{},
			Tools:         registry,
			Toolsets:      registry,
			ToolApprovals: approvals,
			HomePaths:     homePaths,
			Config:        cfg,
			Logger:        testutil.DiscardLogger(),
			StartedAt:     time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			Now:           func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
			PollInterval:  time.Millisecond,
			StreamDone:    make(chan struct{}),
		})
		engine := newToolCoreEngine(t, handlers)

		missingTokenResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/ext__ask_tool/invoke",
			[]byte(`{"session_id":"sess-1","workspace_id":"ws-1","input":{"message":"hello"}}`),
		)
		if missingTokenResp.Code != http.StatusAccepted {
			t.Fatalf(
				"missing token status = %d, want %d; body=%s",
				missingTokenResp.Code,
				http.StatusAccepted,
				missingTokenResp.Body.String(),
			)
		}
		var missingToken contract.ToolErrorResponse
		decodeToolJSON(t, missingTokenResp.Body.Bytes(), &missingToken)
		if missingToken.Error.Code != toolspkg.ErrorCodeApprovalRequired ||
			!containsReason(missingToken.Error.ReasonCodes, toolspkg.ReasonApprovalTokenMissing) {
			t.Fatalf("missing token error = %#v, want approval token missing", missingToken.Error)
		}
		if registry.callCount("ext__ask_tool") != 0 {
			t.Fatal("approval-required tool executed without token")
		}

		conflictResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/ext__ask_tool/approvals?session_id=sess-query",
			[]byte(`{"session_id":"sess-body","workspace_id":"ws-1","input":{"message":"hello"}}`),
		)
		if conflictResp.Code != http.StatusBadRequest {
			t.Fatalf(
				"approval scope conflict status = %d, want %d; body=%s",
				conflictResp.Code,
				http.StatusBadRequest,
				conflictResp.Body.String(),
			)
		}
		var conflict contract.ToolErrorResponse
		decodeToolJSON(t, conflictResp.Body.Bytes(), &conflict)
		if !containsReason(conflict.Error.ReasonCodes, toolspkg.ReasonApprovalTokenMismatch) {
			t.Fatalf("approval scope conflict error = %#v, want token mismatch reason", conflict.Error)
		}

		approvalResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/ext__ask_tool/approvals",
			[]byte(`{"session_id":"sess-1","workspace_id":"ws-1","input":{"message":"hello"}}`),
		)
		if approvalResp.Code != http.StatusCreated {
			t.Fatalf(
				"approval status = %d, want %d; body=%s",
				approvalResp.Code,
				http.StatusCreated,
				approvalResp.Body.String(),
			)
		}
		var approval contract.ToolApprovalResponse
		decodeToolJSON(t, approvalResp.Body.Bytes(), &approval)
		if approval.Approval.ApprovalToken == "" || approval.Approval.InputDigest == "" {
			t.Fatalf("approval response = %#v, want token and digest", approval)
		}

		body := []byte(`{"session_id":"sess-1","workspace_id":"ws-1","approval_token":"` +
			approval.Approval.ApprovalToken + `","input":{"message":"hello"}}`)
		invokeResp := performRequest(t, engine, http.MethodPost, "/tools/ext__ask_tool/invoke", body)
		if invokeResp.Code != http.StatusOK {
			t.Fatalf("invoke status = %d, want %d; body=%s", invokeResp.Code, http.StatusOK, invokeResp.Body.String())
		}
		var invokePayload contract.ToolInvokeResponse
		decodeToolJSON(t, invokeResp.Body.Bytes(), &invokePayload)
		if invokePayload.ToolID != "ext__ask_tool" || invokePayload.Status != "completed" {
			t.Fatalf("invoke payload = %#v, want completed ext__ask_tool", invokePayload)
		}
		if registry.callCount("ext__ask_tool") != 1 {
			t.Fatalf("registry call count = %d, want 1", registry.callCount("ext__ask_tool"))
		}

		replayResp := performRequest(t, engine, http.MethodPost, "/tools/ext__ask_tool/invoke", body)
		if replayResp.Code != http.StatusForbidden {
			t.Fatalf(
				"replay status = %d, want %d; body=%s",
				replayResp.Code,
				http.StatusForbidden,
				replayResp.Body.String(),
			)
		}
		var replay contract.ToolErrorResponse
		decodeToolJSON(t, replayResp.Body.Bytes(), &replay)
		if !containsReason(replay.Error.ReasonCodes, toolspkg.ReasonApprovalTokenReplayed) {
			t.Fatalf("replay error = %#v, want replay reason", replay.Error)
		}
	})
}

func TestToolHandlersPropagateScopeDefaultsAndSanitizeErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should propagate query scope defaults into invoke requests", func(t *testing.T) {
		t.Parallel()

		registry := newAPITestToolRegistry(t, false)
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			Sessions:           testutil.StubSessionManager{},
			Observer:           testutil.StubObserver{},
			Tasks:              &testutil.StubTaskManager{},
			Workspaces:         testutil.StubWorkspaceService{},
			Tools:              registry,
			Toolsets:           registry,
			ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
			HomePaths:          homePaths,
			Config:             cfg,
			Logger:             testutil.DiscardLogger(),
			StartedAt:          time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			Now:                func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
			PollInterval:       time.Millisecond,
			StreamDone:         make(chan struct{}),
			MaskInternalErrors: false,
		})
		engine := newToolCoreEngine(t, handlers)

		resp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/compozy__skill_view/invoke?session_id=sess-query&workspace_id=ws-query&agent_name=coder",
			[]byte(`{"input":{"message":"hello"}}`),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("invoke status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var invokePayload contract.ToolInvokeResponse
		decodeToolJSON(t, resp.Body.Bytes(), &invokePayload)
		if invokePayload.ToolID != toolspkg.ToolIDSkillView || invokePayload.Status != "completed" {
			t.Fatalf("invoke payload = %#v, want completed skill_view", invokePayload)
		}
		scope, call := registry.lastCall()
		if scope.SessionID != "sess-query" || scope.WorkspaceID != "ws-query" || scope.AgentName != "coder" {
			t.Fatalf("call scope = %#v, want query defaults", scope)
		}
		if call.SessionID != "sess-query" || call.WorkspaceID != "ws-query" || call.AgentName != "coder" {
			t.Fatalf("call request = %#v, want normalized scope values", call)
		}
	})

	t.Run("Should return safe tool error messages", func(t *testing.T) {
		t.Parallel()

		registry := newAPITestToolRegistry(t, true)
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			Sessions:           testutil.StubSessionManager{},
			Observer:           testutil.StubObserver{},
			Tasks:              &testutil.StubTaskManager{},
			Workspaces:         testutil.StubWorkspaceService{},
			Tools:              registry,
			Toolsets:           registry,
			ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
			HomePaths:          homePaths,
			Config:             cfg,
			Logger:             testutil.DiscardLogger(),
			StartedAt:          time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			Now:                func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
			PollInterval:       time.Millisecond,
			StreamDone:         make(chan struct{}),
			MaskInternalErrors: false,
		})
		engine := newToolCoreEngine(t, handlers)

		resp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/tools/ext__ask_tool/invoke",
			[]byte(`{"session_id":"sess-1","workspace_id":"ws-1","input":{"token":"compozy_claim_secret"}}`),
		)
		if resp.Code != http.StatusAccepted {
			t.Fatalf("invoke status = %d, want %d; body=%s", resp.Code, http.StatusAccepted, resp.Body.String())
		}
		var payload contract.ToolErrorResponse
		decodeToolJSON(t, resp.Body.Bytes(), &payload)
		if payload.Error.Message != "tool approval required" {
			t.Fatalf("tool error message = %q, want safe approval message", payload.Error.Message)
		}
		if strings.Contains(resp.Body.String(), "compozy_claim_secret") ||
			strings.Contains(resp.Body.String(), "tool approval token is required") {
			t.Fatalf("tool error leaked raw backend detail: %s", resp.Body.String())
		}
	})

	t.Run("Should expose provider model input errors as safe 422 payloads", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name    string
			code    toolspkg.ErrorCode
			message string
		}{
			{
				name:    "Should expose model_not_found",
				code:    toolspkg.ErrorCodeModelNotFound,
				message: "provider model not found",
			},
			{
				name:    "Should expose reasoning_effort_unsupported",
				code:    toolspkg.ErrorCodeReasoningEffortUnsupported,
				message: "reasoning effort unsupported",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				registry := newAPITestToolRegistry(t, false)
				registry.setCallError(
					toolspkg.ToolIDSkillView,
					toolspkg.NewToolError(
						tc.code,
						toolspkg.ToolIDSkillView,
						"internal provider model detail compozy_claim_secret",
						toolspkg.ErrToolInvalidInput,
					),
				)
				homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
				handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
					TransportName:      "api-core-test",
					Sessions:           testutil.StubSessionManager{},
					Observer:           testutil.StubObserver{},
					Tasks:              &testutil.StubTaskManager{},
					Workspaces:         testutil.StubWorkspaceService{},
					Tools:              registry,
					Toolsets:           registry,
					ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
					HomePaths:          homePaths,
					Config:             cfg,
					Logger:             testutil.DiscardLogger(),
					StartedAt:          time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
					Now:                func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
					PollInterval:       time.Millisecond,
					StreamDone:         make(chan struct{}),
					MaskInternalErrors: false,
				})
				resp := performRequest(
					t,
					newToolCoreEngine(t, handlers),
					http.MethodPost,
					"/tools/compozy__skill_view/invoke",
					[]byte(`{"input":{"name":"test"}}`),
				)
				if got, want := resp.Code, http.StatusUnprocessableEntity; got != want {
					t.Fatalf("invoke status = %d, want %d; body=%s", got, want, resp.Body.String())
				}
				var payload contract.ToolErrorResponse
				decodeToolJSON(t, resp.Body.Bytes(), &payload)
				if payload.Error.Code != tc.code || payload.Error.Message != tc.message {
					t.Fatalf("tool error payload = %#v, want %s/%q", payload, tc.code, tc.message)
				}
				if strings.Contains(resp.Body.String(), "compozy_claim_secret") {
					t.Fatalf("tool error leaked raw provider model detail: %s", resp.Body.String())
				}
			})
		}
	})
}

func TestSessionToolHandlersUseResolvedRouteScope(t *testing.T) {
	t.Parallel()

	t.Run("Should use resolved workspace identity for session-scoped tool projections", func(t *testing.T) {
		t.Parallel()

		registry := newAPITestToolRegistry(t, false)
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		workspaces := defaultCoreWorkspaceService(testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				switch ref {
				case "ws-alias", "ws-stable":
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      "ws-registry",
							RootDir: "/workspace",
							Name:    "alias",
						},
						WorkspaceID: "ws-stable",
					}, nil
				default:
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
			},
		})
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			Sessions:           networkTestSessionManager("ws-registry", "sess-1"),
			Observer:           testutil.StubObserver{},
			Tasks:              &testutil.StubTaskManager{},
			Workspaces:         workspaces,
			Tools:              registry,
			Toolsets:           registry,
			ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
			HomePaths:          homePaths,
			Config:             cfg,
			Logger:             testutil.DiscardLogger(),
			StartedAt:          time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			Now:                func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
			PollInterval:       time.Millisecond,
			StreamDone:         make(chan struct{}),
			MaskInternalErrors: false,
		})
		engine := newToolCoreEngine(t, handlers)

		listResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-alias/sessions/sess-1/tools", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("session list status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
		}
		listScope := registry.lastList()
		if listScope.WorkspaceID != "ws-stable" || listScope.SessionID != "sess-1" {
			t.Fatalf("list scope = %#v, want resolved stable workspace and route session", listScope)
		}

		searchResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-alias/sessions/sess-1/tools/search",
			[]byte(`{"query":"skill","limit":1,"workspace_id":"ws-stable","session_id":"sess-1"}`),
		)
		if searchResp.Code != http.StatusOK {
			t.Fatalf(
				"session search status = %d, want %d; body=%s",
				searchResp.Code,
				http.StatusOK,
				searchResp.Body.String(),
			)
		}
		searchScope, _ := registry.lastSearch()
		if searchScope.WorkspaceID != "ws-stable" || searchScope.SessionID != "sess-1" {
			t.Fatalf("search scope = %#v, want resolved stable workspace and route session", searchScope)
		}

		mismatchResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-alias/sessions/sess-1/tools/search",
			[]byte(`{"query":"skill","workspace_id":"ws-alias","session_id":"sess-1"}`),
		)
		if mismatchResp.Code != http.StatusBadRequest {
			t.Fatalf(
				"mismatch status = %d, want %d; body=%s",
				mismatchResp.Code,
				http.StatusBadRequest,
				mismatchResp.Body.String(),
			)
		}
		if !strings.Contains(mismatchResp.Body.String(), "workspace_id does not match path") {
			t.Fatalf("mismatch body = %s, want workspace mismatch error", mismatchResp.Body.String())
		}
	})

	t.Run("Should reject empty session route identifiers deterministically", func(t *testing.T) {
		t.Parallel()

		registry := newAPITestToolRegistry(t, false)
		homePaths, cfg := testutil.NewDisabledNetworkHomeConfig(t)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			Sessions:           networkTestSessionManager("ws-workspace", "sess-1"),
			Observer:           testutil.StubObserver{},
			Tasks:              &testutil.StubTaskManager{},
			Workspaces:         defaultCoreWorkspaceService(testutil.StubWorkspaceService{}),
			Tools:              registry,
			Toolsets:           registry,
			ToolApprovals:      toolspkg.NewApprovalTokenStore(time.Minute),
			HomePaths:          homePaths,
			Config:             cfg,
			Logger:             testutil.DiscardLogger(),
			StartedAt:          time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			Now:                func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
			PollInterval:       time.Millisecond,
			StreamDone:         make(chan struct{}),
			MaskInternalErrors: false,
		})
		engine := newToolCoreEngine(t, handlers)

		resp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-workspace/sessions/%20/tools", nil)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"empty session status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}
		if !strings.Contains(resp.Body.String(), "session_id path is required") {
			t.Fatalf("empty session body = %s, want session_id path error", resp.Body.String())
		}
	})
}

func newToolCoreEngine(t *testing.T, handlers *core.BaseHandlers) *gin.Engine {
	t.Helper()
	engine := gin.New()
	engine.GET("/tools", handlers.ListTools)
	engine.POST("/tools/search", handlers.SearchTools)
	engine.GET("/tools/:id", handlers.GetTool)
	engine.POST("/tools/:id/approvals", handlers.CreateToolApproval)
	engine.POST("/tools/:id/invoke", handlers.InvokeTool)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/tools", handlers.ListSessionTools)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/tools/search", handlers.SearchSessionTools)
	engine.GET("/workspaces/:workspace_id/tool-artifacts/:artifact_id", handlers.ReadToolArtifact)
	engine.GET("/toolsets", handlers.ListToolsets)
	engine.GET("/toolsets/:id", handlers.GetToolset)
	return engine
}

type apiTestToolRegistry struct {
	registry             *toolspkg.RuntimeRegistry
	mu                   sync.Mutex
	calls                map[toolspkg.ToolID]int
	callErrors           map[toolspkg.ToolID]error
	lastListScope        toolspkg.Scope
	lastCallScope        toolspkg.Scope
	lastCallRequest      toolspkg.CallRequest
	lastSearchScope      toolspkg.Scope
	lastSearchQuery      toolspkg.SearchQuery
	lastToolset          toolspkg.ToolsetID
	lastGetScope         toolspkg.Scope
	lastToolsetListScope toolspkg.Scope
	lastToolsetGetScope  toolspkg.Scope
	listProfileID        string
	listWorkspaceID      string
}

func newAPITestToolRegistry(
	t *testing.T,
	approvalRequired bool,
	approvalConsumers ...toolspkg.ApprovalTokenConsumer,
) *apiTestToolRegistry {
	t.Helper()
	ids := []toolspkg.ToolID{toolspkg.ToolIDSkillView}
	source := toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: "compozy"}
	descriptors := []toolspkg.Descriptor{
		testToolDescriptor(toolspkg.ToolIDSkillView, source, toolspkg.VisibilityModel),
		testToolDescriptor("compozy__operator_diag", source, toolspkg.VisibilityOperator),
	}
	inputs := toolspkg.DefaultPolicyInputs()
	if approvalRequired {
		source = toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: "ext"}
		descriptors = []toolspkg.Descriptor{
			testToolDescriptor("ext__ask_tool", source, toolspkg.VisibilityModel),
		}
		ids = []toolspkg.ToolID{"ext__ask_tool"}
		inputs.ExternalDefault = toolspkg.ExternalDefaultAsk
		inputs.ApprovalAvailable = true
	}
	catalog, err := toolspkg.NewToolsetCatalog(toolspkg.Toolset{
		ID:    "compozy__catalog",
		Tools: []string{string(ids[0])},
	})
	if err != nil {
		t.Fatalf("NewToolsetCatalog() error = %v", err)
	}
	wrapper := &apiTestToolRegistry{
		calls:      make(map[toolspkg.ToolID]int),
		callErrors: make(map[toolspkg.ToolID]error),
	}
	provider := &apiTestToolProvider{
		source:  source,
		handles: make(map[toolspkg.ToolID]*apiTestToolHandle),
	}
	for _, descriptor := range descriptors {
		provider.handles[descriptor.ID] = &apiTestToolHandle{
			descriptor: descriptor,
			call: func(_ context.Context, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
				wrapper.mu.Lock()
				wrapper.calls[req.ToolID]++
				callErr := wrapper.callErrors[req.ToolID]
				wrapper.mu.Unlock()
				if callErr != nil {
					return toolspkg.ToolResult{}, callErr
				}
				return toolspkg.ToolResult{
					Content:    []toolspkg.ToolContent{{Type: "text", Text: "ok"}},
					Structured: json.RawMessage(`{"ok":true}`),
					DurationMS: 12,
				}, nil
			},
		}
	}
	options := []toolspkg.RegistryOption{
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(inputs, catalog),
	}
	if approvalRequired {
		var consumer toolspkg.ApprovalTokenConsumer
		if len(approvalConsumers) > 0 {
			consumer = approvalConsumers[0]
		}
		options = append(options, toolspkg.WithApprovalBridge(apiTestApprovalBridge{approvals: consumer}))
	}
	registry, err := toolspkg.NewRegistry(options...)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	wrapper.registry = registry
	return wrapper
}

func (r *apiTestToolRegistry) List(ctx context.Context, scope toolspkg.Scope) ([]toolspkg.ToolView, error) {
	r.mu.Lock()
	r.lastListScope = scope
	requiredProfileID := r.listProfileID
	requiredWorkspaceID := r.listWorkspaceID
	r.mu.Unlock()
	if requiredProfileID != "" && (scope.ProfileID != requiredProfileID || scope.WorkspaceID != requiredWorkspaceID) {
		return []toolspkg.ToolView{}, nil
	}
	return r.registry.List(ctx, scope)
}

func (r *apiTestToolRegistry) restrictListTo(profileID string, workspaceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listProfileID = profileID
	r.listWorkspaceID = workspaceID
}

func (r *apiTestToolRegistry) Search(
	ctx context.Context,
	scope toolspkg.Scope,
	q toolspkg.SearchQuery,
) ([]toolspkg.ToolView, error) {
	r.mu.Lock()
	r.lastSearchScope = scope
	r.lastSearchQuery = q
	r.mu.Unlock()
	return r.registry.Search(ctx, scope, q)
}

func (r *apiTestToolRegistry) Get(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	r.mu.Lock()
	r.lastGetScope = scope
	r.mu.Unlock()
	return r.registry.Get(ctx, scope, id)
}

func (r *apiTestToolRegistry) Call(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	r.mu.Lock()
	r.lastCallScope = scope
	r.lastCallRequest = req
	r.mu.Unlock()
	return r.registry.Call(ctx, scope, req)
}

func (r *apiTestToolRegistry) ListToolsets(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.ToolsetView, error) {
	r.mu.Lock()
	r.lastToolsetListScope = scope
	r.mu.Unlock()
	return r.registry.ListToolsets(ctx, scope)
}

func (r *apiTestToolRegistry) GetToolset(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolsetID,
) (toolspkg.ToolsetView, error) {
	r.mu.Lock()
	r.lastToolset = id
	r.lastToolsetGetScope = scope
	r.mu.Unlock()
	return r.registry.GetToolset(ctx, scope, id)
}

func (r *apiTestToolRegistry) callCount(id toolspkg.ToolID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[id]
}

func (r *apiTestToolRegistry) setCallError(id toolspkg.ToolID, err error) {
	r.mu.Lock()
	r.callErrors[id] = err
	r.mu.Unlock()
}

func (r *apiTestToolRegistry) lastCall() (toolspkg.Scope, toolspkg.CallRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastCallScope, r.lastCallRequest
}

func (r *apiTestToolRegistry) lastList() toolspkg.Scope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastListScope
}

func (r *apiTestToolRegistry) lastSearch() (toolspkg.Scope, toolspkg.SearchQuery) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastSearchScope, r.lastSearchQuery
}

func (r *apiTestToolRegistry) lastGet() toolspkg.Scope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastGetScope
}

func (r *apiTestToolRegistry) lastToolsetList() toolspkg.Scope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastToolsetListScope
}

func (r *apiTestToolRegistry) lastToolsetGet() toolspkg.Scope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastToolsetGetScope
}

func (r *apiTestToolRegistry) lastToolsetID() toolspkg.ToolsetID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastToolset
}

type apiTestToolProvider struct {
	source  toolspkg.SourceRef
	handles map[toolspkg.ToolID]*apiTestToolHandle
}

func (p *apiTestToolProvider) ID() toolspkg.SourceRef {
	return p.source
}

func (p *apiTestToolProvider) List(context.Context, toolspkg.Scope) ([]toolspkg.Descriptor, error) {
	descriptors := make([]toolspkg.Descriptor, 0, len(p.handles))
	for _, handle := range p.handles {
		descriptors = append(descriptors, handle.Descriptor())
	}
	return descriptors, nil
}

func (p *apiTestToolProvider) Resolve(
	_ context.Context,
	_ toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	handle, ok := p.handles[id]
	return handle, ok, nil
}

type apiTestToolHandle struct {
	descriptor toolspkg.Descriptor
	call       func(context.Context, toolspkg.CallRequest) (toolspkg.ToolResult, error)
}

func (h *apiTestToolHandle) Descriptor() toolspkg.Descriptor {
	return h.descriptor
}

func (h *apiTestToolHandle) Availability(context.Context, toolspkg.Scope) toolspkg.Availability {
	return toolspkg.Availability{
		Registered: true,
		Enabled:    true,
		Available:  true,
		Authorized: true,
		Executable: true,
	}
}

func (h *apiTestToolHandle) Call(ctx context.Context, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	if h.call == nil {
		return toolspkg.ToolResult{}, errors.New("test handle call not configured")
	}
	return h.call(ctx, req)
}

type apiTestApprovalBridge struct {
	approvals toolspkg.ApprovalTokenConsumer
}

func (b apiTestApprovalBridge) RequestToolApproval(
	ctx context.Context,
	scope toolspkg.Scope,
	call *toolspkg.CallRequest,
	_ *toolspkg.ToolView,
) error {
	if call == nil {
		return toolspkg.ErrToolApprovalRequired
	}
	if b.approvals == nil {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeApprovalRequired,
			call.ToolID,
			"tool approval token is required",
			toolspkg.ErrToolApprovalRequired,
			toolspkg.ReasonApprovalRequired,
			toolspkg.ReasonApprovalTokenMissing,
		)
	}
	return b.approvals.ConsumeToolApproval(ctx, scope, *call)
}

func testToolDescriptor(
	id toolspkg.ToolID,
	source toolspkg.SourceRef,
	visibility toolspkg.Visibility,
) toolspkg.Descriptor {
	return toolspkg.Descriptor{
		ID:               id,
		Backend:          toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: id.String()},
		ToolPresentation: toolspkg.NewToolPresentation(id.String(), "", ""),
		Description:      "Test tool " + id.String(),
		InputSchema:      json.RawMessage(`{"type":"object"}`),
		Source:           source,
		Visibility:       visibility,
		Risk:             toolspkg.RiskRead,
		ReadOnly:         true,
		Toolsets:         []toolspkg.ToolsetID{"compozy__catalog"},
		Tags:             []string{"skill", "test"},
	}
}

func decodeToolJSON(t *testing.T, data []byte, dest any) {
	t.Helper()
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", data, err)
	}
}

func containsReason(reasons []toolspkg.ReasonCode, want toolspkg.ReasonCode) bool {
	return slices.Contains(reasons, want)
}
