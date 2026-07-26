package extensionpkg

import (
	"fmt"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
)

// Validate ensures one bundle job is internally consistent.
func (j BundleJob) Validate(bundleName string, profileName string, channelNames map[string]struct{}) error {
	job := automationpkg.Job{
		ID:          "bundle-validation",
		Scope:       automationpkg.AutomationScopeWorkspace,
		Name:        strings.TrimSpace(j.Name),
		AgentName:   strings.TrimSpace(j.AgentName),
		WorkspaceID: "bundle-validation-workspace",
		Prompt:      strings.TrimSpace(j.Prompt),
		Schedule:    &j.Schedule,
		Task:        cloneBundleTaskConfig(j.Task),
		Enabled:     j.Enabled,
		Retry:       j.Retry,
		FireLimit:   j.FireLimit,
		Source:      automationpkg.JobSourcePackage,
	}
	if err := job.Validate("bundle.jobs"); err != nil {
		return fmt.Errorf("%w: bundle %q profile %q job %q: %w", ErrBundleInvalid, bundleName, profileName, j.Name, err)
	}
	if j.Task == nil {
		return nil
	}

	request, err := automationpkg.NormalizeDirectTaskParticipation(j.Task.NetworkParticipation)
	if err != nil {
		return fmt.Errorf(
			"%w: bundle %q profile %q job %q: %w",
			ErrBundleInvalid,
			bundleName,
			profileName,
			j.Name,
			err,
		)
	}
	if request == nil || request.ChannelStrategy == nil ||
		*request.ChannelStrategy != participation.StrategyNamed || request.ChannelID == nil {
		return nil
	}

	channel := strings.TrimSpace(*request.ChannelID)
	if _, ok := channelNames[channel]; !ok {
		return fmt.Errorf(
			"%w: bundle %q profile %q job %q references undeclared channel %q",
			ErrBundleInvalid,
			bundleName,
			profileName,
			j.Name,
			channel,
		)
	}
	return nil
}
