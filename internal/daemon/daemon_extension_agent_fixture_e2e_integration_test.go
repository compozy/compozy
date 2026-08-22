//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type extensionAgentFixtureConfig struct {
	DriverPath         string
	FixturePath        string
	FixtureAgentName   string
	ExtensionAgentName string
	DiagnosticsPath    string
}

type hostedSourceQualifiedSkillViewPayload struct {
	Name   string            `json:"name"`
	Source commandpkg.Source `json:"source"`
}

var (
	extensionCurrentPromptCatalogPattern = regexp.MustCompile(
		`(?s)<current-available-skills>(.*?)</current-available-skills>`,
	)
	extensionPromptSkillNamePattern = regexp.MustCompile(`<skill name="([^"]+)">`)
	reviewerSkillNames              = append([]string{"compozy"}, specCycleIntegrationSkillNames...)
)

func TestDaemonE2EExtensionPublishedAgentSessionCommandsAndPrompt(t *testing.T) {
	t.Run(
		"Should expose extension skills and prompt through an unbound session",
		testDaemonE2EExtensionPublishedAgentSessionCommandsAndPrompt,
	)
}

func testDaemonE2EExtensionPublishedAgentSessionCommandsAndPrompt(t *testing.T) {
	t.Parallel()

	driverPath := acpmock.RequireDriver(t)
	fixturePath := mockFixturePath(t, "review_and_fix_fixture.json")
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultProvider: acpmock.ProviderName,
			PermissionMode:  compozyconfig.PermissionModeApproveAll,
			Providers: map[string]compozyconfig.ProviderConfig{
				acpmock.ProviderName: acpmock.ProviderConfig(driverPath),
			},
		},
		StartTimeout: 30 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	requireSpecCycleExtensionEnabled(t, ctx, harness)
	diagnosticsPath := filepath.Join(harness.HomePaths.LogsDir, "extension-reviewer.jsonl")
	configureExtensionAgentFixture(t, ctx, harness, extensionAgentFixtureConfig{
		DriverPath:         driverPath,
		FixturePath:        fixturePath,
		FixtureAgentName:   "review_and_fix_invalid_reviewer",
		ExtensionAgentName: "reviewer",
		DiagnosticsPath:    diagnosticsPath,
	})

	created, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     "reviewer",
		Name:          "extension-command-catalog",
		WorkspacePath: harness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession(reviewer) error = %v", err)
	}
	active, err := harness.WaitForSessionActive(ctx, created.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(reviewer) error = %v", err)
	}
	if active.Runtime.ACPSessionID != "" {
		t.Fatalf("session runtime ACP ID = %q before first prompt, want unbound", active.Runtime.ACPSessionID)
	}

	commandsPath := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/sessions/" + url.PathEscape(active.ID) + "/commands"
	var catalog compozycontract.SessionCommandsResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, commandsPath, nil, &catalog); err != nil {
		t.Fatalf("HTTP extension session commands error = %v", err)
	}
	if strings.TrimSpace(catalog.Revision) == "" {
		t.Fatal("HTTP extension session commands revision is empty")
	}
	reviewerCommands := reviewerSkillCommands(t, catalog.Commands)
	extensionCommands := extensionSkillCommands(t, reviewerCommands)
	extensionCommand, ok := extensionCommands["cy-review-round"]
	if !ok {
		t.Fatalf("HTTP extension session commands = %+v, want cy-review-round", catalog.Commands)
	}
	if strings.TrimSpace(extensionCommand.CanonicalToken) == "" {
		t.Fatalf("spec-cycle command = %+v, want canonical token", extensionCommand)
	}

	plainStream, err := harness.PromptSessionHTTP(ctx, active.ID, "hello")
	if err != nil {
		t.Fatalf("PromptSessionHTTP(plain) error = %v", err)
	}
	assertSuccessfulExtensionPromptStream(t, plainStream, "plain")
	waitForExtensionSessionIdle(t, ctx, harness, active.ID)
	assertExtensionPromptCatalog(t, diagnosticsPath)
	assertExtensionHostedMCPTools(t, ctx, harness, diagnosticsPath, reviewerCommands)

	skillMessage := extensionCommand.CanonicalToken + " verify the current change"
	skillStream, err := harness.PromptSessionHTTP(ctx, active.ID, skillMessage)
	if err != nil {
		t.Fatalf("PromptSessionHTTP(skill) error = %v", err)
	}
	assertSuccessfulExtensionPromptStream(t, skillStream, "skill")
	assertPersistedExtensionSkillInvocation(t, ctx, harness, active.ID, extensionCommand)
}

