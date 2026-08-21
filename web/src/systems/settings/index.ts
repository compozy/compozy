// Types
export type {
  ConfigApplyLifecycle,
  ConfigApplyRecord,
  ConfigApplyRecordsResponse,
  ConfigApplyRecordStatus,
  SettingsApplyNextAction,
  SettingsApplyRecordsFilter,
  SettingsApplyResponse,
  SettingsAutomationSection,
  SettingsAttentionSection,
  SettingsShellSection,
  SettingsCollectionName,
  SettingsSandboxCollection,
  SettingsSandboxDetail,
  SettingsSandboxEntry,
  SettingsSandboxRequest,
  SettingsGeneralSection,
  SettingsHookCollection,
  SettingsHookEntry,
  SettingsHookRequest,
  SettingsHooksExtensionsHook,
  SettingsHooksExtensionsInstalled,
  SettingsHooksExtensionsSection,
  SettingsWindowManagerSection,
  SettingsCmdPaletteFilter,
  SettingsCmdPaletteSection,
  SettingsUpdateCmdPaletteFilter,
  SettingsUpdateCmdPaletteRequest,
  SettingsMCPAuthBeginResponse,
  SettingsMCPAuthBeginRequest,
  SettingsMCPAuthBeginMode,
  SettingsMCPAuthExchangeRequest,
  SettingsMCPAuthFilter,
  SettingsMCPAuthStatusResponse,
  SettingsMCPServerCollection,
  SettingsMCPServerDeleteFilter,
  SettingsMCPServerEntry,
  SettingsMCPServerListFilter,
  SettingsMCPServerPutFilter,
  SettingsMCPServerRequest,
  SettingsMCPServerTarget,
  SettingsMemorySection,
  RoleName,
  RoleDiagnostic,
  RoleFallbackEntry,
  RoleFallbackStatus,
  RoleResolutionMode,
  RoleStatus,
  RolesStatusResponse,
  SettingsRolesConfig,
  SettingsRolesSection,
  SettingsMutationResult,
  SettingsNetworkSection,
  SettingsObservabilitySection,
  ProviderAuthMode,
  ProviderCredentialSlotDraft,
  ProviderDraft,
  SettingsProviderCollection,
  SettingsProviderCredentialSlotRequest,
  SettingsProviderDetail,
  SettingsProviderEntry,
  SettingsProviderModelRequest,
  SettingsProviderModelsRequest,
  SettingsProviderRequest,
  SettingsRestartResponse,
  SettingsRestartStatus,
  SettingsRestartStatusName,
  SettingsScope,
  SettingsSectionDescriptor,
  SettingsSectionName,
  SettingsSectionSlug,
  SettingsSkillsFilter,
  SettingsSkillsSection,
  SettingsSource,
  SettingsSourceKind,
  SettingsUpdateStatus,
  SettingsUpdateAppTrack,
  SettingsUpdateApplyRequest,
  SettingsUpdateApplyResult,
  SettingsUpdateCancelResult,
  SettingsUpdateHolder,
  SettingsUpdateOperation,
  SettingsUpdateRuntimeTrack,
  SettingsUpdateStatusKind,
  SettingsUpdateTarget,
  SettingsCreateNotificationPresetRequest,
  SettingsNotificationPresetCollection,
  SettingsNotificationPresetEntry,
  SettingsNotificationPresetFilter,
  SettingsNotificationPresetTarget,
  SettingsUpdateAutomationRequest,
  SettingsUpdateAttentionRequest,
  SettingsUpdateShellRequest,
  SettingsUpdateGeneralRequest,
  SettingsUpdateHooksExtensionsRequest,
  SettingsUpdateWindowManagerRequest,
  SettingsUpdateNotificationPresetRequest,
  SettingsUpdateMemoryRequest,
  SettingsUpdateRolesRequest,
  SettingsUpdateNetworkRequest,
  SettingsUpdateObservabilityRequest,
  SettingsUpdateSkillsRequest,
  SettingsUpdateSkillsFilter,
  SettingsWriteTarget,
} from "./types";

export {
  isMCPAuthorizeAwaiting,
  isMCPAuthorizePending,
  useMCPAuthorize,
  type MCPAuthorizePhase,
  type MCPAuthorizePriorStatus,
  type MCPAuthorizeState,
  type UseMCPAuthorizeReturn,
} from "./hooks/use-mcp-authorize";

