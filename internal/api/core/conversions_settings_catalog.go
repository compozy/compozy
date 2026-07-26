package core

import (
	"maps"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"

	settingspkg "github.com/compozy/agh/internal/settings"
)

func settingsInstalledExtensionPayloads(
	values []settingspkg.InstalledExtension,
) []contract.SettingsInstalledExtensionPayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsInstalledExtensionPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsInstalledExtensionPayload{
			Name:          strings.TrimSpace(value.Name),
			Version:       strings.TrimSpace(value.Version),
			Enabled:       value.Enabled,
			State:         strings.TrimSpace(value.State),
			Health:        strings.TrimSpace(value.Health),
			HealthMessage: strings.TrimSpace(value.HealthMessage),
			LastError:     strings.TrimSpace(value.LastError),
			RequiresEnv:   append([]string(nil), value.RequiresEnv...),
			MissingEnv:    append([]string(nil), value.MissingEnv...),
		})
	}
	return payloads
}

func settingsProviderItemPayloads(values []settingspkg.ProviderItem) []contract.SettingsProviderItemPayload {
	if len(values) == 0 {
		return []contract.SettingsProviderItemPayload{}
	}
	payloads := make([]contract.SettingsProviderItemPayload, 0, len(values))
	for idx := range values {
		payloads = append(payloads, settingsProviderItemPayload(&values[idx]))
	}
	return payloads
}

func settingsProviderItemPayload(value *settingspkg.ProviderItem) contract.SettingsProviderItemPayload {
	if value == nil {
		return contract.SettingsProviderItemPayload{}
	}
	payload := contract.SettingsProviderItemPayload{
		Name:             strings.TrimSpace(value.Name),
		Settings:         settingsProviderSettingsPayload(value.Settings),
		Default:          value.Default,
		CommandAvailable: value.CommandAvailable,
		Credentials:      settingsProviderCredentialStatusPayloads(value.Credentials),
		AuthStatus:       settingsProviderAuthStatusPayload(value.AuthStatus),
		SourceMetadata:   settingsSourceMetadataPayload(value.SourceMetadata),
	}
	if value.Fallback != nil {
		payload.Fallback = &contract.SettingsProviderFallbackPayload{
			Source:   settingsSourceRefPayload(value.Fallback.Source),
			Settings: settingsProviderSettingsPayload(value.Fallback.Settings),
		}
	}
	return payload
}

func settingsProviderSettingsPayload(value settingspkg.ProviderSettings) contract.SettingsProviderSettingsPayload {
	return contract.SettingsProviderSettingsPayload{
		Command:         strings.TrimSpace(value.Command),
		DisplayName:     strings.TrimSpace(value.DisplayName),
		Models:          settingsProviderModelsPayload(value.Models),
		Harness:         string(value.Harness),
		RuntimeProvider: strings.TrimSpace(value.RuntimeProvider),
		Transport:       strings.TrimSpace(value.Transport),
		BaseURL:         strings.TrimSpace(value.BaseURL),
		AuthMode:        string(value.AuthMode),
		EnvPolicy:       string(value.EnvPolicy),
		HomePolicy:      string(value.HomePolicy),
		AuthStatusCmd:   strings.TrimSpace(value.AuthStatusCmd),
		AuthLoginCmd:    strings.TrimSpace(value.AuthLoginCmd),
		CredentialSlots: settingsProviderCredentialSlotPayloads(value.CredentialSlots),
	}
}

func settingsProviderCredentialSlotPayloads(
	values []aghconfig.ProviderCredentialSlot,
) []contract.SettingsProviderCredentialSlotPayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsProviderCredentialSlotPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsProviderCredentialSlotPayload{
			Name:      strings.TrimSpace(value.Name),
			TargetEnv: strings.TrimSpace(value.TargetEnv),
			SecretRef: strings.TrimSpace(value.SecretRef),
			Kind:      strings.TrimSpace(value.Kind),
			Required:  value.Required,
		})
	}
	return payloads
}

