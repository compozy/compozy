package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/setup"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	"github.com/spf13/cobra"
)

var cliProcessIOMu sync.Mutex
var originalCLIHome = os.Getenv("HOME")

func TestMigrateCommandExecuteDirectReportsUnmappedTypeFollowUp(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"domain: backend",
			"type: Feature Implementation",
			"scope: full",
			"complexity: low",
		},
		"# Task 1: Needs Classification",
	))

	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("migrate", "--tasks-dir", tasksDir)
	if err != nil {
		t.Fatalf("execute migrate: %v\noutput:\n%s", err, output)
	}
	if !containsAll(output,
		"V1->V2 migrated: 1",
		"Unmapped type files: 1",
		"Fix prompt:",
		"type value is unmapped; must be one of:",
	) {
		t.Fatalf("unexpected migrate output:\n%s", output)
	}
}

func TestTasksValidateCommandExecuteDirectCoversFailureAndSuccess(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Missing Title",
	))

	withWorkingDir(t, workspaceRoot)

	output, err := executeRootCommand("tasks", "validate", "--tasks-dir", tasksDir)
	if err == nil {
		t.Fatalf("expected validation failure\noutput:\n%s", output)
	}
	if !containsAll(output, "task validation failed", "Fix prompt:", "title is required") {
		t.Fatalf("unexpected invalid validation output:\n%s", output)
	}

	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Missing Title",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Missing Title",
	))

	output, err = executeRootCommand("tasks", "validate", "--tasks-dir", tasksDir)
	if err != nil {
		t.Fatalf("expected validation success: %v\noutput:\n%s", err, output)
	}
	if output != "all tasks valid (1 scanned)\n" {
		t.Fatalf("unexpected validation success output: %q", output)
	}
}

func TestExecCommandExecuteDirectPromptIsEphemeralByDefault(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if got := strings.TrimSpace(stdout); got != "Summarize the repository state" {
		t.Fatalf("unexpected exec stdout: %q", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run exec, got %q", stderr)
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
}

func TestRunsPurgeCommandRemovesTerminalRunArtifactsOldestFirst(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := os.WriteFile(
		paths.ConfigFile,
		[]byte("[runs]\nkeep_terminal_days = 14\nkeep_max = 1\n"),
		0o600,
	); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	db, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workspace marker: %v", err)
	}
	workspaceRow, err := db.Register(context.Background(), workspaceRoot, "purge-workspace")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	for _, item := range []struct {
		runID   string
		status  string
		endedAt time.Time
	}{
		{runID: "run-oldest", status: "completed", endedAt: now.AddDate(0, 0, -21)},
		{runID: "run-middle", status: "failed", endedAt: now.AddDate(0, 0, -15)},
		{runID: "run-newest", status: "crashed", endedAt: now.AddDate(0, 0, -1)},
		{runID: "run-active", status: "running", endedAt: now},
	} {
		seedCLIRunForPurge(t, db, workspaceRow.ID, item.runID, item.status, item.endedAt)
		createCLIRunArtifacts(t, item.runID)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	withWorkingDir(t, t.TempDir())
	output, err := executeRootCommand("runs", "purge")
	if err != nil {
		t.Fatalf("execute runs purge: %v\noutput:\n%s", err, output)
	}
	if strings.TrimSpace(output) != "purged 2 run(s)" {
		t.Fatalf("unexpected runs purge output: %q", output)
	}

	reopened, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer func() {
		_ = reopened.Close()
	}()

	for _, runID := range []string{"run-oldest", "run-middle"} {
		if _, err := reopened.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
		}
		assertCLIRunDirMissing(t, runID)
	}
	for _, runID := range []string{"run-newest", "run-active"} {
		if _, err := reopened.GetRun(context.Background(), runID); err != nil {
			t.Fatalf("GetRun(%q) error = %v", runID, err)
		}
		assertCLIRunDirPresent(t, runID)
	}
}

func TestExecCommandWithInstalledWorkspaceExtensionStaysEphemeralWithoutFlag(t *testing.T) {
	workspaceRoot, recordPath := prepareWorkspaceExtensionFixtureForCLI(t, "normal")
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeDaemonBackedCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"exec",
		"--dry-run",
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute exec without extensions: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Summarize the repository state" {
		t.Fatalf("unexpected exec stdout: %q", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for exec without extensions, got %q", stderr)
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
	if _, statErr := os.Stat(recordPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected extension record file to remain absent, got stat err=%v", statErr)
	}
}

func TestExecCommandWithExtensionsFlagSpawnsWorkspaceExtensionAndWritesAudit(t *testing.T) {
	workspaceRoot, recordPath := prepareWorkspaceExtensionFixtureForCLI(t, "normal")
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeDaemonBackedCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"exec",
		"--extensions",
		"--dry-run",
		"Summarize the repository state",
	)
	if err != nil {
		t.Fatalf("execute exec with extensions: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Summarize the repository state" {
		t.Fatalf("unexpected exec stdout: %q", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for exec with extensions, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	records := readMockExtensionRecordsForCLI(t, recordPath)
	assertMockExtensionRecordKinds(t, records, "initialize_request", "shutdown")

	auditPath := filepath.Join(runDir, extensions.AuditLogFileName)
	auditContent, readErr := os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatalf("read extension audit log: %v", readErr)
	}
	if !strings.Contains(string(auditContent), `"method":"initialize"`) {
		t.Fatalf("expected audit log to include initialize, got:\n%s", string(auditContent))
	}
}

func TestExecCommandExecutePromptFileJSONEmitsJSONLByDefault(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	promptPath := filepath.Join(workspaceRoot, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("Prompt from file\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--prompt-file",
		promptPath,
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute exec json: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected json exec to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 2 {
		t.Fatalf("expected two jsonl events, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.started" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1]["type"] != "run.succeeded" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
	if output, ok := events[1]["output"].(string); !ok || output != "Prompt from file\n" {
		t.Fatalf("unexpected final output payload: %#v", events[1])
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
}

func TestExecCommandExecutePersistCreatesTurnArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--persist",
		"Persist this prompt",
	)
	if err != nil {
		t.Fatalf("execute persisted exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "Persist this prompt" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run persisted exec, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	for _, relPath := range []string{
		"run.json",
		"events.jsonl",
		filepath.Join("turns", "0001", "prompt.md"),
		filepath.Join("turns", "0001", "response.txt"),
		filepath.Join("turns", "0001", "result.json"),
	} {
		if _, statErr := os.Stat(filepath.Join(runDir, relPath)); statErr != nil {
			t.Fatalf("expected persisted exec artifact %s: %v", relPath, statErr)
		}
	}
}

func TestExecCommandExecuteRunIDUsesPersistedRuntimeDefaults(t *testing.T) {
	workspaceRoot := t.TempDir()
	prepareInProcessCLIDaemonHome(t)
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)
	resolvedWorkspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", workspaceRoot, err)
	}

	runID := "exec-resume"
	writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
		Version:         1,
		Mode:            model.ModeExec,
		RunID:           runID,
		Status:          execStatusSucceeded,
		WorkspaceRoot:   resolvedWorkspaceRoot,
		IDE:             model.IDECodex,
		Model:           "gpt-5-codex",
		ReasoningEffort: "high",
		AccessMode:      model.AccessModeDefault,
		AddDirs:         []string{filepath.Join(workspaceRoot, "docs")},
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TurnCount:       1,
		ACPSessionID:    "sess-existing",
	})

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--dry-run",
		"--run-id",
		runID,
		"Resume this conversation",
	)
	if err != nil {
		t.Fatalf("execute resumed exec dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "Resume this conversation" {
		t.Fatalf("unexpected resumed dry-run stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for resumed dry-run exec, got %q", stderr)
	}
}

func TestExecCommandExecutePersistedAgentParentChildEmitsReusableAgentLifecycleEvents(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeReusableAgentForCLI(t, workspaceRoot, "parent", strings.Join([]string{
		"---",
		"title: Parent",
		"description: Parent agent",
		"ide: codex",
		"---",
		"",
		"Parent prompt.",
		"",
	}, "\n"), `{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`)
	writeReusableAgentForCLI(t, workspaceRoot, "child", strings.Join([]string{
		"---",
		"title: Child",
		"description: Child agent",
		"ide: codex",
		"---",
		"",
		"Child prompt.",
		"",
	}, "\n"), "")
	withWorkingDir(t, workspaceRoot)

	restore := coreRun.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &cliCapturingACPClient{
				createSessionFn: func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
					return newCLIACPTestSession(
						"sess-parent",
						agent.SessionIdentity{ACPSessionID: "sess-parent"},
						[]model.SessionUpdate{
							{
								Kind:          model.UpdateKindToolCallStarted,
								ToolCallID:    "tool-1",
								ToolCallState: model.ToolCallStatePending,
								Blocks: []model.ContentBlock{mustCLIContentBlock(t, model.ToolUseBlock{
									ID:       "tool-1",
									Name:     "run_agent",
									ToolName: "run_agent",
									Input:    json.RawMessage(`{"name":"child","input":"delegate this"}`),
								})},
								Status: model.StatusRunning,
							},
							{
								Kind:          model.UpdateKindToolCallUpdated,
								ToolCallID:    "tool-1",
								ToolCallState: model.ToolCallStateCompleted,
								Blocks: []model.ContentBlock{mustCLIContentBlock(t, model.ToolResultBlock{
									ToolUseID: "tool-1",
									Content:   `{"name":"child","source":"workspace","run_id":"run-child","success":true,"parent_agent_name":"parent","depth":1,"max_depth":3}`,
								})},
								Status: model.StatusRunning,
							},
							{
								Kind: model.UpdateKindAgentMessageChunk,
								Blocks: []model.ContentBlock{
									mustCLIContentBlock(t, model.TextBlock{Text: "parent done"}),
								},
								Status: model.StatusRunning,
							},
						},
						nil,
					), nil
				},
			}, nil
		},
	)
	defer restore()

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--persist",
		"--agent",
		"parent",
		"Finish the task",
	)
	if err != nil {
		t.Fatalf("execute persisted agent exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "parent done") {
		t.Fatalf("expected parent output on stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for successful agent exec, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	lifecycleEvents := cliReusableAgentLifecyclePayloads(t, filepath.Join(runDir, "events.jsonl"))
	gotStages := make([]kinds.ReusableAgentLifecycleStage, 0, len(lifecycleEvents))
	for _, payload := range lifecycleEvents {
		gotStages = append(gotStages, payload.Stage)
	}
	wantStages := []kinds.ReusableAgentLifecycleStage{
		kinds.ReusableAgentLifecycleStageResolved,
		kinds.ReusableAgentLifecycleStagePromptAssembled,
		kinds.ReusableAgentLifecycleStageMCPMerged,
		kinds.ReusableAgentLifecycleStageNestedStarted,
		kinds.ReusableAgentLifecycleStageNestedCompleted,
	}
	if !slices.Equal(gotStages, wantStages) {
		t.Fatalf("unexpected reusable-agent lifecycle stages: got %v want %v", gotStages, wantStages)
	}
	if got, want := lifecycleEvents[2].MCPServers, []string{
		reusableagents.ReservedMCPServerName,
		"filesystem",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("unexpected merged MCP servers: got %v want %v", got, want)
	}
}

func TestExecCommandExecuteRunIDWithAgentReattachesMCPServersAndLifecycleEvents(t *testing.T) {
	workspaceRoot := t.TempDir()
	prepareInProcessCLIDaemonHome(t)
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeReusableAgentForCLI(t, workspaceRoot, "parent", strings.Join([]string{
		"---",
		"title: Parent",
		"description: Parent agent",
		"ide: codex",
		"---",
		"",
		"Parent prompt.",
		"",
	}, "\n"), `{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`)
	withWorkingDir(t, workspaceRoot)
	resolvedWorkspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", workspaceRoot, err)
	}

	runID := "exec-agent-resume"
	writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
		Version:         1,
		Mode:            model.ModeExec,
		RunID:           runID,
		Status:          execStatusSucceeded,
		WorkspaceRoot:   resolvedWorkspaceRoot,
		IDE:             model.IDECodex,
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		AccessMode:      model.AccessModeDefault,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TurnCount:       1,
		ACPSessionID:    "sess-existing",
	})

	var capturedResume agent.ResumeSessionRequest
	restore := coreRun.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &cliCapturingACPClient{
				resumeSessionFn: func(_ context.Context, req agent.ResumeSessionRequest) (agent.Session, error) {
					capturedResume = req
					return newCLIACPTestSession(
						"sess-existing",
						agent.SessionIdentity{ACPSessionID: "sess-existing", Resumed: true},
						[]model.SessionUpdate{
							{
								Kind: model.UpdateKindAgentMessageChunk,
								Blocks: []model.ContentBlock{
									mustCLIContentBlock(t, model.TextBlock{Text: "resumed parent"}),
								},
								Status: model.StatusRunning,
							},
						},
						nil,
					), nil
				},
			}, nil
		},
	)
	defer restore()

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--run-id",
		runID,
		"--agent",
		"parent",
		"Continue the session",
	)
	if err != nil {
		t.Fatalf("execute resumed agent exec: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "resumed parent") {
		t.Fatalf("expected resumed output on stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for resumed agent exec, got %q", stderr)
	}

	if got, want := len(capturedResume.MCPServers), 2; got != want {
		t.Fatalf("expected reserved plus agent-local MCP servers on resume, got %#v", capturedResume.MCPServers)
	}
	if got, want := capturedResume.MCPServers[0].Stdio.Name, reusableagents.ReservedMCPServerName; got != want {
		t.Fatalf("unexpected resumed reserved MCP server: %#v", capturedResume.MCPServers)
	}
	if got, want := capturedResume.MCPServers[1].Stdio.Name, "filesystem"; got != want {
		t.Fatalf("unexpected resumed agent-local MCP server: %#v", capturedResume.MCPServers)
	}

	lifecycleEvents := cliReusableAgentLifecyclePayloads(
		t,
		filepath.Join(persistedRunDirForCLI(t, workspaceRoot, runID), "events.jsonl"),
	)
	foundResumedMerge := false
	for _, payload := range lifecycleEvents {
		if payload.Stage == kinds.ReusableAgentLifecycleStageMCPMerged && payload.Resumed {
			foundResumedMerge = true
		}
	}
	if !foundResumedMerge {
		t.Fatalf("expected resumed MCP merge lifecycle event, got %#v", lifecycleEvents)
	}
}

