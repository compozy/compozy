package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/core"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/agents/mcpserver"
	"github.com/compozy/compozy/internal/core/model"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func TestExecCommandPassesSelectedAgentIntoWorkflowConfig(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	var got core.Config
	state := newCommandState(commandKindExec, core.ModeExec)
	state.runWorkflow = func(_ context.Context, cfg core.Config) error {
		got = cfg
		return nil
	}

	cmd := newExecTestCommand(state)
	err := cmd.Flags().Set("agent", "council")
	if err != nil {
		t.Fatalf("set agent flag: %v", err)
	}
	_, err = executeCommandCombinedOutput(
		cmd,
		nil,
		"--dry-run",
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute exec command: %v", err)
	}

	if got.AgentName != "council" {
		t.Fatalf("expected selected agent to be forwarded, got %#v", got)
	}
	if got.Mode != core.ModeExec {
		t.Fatalf("expected exec mode, got %#v", got)
	}
	if got.ResolvedPromptText != "Summarize the repository state" {
		t.Fatalf("unexpected resolved prompt text: %#v", got)
	}
}

func TestExecCommandUnknownAgentReturnsActionableError(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	_, err := executeRootCommand("exec", "--dry-run", "--agent", "missing-agent", "Summarize the repository state")
	if err == nil {
		t.Fatal("expected unknown reusable agent error")
	}
	if !strings.Contains(err.Error(), "agent not found") {
		t.Fatalf("expected agent not found context, got %v", err)
	}
	if !strings.Contains(err.Error(), "compozy agents list") {
		t.Fatalf("expected actionable follow-up, got %v", err)
	}
}

func TestExecCommandExecuteWithAgentUsesResolvedRuntimeAndPreservesStdout(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeCLIWorkspaceAgent(
		t,
		workspaceRoot,
		"council",
		agentMarkdownForCLI(
			"Council",
			"Multi-advisor decision agent",
			"codex",
			"agent-model",
			"high",
			model.AccessModeDefault,
			"You are the council agent.",
		),
		"",
	)
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--persist",
		"--agent",
		"council",
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute agent-backed exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "Summarize the repository state" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	record := mustLoadLatestExecRunRecord(t, workspaceRoot)
	if record.Model != "agent-model" {
		t.Fatalf("expected agent model default, got %#v", record)
	}
	if record.ReasoningEffort != "high" {
		t.Fatalf("expected agent reasoning default, got %#v", record)
	}
	if record.AccessMode != model.AccessModeDefault {
		t.Fatalf("expected agent access default, got %#v", record)
	}
}

func TestExecCommandExecuteWithAgentPreservesExplicitRuntimeOverrides(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeCLIWorkspaceAgent(
		t,
		workspaceRoot,
		"council",
		agentMarkdownForCLI(
			"Council",
			"Multi-advisor decision agent",
			"codex",
			"agent-model",
			"high",
			model.AccessModeDefault,
			"You are the council agent.",
		),
		"",
	)
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--persist",
		"--agent",
		"council",
		"--model",
		"explicit-model",
		"--reasoning-effort",
		"low",
		"--access-mode",
		model.AccessModeFull,
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute agent-backed exec with overrides: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "Summarize the repository state" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	record := mustLoadLatestExecRunRecord(t, workspaceRoot)
	if record.Model != "explicit-model" {
		t.Fatalf("expected explicit model to win, got %#v", record)
	}
	if record.ReasoningEffort != "low" {
		t.Fatalf("expected explicit reasoning to win, got %#v", record)
	}
	if record.AccessMode != model.AccessModeFull {
		t.Fatalf("expected explicit access mode to win, got %#v", record)
	}
}

func TestAgentsListShowsWorkspaceAndGlobalSources(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeCLIGlobalAgent(
		t,
		homeDir,
		"global-reviewer",
		agentMarkdownForCLI(
			"Global Reviewer",
			"Global scope agent",
			"codex",
			"",
			"medium",
			model.AccessModeDefault,
			"Review globally.",
		),
		"",
	)
	writeCLIWorkspaceAgent(
		t,
		workspaceRoot,
		"workspace-reviewer",
		agentMarkdownForCLI(
			"Workspace Reviewer",
			"Workspace scope agent",
			"codex",
			"",
			"medium",
			model.AccessModeDefault,
			"Review locally.",
		),
		"",
	)
	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("agents", "list")
	if err != nil {
		t.Fatalf("execute agents list: %v\noutput:\n%s", err, output)
	}
	for _, want := range []string{
		"global-reviewer",
		"workspace-reviewer",
		"source: global",
		"source: workspace",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q\noutput:\n%s", want, output)
		}
	}
}

