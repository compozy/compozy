//go:build integration && !windows

package daemon

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const (
	implementTasksE2ESlug      = "implement-tasks"
	implementTasksImplementer  = "code_implementer"
	implementTasksCustomAgent  = "custom_implementer"
	implementTasksSentinel     = "CUSTOM_IMPLEMENTER_SENTINEL_V1"
	implementTasksFixtureAgent = "implement_tasks_implementer"
	implementTasksOrchestrator = "orchestrator"
	implementTasksConductor    = "implement_tasks_orchestrator"
)

func TestDaemonE2EImplementTasksShouldCompleteTaskJourney(t *testing.T) {
	t.Parallel()

	t.Run("Should complete the default per-task mode with category runtimes", func(t *testing.T) {
		t.Parallel()

		harness, ctx := startImplementTasksE2EHarness(t, implementTasksImplementer)
		detail := runImplementTasksE2E(t, ctx, harness, []string{
			"--input",
			`orchestrator_runtime={"provider":"acpmock","model":"base-model","reasoning":"high","speed":"normal"}`,
			"--input",
			`backend_runtime={"provider":"acpmock","model":"base-model","reasoning":"high","speed":"fast"}`,
			"--input",
			`frontend_runtime={"provider":"claude","model":"opus","reasoning":"high","speed":"normal"}`,
			"--input",
			`default_runtime={"provider":"claude","model":"base-model","reasoning":"low","speed":"normal"}`,
		})
		assertImplementTasksPerTaskRuntimes(t, detail)
		assertImplementTasksRoute(t, detail, "orchestrate", "route_not_taken:select_delivery")
	})

	t.Run("Should complete orchestrated mode and stop every category worker", func(t *testing.T) {
		t.Parallel()

		harness, ctx := startImplementTasksE2EHarness(t, implementTasksImplementer)
		detail := runImplementTasksE2E(t, ctx, harness, []string{"--input", "mode=orchestrated"})
		assertImplementTasksOrchestratorRuntimeFallback(t, detail)
		assertImplementTasksRoute(t, detail, "select_category", "route_not_taken:select_mode")
		assertImplementTasksSpawnedWorkerRuntimes(t, ctx, harness, implementTasksImplementer)
	})

	t.Run("Should use the selected Agent and its local skill in orchestrated mode", func(t *testing.T) {
		t.Parallel()

		harness, ctx := startImplementTasksE2EHarness(t, implementTasksCustomAgent)
		detail := runImplementTasksE2E(t, ctx, harness, []string{
			"--input", "mode=orchestrated",
			"--input", "implementer=" + implementTasksCustomAgent,
			"--input",
			`backend_runtime={"provider":"acpmock","model":"base-model","reasoning":"high","speed":"fast"}`,
			"--input",
			`frontend_runtime={"provider":"claude","model":"opus","reasoning":"high","speed":"normal"}`,
			"--input",
			`default_runtime={"provider":"acpmock","model":"docs-model","reasoning":"high","speed":"normal"}`,
		})
		assertImplementTasksOrchestratorRuntimeFallback(t, detail)
		assertImplementTasksRoute(t, detail, "select_category", "route_not_taken:select_mode")
		assertImplementTasksConductorPromptContains(t, harness,
			"Selected implementer Agent: `custom_implementer`.",
			`- backend: {"model":"base-model","provider":"acpmock","reasoning":"high","speed":"fast"}`,
			`- frontend: {"model":"opus","provider":"claude","reasoning":"high","speed":"normal"}`,
			`- default: {"model":"docs-model","provider":"acpmock","reasoning":"high","speed":"normal"}`,
		)
		assertImplementTasksSpawnedWorkerRuntimes(t, ctx, harness, implementTasksCustomAgent)
		assertImplementTasksWorkerPromptContains(t, harness, implementTasksSentinel)
	})
}

