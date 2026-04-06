package commands

import core "github.com/compozy/compozy/internal/core"

// WorkflowSyncCommand refreshes task workflow metadata.
type WorkflowSyncCommand struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
	DryRun        bool
}

// WorkflowSyncResult wraps the existing sync result contract.
type WorkflowSyncResult struct {
	Result *core.SyncResult
}

// WorkflowSyncFromConfig translates the legacy core.Config shape into a typed sync command.
func WorkflowSyncFromConfig(cfg core.Config) WorkflowSyncCommand {
	return WorkflowSyncCommand{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
		DryRun:        cfg.DryRun,
	}
}

// WorkflowSyncFromSyncConfig translates the direct sync config into a typed sync
// command without caller-side field copying.
func WorkflowSyncFromSyncConfig(cfg core.SyncConfig) WorkflowSyncCommand {
	return WorkflowSyncCommand{
		WorkspaceRoot: cfg.WorkspaceRoot,
		RootDir:       cfg.RootDir,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
	}
}

// CoreConfig converts the command into the existing sync configuration shape.
func (c WorkflowSyncCommand) CoreConfig() core.SyncConfig {
	return core.SyncConfig{
		WorkspaceRoot: c.WorkspaceRoot,
		RootDir:       c.RootDir,
		Name:          c.Name,
		TasksDir:      c.TasksDir,
	}
}
