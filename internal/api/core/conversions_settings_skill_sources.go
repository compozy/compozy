package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsSkillSourcePayloads(items []settingspkg.SkillSourceItem) []contract.SettingsSkillSourcePayload {
	payloads := make([]contract.SettingsSkillSourcePayload, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, contract.SettingsSkillSourcePayload{
			Slug: item.Slug, Label: item.Label, Kind: item.Kind, Enabled: item.Enabled,
			AlwaysOn: item.AlwaysOn, Default: item.DefaultOn, WorkspacePath: item.WorkspacePath,
			GlobalPath: item.GlobalPath, Path: item.Path, Roots: settingsSkillSourceRootPayloads(item.Roots),
		})
	}
	return payloads
}

func settingsSkillSourceRootPayloads(
	items []settingspkg.SkillSourceRootItem,
) []contract.SettingsSkillSourceRootPayload {
	payloads := make([]contract.SettingsSkillSourceRootPayload, 0, len(items))
	for _, item := range items {
		payload := contract.SettingsSkillSourceRootPayload{
			RootID: item.RootID, Path: item.Path, Exists: item.Exists, Readable: item.Readable,
			ScannedCount: item.ScannedCount, SkillCount: item.SkillCount, Truncated: item.Truncated,
			SkippedLinks: make([]contract.SettingsSkillSourceSkippedLinkPayload, 0, len(item.SkippedLinks)),
			Collisions:   make([]contract.SettingsSkillSourceCollisionPayload, 0, len(item.Collisions)),
			Verification: contract.SettingsSkillSourceVerificationPayload{
				Blocked: item.Verification.Blocked, Warned: item.Verification.Warned,
			},
			NativeReaders: append([]string{}, item.NativeReaders...),
		}
		for _, skipped := range item.SkippedLinks {
			payload.SkippedLinks = append(payload.SkippedLinks, contract.SettingsSkillSourceSkippedLinkPayload{
				Path: skipped.Path, Reason: skipped.Reason,
			})
		}
		for _, collision := range item.Collisions {
			payload.Collisions = append(payload.Collisions, contract.SettingsSkillSourceCollisionPayload{
				Name: collision.Name, WinnerRootID: collision.WinnerRootID,
				QualifiedForm: collision.QualifiedForm,
			})
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func settingsSkillSourceInheritancePayload(
	value *settingspkg.SkillSourceInheritance,
) *contract.SettingsSkillSourceInheritancePayload {
	if value == nil {
		return nil
	}
	return &contract.SettingsSkillSourceInheritancePayload{
		Sources: value.Sources, CustomSources: value.CustomSources,
	}
}
