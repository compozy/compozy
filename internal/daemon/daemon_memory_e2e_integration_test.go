//go:build integration && !windows

package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	toolspkg "github.com/compozy/agh/internal/tools"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	_ "modernc.org/sqlite"
)

const (
	roleDreamProviderName = "role-dream"
	roleDreamModel        = "routed-dream-model"
)

func TestDaemonE2ERolesLiveApplyChangesNextMemoryExtractorModel(t *testing.T) {
	t.Parallel()

	t.Run("Should record the HTTP apply and route the next extractor through the applied model", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *aghconfig.Config) {
				cfg.Roles.AutoTitle.Enabled = false
				cfg.Roles.MemoryExtractor.Provider = acpmock.ProviderName
				cfg.Roles.MemoryExtractor.Model = "extractor-model-v1"
			}},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "agent_roles_fixture.json"),
				FixtureAgent: "role-agent",
				AgentName:    "memory-extractor-live-apply",
			}},
		})
		registration, ok := harness.MockAgentRegistration("memory-extractor-live-apply")
		if !ok {
			t.Fatal("MockAgentRegistration(memory-extractor-live-apply) = missing, want present")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var roles aghcontract.SettingsRolesResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/settings/roles", nil, &roles); err != nil {
			t.Fatalf("HTTP GET settings roles error = %v", err)
		}
		roles.Config.MemoryExtractor.Model = "extractor-model-v2"
		var apply aghcontract.SettingsApplyResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPatch,
			"/api/settings/roles",
			aghcontract.UpdateSettingsRolesRequest{Config: roles.Config},
			&apply,
		); err != nil {
			t.Fatalf("HTTP PATCH settings roles error = %v", err)
		}
		if !apply.Applied || apply.Lifecycle != aghcontract.SettingsApplyLifecycleLive || apply.ApplyRecordID == "" {
			t.Fatalf("roles live apply = %#v, want applied live record", apply)
		}

		var history aghcontract.ConfigApplyRecordsResponse
		if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/settings/apply", nil, &history); err != nil {
			t.Fatalf("HTTP GET settings apply history error = %v", err)
		}
		foundApply := false
		for _, entry := range history.Entries {
			if entry.ID == apply.ApplyRecordID && entry.Actor == "httpapi" &&
				entry.Lifecycle == aghcontract.SettingsApplyLifecycleLive &&
				entry.Status == aghcontract.ConfigApplyStatusApplied {
				foundApply = true
				break
			}
		}
		if !foundApply {
			t.Fatalf("settings apply history = %#v, want live HTTP record %q", history.Entries, apply.ApplyRecordID)
		}

		root := createFixtureBackedSession(t, ctx, harness, "memory-extractor-live-apply", "")
		if _, err := harness.PromptSession(ctx, root.ID, "Record the extractor routing decision"); err != nil {
			t.Fatalf("PromptSession(memory extractor live apply) error = %v", err)
		}

		var childID string
		waitForRuntimeCondition(t, "memory extractor uses live-applied model", 10*time.Second, func() bool {
			for _, candidate := range readWorkspaceRoleSessions(t, ctx, harness) {
				if candidate.Lineage != nil && candidate.Lineage.SpawnRole == sessionpkg.SpawnRoleMemoryExtractor {
					childID = candidate.ID
				}
			}
			if childID == "" {
				return false
			}
			records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
			if err != nil {
				return false
			}
			for _, record := range acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForAGHSession(records, childID)) {
				if record.ProtocolMethod == acpsdk.AgentMethodSessionSetConfigOption &&
					record.ConfigOptionID == "model" && record.ConfigOptionValue == "extractor-model-v2" {
					return true
				}
			}
			return false
		})
	})
}