func startImplementTasksE2EHarness(
	t testing.TB,
	orchestratedImplementer string,
) (*e2etest.RuntimeHarness, context.Context) {
	t.Helper()

	driverPath := acpmock.RequireDriver(t)
	binaryPath := e2etest.BuildCompozyBinary(t)
	homePaths := e2etest.NewHomePaths(t)
	workspaceRoot := filepath.Join(t.TempDir(), "implement-tasks-workspace")
	fixturePath := materializeImplementTasksFixture(
		t,
		mockFixturePath(t, "implement_tasks_fixture.json"),
		binaryPath,
		orchestratedImplementer,
	)
	workerDiagnosticsPath := filepath.Join(homePaths.LogsDir, "implement-tasks-worker.jsonl")
	seedImplementTasksTree(t, workspaceRoot, driverPath, fixturePath, workerDiagnosticsPath)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		HomePaths:  homePaths,
		Workspace:  e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    implementTasksImplementer,
			DefaultProvider: acpmock.ProviderName,
			PermissionMode:  config.PermissionModeApproveAll,
			Mutate: func(cfg *config.Config) {
				acpMockProvider := acpmock.ProviderConfig(driverPath)
				acpMockProvider.Models.Reasoning.Apply = config.ReasoningApplyACPOption
				cfg.Providers[acpmock.ProviderName] = acpMockProvider
				claudeProvider := acpmock.ProviderConfig(acpmock.BuildCommand(
					driverPath,
					fixturePath,
					implementTasksFixtureAgent,
					filepath.Join(homePaths.LogsDir, "implement-tasks-claude.jsonl"),
				))
				claudeProvider.Models.Reasoning.Apply = config.ReasoningApplyACPOption
				cfg.Providers["claude"] = claudeProvider
			},
		},
		StartTimeout: 30 * time.Second,
	})
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	t.Cleanup(cancel)
	requireSpecCycleExtensionEnabled(t, ctx, harness)
	diagnostics := map[string]string{
		implementTasksFixtureAgent: workerDiagnosticsPath,
		implementTasksConductor:    filepath.Join(homePaths.LogsDir, "implement-tasks-conductor.jsonl"),
		"implement_tasks_claude":   filepath.Join(homePaths.LogsDir, "implement-tasks-claude.jsonl"),
	}
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		for name, path := range diagnostics {
			records, err := acpmock.ReadDiagnostics(path)
			if err != nil {
				t.Logf("read %s diagnostics: %v", name, err)
				continue
			}
			for _, record := range acpmock.PromptDiagnostics(records) {
				t.Logf(
					"%s prompt turn=%q meta=%#v match=%#v",
					name,
					record.TurnName,
					record.PromptMeta,
					record.Match,
				)
				for _, step := range record.Steps {
					t.Logf(
						"%s step kind=%q command=%q exit=%v",
						name,
						step.Kind,
						step.Command,
						step.ExitCode,
					)
					logDiagnosticChunks(t, name+" output", diagnosticTail(step.Output, 600))
					logDiagnosticChunks(t, name+" error", diagnosticTail(step.Error, 600))
				}
			}
		}
		for name, path := range map[string]string{
			"daemon":         homePaths.LogFile,
			"daemon-process": filepath.Join(harness.Artifacts.RootDir(), "daemon-process.log"),
		} {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Logf("read %s log: %v", name, err)
				continue
			}
			logDiagnosticChunks(t, name+" log", diagnosticTail(string(data), 5000))
		}
	})
	for _, agent := range []extensionAgentFixtureConfig{
		{
			DriverPath: driverPath, FixturePath: fixturePath,
			FixtureAgentName: implementTasksFixtureAgent, ExtensionAgentName: implementTasksImplementer,
			DiagnosticsPath: diagnostics[implementTasksFixtureAgent],
		},
		{
			DriverPath: driverPath, FixturePath: fixturePath,
			FixtureAgentName: implementTasksConductor, ExtensionAgentName: implementTasksOrchestrator,
			DiagnosticsPath: diagnostics[implementTasksConductor],
		},
	} {
		configureExtensionAgentFixture(t, ctx, harness, agent)
	}
	waitForLoopCatalogEntry(t, ctx, harness, "implement-tasks")
	return harness, ctx
}

func logDiagnosticChunks(t testing.TB, label string, value string) {
	t.Helper()
	if value == "" {
		return
	}
	const chunkSize = 80
	for len(value) > chunkSize {
		t.Logf("%s: %s", label, value[:chunkSize])
		value = value[chunkSize:]
	}
	t.Logf("%s: %s", label, value)
}

func diagnosticTail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}

func materializeImplementTasksFixture(
	t testing.TB,
	sourcePath string,
	binaryPath string,
	implementer string,
) string {
	t.Helper()
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", sourcePath, err)
	}
	source := string(data)
	if !strings.Contains(source, `"__COMPOZY_BINARY__"`) || !strings.Contains(source, "__IMPLEMENTER__") {
		t.Fatalf("implement-tasks fixture %q is missing a required placeholder", sourcePath)
	}
	rendered := strings.ReplaceAll(source, `"__COMPOZY_BINARY__"`, strconv.Quote(binaryPath))
	rendered = strings.ReplaceAll(rendered, "__IMPLEMENTER__", implementer)
	destination := filepath.Join(t.TempDir(), "implement_tasks_fixture.json")
	if err := os.WriteFile(destination, []byte(rendered), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", destination, err)
	}
	return destination
}

