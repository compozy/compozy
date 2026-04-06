package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/pkg/compozy/events"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = errors.New("no issues to process")

func Prepare(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	bus *events.Bus[events.Event],
) (*model.SolvePreparation, error) {
	prep := &model.SolvePreparation{}
	var prepared bool
	defer func() {
		if prepared {
			return
		}
		closePreparedJournal(ctx, prep)
	}()

	if cfg.Mode == model.ExecutionModeExec {
		execPrep, err := prepareExec(prep, cfg, bus)
		if err != nil {
			return nil, err
		}
		prepared = true
		return execPrep, nil
	}

	if err := prepareWorkflowRun(prep, cfg, bus); err != nil {
		return nil, err
	}

	prepared = true
	return prep, nil
}

func prepareWorkflowRun(
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	bus *events.Bus[events.Event],
) error {
	entries, err := resolvePreparedEntries(prep, cfg)
	if err != nil {
		return err
	}

	prep.RunArtifacts, err = allocateRunArtifacts(cfg)
	if err != nil {
		return err
	}
	prep.Journal, err = journal.Open(prep.RunArtifacts.EventsPath, bus, 0)
	if err != nil {
		return fmt.Errorf("open run journal: %w", err)
	}

	prep.Jobs, err = prepareJobs(cfg, groupIssues(entries), prep.RunArtifacts)
	if err != nil {
		return err
	}

	return writeRunMetadata(prep, cfg)
}

func resolvePreparedEntries(prep *model.SolvePreparation, cfg *model.RuntimeConfig) ([]model.IssueEntry, error) {
	var err error
	prep.ResolvedName, prep.InputDir, prep.InputDirPath, err = resolveInputs(cfg)
	if err != nil {
		return nil, err
	}
	if err := configureWorkflowInput(prep, cfg); err != nil {
		return nil, err
	}
	if err := agent.EnsureAvailable(cfg); err != nil {
		return nil, err
	}

	entries, err := readIssueEntries(prep.InputDirPath, cfg.Mode, cfg.IncludeCompleted)
	if err != nil {
		return nil, err
	}
	return validateAndFilterEntries(entries, cfg)
}

func configureWorkflowInput(prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
	if cfg.Mode == model.ExecutionModePRReview {
		return configureReviewInput(prep, cfg)
	}

	if _, err := tasks.RefreshTaskMeta(prep.InputDirPath); err != nil {
		return err
	}
	cfg.TasksDir = prep.InputDirPath
	return nil
}

func configureReviewInput(prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
	meta, err := reviews.ReadRoundMeta(prep.InputDirPath)
	if err != nil {
		return err
	}
	cfg.Provider = meta.Provider
	cfg.PR = meta.PR
	cfg.Round = meta.Round
	cfg.ReviewsDir = prep.InputDirPath
	prep.ResolvedProvider = meta.Provider
	prep.ResolvedPR = meta.PR
	prep.ResolvedRound = meta.Round
	return nil
}

func prepareExec(
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	bus *events.Bus[events.Event],
) (*model.SolvePreparation, error) {
	promptText, err := resolveExecPrompt(cfg)
	if err != nil {
		return nil, err
	}
	if err := agent.EnsureAvailable(cfg); err != nil {
		return nil, err
	}

	prep.ResolvedName, prep.InputDir, prep.InputDirPath, err = resolveInputs(cfg)
	if err != nil {
		return nil, err
	}
	prep.RunArtifacts, err = allocateRunArtifacts(cfg)
	if err != nil {
		return nil, err
	}
	prep.Journal, err = journal.Open(prep.RunArtifacts.EventsPath, bus, 0)
	if err != nil {
		return nil, fmt.Errorf("open run journal: %w", err)
	}

	job, err := buildExecJob(prep.RunArtifacts, promptText)
	if err != nil {
		return nil, err
	}
	prep.Jobs = []model.Job{job}

	if err := writeRunMetadata(prep, cfg); err != nil {
		return nil, err
	}
	return prep, nil
}

func closePreparedJournal(ctx context.Context, prep *model.SolvePreparation) {
	if prep == nil || prep.Journal == nil {
		return
	}

	closeCtx := ctx
	if closeCtx == nil {
		closeCtx = context.Background()
	}
	if _, hasDeadline := closeCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		closeCtx, cancel = context.WithTimeout(closeCtx, time.Second)
		defer cancel()
	}
	_ = prep.Journal.Close(closeCtx)
	prep.Journal = nil
}