func TestDaemonE2EMemoryCatalogCLIHTTPParityAndLegacyPathIsolation(t *testing.T) {
	t.Parallel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	legacyPath := filepath.Join(harness.WorkspaceRoot, ".compozy", "memory", "legacy-only.md")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(legacyPath), err)
	}
	if err := os.WriteFile(
		legacyPath,
		[]byte(
			memoryDocument("Legacy Decoy", "Legacy path should stay ignored", memcontract.TypeProject, "legacy decoy"),
		),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", legacyPath, err)
	}

	writeMemoryViaCLI(
		t,
		ctx,
		harness,
		"",
		"prefs.md",
		memcontract.TypeUser,
		"User prefers concise answers",
		"Prefer concise answers with direct technical detail.",
		memcontract.ScopeGlobal,
	)
	writeMemoryViaCLI(
		t,
		ctx,
		harness,
		harness.WorkspaceRoot,
		"auth.md",
		memcontract.TypeProject,
		"Auth migration details",
		"Remember me: auth migration uses sessions and workspace-scoped recall.",
		memcontract.ScopeWorkspace,
	)
	writeMemoryViaCLI(
		t,
		ctx,
		harness,
		harness.WorkspaceRoot,
		"release-plan.md",
		memcontract.TypeProject,
		"Release plan details",
		"Release plan covers durable release checklist and observability ownership.",
		memcontract.ScopeWorkspace,
	)

	t.Run("Should return matching CLI and HTTP search results while ignoring legacy paths", func(t *testing.T) {
		var cliSearch aghcontract.MemorySearchResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliSearch,
			"memory",
			"search",
			"auth sessions",
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI memory search error = %v", err)
		}
		if !containsSearchResult(cliSearch, "project_auth.md", memcontract.ScopeWorkspace) {
			t.Fatalf("CLI search results = %#v, want workspace project_auth.md hit", cliSearch)
		}
		if containsSearchResult(cliSearch, "legacy-only.md", memcontract.ScopeWorkspace) {
			t.Fatalf("CLI search results = %#v, want legacy path ignored", cliSearch)
		}

		var httpSearch aghcontract.MemorySearchResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPost,
			"/api/memory/search",
			memorySearchRequest("auth sessions", memcontract.ScopeWorkspace, harness.WorkspaceRoot),
			&httpSearch,
		); err != nil {
			t.Fatalf("HTTP memory search error = %v", err)
		}
		if !containsSearchResult(httpSearch, "project_auth.md", memcontract.ScopeWorkspace) {
			t.Fatalf("HTTP search results = %#v, want workspace project_auth.md hit", httpSearch)
		}
		if len(cliSearch.Results) == 0 || len(httpSearch.Results) == 0 {
			t.Fatalf("search results = cli:%#v http:%#v, want non-empty parity", cliSearch, httpSearch)
		}
		if got, want := httpSearch.Results[0].Memory.Filename, cliSearch.Results[0].Memory.Filename; got != want {
			t.Fatalf("HTTP top search filename = %q, want %q", got, want)
		}
		if got, want := httpSearch.Results[0].Memory.Scope, cliSearch.Results[0].Memory.Scope; got != want {
			t.Fatalf("HTTP top search scope = %q, want %q", got, want)
		}

		var cliLegacy aghcontract.MemorySearchResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliLegacy,
			"memory",
			"search",
			"legacy decoy",
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI legacy search error = %v", err)
		}
		if len(cliLegacy.Results) != 0 {
			t.Fatalf("CLI legacy search results = %#v, want empty result set", cliLegacy)
		}

		var httpLegacy aghcontract.MemorySearchResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPost,
			"/api/memory/search",
			memorySearchRequest("legacy decoy", memcontract.ScopeWorkspace, harness.WorkspaceRoot),
			&httpLegacy,
		); err != nil {
			t.Fatalf("HTTP legacy search error = %v", err)
		}
		if len(httpLegacy.Results) != 0 {
			t.Fatalf("HTTP legacy search results = %#v, want empty result set", httpLegacy)
		}
	})

	t.Run("Should reindex through CLI and HTTP with matching counts", func(t *testing.T) {
		var cliReindex memcontract.ReindexResult
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliReindex,
			"memory",
			"reindex",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI memory reindex error = %v", err)
		}
		if got, want := cliReindex.IndexedFiles, 3; got != want {
			t.Fatalf("CLI reindex indexed_files = %d, want %d", got, want)
		}

		var httpReindex memcontract.ReindexResult
		if err := harness.HTTPJSON(
			ctx,
			http.MethodPost,
			"/api/memory/reindex",
			aghcontract.MemoryReindexV2Request{WorkspaceID: harness.WorkspaceRoot},
			&httpReindex,
		); err != nil {
			t.Fatalf("HTTP memory reindex error = %v", err)
		}
		if got, want := httpReindex.IndexedFiles, 3; got != want {
			t.Fatalf("HTTP reindex indexed_files = %d, want %d", got, want)
		}
	})

	t.Run("Should surface memory health and logs", func(t *testing.T) {
		var health aghcontract.StatusPayload
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			"/api/status?workspace="+url.QueryEscape(harness.WorkspaceRoot),
			nil,
			&health,
		); err != nil {
			t.Fatalf("HTTP status error = %v", err)
		}
		if got, want := health.Memory.GlobalFiles, 1; got != want {
			t.Fatalf("health.Memory.GlobalFiles = %d, want %d", got, want)
		}
		if got, want := health.Memory.WorkspaceFiles, 2; got != want {
			t.Fatalf("health.Memory.WorkspaceFiles = %d, want %d", got, want)
		}
		if got, want := health.Memory.IndexedFiles, 3; got != want {
			t.Fatalf("health.Memory.IndexedFiles = %d, want %d", got, want)
		}
		if got := health.Memory.OrphanedFiles; got != 0 {
			t.Fatalf("health.Memory.OrphanedFiles = %d, want 0", got)
		}
		if health.Memory.LastReindex == nil {
			t.Fatalf("health.Memory.LastReindex = nil, want non-nil")
		}

		var reindexEvents aghcontract.LogsListResponse
		reindexEventsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
			"&type=memory.write.reindex&limit=10"
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			reindexEventsPath,
			nil,
			&reindexEvents,
		); err != nil {
			t.Fatalf("UDS logs reindex error = %v", err)
		}
		if !containsLogEventSummary(reindexEvents.Events, "memory.write.reindex", "indexed=3") {
			t.Fatalf("reindex logs = %#v, want indexed=3 summary", reindexEvents.Events)
		}

		var searchEvents aghcontract.LogsListResponse
		searchEventsPath := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
			"&type=memory.recall.executed&limit=10"
		if err := harness.UDSJSON(
			ctx,
			http.MethodGet,
			searchEventsPath,
			nil,
			&searchEvents,
		); err != nil {
			t.Fatalf("UDS logs search error = %v", err)
		}
		if !containsLogEventSummary(searchEvents.Events, "memory.recall.executed", `auth sessions`) {
			t.Fatalf("search logs = %#v, want auth search summary", searchEvents.Events)
		}
		if !containsLogEventSummary(searchEvents.Events, "memory.recall.executed", `legacy decoy`) {
			t.Fatalf("search logs = %#v, want legacy search summary", searchEvents.Events)
		}
	})
}

