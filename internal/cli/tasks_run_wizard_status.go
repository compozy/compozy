package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	apiclient "github.com/compozy/compozy/internal/api/client"
	"github.com/compozy/compozy/internal/core/taskgroups"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
)

type taskRunWizardWorkflowStatus int

const (
	taskRunWizardWorkflowReady taskRunWizardWorkflowStatus = iota
	taskRunWizardWorkflowReadyToRetry
	taskRunWizardWorkflowBlocked
	taskRunWizardWorkflowRunning
	taskRunWizardWorkflowCompleted
)

const taskRunWizardRunStatusRunning = "running"

func (s taskRunWizardWorkflowStatus) label() string {
	switch s {
	case taskRunWizardWorkflowCompleted:
		return "Completed"
	case taskRunWizardWorkflowRunning:
		return "Running"
	case taskRunWizardWorkflowBlocked:
		return "Blocked"
	case taskRunWizardWorkflowReadyToRetry:
		return "Ready to retry"
	default:
		return "Ready"
	}
}

func loadTaskRunWizardLatestRunStatuses(
	ctx context.Context,
	client daemonCommandClient,
	workspaceRoot string,
) (map[string]string, error) {
	return loadTaskGroupPickerLatestRunStatuses(ctx, client, workspaceRoot, daemonRunModeTask)
}

func loadTaskGroupPickerLatestRunStatuses(
	ctx context.Context,
	client daemonCommandClient,
	workspaceRoot string,
	runMode string,
) (map[string]string, error) {
	if client == nil {
		return nil, errors.New("load run statuses: daemon client is required")
	}
	mode := strings.TrimSpace(runMode)
	if mode == "" {
		return nil, errors.New("load run statuses: run mode is required")
	}
	runs, err := client.ListRuns(ctx, apiclient.RunListOptions{
		Workspace: strings.TrimSpace(workspaceRoot),
		Mode:      mode,
		Limit:     taskRunGuardRunListLimit,
	})
	if err != nil {
		if isWorkspaceContextStaleError(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("list %s runs for picker: %w", mode, err)
	}

	latest := make(map[string]string)
	for i := range runs {
		target := strings.TrimSpace(runs[i].WorkflowSlug)
		status := strings.ToLower(strings.TrimSpace(runs[i].Status))
		if target == "" || status == "" {
			continue
		}
		if _, exists := latest[target]; exists {
			continue
		}
		latest[target] = status
	}
	return latest, nil
}

func isWorkspaceContextStaleError(err error) bool {
	var remoteErr *apiclient.RemoteError
	return errors.As(err, &remoteErr) &&
		remoteErr.StatusCode == http.StatusPreconditionFailed &&
		strings.TrimSpace(remoteErr.Envelope.Code) == "workspace_context_stale"
}

func buildTaskRunWizardWorkflowOptions(
	baseDir string,
	latestRunStatuses map[string]string,
) []taskRunWizardWorkflowOption {
	slugs := listTaskSubdirs(baseDir)
	options := make([]taskRunWizardWorkflowOption, 0, len(slugs))
	for _, slug := range slugs {
		plan, ok := readTaskRunWizardPlan(baseDir, slug)
		if !ok || len(plan.TaskGroups) == 0 {
			options = append(options, taskRunWizardOrdinaryOption(baseDir, slug, latestRunStatuses[slug]))
			continue
		}

		children := make([]taskRunWizardWorkflowOption, 0, len(plan.TaskGroups))
		for index := range plan.TaskGroups {
			children = append(
				children,
				taskRunWizardTaskGroupOption(baseDir, slug, plan, plan.TaskGroups[index], latestRunStatuses),
			)
		}
		options = append(options, taskRunWizardGroupOption(slug, children))
		options = append(options, children...)
	}
	return options
}

func taskRunWizardOrdinaryOption(baseDir, slug, latestRunStatus string) taskRunWizardWorkflowOption {
	completedTasks, totalTasks, progressKnown := taskRunWizardTaskProgress(filepath.Join(baseDir, slug))
	completed := taskRunWizardTaskProgressCompleted(completedTasks, totalTasks, progressKnown)
	return taskRunWizardWorkflowOption{
		Value:             slug,
		Label:             slug,
		Initiative:        slug,
		Status:            taskRunWizardStatus(completed, false, latestRunStatus),
		Completed:         completed,
		CompletedTasks:    completedTasks,
		TotalTasks:        totalTasks,
		TaskProgressKnown: progressKnown,
	}
}

func taskRunWizardTaskGroupOption(
	baseDir string,
	initiative string,
	plan taskgroups.Plan,
	taskGroup taskgroups.TaskGroup,
	latestRunStatuses map[string]string,
) taskRunWizardWorkflowOption {
	reference := initiative + "/" + taskGroup.ID
	completedTasks, totalTasks, progressKnown := taskRunWizardTaskProgress(
		filepath.Join(baseDir, initiative, filepath.FromSlash(taskGroup.Directory)),
	)
	readiness, err := taskgroups.EvaluateReadiness(plan, taskGroup.ID)
	blocked := err == nil && !readiness.Eligible
	blockedBy := taskRunWizardBlockedBy(readiness)
	completed := taskGroup.Completed || taskRunWizardTaskProgressCompleted(completedTasks, totalTasks, progressKnown)
	return taskRunWizardWorkflowOption{
		Value:             reference,
		Label:             taskGroup.ID + " — " + taskGroup.Title,
		Initiative:        initiative,
		Depth:             1,
		Status:            taskRunWizardStatus(completed, blocked, latestRunStatuses[reference]),
		Completed:         completed,
		CompletedTasks:    completedTasks,
		TotalTasks:        totalTasks,
		TaskProgressKnown: progressKnown,
		BlockedBy:         blockedBy,
	}
}

func taskRunWizardTaskProgressCompleted(completed int, total int, known bool) bool {
	return known && total > 0 && completed == total
}

func taskRunWizardGroupOption(
	initiative string,
	children []taskRunWizardWorkflowOption,
) taskRunWizardWorkflowOption {
	group := taskRunWizardWorkflowOption{
		Value:             initiative,
		Label:             initiative,
		Initiative:        initiative,
		Group:             true,
		TaskProgressKnown: true,
		TotalTaskGroups:   len(children),
	}
	for index := range children {
		child := &children[index]
		group.CompletedTasks += child.CompletedTasks
		group.TotalTasks += child.TotalTasks
		group.TaskProgressKnown = group.TaskProgressKnown && child.TaskProgressKnown
		if child.Completed {
			group.CompletedTaskGroups++
		}
	}
	group.Completed = len(children) > 0 && group.CompletedTaskGroups == len(children)
	group.Status = taskRunWizardGroupStatus(children, group.Completed)
	return group
}

func taskRunWizardGroupStatus(
	children []taskRunWizardWorkflowOption,
	completed bool,
) taskRunWizardWorkflowStatus {
	if completed {
		return taskRunWizardWorkflowCompleted
	}
	for _, status := range []taskRunWizardWorkflowStatus{
		taskRunWizardWorkflowRunning,
		taskRunWizardWorkflowBlocked,
		taskRunWizardWorkflowReadyToRetry,
		taskRunWizardWorkflowReady,
	} {
		if slices.ContainsFunc(children, func(child taskRunWizardWorkflowOption) bool {
			return child.Status == status
		}) {
			return status
		}
	}
	return taskRunWizardWorkflowReady
}

func taskRunWizardStatus(
	completed bool,
	blocked bool,
	latestRunStatus string,
) taskRunWizardWorkflowStatus {
	if completed {
		return taskRunWizardWorkflowCompleted
	}
	if taskRunWizardRunIsActive(latestRunStatus) {
		return taskRunWizardWorkflowRunning
	}
	if blocked {
		return taskRunWizardWorkflowBlocked
	}
	if taskRunWizardRunCanRetry(latestRunStatus) {
		return taskRunWizardWorkflowReadyToRetry
	}
	return taskRunWizardWorkflowReady
}

func taskRunWizardRunIsActive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "starting", taskRunWizardRunStatusRunning, "pending", "retrying", "pausing", "paused":
		return true
	default:
		return false
	}
}

