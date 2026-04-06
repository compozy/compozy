package commands

import (
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

// RuntimeFields captures the runtime-facing command fields shared by run and prepare operations.
type RuntimeFields struct {
	WorkspaceRoot          string
	Name                   string
	Round                  int
	Provider               string
	PR                     string
	ReviewsDir             string
	TasksDir               string
	DryRun                 bool
	AutoCommit             bool
	Concurrent             int
	BatchSize              int
	IDE                    string
	Model                  string
	AddDirs                []string
	TailLines              int
	ReasoningEffort        string
	AccessMode             string
	Mode                   model.ExecutionMode
	OutputFormat           model.OutputFormat
	Verbose                bool
	TUI                    bool
	Persist                bool
	RunID                  string
	PromptText             string
	PromptFile             string
	ReadPromptStdin        bool
	ResolvedPromptText     string
	IncludeCompleted       bool
	IncludeResolved        bool
	Timeout                time.Duration
	MaxRetries             int
	RetryBackoffMultiplier float64
}

// RuntimeConfig converts the command fields into the shared runtime configuration shape.
func (f RuntimeFields) RuntimeConfig() *model.RuntimeConfig {
	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          f.WorkspaceRoot,
		Name:                   f.Name,
		Round:                  f.Round,
		Provider:               f.Provider,
		PR:                     f.PR,
		ReviewsDir:             f.ReviewsDir,
		TasksDir:               f.TasksDir,
		DryRun:                 f.DryRun,
		AutoCommit:             f.AutoCommit,
		Concurrent:             f.Concurrent,
		BatchSize:              f.BatchSize,
		IDE:                    f.IDE,
		Model:                  f.Model,
		AddDirs:                core.NormalizeAddDirs(f.AddDirs),
		TailLines:              f.TailLines,
		ReasoningEffort:        f.ReasoningEffort,
		AccessMode:             f.AccessMode,
		Mode:                   f.Mode,
		OutputFormat:           f.OutputFormat,
		Verbose:                f.Verbose,
		TUI:                    f.TUI,
		Persist:                f.Persist,
		RunID:                  f.RunID,
		PromptText:             f.PromptText,
		PromptFile:             f.PromptFile,
		ReadPromptStdin:        f.ReadPromptStdin,
		ResolvedPromptText:     f.ResolvedPromptText,
		IncludeCompleted:       f.IncludeCompleted,
		IncludeResolved:        f.IncludeResolved,
		Timeout:                f.Timeout,
		MaxRetries:             f.MaxRetries,
		RetryBackoffMultiplier: f.RetryBackoffMultiplier,
	}
	cfg.ApplyDefaults()
	return cfg
}

func runtimeFieldsFromConfig(cfg core.Config) RuntimeFields {
	return RuntimeFields{
		WorkspaceRoot:          cfg.WorkspaceRoot,
		Name:                   cfg.Name,
		Round:                  cfg.Round,
		Provider:               cfg.Provider,
		PR:                     cfg.PR,
		ReviewsDir:             cfg.ReviewsDir,
		TasksDir:               cfg.TasksDir,
		DryRun:                 cfg.DryRun,
		AutoCommit:             cfg.AutoCommit,
		Concurrent:             cfg.Concurrent,
		BatchSize:              cfg.BatchSize,
		IDE:                    string(cfg.IDE),
		Model:                  cfg.Model,
		AddDirs:                append([]string(nil), cfg.AddDirs...),
		TailLines:              cfg.TailLines,
		ReasoningEffort:        cfg.ReasoningEffort,
		AccessMode:             cfg.AccessMode,
		Mode:                   model.ExecutionMode(cfg.Mode),
		OutputFormat:           model.OutputFormat(cfg.OutputFormat),
		Verbose:                cfg.Verbose,
		TUI:                    cfg.TUI,
		Persist:                cfg.Persist,
		RunID:                  cfg.RunID,
		PromptText:             cfg.PromptText,
		PromptFile:             cfg.PromptFile,
		ReadPromptStdin:        cfg.ReadPromptStdin,
		ResolvedPromptText:     cfg.ResolvedPromptText,
		IncludeCompleted:       cfg.IncludeCompleted,
		IncludeResolved:        cfg.IncludeResolved,
		Timeout:                cfg.Timeout,
		MaxRetries:             cfg.MaxRetries,
		RetryBackoffMultiplier: cfg.RetryBackoffMultiplier,
	}
}