func TestExecCommandExecuteAgentValidationFailureReportsInvalidMCPReason(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeReusableAgentForCLI(t, workspaceRoot, "broken", strings.Join([]string{
		"---",
		"title: Broken",
		"description: Broken agent",
		"ide: codex",
		"---",
		"",
		"Broken prompt.",
		"",
	}, "\n"), `{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"],"env":{"ROOT":"${MISSING_AGENT_ROOT}"}}}}`)
	withWorkingDir(t, workspaceRoot)

	_, _, err := executeDaemonBackedRootCommandCapturingProcessIO(t, nil, "exec", "--agent", "broken", "Do work")
	if err == nil {
		t.Fatal("expected invalid-mcp agent execution failure")
	}
	if !strings.Contains(err.Error(), "reusable agent blocked (invalid-mcp)") {
		t.Fatalf("expected invalid-mcp blocked reason in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "MISSING_AGENT_ROOT") {
		t.Fatalf("expected actionable env detail in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "agents inspect broken") {
		t.Fatalf("expected inspect follow-up in error, got %v", err)
	}
}

func TestExecCommandExecuteAgentWorkspaceOverrideWinsOverGlobalDefinition(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	writeReusableAgentForCLI(t, workspaceRoot, "reviewer", strings.Join([]string{
		"---",
		"title: Workspace Reviewer",
		"description: Workspace reviewer",
		"ide: codex",
		"---",
		"",
		"Workspace review prompt.",
		"",
	}, "\n"), "")
	writeGlobalReusableAgentForCLI(t, homeDir, "reviewer", strings.Join([]string{
		"---",
		"title: Global Reviewer",
		"description: Global reviewer",
		"ide: codex",
		"---",
		"",
		"Global review prompt.",
		"",
	}, "\n"), "")
	withWorkingDir(t, workspaceRoot)

	var capturedPrompt string
	restore := coreRun.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &cliCapturingACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
					capturedPrompt = string(req.Prompt)
					return newCLIACPTestSession(
						"sess-reviewer",
						agent.SessionIdentity{ACPSessionID: "sess-reviewer"},
						[]model.SessionUpdate{
							{
								Kind: model.UpdateKindAgentMessageChunk,
								Blocks: []model.ContentBlock{
									mustCLIContentBlock(t, model.TextBlock{Text: "review complete"}),
								},
								Status: model.StatusRunning,
							},
						},
						nil,
					), nil
				},
			}, nil
		},
	)
	defer restore()

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--agent",
		"reviewer",
		"Review the change",
	)
	if err != nil {
		t.Fatalf("execute agent override CLI run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "review complete") {
		t.Fatalf("expected successful reviewer output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for reviewer override run, got %q", stderr)
	}
	if !strings.Contains(capturedPrompt, "Workspace review prompt.") ||
		strings.Contains(capturedPrompt, "Global review prompt.") {
		t.Fatalf("expected workspace override prompt to win, got:\n%s", capturedPrompt)
	}
}

