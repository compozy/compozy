package daemon

import (
	"fmt"
	"strings"
	"testing"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

type automationRunLinkage string

const (
	automationRunLinkageSession   automationRunLinkage = "session"
	automationRunLinkageDelegated automationRunLinkage = "delegated_task"
)

func classifyAutomationRunLinkage(run aghcontract.RunPayload) (automationRunLinkage, error) {
	hasSession := strings.TrimSpace(run.SessionID) != ""
	hasTaskID := strings.TrimSpace(run.TaskID) != ""
	hasTaskRunID := strings.TrimSpace(run.TaskRunID) != ""

	switch {
	case hasSession && (hasTaskID || hasTaskRunID):
		return "", fmt.Errorf("automation run %q mixes session and task linkage", strings.TrimSpace(run.ID))
	case hasSession:
		return automationRunLinkageSession, nil
	case hasTaskID || hasTaskRunID:
		if !hasTaskID || !hasTaskRunID {
			return "", fmt.Errorf(
				"automation run %q requires both task_id and task_run_id for delegated linkage",
				strings.TrimSpace(run.ID),
			)
		}
		return automationRunLinkageDelegated, nil
	default:
		return "", fmt.Errorf("automation run %q exposes no downstream linkage", strings.TrimSpace(run.ID))
	}
}

func requireCompletedSessionAutomationRun(run aghcontract.RunPayload) error {
	linkage, err := classifyAutomationRunLinkage(run)
	if err != nil {
		return err
	}
	if linkage != automationRunLinkageSession {
		return fmt.Errorf("automation run %q linkage = %q, want %q", run.ID, linkage, automationRunLinkageSession)
	}
	if got, want := run.Status, automationpkg.RunCompleted; got != want {
		return fmt.Errorf("automation run %q status = %q, want %q", run.ID, got, want)
	}
	return nil
}

func requireDelegatedTaskAutomationRun(run aghcontract.RunPayload) error {
	linkage, err := classifyAutomationRunLinkage(run)
	if err != nil {
		return err
	}
	if linkage != automationRunLinkageDelegated {
		return fmt.Errorf("automation run %q linkage = %q, want %q", run.ID, linkage, automationRunLinkageDelegated)
	}
	if got, want := run.Status, automationpkg.RunDelegated; got != want {
		return fmt.Errorf("automation run %q status = %q, want %q", run.ID, got, want)
	}
	return nil
}

func findTaskRunPayload(
	runs []aghcontract.TaskRunPayload,
	runID string,
) (aghcontract.TaskRunPayload, bool) {
	target := strings.TrimSpace(runID)
	for _, run := range runs {
		if strings.TrimSpace(run.ID) == target {
			return run, true
		}
	}
	return aghcontract.TaskRunPayload{}, false
}

func findTaskRunInDetail(
	detail *aghcontract.TaskDetailPayload,
	runID string,
) (aghcontract.TaskRunPayload, bool) {
	if detail == nil {
		return aghcontract.TaskRunPayload{}, false
	}
	return findTaskRunPayload(detail.Runs, runID)
}

func TestRequireCompletedSessionAutomationRun(t *testing.T) {
	t.Parallel()

	run := aghcontract.RunPayload{
		ID:        "run-session",
		SessionID: "sess-1",
		Status:    automationpkg.RunCompleted,
	}
	if err := requireCompletedSessionAutomationRun(run); err != nil {
		t.Fatalf("requireCompletedSessionAutomationRun() error = %v", err)
	}
}

func TestRequireDelegatedTaskAutomationRun(t *testing.T) {
	t.Parallel()

	run := aghcontract.RunPayload{
		ID:        "run-delegated",
		TaskID:    "task-1",
		TaskRunID: "task-run-1",
		Status:    automationpkg.RunDelegated,
	}
	if err := requireDelegatedTaskAutomationRun(run); err != nil {
		t.Fatalf("requireDelegatedTaskAutomationRun() error = %v", err)
	}
}

func TestFindTaskRunInDetailReturnsMissingForNilDetail(t *testing.T) {
	t.Parallel()

	t.Run("Should return missing for nil detail", func(t *testing.T) {
		t.Parallel()

		if _, ok := findTaskRunInDetail(nil, "task-run-1"); ok {
			t.Fatal("findTaskRunInDetail(nil) = present, want missing")
		}
	})
}

func TestClassifyAutomationRunLinkageRejectsMixedSurfaces(t *testing.T) {
	t.Parallel()

	run := aghcontract.RunPayload{
		ID:        "run-mixed",
		SessionID: "sess-1",
		TaskID:    "task-1",
		TaskRunID: "task-run-1",
		Status:    automationpkg.RunCompleted,
	}
	if _, err := classifyAutomationRunLinkage(run); err == nil {
		t.Fatal("classifyAutomationRunLinkage() error = nil, want mixed-surface failure")
	}
}

func TestFindTaskRunHelpers(t *testing.T) {
	t.Parallel()

	runs := []aghcontract.TaskRunPayload{{
		ID:        "task-run-1",
		TaskID:    "task-1",
		SessionID: "sess-1",
	}}
	run, ok := findTaskRunPayload(runs, "task-run-1")
	if !ok {
		t.Fatal("findTaskRunPayload() = missing, want present")
	}
	if got, want := run.SessionID, "sess-1"; got != want {
		t.Fatalf("run.SessionID = %q, want %q", got, want)
	}

	detailRun, ok := findTaskRunInDetail(&aghcontract.TaskDetailPayload{Runs: runs}, "task-run-1")
	if !ok {
		t.Fatal("findTaskRunInDetail() = missing, want present")
	}
	if got, want := detailRun.TaskID, "task-1"; got != want {
		t.Fatalf("detailRun.TaskID = %q, want %q", got, want)
	}
}
