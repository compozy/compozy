package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestDaemonStatusAndStopCommandsOperateAgainstRealDaemon(t *testing.T) {
	homeDir := newShortCLITestHomeDir(t)
	t.Setenv("HOME", homeDir)
	configureCLITestDaemonHTTPPort(t)

	paths := mustCLITestHomePaths(t)
	commandDir := t.TempDir()
	t.Cleanup(func() {
		_, _, _ = runCLICommand(t, commandDir, "daemon", "stop", "--force", "--format", "json")
		waitForCLITestDaemonState(t, paths, daemon.ReadyStateStopped)
	})

	stdout, stderr, exitCode := runCLICommand(t, commandDir, "daemon", "status", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute daemon status (stopped): exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var stoppedPayload struct {
		State  string `json:"state"`
		Health struct {
			Ready bool `json:"ready"`
		} `json:"health"`
	}
	if err := json.Unmarshal([]byte(stdout), &stoppedPayload); err != nil {
		t.Fatalf("decode stopped daemon status: %v\nstdout:\n%s", err, stdout)
	}
	if stoppedPayload.State != string(daemon.ReadyStateStopped) || stoppedPayload.Health.Ready {
		t.Fatalf("unexpected stopped daemon payload: %#v", stoppedPayload)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "daemon", "start", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute daemon start: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var startPayload struct {
		State  string `json:"state"`
		Health struct {
			Ready bool `json:"ready"`
		} `json:"health"`
		Daemon struct {
			PID      int `json:"pid"`
			HTTPPort int `json:"http_port"`
		} `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(stdout), &startPayload); err != nil {
		t.Fatalf("decode daemon start payload: %v\nstdout:\n%s", err, stdout)
	}
	if startPayload.State != string(daemon.ReadyStateReady) || !startPayload.Health.Ready ||
		startPayload.Daemon.PID <= 0 || startPayload.Daemon.HTTPPort <= 0 ||
		startPayload.Daemon.HTTPPort == daemon.DefaultHTTPPort {
		t.Fatalf("unexpected daemon start payload: %#v", startPayload)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "daemon", "status", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute daemon status (ready): exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var readyPayload struct {
		State  string `json:"state"`
		Health struct {
			Ready bool `json:"ready"`
		} `json:"health"`
		Daemon struct {
			PID      int `json:"pid"`
			HTTPPort int `json:"http_port"`
		} `json:"daemon"`
	}
	if err := json.Unmarshal([]byte(stdout), &readyPayload); err != nil {
		t.Fatalf("decode ready daemon status: %v\nstdout:\n%s", err, stdout)
	}
	if readyPayload.State != string(daemon.ReadyStateReady) || !readyPayload.Health.Ready ||
		readyPayload.Daemon.PID <= 0 || readyPayload.Daemon.HTTPPort != startPayload.Daemon.HTTPPort {
		t.Fatalf("unexpected ready daemon payload: %#v", readyPayload)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "daemon", "stop", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute daemon stop: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var stopPayload struct {
		Accepted bool `json:"accepted"`
	}
	if err := json.Unmarshal([]byte(stdout), &stopPayload); err != nil {
		t.Fatalf("decode daemon stop payload: %v\nstdout:\n%s", err, stdout)
	}
	if !stopPayload.Accepted {
		t.Fatalf("expected daemon stop to be accepted, got %#v", stopPayload)
	}
	waitForCLITestDaemonState(t, paths, daemon.ReadyStateStopped)

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "daemon", "status")
	if exitCode != 0 {
		t.Fatalf("execute daemon status (text): exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "stopped" {
		t.Fatalf("unexpected stopped daemon text output: %q", stdout)
	}
}

func TestWorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon(t *testing.T) {
	homeDir := newShortCLITestHomeDir(t)
	t.Setenv("HOME", homeDir)
	configureCLITestDaemonHTTPPort(t)

	paths := mustCLITestHomePaths(t)
	commandDir := t.TempDir()
	t.Cleanup(func() {
		_, _, _ = runCLICommand(t, commandDir, "daemon", "stop", "--force", "--format", "json")
		waitForCLITestDaemonState(t, paths, daemon.ReadyStateStopped)
	})

	workspaceA := filepath.Join(t.TempDir(), "workspace-a")
	workspaceB := filepath.Join(t.TempDir(), "workspace-b")
	for _, dir := range []string{workspaceA, workspaceB} {
		if err := os.MkdirAll(filepath.Join(dir, ".compozy", "tasks"), 0o755); err != nil {
			t.Fatalf("mkdir workspace marker %q: %v", dir, err)
		}
	}

	stdout, stderr, exitCode := runCLICommand(
		t,
		commandDir,
		"workspaces",
		"register",
		workspaceA,
		"--name",
		"alpha",
		"--format",
		"json",
	)
	if exitCode != 0 {
		t.Fatalf("execute workspaces register: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var registerPayload struct {
		Created   bool `json:"created"`
		Workspace struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			RootDir string `json:"root_dir"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(stdout), &registerPayload); err != nil {
		t.Fatalf("decode register payload: %v\nstdout:\n%s", err, stdout)
	}
	if !registerPayload.Created ||
		mustEvalSymlinksCLITest(t, registerPayload.Workspace.RootDir) != mustEvalSymlinksCLITest(t, workspaceA) ||
		registerPayload.Workspace.Name != "alpha" {
		t.Fatalf("unexpected register payload: %#v", registerPayload)
	}

	stdout, stderr, exitCode = runCLICommand(
		t,
		commandDir,
		"workspaces",
		"show",
		registerPayload.Workspace.ID,
		"--format",
		"json",
	)
	if exitCode != 0 {
		t.Fatalf("execute workspaces show: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var showPayload struct {
		Workspace struct {
			ID      string `json:"id"`
			RootDir string `json:"root_dir"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(stdout), &showPayload); err != nil {
		t.Fatalf("decode show payload: %v\nstdout:\n%s", err, stdout)
	}
	if showPayload.Workspace.ID != registerPayload.Workspace.ID ||
		mustEvalSymlinksCLITest(t, showPayload.Workspace.RootDir) != mustEvalSymlinksCLITest(t, workspaceA) {
		t.Fatalf("unexpected show payload: %#v", showPayload)
	}

	stdout, stderr, exitCode = runCLICommand(
		t,
		commandDir,
		"workspaces",
		"resolve",
		workspaceB,
		"--format",
		"json",
	)
	if exitCode != 0 {
		t.Fatalf("execute workspaces resolve: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var resolvePayload struct {
		Action    string `json:"action"`
		Workspace struct {
			RootDir string `json:"root_dir"`
		} `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(stdout), &resolvePayload); err != nil {
		t.Fatalf("decode resolve payload: %v\nstdout:\n%s", err, stdout)
	}
	if resolvePayload.Action != "resolved" ||
		mustEvalSymlinksCLITest(t, resolvePayload.Workspace.RootDir) != mustEvalSymlinksCLITest(t, workspaceB) {
		t.Fatalf("unexpected resolve payload: %#v", resolvePayload)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "workspaces", "list", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute workspaces list: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var listPayload struct {
		Workspaces []struct {
			RootDir string `json:"root_dir"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(stdout), &listPayload); err != nil {
		t.Fatalf("decode workspace list payload: %v\nstdout:\n%s", err, stdout)
	}
	if len(listPayload.Workspaces) != 2 {
		t.Fatalf("unexpected workspace list payload: %#v", listPayload)
	}

	stdout, stderr, exitCode = runCLICommand(
		t,
		commandDir,
		"workspaces",
		"unregister",
		registerPayload.Workspace.ID,
		"--format",
		"json",
	)
	if exitCode != 0 {
		t.Fatalf("execute workspaces unregister: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var unregisterPayload struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &unregisterPayload); err != nil {
		t.Fatalf("decode unregister payload: %v\nstdout:\n%s", err, stdout)
	}
	if unregisterPayload.Action != "unregistered" {
		t.Fatalf("unexpected unregister payload: %#v", unregisterPayload)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "workspaces", "list", "--format", "json")
	if exitCode != 0 {
		t.Fatalf(
			"execute workspaces list after unregister: exit=%d\nstdout:\n%s\nstderr:\n%s",
			exitCode,
			stdout,
			stderr,
		)
	}
	if err := json.Unmarshal([]byte(stdout), &listPayload); err != nil {
		t.Fatalf("decode workspace list after unregister: %v\nstdout:\n%s", err, stdout)
	}
	if len(listPayload.Workspaces) != 1 ||
		mustEvalSymlinksCLITest(t, listPayload.Workspaces[0].RootDir) != mustEvalSymlinksCLITest(t, workspaceB) {
		t.Fatalf("unexpected workspace list after unregister: %#v", listPayload)
	}
}

func TestWorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon(t *testing.T) {
	homeDir := newShortCLITestHomeDir(t)
	t.Setenv("HOME", homeDir)
	configureCLITestDaemonHTTPPort(t)

	paths := mustCLITestHomePaths(t)
	commandDir := t.TempDir()
	t.Cleanup(func() {
		_, _, _ = runCLICommand(t, commandDir, "daemon", "stop", "--force", "--format", "json")
		waitForCLITestDaemonState(t, paths, daemon.ReadyStateStopped)
	})

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks"), 0o755); err != nil {
		t.Fatalf("mkdir workspace marker: %v", err)
	}

	stdout, stderr, exitCode := runCLICommand(
		t,
		commandDir,
		"workspaces",
		"register",
		workspaceRoot,
		"--format",
		"json",
	)
	if exitCode != 0 {
		t.Fatalf("execute workspaces register: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	db := openCLITestGlobalDB(t, paths)
	workspaceRow, err := db.Get(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", workspaceRoot, err)
	}
	seedCLIRunForPurge(
		t,
		db,
		workspaceRow.ID,
		"run-active-workspace",
		"running",
		time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
	)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "workspaces", "unregister", workspaceRoot)
	if exitCode != 1 {
		t.Fatalf("expected unregister conflict exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stderr, "active run") {
		t.Fatalf("expected active run conflict stderr, got:\n%s", stderr)
	}

	stdout, stderr, exitCode = runCLICommand(t, commandDir, "workspaces", "show", workspaceRoot, "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected workspace to remain registered: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
}

func TestSyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory(t *testing.T) {
	homeDir := newShortCLITestHomeDir(t)
	t.Setenv("HOME", homeDir)
	configureCLITestDaemonHTTPPort(t)

	paths := mustCLITestHomePaths(t)
	commandDir := t.TempDir()
	t.Cleanup(func() {
		_, _, _ = runCLICommand(t, commandDir, "daemon", "stop", "--force", "--format", "json")
		waitForCLITestDaemonState(t, paths, daemon.ReadyStateStopped)
	})

	workspaceRoot := t.TempDir()
	writeCLITestWorkflowTask(t, workspaceRoot, "complete", "completed")
	writeCLITestWorkflowTask(t, workspaceRoot, "blocked", "pending")

	nestedDir := filepath.Join(workspaceRoot, "pkg", "feature", "subdir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	stdout, stderr, exitCode := runCLICommand(t, nestedDir, "sync", "--name", "complete", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute single-workflow sync: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var singleSyncPayload struct {
		WorkflowSlug     string `json:"workflow_slug"`
		WorkflowsScanned int    `json:"workflows_scanned"`
	}
	if err := json.Unmarshal([]byte(stdout), &singleSyncPayload); err != nil {
		t.Fatalf("decode single sync payload: %v\nstdout:\n%s", err, stdout)
	}
	if singleSyncPayload.WorkflowSlug != "complete" || singleSyncPayload.WorkflowsScanned != 1 {
		t.Fatalf("unexpected single sync payload: %#v", singleSyncPayload)
	}

	stdout, stderr, exitCode = runCLICommand(t, nestedDir, "sync", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute workspace sync: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var syncPayload struct {
		WorkflowsScanned  int `json:"workflows_scanned"`
		TaskItemsUpserted int `json:"task_items_upserted"`
	}
	if err := json.Unmarshal([]byte(stdout), &syncPayload); err != nil {
		t.Fatalf("decode workspace sync payload: %v\nstdout:\n%s", err, stdout)
	}
	if syncPayload.WorkflowsScanned != 2 || syncPayload.TaskItemsUpserted != 2 {
		t.Fatalf("unexpected workspace sync payload: %#v", syncPayload)
	}

	db := openCLITestGlobalDB(t, paths)
	workspaceRow, err := db.Get(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", workspaceRoot, err)
	}
	workflows, err := db.ListWorkflows(
		context.Background(),
		globaldb.ListWorkflowsOptions{WorkspaceID: workspaceRow.ID},
	)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(workflows) != 2 {
		t.Fatalf("unexpected synced workflow rows: %#v", workflows)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	stdout, stderr, exitCode = runCLICommand(t, nestedDir, "archive", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("execute workspace archive: exit=%d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	var archivePayload struct {
		Archived       int               `json:"archived"`
		Skipped        int               `json:"skipped"`
		ArchivedPaths  []string          `json:"archived_paths"`
		SkippedPaths   []string          `json:"skipped_paths"`
		SkippedReasons map[string]string `json:"skipped_reasons"`
	}
	if err := json.Unmarshal([]byte(stdout), &archivePayload); err != nil {
		t.Fatalf("decode workspace archive payload: %v\nstdout:\n%s", err, stdout)
	}
	if archivePayload.Archived != 1 || archivePayload.Skipped != 1 {
		t.Fatalf("unexpected workspace archive payload: %#v", archivePayload)
	}
	if len(archivePayload.ArchivedPaths) != 1 || archivePayload.ArchivedPaths[0] != "complete" {
		t.Fatalf("unexpected archived paths: %#v", archivePayload)
	}
	if len(archivePayload.SkippedPaths) != 1 || archivePayload.SkippedPaths[0] != "blocked" {
		t.Fatalf("unexpected skipped paths: %#v", archivePayload)
	}
	if archivePayload.SkippedReasons["blocked"] == "" {
		t.Fatalf("expected skip reason for blocked workflow, got %#v", archivePayload.SkippedReasons)
	}

	matches, err := filepath.Glob(filepath.Join(workspaceRoot, ".compozy", "tasks", "_archived", "*-complete"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("unexpected archived workflow matches: %#v", matches)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".compozy", "tasks", "blocked")); err != nil {
		t.Fatalf("expected blocked workflow to remain active: %v", err)
	}

	stdout, stderr, exitCode = runCLICommand(t, nestedDir, "archive", "--name", "blocked")
	if exitCode != 1 {
		t.Fatalf(
			"expected single-workflow archive conflict exit code 1, got %d\nstdout:\n%s\nstderr:\n%s",
			exitCode,
			stdout,
			stderr,
		)
	}
	if !strings.Contains(stderr, "not archivable") {
		t.Fatalf("expected archive conflict stderr, got:\n%s", stderr)
	}
}

func mustCLITestHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	return paths
}

func newShortCLITestHomeDir(t *testing.T) string {
	t.Helper()

	parent := ""
	if runtime.GOOS != "windows" {
		parent = "/tmp"
	}
	homeDir, err := os.MkdirTemp(parent, "compozy-cli-home-")
	if err != nil {
		t.Fatalf("MkdirTemp(home) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(homeDir)
	})
	return homeDir
}

func configureCLITestDaemonHTTPPort(t *testing.T) {
	t.Helper()

	t.Setenv(daemonHTTPPortEnv, "0")
}

func openCLITestGlobalDB(t *testing.T, paths compozyconfig.HomePaths) *globaldb.GlobalDB {
	t.Helper()

	db, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	return db
}

func waitForCLITestDaemonState(t *testing.T, paths compozyconfig.HomePaths, want daemon.ReadyState) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := daemon.QueryStatus(context.Background(), paths, daemon.ProbeOptions{})
		if err == nil && status.State == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	status, err := daemon.QueryStatus(context.Background(), paths, daemon.ProbeOptions{})
	if err != nil {
		t.Fatalf("QueryStatus() error while waiting for %q: %v", want, err)
	}
	t.Fatalf("daemon state = %q, want %q", status.State, want)
}

func writeCLITestWorkflowTask(t *testing.T, workspaceRoot string, slug string, status string) {
	t.Helper()

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", slug)
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir %q: %v", workflowDir, err)
	}
	body := strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + cliWorkflowDisplayName(slug),
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + cliWorkflowDisplayName(slug),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workflowDir, "task_001.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write task file for %q: %v", slug, err)
	}
}

func cliWorkflowDisplayName(slug string) string {
	if strings.TrimSpace(slug) == "" {
		return ""
	}
	return strings.ToUpper(slug[:1]) + slug[1:]
}
