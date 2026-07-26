package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAgentDefFileLoadsCapabilityCatalogAndMCPSidecar(t *testing.T) {
	t.Parallel()

	agentDir := filepath.Join(t.TempDir(), "coder")
	agentPath := filepath.Join(agentDir, agentDefName)
	writeFile(t, agentPath, `---
name: coder
provider: claude
mcp_servers:
  - name: inline-only
    command: inline-only-command
---

Prompt.
`)
	writeFile(t, filepath.Join(agentDir, MCPJSONName), `{
  "mcpServers": {
    "sidecar-only": {
      "command": "sidecar-only-command"
    }
  }
}`)
	writeFile(t, filepath.Join(agentDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
version = "1.0.0"
execution_outline = ["inspect", "build"]
requirements = ["workspace-write", "review-guidelines"]
`)

	agent, err := LoadAgentDefFile(agentPath)
	if err != nil {
		t.Fatalf("LoadAgentDefFile() error = %v", err)
	}
	if got, want := len(agent.MCPServers), 2; got != want {
		t.Fatalf("len(MCPServers) = %d, want %d", got, want)
	}
	if agent.Capabilities == nil {
		t.Fatal("LoadAgentDefFile() Capabilities = nil, want loaded catalog")
	}
	if got, want := len(agent.Capabilities.Capabilities), 1; got != want {
		t.Fatalf("len(Capabilities) = %d, want %d", got, want)
	}
	if got, want := agent.Capabilities.Capabilities[0].ID, "build-site"; got != want {
		t.Fatalf("Capabilities[0].ID = %q, want %q", got, want)
	}
	if got, want := agent.Capabilities.Capabilities[0].Version, "1.0.0"; got != want {
		t.Fatalf("Capabilities[0].Version = %q, want %q", got, want)
	}
	if got, want := agent.Capabilities.Capabilities[0].Requirements, []string{
		"review-guidelines",
		"workspace-write",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("Capabilities[0].Requirements = %#v, want %#v", got, want)
	}
	if !strings.HasPrefix(agent.Capabilities.Capabilities[0].Digest, "sha256:") {
		t.Fatalf("Capabilities[0].Digest = %q, want sha256 digest", agent.Capabilities.Capabilities[0].Digest)
	}
}

func TestLoadAgentDefFileNormalizesEquivalentTOMLAndJSONCapabilitiesToSameRuntimeShape(t *testing.T) {
	t.Parallel()

	makeAgent := func(t *testing.T, dir string) string {
		t.Helper()

		agentPath := filepath.Join(dir, agentDefName)
		writeFile(t, agentPath, `---
name: coder
provider: claude
---

Prompt.
`)
		return agentPath
	}

	tomlDir := filepath.Join(t.TempDir(), "toml-agent")
	jsonDir := filepath.Join(t.TempDir(), "json-agent")
	tomlAgentPath := makeAgent(t, tomlDir)
	jsonAgentPath := makeAgent(t, jsonDir)

	writeFile(t, filepath.Join(tomlDir, capabilityCatalogTOMLName), `
[[capabilities]]
id = "review-copy"
summary = " Review conversion copy. "
outcome = " A prioritized copy review. "
version = " 1.2.0 "
context_needed = [" brief ", " analytics "]
requirements = [" workspace-write ", "review-guidelines"]
`)
	writeFile(t, filepath.Join(jsonDir, capabilityCatalogJSONName), `{
  "capabilities": [
    {
      "id": "review-copy",
      "summary": "Review conversion copy.",
      "outcome": "A prioritized copy review.",
      "version": "1.2.0",
      "context_needed": ["brief", "analytics"],
      "requirements": ["review-guidelines", "workspace-write"]
    }
  ]
}`)

	tomlAgent, err := LoadAgentDefFile(tomlAgentPath)
	if err != nil {
		t.Fatalf("LoadAgentDefFile(TOML) error = %v", err)
	}
	jsonAgent, err := LoadAgentDefFile(jsonAgentPath)
	if err != nil {
		t.Fatalf("LoadAgentDefFile(JSON) error = %v", err)
	}
	if !reflect.DeepEqual(tomlAgent.Capabilities, jsonAgent.Capabilities) {
		t.Fatalf(
			"capabilities differ after normalization:\nTOML: %#v\nJSON: %#v",
			tomlAgent.Capabilities,
			jsonAgent.Capabilities,
		)
	}
}

func TestLoadWorkspaceAgentDefsPreservesPrecedenceWithCapabilities(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	root := t.TempDir()
	additional := t.TempDir()

	writeAgentDefinition(
		t,
		filepath.Join(root, DirName, AgentsDirName, "coder", agentDefName),
		"coder",
		"claude",
		"workspace",
	)
	writeAgentDefinition(
		t,
		filepath.Join(additional, DirName, AgentsDirName, "coder", agentDefName),
		"coder",
		"claude",
		"additional",
	)
	writeFile(
		t,
		filepath.Join(additional, DirName, AgentsDirName, "coder", capabilityCatalogDirName, "build-site.toml"),
		`
id = "build-site"
summary = "Build the landing page."
outcome = "A finished landing page."
`,
	)

	writeAgentDefinition(
		t,
		filepath.Join(additional, DirName, AgentsDirName, "pairer", agentDefName),
		"pairer",
		"claude",
		"additional-pair",
	)
	writeFile(t, filepath.Join(additional, DirName, AgentsDirName, "pairer", capabilityCatalogTOMLName), `
[[capabilities]]
id = "review-copy"
summary = "Review conversion copy."
outcome = "A prioritized copy review."
`)

	writeAgentDefinition(
		t,
		filepath.Join(homePaths.AgentsDir, "reviewer", agentDefName),
		"reviewer",
		"claude",
		"global-review",
	)
	writeFile(t, filepath.Join(homePaths.AgentsDir, "reviewer", capabilityCatalogJSONName), `{
  "capabilities": [
    {
      "id": "triage-pr",
      "summary": "Triage pull request feedback.",
      "outcome": "A prioritized remediation plan."
    }
  ]
}`)

	agents, err := LoadWorkspaceAgentDefs(root, []string{additional}, homePaths)
	if err != nil {
		t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
	}

	tests := []struct {
		name      string
		agentName string
		assert    func(t *testing.T, agent AgentDef)
	}{
		{
			name:      "ShouldPreferWorkspaceDefinitionForCoderWithoutCapabilityFallback",
			agentName: "coder",
			assert: func(t *testing.T, agent AgentDef) {
				t.Helper()

				if got, want := agent.Model, "workspace"; got != want {
					t.Fatalf("coder.Model = %q, want %q", got, want)
				}
				if agent.Capabilities != nil {
					t.Fatalf(
						"coder.Capabilities = %#v, want nil because winning workspace definition has no catalog",
						agent.Capabilities,
					)
				}
			},
		},
		{
			name:      "ShouldLoadAdditionalDirectoryCapabilitiesForPairer",
			agentName: "pairer",
			assert: func(t *testing.T, agent AgentDef) {
				t.Helper()

				if agent.Capabilities == nil || len(agent.Capabilities.Capabilities) != 1 {
					t.Fatalf("pairer.Capabilities = %#v, want single loaded capability", agent.Capabilities)
				}
				if got, want := agent.Capabilities.Capabilities[0].ID, "review-copy"; got != want {
					t.Fatalf("pairer capability ID = %q, want %q", got, want)
				}
			},
		},
		{
			name:      "ShouldLoadGlobalCapabilitiesForReviewer",
			agentName: "reviewer",
			assert: func(t *testing.T, agent AgentDef) {
				t.Helper()

				if agent.Capabilities == nil || len(agent.Capabilities.Capabilities) != 1 {
					t.Fatalf("reviewer.Capabilities = %#v, want single loaded capability", agent.Capabilities)
				}
				if got, want := agent.Capabilities.Capabilities[0].ID, "triage-pr"; got != want {
					t.Fatalf("reviewer capability ID = %q, want %q", got, want)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.assert(t, findAgentByName(t, agents, tc.agentName))
		})
	}
}

func TestLoadWorkspaceAgentDefsLoadsAgentsWithoutCapabilityCatalog(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	root := t.TempDir()
	writeAgentDefinition(
		t,
		filepath.Join(root, DirName, AgentsDirName, "coder", agentDefName),
		"coder",
		"claude",
		"workspace",
	)

	agents, err := LoadWorkspaceAgentDefs(root, nil, homePaths)
	if err != nil {
		t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
	}
	if got, want := len(agents), 1; got != want {
		t.Fatalf("len(LoadWorkspaceAgentDefs()) = %d, want %d", got, want)
	}
	if agents[0].Capabilities != nil {
		t.Fatalf("agents[0].Capabilities = %#v, want nil for missing catalog", agents[0].Capabilities)
	}
}

func TestAgentDefValidateNormalizesCapabilitiesInPlace(t *testing.T) {
	t.Parallel()

	agent := AgentDef{
		Name:   "coder",
		Prompt: "You write reliable code.",
		Capabilities: &CapabilityCatalog{
			Capabilities: []CapabilityDef{{
				ID:                " build-site ",
				Summary:           " Build the landing page. ",
				Outcome:           " A finished landing page. ",
				Version:           " 1.0.0 ",
				ContextNeeded:     []string{" repo ", "", " brand brief "},
				ExecutionOutline:  []string{" inspect ", "", " build "},
				ArtifactsExpected: []string{" final page ", ""},
				Requirements:      []string{" workspace-write ", "review-guidelines"},
			}},
		},
	}

	if err := agent.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if agent.Capabilities == nil || len(agent.Capabilities.Capabilities) != 1 {
		t.Fatalf("Capabilities = %#v, want one normalized capability", agent.Capabilities)
	}

	capability := agent.Capabilities.Capabilities[0]
	if got, want := capability.ID, "build-site"; got != want {
		t.Fatalf("ID = %q, want %q", got, want)
	}
	if got, want := capability.Summary, "Build the landing page."; got != want {
		t.Fatalf("Summary = %q, want %q", got, want)
	}
	if got, want := capability.Outcome, "A finished landing page."; got != want {
		t.Fatalf("Outcome = %q, want %q", got, want)
	}
	if got, want := capability.Version, "1.0.0"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
	if got, want := capability.ContextNeeded, []string{"repo", "brand brief"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ContextNeeded = %#v, want %#v", got, want)
	}
	if got, want := capability.ExecutionOutline, []string{"inspect", "build"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ExecutionOutline = %#v, want %#v", got, want)
	}
	if got, want := capability.ArtifactsExpected, []string{"final page"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ArtifactsExpected = %#v, want %#v", got, want)
	}
	if got, want := capability.Requirements, []string{
		"review-guidelines",
		"workspace-write",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("Requirements = %#v, want %#v", got, want)
	}
	if !strings.HasPrefix(capability.Digest, "sha256:") {
		t.Fatalf("Digest = %q, want sha256 digest", capability.Digest)
	}
}

func findAgentByName(t *testing.T, agents []AgentDef, name string) AgentDef {
	t.Helper()

	for _, agent := range agents {
		if agent.Name == name {
			return agent
		}
	}

	t.Fatalf("agent %q not found in %#v", name, agents)
	return AgentDef{}
}
