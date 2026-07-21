package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/workpackages"
	"github.com/spf13/cobra"
)

const (
	daemonRunModeTask   = "task"
	daemonRunModeReview = "review"
)

type workPackagePickerInput struct {
	Target           workpackages.Target
	WorkspaceRoot    string
	RunMode          string
	LockCompleted    bool
	IncludeCompleted bool
}

type workPackagePickerOption struct {
	Value     string
	Label     string
	Completed bool
}

type reviewRoundPickerSummary struct {
	Round             int
	IssueCount        int
	PendingIssueCount int
}

func (s *commandState) workPackagePickerRunMode() string {
	switch s.kind {
	case commandKindTasksRun:
		return daemonRunModeTask
	case commandKindFixReviews:
		return daemonRunModeReview
	default:
		return ""
	}
}

func defaultPickWorkPackage(cmd *cobra.Command, input workPackagePickerInput) (string, error) {
	latestRunStatuses := map[string]string(nil)
	if strings.TrimSpace(input.RunMode) != "" {
		client, err := newCLIDaemonBootstrap().ensure(cmd.Context())
		if err != nil {
			return "", fmt.Errorf("prepare Work Package status picker: %w", err)
		}
		latestRunStatuses, err = loadWorkPackagePickerLatestRunStatuses(
			cmd.Context(),
			client,
			input.WorkspaceRoot,
			input.RunMode,
		)
		if err != nil {
			return "", err
		}
	}

	options, err := buildWorkPackagePickerOptions(input, latestRunStatuses)
	if err != nil {
		return "", err
	}
	if len(options) == 0 {
		return "", errWorkPackageSelectionCanceled
	}

	huhOptions := make([]huh.Option[string], 0, len(options))
	selected := ""
	allowCompleted := !input.LockCompleted || input.IncludeCompleted
	for index := range options {
		option := &options[index]
		huhOptions = append(huhOptions, huh.NewOption(workPackagePickerOptionLabel(*option), option.Value))
		if selected == "" && (!option.Completed || allowCompleted) {
			selected = option.Value
		}
	}

	description := "Each row includes completion, live run status, dependency readiness, and task progress."
	if input.RunMode == daemonRunModeReview {
		description = "Rows show the latest review round and pending issues. (!) means no issues are pending."
	} else if !allowCompleted {
		description = "Completed packages stay visible with a check but stay locked. Rows include status and task progress."
	}
	field := huh.NewSelect[string]().
		Title("Select a Work Package").
		Description(description).
		Options(huhOptions...).
		Filtering(true).
		Validate(func(value string) error {
			return validateWorkPackagePickerSelection(options, value, allowCompleted)
		}).
		Value(&selected)
	form := huh.NewForm(huh.NewGroup(field))
	if err := form.Run(); err != nil {
		return "", err
	}
	if err := validateWorkPackagePickerSelection(options, selected, allowCompleted); err != nil {
		return "", err
	}
	return strings.TrimSpace(selected), nil
}

func loadReviewFixTargetPickerOptions(
	ctx context.Context,
	client daemonCommandClient,
	workspaceRoot string,
) ([]workPackagePickerOption, error) {
	latestRunStatuses, err := loadWorkPackagePickerLatestRunStatuses(
		ctx,
		client,
		workspaceRoot,
		daemonRunModeReview,
	)
	if err != nil {
		return nil, err
	}
	return buildReviewFixTargetPickerOptions(ctx, workspaceRoot, latestRunStatuses)
}

