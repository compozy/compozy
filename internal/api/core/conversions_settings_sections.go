package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	settingspkg "github.com/compozy/agh/internal/settings"
)

// SettingsSectionResponseFromEnvelope converts one settings section envelope into the shared response payload.
func SettingsSectionResponseFromEnvelope(envelope settingspkg.SectionEnvelope) (any, error) {
	switch envelope.Section {
	case settingspkg.SectionGeneral:
		return settingsGeneralSectionResponse(envelope)
	case settingspkg.SectionMemory:
		return settingsMemorySectionResponse(envelope)
	case settingspkg.SectionRoles:
		return settingsRolesSectionResponse(envelope)
	case settingspkg.SectionSkills:
		return settingsSkillsSectionResponse(envelope)
	case settingspkg.SectionAutomation:
		return settingsAutomationSectionResponse(envelope)
	case settingspkg.SectionNetwork:
		return settingsNetworkSectionResponse(envelope)
	case settingspkg.SectionWindowManager:
		return settingsWindowManagerSectionResponse(envelope)
	case settingspkg.SectionObservability:
		return settingsObservabilitySectionResponse(envelope)
	case settingspkg.SectionHooksExtensions:
		return settingsHooksExtensionsSectionResponse(envelope)
	default:
		return nil, fmt.Errorf("unknown settings section %q", envelope.Section)
	}
}

func settingsRolesSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Roles == nil {
		return nil, errors.New("settings roles section is required")
	}
	return contract.SettingsRolesResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                   settingsRolesConfigPayload(&envelope.Roles.Config),
	}, nil
}

func settingsGeneralSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.General == nil {
		return nil, errors.New("settings general section is required")
	}
	return contract.SettingsGeneralResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		ConfigPaths:                              settingsConfigPathsPayload(envelope.General.ConfigPaths),
		Config:                                   settingsGeneralConfigPayload(envelope.General.Settings),
		Runtime:                                  settingsDaemonRuntimePayload(envelope.General.Runtime),
		Actions: contract.SettingsGeneralActionsPayload{
			Restart: settingsActionMetadataPayload(envelope.General.Actions.Restart),
		},
	}, nil
}

func settingsMemorySectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Memory == nil {
		return nil, errors.New("settings memory section is required")
	}
	return contract.SettingsMemoryResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                   settingsMemoryConfigPayload(&envelope.Memory.Config),
		Health:                                   settingsMemoryHealthPayload(envelope.Memory.Health),
		Actions: contract.SettingsMemoryActionsPayload{
			Consolidate: settingsActionMetadataPayload(envelope.Memory.Actions.Consolidate),
		},
	}, nil
}

func settingsSkillsSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Skills == nil {
		return nil, errors.New("settings skills section is required")
	}
	return contract.SettingsSkillsResponse{
		SettingsSkillsSectionResponseMetaPayload: settingsSkillsSectionMetaPayload(envelope),
		Config:                                   settingsSkillsConfigPayload(envelope.Skills.Config),
		DiscoveredCount:                          envelope.Skills.DiscoveredCount,
		DisabledCount:                            envelope.Skills.DisabledCount,
		RuntimeAvailable:                         envelope.Skills.RuntimeAvailable,
		Diagnostics:                              SkillDiagnosticPayloadsFromDiagnostics(envelope.Skills.Diagnostics),
		Links:                                    settingsOperationalLinkPayloads(envelope.Skills.Links),
	}, nil
}

func settingsAutomationSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Automation == nil {
		return nil, errors.New("settings automation section is required")
	}
	return contract.SettingsAutomationResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                   settingsAutomationConfigPayload(envelope.Automation.Config),
		Runtime:                                  settingsAutomationRuntimePayload(envelope.Automation.Runtime),
		Links:                                    settingsOperationalLinkPayloads(envelope.Automation.Links),
	}, nil
}

func settingsNetworkSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Network == nil {
		return nil, errors.New("settings network section is required")
	}
	return contract.SettingsNetworkResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                   settingsNetworkConfigPayload(envelope.Network.Config),
		Runtime:                                  settingsNetworkRuntimePayload(envelope.Network.Runtime),
		Links:                                    settingsOperationalLinkPayloads(envelope.Network.Links),
	}, nil
}

func settingsObservabilitySectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Observability == nil {
		return nil, errors.New("settings observability section is required")
	}
	return contract.SettingsObservabilityResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                   settingsObservabilityConfigPayload(envelope.Observability.Config),
		Runtime:                                  settingsObservabilityRuntimePayload(envelope.Observability.Runtime),
		LogTail: settingsLogTailCapabilityPayload(
			envelope.Observability.LogTailSupport,
		),
	}, nil
}

func settingsHooksExtensionsSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.HooksExtensions == nil {
		return nil, errors.New("settings hooks-extensions section is required")
	}
	return contract.SettingsHooksExtensionsResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Hooks:                                    settingsHookItemPayloads(envelope.HooksExtensions.Hooks),
		Config:                                   settingsExtensionsConfigPayload(envelope.HooksExtensions.Extensions),
		Installed: settingsInstalledExtensionPayloads(
			envelope.HooksExtensions.Installed,
		),
		TransportParity: settingsTransportParityPayload(
			envelope.HooksExtensions.TransportParity,
		),
	}, nil
}

// SettingsCollectionResponseFromEnvelope converts one settings collection envelope into the shared response payload.
func SettingsCollectionResponseFromEnvelope(envelope settingspkg.CollectionEnvelope) (any, error) {
	switch envelope.Collection {
	case settingspkg.CollectionProviders:
		return contract.SettingsProvidersResponse{
			SettingsGlobalCollectionResponseMetaPayload: settingsGlobalCollectionMetaPayload(envelope),
			Providers: settingsProviderItemPayloads(envelope.Providers),
		}, nil
	case settingspkg.CollectionMCPServers:
		return contract.SettingsMCPServersResponse{
			SettingsGlobalWorkspaceCollectionResponseMetaPayload: settingsGlobalWorkspaceCollectionMetaPayload(
				envelope,
			),
			MCPServers: settingsMCPServerItemPayloads(envelope.MCPServers),
		}, nil
	case settingspkg.CollectionSandboxes:
		return contract.SettingsSandboxesResponse{
			SettingsGlobalCollectionResponseMetaPayload: settingsGlobalCollectionMetaPayload(envelope),
			Sandboxes: settingsSandboxItemPayloads(envelope.Sandboxes),
		}, nil
	case settingspkg.CollectionHooks:
		return contract.SettingsHooksResponse{
			SettingsGlobalCollectionResponseMetaPayload: settingsGlobalCollectionMetaPayload(envelope),
			Hooks: settingsHookItemPayloads(envelope.Hooks),
		}, nil
	default:
		return nil, fmt.Errorf("unknown settings collection %q", envelope.Collection)
	}
}

// SettingsSectionMutationResultPayloadFromResult converts one settings section mutation result into the shared payload.
func SettingsSectionMutationResultPayloadFromResult(result settingspkg.MutationResult) (any, error) {
	switch result.Section {
	case settingspkg.SectionGeneral,
		settingspkg.SectionMemory,
		settingspkg.SectionRoles,
		settingspkg.SectionAutomation,
		settingspkg.SectionNetwork,
		settingspkg.SectionWindowManager,
		settingspkg.SectionObservability,
		settingspkg.SectionHooksExtensions:
		return contract.SettingsGlobalSectionMutationResult{
			Section:         contract.SettingsSectionName(result.Section),
			Scope:           contract.SettingsGlobalScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			Behavior:        contract.SettingsMutationBehavior(result.Behavior),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			RestartScope:    strings.TrimSpace(result.RestartScope),
			Warnings:        cloneStrings(result.Warnings),
		}, nil
	case settingspkg.SectionSkills:
		return contract.SettingsSkillsMutationResult{
			Section:         contract.SettingsSectionName(result.Section),
			Scope:           contract.SettingsAgentScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			WorkspaceID:     strings.TrimSpace(result.WorkspaceID),
			AgentName:       strings.TrimSpace(result.AgentName),
			Behavior:        contract.SettingsMutationBehavior(result.Behavior),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			RestartScope:    strings.TrimSpace(result.RestartScope),
			Warnings:        cloneStrings(result.Warnings),
		}, nil
	default:
		return nil, fmt.Errorf("unknown settings section mutation %q", result.Section)
	}
}