func runImplementTasksE2E(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	extraArgs []string,
) contract.LoopRunResponse {
	t.Helper()

	args := []string{
		"loop", "run", "--workspace", harness.WorkspaceRoot, "--name", "implement-tasks",
		"--input", "slug=" + implementTasksE2ESlug,
	}
	args = append(args, extraArgs...)
	stdout, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, args...)
	if err != nil {
		t.Fatalf("CLI implement-tasks run error = %v; stderr=%s", err, strings.TrimSpace(stderr))
	}
	webURL, runID := implementTasksRunURL(t, harness, stdout)
	if !strings.HasSuffix(strings.TrimSpace(stdout), webURL) {
		t.Fatalf("CLI implement-tasks output = %q, want web URL as final line", stdout)
	}
	waitForImplementTasksRunDone(t, ctx, harness, runID)
	var detail contract.LoopRunResponse
	if err := harness.CLI.RunJSONInDir(
		ctx, harness.WorkspaceRoot, &detail, "loop", "status",
		"--workspace", harness.WorkspaceRoot, "--run-id", runID, "-o", "json",
	); err != nil {
		t.Fatalf("CLI implement-tasks status error = %v", err)
	}
	return detail
}

func waitForImplementTasksRunDone(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var response contract.LoopRunResponse
		path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
			"/loop-runs/" + url.PathEscape(runID)
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			t.Fatalf("HTTP implement-tasks status error = %v", err)
		}
		if response.Run.Status == contract.LoopRunStatusDone {
			return
		}
		if loopRunStatusTerminal(response.Run.Status) {
			t.Fatalf(
				"implement-tasks reached terminal status %s; detail=%#v",
				response.Run.Status,
				response,
			)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait implement-tasks run %s: %v", runID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func seedImplementTasksTree(
	t testing.TB,
	workspaceRoot string,
	driverPath string,
	fixturePath string,
	diagnosticsPath string,
) {
	t.Helper()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", implementTasksE2ESlug)
	files := map[string]string{
		"_tasks.md": `---
schema_version: "compozy.tasks/v2"
workflow: implement-tasks
graph:
  nodes:
    - id: task_01
      file: task_01.md
    - id: task_02
      file: task_02.md
    - id: task_03
      file: task_03.md
  edges: []
---

# Implementation tasks
`,
		"task_01.md": `---
status: pending
title: Frontend implementation
type: frontend
complexity: high
---

# Frontend implementation
`,
		"task_02.md": `---
status: pending
title: Documentation implementation
type: docs
complexity: medium
runtime:
  provider: acpmock
  model: docs-model
  reasoning: high
---

# Documentation implementation
`,
		"task_03.md": `---
status: pending
title: Backend implementation
type: backend
complexity: low
---

# Backend implementation
`,
		".compozy/agents/" + implementTasksCustomAgent + "/AGENT.md": fmt.Sprintf(`---
name: %s
provider: %s
command: %s
model: base-model
reasoning_effort: low
permissions: approve-all
---

Implement the assigned task.
`, implementTasksCustomAgent, acpmock.ProviderName, quotedYAMLString(acpmock.BuildCommand(
			driverPath,
			fixturePath,
			implementTasksFixtureAgent,
			diagnosticsPath,
		))),
		".compozy/agents/" + implementTasksCustomAgent +
			"/skills/implementer-sentinel/SKILL.md": `---
name: implementer-sentinel
description: CUSTOM_IMPLEMENTER_SENTINEL_V1
---

# Implementer sentinel
`,
	}
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", tasksDir, err)
	}
	for name, content := range files {
		path := filepath.Join(tasksDir, name)
		if strings.HasPrefix(name, ".compozy/") {
			path = filepath.Join(workspaceRoot, filepath.FromSlash(name))
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}
}

