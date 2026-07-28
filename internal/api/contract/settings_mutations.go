package contract

import "time"

type UpdateSettingsGeneralRequest struct {
	Config SettingsGeneralConfigPayload `json:"config"`
}

type UpdateSettingsMemoryRequest struct {
	Config SettingsMemoryConfigPayload `json:"config"`
}

type UpdateSettingsRolesRequest struct {
	Config SettingsRolesConfigPayload `json:"config"`
}

type UpdateSettingsSkillsRequest struct {
	Config SettingsSkillsConfigPayload `json:"config"`
}

type UpdateSettingsAutomationRequest struct {
	Config SettingsAutomationConfigPayload `json:"config"`
}

type UpdateSettingsNetworkRequest struct {
	Config SettingsNetworkConfigPayload `json:"config"`
}

type UpdateSettingsObservabilityRequest struct {
	Config SettingsObservabilityConfigPayload `json:"config"`
}

type UpdateSettingsHooksExtensionsRequest struct {
	Config SettingsExtensionsConfigPayload `json:"config"`
}

type PutSettingsProviderRequest struct {
	Settings      SettingsProviderSettingsPayload      `json:"settings"`
	ModelCuration *ProviderModelCurationRequest        `json:"model_curation,omitempty"`
	Secrets       []SettingsProviderSecretWritePayload `json:"secrets,omitempty"`
}

type PutSettingsMCPServerRequest struct {
	Server          SettingsMCPServerPayload              `json:"server"`
	SecretValues    *SettingsMCPSecretValuesPayload       `json:"secret_values,omitempty"`
	PreserveSecrets *SettingsMCPSecretPreservationPayload `json:"preserve_secrets,omitempty"`
	PreserveEnv     []string                              `json:"preserve_env,omitempty"`
}

type PutSettingsSandboxRequest struct {
	Profile SettingsSandboxProfilePayload `json:"profile"`
}

type PutSettingsHookRequest struct {
	Declaration SettingsHookDeclarationPayload `json:"declaration"`
}

type SettingsGeneralResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	ConfigPaths SettingsConfigPathsPayload    `json:"config_paths"`
	Config      SettingsGeneralConfigPayload  `json:"config"`
	Runtime     SettingsDaemonRuntimePayload  `json:"runtime"`
	Actions     SettingsGeneralActionsPayload `json:"actions"`
}

type SettingsMemoryResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config  SettingsMemoryConfigPayload  `json:"config"`
	Health  SettingsMemoryHealthPayload  `json:"health"`
	Actions SettingsMemoryActionsPayload `json:"actions"`
}

type SettingsRolesResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config SettingsRolesConfigPayload `json:"config"`
}

type SettingsSkillsResponse struct {
	SettingsSkillsSectionResponseMetaPayload
	Config           SettingsSkillsConfigPayload      `json:"config"`
	DiscoveredCount  int                              `json:"discovered_count"`
	DisabledCount    int                              `json:"disabled_count"`
	RuntimeAvailable bool                             `json:"runtime_available"`
	Diagnostics      []SkillDiagnosticPayload         `json:"diagnostics,omitempty"`
	Links            []SettingsOperationalLinkPayload `json:"links,omitempty"`
}

type SettingsAutomationResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config  SettingsAutomationConfigPayload  `json:"config"`
	Runtime SettingsAutomationRuntimePayload `json:"runtime"`
	Links   []SettingsOperationalLinkPayload `json:"links,omitempty"`
}

type SettingsNetworkResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config  SettingsNetworkConfigPayload     `json:"config"`
	Runtime SettingsNetworkRuntimePayload    `json:"runtime"`
	Links   []SettingsOperationalLinkPayload `json:"links,omitempty"`
}

type SettingsObservabilityResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config  SettingsObservabilityConfigPayload  `json:"config"`
	Runtime SettingsObservabilityRuntimePayload `json:"runtime"`
	LogTail SettingsLogTailCapabilityPayload    `json:"log_tail"`
}

type SettingsHooksExtensionsResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Hooks           []SettingsHookItemPayload           `json:"hooks,omitempty"`
	Config          SettingsExtensionsConfigPayload     `json:"config"`
	Installed       []SettingsInstalledExtensionPayload `json:"installed,omitempty"`
	TransportParity SettingsTransportParityPayload      `json:"transport_parity"`
}

type SettingsProvidersResponse struct {
	SettingsGlobalCollectionResponseMetaPayload
	Providers []SettingsProviderItemPayload `json:"providers"`
}

type SettingsProviderResponse struct {
	Provider SettingsProviderItemPayload `json:"provider"`
}

type SettingsMCPServersResponse struct {
	SettingsGlobalWorkspaceCollectionResponseMetaPayload
	MCPServers []SettingsMCPServerItemPayload `json:"mcp_servers"`
}

type SettingsSandboxesResponse struct {
	SettingsGlobalCollectionResponseMetaPayload
	Sandboxes []SettingsSandboxItemPayload `json:"sandboxes"`
}

type SettingsSandboxResponse struct {
	Sandbox SettingsSandboxItemPayload `json:"sandbox"`
}