func TestExecCommandExecuteJSONMissingPromptEmitsFailureJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(t, nil, "exec", "--format", "json")
	if err == nil {
		t.Fatalf("expected exec json missing-prompt failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected json exec failure to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected one json failure event, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.failed" {
		t.Fatalf("unexpected failure event: %#v", events[0])
	}
	errorMessage, _ := events[0]["error"].(string)
	if !strings.Contains(errorMessage, "requires exactly one prompt source") {
		t.Fatalf("unexpected json error message: %#v", events[0])
	}
}

func TestExecCommandExecuteRawJSONMissingPromptEmitsFailureJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(t, nil, "exec", "--format", "raw-json")
	if err == nil {
		t.Fatalf("expected exec raw-json missing-prompt failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected raw-json exec failure to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected one raw-json failure event, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.failed" {
		t.Fatalf("unexpected failure event: %#v", events[0])
	}
	errorMessage, _ := events[0]["error"].(string)
	if !strings.Contains(errorMessage, "requires exactly one prompt source") {
		t.Fatalf("unexpected raw-json error message: %#v", events[0])
	}
}

func TestExecCommandExecuteJSONValidationFailureEmitsFailureJSON(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		nil,
		"exec",
		"--format",
		"json",
		"--tui",
		"Prompt for validation failure",
	)
	if err == nil {
		t.Fatalf("expected exec json validation failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected json exec validation failure to suppress stderr, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected one json validation failure event, got %d\nstdout:\n%s", len(events), stdout)
	}
	if events[0]["type"] != "run.failed" {
		t.Fatalf("unexpected validation failure event: %#v", events[0])
	}
	errorMessage, _ := events[0]["error"].(string)
	if !strings.Contains(errorMessage, "tui mode is not supported with json or raw-json output") {
		t.Fatalf("unexpected validation error message: %#v", events[0])
	}
}

func TestExecCommandExecuteStdinWorksEndToEnd(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	stdout, stderr, err := executeDaemonBackedRootCommandCapturingProcessIO(
		t,
		strings.NewReader("Prompt from stdin\n"),
		"exec",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute exec stdin: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if got := strings.TrimSpace(stdout); got != "Prompt from stdin" {
		t.Fatalf("unexpected stdin stdout: %q", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run stdin exec, got %q", stderr)
	}
	assertNoRunArtifactsForCLI(t, workspaceRoot)
}

func TestTasksRunCommandDispatchesResolvedWorkspaceAndConfiguredAttachMode(t *testing.T) {
	homeDir := isolateCLIConfigHome(t)
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	writeCLIGlobalConfig(t, homeDir, `
[runs]
default_attach_mode = "stream"
`)
	writeCLIWorkspaceConfig(t, workspaceRoot, `
[runs]
default_attach_mode = "detach"
`)

	nestedDir := filepath.Join(workspaceRoot, "pkg", "feature")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	chdirCLITest(t, nestedDir)

	staleClient := &stubDaemonCommandClient{
		target:    apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		healthErr: errors.New("dial unix /tmp/compozy-daemon.sock: connect: no such file or directory"),
	}
	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeDetach,
			StartedAt:        time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC),
		},
	}
	nowSequence := []time.Time{
		time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 13, 0, 0, 250000000, time.UTC),
	}
	nowIndex := 0
	nextNow := func() time.Time {
		if nowIndex >= len(nowSequence) {
			return nowSequence[len(nowSequence)-1]
		}
		value := nowSequence[nowIndex]
		nowIndex++
		return value
	}

	var launchCalls int
	var clientCalls int
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  nowSequence[0],
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			clientCalls++
			if clientCalls == 1 {
				return staleClient, nil
			}
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			launchCalls++
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            nextNow,
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"tasks",
		"run",
		"demo",
		"--dry-run",
		"--include-completed",
	)
	if err != nil {
		t.Fatalf("execute tasks run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "task run started: run-task-001 (mode=detach)") {
		t.Fatalf("unexpected tasks run stdout: %q", stdout)
	}
	if launchCalls != 1 {
		t.Fatalf("expected one daemon auto-bootstrap launch, got %d", launchCalls)
	}
	if readyClient.startCalls != 1 {
		t.Fatalf("expected one task run request, got %d", readyClient.startCalls)
	}
	if readyClient.startSlug != "demo" {
		t.Fatalf("unexpected workflow slug: %q", readyClient.startSlug)
	}
	if mustEvalSymlinksCLITest(t, readyClient.startRequest.Workspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf(
			"unexpected workspace root dispatched\nwant: %q\ngot:  %q",
			workspaceRoot,
			readyClient.startRequest.Workspace,
		)
	}
	if readyClient.startRequest.PresentationMode != attachModeDetach {
		t.Fatalf("unexpected presentation mode: %q", readyClient.startRequest.PresentationMode)
	}
	overrides := decodeTaskRunOverrides(t, readyClient.startRequest.RuntimeOverrides)
	if overrides.DryRun == nil || !*overrides.DryRun {
		t.Fatalf("expected dry-run override in request, got %#v", overrides)
	}
	if overrides.IncludeCompleted == nil || !*overrides.IncludeCompleted {
		t.Fatalf("expected include-completed override in request, got %#v", overrides)
	}
}

func TestTasksRunCommandAutoModeResolvesToStreamInNonInteractiveExecution(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-002",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeStream,
			StartedAt:        time.Date(2026, 4, 17, 13, 5, 0, 0, time.UTC),
		},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 5, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 5, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})
	var watchedRunID string
	installTestCLIRunObservers(
		t,
		nil,
		func(_ context.Context, dst io.Writer, _ daemonCommandClient, runID string) error {
			watchedRunID = runID
			_, err := io.WriteString(dst, "run completed | succeeded=1 failed=0 canceled=0\n")
			return err
		},
	)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"tasks",
		"run",
		"--name",
		"demo",
	)
	if err != nil {
		t.Fatalf("execute tasks run auto stream: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if readyClient.startRequest.PresentationMode != attachModeStream {
		t.Fatalf("expected auto attach mode to resolve to stream, got %q", readyClient.startRequest.PresentationMode)
	}
	if watchedRunID != "run-task-002" {
		t.Fatalf("expected stream watch to attach to run-task-002, got %q", watchedRunID)
	}
	if !strings.Contains(stdout, "task run started: run-task-002 (mode=stream)") ||
		!strings.Contains(stdout, "run completed | succeeded=1 failed=0 canceled=0") {
		t.Fatalf("unexpected tasks run stdout: %q", stdout)
	}
}

func TestTasksRunCommandPositionalSlugSkipsInteractiveFormWithoutTTY(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-slug-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeStream,
			StartedAt:        time.Date(2026, 4, 17, 13, 5, 15, 0, time.UTC),
		},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 5, 15, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 5, 15, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var watchedRunID string
	installTestCLIRunObservers(
		t,
		nil,
		func(_ context.Context, dst io.Writer, _ daemonCommandClient, runID string) error {
			watchedRunID = runID
			_, err := io.WriteString(dst, "run completed | succeeded=1 failed=0 canceled=0\n")
			return err
		},
	)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	defaults.collectForm = func(_ *cobra.Command, _ *commandState) error {
		t.Fatal("did not expect interactive form collection for positional task slug")
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"tasks",
		"run",
		"demo",
	)
	if err != nil {
		t.Fatalf("execute tasks run positional slug: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if readyClient.startSlug != "demo" {
		t.Fatalf("unexpected workflow slug: %q", readyClient.startSlug)
	}
	if readyClient.startRequest.PresentationMode != attachModeStream {
		t.Fatalf("expected auto attach mode to resolve to stream, got %q", readyClient.startRequest.PresentationMode)
	}
	if watchedRunID != "run-task-slug-001" {
		t.Fatalf("expected stream watch to attach to run-task-slug-001, got %q", watchedRunID)
	}
	if !strings.Contains(stdout, "task run started: run-task-slug-001 (mode=stream)") ||
		!strings.Contains(stdout, "run completed | succeeded=1 failed=0 canceled=0") {
		t.Fatalf("unexpected tasks run stdout: %q", stdout)
	}
}

