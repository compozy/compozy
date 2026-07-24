package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/taskgroups"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// preparedTaskMultiGroupLaunch is the validated, immutable group selection.
// Result branches are filled once the parent run ID exists, before fan-out.
type preparedTaskMultiGroupLaunch struct {
	initiative     string
	groups         []RenderedGroupContext
	slugsByGroupID map[string]string
	resultBranches map[string]string
}

func taskMultiGroupSelectionFingerprint(prepared *preparedTaskMulti) (string, error) {
	if prepared == nil || prepared.taskGroupLaunch == nil {
		return "", errors.New("parallel task-group selection is not prepared")
	}
	groupIDs := make([]string, 0, len(prepared.items))
	initiative := ""
	planChecksum := ""
	for index := range prepared.items {
		evidence := prepared.items[index].taskGroupPreflight
		if evidence == nil {
			return "", fmt.Errorf(
				"parallel task group %q is missing preflight evidence",
				prepared.items[index].slug,
			)
		}
		if initiative == "" {
			initiative = strings.TrimSpace(evidence.initiativeSlug)
			planChecksum = strings.TrimSpace(evidence.planChecksum)
		}
		if strings.TrimSpace(evidence.initiativeSlug) != initiative {
			return "", errors.New("parallel task-group selection spans multiple initiatives")
		}
		if strings.TrimSpace(evidence.planChecksum) != planChecksum {
			return "", errors.New("parallel task-group selection has inconsistent plan checksums")
		}
		groupID := strings.TrimSpace(evidence.taskGroupID)
		if groupID == "" {
			return "", fmt.Errorf(
				"parallel task group %q is missing its stable id",
				prepared.items[index].slug,
			)
		}
		groupIDs = append(groupIDs, groupID)
	}
	if initiative == "" || planChecksum == "" || len(groupIDs) == 0 {
		return "", errors.New("parallel task-group selection fingerprint inputs are incomplete")
	}
	return taskgroups.SelectionFingerprint(initiative, groupIDs, planChecksum), nil
}

// isRelaunchSettledRunStatus reports whether an existing run for a selection is
// genuinely settled, so the relaunch gate routes it through the terminal-report
// path (requiring --new) instead of re-attaching. It deliberately mirrors
// globaldb's active-run predicate (status NOT IN completed/failed/canceled/crashed
// in registry.go/runs.go): a `parked` run is a recoverable stall, not a terminal
// outcome, so it must re-attach like an active run rather than being pushed toward
// --new. Keeping this list identical to the globaldb predicate stops the two
// classifiers from drifting. This is intentionally narrower than
// isTerminalRunStatus, which treats `parked` as terminal for settlement/stall
// bookkeeping.
func isRelaunchSettledRunStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case runStatusCompleted, runStatusFailed, runStatusCancelled, runStatusCrashed:
		return true
	default:
		return false
	}
}

func (m *RunManager) taskMultiGroupRelaunchGate(
	ctx context.Context,
	prepared *preparedTaskMulti,
	fingerprint string,
) (apicore.Run, bool, error) {
	row, err := m.globalDB.FindRunBySelectionFingerprint(
		ctx,
		prepared.workspace.ID,
		fingerprint,
	)
	if errors.Is(err, globaldb.ErrRunNotFound) {
		return apicore.Run{}, false, nil
	}
	if err != nil {
		return apicore.Run{}, false, fmt.Errorf("find equivalent parallel task-group run: %w", err)
	}
	if !isRelaunchSettledRunStatus(row.Status) {
		run, err := m.toCoreRun(ctx, row, "")
		if err != nil {
			return apicore.Run{}, false, err
		}
		run.PresentationMode = prepared.presentationMode
		return run, true, nil
	}

	snapshot, err := m.RunMultipleSnapshot(ctx, row.RunID)
	if err != nil {
		return apicore.Run{}, false, apicore.NewProblem(
			http.StatusConflict,
			"parallel_task_groups_relaunch_unreadable",
			fmt.Sprintf(
				"parallel task-group selection has terminal run %s, but its result "+
					"record is unreadable; resolve it before retrying",
				row.RunID,
			),
			map[string]any{
				"run_id":                row.RunID,
				"status":                row.Status,
				"selection_fingerprint": fingerprint,
				"new_required":          true,
			},
			err,
		)
	}
	return apicore.Run{}, false, taskMultiGroupTerminalRelaunchProblem(row, fingerprint, snapshot)
}