func TestDaemonE2EAgentMemoryBatchIsRecalledByNextSession(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:  mockFixturePath(t, "hosted_native_tools_fixture.json"),
				FixtureAgent: "hosted-native",
				AgentName:    "memory-batch-writer",
			},
			{
				FixturePath:  mockFixturePath(t, "memory_recall_fixture.json"),
				FixtureAgent: "memory-recall-agent",
				AgentName:    "memory-batch-reader",
			},
		},
	})

	writerRegistration, ok := harness.MockAgentRegistration("memory-batch-writer")
	if !ok {
		t.Fatal("MockAgentRegistration(memory-batch-writer) = missing, want present")
	}
	readerRegistration, ok := harness.MockAgentRegistration("memory-batch-reader")
	if !ok {
		t.Fatal("MockAgentRegistration(memory-batch-reader) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createFixtureBackedSession(t, ctx, harness, "memory-batch-writer", "memory-batch-writer-session")
	writerDiagnostics, err := acpmock.ReadDiagnostics(writerRegistration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(memory-batch-writer) error = %v", err)
	}
	hostedServer := requireHostedMCPStdioServer(t, writerDiagnostics)
	client := startHostedMCPClient(t, hostedServer)
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("Close(memory batch hosted MCP client) error = %v", closeErr)
		}
	}()

	var init sdkmcp.InitializeRequest
	init.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = sdkmcp.Implementation{Name: "agh-memory-batch-e2e", Version: "1.0.0"}
	if _, err := client.Initialize(ctx, init); err != nil {
		t.Fatalf("Initialize(memory batch hosted MCP client) error = %v", err)
	}
	var call sdkmcp.CallToolRequest
	call.Params.Name = toolspkg.ToolIDMemoryPropose.String()
	call.Params.Arguments = map[string]any{
		"scope":       "workspace",
		"workspace":   harness.WorkspaceID,
		"filename":    "project_atomic_batch.md",
		"name":        "Atomic Batch Recall",
		"description": "Fact committed through an agent-hosted native memory batch",
		"type":        "project",
		"operations": []map[string]any{
			{
				"action":  "add",
				"content": "Remember me: the atomic batch release codename is cobalt.",
			},
			{
				"action":   "replace",
				"old_text": "codename is cobalt",
				"content":  "codename is cobalt-blue",
			},
		},
	}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", call.Params.Name, err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool(%s) result = %#v, want successful batch", call.Params.Name, result)
	}
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(memory batch structuredContent) error = %v", err)
	}
	if !strings.Contains(string(structured), `"applied":true`) ||
		!strings.Contains(string(structured), `"operations"`) {
		t.Fatalf("memory batch structuredContent = %s, want applied operations", structured)
	}

	readerSession := createFixtureBackedSession(
		t,
		ctx,
		harness,
		"memory-batch-reader",
		"memory-batch-reader-session",
	)
	if _, err := harness.PromptSession(ctx, readerSession.ID, "remember the cobalt-blue atomic batch"); err != nil {
		t.Fatalf("PromptSession(memory batch reader) error = %v", err)
	}
	readerDiagnostics, err := acpmock.ReadDiagnostics(readerRegistration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(memory-batch-reader) error = %v", err)
	}
	prompts := acpmock.PromptDiagnostics(readerDiagnostics)
	if len(prompts) != 1 {
		t.Fatalf("len(memory batch reader prompts) = %d, want 1; diagnostics=%#v", len(prompts), readerDiagnostics)
	}
	if !strings.Contains(prompts[0].Prompt, "atomic batch release codename is cobalt-blue") ||
		!strings.Contains(prompts[0].Prompt, "Relevant durable memory for this turn:") {
		t.Fatalf("memory batch reader prompt = %q, want next-session recalled batch fact", prompts[0].Prompt)
	}

	if err := harness.CaptureMockAgentDiagnostics(writerRegistration); err != nil {
		t.Fatalf("CaptureMockAgentDiagnostics(writer) error = %v", err)
	}
	if err := harness.CaptureMockAgentDiagnostics(readerRegistration); err != nil {
		t.Fatalf("CaptureMockAgentDiagnostics(reader) error = %v", err)
	}
}

