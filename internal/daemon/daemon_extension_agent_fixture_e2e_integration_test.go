//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	devcycle "github.com/compozy/compozy/extensions/dev-cycle"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

type extensionAgentFixtureConfig struct {
	DriverPath         string
	FixturePath        string
	FixtureAgentName   string
	ExtensionAgentName string
}

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

	requireDevCycleExtensionEnabled(t, ctx, harness)
	configureExtensionAgentFixture(t, ctx, harness, extensionAgentFixtureConfig{
		DriverPath:         driverPath,
		FixturePath:        fixturePath,
		FixtureAgentName:   "review_and_fix_invalid_reviewer",
		ExtensionAgentName: "reviewer",
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
	var extensionCommand *compozycontract.SessionCommandPayload
	for index := range catalog.Commands {
		command := &catalog.Commands[index]
		if command.Lane == "skill" && command.Source.Kind == "extension" && command.Source.ID == devcycle.Name {
			extensionCommand = command
			break
		}
	}
	if extensionCommand == nil {
		t.Fatalf("HTTP extension session commands = %+v, want a dev-cycle skill", catalog.Commands)
	}
	if strings.TrimSpace(extensionCommand.CanonicalToken) == "" {
		t.Fatalf("dev-cycle command = %+v, want canonical token", extensionCommand)
	}

	plainStream, err := harness.PromptSessionHTTP(ctx, active.ID, "hello")
	if err != nil {
		t.Fatalf("PromptSessionHTTP(plain) error = %v", err)
	}
	assertSuccessfulExtensionPromptStream(t, plainStream, "plain")
	waitForExtensionSessionIdle(t, ctx, harness, active.ID)

	skillMessage := extensionCommand.CanonicalToken + " verify the current change"
	skillStream, err := harness.PromptSessionHTTP(ctx, active.ID, skillMessage)
	if err != nil {
		t.Fatalf("PromptSessionHTTP(skill) error = %v", err)
	}
	assertSuccessfulExtensionPromptStream(t, skillStream, "skill")
	assertPersistedExtensionSkillInvocation(t, ctx, harness, active.ID, *extensionCommand)
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

func requireDevCycleExtensionEnabled(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()

	enabled, err := harness.EnableExtension(ctx, devcycle.Name)
	if err != nil {
		t.Fatalf("EnableExtension(%s) error = %v", devcycle.Name, err)
	}
	if !enabled.Enabled {
		t.Fatalf("EnableExtension(%s).Enabled = false, want true", devcycle.Name)
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
				"",
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