type SettingsHooksResponse struct {
	SettingsGlobalCollectionResponseMetaPayload
	Hooks []SettingsHookItemPayload `json:"hooks"`
}

type SettingsHookResponse struct {
	Hook SettingsHookItemPayload `json:"hook"`
}

type SettingsGlobalSectionMutationResult struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsGlobalScopeKind  `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type SettingsSkillsMutationResult struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsAgentScopeKind   `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	WorkspaceID     string                   `json:"workspace_id,omitempty"`
	AgentName       string                   `json:"agent_name,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type SettingsGlobalCollectionMutationResult struct {
	Section         SettingsCollectionName   `json:"section"`
	Scope           SettingsGlobalScopeKind  `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type SettingsGlobalWorkspaceCollectionMutationResult struct {
	Section         SettingsCollectionName     `json:"section"`
	Scope           SettingsWorkspaceScopeKind `json:"scope"`
	WriteTarget     SettingsWriteTargetKind    `json:"write_target,omitempty"`
	WorkspaceID     string                     `json:"workspace_id,omitempty"`
	Behavior        SettingsMutationBehavior   `json:"behavior"`
	Applied         bool                       `json:"applied"`
	RestartRequired bool                       `json:"restart_required"`
	RestartScope    string                     `json:"restart_scope,omitempty"`
	Warnings        []string                   `json:"warnings,omitempty"`
}

type SettingsApplyFailurePayload struct {
	Subsystem  string         `json:"subsystem"`
	Diagnostic DiagnosticItem `json:"diagnostic"`
}

type SettingsApplyResponse struct {
	Section          SettingsApplyTargetName       `json:"section,omitempty"`
	Scope            SettingsScopeKind             `json:"scope,omitempty"`
	WriteTarget      SettingsWriteTargetKind       `json:"write_target,omitempty"`
	WorkspaceID      string                        `json:"workspace_id,omitempty"`
	AgentName        string                        `json:"agent_name,omitempty"`
	Applied          bool                          `json:"applied"`
	Lifecycle        SettingsApplyLifecycle        `json:"lifecycle"`
	ApplyRecordID    string                        `json:"apply_record_id"`
	ActiveGeneration int64                         `json:"active_generation"`
	ActiveConfigHash string                        `json:"active_config_hash"`
	NextAction       SettingsApplyNextAction       `json:"next_action"`
	RestartRequired  bool                          `json:"restart_required,omitempty"`
	RestartScope     string                        `json:"restart_scope,omitempty"`
	Warnings         []string                      `json:"warnings,omitempty"`
	PartialFailures  []SettingsApplyFailurePayload `json:"partial_failures,omitempty"`
	Skipped          bool                          `json:"skipped,omitempty"`
	SkippedReason    string                        `json:"skipped_reason,omitempty"`
}

type ConfigApplyRecordPayload struct {
	ID                string                  `json:"id"`
	DesiredConfigHash string                  `json:"desired_config_hash"`
	ActiveConfigHash  string                  `json:"active_config_hash"`
	Generation        int64                   `json:"generation"`
	Actor             string                  `json:"actor"`
	DiffClass         SettingsApplyLifecycle  `json:"diff_class"`
	Status            ConfigApplyStatus       `json:"status"`
	Lifecycle         SettingsApplyLifecycle  `json:"lifecycle"`
	NextAction        SettingsApplyNextAction `json:"next_action"`
	Diagnostics       []DiagnosticItem        `json:"diagnostics,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	AppliedAt         *time.Time              `json:"applied_at,omitempty"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type ConfigApplyRecordsResponse struct {
	Entries []ConfigApplyRecordPayload `json:"entries"`
}

type RestartActionResponse struct {
	OperationID        string                 `json:"operation_id"`
	Status             RestartOperationStatus `json:"status"`
	StatusURL          string                 `json:"status_url"`
	ActiveSessionCount int                    `json:"active_session_count"`
}

type RestartActionStatus struct {
	OperationID        string                 `json:"operation_id"`
	Status             RestartOperationStatus `json:"status"`
	OldPID             int                    `json:"old_pid"`
	OldStartedAt       time.Time              `json:"old_started_at"`
	OldSocketPath      string                 `json:"old_socket_path"`
	NewPID             int                    `json:"new_pid,omitempty"`
	ActiveSessionCount int                    `json:"active_session_count"`
	FailureReason      string                 `json:"failure_reason,omitempty"`
	StartedAt          time.Time              `json:"started_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	CompletedAt        *time.Time             `json:"completed_at,omitempty"`
}

type SettingsUpdateResponse struct {
	Supported      bool                     `json:"supported"`
	Managed        bool                     `json:"managed"`
	InstallMethod  string                   `json:"install_method"`
	CurrentVersion string                   `json:"current_version"`
	LatestVersion  string                   `json:"latest_version,omitempty"`
	Available      bool                     `json:"available"`
	Status         SettingsUpdateStatusKind `json:"status"`
	Recommendation string                   `json:"recommendation,omitempty"`
	ReleaseURL     string                   `json:"release_url,omitempty"`
	CheckedAt      *time.Time               `json:"checked_at,omitempty"`
	LastError      string                   `json:"last_error,omitempty"`
}