func settingsProviderAuthStatusPayload(
	value settingspkg.ProviderAuthStatus,
) *contract.SettingsProviderAuthStatusPayload {
	payload := contract.SettingsProviderAuthStatusPayload{
		Mode:       string(value.Mode),
		EnvPolicy:  string(value.EnvPolicy),
		HomePolicy: string(value.HomePolicy),
		State:      strings.TrimSpace(value.State),
		Code:       strings.TrimSpace(value.Code),
		Message:    strings.TrimSpace(value.Message),
		StatusCmd:  strings.TrimSpace(value.StatusCmd),
		LoginCmd:   strings.TrimSpace(value.LoginCmd),
		LoginEnv:   cloneStrings(value.LoginEnv),
	}
	if value.NativeCLI != nil {
		payload.NativeCLI = &contract.SettingsProviderNativeCLIStatusPayload{
			Command: strings.TrimSpace(value.NativeCLI.Command),
			Present: value.NativeCLI.Present,
			Path:    strings.TrimSpace(value.NativeCLI.Path),
			Source:  strings.TrimSpace(value.NativeCLI.Source),
			Error:   strings.TrimSpace(value.NativeCLI.Error),
		}
	}
	return &payload
}

func settingsProviderCredentialStatusPayloads(
	values []settingspkg.ProviderCredentialStatus,
) []contract.SettingsProviderCredentialStatusPayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsProviderCredentialStatusPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsProviderCredentialStatusPayload{
			Name:      strings.TrimSpace(value.Name),
			TargetEnv: strings.TrimSpace(value.TargetEnv),
			SecretRef: strings.TrimSpace(value.SecretRef),
			Kind:      strings.TrimSpace(value.Kind),
			Required:  value.Required,
			Present:   value.Present,
			Source:    strings.TrimSpace(value.Source),
		})
	}
	return payloads
}

func settingsSandboxItemPayloads(values []settingspkg.SandboxItem) []contract.SettingsSandboxItemPayload {
	if len(values) == 0 {
		return []contract.SettingsSandboxItemPayload{}
	}
	payloads := make([]contract.SettingsSandboxItemPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsSandboxItemPayload{
			Name:                strings.TrimSpace(value.Name),
			Profile:             settingsSandboxProfilePayload(value.Profile),
			WorkspaceUsageCount: value.WorkspaceUsageCount,
			SourceMetadata:      settingsSourceMetadataPayload(value.SourceMetadata),
		})
	}
	return payloads
}

func settingsSandboxProfilePayload(value aghconfig.SandboxProfile) contract.SettingsSandboxProfilePayload {
	payload := contract.SettingsSandboxProfilePayload{
		Backend:     strings.TrimSpace(value.Backend),
		SyncMode:    strings.TrimSpace(value.SyncMode),
		Persistence: strings.TrimSpace(value.Persistence),
		RuntimeRoot: strings.TrimSpace(value.RuntimeRoot),
		Env:         cloneStringMap(value.Env),
		SecretEnv:   cloneStringMap(value.SecretEnv),
	}
	if network := settingsSandboxNetworkPayload(value.Network); network != nil {
		payload.Network = network
	}
	if daytona := settingsSandboxDaytonaPayload(value.Daytona); daytona != nil {
		payload.Daytona = daytona
	}
	return payload
}

func settingsSandboxNetworkPayload(
	value aghconfig.NetworkProfile,
) *contract.SettingsSandboxNetworkPayload {
	if !value.AllowPublicIngress &&
		!value.AllowOutbound &&
		!value.Required &&
		len(value.AllowList) == 0 &&
		len(value.DenyList) == 0 {
		return nil
	}
	return &contract.SettingsSandboxNetworkPayload{
		AllowPublicIngress: value.AllowPublicIngress,
		AllowOutbound:      value.AllowOutbound,
		AllowList:          cloneStrings(value.AllowList),
		DenyList:           cloneStrings(value.DenyList),
		Required:           value.Required,
	}
}

