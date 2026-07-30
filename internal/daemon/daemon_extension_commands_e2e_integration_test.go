//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"golang.org/x/sys/execabs"
)

const commandFixtureExtensionName = "cmd-fixture"

func TestDaemonE2EExtensionContributedCommandsPreserveToolPolicy(t *testing.T) {
	t.Run(
		"Should preserve tool policy for contributed commands",
		testDaemonE2EExtensionContributedCommandsPreserveToolPolicy,
	)
}

func testDaemonE2EExtensionContributedCommandsPreserveToolPolicy(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	acpmock.RequireDriver(t)
	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent: "command-operator",
			Mutate: func(cfg *compozyconfig.Config) {
				cfg.Extensions.Trust.AllowUnverified = true
				cfg.Tools.Policy.ExternalDefault = compozyconfig.ToolsExternalDefaultAsk
				cfg.Tools.Policy.TrustedSources = append(
					cfg.Tools.Policy.TrustedSources,
					"extension:"+commandFixtureExtensionName,
				)
			},
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "permission_env_fixture.json"),
			FixtureAgent: "approver",
			AgentName:    "command-operator",
		}},
		StartTimeout: 30 * time.Second,
	})

	sourceDir := filepath.Join(harness.WorkspaceRoot, commandFixtureExtensionName)
	fixtureDir := filepath.Join(repoRoot, "internal", "extension", "testdata", "command-fixture-go")
	if err := copyDirectory(fixtureDir, sourceDir); err != nil {
		t.Fatalf("copy command fixture error = %v", err)
	}
	configureExtensionAuthoringSDKReplaceWithoutShell(t, sourceDir, repoRoot)
	var build extensionpkg.BuildResult
	runExtensionAuthoringCLI(t, ctx, harness, &build, "extension", "build", sourceDir, "-o", "json")
	assertCommandFixtureDescribe(t, ctx, build.GenerationDir)
	approvedTool, ok := build.Manifest.Resources.Tools["approved"]
	if !ok || !approvedTool.RequiresInteraction {
		t.Fatalf("built approved command = %#v, want requires_interaction", approvedTool)
	}
	var installed compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&installed,
		"extension", "install", build.GenerationDir, "--allow-unverified", "--yes", "-o", "json",
	)
	if installed.Name != commandFixtureExtensionName || !installed.Enabled {
		t.Fatalf("installed command fixture = %#v", installed)
	}

	assertCommandFixtureDiscovery(t, ctx, harness)
	assertCommandFixtureFlatAndNestedExecution(t, ctx, harness)
	assertCommandFixtureApprovalParity(t, ctx, harness)
}

func assertCommandFixtureDescribe(t *testing.T, ctx context.Context, generationDir string) {
	t.Helper()
	command := execabs.CommandContext(ctx, filepath.Join(generationDir, "bin"), "__describe")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("command fixture describe error = %v", err)
	}
	var payload extensioncontract.DescribePayload
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("decode command fixture describe error = %v; output=%s", err, output)
	}
	for _, descriptor := range payload.Tools {
		if descriptor.Handler == "approved" && descriptor.RequiresInteraction {
			return
		}
	}
	t.Fatalf("command fixture describe tools = %#v, want approved requires_interaction", payload.Tools)
}

func assertCommandFixtureDiscovery(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"extension", "commands", commandFixtureExtensionName,
		"--workspace", harness.WorkspaceRoot,
	)
	if err != nil {
		t.Fatalf("extension commands human error = %v; stderr=%s", err, stderr)
	}
	for _, fragment := range []string{"review/", "Review fixture rounds", "review/fetch", "approved", "greet"} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("extension commands human output = %q, want %q", stdout, fragment)
		}
	}

	var commands []compozycontract.ExtensionCommandPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&commands,
		"extension", "commands", commandFixtureExtensionName,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if len(commands) != 3 {
		t.Fatalf("structured command discovery = %#v, want three leaves", commands)
	}
	if commands[0].Verb != "approved" || !commands[0].ApprovalRequired ||
		commands[1].Verb != "greet" || commands[2].Verb != "review/fetch" {
		t.Fatalf("structured command discovery = %#v", commands)
	}
}