func assertExtensionPromptCatalog(t testing.TB, diagnosticsPath string) {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(extension reviewer) error = %v", err)
	}
	for _, record := range acpmock.PromptDiagnostics(records) {
		if strings.Contains(record.Prompt, "<current-available-skills>") {
			assertSkillNames(
				t,
				"extension prompt catalog",
				extensionPromptSkillNames(record.Prompt),
				reviewerSkillNames,
			)
			return
		}
	}
	t.Fatalf("extension prompt diagnostics = %#v, want current complete spec-cycle catalog", records)
}

func assertExtensionHostedMCPTools(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	diagnosticsPath string,
	commands map[string]compozycontract.SessionCommandPayload,
) {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(extension hosted MCP) error = %v", err)
	}
	client := startHostedMCPClient(t, ctx, requireHostedMCPStdioServer(t, records, hostedMCPServerEarliest))
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("Close(extension hosted MCP client) error = %v", closeErr)
		}
	}()

	availableTools, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools(extension hosted MCP) error = %v", err)
	}
	for _, toolID := range []toolspkg.ToolID{
		toolspkg.ToolIDSkillList,
		toolspkg.ToolIDSkillSearch,
		toolspkg.ToolIDSkillView,
		toolspkg.ToolIDConfigGet,
	} {
		if !sdkToolListContains(availableTools.Tools, toolID.String()) {
			t.Fatalf("extension hosted MCP tools = %#v, want %s", sdkToolNames(availableTools.Tools), toolID)
		}
	}

	var listed struct {
		Skills []compozycontract.SkillPayload `json:"skills"`
	}
	callHostedMCPToolJSON(t, ctx, client, toolspkg.ToolIDSkillList.String(), map[string]any{}, &listed)
	assertHostedSkillCatalog(t, "extension hosted skill_list", listed.Skills)

	var searched struct {
		Skills []compozycontract.SkillPayload `json:"skills"`
	}
	callHostedMCPToolJSON(
		t,
		ctx,
		client,
		toolspkg.ToolIDSkillSearch.String(),
		map[string]any{"query": ""},
		&searched,
	)
	assertHostedSkillCatalog(t, "extension hosted skill_search", searched.Skills)

	viewedSkills := make([]hostedSourceQualifiedSkillViewPayload, 0, len(commands))
	for name, command := range commands {
		var viewed struct {
			CommandID string                                `json:"command_id"`
			Skill     hostedSourceQualifiedSkillViewPayload `json:"skill"`
			Content   string                                `json:"content"`
		}
		callHostedMCPToolJSON(
			t,
			ctx,
			client,
			toolspkg.ToolIDSkillView.String(),
			map[string]any{"command_id": command.ID},
			&viewed,
		)
		if viewed.CommandID != command.ID {
			t.Fatalf("extension hosted skill_view(%q) command ID = %q, want %q", name, viewed.CommandID, command.ID)
		}
		if viewed.Skill.Name != name {
			t.Fatalf("extension hosted skill_view(%q) skill name = %q, want %q", name, viewed.Skill.Name, name)
		}
		wantSource := commandpkg.Source{
			Kind:  command.Source.Kind,
			ID:    command.Source.ID,
			Key:   command.Source.Key,
			Scope: command.Source.Scope,
		}
		if viewed.Skill.Source != wantSource {
			t.Fatalf("extension hosted skill_view(%q) source = %+v, want %+v", name, viewed.Skill.Source, wantSource)
		}
		if strings.TrimSpace(viewed.Content) == "" {
			t.Fatalf("extension hosted skill_view(%q) content is empty", name)
		}
		viewedSkills = append(viewedSkills, viewed.Skill)
	}
	assertHostedSkillViewCatalog(t, "extension hosted skill_view", viewedSkills)

	command, ok := commands["cy-review-round"]
	if !ok {
		t.Fatal("extension commands missing cy-review-round after exact-set assertion")
	}
	missingConfigMessage := assertExtensionHostedToolReason(
		t,
		ctx,
		client,
		toolspkg.ToolIDConfigGet,
		map[string]any{
			"path":      "loops.inputs.extension-probe.preference",
			"workspace": harness.WorkspaceRoot,
		},
		toolspkg.ReasonConfigPathNotFound,
	)
	if want := "config_path_not_found: config path not found"; missingConfigMessage != want {
		t.Fatalf("extension hosted config_get missing path message = %q, want %q", missingConfigMessage, want)
	}
	if strings.Contains(missingConfigMessage, "tool not found") {
		t.Fatalf(
			"extension hosted config_get missing path message = %q, must not identify the tool as missing",
			missingConfigMessage,
		)
	}

	var foreign compozycontract.WorkspaceResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/workspaces",
		compozycontract.CreateWorkspaceRequest{RootDir: t.TempDir(), Name: "extension-foreign"},
		&foreign,
	); err != nil {
		t.Fatalf("UDS create foreign extension workspace error = %v", err)
	}
	assertExtensionHostedToolReason(
		t,
		ctx,
		client,
		toolspkg.ToolIDSkillView,
		map[string]any{"command_id": command.ID, "workspace": foreign.Workspace.ID},
		toolspkg.ReasonWorkspaceAccessDenied,
	)
}