func TestDaemonE2EMemoryRecallUsesCatalogSynthesisWithoutMutatingStoredUserMessage(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "memory_recall_fixture.json"),
			FixtureAgent: "memory-recall-agent",
			AgentName:    "memory-recall-agent",
		}},
	})

	registration, ok := harness.MockAgentRegistration("memory-recall-agent")
	if !ok {
		t.Fatal("MockAgentRegistration(memory-recall-agent) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	writeMemoryViaUDS(
		t,
		ctx,
		harness,
		"auth.md",
		harness.WorkspaceRoot,
		"Remember me: auth migration uses sessions and workspace-scoped recall.",
	)

	indexPath := filepath.Join(harness.WorkspaceRoot, ".agh", "memory", "MEMORY.md")
	staleIndex := "- [Old](missing.md) - stale index entry\n"
	if err := os.WriteFile(indexPath, []byte(staleIndex), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", indexPath, err)
	}

	session := createFixtureBackedSession(t, ctx, harness, "memory-recall-agent", "memory-recall-session")
	if _, err := harness.PromptSession(ctx, session.ID, "remember me"); err != nil {
		t.Fatalf("PromptSession() error = %v", err)
	}

	eventsResp, err := harness.SessionEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	events := decodeAgentEvents(t, eventsResp.Events)
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}

	t.Run("Should persist the original user message without injected recall", func(t *testing.T) {
		if !containsAgentEvent(events, aghcontract.AgentEventPayload{
			Type: "agent_message",
			Text: "qa-memory acknowledged",
		}) {
			t.Fatalf("session events = %#v, want recall fixture assistant response", events)
		}
		userEvent, ok := firstAgentEventByType(events, "user_message")
		if !ok {
			t.Fatalf("session events = %#v, want stored user_message event", events)
		}
		if got, want := userEvent.Text, "remember me"; got != want {
			t.Fatalf("stored user_message text = %q, want %q", got, want)
		}
		if strings.Contains(userEvent.Text, "Relevant durable memory for this turn:") {
			t.Fatalf("stored user_message text = %q, want no injected recall block", userEvent.Text)
		}
	})

	t.Run("Should dispatch recalled context to the agent prompt", func(t *testing.T) {
		promptDiagnostics := acpmock.PromptDiagnostics(diagnostics)
		if got, want := len(promptDiagnostics), 1; got != want {
			t.Fatalf("len(promptDiagnostics) = %d, want %d; diagnostics=%#v", got, want, diagnostics)
		}
		prompt := promptDiagnostics[0].Prompt
		if !strings.Contains(prompt, "Relevant durable memory for this turn:") {
			t.Fatalf("mock prompt = %q, want recall preamble", prompt)
		}
		if !strings.Contains(prompt, "- auth [workspace]") {
			t.Fatalf("mock prompt = %q, want workspace recall heading", prompt)
		}
		if !strings.Contains(prompt, "auth migration uses sessions") {
			t.Fatalf("mock prompt = %q, want recalled auth snippet", prompt)
		}
		if !strings.Contains(prompt, "</turn-recall>\n\n<user-message>\nremember me\n</user-message>") {
			t.Fatalf("mock prompt = %q, want raw user message suffix", prompt)
		}
		if strings.Contains(prompt, "User message:") {
			t.Fatalf("mock prompt = %q, want no legacy user message marker", prompt)
		}
		if strings.Contains(prompt, "missing.md") {
			t.Fatalf("mock prompt = %q, want stale MEMORY.md entry ignored", prompt)
		}
	})

	t.Run("Should leave the stale index file unchanged on disk", func(t *testing.T) {
		indexBytes, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", indexPath, err)
		}
		if got := string(indexBytes); got != staleIndex {
			t.Fatalf("workspace MEMORY.md = %q, want unchanged stale index", got)
		}
	})
}

