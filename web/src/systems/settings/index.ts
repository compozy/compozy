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
  SettingsCreateNotificationPresetRequest,
  SettingsNotificationPresetCollection,
  SettingsNotificationPresetEntry,
  SettingsNotificationPresetFilter,
  SettingsNotificationPresetTarget,
  SettingsUpdateAutomationRequest,
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
  MCPOAuthDiscovery,
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
  getSettingsGeneral,
  getSettingsHooksExtensions,
  getSettingsMemory,
  getRolesStatus,
  getSettingsRoles,
  getSettingsNetwork,
  getSettingsObservability,
  getSettingsProvider,
  getSettingsRestartStatus,
  getSettingsSkills,
  getSettingsUpdate,
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
  updateSettingsGeneral,
  updateSettingsHooksExtensions,
  updateSettingsMemory,
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
export { useSettingsRestartStore } from "./stores/use-settings-restart-store";
export type {
  PendingSettingsMutation,
  SettingsRestartActions,
  SettingsRestartState,
  SettingsRestartStore,
} from "./stores/settings-restart-store";

// Hooks -- reads
export {
  useSettingsAutomation,
  useSettingsApplyRecords,
  useSettingsGeneral,
  useSettingsHooksExtensions,
  useSettingsMemory,
  useSettingsNetwork,
  useSettingsObservability,
  useRolesStatus,
  useSettingsRoles,
  useSettingsSkills,
  useSettingsUpdate,
} from "./hooks/use-settings-sections";
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
  useUpdateSettingsNetwork,
  useUpdateSettingsObservability,
  useUpdateSettingsRoles,
  useUpdateSettingsSkills,
} from "./hooks/use-settings-mutations";

// Hooks -- restart
export { useSettingsRestart } from "./hooks/use-settings-restart";
export { useSettingsRolesPage } from "./hooks/use-settings-roles-page";
export { useSettingsSaveBarState } from "./hooks/use-settings-save-bar-state";
export { useSettingsTopbar, type UseSettingsTopbarOptions } from "./hooks/use-settings-topbar";
export type { SettingsSaveBarState } from "./lib/save-state";
export { buildRolesViewModel, type RoleViewModel } from "./lib/roles-view-model";
export { ROLE_ORDER, type RoleRuntimeValue } from "./lib/roles-config";
export type { RolesDisclosure } from "./hooks/use-roles-disclosure";
export type { RolesRuntimeOptions } from "./hooks/use-roles-runtime-options";