func reviewerSkillCommands(
	t testing.TB,
	commands []compozycontract.SessionCommandPayload,
) map[string]compozycontract.SessionCommandPayload {
	t.Helper()

	byName := make(map[string]compozycontract.SessionCommandPayload)
	for _, command := range commands {
		if command.Lane != "skill" {
			continue
		}
		name := strings.TrimPrefix(command.DisplayName, "/")
		if _, exists := byName[name]; exists {
			t.Fatalf("HTTP reviewer session commands duplicate skill %q: %+v", name, commands)
		}
		byName[name] = command
	}
	assertReviewerSkillNames(t, "HTTP reviewer session commands", extensionSkillCommandNames(byName))
	return byName
}

func extensionSkillCommands(
	t testing.TB,
	commands map[string]compozycontract.SessionCommandPayload,
) map[string]compozycontract.SessionCommandPayload {
	t.Helper()

	byName := make(map[string]compozycontract.SessionCommandPayload)
	for name, command := range commands {
		if command.Source.Kind == "extension" && command.Source.ID == speccycle.Name {
			byName[name] = command
		}
	}
	assertExtensionSkillNames(t, "HTTP reviewer extension commands", extensionSkillCommandNames(byName))
	return byName
}

func extensionPromptSkillNames(prompt string) []string {
	catalog := extensionCurrentPromptCatalogPattern.FindStringSubmatch(prompt)
	if len(catalog) != 2 {
		return nil
	}
	matches := extensionPromptSkillNamePattern.FindAllStringSubmatch(catalog[1], -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	return names
}

func assertHostedSkillCatalog(
	t testing.TB,
	surface string,
	skills []compozycontract.SkillPayload,
) {
	t.Helper()

	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
	}
	assertReviewerSkillNames(t, surface, names)

	extensionNames := make([]string, 0, len(skills))
	for _, skill := range skills {
		if skill.Provenance != nil && skill.Provenance.InstalledFromExtension == speccycle.Name {
			extensionNames = append(extensionNames, skill.Name)
		}
	}
	assertExtensionSkillNames(t, surface+" spec-cycle provenance", extensionNames)
}