// SettingsCollectionMutationResultPayloadFromResult converts one settings
// collection mutation result into the shared payload.
func SettingsCollectionMutationResultPayloadFromResult(result settingspkg.MutationResult) (any, error) {
	collection := contract.SettingsCollectionName(result.Section)
	switch collection {
	case contract.SettingsCollectionProviders,
		contract.SettingsCollectionSandboxes,
		contract.SettingsCollectionHooks:
		return contract.SettingsGlobalCollectionMutationResult{
			Section:         collection,
			Scope:           contract.SettingsGlobalScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			Behavior:        contract.SettingsMutationBehavior(result.Behavior),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			RestartScope:    strings.TrimSpace(result.RestartScope),
			Warnings:        cloneStrings(result.Warnings),
		}, nil
	case contract.SettingsCollectionMCPServers:
		return contract.SettingsGlobalWorkspaceCollectionMutationResult{
			Section:         collection,
			Scope:           contract.SettingsWorkspaceScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			WorkspaceID:     strings.TrimSpace(result.WorkspaceID),
			Behavior:        contract.SettingsMutationBehavior(result.Behavior),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			RestartScope:    strings.TrimSpace(result.RestartScope),
			Warnings:        cloneStrings(result.Warnings),
		}, nil
	default:
		return nil, fmt.Errorf("unknown settings collection mutation %q", result.Section)
	}
}

// SettingsApplyResponseFromResult converts one settings apply result into the public payload.
func SettingsApplyResponseFromResult(result settingspkg.ApplyResult) contract.SettingsApplyResponse {
	return contract.SettingsApplyResponse{
		Section:          contract.SettingsApplyTargetName(strings.TrimSpace(string(result.Section))),
		Scope:            contract.SettingsScopeKind(strings.TrimSpace(string(result.Scope))),
		WriteTarget:      contract.SettingsWriteTargetKind(result.WriteTarget),
		WorkspaceID:      strings.TrimSpace(result.WorkspaceID),
		AgentName:        strings.TrimSpace(result.AgentName),
		Applied:          result.Applied,
		Lifecycle:        contract.SettingsApplyLifecycle(result.Record.Lifecycle),
		ApplyRecordID:    strings.TrimSpace(result.Record.ID),
		ActiveGeneration: result.Record.Generation,
		ActiveConfigHash: strings.TrimSpace(result.Record.ActiveHash),
		NextAction:       contract.SettingsApplyNextAction(result.NextAction),
		RestartRequired:  result.RestartRequired,
		RestartScope:     strings.TrimSpace(result.RestartScope),
		Warnings:         cloneStrings(result.Warnings),
		PartialFailures:  settingsApplyFailurePayloads(result.PartialFailures),
		Skipped:          result.Skipped,
		SkippedReason:    strings.TrimSpace(result.SkippedReason),
	}
}

// ConfigApplyRecordsResponseFromRecords converts apply history rows into the public payload.
func ConfigApplyRecordsResponseFromRecords(
	records []settingspkg.ApplyRecord,
) contract.ConfigApplyRecordsResponse {
	entries := make([]contract.ConfigApplyRecordPayload, 0, len(records))
	for _, record := range records {
		entries = append(entries, configApplyRecordPayload(record))
	}
	return contract.ConfigApplyRecordsResponse{Entries: entries}
}

func configApplyRecordPayload(record settingspkg.ApplyRecord) contract.ConfigApplyRecordPayload {
	return contract.ConfigApplyRecordPayload{
		ID:                strings.TrimSpace(record.ID),
		DesiredConfigHash: strings.TrimSpace(record.DesiredHash),
		ActiveConfigHash:  strings.TrimSpace(record.ActiveHash),
		Generation:        record.Generation,
		Actor:             strings.TrimSpace(record.Actor),
		DiffClass:         contract.SettingsApplyLifecycle(record.DiffClass),
		Status:            contract.ConfigApplyStatus(record.Status),
		Lifecycle:         contract.SettingsApplyLifecycle(record.Lifecycle),
		NextAction:        contract.SettingsApplyNextAction(record.NextAction),
		Diagnostics:       append([]contract.DiagnosticItem(nil), record.Diagnostics...),
		CreatedAt:         record.CreatedAt,
		AppliedAt:         record.AppliedAt,
		UpdatedAt:         record.UpdatedAt,
	}
}