func TestDaemonE2EDreamRoleRoutesBuiltinIdentityAndModel(t *testing.T) {
	t.Parallel()

	t.Run("Should route dream sessions through the configured builtin model", func(t *testing.T) {
		t.Parallel()
		runDaemonE2EDreamRoleRoutesBuiltinIdentityAndModel(t)
	})
}

func runDaemonE2EDreamRoleRoutesBuiltinIdentityAndModel(t *testing.T) {
	t.Helper()

	acpmock.RequireDriver(t)

	diagnosticsPath := filepath.Join(t.TempDir(), "builtin-dream.jsonl")
	command := roleDreamMockCommand(t, diagnosticsPath)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			AgentDefs: []e2etest.AgentSeed{{
				Name:     "session-driver",
				Provider: roleDreamProviderName,
				Command:  command,
				Model:    roleDreamModel,
				Prompt:   "Generate one completed session for dream eligibility.",
			}},
			Mutate: func(cfg *aghconfig.Config) {
				configureRoleDreamE2E(cfg, command)
				cfg.Roles.Dream.Provider = roleDreamProviderName
				cfg.Roles.Dream.Model = roleDreamModel
				cfg.Roles.AutoTitle.Enabled = false
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	seedDreamEligibility(t, ctx, harness, "session-driver", diagnosticsPath)
	triggerDreamEventually(t, ctx, harness, diagnosticsPath)

	dreamRecord := requireDreamPromptDiagnostic(t, diagnosticsPath)
	dreamSession, err := harness.GetSession(ctx, dreamRecord.AGHSessionID)
	if err != nil {
		t.Fatalf("GetSession(dream) error = %v", err)
	}
	if dreamSession.AgentName != aghconfig.BuiltinDreamingCuratorAgentName ||
		dreamSession.Model != roleDreamModel || dreamSession.Type != "dream" {
		t.Fatalf("dream session = %#v, want builtin dreaming-curator routed to %s", dreamSession, roleDreamModel)
	}
	assertDreamHiddenFromFleet(t, ctx, harness, dreamSession.ID)
}

func TestDaemonE2EDreamRoleUsesWorkspaceAgentWithoutAutoTitle(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate workspace dream routing and keep auto-title disabled", func(t *testing.T) {
		t.Parallel()
		runDaemonE2EDreamRoleUsesWorkspaceAgentWithoutAutoTitle(t)
	})
}

