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
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/spf13/cobra"
)

const (
	daemonRunModeTask                  = "task"
	daemonRunModeReview                = "review"
	taskGroupPickerUnselectedMarker    = "[ ]"
	taskGroupPickerSelectedMarker      = "[x]"
	taskGroupPickerNotStartedMarker    = "[!]"
	taskGroupPickerBlockedMarker       = "[⊘]"
	taskGroupPickerCompletedMarker     = "[✓]"
	reviewImplementationBlockedReason  = "review is blocked until at least one implementation task is complete"
	reviewNoPendingIssuesReason        = "review target has no pending issues"
	reviewNoActionableTaskGroupsReason = "review target has no Task Groups with pending issues that can be fixed"
)

type taskGroupPickerInput struct {
	Target           taskgroups.Target
	WorkspaceRoot    string
	RunMode          string
	LockCompleted    bool
	IncludeCompleted bool
}

type taskGroupPickerOption struct {
	Value                  string
	Label                  string
	Depth                  int
	Completed              bool
	SelectionBlocked       bool
	SelectionBlockedReason string
}

type reviewRoundPickerSummary struct {
	Round             int
	IssueCount        int
	PendingIssueCount int
}

func (s *commandState) taskGroupPickerRunMode() string {
	switch s.kind {
	case commandKindTasksRun:
		return daemonRunModeTask
	case commandKindFixReviews:
		return daemonRunModeReview
	default:
		return ""
	}
}

// loadTaskGroupPickerOptions resolves the picker rows and the completion-lock
// policy shared by the single-select and multi-select pickers, loading live run
// statuses from the daemon when a run mode is set.
func loadTaskGroupPickerOptions(
	cmd *cobra.Command,
	input taskGroupPickerInput,
) ([]taskGroupPickerOption, bool, error) {
	latestRunStatuses := map[string]string(nil)
	if strings.TrimSpace(input.RunMode) != "" {
		client, err := newCLIDaemonBootstrap().ensure(cmd.Context())
		if err != nil {
			return nil, false, fmt.Errorf("prepare Task Group status picker: %w", err)
		}
		latestRunStatuses, err = loadTaskGroupPickerLatestRunStatuses(
			cmd.Context(),
			client,
			input.WorkspaceRoot,
			input.RunMode,
		)
		if err != nil {
			return nil, false, err
		}
	}

	options, err := buildTaskGroupPickerOptions(input, latestRunStatuses)
	if err != nil {
		return nil, false, err
	}
	allowCompleted := !input.LockCompleted || input.IncludeCompleted
	return options, allowCompleted, nil
}

func taskGroupPickerHuhOptions(options []taskGroupPickerOption) []huh.Option[string] {
	huhOptions := make([]huh.Option[string], 0, len(options))
	for index := range options {
		option := &options[index]
		huhOptions = append(huhOptions, huh.NewOption(taskGroupPickerOptionLabel(*option), option.Value))
	}
	return huhOptions
}

func taskGroupPickerDescription(input taskGroupPickerInput, allowCompleted bool) string {
	description := "Each row includes completion, live run status, dependency readiness, and task progress. " +
		"[⊘] means dependency blocked; [!] means no tasks are complete."
	if input.RunMode == daemonRunModeReview {
		description = "Rows without pending issues stay visible but stay locked. " +
			"[⊘] means no implementation tasks are complete and review is blocked."
	} else if !allowCompleted {
		description = "Completed task groups stay visible with a check but stay locked. " +
			"Rows include status and task progress; [⊘] means dependency blocked; [!] means no tasks are complete."
	}
	return description
}

func defaultPickTaskGroup(cmd *cobra.Command, input taskGroupPickerInput) (string, error) {
	options, allowCompleted, err := loadTaskGroupPickerOptions(cmd, input)
	if err != nil {
		return "", err
	}
	if len(options) == 0 {
		return "", errTaskGroupSelectionCanceled
	}

	huhOptions := taskGroupPickerHuhOptions(options)
	selected := defaultTaskGroupPickerSelection(options, allowCompleted)
	field := huh.NewSelect[string]().
		Title("Select a Task Group").
		Description(taskGroupPickerDescription(input, allowCompleted)).
		Options(huhOptions...).
		Validate(func(value string) error {
			return validateTaskGroupPickerSelection(options, value, allowCompleted)
		}).
		Value(&selected)
	if input.RunMode != daemonRunModeReview {
		field = field.Filtering(true)
	}
	form := huh.NewForm(huh.NewGroup(field))
	if err := form.Run(); err != nil {
		return "", err
	}
	if err := validateTaskGroupPickerSelection(options, selected, allowCompleted); err != nil {
		return "", err
	}
	return strings.TrimSpace(selected), nil
}

