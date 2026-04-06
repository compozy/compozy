package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/preputil"
	"github.com/compozy/compozy/internal/core/run"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = plan.ErrNoWork

// DispatcherAdapters lets the kernel register dispatcher-backed core adapters
// without creating an import cycle from core back to kernel.
type DispatcherAdapters struct {
	Prepare      func(context.Context, Config) (*Preparation, error)
	Run          func(context.Context, Config) error
	FetchReviews func(context.Context, Config) (*FetchResult, error)
	Migrate      func(context.Context, MigrationConfig) (*MigrationResult, error)
	Sync         func(context.Context, SyncConfig) (*SyncResult, error)
	Archive      func(context.Context, ArchiveConfig) (*ArchiveResult, error)
}

var registeredDispatcherAdapters DispatcherAdapters

// RegisterDispatcherAdapters installs dispatcher-backed adapters for the legacy
// core API entry points.
func RegisterDispatcherAdapters(adapters DispatcherAdapters) {
	registeredDispatcherAdapters = adapters
}

// Mode identifies the execution flow used by compozy.
type Mode string

const (
	// ModePRReview processes PR review issue markdown files.
	ModePRReview Mode = model.ModeCodeReview
	// ModePRDTasks processes PRD task markdown files.
	ModePRDTasks Mode = model.ModePRDTasks
	// ModeExec processes one ad hoc prompt through the shared runtime.
	ModeExec Mode = model.ModeExec
)

// IDE identifies the downstream coding tool that compozy should invoke.
type IDE string

const (
	// IDECodex runs Codex jobs.
	IDECodex IDE = model.IDECodex
	// IDEClaude runs Claude Code jobs.
	IDEClaude IDE = model.IDEClaude
	// IDEDroid runs Droid jobs.
	IDEDroid IDE = model.IDEDroid
	// IDECursor runs Cursor Agent jobs.
	IDECursor IDE = model.IDECursor
	// IDEOpenCode runs OpenCode jobs.
	IDEOpenCode IDE = model.IDEOpenCode
	// IDEPi runs Pi jobs.
	IDEPi IDE = model.IDEPi
	// IDEGemini runs Gemini jobs.
	IDEGemini IDE = model.IDEGemini
	// IDECopilot runs GitHub Copilot CLI jobs.
	IDECopilot IDE = model.IDECopilot
)

const (
	// AccessModeDefault leaves runtime permissions at the agent's native defaults.
	AccessModeDefault = model.AccessModeDefault
	// AccessModeFull requests the most permissive execution mode Compozy knows how to configure.
	AccessModeFull = model.AccessModeFull
)

// OutputFormat identifies the user-facing result contract for a run.
type OutputFormat string

const (
	// OutputFormatText keeps the standard human-oriented presentation.
	OutputFormatText OutputFormat = OutputFormat(model.OutputFormatText)
	// OutputFormatJSON emits the lean machine-readable result contract.
	OutputFormatJSON OutputFormat = OutputFormat(model.OutputFormatJSON)
	// OutputFormatRawJSON emits the full machine-readable event stream.
	OutputFormatRawJSON OutputFormat = OutputFormat(model.OutputFormatRawJSON)
)

// Config configures compozy preparation and execution.
//
// Transitional note: during Phase A of the kernel refactor this struct remains
// exported as the legacy translation shape used by CLI flag parsing and older
// call sites before they move to typed kernel commands directly.
type Config struct {
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
	IDE                    IDE
	Model                  string
	AddDirs                []string
	TailLines              int
	ReasoningEffort        string
	AccessMode             string
	Mode                   Mode
	OutputFormat           OutputFormat
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

// Job is a prepared execution unit with its generated artifacts.
type Job struct {
	CodeFiles     []string
	SafeName      string
	Prompt        []byte
	PromptPath    string
	StdoutLogPath string
	StderrLogPath string
	IssueCount    int

	groups map[string][]model.IssueEntry
}

// Preparation contains the resolved execution plan for a compozy run.
type Preparation struct {
	Jobs             []Job
	InputDir         string
	ResolvedPR       string
	ResolvedName     string
	ResolvedProvider string
	ResolvedRound    int
	InputDirPath     string
}

type FetchResult struct {
	Name       string
	Provider   string
	PR         string
	Round      int
	ReviewsDir string
	Total      int
}

type MigrationConfig struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
	ReviewsDir    string
	DryRun        bool
}

type MigrationResult struct {
	Target                  string
	DryRun                  bool
	FilesScanned            int
	FilesMigrated           int
	V1ToV2Migrated          int
	FilesAlreadyFrontmatter int
	FilesSkipped            int
	FilesInvalid            int
	MigratedPaths           []string
	UnmappedTypeFiles       []string
	InvalidPaths            []string
}

type SyncConfig struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
}

type ArchiveConfig struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
}

type SyncResult struct {
	Target           string
	WorkflowsScanned int
	MetaCreated      int
	MetaUpdated      int
	SyncedPaths      []string
}

type ArchiveResult struct {
	Target           string
	ArchiveRoot      string
	WorkflowsScanned int
	Archived         int
	Skipped          int
	ArchivedPaths    []string
	SkippedReasons   map[string]string
}

