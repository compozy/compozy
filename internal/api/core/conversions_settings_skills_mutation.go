package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

// SettingsSkillsMutationResponseFromResult combines apply metadata with the refreshed skills section.
func SettingsSkillsMutationResponseFromResult(
	result settingspkg.ApplyResult,
	response contract.SettingsSkillsResponse,
) contract.SettingsSkillsMutationResponse {
	apply := SettingsApplyResponseFromResult(result)
	return contract.SettingsSkillsMutationResponse{
		SettingsSkillsResponse: response,
		WriteTarget:            apply.WriteTarget,
		Applied:                apply.Applied,
		Lifecycle:              apply.Lifecycle,
		ApplyRecordID:          apply.ApplyRecordID,
		ActiveGeneration:       apply.ActiveGeneration,
		ActiveConfigHash:       apply.ActiveConfigHash,
		NextAction:             apply.NextAction,
		RestartRequired:        apply.RestartRequired,
		RestartScope:           apply.RestartScope,
		Warnings:               apply.Warnings,
		PartialFailures:        apply.PartialFailures,
		Skipped:                apply.Skipped,
		SkippedReason:          apply.SkippedReason,
	}
}