// defaultPickTaskGroups is the multi-select variant used by the
// --parallel-task-groups interactive journey: it returns every chosen Task
// Group ID so the launcher can run an independent set concurrently. It reuses
// the same rows and completion-lock policy as the single-select picker.
func defaultPickTaskGroups(cmd *cobra.Command, input taskGroupPickerInput) ([]string, error) {
	options, allowCompleted, err := loadTaskGroupPickerOptions(cmd, input)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, errTaskGroupSelectionCanceled
	}

	huhOptions := taskGroupPickerHuhOptions(options)
	selected := []string(nil)
	description := "Select one or more dependency-independent Task Groups to run in parallel on isolated " +
		"result branches. [⊘] means dependency blocked; [!] means no tasks are complete; completed groups stay locked."
	field := huh.NewMultiSelect[string]().
		Title("Select Task Groups to run in parallel").
		Description(description).
		Options(huhOptions...).
		Limit(len(huhOptions)).
		Validate(func(values []string) error {
			return validateTaskGroupPickerMultiSelection(options, values, allowCompleted)
		}).
		Value(&selected)
	form := huh.NewForm(huh.NewGroup(field))
	if err := form.Run(); err != nil {
		return nil, err
	}
	if err := validateTaskGroupPickerMultiSelection(options, selected, allowCompleted); err != nil {
		return nil, err
	}
	return normalizeTaskGroupSelections(selected), nil
}

func defaultTaskGroupPickerSelection(options []taskGroupPickerOption, allowCompleted bool) string {
	for index := range options {
		option := &options[index]
		if option.SelectionBlocked || (option.Completed && !allowCompleted) {
			continue
		}
		return option.Value
	}
	return ""
}