func TestTasksRunCommandNoFlagsUsesInteractiveForm(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-form-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeUI,
			StartedAt:        time.Date(2026, 4, 17, 13, 5, 30, 0, time.UTC),
		},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 5, 30, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 5, 30, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var attachedRunID string
	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		attachedRunID = runID
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	var collectFormCalls int
	defaults.collectForm = func(_ *cobra.Command, state *commandState) error {
		collectFormCalls++
		state.name = "demo"
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "tasks", "run")
	if err != nil {
		t.Fatalf("execute tasks run no-flags form flow: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if collectFormCalls != 1 {
		t.Fatalf("expected one interactive form call, got %d", collectFormCalls)
	}
	if readyClient.startSlug != "demo" {
		t.Fatalf("unexpected workflow slug from form: %q", readyClient.startSlug)
	}
	if readyClient.startRequest.PresentationMode != attachModeUI {
		t.Fatalf("expected interactive attach mode to resolve to ui, got %q", readyClient.startRequest.PresentationMode)
	}
	if attachedRunID != "run-task-form-001" {
		t.Fatalf("expected ui attach for run-task-form-001, got %q", attachedRunID)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout before ui attach, got %q", stdout)
	}
}

func TestTasksRunCommandInteractiveUIModeAttachesThroughRemoteClient(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-ui-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeUI,
			StartedAt:        time.Date(2026, 4, 17, 13, 6, 0, 0, time.UTC),
		},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 6, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 6, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var attachedRunID string
	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		attachedRunID = runID
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"tasks",
		"run",
		"--name",
		"demo",
	)
	if err != nil {
		t.Fatalf("execute tasks run interactive ui: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if readyClient.startRequest.PresentationMode != attachModeUI {
		t.Fatalf("expected interactive attach mode to resolve to ui, got %q", readyClient.startRequest.PresentationMode)
	}
	if attachedRunID != "run-task-ui-001" {
		t.Fatalf("expected ui attach for run-task-ui-001, got %q", attachedRunID)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout before ui attach, got %q", stdout)
	}
}

func TestTasksRunCommandExplicitUIFailsWithoutTTY(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"tasks",
		"run",
		"demo",
		"--ui",
	)
	if err == nil {
		t.Fatalf("expected tasks run explicit ui failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on explicit ui failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "requires an interactive terminal for ui mode") {
		t.Fatalf("unexpected explicit ui error output: %q", stderr)
	}
}

func TestTasksRunCommandBootstrapFailureReturnsStableExitCode(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	withWorkingDir(t, workspaceRoot)

	nowSequence := []time.Time{
		time.Date(2026, 4, 17, 13, 10, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 13, 10, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 13, 10, 2, 0, time.UTC),
	}
	nowIndex := 0
	nextNow := func() time.Time {
		if nowIndex >= len(nowSequence) {
			return nowSequence[len(nowSequence)-1]
		}
		value := nowSequence[nowIndex]
		nowIndex++
		return value
	}

	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  nowSequence[0],
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return &stubDaemonCommandClient{
				target:    apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
				healthErr: errors.New("dial unix /tmp/compozy-daemon.sock: connect: connection refused"),
			}, nil
		},
		launch:         func(compozyconfig.HomePaths) error { return nil },
		sleep:          func(time.Duration) {},
		now:            nextNow,
		startupTimeout: 500 * time.Millisecond,
		pollInterval:   time.Millisecond,
	})

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"tasks",
		"run",
		"--name",
		"demo",
	)
	if err == nil {
		t.Fatalf("expected tasks run bootstrap failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	var exitErr *commandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("unexpected exit code: got %d want 2", exitErr.ExitCode())
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on bootstrap failure, got %q", stdout)
	}
	if !containsAll(stderr, "wait for daemon readiness", "probe daemon health via unix:///tmp/compozy-daemon.sock") {
		t.Fatalf("expected explicit daemon transport failure, got %q", stderr)
	}
}