func TestAgentsInspectInvalidAgentPrintsValidationBeforeNonZero(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeCLIWorkspaceAgent(
		t,
		workspaceRoot,
		"planner",
		agentMarkdownForCLI(
			"Planner",
			"Agent with invalid MCP config",
			"codex",
			"",
			"medium",
			model.AccessModeDefault,
			"Plan the work.",
		),
		`{
  "mcpServers": {
    "github": {
      "command": "npx",
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}`,
	)
	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("agents", "inspect", "planner")
	if err == nil {
		t.Fatalf("expected invalid agent inspection to fail\noutput:\n%s", output)
	}
	var exitErr interface{ ExitCode() int }
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	for _, want := range []string{
		"Agent: planner",
		"Status: invalid",
		"Validation: FAILED",
		"GITHUB_TOKEN",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q\noutput:\n%s", want, output)
		}
	}
}

func TestAgentsInspectValidAgentReportsSourceRuntimeAndValidation(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeCLIWorkspaceAgent(
		t,
		workspaceRoot,
		"council",
		agentMarkdownForCLI(
			"Council",
			"Multi-advisor decision agent",
			"codex",
			"agent-model",
			"high",
			model.AccessModeDefault,
			"You are the council agent.",
		),
		`{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y"]
    }
  }
}`,
	)
	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("agents", "inspect", "council")
	if err != nil {
		t.Fatalf("execute agents inspect: %v\noutput:\n%s", err, output)
	}
	for _, want := range []string{
		"Agent: council",
		"Status: valid",
		"Source: workspace",
		"Runtime defaults: ide=codex model=agent-model reasoning=high access=default",
		"MCP servers: 1",
		"github: command=npx args=1 env_keys=-",
		"Validation: OK",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q\noutput:\n%s", want, output)
		}
	}
}

func TestHiddenMCPServeCommandHostsReservedServer(t *testing.T) {
	repoRoot := mustCLIRepoRootPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=TestCLIHelperProcessMCPServe",
		"--",
		"mcp-serve",
		"--server",
		reusableagents.ReservedMCPServerName,
	)
	cmd.Dir = repoRoot
	payload, err := json.Marshal(reusableagents.ReservedServerRuntimeContext{})
	if err != nil {
		t.Fatalf("marshal reserved host context: %v", err)
	}
	cmd.Env = append(
		os.Environ(),
		"GO_WANT_CLI_MCP_HELPER=1",
		reusableagents.RunAgentContextEnvVar+"="+string(payload),
	)

	client := mcp.NewClient(&mcp.Implementation{Name: "cli-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect to hidden mcp server: %v", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("expected one reserved tool, got %#v", result.Tools)
	}
	if result.Tools[0].Name != "run_agent" {
		t.Fatalf("unexpected tool list: %#v", result.Tools)
	}
}