func implementTasksRunURL(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	stdout string,
) (string, string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("CLI implement-tasks output = %q, want summary and final URL", stdout)
	}
	webURL := strings.TrimSpace(lines[len(lines)-1])
	parsed, err := url.Parse(webURL)
	if err != nil {
		t.Fatalf("parse implement-tasks URL %q error = %v", webURL, err)
	}
	if got := parsed.Scheme + "://" + parsed.Host; got != harness.HTTPBaseURL {
		t.Fatalf("implement-tasks URL base = %q, want %q", got, harness.HTTPBaseURL)
	}
	runID, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/loop-runs/"))
	if err != nil {
		t.Fatalf("unescape implement-tasks run ID error = %v", err)
	}
	if runID == "" || parsed.Path != fmt.Sprintf(contract.LoopRunWebRoute, runID) {
		t.Fatalf("implement-tasks URL path = %q, want pinned route for run %q", parsed.Path, runID)
	}
	return webURL, runID
}

func assertImplementTasksPerTaskRuntimes(t testing.TB, detail contract.LoopRunResponse) {
	t.Helper()
	got := make(map[int]contract.LoopResolvedRuntime, 3)
	for _, generation := range detail.Generations {
		for _, output := range generation.Outputs {
			if strings.HasPrefix(output.NodeID, "execute_") && output.ResolvedRuntime != nil {
				if existing, ok := got[output.ItemIndex]; ok {
					t.Fatalf(
						"implement-tasks item %d resolved twice: %#v then %s/%#v",
						output.ItemIndex,
						existing,
						output.NodeID,
						*output.ResolvedRuntime,
					)
				}
				got[output.ItemIndex] = *output.ResolvedRuntime
			}
		}
	}
	want := map[int]contract.LoopResolvedRuntime{
		0: {
			Provider: "claude", Model: "opus", Reasoning: "high", Speed: speed.SpeedNormal,
			SpeedResolution: unsupportedNormalSpeedResolution(),
			Source: contract.LoopRuntimeProvenance{
				Provider: "input", Model: "input", Reasoning: "input", Speed: "input",
			},
		},
		1: {
			Provider: acpmock.ProviderName, Model: "docs-model", Reasoning: "high", Speed: speed.SpeedNormal,
			SpeedResolution: unsupportedNormalSpeedResolution(),
			Source: contract.LoopRuntimeProvenance{
				Provider: "frontmatter", Model: "frontmatter", Reasoning: "frontmatter", Speed: "input",
			},
		},
		2: {
			Provider: acpmock.ProviderName, Model: "base-model", Reasoning: "high", Speed: speed.SpeedFast,
			SpeedResolution: unsupportedSpeedResolution(speed.SpeedFast),
			Source: contract.LoopRuntimeProvenance{
				Provider: "input", Model: "input", Reasoning: "input", Speed: "input",
			},
		},
	}
	if len(got) != len(want) {
		t.Fatalf("implement-tasks resolved runtimes = %#v, want three task rows", got)
	}
	for itemIndex, expected := range want {
		loopRuntimeAssertJSONEqual(
			t,
			fmt.Sprintf("implement-tasks item %d runtime", itemIndex),
			got[itemIndex],
			expected,
		)
	}
}

func assertImplementTasksOrchestratorRuntimeFallback(t testing.TB, detail contract.LoopRunResponse) {
	t.Helper()
	var got []contract.LoopResolvedRuntime
	for _, generation := range detail.Generations {
		for _, output := range generation.Outputs {
			if output.NodeID == "orchestrate" && output.ResolvedRuntime != nil {
				got = append(got, *output.ResolvedRuntime)
			}
		}
	}
	if len(got) != 1 {
		t.Fatalf("implement-tasks orchestrator resolved runtimes = %#v, want exactly one", got)
	}
	want := contract.LoopResolvedRuntime{
		Provider: acpmock.ProviderName, Model: "base-model", Reasoning: "low", Speed: speed.SpeedNormal,
		SpeedResolution: unsupportedNormalSpeedResolution(),
		Source: contract.LoopRuntimeProvenance{
			Provider: "agent", Model: "agent", Reasoning: "agent", Speed: "agent",
		},
	}
	loopRuntimeAssertJSONEqual(t, "implement-tasks orchestrator runtime", got[0], want)
}

func assertImplementTasksRoute(
	t testing.TB,
	detail contract.LoopRunResponse,
	nodeID string,
	wantOutputRef string,
) {
	t.Helper()
	for _, generation := range detail.Generations {
		for _, output := range generation.Outputs {
			if output.NodeID == nodeID && output.OutputRef == wantOutputRef {
				return
			}
		}
	}
	t.Fatalf("implement-tasks node %q did not record %q: %#v", nodeID, wantOutputRef, detail.Generations)
}