func taskRunWizardRunCanRetry(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "canceled", "crashed", "parked":
		return true
	default:
		return false
	}
}

func taskRunWizardTaskProgress(tasksDir string) (completed int, total int, known bool) {
	meta, err := taskscore.SnapshotTaskMeta(tasksDir)
	if err != nil {
		return 0, 0, false
	}
	return meta.Completed, meta.Total, true
}

func taskRunWizardBlockedBy(readiness taskgroups.Readiness) []string {
	blockedBy := make([]string, 0, len(readiness.DirectUnmet)+len(readiness.TransitiveUnmet))
	for _, dependency := range readiness.DirectUnmet {
		if !slices.Contains(blockedBy, dependency.From) {
			blockedBy = append(blockedBy, dependency.From)
		}
	}
	for _, path := range readiness.TransitiveUnmet {
		for _, taskGroupID := range path.TaskGroupIDs {
			if !slices.Contains(blockedBy, taskGroupID) {
				blockedBy = append(blockedBy, taskGroupID)
			}
		}
	}
	slices.Sort(blockedBy)
	return blockedBy
}

func taskRunWizardWorkflowOptionLabel(option taskRunWizardWorkflowOption) string {
	parts := []string{option.Label, option.Status.label()}
	if option.Group {
		parts = append(parts, fmt.Sprintf(
			"%d/%d Task Groups completed",
			option.CompletedTaskGroups,
			option.TotalTaskGroups,
		))
	}
	if option.TaskProgressKnown {
		parts = append(parts, fmt.Sprintf("%d/%d tasks completed", option.CompletedTasks, option.TotalTasks))
	} else {
		parts = append(parts, "task progress unavailable")
	}
	if option.Status == taskRunWizardWorkflowBlocked && len(option.BlockedBy) > 0 {
		parts = append(parts, "waits for "+strings.Join(option.BlockedBy, ", "))
	}
	return strings.Join(parts, " — ")
}

func taskRunWizardWorkflowNotStarted(option taskRunWizardWorkflowOption) bool {
	return option.TaskProgressKnown && option.TotalTasks > 0 && option.CompletedTasks == 0
}