func prepareJobs(
	cfg *model.RuntimeConfig,
	groups map[string][]model.IssueEntry,
	runArtifacts model.RunArtifacts,
) ([]model.Job, error) {
	effectiveBatchSize := cfg.BatchSize
	if cfg.Mode == model.ExecutionModePRDTasks {
		effectiveBatchSize = 1
	}
	if effectiveBatchSize <= 0 {
		effectiveBatchSize = 1
	}

	collected := prompt.FlattenAndSortIssues(groups, cfg.Mode)
	batches := createIssueBatches(collected, effectiveBatchSize)
	if len(batches) == 0 {
		return nil, errors.New("no batches created for prompt preparation")
	}

	jobs := make([]model.Job, 0, len(batches))
	for idx, batchIssues := range batches {
		job, err := buildBatchJob(cfg, runArtifacts, idx, batchIssues)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		return nil, errors.New("no jobs finalized")
	}
	return jobs, nil
}

func buildBatchJob(
	cfg *model.RuntimeConfig,
	runArtifacts model.RunArtifacts,
	batchIdx int,
	batchIssues []model.IssueEntry,
) (model.Job, error) {
	batchGroups, batchFiles := groupIssuesByCodeFile(batchIssues)
	safeName := determineBatchName(batchIdx, batchFiles, cfg.Mode)
	var (
		taskData model.TaskEntry
		err      error
	)
	params := prompt.BatchParams{
		Name:        cfg.Name,
		Round:       cfg.Round,
		Provider:    cfg.Provider,
		PR:          cfg.PR,
		ReviewsDir:  cfg.ReviewsDir,
		BatchGroups: batchGroups,
		AutoCommit:  cfg.AutoCommit,
		Mode:        cfg.Mode,
	}
	if cfg.Mode == model.ExecutionModePRDTasks {
		if len(batchIssues) == 0 {
			return model.Job{}, errors.New("prepare prd job: missing task issue")
		}
		taskData, err = prompt.ParseTaskFile(batchIssues[0].Content)
		if err != nil {
			return model.Job{}, wrapTaskParseError(batchIssues[0].AbsPath, err)
		}
		memoryCtx, err := memory.Prepare(cfg.TasksDir, batchIssues[0].Name)
		if err != nil {
			return model.Job{}, fmt.Errorf("prepare memory for %s: %w", batchIssues[0].AbsPath, err)
		}
		params.Memory = &prompt.WorkflowMemoryContext{
			Directory:               memoryCtx.Directory,
			WorkflowPath:            memoryCtx.Workflow.Path,
			TaskPath:                memoryCtx.Task.Path,
			WorkflowNeedsCompaction: memoryCtx.Workflow.NeedsCompaction,
			TaskNeedsCompaction:     memoryCtx.Task.NeedsCompaction,
		}
	}

	promptText := prompt.Build(params)
	systemPrompt := prompt.BuildSystemPromptAddendum(params)
	outPromptPath, outLog, errLog, err := writeBatchArtifacts(runArtifacts, safeName, promptText)
	if err != nil {
		return model.Job{}, err
	}
	return model.Job{
		CodeFiles:     batchFiles,
		Groups:        batchGroups,
		TaskTitle:     taskData.Title,
		TaskType:      taskData.TaskType,
		SafeName:      safeName,
		Prompt:        []byte(promptText),
		SystemPrompt:  systemPrompt,
		OutPromptPath: outPromptPath,
		OutLog:        outLog,
		ErrLog:        errLog,
	}, nil
}

func buildExecJob(runArtifacts model.RunArtifacts, promptText string) (model.Job, error) {
	const safeName = "exec"

	outPromptPath, outLog, errLog, err := writeBatchArtifacts(runArtifacts, safeName, promptText)
	if err != nil {
		return model.Job{}, err
	}

	return model.Job{
		CodeFiles: []string{safeName},
		Groups: map[string][]model.IssueEntry{
			safeName: {{
				Name:     safeName,
				Content:  promptText,
				CodeFile: safeName,
			}},
		},
		SafeName:      safeName,
		Prompt:        []byte(promptText),
		OutPromptPath: outPromptPath,
		OutLog:        outLog,
		ErrLog:        errLog,
	}, nil
}

func allocateRunArtifacts(cfg *model.RuntimeConfig) (model.RunArtifacts, error) {
	runArtifacts := model.NewRunArtifacts(cfg.WorkspaceRoot, buildRunID(cfg))
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		return model.RunArtifacts{}, fmt.Errorf("mkdir run artifacts: %w", err)
	}
	return runArtifacts, nil
}

func buildRunID(cfg *model.RuntimeConfig) string {
	label := runLabel(cfg)
	timestamp := time.Now().UTC().Format("20060102-150405-000000000")
	return fmt.Sprintf("%s-%s", label, timestamp)
}