func assertHostedSkillViewCatalog(
	t testing.TB,
	surface string,
	skills []hostedSourceQualifiedSkillViewPayload,
) {
	t.Helper()

	names := make([]string, 0, len(skills))
	extensionNames := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
		if skill.Source.Kind == "extension" && skill.Source.ID == speccycle.Name {
			extensionNames = append(extensionNames, skill.Name)
		}
	}
	assertReviewerSkillNames(t, surface, names)
	assertExtensionSkillNames(t, surface+" spec-cycle source", extensionNames)
}

func extensionSkillCommandNames(commands map[string]compozycontract.SessionCommandPayload) []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	return names
}

func assertExtensionSkillNames(t testing.TB, surface string, got []string) {
	t.Helper()

	assertSkillNames(t, surface, got, specCycleIntegrationSkillNames)
}

func assertReviewerSkillNames(t testing.TB, surface string, got []string) {
	t.Helper()

	assertSkillNames(t, surface, got, reviewerSkillNames)
}

func assertSkillNames(t testing.TB, surface string, got, want []string) {
	t.Helper()

	want = slices.Clone(want)
	got = slices.Clone(got)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("%s skill names = %#v, want exactly %#v", surface, got, want)
	}
}

func assertExtensionHostedToolReason(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	toolID toolspkg.ToolID,
	arguments map[string]any,
	want toolspkg.ReasonCode,
) string {
	t.Helper()
	result, err := client.CallTool(ctx, &sdkmcp.CallToolParams{Name: toolID.String(), Arguments: arguments})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolID, err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("CallTool(%s) result = %#v, want error reason %q", toolID, result, want)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal(CallTool(%s) error result) error = %v", toolID, err)
	}
	if !strings.Contains(string(payload), string(want)) {
		t.Fatalf("CallTool(%s) error result = %s, want reason %q", toolID, payload, want)
	}
	for _, content := range result.Content {
		if text, ok := content.(*sdkmcp.TextContent); ok {
			return text.Text
		}
	}
	t.Fatalf("CallTool(%s) error result = %s, want text content", toolID, payload)
	return ""
}

func assertPersistedExtensionSkillInvocation(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	expected compozycontract.SessionCommandPayload,
) {
	t.Helper()
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + "/history?last=1"
	var history compozycontract.SessionHistoryResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &history); err != nil {
		t.Fatalf("HTTP extension session history error = %v", err)
	}
	for _, turn := range history.History {
		for _, event := range turn.Events {
			if event.Type != "user_message" {
				continue
			}
			var content struct {
				SkillInvocations []commandpkg.Invocation `json:"skill_invocations"`
			}
			if err := json.Unmarshal(event.Content, &content); err != nil {
				t.Fatalf("decode extension prompt event content: %v", err)
			}
			for _, invocation := range content.SkillInvocations {
				if invocation.Ref.CommandID == expected.ID &&
					invocation.Ref.Source.Kind == expected.Source.Kind &&
					invocation.Ref.Source.ID == expected.Source.ID {
					return
				}
			}
		}
	}
	t.Fatalf("session history = %+v, want persisted extension invocation %q", history.History, expected.ID)
}