func TestRunsAttachCommandUsesRemoteUIAttach(t *testing.T) {
	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 20, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 20, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var attachedRunID string
	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		attachedRunID = runID
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "runs", "attach", "run-attach-001")
	if err != nil {
		t.Fatalf("execute runs attach: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if attachedRunID != "run-attach-001" {
		t.Fatalf("expected attach for run-attach-001, got %q", attachedRunID)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet attach command, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRunsAttachCommandFallsBackToWatchWhenRunIsAlreadySettled(t *testing.T) {
	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 20, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 20, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var (
		attachedRunID string
		watchedRunID  string
	)
	installTestCLIRunObservers(
		t,
		func(_ context.Context, _ daemonCommandClient, runID string) error {
			attachedRunID = runID
			return errRunSettledBeforeUIAttach
		},
		func(_ context.Context, dst io.Writer, _ daemonCommandClient, runID string) error {
			watchedRunID = runID
			_, err := io.WriteString(dst, "run completed | completed\n")
			return err
		},
	)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "runs", "attach", "run-attach-001")
	if err != nil {
		t.Fatalf("execute runs attach settled fallback: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if attachedRunID != "run-attach-001" {
		t.Fatalf("expected attach attempt for run-attach-001, got %q", attachedRunID)
	}
	if watchedRunID != "run-attach-001" {
		t.Fatalf("expected watch fallback for run-attach-001, got %q", watchedRunID)
	}
	if stdout != "run completed | completed\n" || stderr != "" {
		t.Fatalf("expected replay stdout only, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRunsWatchCommandStreamsWithoutLaunchingUI(t *testing.T) {
	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 21, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 21, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var watchedRunID string
	var attachCalls int
	installTestCLIRunObservers(t, func(context.Context, daemonCommandClient, string) error {
		attachCalls++
		return nil
	}, func(_ context.Context, dst io.Writer, _ daemonCommandClient, runID string) error {
		watchedRunID = runID
		_, err := io.WriteString(dst, "run completed | all good\n")
		return err
	})

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "runs", "watch", "run-watch-001")
	if err != nil {
		t.Fatalf("execute runs watch: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if attachCalls != 0 {
		t.Fatalf("expected watch path to avoid ui attach, got %d attach calls", attachCalls)
	}
	if watchedRunID != "run-watch-001" {
		t.Fatalf("expected watch for run-watch-001, got %q", watchedRunID)
	}
	if stdout != "run completed | all good\n" || stderr != "" {
		t.Fatalf("unexpected runs watch output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestLegacyCommandsAreRemoved(t *testing.T) {
	testCases := []struct {
		name    string
		command string
	}{
		{name: "start", command: "start"},
		{name: "validate-tasks", command: "validate-tasks"},
		{name: "fetch-reviews", command: "fetch-reviews"},
		{name: "fix-reviews", command: "fix-reviews"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeRootCommand(tc.command, "--help")
			if err == nil {
				t.Fatalf("expected legacy %s command removal error\noutput:\n%s", tc.command, output)
			}

			expected := fmt.Sprintf("unknown command %q for %q", tc.command, "compozy")
			if !strings.Contains(output, expected) {
				t.Fatalf("unexpected legacy %s output:\n%s", tc.command, output)
			}
		})
	}
}

func TestReviewsFixCommandExecuteDryRunPersistsKernelArtifacts(t *testing.T) {
	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{{
		Title:       "Add nil check",
		File:        "internal/app/service.go",
		Line:        42,
		Author:      "coderabbitai[bot]",
		ProviderRef: "thread:PRT_1,comment:RC_1",
		Body:        "Please add a nil check before dereferencing the pointer.",
	}}); err != nil {
		t.Fatalf("write review round: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeDaemonBackedCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"reviews",
		"fix",
		"--name",
		"demo",
		"--round",
		"1",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute reviews fix dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run reviews fix, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	runMeta := readCLIArtifactJSON(t, filepath.Join(runDir, "run.json"))
	if got := runMeta["mode"]; got != string(model.ModeCodeReview) {
		t.Fatalf("unexpected review run mode: %#v", runMeta)
	}

	result := readCLIArtifactJSON(t, filepath.Join(runDir, "result.json"))
	if got := result["status"]; got != execStatusSucceeded {
		t.Fatalf("unexpected review result payload: %#v", result)
	}

	promptPath := singleCLIJobArtifact(t, runDir, "*.prompt.md")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read review prompt artifact: %v", err)
	}
	for _, want := range []string{"`cy-fix-reviews`", "issue_001.md", "internal/app/service.go"} {
		if !strings.Contains(string(promptBytes), want) {
			t.Fatalf("expected review prompt to contain %q, got:\n%s", want, string(promptBytes))
		}
	}

	eventKinds := cliRuntimeEventKinds(t, filepath.Join(runDir, "events.jsonl"))
	for _, want := range []eventspkg.EventKind{
		eventspkg.EventKindRunStarted,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindRunCompleted,
	} {
		if !slices.Contains(eventKinds, want) {
			t.Fatalf("expected runtime events to include %s, got %v", want, eventKinds)
		}
	}
}

func TestReviewsFetchCommandNoFlagsUsesInteractiveForm(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
			health: apicore.DaemonHealth{Ready: true},
			reviewFetch: apicore.ReviewFetchResult{
				Summary: apicore.ReviewSummary{
					WorkflowSlug:    "demo",
					RoundNumber:     1,
					Provider:        "coderabbit",
					PRRef:           "259",
					ResolvedCount:   0,
					UnresolvedCount: 1,
				},
			},
		},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 40, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return client, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 40, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	var collectFormCalls int
	defaults.collectForm = func(_ *cobra.Command, state *commandState) error {
		collectFormCalls++
		state.name = "demo"
		state.provider = "coderabbit"
		state.pr = "259"
		state.round = 1
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "reviews", "fetch")
	if err != nil {
		t.Fatalf("execute reviews fetch no-flags form flow: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if collectFormCalls != 1 {
		t.Fatalf("expected one interactive form call, got %d", collectFormCalls)
	}
	if mustEvalSymlinksCLITest(t, client.fetchWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("fetch workspace = %q, want %q", client.fetchWorkspace, workspaceRoot)
	}
	if mustEvalSymlinksCLITest(t, client.fetchReq.Workspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("fetch request workspace = %q, want %q", client.fetchReq.Workspace, workspaceRoot)
	}
	if client.fetchSlug != "demo" {
		t.Fatalf("fetch slug = %q, want demo", client.fetchSlug)
	}
	if client.fetchReq.Provider != "coderabbit" || client.fetchReq.PRRef != "259" ||
		client.fetchReq.Round == nil || *client.fetchReq.Round != 1 {
		t.Fatalf("unexpected fetch request: %#v", client.fetchReq)
	}
	if !containsAll(stdout, "Fetched review issues from coderabbit", "PR 259", "round 001") {
		t.Fatalf("unexpected reviews fetch stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for reviews fetch form flow, got %q", stderr)
	}
}

func TestReviewsFixCommandNoFlagsUsesInteractiveForm(t *testing.T) {
	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{{
		Title:       "Add nil check",
		File:        "internal/app/service.go",
		Line:        42,
		Author:      "coderabbitai[bot]",
		ProviderRef: "thread:PRT_1,comment:RC_1",
		Body:        "Please add a nil check before dereferencing the pointer.",
	}}); err != nil {
		t.Fatalf("write review round: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			target: apiclient.Target{SocketPath: "/tmp/compozy-daemon.sock"},
			health: apicore.DaemonHealth{Ready: true},
			reviewRun: apicore.Run{
				RunID:            "run-review-form-001",
				Mode:             string(core.ModePRReview),
				Status:           "running",
				PresentationMode: attachModeUI,
				StartedAt:        time.Date(2026, 4, 17, 13, 41, 0, 0, time.UTC),
			},
		},
	}
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				SocketPath: "/tmp/compozy-daemon.sock",
				StartedAt:  time.Date(2026, 4, 17, 13, 41, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return client, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			t.Fatal("expected healthy daemon probe to avoid launch")
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 17, 13, 41, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	var attachedRunID string
	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		attachedRunID = runID
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	var collectFormCalls int
	defaults.collectForm = func(_ *cobra.Command, state *commandState) error {
		collectFormCalls++
		state.name = "demo"
		state.round = 1
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "reviews", "fix")
	if err != nil {
		t.Fatalf("execute reviews fix no-flags form flow: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if collectFormCalls != 1 {
		t.Fatalf("expected one interactive form call, got %d", collectFormCalls)
	}
	if mustEvalSymlinksCLITest(t, client.startReviewWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("review run workspace = %q, want %q", client.startReviewWorkspace, workspaceRoot)
	}
	if mustEvalSymlinksCLITest(t, client.startReviewReq.Workspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("review run request workspace = %q, want %q", client.startReviewReq.Workspace, workspaceRoot)
	}
	if client.startReviewSlug != "demo" || client.startReviewRound != 1 {
		t.Fatalf(
			"unexpected review run target: slug=%q round=%d",
			client.startReviewSlug,
			client.startReviewRound,
		)
	}
	if client.startReviewReq.PresentationMode != attachModeUI {
		t.Fatalf("expected interactive attach mode to resolve to ui, got %q", client.startReviewReq.PresentationMode)
	}
	if attachedRunID != "run-review-form-001" {
		t.Fatalf("expected ui attach for run-review-form-001, got %q", attachedRunID)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet reviews fix form flow before ui attach, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestReviewsFixCommandExecuteDryRunRawJSONStreamsCanonicalEvents(t *testing.T) {
	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{{
		Title:       "Add nil check",
		File:        "internal/app/service.go",
		Line:        42,
		Author:      "coderabbitai[bot]",
		ProviderRef: "thread:PRT_1,comment:RC_1",
		Body:        "Please add a nil check before dereferencing the pointer.",
	}}); err != nil {
		t.Fatalf("write review round: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeDaemonBackedCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"reviews",
		"fix",
		"--name",
		"demo",
		"--round",
		"1",
		"--dry-run",
		"--format",
		"raw-json",
	)
	if err != nil {
		t.Fatalf("execute reviews fix raw-json dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for raw-json reviews fix, got %q", stderr)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) < 3 {
		t.Fatalf("expected multiple streamed canonical events, got %d\nstdout:\n%s", len(events), stdout)
	}
	if got := events[0]["kind"]; got != string(eventspkg.EventKindRunStarted) {
		t.Fatalf("unexpected first raw event: %#v", events[0])
	}
	if got := events[len(events)-1]["kind"]; got != string(eventspkg.EventKindRunCompleted) {
		t.Fatalf("unexpected terminal raw event: %#v", events[len(events)-1])
	}
	if got := events[0]["schema_version"]; got != eventspkg.SchemaVersion {
		t.Fatalf("unexpected schema version in raw event: %#v", events[0])
	}
	if _, ok := events[0]["type"]; ok {
		t.Fatalf("raw workflow stream should preserve canonical envelopes, got %#v", events[0])
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	eventKinds := cliRuntimeEventKinds(t, filepath.Join(runDir, "events.jsonl"))
	if got := eventKinds[len(eventKinds)-1]; got != eventspkg.EventKindRunCompleted {
		t.Fatalf("unexpected persisted terminal event: %v", eventKinds)
	}
}

func TestTaskAndReviewCommandsExecuteDryRunAgainstTempNodeWorkspace(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeCLINodeWorkflowFixture(t, workspaceRoot, "node-health")
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	chdirCLITest(t, workspaceRoot)

	installInProcessCLIDaemonBootstrap(t)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }

	syncStdout, syncStderr, err := executeCommandCapturingProcessIO(
		t,
		newRootCommandWithDefaults(newLazyRootDispatcher(), defaults),
		nil,
		"sync",
		"--name",
		"node-health",
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute sync for review fixture: %v\nstdout:\n%s\nstderr:\n%s", err, syncStdout, syncStderr)
	}
	var syncPayload struct {
		WorkflowSlug         string `json:"workflow_slug"`
		ReviewRoundsUpserted int    `json:"review_rounds_upserted"`
		ReviewIssuesUpserted int    `json:"review_issues_upserted"`
	}
	if err := json.Unmarshal([]byte(syncStdout), &syncPayload); err != nil {
		t.Fatalf("decode sync payload: %v\nstdout:\n%s", err, syncStdout)
	}
	if syncPayload.WorkflowSlug != "node-health" ||
		syncPayload.ReviewRoundsUpserted != 1 ||
		syncPayload.ReviewIssuesUpserted != 1 {
		t.Fatalf("unexpected sync payload: %#v", syncPayload)
	}

	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		newRootCommandWithDefaults(newLazyRootDispatcher(), defaults),
		nil,
		"tasks",
		"run",
		"node-health",
		"--dry-run",
		"--stream",
	)
	if err != nil {
		t.Fatalf("execute tasks run dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected task preflight success on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "task run started:") || !strings.Contains(stdout, "(mode=stream)") {
		t.Fatalf("unexpected tasks run stdout: %q", stdout)
	}

	runDirs := runDirsForCLI(t)
	if len(runDirs) != 1 {
		t.Fatalf("expected one run dir after task dry-run, got %d (%v)", len(runDirs), runDirs)
	}
	taskRunDir := runDirs[0]
	taskRunMeta := readCLIArtifactJSON(t, filepath.Join(taskRunDir, "run.json"))
	if got := taskRunMeta["mode"]; got != string(model.ExecutionModePRDTasks) {
		t.Fatalf("unexpected task run mode: %#v", taskRunMeta)
	}
	taskResult := readCLIArtifactJSON(t, filepath.Join(taskRunDir, "result.json"))
	if got := taskResult["status"]; got != execStatusSucceeded {
		t.Fatalf("unexpected task result payload: %#v", taskResult)
	}
	taskPromptPath := singleCLIJobArtifact(t, taskRunDir, "*.prompt.md")
	taskPromptBytes, err := os.ReadFile(taskPromptPath)
	if err != nil {
		t.Fatalf("read task prompt artifact: %v", err)
	}
	for _, want := range []string{"Add Ready Endpoint", "GET /ready", "src/server.js"} {
		if !strings.Contains(string(taskPromptBytes), want) {
			t.Fatalf("expected task prompt to contain %q, got:\n%s", want, string(taskPromptBytes))
		}
	}
	for _, want := range []eventspkg.EventKind{
		eventspkg.EventKindRunStarted,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindRunCompleted,
	} {
		if !slices.Contains(cliRuntimeEventKinds(t, filepath.Join(taskRunDir, "events.jsonl")), want) {
			t.Fatalf("expected task runtime events to include %s", want)
		}
	}

	listOutput, err := executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(defaults),
		nil,
		"list",
		"node-health",
	)
	if err != nil {
		t.Fatalf("execute reviews list: %v\noutput:\n%s", err, listOutput)
	}
	if !containsAll(
		listOutput,
		"node-health round 001",
		"provider=manual",
		"pr=fixture",
		"unresolved=1",
		"resolved=0",
	) {
		t.Fatalf("unexpected reviews list output:\n%s", listOutput)
	}

	showOutput, err := executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(defaults),
		nil,
		"show",
		"node-health",
		"1",
	)
	if err != nil {
		t.Fatalf("execute reviews show: %v\noutput:\n%s", err, showOutput)
	}
	if !containsAll(
		showOutput,
		"node-health round 001",
		"- issue_001 | status=pending severity=warning path=reviews-001/issue_001.md",
	) {
		t.Fatalf("unexpected reviews show output:\n%s", showOutput)
	}

	stdout, stderr, err = executeCommandCapturingProcessIO(
		t,
		newRootCommandWithDefaults(newLazyRootDispatcher(), defaults),
		nil,
		"reviews",
		"fix",
		"node-health",
		"--round",
		"1",
		"--dry-run",
		"--stream",
	)
	if err != nil {
		t.Fatalf("execute reviews fix dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stdout == "" || !strings.Contains(stdout, "task run started:") || !strings.Contains(stdout, "(mode=stream)") {
		t.Fatalf("unexpected reviews fix stdout: %q", stdout)
	}

	runDirsAfterReview := runDirsForCLI(t)
	if len(runDirsAfterReview) != 2 {
		t.Fatalf("expected two run dirs after review dry-run, got %d (%v)", len(runDirsAfterReview), runDirsAfterReview)
	}
	reviewRunDir := newCLIRunDir(t, runDirs, runDirsAfterReview)
	reviewRunMeta := readCLIArtifactJSON(t, filepath.Join(reviewRunDir, "run.json"))
	if got := reviewRunMeta["mode"]; got != string(model.ModeCodeReview) {
		t.Fatalf("unexpected review run mode: %#v", reviewRunMeta)
	}
	reviewResult := readCLIArtifactJSON(t, filepath.Join(reviewRunDir, "result.json"))
	if got := reviewResult["status"]; got != execStatusSucceeded {
		t.Fatalf("unexpected review result payload: %#v", reviewResult)
	}
	reviewPromptPath := singleCLIJobArtifact(t, reviewRunDir, "*.prompt.md")
	reviewPromptBytes, err := os.ReadFile(reviewPromptPath)
	if err != nil {
		t.Fatalf("read review prompt artifact: %v", err)
	}
	for _, want := range []string{"`cy-fix-reviews`", "issue_001.md", "src/server.js"} {
		if !strings.Contains(string(reviewPromptBytes), want) {
			t.Fatalf("expected review prompt to contain %q, got:\n%s", want, string(reviewPromptBytes))
		}
	}
	for _, want := range []eventspkg.EventKind{
		eventspkg.EventKindRunStarted,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindRunCompleted,
	} {
		if !slices.Contains(cliRuntimeEventKinds(t, filepath.Join(reviewRunDir, "events.jsonl")), want) {
			t.Fatalf("expected review runtime events to include %s", want)
		}
	}
}

func latestRunDirForCLI(t *testing.T, _ string) string {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("resolve home paths: %v", err)
	}
	entries, err := os.ReadDir(homePaths.RunsDir)
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one run dir, got %d", len(entries))
	}
	return filepath.Join(homePaths.RunsDir, entries[0].Name())
}

func assertNoRunArtifactsForCLI(t *testing.T, workspaceRoot string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(workspaceRoot, ".compozy", "runs")); !os.IsNotExist(err) {
		t.Fatalf("expected no persisted exec artifacts by default, got stat err=%v", err)
	}
}

func seedCLIRunForPurge(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	runID string,
	status string,
	endedAt time.Time,
) {
	t.Helper()

	run := globaldb.Run{
		RunID:            runID,
		WorkspaceID:      workspaceID,
		Mode:             "task",
		Status:           status,
		PresentationMode: "stream",
		StartedAt:        endedAt.Add(-time.Minute),
	}
	if status != "running" {
		run.EndedAt = &endedAt
	}
	if _, err := db.PutRun(context.Background(), run); err != nil {
		t.Fatalf("PutRun(%q) error = %v", runID, err)
	}
}

func createCLIRunArtifacts(t *testing.T, runID string) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runArtifacts.RunDir, "marker.txt"), []byte(runID), 0o600); err != nil {
		t.Fatalf("write run marker: %v", err)
	}
}

func assertCLIRunDirMissing(t *testing.T, runID string) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	if _, err := os.Stat(runArtifacts.RunDir); !os.IsNotExist(err) {
		t.Fatalf("expected run dir %q to be removed, stat err=%v", runArtifacts.RunDir, err)
	}
}