func loadReviewFixTargetPickerOptions(
	ctx context.Context,
	client daemonCommandClient,
	workspaceRoot string,
) ([]taskGroupPickerOption, error) {
	latestRunStatuses, err := loadTaskGroupPickerLatestRunStatuses(
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

func buildTaskGroupPickerOptions(
	input taskGroupPickerInput,
	latestRunStatuses map[string]string,
) ([]taskGroupPickerOption, error) {
	target := input.Target
	baseDir := filepath.Dir(target.InitiativeDir)
	options := make([]taskGroupPickerOption, 0, len(target.Plan.TaskGroups))
	for index := range target.Plan.TaskGroups {
		taskGroup := target.Plan.TaskGroups[index]
		option, err := buildTaskGroupPickerOption(
			input,
			baseDir,
			taskGroup,
			taskGroup.ID,
			taskGroup.ID,
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
) ([]taskGroupPickerOption, error) {
	baseDir := model.TasksBaseDirForWorkspace(workspaceRoot)
	slugs := listTaskSubdirs(baseDir)
	options := make([]taskGroupPickerOption, 0, len(slugs))
	resolver := taskgroups.TargetResolver{}
	for _, slug := range slugs {
		target, err := resolver.Resolve(ctx, workspaceRoot, slug)
		if err != nil {
			return nil, fmt.Errorf("resolve review target %s: %w", slug, err)
		}
		if target.Mode != taskgroups.TargetModeInitiative {
			option, err := buildOrdinaryReviewFixTargetPickerOption(baseDir, slug, latestRunStatuses[slug])
			if err != nil {
				return nil, err
			}
			options = append(options, option)
			continue
		}

		input := taskGroupPickerInput{Target: target, RunMode: daemonRunModeReview}
		children := make([]taskGroupPickerOption, 0, len(target.Plan.TaskGroups))
		hasActionableTaskGroup := false
		for index := range target.Plan.TaskGroups {
			taskGroup := target.Plan.TaskGroups[index]
			reference := target.Ref.Initiative + "/" + taskGroup.ID
			option, err := buildTaskGroupPickerOption(
				input,
				baseDir,
				taskGroup,
				reference,
				taskGroup.ID,
				latestRunStatuses,
			)
			if err != nil {
				return nil, err
			}
			option.Depth = 1
			children = append(children, option)
			hasActionableTaskGroup = hasActionableTaskGroup || !option.SelectionBlocked
		}
		root := taskGroupPickerOption{
			Value: slug,
			Label: taskGroupPickerUnselectedMarker + " " + slug,
		}
		if !hasActionableTaskGroup {
			root.SelectionBlocked = true
			root.SelectionBlockedReason = reviewNoActionableTaskGroupsReason
		}
		options = append(options, root)
		options = append(options, children...)
	}
	return options, nil
}

func buildTaskGroupPickerOption(
	input taskGroupPickerInput,
	baseDir string,
	taskGroup taskgroups.TaskGroup,
	value string,
	displayReference string,
	latestRunStatuses map[string]string,
) (taskGroupPickerOption, error) {
	target := input.Target
	if _, err := taskgroups.EvaluateReadiness(target.Plan, taskGroup.ID); err != nil {
		return taskGroupPickerOption{}, err
	}
	workflowOption := taskRunWizardTaskGroupOption(
		baseDir,
		target.Ref.Initiative,
		target.Plan,
		taskGroup,
		latestRunStatuses,
	)
	workflowOption.Label = displayReference + " — " + taskGroup.Title
	if input.RunMode == daemonRunModeReview {
		workflowOption.Status = reviewFixPickerStatus(latestRunStatuses[target.Ref.Initiative+"/"+taskGroup.ID])
		workflowOption.BlockedBy = nil
	}

	if input.RunMode == daemonRunModeReview {
		summary, err := reviewRoundPickerSummaryAcrossRounds(filepath.Join(
			target.InitiativeDir,
			filepath.FromSlash(taskGroup.Directory),
		))
		if err != nil {
			return taskGroupPickerOption{}, err
		}
		completed := reviewRoundPickerCompleted(workflowOption.Completed, summary)
		reviewBlocked := taskRunWizardWorkflowNotStarted(workflowOption)
		selectionBlockedReason := reviewFixSelectionBlockedReason(reviewBlocked, summary)
		mark := taskGroupPickerMarker(completed, reviewBlocked, false)
		label := mark + " " + workflowOption.Label + " — " + reviewRoundPickerSummaryLabel(summary)
		return taskGroupPickerOption{
			Value:                  value,
			Label:                  label,
			Completed:              completed,
			SelectionBlocked:       selectionBlockedReason != "",
			SelectionBlockedReason: selectionBlockedReason,
		}, nil
	}
	mark := taskGroupPickerMarker(
		workflowOption.Completed,
		workflowOption.Status == taskRunWizardWorkflowBlocked,
		taskRunWizardWorkflowNotStarted(workflowOption),
	)
	label := mark + " " + taskRunWizardWorkflowOptionLabel(workflowOption)
	return taskGroupPickerOption{Value: value, Label: label, Completed: workflowOption.Completed}, nil
}

func buildOrdinaryReviewFixTargetPickerOption(
	baseDir string,
	slug string,
	latestRunStatus string,
) (taskGroupPickerOption, error) {
	workflowOption := taskRunWizardOrdinaryOption(baseDir, slug, latestRunStatus)
	workflowOption.Status = reviewFixPickerStatus(latestRunStatus)
	summary, err := reviewRoundPickerSummaryAcrossRounds(filepath.Join(baseDir, slug))
	if err != nil {
		return taskGroupPickerOption{}, err
	}
	completed := reviewRoundPickerCompleted(workflowOption.Completed, summary)
	reviewBlocked := taskRunWizardWorkflowNotStarted(workflowOption)
	selectionBlockedReason := reviewFixSelectionBlockedReason(reviewBlocked, summary)
	mark := taskGroupPickerMarker(completed, reviewBlocked, false)
	return taskGroupPickerOption{
		Value:                  slug,
		SelectionBlocked:       selectionBlockedReason != "",
		SelectionBlockedReason: selectionBlockedReason,
		Label: mark + " " + workflowOption.Label + " — " + reviewRoundPickerSummaryLabel(
			summary,
		),
		Completed: completed,
	}, nil
}

func reviewFixSelectionBlockedReason(
	implementationBlocked bool,
	summary reviewRoundPickerSummary,
) string {
	if implementationBlocked {
		return reviewImplementationBlockedReason
	}
	if summary.PendingIssueCount == 0 {
		return reviewNoPendingIssuesReason
	}
	return ""
}

func reviewRoundPickerCompleted(
	implementationCompleted bool,
	summary reviewRoundPickerSummary,
) bool {
	return implementationCompleted && summary.Round > 0 && summary.PendingIssueCount == 0
}

func reviewFixPickerStatus(latestRunStatus string) taskRunWizardWorkflowStatus {
	return taskRunWizardStatus(false, false, latestRunStatus)
}

func reviewRoundPickerSummaryAcrossRounds(reviewRoot string) (reviewRoundPickerSummary, error) {
	rounds, err := reviews.DiscoverRounds(reviewRoot)
	if err != nil {
		return reviewRoundPickerSummary{}, err
	}
	summary := reviewRoundPickerSummary{}
	for _, round := range rounds {
		roundSummary, err := readReviewRoundPickerSummary(
			filepath.Join(reviewRoot, reviews.RoundDirName(round)),
			round,
		)
		if err != nil {
			return reviewRoundPickerSummary{}, err
		}
		if roundSummary.IssueCount == 0 {
			continue
		}
		summary.Round = round
		summary.IssueCount += roundSummary.IssueCount
		summary.PendingIssueCount += roundSummary.PendingIssueCount
	}
	return summary, nil
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
		return "No review round — No issues pending"
	}
	pendingLabel := "No issues pending"
	if summary.PendingIssueCount > 0 {
		pendingLabel = fmt.Sprintf("%d issues pending", summary.PendingIssueCount)
		if summary.PendingIssueCount == 1 {
			pendingLabel = "1 issue pending"
		}
	}
	return fmt.Sprintf("Review round %d — %s", summary.Round, pendingLabel)
}

func taskGroupPickerMarker(completed bool, blocked bool, notStarted bool) string {
	switch {
	case completed:
		return taskGroupPickerCompletedMarker
	case blocked:
		return taskGroupPickerBlockedMarker
	case notStarted:
		return taskGroupPickerNotStartedMarker
	default:
		return taskGroupPickerUnselectedMarker
	}
}

func taskGroupPickerOptionLabel(option taskGroupPickerOption) string {
	label := strings.Repeat("  ", option.Depth) + option.Label
	if !option.Completed {
		return label
	}
	return xansi.SGR(xansi.AttrStrikethrough) + label + xansi.SGR(xansi.AttrNoStrikethrough)
}

func taskGroupPickerSelectedLabel(label string) string {
	label = strings.Replace(label, taskGroupPickerUnselectedMarker, taskGroupPickerSelectedMarker, 1)
	label = strings.Replace(label, taskGroupPickerNotStartedMarker, taskGroupPickerSelectedMarker, 1)
	return strings.Replace(label, taskGroupPickerBlockedMarker, taskGroupPickerSelectedMarker, 1)
}

// normalizeTaskGroupSelections trims and drops empty entries while preserving
// the picker's presentation order.
func normalizeTaskGroupSelections(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	return normalized
}

// validateTaskGroupPickerMultiSelection rejects an empty or duplicated set and
// defers each entry to the single-select validator so blocked/completed rows
// stay unselectable in multi-select mode too.
func validateTaskGroupPickerMultiSelection(
	options []taskGroupPickerOption,
	selected []string,
	allowCompleted bool,
) error {
	normalized := normalizeTaskGroupSelections(selected)
	if len(normalized) == 0 {
		return errTaskGroupSelectionCanceled
	}
	seen := make(map[string]struct{}, len(normalized))
	for _, value := range normalized {
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s: selected more than once", value)
		}
		seen[value] = struct{}{}
		if err := validateTaskGroupPickerSelection(options, value, allowCompleted); err != nil {
			return err
		}
	}
	return nil
}

func validateTaskGroupPickerSelection(
	options []taskGroupPickerOption,
	selected string,
	allowCompleted bool,
) error {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return errTaskGroupSelectionCanceled
	}
	for index := range options {
		option := &options[index]
		if option.Value != selected {
			continue
		}
		if option.SelectionBlocked {
			reason := strings.TrimSpace(option.SelectionBlockedReason)
			if reason == "" {
				reason = "selection is blocked"
			}
			return fmt.Errorf("%s: %s", selected, reason)
		}
		if option.Completed && !allowCompleted {
			return fmt.Errorf("%s: completed Task Group is locked", selected)
		}
		return nil
	}
	return fmt.Errorf("unknown Task Group %q", selected)
}