func assertImplementTasksSpawnedWorkerRuntimes(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	wantAgent string,
) {
	t.Helper()
	var page contract.SessionCatalogResponse
	if err := harness.CLI.RunJSONInDir(
		ctx, harness.WorkspaceRoot, &page,
		"session", "list", "--type", "spawned", "--state", "stopped",
		"--query", "orchestrate-implement-tasks-", "--limit", "10", "-o", "json",
	); err != nil {
		t.Fatalf("CLI spawned worker list error = %v", err)
	}
	want := map[string]contract.RuntimeSelectionPayload{
		"orchestrate-implement-tasks-task_01": {
			Provider: "claude", Model: "opus", ReasoningEffort: "high", Speed: speed.SpeedNormal,
		},
		"orchestrate-implement-tasks-task_02": {
			Provider: acpmock.ProviderName, Model: "docs-model", ReasoningEffort: "high", Speed: speed.SpeedNormal,
		},
		"orchestrate-implement-tasks-task_03": {
			Provider: acpmock.ProviderName, Model: "base-model", ReasoningEffort: "high", Speed: speed.SpeedFast,
		},
	}
	if len(page.Sessions) != len(want) {
		t.Fatalf("spawned implement-tasks workers = %#v, want three stopped workers", page.Sessions)
	}
	for _, worker := range page.Sessions {
		expected, ok := want[worker.Name]
		if !ok {
			t.Fatalf("unexpected spawned worker %q", worker.Name)
		}
		if worker.State != "stopped" || worker.Runtime.Effective == nil {
			t.Fatalf("spawned worker %q state/runtime = %q/%#v", worker.Name, worker.State, worker.Runtime)
		}
		if worker.AgentName != wantAgent {
			t.Fatalf("spawned worker %q agent = %q, want %q", worker.Name, worker.AgentName, wantAgent)
		}
		got := *worker.Runtime.Effective
		got.SpeedResolution = nil
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("spawned worker %q runtime = %#v, want %#v", worker.Name, got, expected)
		}
	}
	for _, state := range []string{"starting", "active", "stopping"} {
		var live contract.SessionCatalogResponse
		if err := harness.CLI.RunJSONInDir(
			ctx, harness.WorkspaceRoot, &live,
			"session", "list", "--type", "spawned", "--state", state,
			"--query", "orchestrate-implement-tasks-", "--limit", "10", "-o", "json",
		); err != nil {
			t.Fatalf("CLI %s worker list error = %v", state, err)
		}
		if len(live.Sessions) != 0 {
			t.Fatalf("spawned implement-tasks workers remain %s: %#v", state, live.Sessions)
		}
	}
}

func assertImplementTasksWorkerPromptContains(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(
		filepath.Join(harness.HomePaths.LogsDir, "implement-tasks-worker.jsonl"),
	)
	if err != nil {
		t.Fatalf("ReadDiagnostics(implement-tasks worker) error = %v", err)
	}
	for _, record := range acpmock.PromptDiagnostics(records) {
		if strings.Contains(record.Prompt, want) {
			return
		}
	}
	t.Fatalf("implement-tasks worker prompts missing Agent-local skill sentinel %q", want)
}

func assertImplementTasksConductorPromptContains(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	wants ...string,
) {
	t.Helper()
	records, err := acpmock.ReadDiagnostics(
		filepath.Join(harness.HomePaths.LogsDir, "implement-tasks-conductor.jsonl"),
	)
	if err != nil {
		t.Fatalf("ReadDiagnostics(implement-tasks conductor) error = %v", err)
	}
	prompts := acpmock.PromptDiagnostics(records)
	if len(prompts) != 1 {
		t.Fatalf("implement-tasks conductor prompts = %#v, want exactly one", prompts)
	}
	for _, want := range wants {
		if !strings.Contains(prompts[0].Prompt, want) {
			t.Fatalf("implement-tasks conductor prompt missing %q: %q", want, prompts[0].Prompt)
		}
	}
}

func unsupportedNormalSpeedResolution() *contract.SpeedResolution {
	return unsupportedSpeedResolution(speed.SpeedNormal)
}

func unsupportedSpeedResolution(requested speed.Speed) *contract.SpeedResolution {
	return &contract.SpeedResolution{
		Requested: requested,
		Status:    speed.ResolutionUnsupported,
		Reason:    speed.ReasonCapabilityAbsent,
	}
}