func TestCLIHelperProcessMCPServe(_ *testing.T) {
	if os.Getenv("GO_WANT_CLI_MCP_HELPER") != "1" {
		return
	}

	args := os.Args
	index := -1
	for i := range args {
		if args[i] == "--" {
			index = i
			break
		}
	}
	if index < 0 || index+1 >= len(args) {
		os.Exit(2)
	}

	cmd := NewRootCommand()
	cmd.SetArgs(args[index+1:])
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func mustLoadLatestExecRunRecord(t *testing.T, workspaceRoot string) coreRun.PersistedExecRun {
	t.Helper()

	runDir := latestRunDirForCLI(t, workspaceRoot)
	record, err := coreRun.LoadPersistedExecRun(workspaceRoot, filepath.Base(runDir))
	if err != nil {
		t.Fatalf("load persisted exec run: %v", err)
	}
	return record
}

func TestBuildInspectAgentReportAndListHelpers(t *testing.T) {
	t.Parallel()

	valid := reusableagents.ResolvedAgent{
		Name: "council",
		Metadata: reusableagents.Metadata{
			Title:       "Council",
			Description: "Multi-advisor decision agent",
		},
		Runtime: reusableagents.RuntimeDefaults{
			IDE:             "codex",
			Model:           "gpt-5.4",
			ReasoningEffort: "high",
			AccessMode:      model.AccessModeDefault,
		},
		Source: reusableagents.Source{
			Scope:          reusableagents.ScopeWorkspace,
			Dir:            "/workspace/.compozy/agents/council",
			DefinitionPath: "/workspace/.compozy/agents/council/AGENT.md",
			MCPConfigPath:  "/workspace/.compozy/agents/council/mcp.json",
		},
		MCP: &reusableagents.MCPConfig{
			Path: "/workspace/.compozy/agents/council/mcp.json",
			Servers: []reusableagents.MCPServer{{
				Name:    "github",
				Command: "npx",
				Args:    []string{"-y"},
			}},
		},
	}
	invalid := reusableagents.Problem{
		Name: "planner",
		Source: reusableagents.Source{
			Scope:          reusableagents.ScopeGlobal,
			Dir:            "/home/.compozy/agents/planner",
			DefinitionPath: "/home/.compozy/agents/planner/AGENT.md",
			MCPConfigPath:  "/home/.compozy/agents/planner/mcp.json",
		},
		Err: fmt.Errorf(
			"%w: /home/.compozy/agents/planner/mcp.json references %q",
			reusableagents.ErrMissingEnvironmentVariable,
			"GITHUB_TOKEN",
		),
	}
	catalog := reusableagents.Catalog{
		Agents:   []reusableagents.ResolvedAgent{valid},
		Problems: []reusableagents.Problem{invalid},
	}

	report, err := buildInspectAgentReport(catalog, "council")
	if err != nil {
		t.Fatalf("build valid report: %v", err)
	}
	if report.Status != "valid" || report.ValidationError != nil {
		t.Fatalf("unexpected valid report: %#v", report)
	}

	report, err = buildInspectAgentReport(catalog, "planner")
	if err != nil {
		t.Fatalf("build invalid report: %v", err)
	}
	if report.Status != "invalid" || report.ValidationError == nil {
		t.Fatalf("unexpected invalid report: %#v", report)
	}

	_, err = buildInspectAgentReport(catalog, "missing")
	if err == nil {
		t.Fatal("expected missing agent error")
	}
	if !strings.Contains(err.Error(), "available agents: council, planner") {
		t.Fatalf("unexpected missing report error: %v", err)
	}

	var listOutput bytes.Buffer
	if err := writeAgentsListText(&listOutput, catalog); err != nil {
		t.Fatalf("write list output: %v", err)
	}
	for _, want := range []string{
		"resolved reusable agents: 1",
		"runtime: ide=codex model=gpt-5.4 reasoning=high access=default",
		"mcp: 1 server(s): github",
		"invalid reusable agent definitions: 1",
		"planner (global)",
	} {
		if !strings.Contains(listOutput.String(), want) {
			t.Fatalf("expected list output to contain %q\noutput:\n%s", want, listOutput.String())
		}
	}

	var inspectOutput bytes.Buffer
	if err := writeInspectAgentText(&inspectOutput, report); err != nil {
		t.Fatalf("write inspect output: %v", err)
	}
	if !strings.Contains(inspectOutput.String(), "Validation: FAILED") {
		t.Fatalf("expected invalid inspect output\noutput:\n%s", inspectOutput.String())
	}

	var emptyListOutput bytes.Buffer
	if err := writeAgentsListText(&emptyListOutput, reusableagents.Catalog{}); err != nil {
		t.Fatalf("write empty list output: %v", err)
	}
	if !strings.Contains(emptyListOutput.String(), "no reusable agents found") {
		t.Fatalf("unexpected empty list output: %s", emptyListOutput.String())
	}
}

func TestDecorateReusableAgentErrorAndHelperFormatting(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "exec"}
	root := &cobra.Command{Use: "compozy"}
	root.AddCommand(cmd)

	notFoundErr := decorateReusableAgentError(
		cmd,
		"missing-agent",
		fmt.Errorf("%w: %q", reusableagents.ErrAgentNotFound, "missing-agent"),
	)
	if !strings.Contains(notFoundErr.Error(), "compozy agents list") {
		t.Fatalf("expected list hint, got %v", notFoundErr)
	}

	invalidErr := decorateReusableAgentError(
		cmd,
		"planner",
		fmt.Errorf("%w: broken mcp", reusableagents.ErrMalformedMCPConfig),
	)
	if !strings.Contains(invalidErr.Error(), "compozy agents inspect planner") {
		t.Fatalf("expected inspect hint, got %v", invalidErr)
	}

	if !isReusableAgentValidationError(fmt.Errorf("%w: broken", reusableagents.ErrMalformedMCPConfig)) {
		t.Fatal("expected malformed mcp error to be classified as validation")
	}
	if isReusableAgentValidationError(errors.New("unrelated")) {
		t.Fatal("did not expect unrelated errors to be classified as validation")
	}

	if got := formatEnvKeys(nil); got != "-" {
		t.Fatalf("unexpected empty env keys: %q", got)
	}
	if got := formatEnvKeys(map[string]string{"B": "2", "A": "1"}); got != "A,B" {
		t.Fatalf("unexpected env keys ordering: %q", got)
	}
	if got := formatMCPSummary(nil); got != "none" {
		t.Fatalf("unexpected empty mcp summary: %q", got)
	}
	if got := formatMCPSummary(
		&reusableagents.MCPConfig{Servers: []reusableagents.MCPServer{{Name: "github"}, {Name: "filesystem"}}},
	); got != "2 server(s): github, filesystem" {
		t.Fatalf("unexpected mcp summary: %q", got)
	}
	if got := blankFallback("   "); got != "-" {
		t.Fatalf("unexpected blank fallback: %q", got)
	}
	if got := displayProblemName(
		reusableagents.Problem{Source: reusableagents.Source{Dir: "/tmp/reviewer"}},
	); got != "reviewer" {
		t.Fatalf("unexpected problem name fallback: %q", got)
	}
	if got := availableAgentNames(reusableagents.Catalog{
		Agents: []reusableagents.ResolvedAgent{{Name: "council"}},
		Problems: []reusableagents.Problem{
			{Name: "planner"},
			{Name: "council"},
		},
	}); strings.Join(got, ",") != "council,planner" {
		t.Fatalf("unexpected available names: %#v", got)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write mcp path: %v", err)
	}
	if got := optionalExistingPath(path); got != path {
		t.Fatalf("unexpected existing optional path: %q", got)
	}
	if got := optionalExistingPath(filepath.Join(dir, "missing.json")); got != "" {
		t.Fatalf("expected missing optional path to be blank, got %q", got)
	}
	notADirectory := filepath.Join(path, "nested.json")
	if got := optionalExistingPath(notADirectory); got != notADirectory {
		t.Fatalf("expected non-ENOENT optional path to remain visible, got %q", got)
	}
}

