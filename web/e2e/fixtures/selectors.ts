import type { Locator, Page } from "@playwright/test";

// Shell-global surfaces: the desktop root and the first-run workspace onboarding.
// These legitimately live at page scope (they exist before any window opens).
export const sessionLifecycleTestIds = {
  osDesktop: "os-desktop",
  workspaceManualPathInput: "workspace-manual-path-input",
  workspaceOnboarding: "workspace-onboarding",
  workspaceRegisterManual: "workspace-register-manual",
  workspaceUseGlobal: "workspace-use-global",
} as const;

export const SESSION_CREATE_FIRST_MESSAGE = "e2e first message";

// Session-window surfaces. Every one of these renders inside an owning
// `os-window-session:<sessionId>` window and must be scoped to it — a page-level
// match would resolve to a second session window (strict-mode violation).
export const sessionWindowTestIds = {
  chatView: "chat-view",
  composerClearButton: "composer-clear-button",
  composerSendButton: "composer-send-button",
  composerTextarea: "composer-textarea",
  deleteButton: "delete-button",
  permissionAllowAlways: "permission-allow-always",
  permissionAllowOnce: "permission-allow-once",
  permissionPrompt: "permission-prompt",
  processingIndicator: "processing-indicator",
  resumeButton: "resume-button",
  stopButton: "stop-button",
  topbarOverflow: "session-topbar-overflow",
} as const;

export interface SessionLifecycleSelectors {
  agentPageNewSession: Locator;
  agentRow(agentName: string): Locator;
  osDesktop: Locator;
  workspaceManualPathInput: Locator;
  workspaceOnboarding: Locator;
  workspaceRegisterManual: Locator;
  workspaceUseGlobal: Locator;
}

export interface SessionWindowSelectors {
  chatView: Locator;
  composerClearButton: Locator;
  composerSendButton: Locator;
  composerTextarea: Locator;
  deleteButton: Locator;
  permissionAllowAlways: Locator;
  permissionAllowOnce: Locator;
  permissionPrompt: Locator;
  processingIndicator: Locator;
  resumeButton: Locator;
  stopButton: Locator;
  topbarOverflow: Locator;
}

export const networkOperatorTestIds = {
  osDesktop: sessionLifecycleTestIds.osDesktop,
  channelNameInput: "network-channel-name-input",
  channelHeader: "network-channel-header",
  channelIdentityMix: "network-channel-identity-mix",
  channelInspectorToggle: "network-channel-inspector-toggle",
  channelTabs: "network-channel-tabs",
  createDialog: "network-create-channel-dialog",
  createSubmit: "network-create-channel-submit",
  createAgentTrigger: "network-create-channel-agent-trigger",
  channelPurposeInput: "network-channel-purpose-input",
  disabledState: "network-disabled-state",
  directList: "network-direct-list",
  directRoom: "network-direct-room",
  directsTab: "network-directs-tab",
  directTab: "network-tab-directs",
  activityFeed: "network-activity-feed",
  inspectorToggle: "network-channel-inspector-toggle",
  messageList: "network-timeline",
  inspector: "network-inspector",
  inspectorMembersTab: "network-inspector-tab-members",
  inspectorPanelMembers: "network-inspector-panel-members",
  inspectorPanelWork: "network-inspector-panel-work",
  inspectorWorkTab: "network-inspector-tab-work",
  noChannelsState: "network-no-channels-state",
  newDirectButton: "network-directs-new-direct",
  newDirectDialog: "network-new-direct-dialog",
  openCreateDialog: "network-open-create-dialog",
  workInspector: "network-work-inspector",
  threadList: "network-thread-list",
  threadOverlay: "network-thread-overlay",
  threadsTab: "network-threads-tab",
  threadTab: "network-tab-threads",
  workspace: "network-shell",
  workspaceOnboarding: sessionLifecycleTestIds.workspaceOnboarding,
  workspaceUseGlobal: sessionLifecycleTestIds.workspaceUseGlobal,
} as const;

export const automationOperatorTestIds = {
  osDesktop: sessionLifecycleTestIds.osDesktop,
  automationDetailPanel: "automation-detail-panel",
  automationEditorDialog: "automation-editor-dialog",
  automationJobScheduler: "automation-job-scheduler",
  automationJobForm: "automation-job-form",
  automationRunHistory: "automation-run-history",
  automationSuggestionsCard: "automation-suggestions-card",
  automationDeleteDialog: "automation-delete-dialog",
  automationDeleteConfirmTyping: "automation-delete-confirm-typing",
  confirmDeleteAutomationButton: "confirm-delete-automation-btn",
  createJobButton: "create-job-btn",
  createTriggerButton: "create-trigger-btn",
  deleteAutomationButton: "delete-automation-btn",
  detailOverflow: "automation-detail-overflow",
  editAutomationButton: "edit-automation-btn",
  jobAgentInput: "job-agent-input",
  jobEnabledToggle: "job-enabled-toggle",
  jobFireLimitMax: "job-fire-limit-max",
  jobFireLimitWindow: "job-fire-limit-window",
  jobGovernanceToggle: "job-governance-toggle",
  jobsListRows: "jobs-list-rows",
  jobsShell: "jobs-shell",
  jobNameInput: "job-name-input",
  jobPromptInput: "job-prompt-input",
  jobScheduleInterval: "job-schedule-interval",
  jobScheduleModeAt: "job-schedule-mode-at",
  jobScheduleModeCron: "job-schedule-mode-cron",
  jobScheduleModeEvery: "job-schedule-mode-every",
  jobScheduleTime: "job-schedule-time",
  jobScopeGlobal: "job-scope-global",
  jobScopeWorkspace: "job-scope-workspace",
  submitJobForm: "submit-job-form",
  submitTriggerForm: "submit-trigger-form",
  triggerAgentInput: "trigger-agent-input",
  triggerEnabledToggle: "trigger-enabled-toggle",
  triggerEndpointSlugInput: "trigger-endpoint-slug-input",
  triggerFireLimitMax: "trigger-fire-limit-max",
  triggerFireLimitWindow: "trigger-fire-limit-window",
  triggersListRows: "triggers-list-rows",
  triggersShell: "triggers-shell",
  triggerPromptInput: "trigger-prompt-input",
  triggerRetryMax: "trigger-retry-max",
  triggerRetryStrategyBackoff: "trigger-retry-strategy-backoff",
  triggerRetryStrategyNone: "trigger-retry-strategy-none",
  triggerScopeGlobal: "trigger-scope-global",
  triggerWebhookIDInput: "trigger-webhook-id-input",
  triggerWebhookSecretValueInput: "trigger-webhook-secret-value-input",
  toggleAutomationButton: "toggle-automation-btn",
  triggerJobButton: "trigger-job-btn",
  triggerNameInput: "trigger-name-input",
  workspaceOnboarding: sessionLifecycleTestIds.workspaceOnboarding,
  workspaceUseGlobal: sessionLifecycleTestIds.workspaceUseGlobal,
} as const;

export const bridgeOperatorTestIds = {
  osDesktop: sessionLifecycleTestIds.osDesktop,
  bridgeCreateDialog: "bridge-create-dialog",
  bridgeDetailPanel: "bridge-detail-panel",
  bridgeEditDialog: "bridge-edit-dialog",
  bridgeListFiltersAdd: "bridge-list-filters-add",
  bridgeListPanel: "bridge-list-panel",
  bridgeManifestHandoff: "bridge-manifest-handoff",
  bridgeManifestJson: "bridge-manifest-json",
  bridgeManifestOpenBridge: "bridge-manifest-open-bridge",
  bridgeMetricActiveRoutes: "bridge-metric-active-routes",
  bridgeRestartRequired: "bridge-restart-required",
  bridgeDeliveryTestPanel: "bridge-delivery-test-panel",
  bridgeSendTestResult: "bridge-send-test-result",
  bridgeSetupChecklist: "bridge-setup-checklist",
  bridgeSearchInput: "bridge-search-input",
  bridgeTestDeliveryResult: "bridge-test-delivery-result",
  createBridgeDeliveryModeSelect: "bridge-delivery-mode-select",
  createBridgeDeliveryPeerInput: "bridge-delivery-peer-input",
  createBridgeDeliveryThreadInput: "bridge-delivery-thread-input",
  createBridgeDisplayNameInput: "bridge-display-name-input",
  createBridgeProviderConfigInput: "bridge-provider-config-input",
  createBridgeProviderConfigError: "bridge-provider-config-error",
  createBridgeRoutingIncludePeer: "bridge-routing-include-peer",
  createBridgeRoutingIncludeThread: "bridge-routing-include-thread",
  createBridgeScopeSelect: "bridge-scope-select",
  createBridgeWizardNext: "bridge-wizard-next",
  createBridgeButton: "create-bridge-btn",
  detailOverflow: "bridge-detail-overflow",
  disableBridgeButton: "disable-bridge-btn",
  editBridgeButton: "edit-bridge-btn",
  enableBridgeButton: "enable-bridge-btn",
  bridgeEditModeAdvanced: "bridge-edit-mode-advanced",
  openTestDeliveryButton: "open-test-delivery-btn",
  restartBridgeButton: "restart-bridge-btn",
  submitBridgeEdit: "submit-bridge-edit",
  submitBridgeCreate: "submit-bridge-create",
  submitSendTest: "submit-send-test",
  submitTestDelivery: "submit-test-delivery",
  testDeliveryMessage: "test-delivery-message",
  testDeliveryModeSelect: "test-delivery-mode-select",
  testDeliveryPeerInput: "test-delivery-peer-input",
  testDeliveryThreadInput: "test-delivery-thread-input",
  verifyBridgeButton: "verify-bridge-btn",
  workspaceOnboarding: sessionLifecycleTestIds.workspaceOnboarding,
  workspaceUseGlobal: sessionLifecycleTestIds.workspaceUseGlobal,
} as const;

export const knowledgeOperatorTestIds = {
  osDesktop: sessionLifecycleTestIds.osDesktop,
  cancelCreateMemory: "cancel-create-memory-btn",
  confirmCreateMemory: "confirm-create-memory-btn",
  confirmDeleteMemory: "confirm-delete-memory-btn",
  confirmEditMemory: "confirm-edit-memory-btn",
  contentPreview: "content-preview",
  createButton: "create-memory-btn",
  createContent: "knowledge-create-content",
  createDescription: "knowledge-create-description",
  createDialog: "knowledge-create-dialog",
  createName: "knowledge-create-name",
  createType: "knowledge-create-type-grid",
  deleteButton: "delete-memory-btn",
  deleteDialog: "knowledge-delete-dialog",
  detailPanel: "knowledge-detail-panel",
  editButton: "edit-memory-btn",
  editContent: "knowledge-edit-content",
  editDescription: "knowledge-edit-description",
  editDialog: "knowledge-edit-dialog",
  guard: "knowledge-guard",
  listPanel: "knowledge-list-panel",
  searchInput: "knowledge-search-input",
  searchInfo: "knowledge-search-info",
  shell: "knowledge-shell",
  tabAgent: "tab-agent",
  tabGlobal: "tab-global",
  tabWorkspace: "tab-workspace",
  workspaceOnboarding: sessionLifecycleTestIds.workspaceOnboarding,
  workspaceUseGlobal: sessionLifecycleTestIds.workspaceUseGlobal,
} as const;