// Section metadata
export {
  findSettingsSection,
  SETTINGS_ROOT_PATH,
  SETTINGS_SECTION_GROUPS,
  SETTINGS_SECTIONS,
  filterSettingsSections,
  SETTINGS_SECTION_SLUGS,
  settingsSectionPath,
} from "./lib/sections";
export { DEFAULT_SETTINGS_SECTION_SLUG } from "./lib/section-paths";

// MCP page models (status matrix + editor draft)
export {
  authorizeLabel,
  composeMCPRowStatus,
  deriveMCPAuthFilter,
  formatStatusLabel,
  isOAuthCapable,
  isOAuthRepairable,
  MCP_AUTH_STATUSES,
  MCP_PROBE_STATES,
  MCP_RUNTIME_STATES,
  probeToolLabel,
} from "./lib/mcp-status-view-model";
export { deriveMCPManagementFilter, mcpManagementScopeLabel } from "./lib/mcp-management-target";
export type { MCPManagementFilter } from "./lib/mcp-management-target";
export type {
  MCPAuthorizeLabel,
  MCPAuthStatus,
  MCPProbeState,
  MCPRowStatus,
  MCPRuntimeState,
  MCPStatusCell,
} from "./lib/mcp-status-view-model";
export {
  emptyDraft,
  toDraft,
  toRequest,
  validateDraft,
  withoutMCPSecretPreservation,
} from "./lib/mcp-editor-model";
export type {
  MCPDraft,
  MCPDraftErrors,
  MCPDraftValidation,
  MCPEnvPair,
  MCPOAuthDraft,
  MCPSecretBinding,
  MCPSecretEnvEntry,
  MCPSecretMode,
  MCPTransport,
} from "./lib/mcp-editor-model";

// Adapters
export {
  deleteSettingsSandbox,
  deleteSettingsHook,
  deleteSettingsMCPServer,
  deleteSettingsProvider,
  getSettingsAutomation,
  getSettingsSandbox,
  getSettingsCmdPalette,
  getSettingsGeneral,
  getSettingsHooksExtensions,
  getSettingsMemory,
  getRolesStatus,
  getSettingsRoles,
  getSettingsAttention,
  getSettingsShell,
  getSettingsNetwork,
  getSettingsObservability,
  getSettingsProvider,
  getSettingsRestartStatus,
  getSettingsSkills,
  getSettingsUpdate,
  applySettingsUpdate,
  cancelSettingsUpdate,
  listSettingsApplyRecords,
  listSettingsSandboxes,
  listSettingsHooks,
  listSettingsMCPServers,
  listSettingsProviders,
  OBSERVABILITY_LOG_TAIL_PATH,
  listSettingsNotificationPresets,
  createSettingsNotificationPreset,
  updateSettingsNotificationPreset,
  deleteSettingsNotificationPreset,
  putSettingsSandbox,
  putSettingsHook,
  putSettingsMCPServer,
  putSettingsProvider,
  reloadSettings,
  SettingsApiError,
  settingsObservabilityLogTailPath,
  triggerSettingsRestart,
  updateSettingsAutomation,
  updateSettingsCmdPalette,
  updateSettingsGeneral,
  updateSettingsHooksExtensions,
  updateSettingsMemory,
  updateSettingsAttention,
  updateSettingsShell,
  updateSettingsNetwork,
  updateSettingsObservability,
  updateSettingsRoles,
  updateSettingsSkills,
} from "./adapters/settings-api";
export {
  beginSettingsMCPAuth,
  exchangeSettingsMCPAuth,
  logoutSettingsMCPAuth,
} from "./adapters/settings-mcp-auth-api";
export {
  applyWindowManagerLayout,
  deleteWindowManagerLayoutProfile,
  exportWindowManagerLayout,
  getWindowManagerLayoutState,
  listWindowManagerLayoutProfiles,
  previewWindowManagerLayout,
  putWindowManagerLayoutProfile,
  updateWindowManagerSettings,
  validateWindowManagerLayout,
  WindowManagerLayoutsApiError,
} from "./adapters/window-manager-layouts-api";

// Query infrastructure
export { settingsKeys } from "./lib/query-keys";
export type {
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutState,
} from "./lib/window-manager-layout-types";
export {
  windowManagerLayoutOptions,
  windowManagerLayoutProfilesOptions,
} from "./lib/window-manager-layout-query";
export {
  useWindowManagerConfigEditor,
  type WindowManagerConfigEditorModel,
} from "./hooks/use-window-manager-config-editor";

