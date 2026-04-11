package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
)

type blockingExecuteOps struct {
	realOperations
	executeStarted chan struct{}
}

func (o *blockingExecuteOps) Execute(
	ctx context.Context,
	_ *model.SolvePreparation,
	_ *model.RuntimeConfig,
) error {
	select {
	case o.executeStarted <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestRunStartHandleCancellationShutsDownActiveExtensions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workspaceRoot, tasksDir, recordPath := prepareKernelExtensionWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ops := &blockingExecuteOps{
		realOperations: realOperations{agentRegistry: agent.DefaultRegistry()},
		executeStarted: make(chan struct{}, 1),
	}
	dispatcher := BuildDefault(KernelDeps{
		AgentRegistry: agent.DefaultRegistry(),
		ops:           ops,
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
			ctx,
			dispatcher,
			commands.RunStartCommand{
				Runtime: model.RuntimeConfig{
					WorkspaceRoot:              workspaceRoot,
					Name:                       "demo",
					TasksDir:                   tasksDir,
					Mode:                       model.ExecutionModePRDTasks,
					IDE:                        model.IDECodex,
					DryRun:                     true,
					BatchSize:                  1,
					Concurrent:                 1,
					EnableExecutableExtensions: true,
					RetryBackoffMultiplier:     1.5,
				},
			},
		)
		errCh <- err
	}()

	select {
	case <-ops.executeStarted:
	case err := <-errCh:
		t.Fatalf("run returned before execute started: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for execute phase to start")
	}

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for canceled run to return")
	}

	records := readMockExtensionRecordsForKernel(t, recordPath)
	for _, want := range []string{"initialize_request", "shutdown"} {
		if !slices.Contains(records, want) {
			t.Fatalf("expected mock extension records to include %q, got %v", want, records)
		}
	}
}

func prepareKernelExtensionWorkspace(t *testing.T) (string, string, string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	taskFile := filepath.Join(tasksDir, "task_01.md")
	taskContent := `---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task

Keep the run open until cancellation.
`
	if err := os.WriteFile(taskFile, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	recordPath := filepath.Join(t.TempDir(), "mock-extension-records.jsonl")
	binary := buildKernelMockExtensionBinary(t)
	installKernelWorkspaceExtension(t, workspaceRoot, binary, recordPath)
	return workspaceRoot, tasksDir, recordPath
}

func buildKernelMockExtensionBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "mock-extension")
	buildCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		buildCtx,
		"go",
		"build",
		"-buildvcs=false",
		"-o",
		binary,
		"./internal/core/extension/testdata/mock_extension",
	)
	cmd.Dir = repoRootForKernelTests(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build mock extension: %v\noutput:\n%s", err, string(output))
	}
	return binary
}

func installKernelWorkspaceExtension(t *testing.T, workspaceRoot, binary, recordPath string) {
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
env = { COMPOZY_MOCK_RECORD_PATH = %q, COMPOZY_MOCK_MODE = "normal" }

[security]
capabilities = []
`, binary, recordPath)
	if err := os.WriteFile(
		filepath.Join(extensionDir, "extension.toml"),
		[]byte(strings.TrimSpace(manifest)+"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write extension manifest: %v", err)
	}

	homeDir := os.Getenv("HOME")
	if strings.TrimSpace(homeDir) == "" {
		t.Fatal("HOME must be set for workspace extension enablement")
	}
	resolvedWorkspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("resolve workspace root symlinks: %v", err)
	}
	statePath := filepath.Join(homeDir, ".compozy", "state", "workspace-extensions.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir workspace state dir: %v", err)
	}
	payload := fmt.Sprintf(
		"{\n  \"workspaces\": {\n    %q: {\n      \"mock-ext\": true\n    }\n  }\n}\n",
		filepath.Clean(resolvedWorkspaceRoot),
	)
	if err := os.WriteFile(statePath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write workspace extension state: %v", err)
	}
}

func readMockExtensionRecordsForKernel(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mock extension records: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	kinds := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode mock extension record: %v\nline:\n%s", err, line)
		}
		if kind, _ := payload["type"].(string); kind != "" {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

func repoRootForKernelTests(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}
