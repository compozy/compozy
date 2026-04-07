package model

import (
	"path/filepath"
	"strings"
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

func IsActiveWorkflowDirName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && !strings.HasPrefix(trimmed, ".") && trimmed != ArchivedWorkflowDirName
}
