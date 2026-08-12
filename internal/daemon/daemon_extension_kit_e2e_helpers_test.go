//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/windowmanager"
)

const (
	extensionKitE2EAgentName = "kit-e2e-writer"
	extensionKitE2EEnvName   = "KIT_E2E_TOKEN"
	extensionKitE2ESecret    = "kit-e2e-secret-value"
)

func configureExtensionKitE2ESource(t *testing.T, sourceDir string) {
	t.Helper()

	writeExtensionKitE2EFile(t, sourceDir, "skills/review/SKILL.md", `---
name: kit-e2e-review
description: Review the extension kit E2E fixture
---

Review the fixture output.
`)
	writeExtensionKitE2EFile(t, sourceDir, "agents/kit-e2e-writer/AGENT.md", `---
name: kit-e2e-writer
provider: claude
---

Write the extension kit report.
`)
	writeExtensionKitE2EFile(t, sourceDir, "agents/kit-e2e-writer/SOUL.md", "Prefer concise reports.\n")
	writeExtensionKitE2EFile(t, sourceDir, "agents/kit-e2e-writer/HEARTBEAT.md", "Check the kit queue.\n")
	writeExtensionKitE2EFile(t, sourceDir, "loops/kit-e2e/loop.yaml", `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: kit-e2e }
concurrency: forbid
inputs:
  done: { type: boolean, required: true }
contract:
  goal: Exercise the extension kit
  definition_of_done: The input is done
  constraints: []
  boundaries: []
  stop_when: "inputs.done == true"
  verification: []
  terminal_states: [done, failed]
  iteration_cap: 1
  no_progress: { window: 1, hash_fields: [] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: source
      class: source
      kind: input
      input_ref: done
  edges: []
start: [{ kind: manual }]
`)
	writeExtensionKitE2EAutomation(t, sourceDir, "daily")
	writeExtensionKitE2ELayout(t, sourceDir)

	mainPath := filepath.Join(sourceDir, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", mainPath, err)
	}
	const oldDefinition = "\t\tPermissions: compozysdk.PermissionsConfig{},\n"
	requiresEnv := fmt.Sprintf("\t\tRequiresEnv: []string{%q},\n", extensionKitE2EEnvName)
	newDefinition := fmt.Sprintf(`%s		Resources: compozysdk.DescribeResources{
			Skills: []string{"skills"}, Loops: []string{"loops"}, Agents: []string{"agents"},
			Automation: []string{"automation"}, Layouts: []string{"layouts/kit-e2e.json"},
		},
		Permissions: compozysdk.PermissionsConfig{},
`, requiresEnv)
	content := strings.Replace(string(data), oldDefinition, newDefinition, 1)
	if content == string(data) {
		t.Fatalf("scaffold source %q is missing the expected extension definition", mainPath)
	}
	if err := os.WriteFile(mainPath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", mainPath, err)
	}
}

func writeExtensionKitE2EAutomation(t *testing.T, sourceDir string, jobName string) {
	t.Helper()
	body := fmt.Sprintf(`[[jobs]]
name = %q
agent = %q
prompt = "Write a kit report."
[jobs.schedule]
mode = "cron"
expr = "0 0 * * *"

[[triggers]]
name = "on-task"
agent = %q
prompt = "Handle the completed task."
event = "ext.task.completed"
`, jobName, extensionKitE2EAgentName, extensionKitE2EAgentName)
	writeExtensionKitE2EFile(t, sourceDir, "automation/main.toml", body)
}

func writeExtensionKitE2ELayout(t *testing.T, sourceDir string) {
	t.Helper()
	layout := windowmanager.LayoutResource{
		Version: windowmanager.LayoutResourceVersion, ID: "kit-e2e", DisplayName: "Kit E2E",
		Document: windowmanager.LayoutDocument{
			Version: windowmanager.SnapshotVersion,
			Desktops: []windowmanager.Desktop{{
				ID: "desktop-main", Name: "Main", Purpose: windowmanager.DesktopPurposeStandard,
				Groups: []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{},
			}},
			Windows: map[windowmanager.WindowID]windowmanager.Window{},
		},
	}
	raw, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("json.Marshal(layout) error = %v", err)
	}
	writeExtensionKitE2EFile(t, sourceDir, "layouts/kit-e2e.json", string(raw))
}

func writeExtensionKitE2EFile(t *testing.T, sourceDir string, relativePath string, content string) {
	t.Helper()
	path := filepath.Join(sourceDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func addExtensionKitE2ENetworkRequirement(
	t *testing.T,
	generationDir string,
	channelScope string,
) string {
	t.Helper()
	manifestPath := filepath.Join(generationDir, "extension.toml")
	file, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("os.OpenFile(%q) error = %v", manifestPath, err)
	}
	_, writeErr := fmt.Fprintf(file, `
[network_participation]
required = true
mode = "live"
channel_scopes = [%q]
`, channelScope)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("append network requirement write error = %v, close error = %v", writeErr, closeErr)
	}
	manifest, err := extensionpkg.LoadManifest(generationDir)
	if err != nil {
		t.Fatalf("extension.LoadManifest(%q) error = %v", generationDir, err)
	}
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		t.Fatalf("NetworkParticipationRequirementDigest() error = %v", err)
	}
	if strings.TrimSpace(digest) == "" {
		t.Fatal("network requirement digest is empty")
	}
	return digest
}

func setExtensionKitE2ESecret(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	extensionName string,
) {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDirWithInput(
		ctx,
		harness.WorkspaceRoot,
		strings.NewReader(extensionKitE2ESecret+"\n"),
		"extension", "secrets", "set", extensionName,
		"--env", extensionKitE2EEnvName, "--value-stdin", "-o", "json",
	)
	if err != nil {
		t.Fatalf("extension secrets set error = %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
	if strings.Contains(stdout, extensionKitE2ESecret) || strings.Contains(stderr, extensionKitE2ESecret) {
		t.Fatalf("extension secrets set leaked the secret; stdout=%s stderr=%s", stdout, stderr)
	}
}