export const marketplaceOperatorTestIds = {
  bundleActivateConfirm: "bundle-activate-confirm",
  bundleActivationDialog: "bundle-activation-dialog",
  bundlePreviewError: "bundle-preview-error",
  detail: "marketplace-detail",
  detailAction: "marketplace-detail-action",
  extensionTrustConfirm: "extension-trust-confirm",
  extensionTrustDialog: "extension-trust-dialog",
  grid: "marketplace-grid",
  kindNavigation: "marketplace-kind-navigation",
  mcpInstallConfirm: "mcp-install-confirm",
  mcpInstallDialog: "mcp-install-dialog",
  mcpInstallError: "mcp-install-error",
  refresh: "marketplace-refresh",
} as const;

export const sandboxOperatorTestIds = {
  actionResult: "sandbox-page-action-result",
  actionResultDismiss: "sandbox-page-action-result-dismiss",
  osDesktop: sessionLifecycleTestIds.osDesktop,
  createButton: "sandbox-page-create",
  deleteConfirm: "settings-sandboxes-delete-confirm",
  deleteDialog: "settings-sandboxes-delete",
  deleteUsage: "sandbox-delete-usage",
  editor: "settings-sandbox-editor",
  editorBackendInput: "sandbox-editor-backend-input",
  editorError: "settings-sandbox-editor-error",
  editorNameInput: "sandbox-editor-name-input",
  editorPersistenceInput: "sandbox-editor-persistence-input",
  editorRuntimeRootInput: "sandbox-editor-runtime-root-input",
  editorSave: "settings-sandbox-editor-save",
  editorSyncModeInput: "sandbox-editor-sync-mode-input",
  empty: "sandbox-page-empty",
  list: "sandbox-page-list",
  restartNotice: "settings-page-sandbox-restart-notice",
  shell: "sandbox-shell",
  total: "sandbox-page-total",
  workspaceReferences: "sandbox-page-workspaces",
  workspaceOnboarding: sessionLifecycleTestIds.workspaceOnboarding,
  workspaceUseGlobal: sessionLifecycleTestIds.workspaceUseGlobal,
} as const;