func buildWorkPackagePickerOptions(
	input workPackagePickerInput,
	latestRunStatuses map[string]string,
) ([]workPackagePickerOption, error) {
	target := input.Target
	baseDir := filepath.Dir(target.InitiativeDir)
	options := make([]workPackagePickerOption, 0, len(target.Plan.Packages))
	for index := range target.Plan.Packages {
		pkg := target.Plan.Packages[index]
		option, err := buildWorkPackagePickerOption(
			input,
			baseDir,
			pkg,
			pkg.ID,
			pkg.ID,
			latestRunStatuses,
		)
		if err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, nil
}

func buildReviewFixTargetPickerOptions(
	ctx context.Context,
	workspaceRoot string,
	latestRunStatuses map[string]string,
) ([]workPackagePickerOption, error) {
	baseDir := model.TasksBaseDirForWorkspace(workspaceRoot)
	slugs := listTaskSubdirs(baseDir)
	options := make([]workPackagePickerOption, 0, len(slugs))
	resolver := workpackages.TargetResolver{}
	for _, slug := range slugs {
		target, err := resolver.Resolve(ctx, workspaceRoot, slug)
		if err != nil {
			return nil, fmt.Errorf("resolve review target %s: %w", slug, err)
		}
		if target.Mode != workpackages.TargetModeInitiative {
			option, err := buildOrdinaryReviewFixTargetPickerOption(baseDir, slug, latestRunStatuses[slug])
			if err != nil {
				return nil, err
			}
			options = append(options, option)
			continue
		}

		input := workPackagePickerInput{Target: target, RunMode: daemonRunModeReview}
		for index := range target.Plan.Packages {
			pkg := target.Plan.Packages[index]
			reference := target.Ref.Initiative + "/" + pkg.ID
			option, err := buildWorkPackagePickerOption(
				input,
				baseDir,
				pkg,
				reference,
				reference,
				latestRunStatuses,
			)
			if err != nil {
				return nil, err
			}
			options = append(options, option)
		}
	}
	return options, nil
}

func buildWorkPackagePickerOption(
	input workPackagePickerInput,
	baseDir string,
	pkg workpackages.Package,
	value string,
	displayReference string,
	latestRunStatuses map[string]string,
) (workPackagePickerOption, error) {
	target := input.Target
	if _, err := workpackages.EvaluateReadiness(target.Plan, pkg.ID); err != nil {
		return workPackagePickerOption{}, err
	}
	workflowOption := taskRunWizardPackageOption(
		baseDir,
		target.Ref.Initiative,
		target.Plan,
		pkg,
		latestRunStatuses,
	)
	workflowOption.Label = displayReference + " — " + pkg.Title
	if input.RunMode == daemonRunModeReview {
		workflowOption.Status = reviewFixPickerStatus(latestRunStatuses[target.Ref.Initiative+"/"+pkg.ID])
		workflowOption.BlockedBy = nil
	}

	mark := "[ ]"
	if workflowOption.Completed {
		mark = "[✓]"
	}
	if input.RunMode == daemonRunModeReview {
		summary, err := latestReviewRoundPickerSummary(filepath.Join(
			target.InitiativeDir,
			filepath.FromSlash(pkg.Directory),
		))
		if err != nil {
			return workPackagePickerOption{}, err
		}
		label := mark + " " + workflowOption.Label + " — " + reviewRoundPickerSummaryLabel(summary)
		return workPackagePickerOption{Value: value, Label: label, Completed: workflowOption.Completed}, nil
	}
	label := mark + " " + taskRunWizardWorkflowOptionLabel(workflowOption)
	return workPackagePickerOption{Value: value, Label: label, Completed: workflowOption.Completed}, nil
}

func buildOrdinaryReviewFixTargetPickerOption(
	baseDir string,
	slug string,
	latestRunStatus string,
) (workPackagePickerOption, error) {
	workflowOption := taskRunWizardOrdinaryOption(baseDir, slug, latestRunStatus)
	workflowOption.Status = reviewFixPickerStatus(latestRunStatus)
	mark := "[ ]"
	if workflowOption.Completed {
		mark = "[✓]"
	}
	summary, err := latestReviewRoundPickerSummary(filepath.Join(baseDir, slug))
	if err != nil {
		return workPackagePickerOption{}, err
	}
	return workPackagePickerOption{
		Value: slug,
		Label: mark + " " + workflowOption.Label + " — " + reviewRoundPickerSummaryLabel(
			summary,
		),
		Completed: workflowOption.Completed,
	}, nil
}

func reviewFixPickerStatus(latestRunStatus string) taskRunWizardWorkflowStatus {
	return taskRunWizardStatus(false, false, latestRunStatus)
}

func latestReviewRoundPickerSummary(reviewRoot string) (reviewRoundPickerSummary, error) {
	round, found, err := latestLocalReviewRoundInDir(reviewRoot)
	if err != nil {
		return reviewRoundPickerSummary{}, err
	}
	if !found {
		return reviewRoundPickerSummary{}, nil
	}
	return readReviewRoundPickerSummary(filepath.Join(reviewRoot, reviews.RoundDirName(round)), round)
}

func readReviewRoundPickerSummary(reviewDir string, round int) (reviewRoundPickerSummary, error) {
	entries, err := reviews.ReadReviewEntries(reviewDir)
	if err != nil {
		return reviewRoundPickerSummary{}, fmt.Errorf("read review round summary %s: %w", reviewDir, err)
	}
	summary := reviewRoundPickerSummary{Round: round, IssueCount: len(entries)}
	for index := range entries {
		resolved, err := reviews.IsReviewResolved(entries[index].Content)
		if err != nil {
			return reviewRoundPickerSummary{}, reviews.WrapParseError(entries[index].AbsPath, err)
		}
		if !resolved {
			summary.PendingIssueCount++
		}
	}
	return summary, nil
}

func reviewRoundPickerSummaryLabel(summary reviewRoundPickerSummary) string {
	if summary.Round <= 0 {
		return "No review round — (!) No issues pending"
	}
	pendingLabel := "(!) No issues pending"
	if summary.PendingIssueCount > 0 {
		pendingLabel = fmt.Sprintf("%d issues pending", summary.PendingIssueCount)
		if summary.PendingIssueCount == 1 {
			pendingLabel = "1 issue pending"
		}
	}
	return fmt.Sprintf("Review round %d — %s", summary.Round, pendingLabel)
}

func workPackagePickerOptionLabel(option workPackagePickerOption) string {
	if !option.Completed {
		return option.Label
	}
	return xansi.SGR(xansi.AttrStrikethrough) + option.Label + xansi.SGR(xansi.AttrNoStrikethrough)
}

func validateWorkPackagePickerSelection(
	options []workPackagePickerOption,
	selected string,
	allowCompleted bool,
) error {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return errWorkPackageSelectionCanceled
	}
	for index := range options {
		option := &options[index]
		if option.Value != selected {
			continue
		}
		if option.Completed && !allowCompleted {
			return fmt.Errorf("%s: completed Work Package is locked", selected)
		}
		return nil
	}
	return fmt.Errorf("unknown Work Package %q", selected)
}
