package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func TasksBaseDir() string {
	return TasksBaseDirForWorkspace("")
}

func TaskDirectory(name string) string {
	return TaskDirectoryForWorkspace("", name)
}

func CompozyDir(workspaceRoot string) string {
	trimmed := strings.TrimSpace(workspaceRoot)
	if trimmed == "" {
		return WorkflowRootDirName
	}
	return filepath.Join(filepath.Clean(trimmed), WorkflowRootDirName)
}

func ConfigPathForWorkspace(workspaceRoot string) string {
	return filepath.Join(CompozyDir(workspaceRoot), WorkflowConfigFileName)
}

func TasksBaseDirForWorkspace(workspaceRoot string) string {
	return filepath.Join(CompozyDir(workspaceRoot), WorkflowTasksDirName)
}

func RunsBaseDirForWorkspace(workspaceRoot string) string {
	return filepath.Join(CompozyDir(workspaceRoot), WorkflowRunsDirName)
}

func TaskDirectoryForWorkspace(workspaceRoot, name string) string {
	return filepath.Join(TasksBaseDirForWorkspace(workspaceRoot), name)
}

func ArchivedTasksDir(baseDir string) string {
	return filepath.Join(baseDir, ArchivedWorkflowDirName)
}

func ArchivedWorkflowName(slug string, workflowID string, archivedAt time.Time) string {
	timestamp := archivedAt.UTC().UnixMilli()
	shortID := ArchivedWorkflowShortID(workflowID)
	if shortID == "" {
		shortID = "workflow"
	}
	return fmt.Sprintf("%d-%s-%s", timestamp, shortID, strings.TrimSpace(slug))
}

func ArchivedWorkflowShortID(workflowID string) string {
	trimmed := strings.TrimSpace(workflowID)
	if idx := strings.IndexRune(trimmed, '-'); idx >= 0 {
		trimmed = trimmed[idx+1:]
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(unicode.ToLower(r))
		}
		if builder.Len() >= 8 {
			break
		}
	}
	return builder.String()
}

func IsActiveWorkflowDirName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && !strings.HasPrefix(trimmed, ".") && trimmed != ArchivedWorkflowDirName
}