func runDaemonE2EDreamRoleUsesWorkspaceAgentWithoutAutoTitle(t *testing.T) {
	t.Helper()

	acpmock.RequireDriver(t)

	diagnosticsPath := filepath.Join(t.TempDir(), "workspace-dream.jsonl")
	command := roleDreamMockCommand(t, diagnosticsPath)
	agentDocument := fmt.Sprintf(`---
name: workspace-curator
provider: %s
command: %s
model: %s
permissions: approve-all
---

Curate durable workspace knowledge.
`, roleDreamProviderName, quotedYAMLString(command), roleDreamModel)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *aghconfig.Config) {
			configureRoleDreamE2E(cfg, command)
			cfg.Roles.MemoryExtractor.Enabled = false
		}},
		Workspace: e2etest.WorkspaceSeedOptions{Files: map[string]string{
			".agh/agents/workspace-curator/AGENT.md": agentDocument,
			".agh/config.toml": `[roles.dream]
agent = "workspace-curator"
provider = "role-dream"
model = "routed-dream-model"

[roles.auto_title]
enabled = false
`,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	seedDreamEligibility(t, ctx, harness, "workspace-curator", diagnosticsPath)
	triggerDreamEventually(t, ctx, harness, diagnosticsPath)

	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(workspace dream) error = %v", err)
	}
	dreamRecord := requireDreamPromptRecord(t, records)
	dreamSession, err := harness.GetSession(ctx, dreamRecord.AGHSessionID)
	if err != nil {
		t.Fatalf("GetSession(workspace dream) error = %v", err)
	}
	if dreamSession.AgentName != "workspace-curator" || dreamSession.Model != roleDreamModel {
		t.Fatalf("workspace dream session = %#v, want workspace-curator routed to %s", dreamSession, roleDreamModel)
	}
	if got := len(acpmock.PromptDiagnostics(records)); got != 2 {
		t.Fatalf("prompt diagnostics = %#v, want one user turn plus one dream and no auto-title", records)
	}
	assertDreamHiddenFromFleet(t, ctx, harness, dreamSession.ID)
}

func configureRoleDreamE2E(cfg *aghconfig.Config, command string) {
	cfg.Providers[roleDreamProviderName] = acpmock.ProviderConfig(command)
	cfg.Memory.Dream.MinHours = 0.000001
	cfg.Memory.Dream.MinSessions = 1
	cfg.Memory.Dream.Gates.MinUnpromoted = 1
	cfg.Memory.Dream.Gates.MinRecallCount = 1
	cfg.Memory.Dream.Gates.MinScore = 0.000001
}

func roleDreamMockCommand(t testing.TB, diagnosticsPath string) string {
	t.Helper()
	driverPath, err := acpmock.DefaultDriverPath()
	if err != nil {
		t.Fatalf("acpmock.DefaultDriverPath() error = %v", err)
	}
	return acpmock.BuildCommand(
		driverPath,
		mockFixturePath(t, "agent_roles_fixture.json"),
		"role-agent",
		diagnosticsPath,
	)
}

func quotedYAMLString(value string) string {
	return strconv.Quote(value)
}

func seedDreamEligibility(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
	diagnosticsPath string,
) {
	t.Helper()
	writeMemoryViaUDS(
		t,
		ctx,
		harness,
		"dream-routing.md",
		harness.WorkspaceRoot,
		"Remember the configurable background dream routing signal.",
	)
	session := createFixtureBackedSession(t, ctx, harness, agentName, "")
	if _, err := harness.PromptSession(ctx, session.ID, "remember the background dream routing signal"); err != nil {
		t.Fatalf("PromptSession(dream eligibility) error = %v", err)
	}
	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(dream eligibility) error = %v", err)
	}
	ownedPrompts := acpmock.PromptDiagnostics(acpmock.DiagnosticsForAGHSession(records, session.ID))
	if len(ownedPrompts) != 1 || !strings.Contains(ownedPrompts[0].Prompt, "Relevant durable memory for this turn:") {
		t.Fatalf("dream eligibility prompts = %#v, want recalled durable memory", ownedPrompts)
	}
	if err := harness.StopSession(ctx, session.ID); err != nil {
		t.Fatalf("StopSession(dream eligibility) error = %v", err)
	}
	waitForRuntimeCondition(t, "dream eligibility session stopped", 10*time.Second, func() bool {
		current, err := harness.GetSession(ctx, session.ID)
		return err == nil && current.State == sessionpkg.StateStopped
	})
}