// The keyboard surface: one write path, the daemon arbitrating every claim.
export { useSettingsPalettePage } from "./hooks/use-settings-palette-page";
export {
  useLayoutsSettingsData,
  type LayoutsSettingsData,
} from "./hooks/use-layouts-settings-data";
export { settingsCmdPaletteOptions } from "./lib/query-options";
export {
  useWindowManagerBindingMutations,
  type WindowManagerBindingCommit,
  type WindowManagerBindingMutations,
} from "./hooks/use-window-manager-binding-mutations";
export {
  useWindowManagerShortcutRecorder,
  type ShortcutRecorderConflict,
  type ShortcutRecorderModel,
} from "./hooks/use-window-manager-shortcut-recorder";
export {
  useGlobalShortcutRecorder,
  type GlobalShortcutRecorderModel,
} from "./hooks/use-global-shortcut-recorder";
export { useWindowManagerKeyboardEditors } from "./hooks/use-window-manager-keyboard-editors";
export {
  ALIAS_RULE_HINT,
  useWindowManagerAliasEditor,
  type AliasCellState,
  type AliasConflict,
  type AliasEditorModel,
} from "./hooks/use-window-manager-alias-editor";
export {
  buildShortcutTableRows,
  isCommandOverridden,
  shortcutSourceCounts,
  withCommandReset,
  type ShortcutTableRow,
} from "./lib/window-manager-shortcut-rows";
export {
  useWindowManagerLayoutEditor,
  type WindowManagerLayoutEditorModel,
} from "./hooks/use-window-manager-layout-editor";
export {
  useWindowManagerLayoutProfiles,
  type WindowManagerLayoutProfilesModel,
} from "./hooks/use-window-manager-layout-profiles";
export {
  SETTINGS_QUERY_INTERVALS,
  settingsAutomationOptions,
  settingsApplyRecordsOptions,
  settingsSandboxDetailOptions,
  settingsSandboxesListOptions,
  settingsGeneralOptions,
  settingsHooksExtensionsOptions,
  settingsHooksListOptions,
  settingsNotificationPresetsOptions,
  settingsMCPServersListOptions,
  settingsMemoryOptions,
  settingsAttentionOptions,
  settingsShellOptions,
  settingsNetworkOptions,
  settingsObservabilityOptions,
  settingsRolesOptions,
  settingsRolesStatusOptions,
  settingsProviderDetailOptions,
  settingsProvidersListOptions,
  settingsRestartStatusOptions,
  settingsSkillsOptions,
  settingsUpdateOptions,
} from "./lib/query-options";
export {
  isFailedRestart,
  isSuccessfulRestart,
  isTerminalRestartStatus,
  RESTART_TERMINAL_STATUSES,
} from "./lib/restart-status";
export {
  settingsRestartPresentation,
  type SettingsRestartPresentation,
  type SettingsRestartViewState,
} from "./lib/restart-presentation";
// Components
export {
  MCPAuthorizeDialog,
  AttentionSystemStateChip,
  attentionSystemStateNote,
  MCPSelectionStrip,
  MCPServerDeleteDialog,
  MCPServerEditor,
  MCPServersTable,
  ProviderCard,
  ProviderEditForm,
  ProviderDetailDialog,
  ProviderInspectView,
  ProviderLogo,
  ProviderModelCatalogStatus,
  NetworkSettingsSections,
  ProviderRow,
  ProvidersToolbar,
  RoleList,
  ModalSettingRow,
  SettingActionRow,
  SettingLinkRow,
  SettingRow,
  SettingValue,
  SettingsAdvancedFold,
  SettingsHeroBoard,
  SettingsHeroGauge,
  SettingsTaglistField,
  SettingsByteField,
  SettingsLiveChip,
  SettingsApplyRecordsPanel,
  SettingsChoiceGroup,
  SettingsDecimalInput,
  SettingsDisabledSkillsSection,
  SettingsEditorDialog,
  ModalSettingsFieldRow,
  SettingsFieldRow,
  SettingsGroup,
  SettingsUpdateTrackRow,
  SettingsNumberInput,
  SettingsPageFrame,
  SettingsProvChip,
  SettingsRestartNotice,
  SettingsRuntimeUnavailable,
  SettingsInlineSaveControls,
  SettingsSaveBar,
  SettingsSourceBadge,
  SettingsTile,
  SettingsTiles,
  LayoutProfileGrid,
  LayoutStage,
  WindowManagerConfigEditor,
} from "./components";
export type { MCPServerEditorProps, ProvidersViewMode } from "./components";
export { deriveProviderStateLabel, getProviderStateView } from "./lib/provider-state";
export type { ProviderStateLabel, ProviderStateView } from "./lib/provider-state";
export { settingsProviderToOption } from "./lib/provider-runtime-option";