func settingsApplyFailurePayloads(
	failures []settingspkg.ApplyFailure,
) []contract.SettingsApplyFailurePayload {
	if len(failures) == 0 {
		return nil
	}
	payloads := make([]contract.SettingsApplyFailurePayload, 0, len(failures))
	for _, failure := range failures {
		payloads = append(payloads, contract.SettingsApplyFailurePayload{
			Subsystem:  strings.TrimSpace(failure.Subsystem),
			Diagnostic: failure.Diagnostic,
		})
	}
	return payloads
}

// SettingsRestartActionResponseFromOperation converts one daemon restart operation into the action response payload.
func SettingsRestartActionResponseFromOperation(operation SettingsRestartOperation) contract.RestartActionResponse {
	return contract.RestartActionResponse{
		OperationID:        strings.TrimSpace(operation.OperationID),
		Status:             contract.RestartOperationStatus(operation.Status),
		StatusURL:          settingsRestartStatusURL(operation.OperationID),
		ActiveSessionCount: operation.ActiveSessionCount,
	}
}

// SettingsRestartActionStatusFromOperation converts one daemon restart operation into the polling payload.
func SettingsRestartActionStatusFromOperation(operation SettingsRestartOperation) contract.RestartActionStatus {
	return contract.RestartActionStatus{
		OperationID:        strings.TrimSpace(operation.OperationID),
		Status:             contract.RestartOperationStatus(operation.Status),
		OldPID:             operation.OldPID,
		OldStartedAt:       operation.OldStartedAt,
		OldSocketPath:      strings.TrimSpace(operation.OldSocketPath),
		NewPID:             operation.NewPID,
		ActiveSessionCount: operation.ActiveSessionCount,
		FailureReason:      strings.TrimSpace(operation.FailureReason),
		StartedAt:          operation.StartedAt,
		UpdatedAt:          operation.UpdatedAt,
		CompletedAt:        cloneTimePointer(operation.CompletedAt),
	}
}

// SettingsUpdateResponseFromStatus converts the daemon-owned update snapshot into the transport payload.
func SettingsUpdateResponseFromStatus(status SettingsUpdateStatus) contract.SettingsUpdateResponse {
	return contract.SettingsUpdateResponse{
		Supported:      status.Supported,
		Managed:        status.Managed,
		InstallMethod:  strings.TrimSpace(status.InstallMethod),
		CurrentVersion: strings.TrimSpace(status.CurrentVersion),
		LatestVersion:  strings.TrimSpace(status.LatestVersion),
		Available:      status.Available,
		Status:         contract.SettingsUpdateStatusKind(strings.TrimSpace(status.Status)),
		Recommendation: strings.TrimSpace(status.Recommendation),
		ReleaseURL:     strings.TrimSpace(status.ReleaseURL),
		CheckedAt:      cloneTimePointer(status.CheckedAt),
		LastError:      strings.TrimSpace(status.LastError),
	}
}

func settingsGlobalSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsGlobalSectionResponseMetaPayload {
	return contract.SettingsGlobalSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsGlobalScopeKind(envelope.Scope),
		AvailableScopes: settingsGlobalScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsSkillsSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsSkillsSectionResponseMetaPayload {
	return contract.SettingsSkillsSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsAgentScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		AgentName:       strings.TrimSpace(envelope.AgentName),
		AvailableScopes: settingsAgentScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsGlobalCollectionMetaPayload(
	envelope settingspkg.CollectionEnvelope,
) contract.SettingsGlobalCollectionResponseMetaPayload {
	return contract.SettingsGlobalCollectionResponseMetaPayload{
		Collection:      contract.SettingsCollectionName(envelope.Collection),
		Scope:           contract.SettingsGlobalScopeKind(envelope.Scope),
		AvailableScopes: settingsGlobalScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsGlobalWorkspaceCollectionMetaPayload(
	envelope settingspkg.CollectionEnvelope,
) contract.SettingsGlobalWorkspaceCollectionResponseMetaPayload {
	return contract.SettingsGlobalWorkspaceCollectionResponseMetaPayload{
		Collection:      contract.SettingsCollectionName(envelope.Collection),
		Scope:           contract.SettingsWorkspaceScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		AvailableScopes: settingsWorkspaceScopeKindsPayload(envelope.AvailableScopes),
	}
}
