package daemon

import (
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func nativeAutomationJobFromCreateRequest(
	toolID toolspkg.ToolID,
	request contract.CreateJobRequest,
) (automationpkg.Job, error) {
	job, err := core.AutomationJobFromCreateRequest(request)
	if err != nil {
		return automationpkg.Job{}, nativeAutomationValidationError(toolID, err)
	}
	if err := job.Validate(nativeAutomationToolsJobKey); err != nil {
		return automationpkg.Job{}, nativeAutomationValidationError(toolID, err)
	}
	return job, nil
}

func nativeAutomationJobFromPatch(
	toolID toolspkg.ToolID,
	current automationpkg.Job,
	patch contract.UpdateJobRequest,
) (automationpkg.Job, error) {
	next, err := core.ApplyAutomationJobPatch(current, patch)
	if err != nil {
		return automationpkg.Job{}, nativeAutomationValidationError(toolID, err)
	}
	if err := next.Validate(nativeAutomationToolsJobKey); err != nil {
		return automationpkg.Job{}, nativeAutomationValidationError(toolID, err)
	}
	return next, nil
}