export interface NetworkOperatorSelectors {
  osDesktop: Locator;
  agentOption(agentName: string): Locator;
  channelItem(channelName: string): Locator;
  channelMessage(messageId: string): Locator;
  channelNameInput: Locator;
  channelHeader: Locator;
  channelIdentityMix: Locator;
  channelInspectorToggle: Locator;
  channelTabs: Locator;
  createDialog: Locator;
  createAgentTrigger: Locator;
  createSubmit: Locator;
  channelPurposeInput: Locator;
  disabledState: Locator;
  activityFeed: Locator;
  directItem(directId: string): Locator;
  directList: Locator;
  directRoom: Locator;
  directsTab: Locator;
  directTab: Locator;
  inspectorToggle: Locator;
  messageList: Locator;
  inspector: Locator;
  inspectorMembersTab: Locator;
  inspectorPanelMembers: Locator;
  inspectorPanelWork: Locator;
  inspectorWorkTab: Locator;
  noChannelsState: Locator;
  newDirectButton: Locator;
  newDirectDialog: Locator;
  newDirectPeer(peerId: string): Locator;
  openCreateDialog: Locator;
  workInspector: Locator;
  workInspectorRow(workId: string): Locator;
  threadItem(threadId: string): Locator;
  threadList: Locator;
  threadOverlay: Locator;
  threadsTab: Locator;
  threadTab: Locator;
  workspace: Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

export interface AutomationOperatorSelectors {
  osDesktop: Locator;
  automationSuggestionsCard: Locator;
  automationDeleteConfirmTyping: Locator;
  automationDeleteDialog: Locator;
  confirmDeleteAutomationButton: Locator;
  createJobButton: Locator;
  createTriggerButton: Locator;
  deleteAutomationButton: Locator;
  detailOverflow: Locator;
  detailPanel: Locator;
  editAutomationButton: Locator;
  item(id: string): Locator;
  suggestion(id: string): Locator;
  editorDialog: Locator;
  jobForm: Locator;
  jobAgentInput: Locator;
  jobEnabledToggle: Locator;
  jobFireLimitMax: Locator;
  jobFireLimitWindow: Locator;
  jobGovernanceToggle: Locator;
  jobNameInput: Locator;
  jobPromptInput: Locator;
  jobScheduleCustom: Locator;
  jobScheduleExpr: Locator;
  jobScheduleInterval: Locator;
  jobScheduleModeAt: Locator;
  jobScheduleModeCron: Locator;
  jobScheduleModeEvery: Locator;
  jobScheduleTime: Locator;
  jobScopeGlobal: Locator;
  jobScopeWorkspace: Locator;
  jobsListRows: Locator;
  jobsShell: Locator;
  itemLink(id: string): Locator;
  run(id: string): Locator;
  runHistory: Locator;
  runNow(id: string): Locator;
  runSessionLink(runId: string): Locator;
  submitJobForm: Locator;
  submitTriggerForm: Locator;
  triggerAgentInput: Locator;
  triggerEnabledToggle: Locator;
  triggerEndpointSlugInput: Locator;
  triggerEventOption(eventId: string): Locator;
  triggerFilterAdd: Locator;
  triggerFilterKey(index: number): Locator;
  triggerFilterValue(index: number): Locator;
  triggerFireLimitMax: Locator;
  triggerFireLimitWindow: Locator;
  triggersListRows: Locator;
  triggersShell: Locator;
  triggerPromptInput: Locator;
  triggerRetryMax: Locator;
  triggerRetryStrategyBackoff: Locator;
  triggerRetryStrategyNone: Locator;
  triggerScopeGlobal: Locator;
  triggerWebhookIDInput: Locator;
  triggerWebhookSecretValueInput: Locator;
  toggleAutomationButton: Locator;
  triggerJobButton: Locator;
  triggerNameInput: Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

export interface BridgeOperatorSelectors {
  activeRoutesMetric: Locator;
  osDesktop: Locator;
  backToList: Locator;
  createBridgeButton: Locator;
  createDialog: Locator;
  createDeliveryModeSelect: Locator;
  createDeliveryPeerInput: Locator;
  createDeliveryThreadInput: Locator;
  createDisplayNameInput: Locator;
  createProviderConfigError: Locator;
  createProviderConfigInput: Locator;
  createRoutingIncludePeer: Locator;
  createRoutingIncludeThread: Locator;
  createScopeSelect: Locator;
  createWizardNext: Locator;
  deleteSecret(bindingName: string): Locator;
  detailPanel: Locator;
  detailOverflow: Locator;
  disableBridgeButton: Locator;
  editBridgeButton: Locator;
  editDialog: Locator;
  editDisplayNameInput: Locator;
  editProviderConfigInput: Locator;
  enableBridgeButton: Locator;
  item(id: string): Locator;
  addListFilter: Locator;
  listPanel: Locator;
  manifestHandoff: Locator;
  manifestJson: Locator;
  manifestOpenBridge: Locator;
  editModeAdvanced: Locator;
  openTestDeliveryButton: Locator;
  providerCard(providerKey: string): Locator;
  restartBridgeButton: Locator;
  restartRequired: Locator;
  route(sessionId: string): Locator;
  saveSecret(bindingName: string): Locator;
  searchInput: Locator;
  secretBinding(bindingName: string): Locator;
  secretCheck(bindingName: string, check: string): Locator;
  secretEnvInput(bindingName: string): Locator;
  deliveryTestPanel: Locator;
  sendTestResult: Locator;
  setupChecklist: Locator;
  setupItem(id: string): Locator;
  submitBridgeCreate: Locator;
  submitBridgeEdit: Locator;
  submitSendTest: Locator;
  submitTestDelivery: Locator;
  testDeliveryMessage: Locator;
  testDeliveryModeSelect: Locator;
  testDeliveryPeerInput: Locator;
  testDeliveryResult: Locator;
  testDeliveryThreadInput: Locator;
  verifyBridgeButton: Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

export interface KnowledgeOperatorSelectors {
  osDesktop: Locator;
  cancelCreateMemory: Locator;
  confirmCreateMemory: Locator;
  confirmDeleteMemory: Locator;
  confirmEditMemory: Locator;
  contentPreview: Locator;
  createButton: Locator;
  createContent: Locator;
  createDescription: Locator;
  createDialog: Locator;
  createName: Locator;
  createType: Locator;
  deleteButton: Locator;
  deleteDialog: Locator;
  detailPanel: Locator;
  editButton: Locator;
  editContent: Locator;
  editDescription: Locator;
  editDialog: Locator;
  guard: Locator;
  item(memoryKey: string): Locator;
  listPanel: Locator;
  revertDecision(decisionId: string): Locator;
  searchInput: Locator;
  searchInfo: Locator;
  shell: Locator;
  tabAgent: Locator;
  tabGlobal: Locator;
  tabWorkspace: Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

export interface MarketplaceOperatorSelectors {
  action(entryId: string): Locator;
  bundleActivateConfirm: Locator;
  bundleActivationDialog: Locator;
  bundlePreviewError: Locator;
  card(entryId: string): Locator;
  detail: Locator;
  detailAction: Locator;
  extensionTrustConfirm: Locator;
  extensionTrustDialog: Locator;
  grid: Locator;
  kind(kind: string): Locator;
  kindNavigation: Locator;
  mcpCreateSecret(envName: string): Locator;
  mcpInstallConfirm: Locator;
  mcpInstallDialog: Locator;
  mcpInstallError: Locator;
  mcpVaultSelector(envName: string): Locator;
  refresh: Locator;
}

export interface SandboxOperatorSelectors {
  actionResult: Locator;
  actionResultDismiss: Locator;
  osDesktop: Locator;
  createButton: Locator;
  deleteConfirm: Locator;
  deleteDialog: Locator;
  deleteProfile(name: string): Locator;
  deleteUsage: Locator;
  editProfile(name: string): Locator;
  editor: Locator;
  editorBackendInput: Locator;
  editorError: Locator;
  editorNameInput: Locator;
  editorPersistenceInput: Locator;
  editorRuntimeRootInput: Locator;
  editorSave: Locator;
  editorSyncModeInput: Locator;
  empty: Locator;
  list: Locator;
  profile(name: string): Locator;
  profileMetadata(name: string): Locator;
  profileSource(name: string): Locator;
  profileUsage(name: string): Locator;
  restartNotice: Locator;
  shell: Locator;
  total: Locator;
  workspaceReferences: Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

export const settingsShellTestIds = {
  shell: "settings-shell",
  shellOutlet: "settings-shell-outlet",
  sectionNav: "settings-section-nav",
} as const;

export const settingsGeneralTestIds = {
  page: "settings-page-general",
  saveBar: "settings-page-general-save-bar",
  saveButton: "settings-page-general-save",
  resetButton: "settings-page-general-reset",
  sessionTimeoutInput: "settings-page-general-session-timeout-input",
  restartNotice: "settings-page-general-restart-notice",
  restartTrigger: "settings-page-general-restart-trigger",
  restartDismiss: "settings-page-general-restart-dismiss",
} as const;

export const settingsSkillsTestIds = {
  page: "settings-page-skills",
  disabledList: "settings-page-skills-disabled-list",
  disabledMessage: "settings-page-skills-disabled-message",
  disabledSave: "settings-page-skills-disabled-save",
  save: "settings-page-skills-save",
  policyRegistryInput: "settings-page-skills-marketplace-registry-input",
  policyBaseURLInput: "settings-page-skills-marketplace-base-url-input",
  operationalLink: "settings-page-skills-link-skills",
  restartNotice: "settings-page-skills-restart-notice",
} as const;

export const settingsRolesTestIds = {
  page: "settings-page-roles",
  saveBar: "settings-page-roles-save-bar",
  saveButton: "settings-page-roles-save",
  resetButton: "settings-page-roles-reset",
  saveMessage: "settings-page-roles-save-message",
} as const;

export const settingsProvidersTestIds = {
  page: "settings-page-providers",
  list: "settings-page-providers-list",
  create: "settings-page-providers-create",
  actionResult: "settings-page-providers-action-result",
  actionResultDismiss: "settings-page-providers-action-result-dismiss",
  editor: "provider-detail-dialog",
  editorEdit: "provider-detail-edit",
  editorDelete: "provider-detail-delete",
  editorNameInput: "settings-providers-editor-name-input",
  editorCommandInput: "settings-providers-editor-command-input",
  editorModelInput: "settings-providers-editor-model-input",
  editorModeSimple: "settings-providers-editor-mode-simple",
  editorModeAdvanced: "settings-providers-editor-mode-advanced",
  editorSave: "provider-detail-save",
  deleteDialog: "settings-providers-delete",
  deleteConfirm: "settings-providers-delete-confirm",
  restartNotice: "settings-page-providers-restart-notice",
} as const;

export const settingsMCPServersTestIds = {
  page: "marketplace-kind-mcp",
  list: "marketplace-installed-grid",
  create: "marketplace-mcp-add",
  actionResult: "marketplace-mcp-save-toast",
  actionResultDismiss: "settings-page-mcp-servers-action-result-dismiss",
  scopeGlobal: "marketplace-mcp-config-scope-global",
  scopeWorkspace: "marketplace-mcp-config-scope-workspace",
  scopeLabel: "marketplace-mcp-config-scope",
  editor: "settings-mcp-servers-editor",
  editorNameInput: "settings-mcp-servers-editor-name-input",
  editorCommandInput: "settings-mcp-servers-editor-command-input",
  editorTargetInput: "settings-mcp-servers-editor-target-input",
  editorSave: "settings-mcp-servers-editor-save",
  editorRemove: "settings-mcp-servers-editor-remove",
  deleteDialog: "settings-mcp-servers-delete",
  deleteConfirm: "settings-mcp-servers-delete-confirm",
} as const;

export const settingsHooksTestIds = {
  hooksList: "settings-page-hooks-list",
  page: "settings-page-hooks",
  restartNotice: "settings-page-hooks-restart-notice",
} as const;

export const settingsExtensionsTestIds = {
  allowUnverified: "settings-page-extensions-policy-allow-unverified-input",
  page: "settings-page-extensions",
  policyBaseURLInput: "settings-page-extensions-policy-base-url-input",
  policyControls: "settings-page-extensions-policy-controls",
  policyRegistryInput: "settings-page-extensions-policy-registry-input",
  save: "settings-page-extensions-save",
  restartNotice: "settings-page-extensions-restart-notice",
} as const;

interface SettingsShellSelectors {
  shell: Locator;
  shellOutlet: Locator;
  sectionItems: Locator;
  sectionNav: Locator;
  sectionLink(slug: string): Locator;
  sectionActive(slug: string): Locator;
}

interface SettingsGeneralSelectors {
  page: Locator;
  resetButton: Locator;
  restartDismiss: Locator;
  restartNotice: Locator;
  restartTrigger: Locator;
  saveBar: Locator;
  saveButton: Locator;
  sessionTimeoutInput: Locator;
}

interface SettingsSkillsSelectors {
  page: Locator;
  disabledList: Locator;
  disabledMessage: Locator;
  disabledSave: Locator;
  disabledToggle(name: string): Locator;
  operationalLink: Locator;
  policyBaseURLInput: Locator;
  policyRegistryInput: Locator;
  save: Locator;
  restartNotice: Locator;
}

interface SettingsRolesSelectors {
  page: Locator;
  saveBar: Locator;
  saveButton: Locator;
  resetButton: Locator;
  saveMessage: Locator;
  group(role: string): Locator;
  toggle(role: string): Locator;
  routeSummary(role: string): Locator;
  runtimeSelect(role: string): Locator;
  runtimeClear(role: string): Locator;
  agentSelect(role: string): Locator;
  fieldInput(role: string, field: string): Locator;
  enabledSwitch(role: string): Locator;
  diagnostics(role: string): Locator;
  fallbackAdd(role: string): Locator;
  fallbackEntrySelect(role: string, index: number): Locator;
}

interface SettingsProvidersSelectors {
  actionResult: Locator;
  actionResultDismiss: Locator;
  card(name: string): Locator;
  cardCommand(name: string): Locator;
  create: Locator;
  deleteConfirm: Locator;
  deleteDialog: Locator;
  editor: Locator;
  editorEdit: Locator;
  editorDelete: Locator;
  editorCommandInput: Locator;
  editorModelInput: Locator;
  editorModeAdvanced: Locator;
  editorModeSimple: Locator;
  editorNameInput: Locator;
  editorSave: Locator;
  list: Locator;
  page: Locator;
  inspectorSource: Locator;
  restartNotice: Locator;
}

interface SettingsMCPServersSelectors {
  actionResult: Locator;
  actionResultDismiss: Locator;
  create: Locator;
  deleteConfirm: Locator;
  deleteDialog: Locator;
  editRow(name: string): Locator;
  editor: Locator;
  editorCommandInput: Locator;
  editorNameInput: Locator;
  editorRemove: Locator;
  editorSave: Locator;
  editorTargetInput: Locator;
  list: Locator;
  page: Locator;
  row(name: string): Locator;
  rowSource(name: string): Locator;
  scopeGlobal: Locator;
  scopeLabel: Locator;
  scopeWorkspace: Locator;
}

interface SettingsHooksSelectors {
  hooksList: Locator;
  hookToggle(name: string): Locator;
  page: Locator;
  restartNotice: Locator;
}

interface SettingsExtensionsSelectors {
  allowUnverified: Locator;
  page: Locator;
  policyBaseURLInput: Locator;
  policyControls: Locator;
  policyRegistryInput: Locator;
  save: Locator;
  restartNotice: Locator;
}

export interface SettingsOperatorSelectors {
  shell: SettingsShellSelectors;
  extensions: SettingsExtensionsSelectors;
  general: SettingsGeneralSelectors;
  hooks: SettingsHooksSelectors;
  mcpServers: SettingsMCPServersSelectors;
  providers: SettingsProvidersSelectors;
  roles: SettingsRolesSelectors;
  skills: SettingsSkillsSelectors;
}
export const tasksOperatorTestIds = {
  osDesktop: sessionLifecycleTestIds.osDesktop,
  createDescription: "task-description-input",
  createEditorSurface: "task-editor-surface",
  createModeAdvanced: "task-mode-advanced",
  createModeSimple: "task-mode-simple",
  createSaveDraft: "task-editor-modal-submit",
  createScopeGlobal: "task-scope-global",
  createScopeWorkspace: "task-scope-workspace",
  createSubmit: "task-editor-modal-submit",
  createTitle: "task-title-input",
  createWorkspaceSelect: "task-workspace-select",
  dashboardView: "tasks-dashboard-view",
  detailActiveRunChannel: "tasks-detail-active-run-channel",
  detailActiveRunEmpty: "tasks-detail-active-run-empty",
  detailActiveRunEmptyHint: "tasks-detail-active-run-empty-hint",
  detailApprovalPill: "tasks-detail-pill-approval",
  detailContent: "tasks-detail-content",
  detailInspectDrawer: "tasks-inspect-drawer",
  detailInspectStream: "tasks-inspect-stream",
  detailCoordination: "tasks-detail-coordination",
  detailCancel: "tasks-detail-cancel",
  detailDelete: "tasks-detail-delete",
  detailDeleteCancel: "tasks-detail-delete-cancel",
  detailDeleteConfirm: "tasks-detail-delete-confirm",
  detailDeleteDialog: "tasks-detail-delete-dialog",
  detailEdit: "tasks-detail-edit",
  detailEnqueue: "tasks-detail-primary-start",
  detailOverflow: "tasks-detail-overflow",
  detailNowApproval: "tasks-detail-now-approval",
  detailNowRun: "tasks-detail-now-run",
  detailPublish: "tasks-detail-primary-publish",
  detailStatus: "tasks-detail-status",
  detailTitle: "tasks-detail-title",
  detailPreviewCoordination: "tasks-detail-preview-coordination",
  detailPreviewDeeplink: "tasks-detail-preview-deeplink",
  detailPreviewLifecycle: "tasks-detail-preview-lifecycle",
  detailPreviewPanel: "tasks-detail-preview-panel",
  detailPreviewPublish: "tasks-detail-preview-publish",
  detailRunsEmpty: "tasks-runs-empty",
  detailTabRuns: "tasks-detail-tab-runs",
  detailSetupEdit: "tasks-setup-edit",
  detailSetupForm: "tasks-setup-form",
  detailSetupOpen: "tasks-rail-edit-setup",
  detailSetupSheet: "tasks-setup-sheet",
  detailSetupWorkerRuntime: "tasks-setup-worker-runtime",
  inboxView: "tasks-inbox-view",
  modeDashboard: "tasks-mode-dashboard",
  modeInbox: "tasks-mode-inbox",
  modeKanban: "tasks-mode-kanban",
  modeList: "tasks-mode-list",
  multiAgentDisconnected: "tasks-multi-agent-disconnected",
  multiAgentEmpty: "tasks-multi-agent-empty",
  multiAgentNoActive: "tasks-multi-agent-no-active",
  multiAgentPanel: "tasks-multi-agent-panel",
  multiAgentSummary: "tasks-multi-agent-summary",
  openCreate: "tasks-open-create",
  runDetailContent: "tasks-run-detail-content",
  runDetailCancel: "tasks-run-cancel",
  runDetailOverflow: "tasks-run-overflow",
  runReviews: "tasks-run-reviews",
  runSessionDrilldown: "tasks-run-open-session",
  workspaceOnboarding: sessionLifecycleTestIds.workspaceOnboarding,
  workspaceUseGlobal: sessionLifecycleTestIds.workspaceUseGlobal,
} as const;

const tasksInboxGroupByLane: Record<string, string> = {
  approvals: "needs_review",
  failed_runs: "needs_review",
};

export interface TasksOperatorSelectors {
  osDesktop: Locator;
  createDescription: Locator;
  createEditorSurface: Locator;
  createPriority(priority: string): Locator;
  createModeAdvanced: Locator;
  createModeSimple: Locator;
  createSaveDraft: Locator;
  createScopeGlobal: Locator;
  createScopeWorkspace: Locator;
  createSubmit: Locator;
  createTemplate(templateId: string): Locator;
  createTitle: Locator;
  createWorkspaceOption(workspaceId: string): Locator;
  createWorkspaceSelect: Locator;
  dashboardActiveRun(runId: string): Locator;
  dashboardActiveRunLink(runId: string): Locator;
  dashboardView: Locator;
  detailActiveRunChannel: Locator;
  detailActiveRunEmpty: Locator;
  detailActiveRunEmptyHint: Locator;
  detailApprovalPill: Locator;
  detailBreadcrumbTasks: Locator;
  detailContent: Locator;
  detailTitle: Locator;
  detailInspectDrawer: Locator;
  detailInspectStream: Locator;
  detailCoordination: Locator;
  detailCancel: Locator;
  detailDelete: Locator;
  detailDeleteCancel: Locator;
  detailDeleteConfirm: Locator;
  detailDeleteDialog: Locator;
  detailEdit: Locator;
  detailEnqueue: Locator;
  detailOverflow: Locator;
  detailNowApproval: Locator;
  detailNowRun: Locator;
  detailPublish: Locator;
  detailStatus: Locator;
  detailPreviewCoordination: Locator;
  detailPreviewDeeplink: Locator;
  detailPreviewLifecycle: Locator;
  detailPreviewPanel: Locator;
  detailPreviewPublish: Locator;
  detailRunsChannel(runId: string): Locator;
  detailRunsEmpty: Locator;
  detailTab(tabId: string): Locator;
  detailTabRuns: Locator;
  detailSetupEdit: Locator;
  detailSetupForm: Locator;
  detailSetupOpen: Locator;
  detailSetupSheet: Locator;
  detailSetupWorkerRuntime: Locator;
  detailChildItem(taskId: string): Locator;
  detailChildLink(taskId: string): Locator;
  detailDependencyItem(taskId: string): Locator;
  detailDependencyLink(taskId: string): Locator;
  inboxApprove(taskId: string): Locator;
  inboxArchive(taskId: string): Locator;
  inboxDismiss(taskId: string): Locator;
  inboxItem(taskId: string): Locator;
  inboxLane(lane: string): Locator;
  inboxOpenTask(taskId: string): Locator;
  inboxReject(taskId: string): Locator;
  inboxRetry(taskId: string): Locator;
  inboxView: Locator;
  modeDashboard: Locator;
  modeInbox: Locator;
  modeKanban: Locator;
  modeList: Locator;
  multiAgentDisconnected: Locator;
  multiAgentEmpty: Locator;
  multiAgentNoActive: Locator;
  multiAgentPanel: Locator;
  multiAgentSummary: Locator;
  multiAgentAgentLink(taskId: string): Locator;
  openCreate: Locator;
  runDetailContent: Locator;
  runDetailCancel: Locator;
  runDetailOverflow: Locator;
  runReviews: Locator;
  runsRow(runId: string): Locator;
  runReviewRow(reviewId: string): Locator;
  runSessionDrilldown: Locator;
  taskCard(taskId: string): Locator;
  taskCardPublish(taskId: string): Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}
export function sessionLifecycleSelectors(
  page: Pick<Page, "getByRole" | "getByTestId">
): SessionLifecycleSelectors {
  return {
    agentPageNewSession: page.getByTestId("agent-page-new-session"),
    agentRow: (agentName: string) => page.getByTestId(`agent-fleet-row-link-${agentName}`),
    osDesktop: page.getByTestId(sessionLifecycleTestIds.osDesktop),
    workspaceManualPathInput: page.getByTestId(sessionLifecycleTestIds.workspaceManualPathInput),
    workspaceOnboarding: page.getByTestId(sessionLifecycleTestIds.workspaceOnboarding),
    workspaceRegisterManual: page.getByTestId(sessionLifecycleTestIds.workspaceRegisterManual),
    workspaceUseGlobal: page.getByTestId(sessionLifecycleTestIds.workspaceUseGlobal),
  };
}

/**
 * Controls that live inside one session window. `win` is REQUIRED — pass the
 * owning `os-window-session:<sessionId>` locator (`sessionWindow(page, id)` from
 * `./os-navigation`) so every locator resolves within that window and never
 * matches a second session window at page scope.
 */
export function sessionWindowSelectors(
  win: Locator,
  portalRoot: Pick<Page, "getByTestId"> = win
): SessionWindowSelectors {
  return {
    chatView: win.getByTestId(sessionWindowTestIds.chatView),
    composerClearButton: portalRoot.getByTestId(sessionWindowTestIds.composerClearButton),
    composerSendButton: win.getByRole("button", { name: "Send message" }),
    composerTextarea: win.getByRole("textbox", { name: "Session prompt" }),
    deleteButton: portalRoot.getByTestId(sessionWindowTestIds.deleteButton),
    permissionAllowAlways: win.getByTestId(sessionWindowTestIds.permissionAllowAlways),
    permissionAllowOnce: win.getByTestId(sessionWindowTestIds.permissionAllowOnce),
    permissionPrompt: win.getByTestId(sessionWindowTestIds.permissionPrompt),
    processingIndicator: win.getByTestId(sessionWindowTestIds.processingIndicator),
    resumeButton: win.getByTestId(sessionWindowTestIds.resumeButton),
    stopButton: win.getByTestId(sessionWindowTestIds.stopButton),
    topbarOverflow: win.getByTestId(sessionWindowTestIds.topbarOverflow),
  };
}

export const toolApprovalGrantsTestIds = {
  section: "settings-page-general-tool-approvals-section",
  list: "settings-page-general-tool-approvals-list",
  empty: "settings-page-general-tool-approvals-empty",
  revokeDialog: "settings-page-general-tool-approvals-revoke",
  revokeConfirm: "settings-page-general-tool-approvals-revoke-confirm",
  revokeCancel: "settings-page-general-tool-approvals-revoke-cancel",
  setOpen: "settings-page-general-tool-approvals-set-open",
  setDialog: "tool-approval-grant-set-dialog",
  setScopeAgent: "tool-approval-grant-scope-agent",
  setScopeTool: "tool-approval-grant-scope-tool",
  setToolID: "tool-approval-grant-tool-id",
  setAgentName: "tool-approval-grant-agent-name",
  setDecisionAllow: "tool-approval-grant-decision-allow",
  setDecisionReject: "tool-approval-grant-decision-reject",
  setConfirm: "tool-approval-grant-set-confirm",
} as const;

export interface ToolApprovalGrantsSelectors {
  section: Locator;
  list: Locator;
  empty: Locator;
  revokeDialog: Locator;
  revokeConfirm: Locator;
  revokeCancel: Locator;
  setOpen: Locator;
  setDialog: Locator;
  setScopeAgent: Locator;
  setScopeTool: Locator;
  setToolID: Locator;
  setAgentName: Locator;
  setDecisionAllow: Locator;
  setDecisionReject: Locator;
  setConfirm: Locator;
  row(grantId: string): Locator;
  decision(grantId: string): Locator;
  revoke(grantId: string): Locator;
}

export function toolApprovalGrantsSelectors(
  page: Pick<Page, "getByTestId" | "locator">
): ToolApprovalGrantsSelectors {
  return {
    section: page.getByTestId(toolApprovalGrantsTestIds.section),
    list: page.getByTestId(toolApprovalGrantsTestIds.list),
    empty: page.getByTestId(toolApprovalGrantsTestIds.empty),
    revokeDialog: page.getByTestId(toolApprovalGrantsTestIds.revokeDialog),
    revokeConfirm: page.getByTestId(toolApprovalGrantsTestIds.revokeConfirm),
    revokeCancel: page.getByTestId(toolApprovalGrantsTestIds.revokeCancel),
    setOpen: page.getByTestId(toolApprovalGrantsTestIds.setOpen),
    setDialog: page.getByTestId(toolApprovalGrantsTestIds.setDialog),
    setScopeAgent: page.getByTestId(toolApprovalGrantsTestIds.setScopeAgent),
    setScopeTool: page.getByTestId(toolApprovalGrantsTestIds.setScopeTool),
    setToolID: page.getByTestId(toolApprovalGrantsTestIds.setToolID),
    setAgentName: page.getByTestId(toolApprovalGrantsTestIds.setAgentName),
    setDecisionAllow: page.getByTestId(toolApprovalGrantsTestIds.setDecisionAllow),
    setDecisionReject: page.getByTestId(toolApprovalGrantsTestIds.setDecisionReject),
    setConfirm: page.getByTestId(toolApprovalGrantsTestIds.setConfirm),
    row: (grantId: string) =>
      page.locator(`[data-testid="tool-approval-grant-row"][data-grant-id="${grantId}"]`),
    decision: (grantId: string) => page.getByTestId(`tool-approval-grant-decision-${grantId}`),
    revoke: (grantId: string) => page.getByTestId(`tool-approval-grant-revoke-${grantId}`),
  };
}

export const sessionClarifyTestIds = {
  card: "clarification-card",
  question: "clarification-question",
  choice: "clarification-choice",
  textInput: "clarification-text-input",
  submit: "clarification-submit",
  receipt: "clarification-receipt",
  receiptAnswer: "clarification-receipt-answer",
  error: "clarification-error",
} as const;

export interface SessionClarifySelectors {
  card: Locator;
  question: Locator;
  textInput: Locator;
  submit: Locator;
  receipt: Locator;
  receiptAnswer: Locator;
  error: Locator;
  choice(index: number): Locator;
}

export function sessionClarifySelectors(
  page: Pick<Page, "getByTestId" | "locator">
): SessionClarifySelectors {
  return {
    card: page.getByTestId(sessionClarifyTestIds.card),
    question: page.getByTestId(sessionClarifyTestIds.question),
    textInput: page.getByTestId(sessionClarifyTestIds.textInput),
    submit: page.getByTestId(sessionClarifyTestIds.submit),
    receipt: page.getByTestId(sessionClarifyTestIds.receipt),
    receiptAnswer: page.getByTestId(sessionClarifyTestIds.receiptAnswer),
    error: page.getByTestId(sessionClarifyTestIds.error),
    choice: (index: number) =>
      page.locator(`[data-testid="clarification-choice"][data-choice-index="${index}"]`),
  };
}

export function networkOperatorSelectors(
  page: Pick<Page, "getByTestId" | "locator">
): NetworkOperatorSelectors {
  return {
    osDesktop: page.getByTestId(networkOperatorTestIds.osDesktop),
    agentOption: (agentName: string) => page.getByTestId(`network-agent-option-${agentName}`),
    channelItem: (channelName: string) => page.getByTestId(`network-channel-row-${channelName}`),
    channelMessage: (messageId: string) =>
      page.locator(
        `[data-testid="network-message-row-full"][data-message-id="${messageId}"], [data-testid="network-message-row-collapsed"][data-message-id="${messageId}"], [data-testid="network-message-row-system"][data-message-id="${messageId}"]`
      ),
    channelNameInput: page.getByTestId(networkOperatorTestIds.channelNameInput),
    channelHeader: page.getByTestId(networkOperatorTestIds.channelHeader),
    channelIdentityMix: page.getByTestId(networkOperatorTestIds.channelIdentityMix),
    channelInspectorToggle: page.getByTestId(networkOperatorTestIds.channelInspectorToggle),
    channelTabs: page.getByTestId(networkOperatorTestIds.channelTabs),
    createDialog: page.getByTestId(networkOperatorTestIds.createDialog),
    createAgentTrigger: page.getByTestId(networkOperatorTestIds.createAgentTrigger),
    createSubmit: page.getByTestId(networkOperatorTestIds.createSubmit),
    channelPurposeInput: page.getByTestId(networkOperatorTestIds.channelPurposeInput),
    disabledState: page.getByTestId(networkOperatorTestIds.disabledState),
    activityFeed: page.getByTestId(networkOperatorTestIds.activityFeed),
    directItem: (directId: string) => page.getByTestId(`network-direct-list-row-${directId}`),
    directList: page.getByTestId(networkOperatorTestIds.directList),
    directRoom: page.getByTestId(networkOperatorTestIds.directRoom),
    directsTab: page.getByTestId(networkOperatorTestIds.directsTab),
    directTab: page.getByTestId(networkOperatorTestIds.directTab),
    inspectorToggle: page.getByTestId(networkOperatorTestIds.inspectorToggle),
    messageList: page.getByTestId(networkOperatorTestIds.messageList),
    inspector: page.getByTestId(networkOperatorTestIds.inspector),
    inspectorMembersTab: page.getByTestId(networkOperatorTestIds.inspectorMembersTab),
    inspectorPanelMembers: page.getByTestId(networkOperatorTestIds.inspectorPanelMembers),
    inspectorPanelWork: page.getByTestId(networkOperatorTestIds.inspectorPanelWork),
    inspectorWorkTab: page.getByTestId(networkOperatorTestIds.inspectorWorkTab),
    noChannelsState: page.getByTestId(networkOperatorTestIds.noChannelsState),
    newDirectButton: page.getByTestId(networkOperatorTestIds.newDirectButton),
    newDirectDialog: page.getByTestId(networkOperatorTestIds.newDirectDialog),
    newDirectPeer: (peerId: string) => page.getByTestId(`network-new-direct-peer-${peerId}`),
    openCreateDialog: page.getByTestId(networkOperatorTestIds.openCreateDialog),
    workInspector: page.getByTestId(networkOperatorTestIds.workInspector),
    workInspectorRow: (workId: string) => page.getByTestId(`network-work-inspector-row-${workId}`),
    threadItem: (threadId: string) => page.getByTestId(`network-thread-list-row-${threadId}`),
    threadList: page.getByTestId(networkOperatorTestIds.threadList),
    threadOverlay: page.getByTestId(networkOperatorTestIds.threadOverlay),
    threadsTab: page.getByTestId(networkOperatorTestIds.threadsTab),
    threadTab: page.getByTestId(networkOperatorTestIds.threadTab),
    workspace: page.getByTestId(networkOperatorTestIds.workspace),
    workspaceOnboarding: page.getByTestId(networkOperatorTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(networkOperatorTestIds.workspaceUseGlobal),
  };
}

export function knowledgeOperatorSelectors(
  page: Pick<Page, "getByTestId">
): KnowledgeOperatorSelectors {
  return {
    osDesktop: page.getByTestId(knowledgeOperatorTestIds.osDesktop),
    cancelCreateMemory: page.getByTestId(knowledgeOperatorTestIds.cancelCreateMemory),
    confirmCreateMemory: page.getByTestId(knowledgeOperatorTestIds.confirmCreateMemory),
    confirmDeleteMemory: page.getByTestId(knowledgeOperatorTestIds.confirmDeleteMemory),
    confirmEditMemory: page.getByTestId(knowledgeOperatorTestIds.confirmEditMemory),
    contentPreview: page.getByTestId(knowledgeOperatorTestIds.contentPreview),
    createButton: page.getByTestId(knowledgeOperatorTestIds.createButton),
    createContent: page.getByTestId(knowledgeOperatorTestIds.createContent),
    createDescription: page.getByTestId(knowledgeOperatorTestIds.createDescription),
    createDialog: page.getByTestId(knowledgeOperatorTestIds.createDialog),
    createName: page.getByTestId(knowledgeOperatorTestIds.createName),
    createType: page.getByTestId(knowledgeOperatorTestIds.createType),
    deleteButton: page.getByTestId(knowledgeOperatorTestIds.deleteButton),
    deleteDialog: page.getByTestId(knowledgeOperatorTestIds.deleteDialog),
    detailPanel: page.getByTestId(knowledgeOperatorTestIds.detailPanel),
    editButton: page.getByTestId(knowledgeOperatorTestIds.editButton),
    editContent: page.getByTestId(knowledgeOperatorTestIds.editContent),
    editDescription: page.getByTestId(knowledgeOperatorTestIds.editDescription),
    editDialog: page.getByTestId(knowledgeOperatorTestIds.editDialog),
    guard: page.getByTestId(knowledgeOperatorTestIds.guard),
    item: (memoryKey: string) => page.getByTestId(`memory-item-${memoryKey}`),
    listPanel: page.getByTestId(knowledgeOperatorTestIds.listPanel),
    revertDecision: (decisionId: string) =>
      page.getByTestId(`revert-memory-decision-${decisionId}`),
    searchInput: page.getByTestId(knowledgeOperatorTestIds.searchInput),
    searchInfo: page.getByTestId(knowledgeOperatorTestIds.searchInfo),
    shell: page.getByTestId(knowledgeOperatorTestIds.shell),
    tabAgent: page.getByTestId(knowledgeOperatorTestIds.tabAgent),
    tabGlobal: page.getByTestId(knowledgeOperatorTestIds.tabGlobal),
    tabWorkspace: page.getByTestId(knowledgeOperatorTestIds.tabWorkspace),
    workspaceOnboarding: page.getByTestId(knowledgeOperatorTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(knowledgeOperatorTestIds.workspaceUseGlobal),
  };
}

export function marketplaceOperatorSelectors(
  page: Pick<Page, "getByTestId">
): MarketplaceOperatorSelectors {
  return {
    action: (entryId: string) => page.getByTestId(`marketplace-action-${entryId}`),
    bundleActivateConfirm: page.getByTestId(marketplaceOperatorTestIds.bundleActivateConfirm),
    bundleActivationDialog: page.getByTestId(marketplaceOperatorTestIds.bundleActivationDialog),
    bundlePreviewError: page.getByTestId(marketplaceOperatorTestIds.bundlePreviewError),
    card: (entryId: string) => page.getByTestId(`marketplace-card-${entryId}`),
    detail: page.getByTestId(marketplaceOperatorTestIds.detail),
    detailAction: page.getByTestId(marketplaceOperatorTestIds.detailAction),
    extensionTrustConfirm: page.getByTestId(marketplaceOperatorTestIds.extensionTrustConfirm),
    extensionTrustDialog: page.getByTestId(marketplaceOperatorTestIds.extensionTrustDialog),
    grid: page.getByTestId(marketplaceOperatorTestIds.grid),
    kind: (kind: string) => page.getByTestId(`marketplace-kind-${kind}`),
    kindNavigation: page.getByTestId(marketplaceOperatorTestIds.kindNavigation),
    mcpCreateSecret: (envName: string) => page.getByTestId(`mcp-create-secret-${envName}`),
    mcpInstallConfirm: page.getByTestId(marketplaceOperatorTestIds.mcpInstallConfirm),
    mcpInstallDialog: page.getByTestId(marketplaceOperatorTestIds.mcpInstallDialog),
    mcpInstallError: page.getByTestId(marketplaceOperatorTestIds.mcpInstallError),
    mcpVaultSelector: (envName: string) => page.getByTestId(`mcp-vault-selector-${envName}`),
    refresh: page.getByTestId(marketplaceOperatorTestIds.refresh),
  };
}

export function sandboxOperatorSelectors(
  page: Pick<Page, "getByTestId">
): SandboxOperatorSelectors {
  return {
    actionResult: page.getByTestId(sandboxOperatorTestIds.actionResult),
    actionResultDismiss: page.getByTestId(sandboxOperatorTestIds.actionResultDismiss),
    osDesktop: page.getByTestId(sandboxOperatorTestIds.osDesktop),
    createButton: page.getByTestId(sandboxOperatorTestIds.createButton),
    deleteConfirm: page.getByTestId(sandboxOperatorTestIds.deleteConfirm),
    deleteDialog: page.getByTestId(sandboxOperatorTestIds.deleteDialog),
    deleteProfile: (name: string) => page.getByTestId(`sandbox-page-card-${name}-delete`),
    deleteUsage: page.getByTestId(sandboxOperatorTestIds.deleteUsage),
    editProfile: (name: string) => page.getByTestId(`sandbox-page-card-${name}-edit`),
    editor: page.getByTestId(sandboxOperatorTestIds.editor),
    editorBackendInput: page.getByTestId(sandboxOperatorTestIds.editorBackendInput),
    editorError: page.getByTestId(sandboxOperatorTestIds.editorError),
    editorNameInput: page.getByTestId(sandboxOperatorTestIds.editorNameInput),
    editorPersistenceInput: page.getByTestId(sandboxOperatorTestIds.editorPersistenceInput),
    editorRuntimeRootInput: page.getByTestId(sandboxOperatorTestIds.editorRuntimeRootInput),
    editorSave: page.getByTestId(sandboxOperatorTestIds.editorSave),
    editorSyncModeInput: page.getByTestId(sandboxOperatorTestIds.editorSyncModeInput),
    empty: page.getByTestId(sandboxOperatorTestIds.empty),
    list: page.getByTestId(sandboxOperatorTestIds.list),
    profile: (name: string) => page.getByTestId(`sandbox-page-card-${name}`),
    profileMetadata: (name: string) => page.getByTestId(`sandbox-page-card-${name}-profile`),
    profileSource: (name: string) => page.getByTestId(`sandbox-page-card-${name}-source`),
    profileUsage: (name: string) => page.getByTestId(`sandbox-page-card-${name}-usage`),
    restartNotice: page.getByTestId(sandboxOperatorTestIds.restartNotice),
    shell: page.getByTestId(sandboxOperatorTestIds.shell),
    total: page.getByTestId(sandboxOperatorTestIds.total),
    workspaceReferences: page.getByTestId(sandboxOperatorTestIds.workspaceReferences),
    workspaceOnboarding: page.getByTestId(sandboxOperatorTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(sandboxOperatorTestIds.workspaceUseGlobal),
  };
}

export function automationOperatorSelectors(
  page: Pick<Page, "getByLabel" | "getByRole" | "getByTestId">,
  portalRoot: Pick<Page, "getByTestId"> = page
): AutomationOperatorSelectors {
  const editorDialog = page.getByTestId(automationOperatorTestIds.automationEditorDialog);

  return {
    osDesktop: page.getByTestId(automationOperatorTestIds.osDesktop),
    automationSuggestionsCard: page.getByTestId(
      automationOperatorTestIds.automationSuggestionsCard
    ),
    automationDeleteConfirmTyping: page.getByTestId(
      automationOperatorTestIds.automationDeleteConfirmTyping
    ),
    automationDeleteDialog: page.getByTestId(automationOperatorTestIds.automationDeleteDialog),
    confirmDeleteAutomationButton: page.getByTestId(
      automationOperatorTestIds.confirmDeleteAutomationButton
    ),
    createJobButton: page.getByTestId(automationOperatorTestIds.createJobButton),
    createTriggerButton: page.getByTestId(automationOperatorTestIds.createTriggerButton),
    deleteAutomationButton: portalRoot.getByTestId(
      automationOperatorTestIds.deleteAutomationButton
    ),
    detailOverflow: page.getByTestId(automationOperatorTestIds.detailOverflow),
    detailPanel: page.getByTestId(automationOperatorTestIds.automationDetailPanel),
    editAutomationButton: portalRoot.getByTestId(automationOperatorTestIds.editAutomationButton),
    editorDialog,
    item: (id: string) => page.getByTestId(`automation-item-${id}`),
    itemLink: (id: string) => page.getByTestId(`automation-item-${id}`).getByRole("link"),
    jobAgentInput: page.getByTestId(automationOperatorTestIds.jobAgentInput),
    jobEnabledToggle: page.getByTestId(automationOperatorTestIds.jobEnabledToggle),
    jobFireLimitMax: page.getByTestId(automationOperatorTestIds.jobFireLimitMax),
    jobFireLimitWindow: page.getByTestId(automationOperatorTestIds.jobFireLimitWindow),
    jobForm: page.getByTestId(automationOperatorTestIds.automationJobForm),
    jobGovernanceToggle: page.getByTestId(automationOperatorTestIds.jobGovernanceToggle),
    jobNameInput: page.getByTestId(automationOperatorTestIds.jobNameInput),
    jobPromptInput: page.getByTestId(automationOperatorTestIds.jobPromptInput),
    jobScheduleCustom: editorDialog.getByRole("button", { name: "Custom" }),
    jobScheduleExpr: editorDialog.getByLabel("Cron expression"),
    jobScheduleInterval: page.getByTestId(automationOperatorTestIds.jobScheduleInterval),
    jobScheduleModeAt: page.getByTestId(automationOperatorTestIds.jobScheduleModeAt),
    jobScheduleModeCron: page.getByTestId(automationOperatorTestIds.jobScheduleModeCron),
    jobScheduleModeEvery: page.getByTestId(automationOperatorTestIds.jobScheduleModeEvery),
    jobScheduleTime: page.getByTestId(automationOperatorTestIds.jobScheduleTime),
    jobScopeGlobal: page.getByTestId(automationOperatorTestIds.jobScopeGlobal),
    jobScopeWorkspace: page.getByTestId(automationOperatorTestIds.jobScopeWorkspace),
    jobsListRows: page.getByTestId(automationOperatorTestIds.jobsListRows),
    jobsShell: page.getByTestId(automationOperatorTestIds.jobsShell),
    run: (id: string) => page.getByTestId(`automation-run-${id}`),
    runHistory: page.getByTestId(automationOperatorTestIds.automationRunHistory),
    runNow: (id: string) => page.getByTestId(`automation-run-now-${id}`),
    runSessionLink: (runId: string) => page.getByTestId(`automation-run-${runId}`),
    submitJobForm: page.getByTestId(automationOperatorTestIds.submitJobForm),
    submitTriggerForm: page.getByTestId(automationOperatorTestIds.submitTriggerForm),
    suggestion: (id: string) => page.getByTestId(`automation-suggestion-${id}`),
    triggerAgentInput: page.getByTestId(automationOperatorTestIds.triggerAgentInput),
    triggerEnabledToggle: page.getByTestId(automationOperatorTestIds.triggerEnabledToggle),
    triggerEndpointSlugInput: page.getByTestId(automationOperatorTestIds.triggerEndpointSlugInput),
    triggerEventOption: (eventId: string) => page.getByTestId(`trigger-event-${eventId}`),
    triggerFilterAdd: page.getByRole("button", { name: "Add condition" }),
    triggerFilterKey: (index: number) => page.getByTestId(`trigger-filter-key-${index}`),
    triggerFilterValue: (index: number) => page.getByTestId(`trigger-filter-value-${index}`),
    triggerFireLimitMax: page.getByTestId(automationOperatorTestIds.triggerFireLimitMax),
    triggerFireLimitWindow: page.getByTestId(automationOperatorTestIds.triggerFireLimitWindow),
    triggersListRows: page.getByTestId(automationOperatorTestIds.triggersListRows),
    triggersShell: page.getByTestId(automationOperatorTestIds.triggersShell),
    triggerPromptInput: page.getByTestId(automationOperatorTestIds.triggerPromptInput),
    triggerRetryMax: page.getByTestId(automationOperatorTestIds.triggerRetryMax),
    triggerRetryStrategyBackoff: page.getByTestId(
      automationOperatorTestIds.triggerRetryStrategyBackoff
    ),
    triggerRetryStrategyNone: page.getByTestId(automationOperatorTestIds.triggerRetryStrategyNone),
    triggerScopeGlobal: page.getByTestId(automationOperatorTestIds.triggerScopeGlobal),
    triggerWebhookIDInput: page.getByTestId(automationOperatorTestIds.triggerWebhookIDInput),
    triggerWebhookSecretValueInput: page.getByTestId(
      automationOperatorTestIds.triggerWebhookSecretValueInput
    ),
    toggleAutomationButton: portalRoot.getByTestId(
      automationOperatorTestIds.toggleAutomationButton
    ),
    triggerJobButton: page.getByTestId(automationOperatorTestIds.triggerJobButton),
    triggerNameInput: page.getByTestId(automationOperatorTestIds.triggerNameInput),
    workspaceOnboarding: page.getByTestId(automationOperatorTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(automationOperatorTestIds.workspaceUseGlobal),
  };
}

export function bridgeOperatorSelectors(
  page: Pick<Page, "getByRole" | "getByTestId">
): BridgeOperatorSelectors {
  const windowPath = page.getByRole("navigation", { name: "Window path" });

  return {
    activeRoutesMetric: page.getByTestId(bridgeOperatorTestIds.bridgeMetricActiveRoutes),
    osDesktop: page.getByTestId(bridgeOperatorTestIds.osDesktop),
    backToList: windowPath.getByRole("button", { exact: true, name: "Bridges" }),
    createBridgeButton: page.getByTestId(bridgeOperatorTestIds.createBridgeButton),
    createDialog: page.getByTestId(bridgeOperatorTestIds.bridgeCreateDialog),
    createDeliveryModeSelect: page.getByTestId(
      bridgeOperatorTestIds.createBridgeDeliveryModeSelect
    ),
    createDeliveryPeerInput: page.getByTestId(bridgeOperatorTestIds.createBridgeDeliveryPeerInput),
    createDeliveryThreadInput: page.getByTestId(
      bridgeOperatorTestIds.createBridgeDeliveryThreadInput
    ),
    createDisplayNameInput: page.getByTestId(bridgeOperatorTestIds.createBridgeDisplayNameInput),
    createProviderConfigError: page.getByTestId(
      bridgeOperatorTestIds.createBridgeProviderConfigError
    ),
    createProviderConfigInput: page.getByTestId(
      bridgeOperatorTestIds.createBridgeProviderConfigInput
    ),
    createRoutingIncludePeer: page.getByTestId(
      bridgeOperatorTestIds.createBridgeRoutingIncludePeer
    ),
    createRoutingIncludeThread: page.getByTestId(
      bridgeOperatorTestIds.createBridgeRoutingIncludeThread
    ),
    createScopeSelect: page.getByTestId(bridgeOperatorTestIds.createBridgeScopeSelect),
    createWizardNext: page.getByTestId(bridgeOperatorTestIds.createBridgeWizardNext),
    deleteSecret: (bindingName: string) => page.getByTestId(`delete-bridge-secret-${bindingName}`),
    detailPanel: page.getByTestId(bridgeOperatorTestIds.bridgeDetailPanel),
    detailOverflow: page.getByTestId(bridgeOperatorTestIds.detailOverflow),
    disableBridgeButton: page.getByTestId(bridgeOperatorTestIds.disableBridgeButton),
    editBridgeButton: page.getByTestId(bridgeOperatorTestIds.editBridgeButton),
    editDialog: page.getByTestId(bridgeOperatorTestIds.bridgeEditDialog),
    editDisplayNameInput: page.getByTestId("bridge-edit-display-name-input"),
    editProviderConfigInput: page.getByTestId("bridge-edit-provider-config-input"),
    enableBridgeButton: page.getByTestId(bridgeOperatorTestIds.enableBridgeButton),
    item: (id: string) => page.getByTestId(`bridge-item-${id}`),
    addListFilter: page.getByTestId(bridgeOperatorTestIds.bridgeListFiltersAdd),
    listPanel: page.getByTestId(bridgeOperatorTestIds.bridgeListPanel),
    manifestHandoff: page.getByTestId(bridgeOperatorTestIds.bridgeManifestHandoff),
    manifestJson: page.getByTestId(bridgeOperatorTestIds.bridgeManifestJson),
    manifestOpenBridge: page.getByTestId(bridgeOperatorTestIds.bridgeManifestOpenBridge),
    editModeAdvanced: page.getByTestId(bridgeOperatorTestIds.bridgeEditModeAdvanced),
    openTestDeliveryButton: page.getByTestId(bridgeOperatorTestIds.openTestDeliveryButton),
    providerCard: (providerKey: string) => page.getByTestId(`bridge-provider-card-${providerKey}`),
    restartBridgeButton: page.getByTestId(bridgeOperatorTestIds.restartBridgeButton),
    restartRequired: page.getByTestId(bridgeOperatorTestIds.bridgeRestartRequired),
    route: (sessionId: string) => page.getByTestId(`bridge-route-${sessionId}`),
    saveSecret: (bindingName: string) => page.getByTestId(`save-bridge-secret-${bindingName}`),
    searchInput: page.getByTestId(bridgeOperatorTestIds.bridgeSearchInput),
    secretBinding: (bindingName: string) =>
      page.getByTestId(`bridge-secret-binding-${bindingName}`),
    secretCheck: (bindingName: string, check: string) =>
      page.getByTestId(`bridge-secret-check-${bindingName}-${check}`),
    secretEnvInput: (bindingName: string) =>
      page.getByTestId(`bridge-secret-env-input-${bindingName}`),
    deliveryTestPanel: page.getByTestId(bridgeOperatorTestIds.bridgeDeliveryTestPanel),
    sendTestResult: page.getByTestId(bridgeOperatorTestIds.bridgeSendTestResult),
    setupChecklist: page.getByTestId(bridgeOperatorTestIds.bridgeSetupChecklist),
    setupItem: (id: string) => page.getByTestId(`bridge-setup-item-${id}`),
    submitBridgeCreate: page.getByTestId(bridgeOperatorTestIds.submitBridgeCreate),
    submitBridgeEdit: page.getByTestId(bridgeOperatorTestIds.submitBridgeEdit),
    submitSendTest: page.getByTestId(bridgeOperatorTestIds.submitSendTest),
    submitTestDelivery: page.getByTestId(bridgeOperatorTestIds.submitTestDelivery),
    testDeliveryMessage: page.getByTestId(bridgeOperatorTestIds.testDeliveryMessage),
    testDeliveryModeSelect: page.getByTestId(bridgeOperatorTestIds.testDeliveryModeSelect),
    testDeliveryPeerInput: page.getByTestId(bridgeOperatorTestIds.testDeliveryPeerInput),
    testDeliveryResult: page.getByTestId(bridgeOperatorTestIds.bridgeTestDeliveryResult),
    testDeliveryThreadInput: page.getByTestId(bridgeOperatorTestIds.testDeliveryThreadInput),
    verifyBridgeButton: page.getByTestId(bridgeOperatorTestIds.verifyBridgeButton),
    workspaceOnboarding: page.getByTestId(bridgeOperatorTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(bridgeOperatorTestIds.workspaceUseGlobal),
  };
}
export function settingsOperatorSelectors(
  page: Pick<Page, "getByTestId" | "locator">
): SettingsOperatorSelectors {
  return {
    shell: {
      shell: page.getByTestId(settingsShellTestIds.shell),
      shellOutlet: page.getByTestId(settingsShellTestIds.shellOutlet),
      sectionNav: page.getByTestId(settingsShellTestIds.sectionNav),
      sectionItems: page.locator(
        '[data-testid="settings-section-nav"] a[data-testid^="settings-section-"]'
      ),
      sectionLink: (slug: string) => page.getByTestId(`settings-section-${slug}`),
      sectionActive: (slug: string) =>
        page.locator(`[data-testid="settings-section-${slug}"][aria-current="page"]`),
    },
    general: {
      page: page.getByTestId(settingsGeneralTestIds.page),
      restartDismiss: page.getByTestId(settingsGeneralTestIds.restartDismiss),
      restartNotice: page.getByTestId(settingsGeneralTestIds.restartNotice),
      restartTrigger: page.getByTestId(settingsGeneralTestIds.restartTrigger),
      saveBar: page.getByTestId(settingsGeneralTestIds.saveBar),
      saveButton: page.getByTestId(settingsGeneralTestIds.saveButton),
      resetButton: page.getByTestId(settingsGeneralTestIds.resetButton),
      sessionTimeoutInput: page.getByTestId(settingsGeneralTestIds.sessionTimeoutInput),
    },
    skills: {
      page: page.getByTestId(settingsSkillsTestIds.page),
      disabledList: page.getByTestId(settingsSkillsTestIds.disabledList),
      disabledMessage: page.getByTestId(settingsSkillsTestIds.disabledMessage),
      disabledSave: page.getByTestId(settingsSkillsTestIds.disabledSave),
      disabledToggle: (name: string) =>
        page.getByTestId(`settings-page-skills-disabled-toggle-${name}`),
      operationalLink: page.getByTestId(settingsSkillsTestIds.operationalLink),
      policyRegistryInput: page.getByTestId(settingsSkillsTestIds.policyRegistryInput),
      policyBaseURLInput: page.getByTestId(settingsSkillsTestIds.policyBaseURLInput),
      save: page.getByTestId(settingsSkillsTestIds.save),
      restartNotice: page.getByTestId(settingsSkillsTestIds.restartNotice),
    },
    roles: {
      page: page.getByTestId(settingsRolesTestIds.page),
      saveBar: page.getByTestId(settingsRolesTestIds.saveBar),
      saveButton: page.getByTestId(settingsRolesTestIds.saveButton),
      resetButton: page.getByTestId(settingsRolesTestIds.resetButton),
      saveMessage: page.getByTestId(settingsRolesTestIds.saveMessage),
      group: (role: string) => page.getByTestId(`settings-page-roles-group-${role}`),
      toggle: (role: string) => page.getByTestId(`settings-page-roles-${role}-toggle`),
      routeSummary: (role: string) => page.getByTestId(`settings-page-roles-${role}-route`),
      runtimeSelect: (role: string) =>
        page.getByTestId(`settings-page-roles-${role}-runtime-select`),
      runtimeClear: (role: string) => page.getByTestId(`settings-page-roles-${role}-runtime-clear`),
      agentSelect: (role: string) => page.getByTestId(`settings-page-roles-${role}-agent-select`),
      fieldInput: (role: string, field: string) =>
        page.getByTestId(`settings-page-roles-${role}-${field}-input`),
      enabledSwitch: (role: string) =>
        page.getByTestId(`settings-page-roles-${role}-enabled-switch`),
      diagnostics: (role: string) => page.getByTestId(`settings-page-roles-${role}-diagnostics`),
      fallbackAdd: (role: string) =>
        page.getByTestId(`settings-page-roles-${role}-advanced-fallback-add`),
      fallbackEntrySelect: (role: string, index: number) =>
        page.getByTestId(`${role}.fallback.${index}-select`),
    },
    providers: {
      page: page.getByTestId(settingsProvidersTestIds.page),
      list: page.getByTestId(settingsProvidersTestIds.list),
      create: page.getByTestId(settingsProvidersTestIds.create),
      actionResult: page.getByTestId(settingsProvidersTestIds.actionResult),
      actionResultDismiss: page.getByTestId(settingsProvidersTestIds.actionResultDismiss),
      editor: page.getByTestId(settingsProvidersTestIds.editor),
      editorEdit: page.getByTestId(settingsProvidersTestIds.editorEdit),
      editorDelete: page.getByTestId(settingsProvidersTestIds.editorDelete),
      editorNameInput: page.getByTestId(settingsProvidersTestIds.editorNameInput),
      editorCommandInput: page.getByTestId(settingsProvidersTestIds.editorCommandInput),
      editorModelInput: page.getByTestId(settingsProvidersTestIds.editorModelInput),
      editorModeSimple: page.getByTestId(settingsProvidersTestIds.editorModeSimple),
      editorModeAdvanced: page.getByTestId(settingsProvidersTestIds.editorModeAdvanced),
      editorSave: page.getByTestId(settingsProvidersTestIds.editorSave),
      deleteDialog: page.getByTestId(settingsProvidersTestIds.deleteDialog),
      deleteConfirm: page.getByTestId(settingsProvidersTestIds.deleteConfirm),
      restartNotice: page.getByTestId(settingsProvidersTestIds.restartNotice),
      card: (name: string) => page.getByTestId(`settings-page-providers-card-${name}`),
      cardCommand: (name: string) =>
        page.getByTestId(`settings-page-providers-card-${name}-command`),
      inspectorSource: page.getByTestId("inspect-source"),
    },
    mcpServers: {
      page: page.getByTestId(settingsMCPServersTestIds.page),
      list: page.getByTestId(settingsMCPServersTestIds.list),
      create: page.getByTestId(settingsMCPServersTestIds.create),
      actionResult: page.locator("[data-sonner-toast]:last-of-type"),
      actionResultDismiss: page.getByTestId(settingsMCPServersTestIds.actionResultDismiss),
      scopeGlobal: page.getByTestId(settingsMCPServersTestIds.scopeGlobal),
      scopeWorkspace: page.getByTestId(settingsMCPServersTestIds.scopeWorkspace),
      scopeLabel: page.getByTestId(settingsMCPServersTestIds.scopeLabel),
      editor: page.getByTestId(settingsMCPServersTestIds.editor),
      editorNameInput: page.getByTestId(settingsMCPServersTestIds.editorNameInput),
      editorCommandInput: page.getByTestId(settingsMCPServersTestIds.editorCommandInput),
      editorTargetInput: page.getByTestId(settingsMCPServersTestIds.editorTargetInput),
      editorSave: page.getByTestId(settingsMCPServersTestIds.editorSave),
      editorRemove: page.getByTestId(settingsMCPServersTestIds.editorRemove),
      deleteDialog: page.getByTestId(settingsMCPServersTestIds.deleteDialog),
      deleteConfirm: page.getByTestId(settingsMCPServersTestIds.deleteConfirm),
      row: (name: string) => page.getByTestId(`marketplace-installed-card-${name}`),
      rowSource: (name: string) =>
        page.locator(`[data-testid="marketplace-installed-card-${name}"] [data-slot="pill"]`),
      editRow: (name: string) =>
        page.locator(
          `[data-testid="marketplace-installed-card-${name}"] button[aria-label^="More for"]`
        ),
    },
    hooks: {
      page: page.getByTestId(settingsHooksTestIds.page),
      hooksList: page.getByTestId(settingsHooksTestIds.hooksList),
      restartNotice: page.getByTestId(settingsHooksTestIds.restartNotice),
      hookToggle: (name: string) => page.getByTestId(`settings-page-hooks-row-${name}-toggle`),
    },
    extensions: {
      allowUnverified: page.getByTestId(settingsExtensionsTestIds.allowUnverified),
      page: page.getByTestId(settingsExtensionsTestIds.page),
      policyControls: page.getByTestId(settingsExtensionsTestIds.policyControls),
      policyRegistryInput: page.getByTestId(settingsExtensionsTestIds.policyRegistryInput),
      policyBaseURLInput: page.getByTestId(settingsExtensionsTestIds.policyBaseURLInput),
      save: page.getByTestId(settingsExtensionsTestIds.save),
      restartNotice: page.getByTestId(settingsExtensionsTestIds.restartNotice),
    },
  };
}

export function tasksOperatorSelectors(
  page: Pick<Page, "getByRole" | "getByTestId">,
  portalRoot: Pick<Page, "getByTestId"> = page
): TasksOperatorSelectors {
  const windowPath = page.getByRole("navigation", { name: "Window path" });

  return {
    osDesktop: page.getByTestId(tasksOperatorTestIds.osDesktop),
    createDescription: page.getByTestId(tasksOperatorTestIds.createDescription),
    createEditorSurface: page.getByTestId(tasksOperatorTestIds.createEditorSurface),
    createModeAdvanced: page.getByTestId(tasksOperatorTestIds.createModeAdvanced),
    createModeSimple: page.getByTestId(tasksOperatorTestIds.createModeSimple),
    createPriority: (priority: string) => page.getByTestId(`task-priority-${priority}`),
    createSaveDraft: page.getByTestId(tasksOperatorTestIds.createSaveDraft),
    createScopeGlobal: page.getByTestId(tasksOperatorTestIds.createScopeGlobal),
    createScopeWorkspace: page.getByTestId(tasksOperatorTestIds.createScopeWorkspace),
    createSubmit: page.getByTestId(tasksOperatorTestIds.createSubmit),
    createTemplate: (templateId: string) => page.getByTestId(`task-template-${templateId}`),
    createTitle: page.getByTestId(tasksOperatorTestIds.createTitle),
    createWorkspaceOption: (workspaceId: string) =>
      page.getByTestId(`task-workspace-item-${workspaceId}`),
    createWorkspaceSelect: page.getByTestId(tasksOperatorTestIds.createWorkspaceSelect),
    dashboardActiveRun: (runId: string) => page.getByTestId(`tasks-dashboard-active-run-${runId}`),
    dashboardActiveRunLink: (runId: string) =>
      page.getByTestId(`tasks-dashboard-active-run-link-${runId}`),
    dashboardView: page.getByTestId(tasksOperatorTestIds.dashboardView),
    detailActiveRunChannel: page.getByTestId(tasksOperatorTestIds.detailActiveRunChannel),
    detailActiveRunEmpty: page.getByTestId(tasksOperatorTestIds.detailActiveRunEmpty),
    detailActiveRunEmptyHint: page.getByTestId(tasksOperatorTestIds.detailActiveRunEmptyHint),
    detailApprovalPill: page.getByTestId(tasksOperatorTestIds.detailApprovalPill),
    detailBreadcrumbTasks: windowPath.getByRole("button", { exact: true, name: "Tasks" }),
    detailContent: page.getByTestId(tasksOperatorTestIds.detailContent),
    detailTitle: page.getByTestId(tasksOperatorTestIds.detailTitle),
    detailInspectDrawer: page.getByTestId(tasksOperatorTestIds.detailInspectDrawer),
    detailInspectStream: page.getByTestId(tasksOperatorTestIds.detailInspectStream),
    detailCoordination: page.getByTestId(tasksOperatorTestIds.detailCoordination),
    detailCancel: page.getByTestId(tasksOperatorTestIds.detailCancel),
    detailDelete: portalRoot.getByTestId(tasksOperatorTestIds.detailDelete),
    detailDeleteCancel: portalRoot.getByTestId(tasksOperatorTestIds.detailDeleteCancel),
    detailDeleteConfirm: portalRoot.getByTestId(tasksOperatorTestIds.detailDeleteConfirm),
    detailDeleteDialog: portalRoot.getByTestId(tasksOperatorTestIds.detailDeleteDialog),
    detailEdit: portalRoot.getByTestId(tasksOperatorTestIds.detailEdit),
    detailEnqueue: page.getByTestId(tasksOperatorTestIds.detailEnqueue),
    detailOverflow: page.getByTestId(tasksOperatorTestIds.detailOverflow),
    detailNowApproval: page.getByTestId(tasksOperatorTestIds.detailNowApproval),
    detailNowRun: page.getByTestId(tasksOperatorTestIds.detailNowRun),
    detailPublish: page.getByTestId(tasksOperatorTestIds.detailPublish),
    detailStatus: page.getByTestId(tasksOperatorTestIds.detailStatus),
    detailPreviewCoordination: page.getByTestId(tasksOperatorTestIds.detailPreviewCoordination),
    detailPreviewDeeplink: page.getByTestId(tasksOperatorTestIds.detailPreviewDeeplink),
    detailPreviewLifecycle: page.getByTestId(tasksOperatorTestIds.detailPreviewLifecycle),
    detailPreviewPanel: page.getByTestId(tasksOperatorTestIds.detailPreviewPanel),
    detailPreviewPublish: page.getByTestId(tasksOperatorTestIds.detailPreviewPublish),
    detailRunsChannel: (runId: string) => page.getByTestId(`tasks-detail-runs-channel-${runId}`),
    detailRunsEmpty: page.getByTestId(tasksOperatorTestIds.detailRunsEmpty),
    detailTab: (tabId: string) => page.getByTestId(`tasks-detail-tab-${tabId}`),
    detailTabRuns: page.getByTestId(tasksOperatorTestIds.detailTabRuns),
    detailSetupEdit: page.getByTestId(tasksOperatorTestIds.detailSetupEdit),
    detailSetupForm: page.getByTestId(tasksOperatorTestIds.detailSetupForm),
    detailSetupOpen: page.getByTestId(tasksOperatorTestIds.detailSetupOpen),
    detailSetupSheet: page.getByTestId(tasksOperatorTestIds.detailSetupSheet),
    detailSetupWorkerRuntime: page.getByTestId(tasksOperatorTestIds.detailSetupWorkerRuntime),
    detailChildItem: (taskId: string) => page.getByTestId(`tasks-detail-subtask-${taskId}`),
    detailChildLink: (taskId: string) => page.getByTestId(`tasks-detail-subtask-${taskId}`),
    detailDependencyItem: (taskId: string) => page.getByTestId(`tasks-detail-dependency-${taskId}`),
    detailDependencyLink: (taskId: string) => page.getByTestId(`tasks-detail-dependency-${taskId}`),
    inboxApprove: (taskId: string) => page.getByTestId(`tasks-inbox-item-approve-${taskId}`),
    inboxArchive: (taskId: string) => page.getByTestId(`tasks-inbox-item-archive-${taskId}`),
    inboxDismiss: (taskId: string) => page.getByTestId(`tasks-inbox-item-dismiss-${taskId}`),
    inboxItem: (taskId: string) => page.getByTestId(`tasks-inbox-item-${taskId}`),
    inboxLane: (lane: string) =>
      page.getByTestId(`tasks-inbox-group-${tasksInboxGroupByLane[lane] ?? lane}`),
    inboxOpenTask: (taskId: string) => page.getByTestId(`tasks-inbox-item-open-${taskId}`),
    inboxReject: (taskId: string) => page.getByTestId(`tasks-inbox-item-reject-${taskId}`),
    inboxRetry: (taskId: string) => page.getByTestId(`tasks-inbox-item-retry-${taskId}`),
    inboxView: page.getByTestId(tasksOperatorTestIds.inboxView),
    modeDashboard: page.getByTestId(tasksOperatorTestIds.modeDashboard),
    modeInbox: page.getByTestId(tasksOperatorTestIds.modeInbox),
    modeKanban: page.getByTestId(tasksOperatorTestIds.modeKanban),
    modeList: page.getByTestId(tasksOperatorTestIds.modeList),
    multiAgentDisconnected: page.getByTestId(tasksOperatorTestIds.multiAgentDisconnected),
    multiAgentEmpty: page.getByTestId(tasksOperatorTestIds.multiAgentEmpty),
    multiAgentNoActive: page.getByTestId(tasksOperatorTestIds.multiAgentNoActive),
    multiAgentPanel: page.getByTestId(tasksOperatorTestIds.multiAgentPanel),
    multiAgentSummary: page.getByTestId(tasksOperatorTestIds.multiAgentSummary),
    multiAgentAgentLink: (taskId: string) =>
      page.getByTestId(`tasks-multi-agent-agent-link-${taskId}`),
    openCreate: page.getByTestId(tasksOperatorTestIds.openCreate),
    runDetailContent: page.getByTestId(tasksOperatorTestIds.runDetailContent),
    runDetailCancel: portalRoot.getByTestId(tasksOperatorTestIds.runDetailCancel),
    runDetailOverflow: page.getByTestId(tasksOperatorTestIds.runDetailOverflow),
    runReviews: page.getByTestId(tasksOperatorTestIds.runReviews),
    runsRow: (runId: string) => page.getByTestId(`tasks-runs-row-${runId}`),
    runReviewRow: (reviewId: string) => page.getByTestId(`tasks-run-review-${reviewId}`),
    runSessionDrilldown: page.getByTestId(tasksOperatorTestIds.runSessionDrilldown),
    taskCard: (taskId: string) => page.getByTestId(`task-card-${taskId}`),
    taskCardPublish: (taskId: string) => page.getByTestId(`task-card-publish-${taskId}`),
    workspaceOnboarding: page.getByTestId(tasksOperatorTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(tasksOperatorTestIds.workspaceUseGlobal),
  };
}