func runLabel(cfg *model.RuntimeConfig) string {
	if cfg.Mode == model.ExecutionModeExec {
		return "exec"
	}
	if cfg.Mode == model.ExecutionModePRDTasks {
		return "tasks-" + prompt.SafeFileName(cfg.Name)
	}
	scope := cfg.Name
	if scope == "" {
		scope = "pr-" + cfg.PR
	}
	return fmt.Sprintf("reviews-%s-round-%03d", prompt.SafeFileName(scope), cfg.Round)
}

func determineBatchName(batchIdx int, batchFiles []string, mode model.ExecutionMode) string {
	if mode == model.ExecutionModePRDTasks {
		if len(batchFiles) == 1 {
			return prompt.SafeFileName(batchFiles[0])
		}
		return fmt.Sprintf("task_%03d", batchIdx+1)
	}
	if len(batchFiles) == 1 {
		filename := batchFiles[0]
		if strings.HasPrefix(filename, "__unknown__") {
			filename = model.UnknownFileName
		}
		return prompt.SafeFileName(filename)
	}
	return fmt.Sprintf("batch_%03d", batchIdx+1)
}

func writeBatchArtifacts(runArtifacts model.RunArtifacts, safeName, promptText string) (string, string, string, error) {
	jobArtifacts := runArtifacts.JobArtifacts(safeName)
	outPromptPath := jobArtifacts.PromptPath
	if err := os.WriteFile(outPromptPath, []byte(promptText), 0o600); err != nil {
		return "", "", "", fmt.Errorf("write prompt: %w", err)
	}
	outLog := jobArtifacts.OutLogPath
	errLog := jobArtifacts.ErrLogPath
	for _, logPath := range []string{outLog, errLog} {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return "", "", "", fmt.Errorf("create log artifact %s: %w", logPath, err)
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", "", "", fmt.Errorf("close log artifact %s: %w", logPath, closeErr)
		}
	}
	return outPromptPath, outLog, errLog, nil
}

type runMetadata struct {
	RunID        string    `json:"run_id"`
	Mode         string    `json:"mode"`
	IDE          string    `json:"ide"`
	Model        string    `json:"model"`
	OutputFormat string    `json:"output_format"`
	PromptSource string    `json:"prompt_source,omitempty"`
	PromptFile   string    `json:"prompt_file,omitempty"`
	ArtifactsDir string    `json:"artifacts_dir"`
	JobsDir      string    `json:"jobs_dir"`
	ResultPath   string    `json:"result_path"`
	JobCount     int       `json:"job_count"`
	CreatedAt    time.Time `json:"created_at"`
}

func writeRunMetadata(prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
	if prep == nil {
		return errors.New("missing preparation for run metadata")
	}
	payload, err := json.MarshalIndent(
		runMetadata{
			RunID:        prep.RunArtifacts.RunID,
			Mode:         string(cfg.Mode),
			IDE:          cfg.IDE,
			Model:        cfg.Model,
			OutputFormat: string(cfg.OutputFormat),
			PromptSource: promptSourceForConfig(cfg),
			PromptFile:   strings.TrimSpace(cfg.PromptFile),
			ArtifactsDir: prep.RunArtifacts.RunDir,
			JobsDir:      prep.RunArtifacts.JobsDir,
			ResultPath:   prep.RunArtifacts.ResultPath,
			JobCount:     len(prep.Jobs),
			CreatedAt:    time.Now().UTC(),
		},
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	if err := os.WriteFile(prep.RunArtifacts.RunMetaPath, payload, 0o600); err != nil {
		return fmt.Errorf("write run metadata: %w", err)
	}
	return nil
}

func promptSourceForConfig(cfg *model.RuntimeConfig) string {
	switch {
	case strings.TrimSpace(cfg.PromptFile) != "":
		return "file"
	case cfg.ReadPromptStdin:
		return "stdin"
	case strings.TrimSpace(cfg.PromptText) != "":
		return "positional"
	default:
		return ""
	}
}

func createIssueBatches(allIssues []model.IssueEntry, batchSize int) [][]model.IssueEntry {
	batches := make([][]model.IssueEntry, 0)
	for i := 0; i < len(allIssues); i += batchSize {
		end := i + batchSize
		if end > len(allIssues) {
			end = len(allIssues)
		}
		batches = append(batches, allIssues[i:end])
	}
	return batches
}

func groupIssuesByCodeFile(issues []model.IssueEntry) (map[string][]model.IssueEntry, []string) {
	batchGroups := make(map[string][]model.IssueEntry)
	for _, issue := range issues {
		batchGroups[issue.CodeFile] = append(batchGroups[issue.CodeFile], issue)
	}
	batchFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		batchFiles = append(batchFiles, codeFile)
	}
	sort.Strings(batchFiles)
	return batchGroups, batchFiles
}