func assertCLIRunDirPresent(t *testing.T, runID string) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", runID, err)
	}
	if _, err := os.Stat(runArtifacts.RunDir); err != nil {
		t.Fatalf("expected run dir %q to remain, stat err=%v", runArtifacts.RunDir, err)
	}
}

func writePersistedExecRunForCLI(t *testing.T, _ string, record coreRun.PersistedExecRun) {
	t.Helper()

	runArtifacts, err := model.ResolveHomeRunArtifacts(record.RunID)
	if err != nil {
		t.Fatalf("resolve home run artifacts: %v", err)
	}
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir persisted exec run dir: %v", err)
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal persisted exec run: %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunMetaPath, payload, 0o600); err != nil {
		t.Fatalf("write persisted exec run: %v", err)
	}
}

func persistedRunDirForCLI(t *testing.T, workspaceRoot, runID string) string {
	t.Helper()

	runArtifacts, err := model.ResolvePersistedRunArtifacts(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("resolve persisted run artifacts: %v", err)
	}
	return runArtifacts.RunDir
}

func decodeExecJSONLEvents(t *testing.T, stdout string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode exec jsonl line: %v\nline:\n%s", err, line)
		}
		events = append(events, payload)
	}
	return events
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()

	cliWorkingDirMu.Lock()

	originalWD, err := os.Getwd()
	if err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("getwd: %v", err)
	}
	originalHome := os.Getenv("HOME")
	restoreHome := false
	if originalHome == originalCLIHome {
		if err := os.Setenv("HOME", t.TempDir()); err != nil {
			cliWorkingDirMu.Unlock()
			t.Fatalf("set HOME: %v", err)
		}
		restoreHome = true
	}
	t.Cleanup(func() {
		defer cliWorkingDirMu.Unlock()
		if restoreHome {
			if err := os.Setenv("HOME", originalHome); err != nil {
				t.Fatalf("restore HOME: %v", err)
			}
		}
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(dir); err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("chdir: %v", err)
	}
}