func waitForExtensionSessionIdle(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + "/health"
	var last compozycontract.SessionHealthResponse
	var lastErr error
	for {
		var current compozycontract.SessionHealthResponse
		err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &current)
		if err == nil {
			last = current
			lastErr = nil
			if !current.Health.ActivePrompt {
				return
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf(
				"wait for extension session %q idle: health=%+v last_error=%v context=%v",
				sessionID,
				last.Health,
				lastErr,
				ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func assertSuccessfulExtensionPromptStream(
	t testing.TB,
	stream []e2etest.SSEEvent,
	label string,
) {
	t.Helper()
	if sseStreamContainsEvent(stream, "error") || !sseStreamContainsEvent(stream, "finish") {
		t.Fatalf("%s prompt events = %+v, want finish without error", label, stream)
	}
}

func requireSpecCycleExtensionEnabled(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()

	enabled, err := harness.SetExtensionEnablement(ctx, speccycle.Name, "default", true)
	if err != nil {
		t.Fatalf("SetExtensionEnablement(%s) error = %v", speccycle.Name, err)
	}
	if !enabled.Enabled {
		t.Fatalf("SetExtensionEnablement(%s).Enabled = false, want true", speccycle.Name)
	}
}

func configureExtensionAgentFixture(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	config extensionAgentFixtureConfig,
) {
	t.Helper()

	fixture, err := acpmock.LoadFixture(config.FixturePath)
	if err != nil {
		t.Fatalf("LoadFixture(%q) error = %v", config.FixturePath, err)
	}
	fixtureAgent, err := fixture.Agent(config.FixtureAgentName)
	if err != nil {
		t.Fatalf("Fixture.Agent(%q) error = %v", config.FixtureAgentName, err)
	}

	path := "/api/agents/" + url.PathEscape(config.ExtensionAgentName)
	var current compozycontract.AgentResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &current); err != nil {
		t.Fatalf("HTTP get extension agent %q error = %v", config.ExtensionAgentName, err)
	}

	permissions := current.Agent.Permissions
	if value := strings.TrimSpace(fixtureAgent.Permissions); value != "" {
		permissions = value
	}
	request := compozycontract.UpdateAgentRequest{
		Agent: compozycontract.CreateAgentPayload{
			Name:     current.Agent.Name,
			Provider: strings.TrimSpace(fixtureAgent.Provider),
			Command: acpmock.BuildCommand(
				config.DriverPath,
				config.FixturePath,
				config.FixtureAgentName,
				config.DiagnosticsPath,
			),
			Model:           strings.TrimSpace(fixtureAgent.Model),
			ReasoningEffort: compozycontract.ReasoningEffort(strings.TrimSpace(fixtureAgent.ReasoningEffort)),
			Tools:           append([]string(nil), current.Agent.Tools...),
			Toolsets:        append([]string(nil), current.Agent.Toolsets...),
			DenyTools:       append([]string(nil), current.Agent.DenyTools...),
			Permissions:     compozycontract.SettingsPermissionMode(permissions),
			CategoryPath:    append([]string(nil), current.Agent.CategoryPath...),
			Skills:          current.Agent.Skills,
			Prompt:          current.Agent.Prompt,
		},
		ExpectedDigest: current.Agent.DefinitionDigest,
	}
	var updated compozycontract.AgentResponse
	if err := harness.HTTPJSON(ctx, http.MethodPut, path, request, &updated); err != nil {
		t.Fatalf("HTTP configure extension agent %q error = %v", config.ExtensionAgentName, err)
	}
	if got, want := updated.Agent.Provider, request.Agent.Provider; got != want {
		t.Fatalf("configured extension agent %q provider = %q, want %q", config.ExtensionAgentName, got, want)
	}
	if got, want := updated.Agent.Command, request.Agent.Command; got != want {
		t.Fatalf("configured extension agent %q command = %q, want %q", config.ExtensionAgentName, got, want)
	}
	if got, want := updated.Agent.Model, request.Agent.Model; got != want {
		t.Fatalf("configured extension agent %q model = %q, want %q", config.ExtensionAgentName, got, want)
	}
	if got, want := updated.Agent.ReasoningEffort, request.Agent.ReasoningEffort; got != want {
		t.Fatalf("configured extension agent %q reasoning effort = %q, want %q", config.ExtensionAgentName, got, want)
	}
	if got, want := updated.Agent.Permissions, string(request.Agent.Permissions); got != want {
		t.Fatalf("configured extension agent %q permissions = %q, want %q", config.ExtensionAgentName, got, want)
	}
}
