package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName            = "memory"
	WorkflowFileName   = "MEMORY.md"
	workflowLineLimit  = 150
	workflowByteLimit  = 12 * 1024
	taskLineLimit      = 200
	taskByteLimit      = 16 * 1024
	workflowHeader     = "# Workflow Memory"
	taskHeaderPrefix   = "# Task Memory: "
	sharedGuidanceLine = "Keep only durable, cross-task context here. " +
		"Do not duplicate facts that are obvious from the repository, PRD documents, or git history."
	taskGuidanceLine = "Keep only task-local execution context here. " +
		"Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history."
)

type FileState struct {
	Path            string
	LineCount       int
	ByteCount       int
	NeedsCompaction bool
}

type Context struct {
	Directory string
	Workflow  FileState
	Task      FileState
}

func Directory(tasksDir string) string {
	return filepath.Join(tasksDir, DirName)
}

func WorkflowPath(tasksDir string) string {
	return filepath.Join(Directory(tasksDir), WorkflowFileName)
}

func TaskPath(tasksDir, taskFileName string) string {
	return filepath.Join(Directory(tasksDir), filepath.Base(taskFileName))
}

func Prepare(tasksDir, taskFileName string) (Context, error) {
	taskBase := filepath.Base(strings.TrimSpace(taskFileName))
	if taskBase == "" || taskBase == "." {
		return Context{}, fmt.Errorf("prepare workflow memory: task file name is required")
	}

	dir := Directory(tasksDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Context{}, fmt.Errorf("prepare workflow memory dir: %w", err)
	}

	workflowPath := WorkflowPath(tasksDir)
	if err := writeIfMissing(workflowPath, workflowTemplate()); err != nil {
		return Context{}, fmt.Errorf("bootstrap workflow memory: %w", err)
	}

	taskPath := TaskPath(tasksDir, taskBase)
	if err := writeIfMissing(taskPath, taskTemplate(taskBase)); err != nil {
		return Context{}, fmt.Errorf("bootstrap task memory: %w", err)
	}

	workflowState, err := inspect(workflowPath, workflowLineLimit, workflowByteLimit)
	if err != nil {
		return Context{}, fmt.Errorf("inspect workflow memory: %w", err)
	}
	taskState, err := inspect(taskPath, taskLineLimit, taskByteLimit)
	if err != nil {
		return Context{}, fmt.Errorf("inspect task memory: %w", err)
	}

	return Context{
		Directory: dir,
		Workflow:  workflowState,
		Task:      taskState,
	}, nil
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	return nil
}

func inspect(path string, lineLimit, byteLimit int) (FileState, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileState{}, err
	}

	lineCount := countLines(string(content))
	byteCount := len(content)
	return FileState{
		Path:            path,
		LineCount:       lineCount,
		ByteCount:       byteCount,
		NeedsCompaction: lineCount > lineLimit || byteCount > byteLimit,
	}, nil
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return len(lines)
}

func workflowTemplate() string {
	return strings.Join([]string{
		workflowHeader,
		"",
		sharedGuidanceLine,
		"",
		"## Current State",
		"",
		"## Shared Decisions",
		"",
		"## Shared Learnings",
		"",
		"## Open Risks",
		"",
		"## Handoffs",
		"",
	}, "\n")
}

func taskTemplate(taskFileName string) string {
	return strings.Join([]string{
		taskHeaderPrefix + taskFileName,
		"",
		taskGuidanceLine,
		"",
		"## Objective Snapshot",
		"",
		"## Important Decisions",
		"",
		"## Learnings",
		"",
		"## Files / Surfaces",
		"",
		"## Errors / Corrections",
		"",
		"## Ready for Next Run",
		"",
	}, "\n")
}