func executeRootCommandCapturingProcessIO(t *testing.T, in io.Reader, args ...string) (string, string, error) {
	t.Helper()

	return executeCommandCapturingProcessIO(t, NewRootCommand(), in, args...)
}

func executeCommandCapturingProcessIO(
	t *testing.T,
	cmd *cobra.Command,
	in io.Reader,
	args ...string,
) (string, string, error) {
	t.Helper()

	cliProcessIOMu.Lock()
	defer cliProcessIOMu.Unlock()

	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	os.Stdout = stdoutWrite
	os.Stderr = stderrWrite
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	cmd.SetOut(stdoutWrite)
	cmd.SetErr(stderrWrite)
	if in != nil {
		cmd.SetIn(in)
	}
	cmd.SetArgs(args)

	runErr := cmd.Execute()

	if err := stdoutWrite.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWrite.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	stdoutBytes, err := io.ReadAll(stdoutRead)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrRead)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := stdoutRead.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := stderrRead.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}

	return string(stdoutBytes), string(stderrBytes), runErr
}

func allowBundledSkillsForExecutionTests() commandStateDefaults {
	defaults := defaultCommandStateDefaults()
	defaults.listBundledSkills = func() ([]setup.Skill, error) {
		return nil, nil
	}
	defaults.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{}, nil
	}
	return defaults
}

func readCLIArtifactJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact %s: %v", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("decode artifact %s: %v", path, err)
	}
	return payload
}

func singleCLIJobArtifact(t *testing.T, runDir string, pattern string) string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(runDir, "jobs", pattern))
	if err != nil {
		t.Fatalf("glob job artifact %s: %v", pattern, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %s artifact, got %d (%v)", pattern, len(matches), matches)
	}
	return matches[0]
}

func cliRuntimeEventKinds(t *testing.T, eventsPath string) []eventspkg.EventKind {
	t.Helper()

	content, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events artifact %s: %v", eventsPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	kinds := make([]eventspkg.EventKind, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event eventspkg.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode runtime event line: %v\nline:\n%s", err, line)
		}
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func runDirsForCLI(t *testing.T) []string {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("resolve home paths: %v", err)
	}
	entries, err := os.ReadDir(homePaths.RunsDir)
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirs = append(dirs, filepath.Join(homePaths.RunsDir, entry.Name()))
	}
	slices.Sort(dirs)
	return dirs
}

func newCLIRunDir(t *testing.T, before []string, after []string) string {
	t.Helper()

	beforeSet := make(map[string]struct{}, len(before))
	for _, path := range before {
		beforeSet[path] = struct{}{}
	}
	for _, path := range after {
		if _, ok := beforeSet[path]; !ok {
			return path
		}
	}
	t.Fatalf("expected one new run dir\nbefore: %v\nafter:  %v", before, after)
	return ""
}

func writeCLINodeWorkflowFixture(t *testing.T, workspaceRoot string, slug string) {
	t.Helper()

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", slug)
	if err := os.MkdirAll(filepath.Join(workflowDir, "adrs"), 0o755); err != nil {
		t.Fatalf("mkdir ADR dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "test"), 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}

	files := map[string]string{
		filepath.Join(workspaceRoot, "package.json"): `{
  "name": "node-health-api",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "start": "node src/server.js",
    "test": "node --test"
  }
}
`,
		filepath.Join(workspaceRoot, "src", "server.js"): `import http from 'node:http';

export function createServer() {
  return http.createServer((req, res) => {
    if (req.method === 'GET' && req.url === '/health') {
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
      return;
    }

    res.writeHead(404, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ error: 'not_found' }));
  });
}

if (process.env.NODE_ENV !== 'test') {
  const server = createServer();
  server.listen(process.env.PORT || 3000);
}
`,
		filepath.Join(workspaceRoot, "test", "server.test.js"): `import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from '../src/server.js';

test('GET /health returns ok', async () => {
  const server = createServer();
  await new Promise((resolve) => server.listen(0, resolve));
  const port = server.address().port;
  const response = await fetch(` + "`http://127.0.0.1:${port}/health`" + `);
  const body = await response.json();
  assert.equal(response.status, 200);
  assert.deepEqual(body, { ok: true });
  await new Promise((resolve, reject) => server.close((err) => err ? reject(err) : resolve()));
});
`,
		filepath.Join(workflowDir, "_techspec.md"): `# Node Health API TechSpec

## Executive Summary
This fixture adds a small readiness endpoint to a minimal Node.js API so daemon-backed Compozy task and review flows have realistic files to inspect. The goal is deterministic operator-flow validation, not production feature depth.

## System Architecture
### Component Overview
- ` + "`src/server.js`" + ` owns HTTP routing.
- ` + "`test/server.test.js`" + ` validates public HTTP behavior.

## Implementation Design
### Core Interfaces
` + "```js" + `
export function createServer() {
  return http.createServer((req, res) => {
    // route handling
  });
}
` + "```" + `

### Data Models
- Success payloads use ` + "`{ ok: boolean }`" + `.
- Error payloads use ` + "`{ error: string }`" + `.

### API Endpoints
- ` + "`GET /health`" + ` returns readiness.
- ` + "`GET /ready`" + ` should return readiness metadata.

## Impact Analysis
| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| ` + "`src/server.js`" + ` | modified | Add ` + "`GET /ready`" + ` behavior. Low risk. | Edit route handling. |
| ` + "`test/server.test.js`" + ` | modified | Add coverage for ` + "`GET /ready`" + `. Low risk. | Add test. |

## Testing Approach
### Unit Tests
- Validate route dispatch and JSON payloads.

### Integration Tests
- Run ` + "`node --test`" + ` against the HTTP server behavior.

## Development Sequencing
### Build Order
1. Extend routing in ` + "`src/server.js`" + ` - no dependencies.
2. Add ` + "`node --test`" + ` coverage for ` + "`/ready`" + ` - depends on step 1.

### Technical Dependencies
- None.

## Technical Considerations
### Key Decisions
- Keep the fixture dependency-free and use Node built-ins only.

### Known Risks
- The task/review flow must stay dry-run to avoid nesting an external ACP runtime during daemon QA.

## Architecture Decision Records
- [ADR-001: Keep the fixture dependency-free](adrs/adr-001.md) — Use built-in Node modules so daemon QA remains deterministic.
`,
		filepath.Join(workflowDir, "adrs", "adr-001.md"): `# ADR-001: Keep the fixture dependency-free

- Status: Accepted
- Date: 2026-04-18

## Context
The daemon QA fixture should stay small and deterministic.

## Decision
Use only built-in Node.js modules.

## Consequences
The fixture is easy to bootstrap but intentionally simple.
`,
		filepath.Join(workflowDir, "_tasks.md"): `# Node Health API — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Add Ready Endpoint | pending | low | — |
`,
		filepath.Join(workflowDir, "task_01.md"): `---
status: pending
title: Add Ready Endpoint
type: backend
complexity: low
dependencies: []
---

# Task 1: Add Ready Endpoint

## Overview
Add a ` + "`GET /ready`" + ` endpoint to the temporary Node.js API so the workflow has a concrete implementation target. Keep the change limited to the existing server and test files.

<critical>
- ALWAYS READ the TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
1. MUST add ` + "`GET /ready`" + ` to the HTTP server.
2. MUST return JSON with a stable readiness payload.
3. MUST add test coverage in ` + "`test/server.test.js`" + `.
</requirements>

## Subtasks
- [ ] 1.1 Add the ` + "`GET /ready`" + ` route.
- [ ] 1.2 Return a JSON readiness response.
- [ ] 1.3 Add automated test coverage.

## Implementation Details
Update the existing server and test files. See ` + "`_techspec.md`" + ` sections "API Endpoints" and "Testing Approach".

### Relevant Files
- ` + "`src/server.js`" + ` — current HTTP route implementation.
- ` + "`test/server.test.js`" + ` — public HTTP behavior tests.

### Dependent Files
- ` + "`_techspec.md`" + ` — approved technical direction for the fixture.

### Related ADRs
- [ADR-001: Keep the fixture dependency-free](adrs/adr-001.md) — Avoid external packages in the temporary fixture.

## Deliverables
- Updated ` + "`src/server.js`" + `
- Updated ` + "`test/server.test.js`" + `
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for the readiness endpoint **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] ` + "`GET /ready`" + ` returns HTTP 200.
  - [ ] ` + "`GET /ready`" + ` returns the expected JSON payload.
- Integration tests:
  - [ ] ` + "`node --test`" + ` passes with the new endpoint.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- ` + "`GET /ready`" + ` is documented in the fixture techspec
- The task is independently executable through Compozy
`,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture file %s: %v", path, err)
		}
	}

	reviewDir := filepath.Join(workflowDir, "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "manual",
		PR:        "fixture",
		Round:     1,
		CreatedAt: time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add a readiness endpoint for operator probes",
			File:        "src/server.js",
			Line:        5,
			Severity:    "warning",
			Author:      "qa-bot",
			ProviderRef: "local:issue-001",
			Body:        "The temporary API exposes `GET /health` but not a stable `GET /ready` endpoint. Add a dedicated readiness route and protect it with test coverage.",
		},
	}); err != nil {
		t.Fatalf("write review round: %v", err)
	}
}

