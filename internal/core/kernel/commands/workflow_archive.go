package commands

import core "github.com/compozy/compozy/internal/core"

// WorkflowArchiveCommand archives completed workflow directories.
type WorkflowArchiveCommand struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
}

// WorkflowArchiveResult wraps the existing archive result contract.
type WorkflowArchiveResult struct {
	Result *core.ArchiveResult
}

// WorkflowArchiveFromConfig translates the legacy core.Config shape into a typed archive command.
func WorkflowArchiveFromConfig(cfg core.Config) WorkflowArchiveCommand {
	return WorkflowArchiveCommand{
		WorkspaceRoot: cfg.WorkspaceRoot,
		Name:          cfg.Name,
		TasksDir:      cfg.TasksDir,
	}
}

// CoreConfig converts the command into the existing archive configuration shape.
func (c WorkflowArchiveCommand) CoreConfig() core.ArchiveConfig {
	return core.ArchiveConfig{
		WorkspaceRoot: c.WorkspaceRoot,
		RootDir:       c.RootDir,
		Name:          c.Name,
		TasksDir:      c.TasksDir,
	}
}