func assertCommandFixtureFlatAndNestedExecution(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"extension", "exec", commandFixtureExtensionName,
		"--cmd", "greet", "--name", "Ada", "--workspace", harness.WorkspaceRoot,
	)
	if err != nil {
		t.Fatalf("extension exec greet human error = %v; stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "hello Ada #1") {
		t.Fatalf("extension exec greet human output = %q", stdout)
	}

	_, groupStderr, groupErr := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"extension", "exec", commandFixtureExtensionName,
		"--cmd", "review", "--workspace", harness.WorkspaceRoot,
	)
	if groupErr == nil || !strings.Contains(groupStderr, "review/fetch") {
		t.Fatalf("group execution error = %v; stderr=%q", groupErr, groupStderr)
	}

	var nested compozycontract.ToolInvokeResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&nested,
		"extension", "exec", commandFixtureExtensionName,
		"--cmd", "review/fetch", "--round", "2", "--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if nested.ToolID != "ext__cmd_fixture__review_fetch" || nested.Result.Preview != "review round 2 #2" {
		t.Fatalf("nested execution response = %#v", nested)
	}
}

func assertCommandFixtureApprovalParity(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	const sessionID = "command-policy-session"
	toolID := toolspkg.ToolID("ext__cmd_fixture__approved")
	var toolView compozycontract.ToolResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&toolView,
		"tool", "info", toolID.String(),
		"--workspace", harness.WorkspaceRoot,
		"--session", sessionID,
		"--agent", "command-operator",
		"-o", "json",
	)
	if !toolView.Tool.Descriptor.RequiresInteraction || !toolView.Tool.Decision.ApprovalRequired {
		t.Fatalf("approved command tool view = %#v, want interactive approval", toolView.Tool)
	}

	_, execDeniedStderr, execDeniedErr := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"extension", "exec", commandFixtureExtensionName,
		"--cmd", "approved", "--message", "ship",
		"--workspace", harness.WorkspaceRoot, "--session", sessionID,
		"--agent", "command-operator",
		"--approval-token", "not-approved",
	)
	if execDeniedErr == nil || !strings.Contains(strings.ToLower(execDeniedStderr), "approval") {
		t.Fatalf("unapproved command error = %v; stderr=%q", execDeniedErr, execDeniedStderr)
	}
	_, toolDeniedStderr, toolDeniedErr := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"tool", "invoke", toolID.String(),
		"--input", `{"message":"ship"}`,
		"--workspace", harness.WorkspaceRoot,
		"--session", sessionID,
		"--agent", "command-operator",
		"--approval-token", "not-approved",
	)
	if toolDeniedErr == nil || strings.TrimSpace(toolDeniedStderr) != strings.TrimSpace(execDeniedStderr) {
		t.Fatalf(
			"unapproved parity: exec error=%v stderr=%q; tool error=%v stderr=%q",
			execDeniedErr,
			execDeniedStderr,
			toolDeniedErr,
			toolDeniedStderr,
		)
	}

	var approval compozycontract.ToolApprovalResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&approval,
		"tool", "approve", toolID.String(),
		"--input", `{"message":"ship"}`,
		"--workspace", harness.WorkspaceRoot,
		"--session", sessionID,
		"--agent", "command-operator",
		"-o", "json",
	)
	if strings.TrimSpace(approval.Approval.ApprovalToken) == "" {
		t.Fatalf("tool approval response = %#v, want token", approval)
	}

	var approved compozycontract.ToolInvokeResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&approved,
		"extension", "exec", commandFixtureExtensionName,
		"--cmd", "approved", "--message", "ship",
		"--workspace", harness.WorkspaceRoot,
		"--session", sessionID,
		"--agent", "command-operator",
		"--approval-token", approval.Approval.ApprovalToken,
		"-o", "json",
	)
	if approved.ToolID != toolID || approved.Result.Preview != "approved ship #3" {
		t.Fatalf("approved execution response = %#v", approved)
	}

	var toolApproval compozycontract.ToolApprovalResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&toolApproval,
		"tool", "approve", toolID.String(),
		"--input", `{"message":"ship"}`,
		"--workspace", harness.WorkspaceRoot,
		"--session", sessionID,
		"--agent", "command-operator",
		"-o", "json",
	)
	var toolApproved compozycontract.ToolInvokeResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&toolApproved,
		"tool", "invoke", toolID.String(),
		"--input", `{"message":"ship"}`,
		"--workspace", harness.WorkspaceRoot,
		"--session", sessionID,
		"--agent", "command-operator",
		"--approval-token", toolApproval.Approval.ApprovalToken,
		"-o", "json",
	)
	if toolApproved.ToolID != toolID || toolApproved.Result.Preview != "approved ship #4" {
		t.Fatalf("approved tool invocation response = %#v", toolApproved)
	}
}