func settingsSandboxDaytonaPayload(
	value aghconfig.DaytonaProfile,
) *contract.SettingsSandboxDaytonaPayload {
	if strings.TrimSpace(value.APIURL) == "" &&
		strings.TrimSpace(value.Target) == "" &&
		strings.TrimSpace(value.Image) == "" &&
		strings.TrimSpace(value.Snapshot) == "" &&
		strings.TrimSpace(value.Class) == "" &&
		strings.TrimSpace(value.AutoStop) == "" &&
		strings.TrimSpace(value.AutoArchive) == "" {
		return nil
	}
	return &contract.SettingsSandboxDaytonaPayload{
		APIURL:      strings.TrimSpace(value.APIURL),
		Target:      strings.TrimSpace(value.Target),
		Image:       strings.TrimSpace(value.Image),
		Snapshot:    strings.TrimSpace(value.Snapshot),
		Class:       strings.TrimSpace(value.Class),
		AutoStop:    strings.TrimSpace(value.AutoStop),
		AutoArchive: strings.TrimSpace(value.AutoArchive),
	}
}

func settingsHookItemPayloads(values []settingspkg.HookItem) []contract.SettingsHookItemPayload {
	if len(values) == 0 {
		return []contract.SettingsHookItemPayload{}
	}
	payloads := make([]contract.SettingsHookItemPayload, 0, len(values))
	for i := range values {
		value := &values[i]
		payloads = append(payloads, contract.SettingsHookItemPayload{
			Name:           strings.TrimSpace(value.Name),
			Declaration:    settingsHookDeclarationPayload(value.Declaration),
			SourceMetadata: settingsSourceMetadataPayload(value.SourceMetadata),
		})
	}
	return payloads
}

func settingsHookDeclarationPayload(value hookspkg.HookDecl) contract.SettingsHookDeclarationPayload {
	return contract.SettingsHookDeclarationPayload{
		Name:         strings.TrimSpace(value.Name),
		Event:        value.Event,
		Mode:         value.Mode,
		Required:     value.Required,
		Enabled:      value.Enabled,
		Priority:     int(value.Priority),
		Timeout:      durationString(value.Timeout),
		Matcher:      value.Matcher,
		ExecutorKind: value.ExecutorKind,
		Command:      strings.TrimSpace(value.Command),
		Args:         cloneStrings(value.Args),
		Env:          cloneStringMap(value.Env),
		SecretEnv:    cloneStringMap(value.SecretEnv),
		Metadata:     cloneStringMap(value.Metadata),
	}
}

func settingsSourceMetadataPayload(value settingspkg.SourceMetadata) contract.SettingsSourceMetadataPayload {
	return contract.SettingsSourceMetadataPayload{
		EffectiveSource:  settingsSourceRefPayload(value.EffectiveSource),
		ShadowedSources:  settingsSourceRefPayloads(value.ShadowedSources),
		AvailableTargets: settingsWriteTargetKindsPayload(value.AvailableTargets),
	}
}

func settingsSourceRefPayload(value settingspkg.SourceRef) contract.SettingsSourceRefPayload {
	return contract.SettingsSourceRefPayload{
		Kind:        contract.SettingsSourceKind(value.Kind),
		Scope:       contract.SettingsScopeKind(value.Scope),
		WorkspaceID: strings.TrimSpace(value.WorkspaceID),
		AgentName:   strings.TrimSpace(value.AgentName),
	}
}

func settingsSourceRefPayloads(values []settingspkg.SourceRef) []contract.SettingsSourceRefPayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsSourceRefPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, settingsSourceRefPayload(value))
	}
	return payloads
}

func settingsWriteTargetKindsPayload(values []settingspkg.WriteTargetKind) []contract.SettingsWriteTargetKind {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsWriteTargetKind, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsWriteTargetKind(value))
	}
	return payloads
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	maps.Copy(dst, src)
	return dst
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	return append([]string(nil), src...)
}

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneTimePointer(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	cloned := src.UTC()
	return &cloned
}

func durationString(value time.Duration) string {
	if value <= 0 {
		return ""
	}
	return value.String()
}
