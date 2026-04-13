package cli

import (
	"context"
	"encoding/json"
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

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/setup"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	"github.com/spf13/cobra"
)

var cliProcessIOMu sync.Mutex

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

func TestValidateTasksCommandExecuteDirectCoversFailureAndSuccess(t *testing.T) {
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

	output, err := executeRootCommand("validate-tasks", "--tasks-dir", tasksDir)
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

	output, err = executeRootCommand("validate-tasks", "--tasks-dir", tasksDir)
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

func TestExecCommandWithInstalledWorkspaceExtensionStaysEphemeralWithoutFlag(t *testing.T) {
	workspaceRoot, recordPath := prepareWorkspaceExtensionFixtureForCLI(t, "normal")
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
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

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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
	writeCLIWorkspaceConfig(t, workspaceRoot, "")
	withWorkingDir(t, workspaceRoot)

	runID := "exec-resume"
	writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
		Version:         1,
		Mode:            model.ModeExec,
		RunID:           runID,
		Status:          "succeeded",
		WorkspaceRoot:   workspaceRoot,
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

	runID := "exec-agent-resume"
	writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
		Version:         1,
		Mode:            model.ModeExec,
		RunID:           runID,
		Status:          "succeeded",
		WorkspaceRoot:   workspaceRoot,
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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
		filepath.Join(workspaceRoot, ".compozy", "runs", runID, "events.jsonl"),
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

	_, _, err := executeRootCommandCapturingProcessIO(t, nil, "exec", "--agent", "broken", "Do work")
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(t, nil, "exec", "--format", "json")
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(t, nil, "exec", "--format", "raw-json")
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

	stdout, stderr, err := executeRootCommandCapturingProcessIO(
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

func TestStartCommandExecuteDryRunPersistsKernelArtifacts(t *testing.T) {
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

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"start",
		"--name",
		"demo",
		"--tasks-dir",
		".compozy/tasks/demo",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute start dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	runMeta := readCLIArtifactJSON(t, filepath.Join(runDir, "run.json"))
	if got := runMeta["mode"]; got != string(model.ModePRDTasks) {
		t.Fatalf("unexpected run mode: %#v", runMeta)
	}

	result := readCLIArtifactJSON(t, filepath.Join(runDir, "result.json"))
	if got := result["status"]; got != "succeeded" {
		t.Fatalf("unexpected result payload: %#v", result)
	}

	promptPath := singleCLIJobArtifact(t, runDir, "*.prompt.md")
	outLogPath := singleCLIJobArtifact(t, runDir, "*.out.log")
	errLogPath := singleCLIJobArtifact(t, runDir, "*.err.log")
	for _, path := range []string{promptPath, outLogPath, errLogPath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected job artifact %s: %v", path, statErr)
		}
	}

	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	if !strings.Contains(string(promptBytes), "`cy-execute-task`") {
		t.Fatalf("expected task prompt to reference cy-execute-task, got:\n%s", string(promptBytes))
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

func TestStartCommandExecuteDryRunJSONStreamsJSONL(t *testing.T) {
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

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"start",
		"--name",
		"demo",
		"--tasks-dir",
		".compozy/tasks/demo",
		"--dry-run",
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute start json dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Execution Summary:") {
		t.Fatalf("expected json mode to suppress human summary, got %q", stdout)
	}

	events := decodeExecJSONLEvents(t, stdout)
	if len(events) < 3 {
		t.Fatalf("expected multiple streamed workflow events, got %d\nstdout:\n%s", len(events), stdout)
	}
	if got := events[0]["type"]; got != string(eventspkg.EventKindRunStarted) {
		t.Fatalf("unexpected first workflow event: %#v", events[0])
	}
	if got := events[len(events)-1]["type"]; got != string(eventspkg.EventKindRunCompleted) {
		t.Fatalf("unexpected terminal workflow event: %#v", events[len(events)-1])
	}

	var eventTypes []string
	for _, event := range events {
		gotType, ok := event["type"].(string)
		if !ok || gotType == "" {
			t.Fatalf("expected lean workflow event type field, got %#v", event)
		}
		eventTypes = append(eventTypes, gotType)
	}
	for _, want := range []string{
		string(eventspkg.EventKindRunStarted),
		string(eventspkg.EventKindJobCompleted),
		string(eventspkg.EventKindRunCompleted),
	} {
		if !slices.Contains(eventTypes, want) {
			t.Fatalf("expected streamed workflow event %q, got %v", want, eventTypes)
		}
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	eventKinds := cliRuntimeEventKinds(t, filepath.Join(runDir, "events.jsonl"))
	if got := eventKinds[len(eventKinds)-1]; got != eventspkg.EventKindRunCompleted {
		t.Fatalf("unexpected persisted terminal event: %v", eventKinds)
	}
}

func TestStartCommandWithInstalledWorkspaceExtensionSpawnsAndWritesAudit(t *testing.T) {
	workspaceRoot, recordPath := prepareWorkspaceExtensionFixtureForCLI(t, "normal")
	withWorkingDir(t, workspaceRoot)

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"start",
		"--name",
		"demo",
		"--tasks-dir",
		".compozy/tasks/demo",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute start with extensions: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "Execution Summary:") {
		t.Fatalf("expected dry-run start summary on stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
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

func TestStartCommandExplicitTUIFailsWithoutTTY(t *testing.T) {
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

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"start",
		"--name",
		"demo",
		"--tasks-dir",
		".compozy/tasks/demo",
		"--dry-run",
		"--tui",
	)
	if err == nil {
		t.Fatalf("expected start explicit tui failure\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on explicit tui failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "requires an interactive terminal for tui mode") {
		t.Fatalf("unexpected explicit tui error output: %q", stderr)
	}
}

func TestNormalizePresentationModeUsesInjectedInteractiveCheck(t *testing.T) {
	t.Parallel()

	if isInteractiveTerminal() {
		t.Skip(
			"requires a non-interactive test terminal to distinguish the injected callback from the process terminal",
		)
	}

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.tui = true
	state.isInteractive = func() bool { return true }

	cmd := &cobra.Command{Use: "start"}
	cmd.Flags().Bool("tui", true, "enable tui")
	if err := cmd.Flags().Set("tui", "true"); err != nil {
		t.Fatalf("set --tui: %v", err)
	}

	if err := state.normalizePresentationMode(cmd); err != nil {
		t.Fatalf("normalizePresentationMode() error = %v", err)
	}
	if !state.tui {
		t.Fatal("expected tui to remain enabled when injected interactivity reports true")
	}
}

func TestFixReviewsCommandExecuteDryRunPersistsKernelArtifacts(t *testing.T) {
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

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"fix-reviews",
		"--name",
		"demo",
		"--round",
		"1",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("execute fix-reviews dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for dry-run fix-reviews, got %q", stderr)
	}

	runDir := latestRunDirForCLI(t, workspaceRoot)
	runMeta := readCLIArtifactJSON(t, filepath.Join(runDir, "run.json"))
	if got := runMeta["mode"]; got != string(model.ModeCodeReview) {
		t.Fatalf("unexpected review run mode: %#v", runMeta)
	}

	result := readCLIArtifactJSON(t, filepath.Join(runDir, "result.json"))
	if got := result["status"]; got != "succeeded" {
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

func TestFixReviewsCommandExecuteDryRunRawJSONStreamsCanonicalEvents(t *testing.T) {
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

	cmd := newRootCommandWithDefaults(newRootDispatcher(), allowBundledSkillsForExecutionTests())
	stdout, stderr, err := executeCommandCapturingProcessIO(
		t,
		cmd,
		nil,
		"fix-reviews",
		"--name",
		"demo",
		"--round",
		"1",
		"--dry-run",
		"--format",
		"raw-json",
	)
	if err != nil {
		t.Fatalf("execute fix-reviews raw-json dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for raw-json fix-reviews, got %q", stderr)
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

func latestRunDirForCLI(t *testing.T, workspaceRoot string) string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(workspaceRoot, ".compozy", "runs"))
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one run dir, got %d", len(entries))
	}
	return filepath.Join(workspaceRoot, ".compozy", "runs", entries[0].Name())
}

func assertNoRunArtifactsForCLI(t *testing.T, workspaceRoot string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(workspaceRoot, ".compozy", "runs")); !os.IsNotExist(err) {
		t.Fatalf("expected no persisted exec artifacts by default, got stat err=%v", err)
	}
}

func writePersistedExecRunForCLI(t *testing.T, workspaceRoot string, record coreRun.PersistedExecRun) {
	t.Helper()

	runArtifacts := model.NewRunArtifacts(workspaceRoot, record.RunID)
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
	t.Cleanup(func() {
		defer cliWorkingDirMu.Unlock()
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
