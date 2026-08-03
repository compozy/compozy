//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	devcycle "github.com/compozy/compozy/extensions/dev-cycle"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

type extensionAgentFixtureConfig struct {
	DriverPath         string
	FixturePath        string
	FixtureAgentName   string
	ExtensionAgentName string
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
