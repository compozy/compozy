//go:build integration && !windows

package daemon

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const (
	softwareDeliveryE2ESlug         = "legacy-delivery"
	softwareDeliveryImplementer     = "code_implementer"
	softwareDeliveryReviewer        = "reviewer"
	softwareDeliveryFixtureAgent    = "software_delivery_implementer"
	softwareDeliveryReviewerFixture = "software_delivery_reviewer"
)

func TestDaemonE2ESoftwareDeliveryShouldCompleteLegacyUserJourney(t *testing.T) {
	t.Parallel()

	driverPath := acpmock.RequireDriver(t)
	homePaths := e2etest.NewHomePaths(t)
	workspaceRoot := filepath.Join(t.TempDir(), "software-delivery-workspace")
	fixturePath := mockFixturePath(t, "software_delivery_fixture.json")
	seedSoftwareDeliveryTaskTree(t, workspaceRoot)

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		HomePaths: homePaths,
		Workspace: e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    softwareDeliveryImplementer,
			DefaultProvider: acpmock.ProviderName,
			PermissionMode:  config.PermissionModeApproveAll,
			Mutate: func(cfg *config.Config) {
				acpMockProvider := acpmock.ProviderConfig(driverPath)
				acpMockProvider.Models.Reasoning.Apply = config.ReasoningApplyACPOption
				cfg.Providers[acpmock.ProviderName] = acpMockProvider
				claudeProvider := acpmock.ProviderConfig(acpmock.BuildCommand(
					driverPath,
					fixturePath,
					softwareDeliveryFixtureAgent,
					filepath.Join(homePaths.LogsDir, "software-delivery-claude.jsonl"),
				))
				claudeProvider.Models.Reasoning.Apply = config.ReasoningApplyACPOption
				cfg.Providers["claude"] = claudeProvider
			},
		},
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:  fixturePath,
				FixtureAgent: softwareDeliveryFixtureAgent,
				AgentName:    softwareDeliveryImplementer,
			},
			{
				FixturePath:  fixturePath,
				FixtureAgent: softwareDeliveryReviewerFixture,
				AgentName:    softwareDeliveryReviewer,
			},
		},
		StartTimeout: 30 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	waitForLoopCatalogEntry(t, ctx, harness, "software-delivery")

	stdout, stderr, err := harness.CLI.RunInDir(
		ctx,
		workspaceRoot,
		"loop", "run",
		"--workspace", workspaceRoot,
		"--name", "software-delivery",
		"--input", "slug="+softwareDeliveryE2ESlug,
		"--runtime", "type=frontend:claude/opus",
	)
	if err != nil {
		t.Fatalf("CLI software-delivery run error = %v; stderr=%s", err, strings.TrimSpace(stderr))
	}
	webURL, runID := softwareDeliveryRunURL(t, harness, stdout)
	if !strings.HasSuffix(strings.TrimSpace(stdout), webURL) {
		t.Fatalf("CLI software-delivery output = %q, want web URL as final line", stdout)
	}

	waitForLoopRunStatus(t, ctx, harness, runID, contract.LoopRunStatusDone)

	var detail contract.LoopRunResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		workspaceRoot,
		&detail,
		"loop", "status",
		"--workspace", workspaceRoot,
		"--run-id", runID,
		"-o", "json",
	); err != nil {
		t.Fatalf("CLI software-delivery status error = %v", err)
	}
	assertSoftwareDeliveryRuntimes(t, detail)
}

func seedSoftwareDeliveryTaskTree(t testing.TB, workspaceRoot string) {
	t.Helper()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", softwareDeliveryE2ESlug)
	files := map[string]string{
		"_tasks.md": `---
schema_version: "compozy.tasks/v2"
workflow: legacy-delivery
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

# Legacy delivery tasks
`,
		"task_01.md": `---
status: pending
title: Frontend delivery
type: frontend
complexity: high
---

# Frontend delivery
`,
		"task_02.md": `---
status: pending
title: Documentation delivery
type: docs
complexity: medium
runtime:
  provider: acpmock
  model: docs-model
  reasoning: high
---

# Documentation delivery
`,
		"task_03.md": `---
status: pending
title: Backend delivery
type: backend
complexity: low
---

# Backend delivery
`,
	}
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", tasksDir, err)
	}
	for name, content := range files {
		path := filepath.Join(tasksDir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}
}

func softwareDeliveryRunURL(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	stdout string,
) (string, string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("CLI software-delivery output = %q, want summary and final URL", stdout)
	}
	webURL := strings.TrimSpace(lines[len(lines)-1])
	parsed, err := url.Parse(webURL)
	if err != nil {
		t.Fatalf("parse software-delivery URL %q error = %v", webURL, err)
	}
	if got := parsed.Scheme + "://" + parsed.Host; got != harness.HTTPBaseURL {
		t.Fatalf("software-delivery URL base = %q, want %q", got, harness.HTTPBaseURL)
	}
	runID, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/loop-runs/"))
	if err != nil {
		t.Fatalf("unescape software-delivery run ID error = %v", err)
	}
	if runID == "" || parsed.Path != fmt.Sprintf(contract.LoopRunWebRoute, runID) {
		t.Fatalf("software-delivery URL path = %q, want pinned route for run %q", parsed.Path, runID)
	}
	return webURL, runID
}

func assertSoftwareDeliveryRuntimes(t testing.TB, detail contract.LoopRunResponse) {
	t.Helper()
	got := make(map[int]contract.LoopResolvedRuntime, 3)
	for _, generation := range detail.Generations {
		for _, output := range generation.Outputs {
			if output.NodeID == "execute_task" && output.ResolvedRuntime != nil {
				got[output.ItemIndex] = *output.ResolvedRuntime
			}
		}
	}
	want := map[int]contract.LoopResolvedRuntime{
		0: {
			Provider: "claude", Model: "opus",
			Source: contract.LoopRuntimeProvenance{Provider: "run", Model: "run"},
		},
		1: {
			Provider: acpmock.ProviderName, Model: "docs-model", Reasoning: "high",
			Source: contract.LoopRuntimeProvenance{
				Provider: "frontmatter", Model: "frontmatter", Reasoning: "frontmatter",
			},
		},
		2: {
			Provider: acpmock.ProviderName, Model: "base-model", Reasoning: "low",
			Source: contract.LoopRuntimeProvenance{Provider: "agent", Model: "agent", Reasoning: "agent"},
		},
	}
	if len(got) != len(want) {
		t.Fatalf("software-delivery resolved runtimes = %#v, want three task rows", got)
	}
	for itemIndex, expected := range want {
		loopRuntimeAssertJSONEqual(
			t,
			fmt.Sprintf("software-delivery item %d runtime", itemIndex),
			got[itemIndex],
			expected,
		)
	}
}