// Validate ensures the configuration is internally consistent.
func (cfg Config) Validate() error {
	if cfg.TailLines < 0 {
		return errors.New("tail-lines must be 0 or greater")
	}
	runtimeCfg := cfg.runtime()
	return agent.ValidateRuntimeConfig(runtimeCfg)
}

// Prepare resolves inputs, validates the environment, and generates batch artifacts.
func Prepare(ctx context.Context, cfg Config) (*Preparation, error) {
	if registeredDispatcherAdapters.Prepare != nil {
		return registeredDispatcherAdapters.Prepare(ctx, cfg)
	}
	return prepareDirect(ctx, cfg)
}

// Run executes compozy end to end for the provided configuration.
func Run(ctx context.Context, cfg Config) error {
	if registeredDispatcherAdapters.Run != nil {
		return registeredDispatcherAdapters.Run(ctx, cfg)
	}
	return runDirect(ctx, cfg)
}

func FetchReviews(ctx context.Context, cfg Config) (*FetchResult, error) {
	if registeredDispatcherAdapters.FetchReviews != nil {
		return registeredDispatcherAdapters.FetchReviews(ctx, cfg)
	}
	return FetchReviewsDirect(ctx, cfg)
}

func Migrate(ctx context.Context, cfg MigrationConfig) (*MigrationResult, error) {
	if registeredDispatcherAdapters.Migrate != nil {
		return registeredDispatcherAdapters.Migrate(ctx, cfg)
	}
	return MigrateDirect(ctx, cfg)
}

func Sync(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	if registeredDispatcherAdapters.Sync != nil {
		return registeredDispatcherAdapters.Sync(ctx, cfg)
	}
	return SyncDirect(ctx, cfg)
}

func Archive(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	if registeredDispatcherAdapters.Archive != nil {
		return registeredDispatcherAdapters.Archive(ctx, cfg)
	}
	return ArchiveDirect(ctx, cfg)
}

// FetchReviewsDirect preserves access to the pre-dispatch fetch implementation for kernel handlers.
func FetchReviewsDirect(ctx context.Context, cfg Config) (*FetchResult, error) {
	return fetchReviews(ctx, cfg.runtime())
}

// MigrateDirect preserves access to the pre-dispatch migration implementation for kernel handlers.
func MigrateDirect(ctx context.Context, cfg MigrationConfig) (*MigrationResult, error) {
	return migrateArtifacts(ctx, cfg)
}

// SyncDirect preserves access to the pre-dispatch sync implementation for kernel handlers.
func SyncDirect(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	return syncTaskMetadata(ctx, cfg)
}

// ArchiveDirect preserves access to the pre-dispatch archive implementation for kernel handlers.
func ArchiveDirect(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	return archiveTaskWorkflows(ctx, cfg)
}

func prepareDirect(ctx context.Context, cfg Config) (*Preparation, error) {
	runtimeCfg := cfg.runtime()
	if err := agent.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return nil, err
	}

	prep, err := plan.Prepare(ctx, runtimeCfg, nil)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return nil, ErrNoWork
		}
		return nil, err
	}
	defer preputil.ClosePreparationJournal(ctx, prep)
	return NewPreparation(prep), nil
}

func runDirect(ctx context.Context, cfg Config) error {
	runtimeCfg := cfg.runtime()
	if err := agent.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return err
	}

	if runtimeCfg.Mode == model.ExecutionModeExec {
		return run.ExecuteExec(ctx, runtimeCfg)
	}

	prep, err := plan.Prepare(ctx, runtimeCfg, nil)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return nil
		}
		return err
	}
	return run.Execute(ctx, prep.Jobs, prep.RunArtifacts, prep.Journal(), nil, runtimeCfg)
}

// NormalizeAddDirs trims, de-duplicates, and normalizes repeated add-dir values.
func NormalizeAddDirs(dirs []string) []string {
	if len(dirs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(dirs))
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func (cfg Config) runtime() *model.RuntimeConfig {
	runtimeCfg := &model.RuntimeConfig{
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
		AddDirs:                NormalizeAddDirs(cfg.AddDirs),
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
	runtimeCfg.ApplyDefaults()
	return runtimeCfg
}

// NewPreparation clones a solve preparation into the public core API shape.
func NewPreparation(prep *model.SolvePreparation) *Preparation {
	if prep == nil {
		return nil
	}

	jobs := make([]Job, 0, len(prep.Jobs))
	for i := range prep.Jobs {
		jobs = append(jobs, NewJob(prep.Jobs[i]))
	}

	return &Preparation{
		Jobs:             jobs,
		InputDir:         prep.InputDir,
		ResolvedName:     prep.ResolvedName,
		ResolvedPR:       prep.ResolvedPR,
		ResolvedProvider: prep.ResolvedProvider,
		ResolvedRound:    prep.ResolvedRound,
		InputDirPath:     prep.InputDirPath,
	}
}

// NewJob clones a model job into the public core API shape.
func NewJob(jb model.Job) Job {
	codeFiles := append([]string(nil), jb.CodeFiles...)
	prompt := append([]byte(nil), jb.Prompt...)
	return Job{
		CodeFiles:     codeFiles,
		SafeName:      jb.SafeName,
		Prompt:        prompt,
		PromptPath:    jb.OutPromptPath,
		StdoutLogPath: jb.OutLog,
		StderrLogPath: jb.ErrLog,
		IssueCount:    jb.IssueCount(),
		groups:        jb.Groups,
	}
}
