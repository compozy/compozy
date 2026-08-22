package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

// SettingsSectionResponseFromEnvelope converts one settings section envelope into the shared response payload.
func SettingsSectionResponseFromEnvelope(envelope settingspkg.SectionEnvelope) (any, error) {
	switch envelope.Section {
	case settingspkg.SectionGeneral:
		return settingsGeneralSectionResponse(envelope)
	case settingspkg.SectionPersona:
		return settingsPersonaSectionResponse(envelope)
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
	case settingspkg.SectionCmdPalette:
		return settingsCmdPaletteSectionResponse(envelope)
	case settingspkg.SectionAttention:
		return settingsAttentionSectionResponse(envelope)
	case settingspkg.SectionShell:
		return settingsShellSectionResponse(envelope)
	case settingspkg.SectionObservability:
		return settingsObservabilitySectionResponse(envelope)
	case settingspkg.SectionHooksExtensions:
		return settingsHooksExtensionsSectionResponse(envelope)
	default:
		return nil, fmt.Errorf("unknown settings section %q", envelope.Section)
	}
}

func settingsPersonaSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Persona == nil {
		return nil, errors.New("settings persona section is required")
	}
	return contract.SettingsPersonaResponse{
		SettingsLayeredSectionResponseMetaPayload: settingsGlobalWorkspaceSectionMetaPayload(envelope),
		Config: settingsDefaultsPayload(envelope.Persona.Config),
	}, nil
}

func settingsRolesSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Roles == nil {
		return nil, errors.New("settings roles section is required")
	}
	return contract.SettingsRolesResponse{
		SettingsWorkspaceSectionResponseMetaPayload: settingsUserWorkspaceSectionMetaPayload(envelope),
		Config: settingsRolesConfigPayload(&envelope.Roles.Config),
	}, nil
}

func settingsGeneralSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.General == nil {
		return nil, errors.New("settings general section is required")
	}
	return contract.SettingsGeneralResponse{
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		ConfigPaths:                            settingsConfigPathsPayload(envelope.General.ConfigPaths),
		Config:                                 settingsGeneralConfigPayload(envelope.General.Settings),
		Runtime:                                settingsDaemonRuntimePayload(envelope.General.Runtime),
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
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                 settingsMemoryConfigPayload(&envelope.Memory.Config),
		Health:                                 settingsMemoryHealthPayload(envelope.Memory.Health),
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
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                 settingsAutomationConfigPayload(envelope.Automation.Config),
		Runtime:                                settingsAutomationRuntimePayload(envelope.Automation.Runtime),
		Links:                                  settingsOperationalLinkPayloads(envelope.Automation.Links),
	}, nil
}

func settingsNetworkSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Network == nil {
		return nil, errors.New("settings network section is required")
	}
	return contract.SettingsNetworkResponse{
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                 settingsNetworkConfigPayload(envelope.Network.Config),
		Runtime:                                settingsNetworkRuntimePayload(envelope.Network.Runtime),
		Links:                                  settingsOperationalLinkPayloads(envelope.Network.Links),
	}, nil
}

func settingsObservabilitySectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Observability == nil {
		return nil, errors.New("settings observability section is required")
	}
	return contract.SettingsObservabilityResponse{
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config:                                 settingsObservabilityConfigPayload(envelope.Observability.Config),
		Runtime:                                settingsObservabilityRuntimePayload(envelope.Observability.Runtime),
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
		SettingsUserSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Hooks:                                  settingsHookItemPayloads(envelope.HooksExtensions.Hooks),
		Config:                                 settingsExtensionsConfigPayload(envelope.HooksExtensions.Extensions),
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
			SettingsUserCollectionResponseMetaPayload: settingsGlobalCollectionMetaPayload(envelope),
			Providers: settingsProviderItemPayloads(envelope.Providers),
		}, nil
	case settingspkg.CollectionMCPServers:
		return contract.SettingsMCPServersResponse{
			SettingsLayeredCollectionResponseMetaPayload: settingsGlobalWorkspaceCollectionMetaPayload(
				envelope,
			),
			MCPServers: settingsMCPServerItemPayloads(envelope.MCPServers),
		}, nil
	case settingspkg.CollectionSandboxes:
		return contract.SettingsSandboxesResponse{
			SettingsUserCollectionResponseMetaPayload: settingsGlobalCollectionMetaPayload(envelope),
			Sandboxes: settingsSandboxItemPayloads(envelope.Sandboxes),
		}, nil
	case settingspkg.CollectionHooks:
		return contract.SettingsHooksResponse{
			SettingsLayeredCollectionResponseMetaPayload: settingsGlobalWorkspaceCollectionMetaPayload(
				envelope,
			),
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
		settingspkg.SectionAutomation,
		settingspkg.SectionNetwork,
		settingspkg.SectionAttention,
		settingspkg.SectionShell,
		settingspkg.SectionObservability,
		settingspkg.SectionHooksExtensions:
		return contract.SettingsUserSectionMutationResult{
			Section:         contract.SettingsSectionName(result.Section),
			Scope:           contract.SettingsUserScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			Behavior:        contract.SettingsMutationBehavior(result.Behavior),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			RestartScope:    strings.TrimSpace(result.RestartScope),
			Warnings:        cloneStrings(result.Warnings),
		}, nil
	case settingspkg.SectionPersona, settingspkg.SectionCmdPalette:
		return contract.SettingsLayeredSectionMutationResult{
			Section: contract.SettingsSectionName(result.Section), Scope: contract.SettingsLayeredScopeKind(result.Scope),
			WriteTarget: contract.SettingsWriteTargetKind(result.WriteTarget),
			WorkspaceID: strings.TrimSpace(result.WorkspaceID), Profile: strings.TrimSpace(result.ProfileName),
			Behavior: contract.SettingsMutationBehavior(result.Behavior), Applied: result.Applied,
			RestartRequired: result.RestartRequired, RestartScope: strings.TrimSpace(result.RestartScope),
			Warnings: cloneStrings(result.Warnings),
		}, nil
	case settingspkg.SectionRoles, settingspkg.SectionWindowManager:
		return contract.SettingsWorkspaceSectionMutationResult{
			Section:         contract.SettingsSectionName(result.Section),
			Scope:           contract.SettingsWorkspaceScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			WorkspaceID:     strings.TrimSpace(result.WorkspaceID),
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
		contract.SettingsCollectionSandboxes:
		return contract.SettingsUserCollectionMutationResult{
			Section:         collection,
			Scope:           contract.SettingsUserScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			Behavior:        contract.SettingsMutationBehavior(result.Behavior),
			Applied:         result.Applied,
			RestartRequired: result.RestartRequired,
			RestartScope:    strings.TrimSpace(result.RestartScope),
			Warnings:        cloneStrings(result.Warnings),
		}, nil
	case contract.SettingsCollectionMCPServers, contract.SettingsCollectionHooks:
		return contract.SettingsLayeredCollectionMutationResult{
			Section:         collection,
			Scope:           contract.SettingsLayeredScopeKind(result.Scope),
			WriteTarget:     contract.SettingsWriteTargetKind(result.WriteTarget),
			WorkspaceID:     strings.TrimSpace(result.WorkspaceID),
			Profile:         strings.TrimSpace(result.ProfileName),
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
		Profile:          strings.TrimSpace(result.ProfileName),
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
		WriteTarget:       contract.SettingsWriteTargetKind(record.WriteTarget),
		WritePath:         strings.TrimSpace(record.WritePath),
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
func SettingsUpdateResponseFromStatus(status compozyupdate.MultiState) contract.SettingsUpdateResponse {
	response := contract.SettingsUpdateResponse{
		Aggregate: contract.SettingsUpdateStatusKind(status.Aggregate),
		Runtime: contract.SettingsUpdateRuntimePayload{
			Status:          contract.SettingsUpdateStatusKind(status.Runtime.Status),
			InstallMethod:   settingsUpdateInstallMethodPayload(status.Runtime.InstallMethod),
			Managed:         status.Runtime.Managed,
			CurrentVersion:  strings.TrimSpace(status.Runtime.CurrentVersion),
			LatestVersion:   strings.TrimSpace(status.Runtime.LatestVersion),
			ReleaseURL:      strings.TrimSpace(status.Runtime.ReleaseURL),
			Recommendation:  strings.TrimSpace(status.Runtime.Recommendation),
			RestoredVersion: strings.TrimSpace(status.Runtime.RestoredVersion),
			DaemonRestarted: status.Runtime.DaemonRestarted,
			Message:         strings.TrimSpace(status.Runtime.Message),
			LastError:       strings.TrimSpace(status.Runtime.LastError),
		},
	}
	if status.Operation != nil {
		response.Operation = settingsUpdateOperationPayload(status.Operation)
	}
	if status.App != nil {
		response.App = &contract.SettingsUpdateAppPayload{
			Status:         contract.SettingsUpdateStatusKind(status.App.Status),
			Running:        status.App.Running,
			CurrentVersion: strings.TrimSpace(status.App.CurrentVersion),
			LatestVersion:  strings.TrimSpace(status.App.LatestVersion),
			ReleaseURL:     strings.TrimSpace(status.App.ReleaseURL),
			AttemptID:      strings.TrimSpace(status.App.AttemptID),
			LastError:      strings.TrimSpace(status.App.LastError),
			Message:        strings.TrimSpace(status.App.Message),
		}
	}
	return response
}

func settingsUpdateInstallMethodPayload(raw string) contract.SettingsUpdateInstallMethod {
	method := contract.SettingsUpdateInstallMethod(strings.TrimSpace(raw))
	if method == "" {
		return contract.SettingsUpdateInstallUnknown
	}
	return method
}

func settingsUpdateOperationPayload(operation *compozyupdate.OperationView) *contract.SettingsUpdateOperationPayload {
	payload := &contract.SettingsUpdateOperationPayload{
		ID:        operation.ID,
		Revision:  operation.Revision,
		Targets:   make([]contract.SettingsUpdateTarget, 0, len(operation.Targets)),
		Phase:     contract.SettingsUpdatePhase(operation.Phase),
		Percent:   operation.Percent,
		Waiting:   contract.SettingsUpdateWaitingState(operation.Waiting),
		StartedAt: operation.StartedAt,
		LastError: operation.LastError,
	}
	for _, target := range operation.Targets {
		payload.Targets = append(payload.Targets, contract.SettingsUpdateTarget(target))
	}
	if operation.ActiveTarget != "" {
		activeTarget := contract.SettingsUpdateTarget(operation.ActiveTarget)
		payload.ActiveTarget = &activeTarget
	}
	payload.Holder = settingsUpdateHolderPayload(operation.Holder)
	return payload
}

func settingsUpdateHolderPayload(holder *compozyupdate.Holder) *contract.SettingsUpdateHolderPayload {
	if holder == nil {
		return nil
	}
	return &contract.SettingsUpdateHolderPayload{
		PID:                holder.PID,
		PIDStartTime:       holder.PIDStartTime,
		Surface:            contract.SettingsUpdateActor(holder.Surface),
		ExecutorGeneration: holder.ExecutorGeneration,
		LeaseExpiresAt:     holder.LeaseExpiresAt,
	}
}

func SettingsUpdateApplyResponseFromResult(result SettingsUpdateApply) contract.SettingsUpdateApplyResponse {
	return contract.SettingsUpdateApplyResponse{
		Target:      contract.SettingsUpdateTarget(result.Target),
		Status:      contract.SettingsUpdateApplyStatus(result.Status),
		OperationID: strings.TrimSpace(result.OperationID),
		Message:     strings.TrimSpace(result.Message),
		Holder:      settingsUpdateHolderPayload(result.Holder),
	}
}

func SettingsUpdateCancelResponseFromResult(result SettingsUpdateCancel) contract.SettingsUpdateCancelResponse {
	return contract.SettingsUpdateCancelResponse{
		Status:      contract.SettingsUpdateStatusKind(result.Status),
		OperationID: strings.TrimSpace(result.OperationID),
		Message:     strings.TrimSpace(result.Message),
		Holder:      settingsUpdateHolderPayload(result.Holder),
	}
}