func TestAgentsCommandHelpersAndMCPServeState(t *testing.T) {
	t.Parallel()

	group := newAgentsCommand()
	if group.Use != "agents" {
		t.Fatalf("unexpected agents group: %#v", group)
	}
	if group.RunE == nil {
		t.Fatal("expected agents group help runner")
	}
	mcpServeCommand, _, findErr := group.Find([]string{"mcp-serve"})
	if findErr != nil {
		t.Fatalf("expected hidden mcp-serve command to be registered: %v", findErr)
	}
	if !mcpServeCommand.Hidden {
		t.Fatalf("expected mcp-serve command to remain hidden, got %#v", mcpServeCommand)
	}

	if registry := (&agentsListCommandState{}).registry(); registry == nil {
		t.Fatal("expected list registry fallback")
	}
	if registry := (&agentsInspectCommandState{}).registry(); registry == nil {
		t.Fatal("expected inspect registry fallback")
	}

	state := &mcpServeCommandState{
		serverName: reusableagents.ReservedMCPServerName,
		loadHostContext: func() (mcpserver.HostContext, error) {
			return mcpserver.HostContext{}, nil
		},
		serveStdio: func(context.Context, mcpserver.HostContext) error {
			return nil
		},
	}
	cmd := &cobra.Command{Use: "mcp-serve"}
	if err := state.run(cmd, nil); err != nil {
		t.Fatalf("expected successful hidden server state run: %v", err)
	}

	badState := &mcpServeCommandState{serverName: "other"}
	err := badState.run(cmd, nil)
	if err == nil {
		t.Fatal("expected unsupported server error")
	}
	var exitErr interface{ ExitCode() int }
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 for unsupported server, got %v", err)
	}
}

