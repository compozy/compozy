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

type WriteMode string

const (
	WriteModeReplace WriteMode = "replace"
	WriteModeAppend  WriteMode = "append"
)

type FileState struct {
	Path            string
	LineCount       int
	ByteCount       int
	NeedsCompaction bool
}

type Document struct {
	FileState
	Content string
	Exists  bool
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

func ResolveDocumentPath(tasksDir, taskFileName string) string {
	if strings.TrimSpace(taskFileName) == "" {
		return WorkflowPath(tasksDir)
	}
	return TaskPath(tasksDir, taskFileName)
}

func ReadDocument(tasksDir, taskFileName string) (Document, error) {
	path := ResolveDocumentPath(tasksDir, taskFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Document{
				FileState: FileState{Path: path},
				Content:   "",
				Exists:    false,
			}, nil
		}
		return Document{}, fmt.Errorf("read memory document: %w", err)
	}

	state, err := inspectResolved(path)
	if err != nil {
		return Document{}, err
	}
	return Document{
		FileState: state,
		Content:   string(content),
		Exists:    true,
	}, nil
}

func WriteDocument(tasksDir, taskFileName, content string, mode WriteMode) (Document, int, error) {
	dir := Directory(tasksDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Document{}, 0, fmt.Errorf("prepare memory document dir: %w", err)
	}

	path := ResolveDocumentPath(tasksDir, taskFileName)
	finalContent, err := renderedDocumentContent(path, content, mode)
	if err != nil {
		return Document{}, 0, err
	}

	if err := writeAtomically(path, finalContent); err != nil {
		return Document{}, 0, err
	}

	document, err := ReadDocument(tasksDir, taskFileName)
	if err != nil {
		return Document{}, 0, err
	}
	return document, len(finalContent), nil
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

func renderedDocumentContent(path, content string, mode WriteMode) ([]byte, error) {
	switch normalizeWriteMode(mode) {
	case WriteModeReplace:
		return []byte(content), nil
	case WriteModeAppend:
		existing, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read memory document for append: %w", err)
		}
		if len(existing) == 0 {
			return []byte(content), nil
		}
		if content == "" {
			return existing, nil
		}

		var builder strings.Builder
		builder.Grow(len(existing) + len(content) + 1)
		builder.WriteString(strings.TrimRight(string(existing), "\n"))
		builder.WriteByte('\n')
		builder.WriteString(strings.TrimLeft(content, "\n"))
		return []byte(builder.String()), nil
	default:
		return nil, fmt.Errorf("write memory document: unsupported mode %q", mode)
	}
}

func writeAtomically(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create memory temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(content); err != nil {
		cleanup()
		return fmt.Errorf("write memory temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close memory temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod memory temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace memory document: %w", err)
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

func inspectResolved(path string) (FileState, error) {
	switch filepath.Base(strings.TrimSpace(path)) {
	case WorkflowFileName:
		return inspect(path, workflowLineLimit, workflowByteLimit)
	default:
		return inspect(path, taskLineLimit, taskByteLimit)
	}
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

func normalizeWriteMode(mode WriteMode) WriteMode {
	normalized := WriteMode(strings.ToLower(strings.TrimSpace(string(mode))))
	if normalized == "" {
		return WriteModeReplace
	}
	return normalized
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