// Stores
export {
  resetSettingsRestartStore,
  useSettingsRestartState,
} from "./stores/use-settings-restart-store";
export { settingsRestartStore } from "./stores/settings-restart-store";
export type {
  PendingSettingsMutation,
  SettingsRestartState,
} from "./stores/settings-restart-store";

// Hooks -- reads
export {
  useSettingsAutomation,
  useSettingsApplyRecords,
  useSettingsGeneral,
  useSettingsHooksExtensions,
  useSettingsMemory,
  useSettingsAttention,
  useSettingsShell,
  useSettingsNetwork,
  useSettingsObservability,
  useRolesStatus,
  useSettingsRoles,
  useSettingsSkills,
  useSettingsUpdate,
} from "./hooks/use-settings-sections";
export {
  useSettingsAttentionPage,
  type SettingsAttentionPageModel,
} from "./hooks/use-settings-attention-page";
export {
  useSettingsSandbox,
  useSettingsSandboxes,
  useSettingsNotificationPresets,
  useSettingsHooks,
  useSettingsMCPServers,
  useSettingsProvider,
  useSettingsProviders,
} from "./hooks/use-settings-collections";

// Hooks -- mutations
export {
  useBeginMCPAuth,
  useDeleteSettingsSandbox,
  useDeleteSettingsHook,
  useDeleteSettingsMCPServer,
  useExchangeMCPAuth,
  useLogoutMCPAuth,
  useDeleteSettingsProvider,
  useCreateSettingsNotificationPreset,
  useUpdateSettingsNotificationPreset,
  useDeleteSettingsNotificationPreset,
  usePutSettingsSandbox,
  usePutSettingsHook,
  usePutSettingsMCPServer,
  usePutSettingsProvider,
  useReloadSettings,
  useUpdateSettingsAutomation,
  useUpdateSettingsGeneral,
  useUpdateSettingsHooksExtensions,
  useUpdateSettingsMemory,
  useUpdateSettingsAttention,
  useUpdateSettingsShell,
  useUpdateSettingsNetwork,
  useUpdateSettingsObservability,
  useUpdateSettingsRoles,
  useUpdateSettingsSkills,
} from "./hooks/use-settings-mutations";
export {
  useApplySettingsUpdate,
  useCancelSettingsUpdate,
} from "./hooks/use-settings-update-mutations";

// Hooks -- restart
export { useSettingsPage } from "./hooks/use-settings-page";
export { useSettingsRestart } from "./hooks/use-settings-restart";
export { useSettingsRolesPage } from "./hooks/use-settings-roles-page";
export { useSettingsSaveBarState } from "./hooks/use-settings-save-bar-state";
export { useSettingsTopbar, type UseSettingsTopbarOptions } from "./hooks/use-settings-topbar";
export type { SettingsSaveBarState } from "./lib/save-state";
export {
  settingsUpdateIndicatorAvailable,
  settingsUpdateStatusLabel,
  settingsUpdateStatusTone,
  settingsUpdateTracks,
  settingsUpdateVersionTransition,
  settingsUpdateView,
  type SettingsUpdateProgress,
  type SettingsUpdateTrackView,
  type SettingsUpdateView,
} from "./lib/update-presentation";
export {
  UPDATE_UI_PHASES,
  updatePhasePercent,
  updateUiPhase,
  type UpdateUiPhase,
} from "./lib/update-phase-map";
export { buildRolesViewModel, type RoleViewModel } from "./lib/roles-view-model";
export { ROLE_ORDER, type RoleRuntimeValue } from "./lib/roles-config";
export type { RolesDisclosure } from "./hooks/use-roles-disclosure";
export type { RolesRuntimeOptions } from "./hooks/use-roles-runtime-options";