func TestIsReusableAgentValidationErrorCoversSupportedValidationKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "invalid name", err: fmt.Errorf("%w: bad", reusableagents.ErrInvalidAgentName), want: true},
		{name: "reserved name", err: fmt.Errorf("%w: compozy", reusableagents.ErrReservedAgentName), want: true},
		{
			name: "missing definition",
			err:  fmt.Errorf("%w: AGENT.md", reusableagents.ErrMissingAgentDefinition),
			want: true,
		},
		{
			name: "malformed frontmatter",
			err:  fmt.Errorf("%w: bad yaml", reusableagents.ErrMalformedFrontmatter),
			want: true,
		},
		{
			name: "unsupported metadata",
			err:  fmt.Errorf("%w: skills", reusableagents.ErrUnsupportedMetadataField),
			want: true,
		},
		{name: "invalid runtime", err: fmt.Errorf("%w: ide", reusableagents.ErrInvalidRuntimeDefaults), want: true},
		{name: "malformed mcp", err: fmt.Errorf("%w: json", reusableagents.ErrMalformedMCPConfig), want: true},
		{name: "missing env", err: fmt.Errorf("%w: TOKEN", reusableagents.ErrMissingEnvironmentVariable), want: true},
		{name: "reserved server", err: fmt.Errorf("%w: compozy", reusableagents.ErrReservedMCPServerName), want: true},
		{name: "other", err: errors.New("other"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isReusableAgentValidationError(tt.err); got != tt.want {
				t.Fatalf("isReusableAgentValidationError(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}

func newExecTestCommand(state *commandState) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "exec [prompt]",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         state.exec,
	}
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().StringVar(&state.agentName, "agent", "", "agent")
	cmd.Flags().StringVar(&state.promptFile, "prompt-file", "", "prompt file")
	cmd.Flags().StringVar(&state.outputFormat, "format", string(core.OutputFormatText), "format")
	cmd.Flags().BoolVar(&state.verbose, "verbose", false, "verbose")
	cmd.Flags().BoolVar(&state.tui, "tui", false, "tui")
	cmd.Flags().BoolVar(&state.persist, "persist", false, "persist")
	cmd.Flags().StringVar(&state.runID, "run-id", "", "run id")
	return cmd
}

func writeCLIWorkspaceAgent(t *testing.T, workspaceRoot, name, agentContent, mcpContent string) string {
	t.Helper()
	return writeCLIAgentFiles(
		t,
		filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents"),
		name,
		agentContent,
		mcpContent,
	)
}

func writeCLIGlobalAgent(t *testing.T, homeDir, name, agentContent, mcpContent string) string {
	t.Helper()
	return writeCLIAgentFiles(
		t,
		filepath.Join(homeDir, model.WorkflowRootDirName, "agents"),
		name,
		agentContent,
		mcpContent,
	)
}

func writeCLIAgentFiles(t *testing.T, root, name, agentContent, mcpContent string) string {
	t.Helper()

	agentDir := filepath.Join(root, name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(agentContent), 0o600); err != nil {
		t.Fatalf("write AGENT.md: %v", err)
	}
	if strings.TrimSpace(mcpContent) != "" {
		if err := os.WriteFile(filepath.Join(agentDir, "mcp.json"), []byte(mcpContent), 0o600); err != nil {
			t.Fatalf("write mcp.json: %v", err)
		}
	}
	return agentDir
}

func agentMarkdownForCLI(
	title string,
	description string,
	ide string,
	modelName string,
	reasoning string,
	accessMode string,
	prompt string,
) string {
	lines := []string{
		"---",
		"title: " + title,
		"description: " + description,
	}
	if strings.TrimSpace(ide) != "" {
		lines = append(lines, "ide: "+ide)
	}
	if strings.TrimSpace(modelName) != "" {
		lines = append(lines, "model: "+modelName)
	}
	if strings.TrimSpace(reasoning) != "" {
		lines = append(lines, "reasoning_effort: "+reasoning)
	}
	if strings.TrimSpace(accessMode) != "" {
		lines = append(lines, "access_mode: "+accessMode)
	}
	lines = append(lines, "---", "", prompt, "")
	return strings.Join(lines, "\n")
}