func cliReusableAgentLifecyclePayloads(
	t *testing.T,
	eventsPath string,
) []kinds.ReusableAgentLifecyclePayload {
	t.Helper()

	events := readCLIRuntimeEvents(t, eventsPath)
	payloads := make([]kinds.ReusableAgentLifecyclePayload, 0)
	for _, event := range events {
		if event.Kind != eventspkg.EventKindReusableAgentLifecycle {
			continue
		}
		var payload kinds.ReusableAgentLifecyclePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode reusable agent payload: %v", err)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func readCLIRuntimeEvents(t *testing.T, eventsPath string) []eventspkg.Event {
	t.Helper()

	content, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events artifact %s: %v", eventsPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	events := make([]eventspkg.Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event eventspkg.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode runtime event line: %v\nline:\n%s", err, line)
		}
		events = append(events, event)
	}
	return events
}

func mustCLIContentBlock(t *testing.T, payload any) model.ContentBlock {
	t.Helper()

	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("new CLI content block: %v", err)
	}
	return block
}

func writeReusableAgentForCLI(t *testing.T, workspaceRoot, name, agentMarkdown, mcpContent string) {
	t.Helper()

	writeReusableAgentFixtureForCLI(
		t,
		filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", name),
		agentMarkdown,
		mcpContent,
	)
}

func writeGlobalReusableAgentForCLI(t *testing.T, homeDir, name, agentMarkdown, mcpContent string) {
	t.Helper()

	writeReusableAgentFixtureForCLI(
		t,
		filepath.Join(homeDir, model.WorkflowRootDirName, "agents", name),
		agentMarkdown,
		mcpContent,
	)
}

func writeReusableAgentFixtureForCLI(t *testing.T, agentDir, agentMarkdown, mcpContent string) {
	t.Helper()

	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir reusable agent dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(agentMarkdown), 0o600); err != nil {
		t.Fatalf("write AGENT.md: %v", err)
	}
	if strings.TrimSpace(mcpContent) != "" {
		if err := os.WriteFile(filepath.Join(agentDir, "mcp.json"), []byte(mcpContent), 0o600); err != nil {
			t.Fatalf("write mcp.json: %v", err)
		}
	}
}

type cliCapturingACPClient struct {
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error)
	resumeSessionFn func(context.Context, agent.ResumeSessionRequest) (agent.Session, error)
}

func (c *cliCapturingACPClient) CreateSession(
	ctx context.Context,
	req agent.SessionRequest,
) (agent.Session, error) {
	if c.createSessionFn == nil {
		return nil, nil
	}
	return c.createSessionFn(ctx, req)
}

func (c *cliCapturingACPClient) ResumeSession(
	ctx context.Context,
	req agent.ResumeSessionRequest,
) (agent.Session, error) {
	if c.resumeSessionFn == nil {
		return nil, nil
	}
	return c.resumeSessionFn(ctx, req)
}

func (*cliCapturingACPClient) SupportsLoadSession() bool { return true }
func (*cliCapturingACPClient) Close() error              { return nil }
func (*cliCapturingACPClient) Kill() error               { return nil }

type cliACPTestSession struct {
	id       string
	identity agent.SessionIdentity
	updates  chan model.SessionUpdate
	done     chan struct{}
	err      error
}

func newCLIACPTestSession(
	id string,
	identity agent.SessionIdentity,
	updates []model.SessionUpdate,
	err error,
) *cliACPTestSession {
	session := &cliACPTestSession{
		id:       id,
		identity: identity,
		updates:  make(chan model.SessionUpdate, len(updates)),
		done:     make(chan struct{}),
		err:      err,
	}
	go func() {
		for i := range updates {
			session.updates <- updates[i]
		}
		close(session.updates)
		close(session.done)
	}()
	return session
}

func (s *cliACPTestSession) ID() string { return s.id }

func (s *cliACPTestSession) Identity() agent.SessionIdentity { return s.identity }

func (s *cliACPTestSession) Updates() <-chan model.SessionUpdate { return s.updates }

func (s *cliACPTestSession) Done() <-chan struct{} { return s.done }

func (s *cliACPTestSession) Err() error { return s.err }

func (*cliACPTestSession) SlowPublishes() uint64 { return 0 }

func (*cliACPTestSession) DroppedUpdates() uint64 { return 0 }

func containsAll(s string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(s, fragment) {
			return false
		}
	}
	return true
}

func prepareWorkspaceExtensionFixtureForCLI(t *testing.T, mode string) (string, string) {
	t.Helper()

	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Demo Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: Demo Task",
	))
	writeCLIWorkspaceConfig(t, workspaceRoot, "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv(testCLIDaemonHomeEnv, homeDir)
	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	t.Setenv(testCLIXDGHomeEnv, xdgConfigHome)

	recordPath := filepath.Join(t.TempDir(), "mock-extension-records.jsonl")
	binary := buildCLIMockExtensionBinary(t)
	installWorkspaceMockExtensionForCLI(t, workspaceRoot, homeDir, binary, recordPath, mode, "demo")
	return workspaceRoot, recordPath
}

func buildCLIMockExtensionBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "mock-extension")
	buildCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		buildCtx,
		"go",
		"build",
		"-o",
		binary,
		"./internal/core/extension/testdata/mock_extension",
	)
	cmd.Dir = mustCLIRepoRootPath(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build mock extension: %v\noutput:\n%s", err, string(output))
	}
	return binary
}

func installWorkspaceMockExtensionForCLI(
	t *testing.T,
	workspaceRoot string,
	homeDir string,
	binary string,
	recordPath string,
	mode string,
	workflow string,
) {
	t.Helper()

	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "mock-ext")
	if err := os.MkdirAll(extensionDir, 0o755); err != nil {
		t.Fatalf("mkdir extension dir: %v", err)
	}

	manifest := fmt.Sprintf(`
[extension]
name = "mock-ext"
version = "1.0.0"
description = "Mock extension"
min_compozy_version = "0.0.1"

[subprocess]
command = %q
shutdown_timeout = "250ms"
env = { COMPOZY_MOCK_RECORD_PATH = %q, COMPOZY_MOCK_MODE = %q, COMPOZY_MOCK_WORKFLOW = %q }

[security]
capabilities = ["tasks.read"]
`, binary, recordPath, mode, workflow)
	if err := os.WriteFile(
		filepath.Join(extensionDir, "extension.toml"),
		[]byte(strings.TrimSpace(manifest)+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write extension manifest: %v", err)
	}

	store, err := extensions.NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("create enablement store: %v", err)
	}
	if err := store.Enable(context.Background(), extensions.Ref{
		Name:          "mock-ext",
		Source:        extensions.SourceWorkspace,
		WorkspaceRoot: workspaceRoot,
	}); err != nil {
		t.Fatalf("enable workspace extension: %v", err)
	}
}

func readMockExtensionRecordsForCLI(t *testing.T, path string) []map[string]any {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mock extension records: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode mock extension record: %v\nline:\n%s", err, line)
		}
		records = append(records, payload)
	}
	return records
}

func assertMockExtensionRecordKinds(t *testing.T, records []map[string]any, wantKinds ...string) {
	t.Helper()

	kinds := make([]string, 0, len(records))
	for _, record := range records {
		kind, _ := record["type"].(string)
		kinds = append(kinds, kind)
	}
	for _, want := range wantKinds {
		if !slices.Contains(kinds, want) {
			t.Fatalf("expected mock extension records to include %q, got %v", want, kinds)
		}
	}
}