func triggerDreamEventually(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	diagnosticsPath string,
) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()

	for {
		recorded, err := dreamPromptRecorded(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(dream poll) error = %v", err)
		}
		if recorded {
			return
		}
		var response aghcontract.MemoryDreamTriggerResponse
		if err := harness.UDSJSON(
			ctx,
			http.MethodPost,
			"/api/memory/dreams/trigger",
			aghcontract.MemoryDreamTriggerRequest{WorkspaceID: harness.WorkspaceID},
			&response,
		); err != nil {
			t.Fatalf("trigger memory dream error = %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf(
				"memory dream never triggered: %v (last reason %q; %s)",
				ctx.Err(),
				response.Reason,
				dreamGateDiagnostics(
					t,
					harness.HomePaths.DatabaseFile,
					harness.HomePaths.SessionsDir,
					harness.WorkspaceRoot,
				),
			)
		case <-deadline.C:
			t.Fatalf(
				"memory dream never triggered before deadline (last reason %q; %s)",
				response.Reason,
				dreamGateDiagnostics(
					t,
					harness.HomePaths.DatabaseFile,
					harness.HomePaths.SessionsDir,
					harness.WorkspaceRoot,
				),
			)
		case <-ticker.C:
		}
	}
}

func dreamPromptRecorded(diagnosticsPath string) (bool, error) {
	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		return false, err
	}
	for _, record := range acpmock.PromptDiagnostics(records) {
		if strings.Contains(record.Prompt, "# Dream Consolidation") {
			return true, nil
		}
	}
	return false, nil
}

func dreamGateDiagnostics(t testing.TB, databasePath string, sessionsDir string, workspaceRoot string) string {
	t.Helper()
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return fmt.Sprintf("open catalog: %v", err)
	}
	var (
		count          int
		minRecallCount int
		maxRecallCount int
		minRecallScore float64
		maxRecallScore float64
		workspaceIDs   string
	)
	queryErr := db.QueryRow(`
SELECT
  COUNT(*),
  COALESCE(MIN(sig.recall_count), 0),
  COALESCE(MAX(sig.recall_count), 0),
  COALESCE(MIN(sig.recall_score), 0),
  COALESCE(MAX(sig.recall_score), 0),
  COALESCE(GROUP_CONCAT(DISTINCT e.workspace_id), '')
FROM memory_recall_signals sig
JOIN memory_chunks c ON c.id = sig.chunk_id
JOIN memory_catalog_entries e ON e.id = c.file_id
WHERE sig.promoted_at IS NULL AND e.injection = 1
`).Scan(&count, &minRecallCount, &maxRecallCount, &minRecallScore, &maxRecallScore, &workspaceIDs)
	closeErr := db.Close()
	if queryErr != nil {
		return fmt.Sprintf("query catalog: %v", queryErr)
	}
	if closeErr != nil {
		return fmt.Sprintf("close catalog: %v", closeErr)
	}
	completedSessions := 0
	entries, entriesErr := os.ReadDir(sessionsDir)
	if entriesErr == nil {
		for _, entry := range entries {
			metadata, readErr := os.ReadFile(filepath.Join(sessionsDir, entry.Name(), "meta.json"))
			if readErr == nil && (strings.Contains(string(metadata), `"state":"stopped"`) ||
				strings.Contains(string(metadata), `"state": "stopped"`)) {
				completedSessions++
			}
		}
	}
	identity, identityErr := os.ReadFile(filepath.Join(workspaceRoot, ".agh", "workspace.toml"))
	return fmt.Sprintf(
		"eligible rows=%d recall_count=%d..%d recall_score=%g..%g workspaces=%q completed_session_files=%d session_scan_error=%v identity=%q identity_error=%v",
		count,
		minRecallCount,
		maxRecallCount,
		minRecallScore,
		maxRecallScore,
		workspaceIDs,
		completedSessions,
		entriesErr,
		strings.TrimSpace(string(identity)),
		identityErr,
	)
}

