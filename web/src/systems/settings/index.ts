// Types
export type * from "./public-types";

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
  settingsSectionSlugFromPathname,
} from "./lib/sections";
export {
  DEFAULT_SETTINGS_SECTION_SLUG,
  retiredSettingsSectionReplacement,
} from "./lib/section-paths";
export {
  readTerminalSettings,
  terminalSettingsInvalidKeyMessage,
} from "./lib/terminal-settings-projection";
export type { TerminalSettingsProjection } from "./lib/terminal-settings-projection";
export type { TerminalSettingsKey } from "./lib/terminal-settings-types";

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
  deleteSettingsHook,
  deleteSettingsMCPServer,
  deleteSettingsProvider,
  getSettingsAutomation,
  getSettingsMCPServer,
  getSettingsCmdPalette,
  getSettingsGeneral,
  getSettingsPersona,
  getSettingsHooksExtensions,
  getRolesStatus,
  getSettingsRoles,
  getSettingsAttention,
  getSettingsShell,
  getSettingsObservability,
  getSettingsProvider,
  getSettingsRestartStatus,
  getSettingsSkills,
  getSettingsUpdate,
  applySettingsUpdate,
  cancelSettingsUpdate,
  listSettingsApplyRecords,
  listSettingsHooks,
  listSettingsMCPServers,
  listSettingsProviders,
  OBSERVABILITY_LOG_TAIL_PATH,
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
  updateSettingsPersona,
  updateSettingsHooksExtensions,
  updateSettingsAttention,
  updateSettingsShell,
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
  settingsGeneralOptions,
  settingsAttentionFilterForProfile,
  settingsPersonaFilterForProfile,
  settingsPersonaOptions,
  settingsHooksExtensionsOptions,
  settingsHooksListOptions,
  settingsMCPServersListOptions,
  settingsMCPServerDetailOptions,
  settingsAttentionOptions,
  settingsShellOptions,
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
  MCPOverrideEditor,
  MCPServersTable,
  MCPServersTableSkeleton,
  mcpServerRowTestId,
  isExtensionOwnedMCPServer,
  mcpAllocatedRuntimeName,
  mcpOwnerExtensionName,
  mcpServerProvenanceLine,
  ProviderCard,
  ProviderCardGrid,
  ProviderEditForm,
  ProviderDetailDialog,
  ProviderInspectView,
  ProviderLogo,
  ProviderModelCatalogStatus,
  ProviderUsageCollectionSection,
  ProviderUsageKeychainSection,
  ProviderUsageProvidersSection,
  TerminalSettingsSections,
  type TerminalSettingsConfig,
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
  SettingsSkillCustomSources,
  SettingsSkillSourceDiagnostics,
  SettingsSkillSourceRow,
  SettingsSkillSourcesSection,
  SettingsSkillsScopeNotice,
  SettingsSkillsEngineSection,
  SettingsSkillsInstallPolicySection,
  SettingsSkillsDiscoverySection,
  SettingsSkillsScopeSelector,
  SettingsEditorDialog,
  ModalSettingsFieldRow,
  SettingsFieldRow,
  SettingsGroup,
  SettingsNumberInput,
  SettingsPageFrame,
  SettingsPageState,
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
  MarketplaceSourceRow,
  SettingsMarketplaceCatalogSection,
  SettingsMarketplaceSourcesSection,
  marketplaceCatalogDraftInvalid,
  sameMarketplaceCatalogConfig,
  WindowManagerConfigEditor,
} from "./components";
export type {
  MarketplaceSourceRowProps,
  SettingsMarketplaceCatalogConfig,
  SettingsMarketplaceCatalogSectionProps,
  SettingsMarketplaceSourcesSectionProps,
} from "./components";
export type { MCPOverrideEditorProps, MCPServerEditorProps } from "./components";
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
  useSettingsPersona,
  useSettingsHooksExtensions,
  useSettingsAttention,
  useSettingsShell,
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
  useSettingsHooks,
  useSettingsMCPServers,
  useSettingsMCPServer,
  useSettingsProvider,
  useSettingsProviders,
} from "./hooks/use-settings-collections";