type taskMultiGroupTerminalReport struct {
	branches       []string
	succeeded      []string
	failed         []string
	preservedPaths []string
}

func taskMultiGroupTerminalRelaunchProblem(
	row globaldb.Run,
	fingerprint string,
	snapshot apicore.TaskRunMultipleSnapshot,
) error {
	report := buildTaskMultiGroupTerminalReport(snapshot.Items)
	details := map[string]any{
		"run_id":                row.RunID,
		"status":                row.Status,
		"selection_fingerprint": fingerprint,
		"new_required":          true,
		"items":                 snapshot.Items,
		"result_branches":       report.branches,
		"succeeded":             report.succeeded,
		"failed":                report.failed,
		"preserved_paths":       report.preservedPaths,
	}
	if row.Status == runStatusCompleted && len(report.failed) == 0 {
		return apicore.NewProblem(
			http.StatusConflict,
			"parallel_task_groups_selection_completed",
			fmt.Sprintf(
				"parallel task-group selection already completed in run %s; existing branches: %s; use --new to start a fresh run",
				row.RunID,
				taskMultiGroupReportList(report.branches),
			),
			details,
			nil,
		)
	}
	return apicore.NewProblem(
		http.StatusConflict,
		"parallel_task_groups_selection_terminal",
		fmt.Sprintf(
			"parallel task-group selection already has terminal run %s (%s); "+
				"succeeded: %s; failed: %s; preserved paths: %s; "+
				"use --new for an explicit fresh namespace",
			row.RunID,
			row.Status,
			taskMultiGroupReportList(report.succeeded),
			taskMultiGroupReportList(report.failed),
			taskMultiGroupReportList(report.preservedPaths),
		),
		details,
		nil,
	)
}

func buildTaskMultiGroupTerminalReport(items []apicore.TaskRunMultipleItem) taskMultiGroupTerminalReport {
	report := taskMultiGroupTerminalReport{
		branches:       make([]string, 0),
		succeeded:      make([]string, 0),
		failed:         make([]string, 0),
		preservedPaths: make([]string, 0),
	}
	for index := range items {
		item := &items[index]
		group := taskMultiTaskGroupID(item.Slug)
		if group == "" {
			group = strings.TrimSpace(item.Slug)
		}
		if branch := strings.TrimSpace(item.ResultBranch); branch != "" {
			report.branches = append(report.branches, group+"="+branch)
		}
		switch item.Status {
		case taskMultiItemStatusCompleted, taskMultiItemStatusNoChanges:
			report.succeeded = append(report.succeeded, group+"="+item.Status)
		default:
			failed := group + "=" + item.Status
			if reason := strings.TrimSpace(item.ErrorText); reason != "" {
				failed += ": " + reason
			}
			report.failed = append(report.failed, failed)
		}
		if path := strings.TrimSpace(item.WorktreePath); path != "" &&
			item.WorktreeStatus == taskMultiWorktreeStatusPreserved {
			report.preservedPaths = append(report.preservedPaths, group+"="+path)
		}
	}
	return report
}

func taskMultiGroupReportList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func resolveTaskMultiParallelExecutionKind(
	descriptor *apicore.TaskExecutionDescriptor,
) string {
	if descriptor != nil &&
		strings.TrimSpace(descriptor.Kind) == apicore.ExecutionKindTaskMultiGroupParallel {
		return apicore.ExecutionKindTaskMultiGroupParallel
	}
	return apicore.ExecutionKindTaskMultiParallel
}