func requireDreamPromptDiagnostic(t testing.TB, diagnosticsPath string) acpmock.DiagnosticsRecord {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(dream) error = %v", err)
	}
	return requireDreamPromptRecord(t, records)
}

func requireDreamPromptRecord(
	t testing.TB,
	records []acpmock.DiagnosticsRecord,
) acpmock.DiagnosticsRecord {
	t.Helper()
	for _, record := range acpmock.PromptDiagnostics(records) {
		if strings.Contains(record.Prompt, "# Dream Consolidation") {
			return record
		}
	}
	t.Fatalf("prompt diagnostics = %#v, want dream consolidation prompt", records)
	return acpmock.DiagnosticsRecord{}
}

func assertDreamHiddenFromFleet(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	dreamSessionID string,
) {
	t.Helper()
	var response aghcontract.SessionCatalogResponse
	path := "/api/sessions?workspace=" + url.QueryEscape(harness.WorkspaceID) + "&limit=100"
	if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("list fleet sessions error = %v", err)
	}
	for _, candidate := range response.Sessions {
		if candidate.ID == dreamSessionID {
			t.Fatalf("fleet sessions = %#v, want dream session %q hidden", response.Sessions, dreamSessionID)
		}
	}
}

func writeMemoryViaCLI(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workdir string,
	filename string,
	memoryType memcontract.Type,
	description string,
	content string,
	scope memcontract.Scope,
) {
	t.Helper()

	var result aghcontract.MemoryMutationDecisionResponse
	args := []string{
		"memory",
		"write",
		"--type",
		string(memoryType),
		"--name",
		strings.TrimSuffix(filename, filepath.Ext(filename)),
		"--description",
		description,
		"--content",
		content,
		"--scope",
		string(scope),
		"-o",
		"json",
	}
	if scope == memcontract.ScopeWorkspace {
		args = append(args[:len(args)-2], "--workspace", workdir, "-o", "json")
	}
	if err := harness.CLI.RunJSONInDir(ctx, workdir, &result, args...); err != nil {
		t.Fatalf("CLI memory write %q error = %v", filename, err)
	}
	if !result.Applied {
		t.Fatalf("CLI memory write %q = %#v, want applied=true", filename, result)
	}
}

func writeMemoryViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	filename string,
	workspace string,
	content string,
) {
	t.Helper()

	var response aghcontract.MemoryMutationDecisionResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/memory",
		aghcontract.MemoryCreateRequest{
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: workspace,
			Type:        memcontract.TypeProject,
			Name:        strings.TrimSuffix(filename, filepath.Ext(filename)),
			Content:     content,
		},
		&response,
	); err != nil {
		t.Fatalf("UDS memory write %q error = %v", filename, err)
	}
	if !response.Applied {
		t.Fatalf("UDS memory write %q = %#v, want applied=true", filename, response)
	}
}

func memorySearchRequest(
	query string,
	scope memcontract.Scope,
	workspace string,
) aghcontract.MemorySearchRequest {
	return aghcontract.MemorySearchRequest{
		QueryText:   query,
		Scope:       scope,
		WorkspaceID: workspace,
	}
}

func containsSearchResult(response aghcontract.MemorySearchResponse, filename string, scope memcontract.Scope) bool {
	for _, result := range response.Results {
		if result.Memory.Filename == filename && result.Memory.Scope == scope {
			return true
		}
	}
	return false
}

func containsLogEventSummary(
	events []aghcontract.LogEventPayload,
	eventType string,
	summaryFragment string,
) bool {
	for _, event := range events {
		if event.Type != eventType {
			continue
		}
		if strings.Contains(event.Summary, summaryFragment) {
			return true
		}
	}
	return false
}

func firstAgentEventByType(
	events []aghcontract.AgentEventPayload,
	eventType string,
) (aghcontract.AgentEventPayload, bool) {
	for _, event := range events {
		if event.Type == eventType {
			return event, true
		}
	}
	return aghcontract.AgentEventPayload{}, false
}
