package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/run"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = plan.ErrNoWork

// Mode identifies the execution flow used by compozy.
type Mode string

const (
	// ModePRReview processes PR review issue markdown files.
	ModePRReview Mode = model.ModeCodeReview
	// ModePRDTasks processes PRD task markdown files.
	ModePRDTasks Mode = model.ModePRDTasks
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
)

// Config configures compozy preparation and execution.
type Config struct {
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
	Grouped                bool
	TailLines              int
	ReasoningEffort        string
	Mode                   Mode
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
	Jobs                    []Job
	InputDir                string
	ResolvedPR              string
	ResolvedName            string
	ResolvedProvider        string
	ResolvedRound           int
	InputDirPath            string
	GroupedSummariesWritten bool
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
	RootDir    string
	Name       string
	TasksDir   string
	ReviewsDir string
	DryRun     bool
}

type MigrationResult struct {
	Target                  string
	DryRun                  bool
	FilesScanned            int
	FilesMigrated           int
	FilesAlreadyFrontmatter int
	FilesSkipped            int
	FilesInvalid            int
	GroupedRegenerated      int
	MigratedPaths           []string
	InvalidPaths            []string
}

type SyncConfig struct {
	RootDir  string
	Name     string
	TasksDir string
}

type ArchiveConfig struct {
	RootDir  string
	Name     string
	TasksDir string
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
	runtimeCfg := cfg.runtime()
	if err := agent.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return nil, err
	}

	prep, err := plan.Prepare(ctx, runtimeCfg)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return nil, ErrNoWork
		}
		return nil, err
	}
	return newPreparation(prep), nil
}

// Run executes compozy end to end for the provided configuration.
func Run(ctx context.Context, cfg Config) error {
	runtimeCfg := cfg.runtime()
	if err := agent.ValidateRuntimeConfig(runtimeCfg); err != nil {
		return err
	}

	prep, err := plan.Prepare(ctx, runtimeCfg)
	if err != nil {
		if errors.Is(err, plan.ErrNoWork) {
			return nil
		}
		return err
	}
	return run.Execute(ctx, prep.Jobs, runtimeCfg)
}

func FetchReviews(ctx context.Context, cfg Config) (*FetchResult, error) {
	return fetchReviews(ctx, cfg.runtime())
}

func Migrate(ctx context.Context, cfg MigrationConfig) (*MigrationResult, error) {
	return migrateArtifacts(ctx, cfg)
}

func Sync(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	return syncTaskMetadata(ctx, cfg)
}

func Archive(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	return archiveTaskWorkflows(ctx, cfg)
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
		Grouped:                cfg.Grouped,
		TailLines:              cfg.TailLines,
		ReasoningEffort:        cfg.ReasoningEffort,
		Mode:                   model.ExecutionMode(cfg.Mode),
		IncludeCompleted:       cfg.IncludeCompleted,
		IncludeResolved:        cfg.IncludeResolved,
		Timeout:                cfg.Timeout,
		MaxRetries:             cfg.MaxRetries,
		RetryBackoffMultiplier: cfg.RetryBackoffMultiplier,
	}
	runtimeCfg.ApplyDefaults()
	return runtimeCfg
}

func newPreparation(prep *model.SolvePreparation) *Preparation {
	if prep == nil {
		return nil
	}

	jobs := make([]Job, 0, len(prep.Jobs))
	for i := range prep.Jobs {
		jobs = append(jobs, newJob(prep.Jobs[i]))
	}

	return &Preparation{
		Jobs:                    jobs,
		InputDir:                prep.InputDir,
		ResolvedName:            prep.ResolvedName,
		ResolvedPR:              prep.ResolvedPR,
		ResolvedProvider:        prep.ResolvedProvider,
		ResolvedRound:           prep.ResolvedRound,
		InputDirPath:            prep.InputDirPath,
		GroupedSummariesWritten: prep.GroupedSummarized,
	}
}

func newJob(jb model.Job) Job {
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