func (m *RunManager) prepareTaskMultiGroupLaunch(
	ctx context.Context,
	workspaceRef string,
	slugs []string,
) (*preparedTaskMultiGroupLaunch, error) {
	workspaceRow, err := resolveWorkspaceReference(ctx, m.globalDB, workspaceRef)
	if err != nil {
		return nil, err
	}
	if err := requireWorkspacePathAvailable(workspaceRow); err != nil {
		return nil, err
	}

	initiative := ""
	groupIDs := make([]string, 0, len(slugs))
	slugsByGroupID := make(map[string]string, len(slugs))
	for _, slug := range slugs {
		ref, err := taskgroups.ParseTaskGroupRef(strings.TrimSpace(slug))
		if err != nil {
			return nil, apicore.NewProblem(
				http.StatusUnprocessableEntity,
				"parallel_task_groups_targets_required",
				"parallel task-group execution requires only initiative/TG-NNN targets",
				map[string]any{"target": strings.TrimSpace(slug)},
				err,
			)
		}
		if initiative == "" {
			initiative = ref.Initiative
		} else if ref.Initiative != initiative {
			return nil, apicore.NewProblem(
				http.StatusUnprocessableEntity,
				"parallel_task_groups_mixed_initiatives",
				"parallel task groups must belong to one initiative",
				map[string]any{"expected_initiative": initiative, "target": ref.String()},
				nil,
			)
		}
		groupIDs = append(groupIDs, ref.TaskGroupID)
		slugsByGroupID[ref.TaskGroupID] = ref.String()
	}

	m.hydrateTaskGroupPlanBestEffort(ctx, workspaceRow.RootDir, initiative)
	target, err := (taskgroups.TargetResolver{}).Resolve(ctx, workspaceRow.RootDir, initiative)
	if err != nil {
		return nil, err
	}
	validation, err := taskgroups.ValidateIndependentSet(target.Plan, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(validation.Rejected) > 0 || len(validation.Eligible) != len(groupIDs) {
		return nil, parallelTaskGroupSelectionProblem(initiative, validation)
	}

	groups := make([]RenderedGroupContext, 0, len(groupIDs))
	for index, groupID := range groupIDs {
		group, found := target.Plan.TaskGroup(groupID)
		if !found {
			return nil, fmt.Errorf("validated parallel task group %s is missing from plan", groupID)
		}
		groups = append(groups, RenderedGroupContext{
			ID:        group.ID,
			Directory: group.Directory,
			Index:     index + 1,
		})
	}
	return &preparedTaskMultiGroupLaunch{
		initiative:     initiative,
		groups:         groups,
		slugsByGroupID: slugsByGroupID,
	}, nil
}

func parallelTaskGroupSelectionProblem(
	initiative string,
	validation taskgroups.SetValidationResult,
) error {
	rejected := make(map[string]any, len(validation.Rejected))
	for groupID, rejection := range validation.Rejected {
		rejected[groupID] = map[string]any{
			"reason":   rejection.Reason,
			"blockers": append([]string(nil), rejection.Blockers...),
		}
	}
	return apicore.NewProblem(
		http.StatusConflict,
		"task_group_dependencies_unmet",
		"selected task groups are not a mutually independent runnable set; use the sequential path for dependent groups",
		map[string]any{
			"initiative_slug": initiative,
			"plan_checksum":   validation.PlanChecksum,
			"rejected":        rejected,
		},
		taskgroups.ErrDependenciesUnmet,
	)
}

// runTaskMultiGroupParallel is the group-only routing layer. It plans result
// branches, then delegates scheduling and child lifecycle to the existing
// fail-late queue. No runner-owned commit or merge lifecycle is constructed.
func (m *RunManager) runTaskMultiGroupParallel(
	active *activeRun,
	prepared *preparedTaskMulti,
	total int,
) error {
	if prepared == nil || prepared.taskGroupLaunch == nil {
		return errors.New("parallel task-group launch is not configured")
	}
	if prepared.mode != workspacecfg.TaskRunMultipleModeParallel {
		return fmt.Errorf("parallel task-group launch requires parallel mode, got %q", prepared.mode)
	}
	projectCfg, err := m.loadProjectConfig(active.ctx, prepared.workspace.RootDir)
	if err != nil {
		return fmt.Errorf("load parallel task-group branch config: %w", err)
	}
	parallelCfg := projectCfg.Tasks.Run.ParallelTaskGroups.ApplyDefaults()
	if parallelCfg.BranchTemplate == nil {
		return errors.New("parallel task-group branch template is required")
	}
	branchesByGroupID, adjusted, err := RenderResultBranches(
		active.ctx,
		prepared.workspace.RootDir,
		BranchRenderInput{
			Template:   *parallelCfg.BranchTemplate,
			Initiative: prepared.taskGroupLaunch.initiative,
			RunSegment: active.runID,
			Groups:     prepared.taskGroupLaunch.groups,
		},
	)
	if err != nil {
		return err
	}
	if adjusted {
		slog.Default().Warn(
			"daemon: adjusted parallel task-group result branches for uniqueness",
			"run_id",
			active.runID,
			"initiative",
			prepared.taskGroupLaunch.initiative,
		)
	}
	resultBranches := make(map[string]string, len(branchesByGroupID))
	for groupID, branch := range branchesByGroupID {
		slug := prepared.taskGroupLaunch.slugsByGroupID[groupID]
		if strings.TrimSpace(slug) == "" {
			return fmt.Errorf("parallel task group %s has no prepared workflow reference", groupID)
		}
		resultBranches[slug] = branch
	}
	prepared.taskGroupLaunch.resultBranches = resultBranches
	return m.runTaskMultiParallelQueue(active, prepared, total)
}

func (m *RunManager) revalidateTaskMultiGroupChildStart(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
) (*taskGroupPreflightEvidence, bool, error) {
	if prepared == nil ||
		prepared.executionKind != apicore.ExecutionKindTaskMultiGroupParallel {
		return nil, false, nil
	}
	current, err := m.resolveTaskGroupPreflightEvidence(
		detachContext(active.ctx),
		prepared.workspace.RootDir,
		item.slug,
	)
	if err != nil {
		return nil, false, err
	}
	if current == nil || item.taskGroupPreflight == nil {
		return nil, false, errors.New("parallel task-group child is missing preflight evidence")
	}
	outOfOrderNeeded, err := taskGroupPreflightDecision(
		current,
		prepared.allowOutOfOrder,
		item.taskGroupPreflight,
	)
	if err != nil {
		return nil, false, err
	}
	current.outOfOrderNeeded = outOfOrderNeeded
	return current, outOfOrderNeeded, nil
}

func taskMultiTaskGroupID(slug string) string {
	ref, err := taskgroups.ParseTaskGroupRef(strings.TrimSpace(slug))
	if err != nil {
		return ""
	}
	return ref.TaskGroupID
}

func taskMultiOutcomeIncompleteReasons(items []apicore.TaskRunMultipleItem) []string {
	reasons := make([]string, 0)
	for index := range items {
		item := &items[index]
		if item.Status != taskMultiItemStatusFailed && item.Status != taskMultiItemStatusCanceled {
			continue
		}
		reason := fmt.Sprintf("%s: %s", item.Slug, item.Status)
		if strings.TrimSpace(item.ErrorText) != "" {
			reason += ": " + strings.TrimSpace(item.ErrorText)
		}
		if strings.TrimSpace(item.WorktreePath) != "" &&
			item.WorktreeStatus == taskMultiWorktreeStatusPreserved {
			reason += "; preserved at " + strings.TrimSpace(item.WorktreePath)
		}
		reasons = append(reasons, reason)
	}
	return reasons
}