// Hooks -- mutations
export {
  useBeginMCPAuth,
  useDeleteSettingsHook,
  useDeleteSettingsMCPServer,
  useExchangeMCPAuth,
  useLogoutMCPAuth,
  useDeleteSettingsProvider,
  usePutSettingsHook,
  usePutSettingsMCPServer,
  usePutSettingsProvider,
  useReloadSettings,
  useUpdateSettingsAutomation,
  useUpdateSettingsGeneral,
  useUpdateSettingsHooksExtensions,
  useUpdateSettingsAttention,
  useUpdateSettingsShell,
  useUpdateSettingsObservability,
  useUpdateSettingsPersona,
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
export { useSettingsPersonaPage } from "./hooks/use-settings-persona-page";
export {
  useSettingsRuntimeCatalog,
  type SettingsRuntimeCatalog,
} from "./hooks/use-settings-runtime-catalog";
export { useSettingsSessionDefaults } from "./hooks/use-settings-session-defaults";
export {
  combineSettingsSaveSources,
  personaConfigWithRuntime,
  sessionDefaultsRuntimeValue,
} from "./lib/session-defaults";
export { useSettingsSaveBarState } from "./hooks/use-settings-save-bar-state";
export { useSettingsTopbar, type UseSettingsTopbarOptions } from "./hooks/use-settings-topbar";
export type { SettingsSaveBarState } from "./lib/save-state";
export { parsePositiveDurationMilliseconds } from "./lib/terminal-settings-duration";
export {
  useProductUpdate,
  useReloadOnProductVersionChange,
  type ProductUpdateModel,
} from "./hooks/use-product-update";
export {
  settingsProductUpdate,
  settingsUpdateHolderLabel,
  settingsUpdateIndicatorState,
  type ProductUpdateIndicatorState,
  type SettingsProductUpdateView,
} from "./lib/update-product";
export {
  settingsUpdateApplicableTargets,
  settingsUpdateStatusLabel,
  settingsUpdateStatusTone,
  settingsUpdateTracks,
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

export { useMCPEditor } from "./hooks/use-mcp-editor";
export { useMCPOverrideEditor } from "./hooks/use-mcp-override-editor";
export { useSettingsMCPPage } from "./hooks/use-settings-mcp-page";
export { mcpDefinitionKey } from "./lib/mcp-management-target";
export type {
  MCPOverrideDraft,
  MCPOverridePair,
  MCPOverrideErrors,
} from "./lib/mcp-override-model";

export { useMCPDefinitionAuthorization } from "./hooks/use-mcp-definition-authorization";

export {
  getSettingsMarketplace,
  updateSettingsMarketplace,
} from "./adapters/settings-marketplace-api";
export {
  useSettingsMarketplace,
  useUpdateSettingsMarketplace,
} from "./hooks/use-settings-marketplace";
export { settingsMarketplaceOptions } from "./lib/query-options";
export { settingsProviderUsageOptions } from "./lib/query-options";
export { useSettingsProviderUsagePage } from "./hooks/use-settings-provider-usage-page";
export {
  useAllowProviderUsageKeychain,
  useSettingsProviderUsage,
  useUpdateSettingsProviderUsage,
} from "./hooks/use-settings-provider-usage";
export {
  PROVIDER_USAGE_COLLECTOR_IDS,
  PROVIDER_USAGE_INTERVAL_OPTIONS,
  providerUsageIntervalValue,
  providerUsageKeychainAccess,
  providerUsageProviderEnabled,
  sameProviderUsageConfig,
  withProviderUsageKeychainAccess,
  withProviderUsageProviderEnabled,
  type ProviderUsageSettingsConfig,
} from "./lib/provider-usage-settings";

export type { SettingsMarketplaceSection, SettingsUpdateMarketplaceRequest } from "./types";
