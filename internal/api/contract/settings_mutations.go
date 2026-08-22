package contract

import "time"

type UpdateSettingsGeneralRequest struct {
	Config SettingsGeneralConfigPayload `json:"config"`
}

type UpdateSettingsPersonaRequest struct {
	Config SettingsDefaultsPayload `json:"config"`
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
	Settings      SettingsProviderWritePayload         `json:"settings"`
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
	SettingsUserSectionResponseMetaPayload
	ConfigPaths SettingsConfigPathsPayload    `json:"config_paths"`
	Config      SettingsGeneralConfigPayload  `json:"config"`
	Runtime     SettingsDaemonRuntimePayload  `json:"runtime"`
	Actions     SettingsGeneralActionsPayload `json:"actions"`
}

type SettingsPersonaResponse struct {
	SettingsLayeredSectionResponseMetaPayload
	Config SettingsDefaultsPayload `json:"config"`
}

type SettingsMemoryResponse struct {
	SettingsUserSectionResponseMetaPayload
	Config  SettingsMemoryConfigPayload  `json:"config"`
	Health  SettingsMemoryHealthPayload  `json:"health"`
	Actions SettingsMemoryActionsPayload `json:"actions"`
}

type SettingsRolesResponse struct {
	SettingsWorkspaceSectionResponseMetaPayload
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
	SettingsUserSectionResponseMetaPayload
	Config  SettingsAutomationConfigPayload  `json:"config"`
	Runtime SettingsAutomationRuntimePayload `json:"runtime"`
	Links   []SettingsOperationalLinkPayload `json:"links,omitempty"`
}

type SettingsNetworkResponse struct {
	SettingsUserSectionResponseMetaPayload
	Config  SettingsNetworkConfigPayload     `json:"config"`
	Runtime SettingsNetworkRuntimePayload    `json:"runtime"`
	Links   []SettingsOperationalLinkPayload `json:"links,omitempty"`
}

type SettingsObservabilityResponse struct {
	SettingsUserSectionResponseMetaPayload
	Config  SettingsObservabilityConfigPayload  `json:"config"`
	Runtime SettingsObservabilityRuntimePayload `json:"runtime"`
	LogTail SettingsLogTailCapabilityPayload    `json:"log_tail"`
}

type SettingsHooksExtensionsResponse struct {
	SettingsUserSectionResponseMetaPayload
	Hooks           []SettingsHookItemPayload           `json:"hooks,omitempty"`
	Config          SettingsExtensionsConfigPayload     `json:"config"`
	Installed       []SettingsInstalledExtensionPayload `json:"installed,omitempty"`
	TransportParity SettingsTransportParityPayload      `json:"transport_parity"`
}

type SettingsProvidersResponse struct {
	SettingsUserCollectionResponseMetaPayload
	Providers []SettingsProviderItemPayload `json:"providers"`
}

type SettingsProviderResponse struct {
	Provider SettingsProviderItemPayload `json:"provider"`
}

type SettingsMCPServersResponse struct {
	SettingsLayeredCollectionResponseMetaPayload
	MCPServers []SettingsMCPServerItemPayload `json:"mcp_servers"`
}

type SettingsSandboxesResponse struct {
	SettingsUserCollectionResponseMetaPayload
	Sandboxes []SettingsSandboxItemPayload `json:"sandboxes"`
}

type SettingsSandboxResponse struct {
	Sandbox SettingsSandboxItemPayload `json:"sandbox"`
}

type SettingsHooksResponse struct {
	SettingsLayeredCollectionResponseMetaPayload
	Hooks []SettingsHookItemPayload `json:"hooks"`
}

type SettingsHookResponse struct {
	Hook SettingsHookItemPayload `json:"hook"`
}

