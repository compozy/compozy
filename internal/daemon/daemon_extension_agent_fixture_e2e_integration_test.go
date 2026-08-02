//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

func configureExtensionAgentFixture(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	driverPath string,
	fixturePath string,
	fixtureAgentName string,
	extensionAgentName string,
) {
	t.Helper()

	fixture, err := acpmock.LoadFixture(fixturePath)
	if err != nil {
		t.Fatalf("LoadFixture(%q) error = %v", fixturePath, err)
	}
	fixtureAgent, err := fixture.Agent(fixtureAgentName)
	if err != nil {
		t.Fatalf("Fixture.Agent(%q) error = %v", fixtureAgentName, err)
	}

	path := "/api/agents/" + url.PathEscape(extensionAgentName)
	var current compozycontract.AgentResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &current); err != nil {
		t.Fatalf("HTTP get extension agent %q error = %v", extensionAgentName, err)
	}

	permissions := current.Agent.Permissions
	if value := strings.TrimSpace(fixtureAgent.Permissions); value != "" {
		permissions = value
	}
	request := compozycontract.UpdateAgentRequest{
		Agent: compozycontract.CreateAgentPayload{
			Name:            current.Agent.Name,
			Provider:        strings.TrimSpace(fixtureAgent.Provider),
			Command:         acpmock.BuildCommand(driverPath, fixturePath, fixtureAgentName, ""),
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
		t.Fatalf("HTTP configure extension agent %q error = %v", extensionAgentName, err)
	}
	if updated.Agent.Provider != request.Agent.Provider || updated.Agent.Command != request.Agent.Command {
		t.Fatalf("configured extension agent %q = %#v, want fixture runtime", extensionAgentName, updated.Agent)
	}
}