type SettingsUserSectionMutationResult struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsUserScopeKind    `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type SettingsLayeredSectionMutationResult struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsLayeredScopeKind `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	WorkspaceID     string                   `json:"workspace_id,omitempty"`
	Profile         string                   `json:"profile,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type SettingsWorkspaceSectionMutationResult struct {
	Section         SettingsSectionName        `json:"section"`
	Scope           SettingsWorkspaceScopeKind `json:"scope"`
	WriteTarget     SettingsWriteTargetKind    `json:"write_target,omitempty"`
	WorkspaceID     string                     `json:"workspace_id,omitempty"`
	Behavior        SettingsMutationBehavior   `json:"behavior"`
	Applied         bool                       `json:"applied"`
	RestartRequired bool                       `json:"restart_required"`
	RestartScope    string                     `json:"restart_scope,omitempty"`
	Warnings        []string                   `json:"warnings,omitempty"`
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

type SettingsUserCollectionMutationResult struct {
	Section         SettingsCollectionName   `json:"section"`
	Scope           SettingsUserScopeKind    `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type SettingsLayeredCollectionMutationResult struct {
	Section         SettingsCollectionName   `json:"section"`
	Scope           SettingsLayeredScopeKind `json:"scope"`
	WriteTarget     SettingsWriteTargetKind  `json:"write_target,omitempty"`
	WorkspaceID     string                   `json:"workspace_id,omitempty"`
	Profile         string                   `json:"profile,omitempty"`
	Behavior        SettingsMutationBehavior `json:"behavior"`
	Applied         bool                     `json:"applied"`
	RestartRequired bool                     `json:"restart_required"`
	RestartScope    string                   `json:"restart_scope,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
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
	Profile          string                        `json:"profile,omitempty"`
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
	WriteTarget       SettingsWriteTargetKind `json:"write_target,omitempty"`
	WritePath         string                  `json:"write_path,omitempty"`
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

type SettingsUpdateApplyRequest struct {
	Target SettingsUpdateTarget `json:"target"`
}

type SettingsUpdateHolderPayload struct {
	PID                int                 `json:"pid"`
	PIDStartTime       time.Time           `json:"pid_start_time"`
	Surface            SettingsUpdateActor `json:"surface"`
	ExecutorGeneration string              `json:"executor_generation"`
	LeaseExpiresAt     time.Time           `json:"lease_expires_at"`
}

type SettingsUpdateOperationPayload struct {
	ID           string                       `json:"id"`
	Revision     int64                        `json:"revision"`
	Targets      []SettingsUpdateTarget       `json:"targets"`
	ActiveTarget *SettingsUpdateTarget        `json:"active_target,omitempty"`
	Phase        SettingsUpdatePhase          `json:"phase"`
	Percent      int                          `json:"percent"`
	Holder       *SettingsUpdateHolderPayload `json:"holder"`
	Waiting      SettingsUpdateWaitingState   `json:"waiting"`
	StartedAt    time.Time                    `json:"started_at"`
	LastError    string                       `json:"last_error"`
}

type SettingsUpdateRuntimePayload struct {
	Status          SettingsUpdateStatusKind    `json:"status"`
	InstallMethod   SettingsUpdateInstallMethod `json:"install_method"`
	Managed         bool                        `json:"managed"`
	CurrentVersion  string                      `json:"current_version"`
	LatestVersion   string                      `json:"latest_version"`
	ReleaseURL      string                      `json:"release_url"`
	Recommendation  string                      `json:"recommendation"`
	RestoredVersion string                      `json:"restored_version"`
	DaemonRestarted bool                        `json:"daemon_restarted"`
	Message         string                      `json:"message"`
	LastError       string                      `json:"last_error"`
}

type SettingsUpdateAppPayload struct {
	Status         SettingsUpdateStatusKind `json:"status"`
	Running        bool                     `json:"running"`
	CurrentVersion string                   `json:"current_version"`
	LatestVersion  string                   `json:"latest_version"`
	ReleaseURL     string                   `json:"release_url"`
	AttemptID      string                   `json:"attempt_id"`
	LastError      string                   `json:"last_error"`
	Message        string                   `json:"message"`
}

type SettingsUpdateResponse struct {
	Aggregate SettingsUpdateStatusKind        `json:"aggregate"`
	Operation *SettingsUpdateOperationPayload `json:"operation"`
	Runtime   SettingsUpdateRuntimePayload    `json:"runtime"`
	App       *SettingsUpdateAppPayload       `json:"app"`
}

type SettingsUpdateApplyResponse struct {
	Target      SettingsUpdateTarget         `json:"target"`
	Status      SettingsUpdateApplyStatus    `json:"status"`
	OperationID string                       `json:"operation_id,omitempty"`
	Message     string                       `json:"message"`
	Holder      *SettingsUpdateHolderPayload `json:"holder,omitempty"`
}

type SettingsUpdateCancelResponse struct {
	Status      SettingsUpdateStatusKind     `json:"status"`
	OperationID string                       `json:"operation_id,omitempty"`
	Message     string                       `json:"message"`
	Holder      *SettingsUpdateHolderPayload `json:"holder,omitempty"`
}
