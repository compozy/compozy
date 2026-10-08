import type { Locator, Page } from "@playwright/test";

// Shell-global surfaces: the desktop root and the first-run workspace onboarding.
// These legitimately live at page scope (they exist before any window opens).
export const sessionLifecycleTestIds = {
  osDesktop: "os-desktop",
  workspaceManualPathInput: "workspace-manual-path-input",
  workspaceRegisterManual: "workspace-register-manual",
} as const;

/**
 * Stable selectors for the daemon-backed desktop shell. Window and frame IDs
 * are authoritative opaque IDs, so callers must obtain them from the snapshot
 * rather than reconstructing an ID from an app name.
 */
export const osShellSelectors = (page: Page) => ({
  desktop: page.getByTestId(sessionLifecycleTestIds.osDesktop),
  deck: (frameId: string) => page.getByTestId(`os-window-deck-${frameId}`),
  frame: (frameId: string) => page.getByTestId(`os-window-frame-${frameId}`),
  tab: (windowId: string) => page.getByTestId(`os-window-tab-${windowId}`),
  tabButton: (windowId: string) =>
    page.getByTestId(`os-window-tab-${windowId}`).locator('[data-slot="os-window-tab-activate"]'),
  tabMenu: (windowId: string) => page.getByTestId(`os-window-tab-menu-${windowId}`),
  window: (windowId: string) => page.getByTestId(`os-window-${windowId}`),
});

export const SESSION_CREATE_FIRST_MESSAGE = "e2e first message";

// Session-window surfaces. Every one of these renders inside an owning window
// selected by the session `instance_key` and must be scoped to it — a page-level
// match would resolve to a second session window (strict-mode violation).
export const sessionWindowTestIds = {
  chatView: "chat-view",
  composerAttachButton: "composer-attach-button",
  composerAttachmentGate: "composer-attachment-gate",
  composerAttachmentRemove: "composer-attachment-remove",
  composerAttachmentStrip: "composer-attachment-strip",
  composerAttachmentTile: "composer-attachment-tile",
  composerClearButton: "composer-clear-button",
  composerDropOverlay: "composer-drop-overlay",
  composerQueuedAttachmentWell: "composer-queued-attachment-well",
  composerEnterHint: "composer-enter-hint",
  composerFeedbackNote: "composer-feedback-note",
  composerInterruptButton: "composer-interrupt-button",
  composerQueueButton: "composer-queue-button",
  composerSendButton: "composer-send-button",
  composerSteerButton: "composer-steer-button",
  composerStopButton: "composer-stop-button",
  deleteButton: "delete-button",
  permissionAllowAlways: "permission-allow-always",
  permissionAllowOnce: "permission-allow-once",
  permissionPrompt: "permission-dock",
  processingIndicator: "processing-indicator",
  resumeButton: "resume-button",
  stopButton: "stop-button",
  topbarOverflow: "session-topbar-overflow",
  userMessageAttachmentFileCard: "user-message-attachment-file-card",
  userMessageAttachmentFrame: "user-message-attachment-frame",
  userMessageAttachmentGallery: "user-message-attachment-gallery",
} as const;

export interface SessionLifecycleSelectors {
  agentPageNewSession: Locator;
  agentRow(agentName: string): Locator;
  osDesktop: Locator;
  workspaceManualPathInput: Locator;
  workspaceRegisterManual: Locator;
}

export interface SessionWindowSelectors {
  chatView: Locator;
  composerAttachButton: Locator;
  composerAttachmentGate: Locator;
  composerAttachmentRemove: Locator;
  composerAttachmentStrip: Locator;
  composerAttachmentTile: Locator;
  composerClearButton: Locator;
  composerDropOverlay: Locator;
  composerEnterHint: Locator;
  composerFeedbackNote: Locator;
  composerInterruptButton: Locator;
  composerQueueButton: Locator;
  composerQueuedAttachmentWell: Locator;
  composerSendButton: Locator;
  composerSteerButton: Locator;
  composerStopButton: Locator;
  composerTextarea: Locator;
  deleteButton: Locator;
  permissionAllowAlways: Locator;
  permissionAllowOnce: Locator;
  permissionPrompt: Locator;
  processingIndicator: Locator;
  resumeButton: Locator;
  stopButton: Locator;
  topbarOverflow: Locator;
  userMessageAttachmentFileCard: Locator;
  userMessageAttachmentFrame: Locator;
  userMessageAttachmentGallery: Locator;
}

export const automationOperatorTestIds = {
  osDesktop: sessionLifecycleTestIds.osDesktop,
  automationDetailPanel: "automation-detail-panel",
  automationEditorDialog: "automation-editor-dialog",
  automationForm: "automation-form",
  automationRunList: "automation-run-list",
  automationSuggestionsCard: "automation-suggestions-card",
  automationDeleteDialog: "automation-delete-dialog",
  automationDeleteConfirmTyping: "automation-delete-confirm-typing",
  confirmDeleteAutomationButton: "confirm-delete-automation-btn",
  automationsCreate: "automations-create",
  automationsListRows: "automations-list-rows",
  automationsShell: "automations-shell",
  automationStartViews: "automation-start-views",
  deleteAutomationButton: "automation-delete-btn",
  detailOverflow: "automation-detail-overflow",
  editAutomationButton: "automation-edit-btn",
  // Editor (one form for jobs and triggers).
  agentInput: "automation-agent-input",
  conditionAdd: "automation-condition-add",
  enabledToggle: "automation-enabled-toggle",
  fireLimitMax: "automation-fire-limit-max",
  fireLimitWindow: "automation-fire-limit-window",
  formSubmit: "automation-form-submit",
  nameInput: "automation-name-input",
  optionsToggle: "automation-options-toggle",
  previewToggle: "automation-preview-toggle",
  promptInput: "automation-prompt-input",
  retryBackoff: "automation-retry-backoff",
  retryMax: "automation-retry-max",
  retryNone: "automation-retry-none",
  scheduleModeAt: "automation-schedule-mode-at",
  scheduleModeCron: "automation-schedule-mode-cron",
  scheduleModeEvery: "automation-schedule-mode-every",
  webhookId: "automation-webhook-id",
  webhookSecret: "automation-webhook-secret",
  webhookSlug: "automation-webhook-slug",
  // Detail page (one grammar for jobs and triggers).
  detailRunNow: "automation-run-now-btn",
  detailSentence: "automation-detail-sentence",
  detailSubhead: "automation-detail-subhead",
  enableLabel: "automation-enable-label",
  enableSwitch: "automation-enable-switch",
  inspectButton: "automation-inspect-btn",
  inspectSheet: "automation-inspect-sheet",
  rail: "automation-rail",
  railReliability: "automation-rail-reliability",
  ruleStarts: "automation-rule-starts",
  ruleOnlyIf: "automation-rule-only-if",
} as const;

export const marketplaceOperatorTestIds = {
  extensionAutomationStarted: "extension-automation-started",
  extensionEnvironmentState: "extension-environment-state",
  extensionFormatBadge: "extension-format-badge",
  extensionKitInventory: "extension-kit-inventory",
  extensionKitInventoryItem: "extension-kit-inventory-item",
  extensionSkippedComponents: "extension-skipped-components",
  extensionSkippedRow: "extension-skipped-row",
  extensionSkippedZeroResources: "extension-skipped-zero-resources",
  extensionInstallAllowUnverified: "extension-install-allow-unverified",
  extensionInstallDialog: "extension-install-dialog",
  extensionInstallError: "extension-install-error",
  extensionInstallRef: "extension-install-ref",
  extensionInstallRefError: "extension-install-ref-error",
  extensionInstallSubmit: "extension-install-submit",
  extensionDevBadge: "extension-dev-badge",
  extensionLogsFollow: "extension-logs-follow",
  extensionLogsLines: "extension-logs-lines",
  extensionLogsPanel: "extension-logs-panel",
  extensionLogsStatus: "extension-logs-status",
  extensionOriginPath: "extension-origin-path",
  extensionOverridesPublishedBadge: "extension-overrides-published-badge",
  detail: "marketplace-detail",
  detailAction: "marketplace-detail-action",
  extensionTrustConfirm: "extension-trust-confirm",
  extensionTrustDialog: "extension-trust-dialog",
  grid: "marketplace-grid",
  refresh: "marketplace-refresh",
} as const;

export interface AutomationOperatorSelectors {
  osDesktop: Locator;
  automationSuggestionsCard: Locator;
  automationDeleteConfirmTyping: Locator;
  automationDeleteDialog: Locator;
  confirmDeleteAutomationButton: Locator;
  automationsCreate: Locator;
  automationsListRows: Locator;
  automationsShell: Locator;
  /** Start view pill: `all` · `schedule` · `event`. */
  automationStartView(start: "all" | "schedule" | "event"): Locator;
  /** Listing row by automation name (unique per kind; both kinds match when names collide). */
  automationRow(name: string): Locator;
  automationSwitch(id: string): Locator;
  deleteAutomationButton: Locator;
  detailOverflow: Locator;
  detailPanel: Locator;
  editAutomationButton: Locator;
  item(id: string): Locator;
  suggestion(id: string): Locator;
  editorDialog: Locator;
  form: Locator;
  agentInput: Locator;
  conditionAdd: Locator;
  conditionField(index: number): Locator;
  conditionValue(index: number): Locator;
  doesChoice(does: "agent" | "loop" | "task"): Locator;
  enabledToggle: Locator;
  eventOption(eventId: string): Locator;
  fireLimitMax: Locator;
  fireLimitWindow: Locator;
  formSubmit: Locator;
  nameInput: Locator;
  optionsToggle: Locator;
  previewToggle: Locator;
  promptInput: Locator;
  retryBackoff: Locator;
  retryMax: Locator;
  retryNone: Locator;
  scheduleExpr: Locator;
  scheduleExpressionToggle: Locator;
  scheduleInterval: Locator;
  scheduleModeAt: Locator;
  scheduleModeCron: Locator;
  scheduleModeEvery: Locator;
  scheduleTime: Locator;
  /** Editor Starts card, scoped to the dialog (the listing's Start views share the prefix). */
  startChoice(start: "schedule" | "event" | "webhook"): Locator;
  webhookId: Locator;
  webhookSecret: Locator;
  webhookSlug: Locator;
  itemLink(id: string): Locator;
  run(id: string): Locator;
  runList: Locator;
  runNow(id: string): Locator;
  detailRunNow: Locator;
  detailSentence: Locator;
  detailSubhead: Locator;
  enableLabel: Locator;
  enableSwitch: Locator;
  inspectButton: Locator;
  inspectSheet: Locator;
  rail: Locator;
  railReliability: Locator;
  ruleStarts: Locator;
  ruleOnlyIf: Locator;
  runDrawer(runId: string): Locator;
  runRetries(runId: string): Locator;
  runOpenLink(runId: string): Locator;
}

export interface MarketplaceOperatorSelectors {
  action(entryId: string): Locator;
  extensionDevBadge: Locator;
  extensionInstallAllowUnverified: Locator;
  extensionInstallDialog: Locator;
  extensionInstallError: Locator;
  extensionInstallRef: Locator;
  extensionInstallRefError: Locator;
  extensionInstallSubmit: Locator;
  extensionInstallSource(source: string): Locator;
  extensionLogsFollow: Locator;
  extensionLogsLines: Locator;
  extensionLogsPanel: Locator;
  extensionLogsStatus: Locator;
  extensionOriginPath: Locator;
  extensionOverridesPublishedBadge: Locator;
  extensionAutomationStarted: Locator;
  extensionEnvironmentState: Locator;
  extensionFormatBadge: Locator;
  extensionKitInventory: Locator;
  extensionKitInventoryItem: Locator;
  extensionSkippedComponents: Locator;
  extensionSkippedRow: Locator;
  extensionSkippedZeroResources: Locator;
  card(entryId: string): Locator;
  detail: Locator;
  detailAction: Locator;
  extensionTrustConfirm: Locator;
  extensionTrustDialog: Locator;
  grid: Locator;
  refresh: Locator;
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
  followUpGroup: "settings-page-general-follow-up-group",
  restartNotice: "settings-page-general-restart-notice",
  restartTrigger: "settings-page-general-restart-trigger",
  restartDismiss: "settings-page-general-restart-dismiss",
  updates: "settings-page-general-updates",
  updateStatus: "settings-page-general-update-status",
  updateRetry: "settings-page-general-update-retry",
  updateRecommendation: "settings-page-general-update-recommendation",
  updateLastError: "settings-page-general-update-last-error",
  updateBlocked: "settings-page-general-update-blocked",
  updateRollback: "settings-page-general-update-rollback",
  updateCancel: "settings-page-general-update-cancel",
} as const;

export const settingsSkillsTestIds = {
  page: "settings-page-skills",
  disabledList: "settings-page-skills-disabled-list",
  disabledMessage: "settings-page-skills-disabled-message",
  disabledSave: "settings-page-skills-disabled-save",
  save: "settings-page-skills-save",
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
  page: "settings-page-mcp",
  list: "settings-page-mcp-servers-list",
  create: "settings-page-mcp-create",
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
  gitEnabled: "settings-page-extensions-policy-git-enabled-input",
  githubBaseURLInput: "settings-page-extensions-policy-github-base-url-input",
  githubEnabled: "settings-page-extensions-policy-github-enabled-input",
  page: "settings-page-extensions",
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
  followUpGroup: Locator;
  followUpOption(mode: "steer" | "queue"): Locator;
  updates: Locator;
  updateStatus: Locator;
  updateRetry: Locator;
  updateRecommendation: Locator;
  updateLastError: Locator;
  updateBlocked: Locator;
  updateRollback: Locator;
  updateCancel: Locator;
  /** Per-track row, shared apply affordance, live progress, and release link. */
  updateTrack: (target: string) => Locator;
  updateApply: () => Locator;
  updateProgress: (target: string) => Locator;
  updateRelease: (target: string) => Locator;
}

interface SettingsSkillsSelectors {
  page: Locator;
  disabledList: Locator;
  disabledMessage: Locator;
  disabledSave: Locator;
  disabledToggle(name: string): Locator;
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
  inspectorCommand: Locator;
  inspectorSource: Locator;
  inspectorTechnical: Locator;
  restartNotice: Locator;
}

interface SettingsMCPServersSelectors {
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
}

interface SettingsHooksSelectors {
  hooksList: Locator;
  hookToggle(name: string): Locator;
  page: Locator;
  restartNotice: Locator;
}

interface SettingsExtensionsSelectors {
  allowUnverified: Locator;
  gitEnabled: Locator;
  githubBaseURLInput: Locator;
  githubEnabled: Locator;
  page: Locator;
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
  createSubmit: "task-editor-modal-submit",
  createTitle: "task-title-input",
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
  createSubmit: Locator;
  createTemplate(templateId: string): Locator;
  createTitle: Locator;
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
}
export function sessionLifecycleSelectors(
  page: Pick<Page, "getByRole" | "getByTestId">
): SessionLifecycleSelectors {
  return {
    agentPageNewSession: page.getByTestId("agent-page-new-session"),
    agentRow: (agentName: string) => page.getByTestId(`agent-fleet-row-link-${agentName}`),
    osDesktop: page.getByTestId(sessionLifecycleTestIds.osDesktop),
    workspaceManualPathInput: page.getByTestId(sessionLifecycleTestIds.workspaceManualPathInput),
    workspaceRegisterManual: page.getByTestId(sessionLifecycleTestIds.workspaceRegisterManual),
  };
}

/**
 * Controls that live inside one session window. `win` is REQUIRED — pass the
 * owning instance locator (`sessionWindow(page, id)` from `./os-navigation`) so
 * every locator resolves within that window and never matches a second session
 * window at page scope.
 */
export function sessionWindowSelectors(
  win: Locator,
  portalRoot: Pick<Page, "getByTestId"> = win
): SessionWindowSelectors {
  return {
    chatView: win.getByTestId(sessionWindowTestIds.chatView),
    composerAttachButton: win.getByTestId(sessionWindowTestIds.composerAttachButton),
    composerAttachmentGate: win.getByTestId(sessionWindowTestIds.composerAttachmentGate),
    composerAttachmentRemove: win.getByTestId(sessionWindowTestIds.composerAttachmentRemove),
    composerAttachmentStrip: win.getByTestId(sessionWindowTestIds.composerAttachmentStrip),
    composerAttachmentTile: win.getByTestId(sessionWindowTestIds.composerAttachmentTile),
    composerClearButton: portalRoot.getByTestId(sessionWindowTestIds.composerClearButton),
    composerDropOverlay: win.getByTestId(sessionWindowTestIds.composerDropOverlay),
    composerEnterHint: win.getByTestId(sessionWindowTestIds.composerEnterHint),
    composerFeedbackNote: win.getByTestId(sessionWindowTestIds.composerFeedbackNote),
    composerInterruptButton: win.getByTestId(sessionWindowTestIds.composerInterruptButton),
    composerQueueButton: win.getByTestId(sessionWindowTestIds.composerQueueButton),
    composerQueuedAttachmentWell: win.getByTestId(
      sessionWindowTestIds.composerQueuedAttachmentWell
    ),
    composerSendButton: win.getByRole("button", { name: "Send message" }),
    composerSteerButton: win.getByTestId(sessionWindowTestIds.composerSteerButton),
    composerStopButton: win.getByTestId(sessionWindowTestIds.composerStopButton),
    composerTextarea: win.getByRole("textbox", { name: "Session prompt" }),
    deleteButton: portalRoot.getByTestId(sessionWindowTestIds.deleteButton),
    permissionAllowAlways: win.getByTestId(sessionWindowTestIds.permissionAllowAlways),
    permissionAllowOnce: win.getByTestId(sessionWindowTestIds.permissionAllowOnce),
    permissionPrompt: win.getByTestId(sessionWindowTestIds.permissionPrompt),
    processingIndicator: win.getByTestId(sessionWindowTestIds.processingIndicator),
    resumeButton: win.getByTestId(sessionWindowTestIds.resumeButton),
    stopButton: win.getByTestId(sessionWindowTestIds.stopButton),
    topbarOverflow: win.getByTestId(sessionWindowTestIds.topbarOverflow),
    userMessageAttachmentFileCard: win.getByTestId(
      sessionWindowTestIds.userMessageAttachmentFileCard
    ),
    userMessageAttachmentFrame: win.getByTestId(sessionWindowTestIds.userMessageAttachmentFrame),
    userMessageAttachmentGallery: win.getByTestId(
      sessionWindowTestIds.userMessageAttachmentGallery
    ),
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
  card: "clarification-dock",
  question: "clarification-dock-question",
  choice: "clarification-dock-choice",
  textInput: "clarification-dock-text-input",
  submit: "clarification-dock-submit",
  receipt: "clarification-receipt",
  receiptAnswer: "clarification-receipt-answer",
  error: "clarification-dock-error",
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
      page.locator(`[data-testid="${sessionClarifyTestIds.choice}"][data-choice-index="${index}"]`),
  };
}

/** Routed confirmation shown when a session deep link belongs to another workspace (ADR-004). */
export const sessionWorkspaceSwitchTestIds = {
  dialog: "session-workspace-switch-dialog",
  confirm: "session-workspace-switch-confirm",
  cancel: "session-workspace-switch-cancel",
} as const;

export interface SessionWorkspaceSwitchSelectors {
  dialog: Locator;
  confirm: Locator;
  cancel: Locator;
}

export function sessionWorkspaceSwitchSelectors(
  page: Pick<Page, "getByTestId">
): SessionWorkspaceSwitchSelectors {
  return {
    dialog: page.getByTestId(sessionWorkspaceSwitchTestIds.dialog),
    confirm: page.getByTestId(sessionWorkspaceSwitchTestIds.confirm),
    cancel: page.getByTestId(sessionWorkspaceSwitchTestIds.cancel),
  };
}

export function marketplaceOperatorSelectors(
  page: Pick<Page, "getByTestId">
): MarketplaceOperatorSelectors {
  return {
    action: (entryId: string) => page.getByTestId(`marketplace-action-${entryId}`),
    extensionAutomationStarted: page.getByTestId(
      marketplaceOperatorTestIds.extensionAutomationStarted
    ),
    extensionEnvironmentState: page.getByTestId(
      marketplaceOperatorTestIds.extensionEnvironmentState
    ),
    extensionFormatBadge: page.getByTestId(marketplaceOperatorTestIds.extensionFormatBadge),
    extensionKitInventory: page.getByTestId(marketplaceOperatorTestIds.extensionKitInventory),
    extensionKitInventoryItem: page.getByTestId(
      marketplaceOperatorTestIds.extensionKitInventoryItem
    ),
    extensionSkippedComponents: page.getByTestId(
      marketplaceOperatorTestIds.extensionSkippedComponents
    ),
    extensionSkippedRow: page.getByTestId(marketplaceOperatorTestIds.extensionSkippedRow),
    extensionSkippedZeroResources: page.getByTestId(
      marketplaceOperatorTestIds.extensionSkippedZeroResources
    ),
    card: (entryId: string) => page.getByTestId(`marketplace-card-${entryId}`),
    detail: page.getByTestId(marketplaceOperatorTestIds.detail),
    extensionDevBadge: page.getByTestId(marketplaceOperatorTestIds.extensionDevBadge),
    extensionInstallAllowUnverified: page.getByTestId(
      marketplaceOperatorTestIds.extensionInstallAllowUnverified
    ),
    extensionInstallDialog: page.getByTestId(marketplaceOperatorTestIds.extensionInstallDialog),
    extensionInstallError: page.getByTestId(marketplaceOperatorTestIds.extensionInstallError),
    extensionInstallRef: page.getByTestId(marketplaceOperatorTestIds.extensionInstallRef),
    extensionInstallRefError: page.getByTestId(marketplaceOperatorTestIds.extensionInstallRefError),
    extensionInstallSource: (source: string) =>
      page.getByTestId(`extension-install-source-${source}`),
    extensionInstallSubmit: page.getByTestId(marketplaceOperatorTestIds.extensionInstallSubmit),
    extensionLogsFollow: page.getByTestId(marketplaceOperatorTestIds.extensionLogsFollow),
    extensionLogsLines: page.getByTestId(marketplaceOperatorTestIds.extensionLogsLines),
    extensionLogsPanel: page.getByTestId(marketplaceOperatorTestIds.extensionLogsPanel),
    extensionLogsStatus: page.getByTestId(marketplaceOperatorTestIds.extensionLogsStatus),
    extensionOriginPath: page.getByTestId(marketplaceOperatorTestIds.extensionOriginPath),
    extensionOverridesPublishedBadge: page.getByTestId(
      marketplaceOperatorTestIds.extensionOverridesPublishedBadge
    ),
    detailAction: page.getByTestId(marketplaceOperatorTestIds.detailAction),
    extensionTrustConfirm: page.getByTestId(marketplaceOperatorTestIds.extensionTrustConfirm),
    extensionTrustDialog: page.getByTestId(marketplaceOperatorTestIds.extensionTrustDialog),
    grid: page.getByTestId(marketplaceOperatorTestIds.grid),
    refresh: page.getByTestId(marketplaceOperatorTestIds.refresh),
  };
}

/** Listing row of either daemon entity for an automation id. */
function automationRowTestId(id: string): RegExp {
  return new RegExp(`^automation-row-(job|trigger)-${id.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`);
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
    automationsCreate: page.getByTestId(automationOperatorTestIds.automationsCreate),
    automationsListRows: page.getByTestId(automationOperatorTestIds.automationsListRows),
    automationsShell: page.getByTestId(automationOperatorTestIds.automationsShell),
    automationStartView: start => page.getByTestId(`automation-start-${start}`),
    automationRow: name =>
      page
        .getByTestId(/^automation-row-(job|trigger)-/)
        .filter({ has: page.getByRole("link", { name: `Open ${name}`, exact: true }) }),
    automationSwitch: id => page.getByTestId(`automation-switch-${id}`),
    deleteAutomationButton: portalRoot.getByTestId(
      automationOperatorTestIds.deleteAutomationButton
    ),
    detailOverflow: page.getByTestId(automationOperatorTestIds.detailOverflow),
    detailPanel: page.getByTestId(automationOperatorTestIds.automationDetailPanel),
    editAutomationButton: portalRoot.getByTestId(automationOperatorTestIds.editAutomationButton),
    editorDialog,
    item: (id: string) => page.getByTestId(automationRowTestId(id)),
    itemLink: (id: string) => page.getByTestId(automationRowTestId(id)).getByRole("link"),
    form: page.getByTestId(automationOperatorTestIds.automationForm),
    agentInput: page.getByTestId(automationOperatorTestIds.agentInput),
    conditionAdd: page.getByTestId(automationOperatorTestIds.conditionAdd),
    conditionField: (index: number) => page.getByTestId(`automation-condition-field-${index}`),
    conditionValue: (index: number) => page.getByTestId(`automation-condition-value-${index}`),
    doesChoice: does => editorDialog.getByTestId(`automation-does-${does}`),
    enabledToggle: page.getByTestId(automationOperatorTestIds.enabledToggle),
    eventOption: (eventId: string) => page.getByTestId(`automation-event-${eventId}`),
    fireLimitMax: page.getByTestId(automationOperatorTestIds.fireLimitMax),
    fireLimitWindow: page.getByTestId(automationOperatorTestIds.fireLimitWindow),
    formSubmit: page.getByTestId(automationOperatorTestIds.formSubmit),
    nameInput: page.getByTestId(automationOperatorTestIds.nameInput),
    optionsToggle: page.getByTestId(automationOperatorTestIds.optionsToggle),
    previewToggle: page.getByTestId(automationOperatorTestIds.previewToggle),
    promptInput: page.getByTestId(automationOperatorTestIds.promptInput),
    retryBackoff: page.getByTestId(automationOperatorTestIds.retryBackoff),
    retryMax: page.getByTestId(automationOperatorTestIds.retryMax),
    retryNone: page.getByTestId(automationOperatorTestIds.retryNone),
    scheduleExpr: editorDialog.getByLabel("Cron expression"),
    scheduleExpressionToggle: editorDialog.getByRole("button", { name: "Edit expression" }),
    scheduleInterval: editorDialog.getByLabel("Interval"),
    scheduleModeAt: page.getByTestId(automationOperatorTestIds.scheduleModeAt),
    scheduleModeCron: page.getByTestId(automationOperatorTestIds.scheduleModeCron),
    scheduleModeEvery: page.getByTestId(automationOperatorTestIds.scheduleModeEvery),
    scheduleTime: editorDialog.getByLabel("Date and time"),
    startChoice: start => editorDialog.getByTestId(`automation-start-${start}`),
    webhookId: page.getByTestId(automationOperatorTestIds.webhookId),
    webhookSecret: page.getByTestId(automationOperatorTestIds.webhookSecret),
    webhookSlug: page.getByTestId(automationOperatorTestIds.webhookSlug),
    run: (id: string) => page.getByTestId(`automation-run-${id}`),
    runDrawer: (runId: string) => page.getByTestId(`automation-run-drawer-${runId}`),
    runList: page.getByTestId(automationOperatorTestIds.automationRunList),
    runNow: (id: string) => page.getByTestId(`automation-run-now-${id}`),
    suggestion: (id: string) => page.getByTestId(`automation-suggestion-${id}`),
    // Runs open from the expanded drawer; a link exists only when the daemon recorded
    // the id it points at.
    runOpenLink: (runId: string) => page.getByTestId(`automation-run-open-${runId}`),
    runRetries: (runId: string) => page.getByTestId(`automation-run-retries-${runId}`),
    detailRunNow: page.getByTestId(automationOperatorTestIds.detailRunNow),
    detailSentence: page.getByTestId(automationOperatorTestIds.detailSentence),
    detailSubhead: page.getByTestId(automationOperatorTestIds.detailSubhead),
    enableLabel: page.getByTestId(automationOperatorTestIds.enableLabel),
    enableSwitch: page.getByTestId(automationOperatorTestIds.enableSwitch),
    inspectButton: page.getByTestId(automationOperatorTestIds.inspectButton),
    inspectSheet: portalRoot.getByTestId(automationOperatorTestIds.inspectSheet),
    rail: page.getByTestId(automationOperatorTestIds.rail),
    railReliability: page.getByTestId(automationOperatorTestIds.railReliability),
    ruleStarts: page.getByTestId(automationOperatorTestIds.ruleStarts),
    ruleOnlyIf: page.getByTestId(automationOperatorTestIds.ruleOnlyIf),
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
      followUpGroup: page.getByTestId(settingsGeneralTestIds.followUpGroup),
      followUpOption: (mode: "steer" | "queue") =>
        page.getByTestId(`settings-page-general-follow-up-${mode}`),
      updates: page.getByTestId(settingsGeneralTestIds.updates),
      updateStatus: page.getByTestId(settingsGeneralTestIds.updateStatus),
      updateRetry: page.getByTestId(settingsGeneralTestIds.updateRetry),
      updateRecommendation: page.getByTestId(settingsGeneralTestIds.updateRecommendation),
      updateLastError: page.getByTestId(settingsGeneralTestIds.updateLastError),
      updateBlocked: page.getByTestId(settingsGeneralTestIds.updateBlocked),
      updateRollback: page.getByTestId(settingsGeneralTestIds.updateRollback),
      updateCancel: page.getByTestId(settingsGeneralTestIds.updateCancel),
      updateTrack: (target: string) =>
        page.getByTestId(`settings-page-general-update-track-${target}`),
      updateApply: () => page.getByTestId("settings-page-general-update-apply"),
      updateProgress: (target: string) =>
        page.getByTestId(`settings-page-general-update-progress-${target}`),
      updateRelease: (target: string) =>
        page.getByTestId(`settings-page-general-update-release-${target}`),
    },
    skills: {
      page: page.getByTestId(settingsSkillsTestIds.page),
      disabledList: page.getByTestId(settingsSkillsTestIds.disabledList),
      disabledMessage: page.getByTestId(settingsSkillsTestIds.disabledMessage),
      disabledSave: page.getByTestId(settingsSkillsTestIds.disabledSave),
      disabledToggle: (name: string) =>
        page.getByTestId(`settings-page-skills-disabled-toggle-${name}`),
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
      inspectorCommand: page.getByTestId("inspect-command"),
      inspectorSource: page.getByTestId("inspect-source"),
      inspectorTechnical: page.getByTestId("provider-detail-technical"),
    },
    mcpServers: {
      page: page.getByTestId(settingsMCPServersTestIds.page),
      list: page.getByTestId(settingsMCPServersTestIds.list),
      create: page.getByTestId(settingsMCPServersTestIds.create),
      editor: page.getByTestId(settingsMCPServersTestIds.editor),
      editorNameInput: page.getByTestId(settingsMCPServersTestIds.editorNameInput),
      editorCommandInput: page.getByTestId(settingsMCPServersTestIds.editorCommandInput),
      editorTargetInput: page.getByTestId(settingsMCPServersTestIds.editorTargetInput),
      editorSave: page.getByTestId(settingsMCPServersTestIds.editorSave),
      editorRemove: page.getByTestId(settingsMCPServersTestIds.editorRemove),
      deleteDialog: page.getByTestId(settingsMCPServersTestIds.deleteDialog),
      deleteConfirm: page.getByTestId(settingsMCPServersTestIds.deleteConfirm),
      row: (name: string) => page.getByTestId(`settings-page-mcp-servers-row-${name}`),
      rowSource: (name: string) => page.getByTestId(`settings-page-mcp-servers-row-${name}-source`),
      editRow: (name: string) => page.getByTestId(`settings-page-mcp-servers-row-${name}-edit`),
    },
    hooks: {
      page: page.getByTestId(settingsHooksTestIds.page),
      hooksList: page.getByTestId(settingsHooksTestIds.hooksList),
      restartNotice: page.getByTestId(settingsHooksTestIds.restartNotice),
      hookToggle: (name: string) => page.getByTestId(`settings-page-hooks-row-${name}-toggle`),
    },
    extensions: {
      allowUnverified: page.getByTestId(settingsExtensionsTestIds.allowUnverified),
      gitEnabled: page.getByTestId(settingsExtensionsTestIds.gitEnabled),
      githubBaseURLInput: page.getByTestId(settingsExtensionsTestIds.githubBaseURLInput),
      githubEnabled: page.getByTestId(settingsExtensionsTestIds.githubEnabled),
      page: page.getByTestId(settingsExtensionsTestIds.page),
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
    createSubmit: page.getByTestId(tasksOperatorTestIds.createSubmit),
    createTemplate: (templateId: string) => page.getByTestId(`task-template-${templateId}`),
    createTitle: page.getByTestId(tasksOperatorTestIds.createTitle),
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
  };
}

/**
 * Profiles — the menubar switcher, the Settings page, and the lifecycle dialogs.
 *
 * Dialogs render into the window's overlay layer, so a caller scopes them to the
 * owning window exactly as it does for every other windowed surface.
 */
export const profilesTestIds = {
  switcher: "os-menubar-profile",
  switcherMenu: "os-menubar-profile-menu",
  switcherAll: "profile-switcher-all",
  switcherCreate: "profile-switcher-create",
  switcherSettingsLink: "profile-switcher-settings-link",
  page: "settings-page-profiles",
  pageLine: "settings-page-profiles-line",
  pageArchived: "settings-page-profiles-archived",
  pageSelectionMap: "settings-page-profiles-selection-map",
  activeList: "profiles-active-list",
  archivedList: "profiles-archived-list",
  createOpen: "profile-create-open",
  createDialog: "profile-create-dialog",
  createName: "profile-create-name-input",
  createConfirm: "profile-create-confirm",
  createPicker: "profile-create-symbol-picker",
  identityDialog: "profile-identity-dialog",
  identityConfirm: "profile-identity-confirm",
  identityPicker: "profile-identity-symbol-picker",
  renameDialog: "profile-rename-dialog",
  renameName: "profile-rename-name-input",
  renameConfirm: "profile-rename-confirm",
  renamePlan: "profile-rename-plan",
  renameDormant: "profile-rename-dormant",
  archiveDialog: "profile-archive-dialog",
  archiveConfirm: "profile-archive-confirm",
  archivePaused: "profile-archive-paused",
  archiveBlocked: "profile-archive-blocked",
  unarchiveDialog: "profile-unarchive-dialog",
  unarchiveConfirm: "profile-unarchive-confirm",
  unarchivePaused: "profile-unarchive-paused",
  deleteDialog: "profile-delete-dialog",
  deleteConfirm: "profile-delete-confirm",
  deleteEnumeration: "profile-delete-enumeration",
  deleteArchiveInstead: "profile-delete-archive-instead",
} as const;

export interface ProfilesOperatorSelectors {
  switcher: Locator;
  switcherMenu: Locator;
  switcherAll: Locator;
  switcherCreate: Locator;
  switcherOption: (name: string) => Locator;
  page: Locator;
  pageLine: Locator;
  pageArchived: Locator;
  pageSelectionMap: Locator;
  activeList: Locator;
  archivedList: Locator;
  row: (name: string) => Locator;
  editIdentityRow: (name: string) => Locator;
  renameRow: (name: string) => Locator;
  archiveRow: (name: string) => Locator;
  unarchiveRow: (name: string) => Locator;
  deleteRow: (name: string) => Locator;
  needsSetup: (name: string) => Locator;
  createOpen: Locator;
  createDialog: Locator;
  createName: Locator;
  createConfirm: Locator;
  createPicker: Locator;
  identityDialog: Locator;
  identityConfirm: Locator;
  identityPicker: Locator;
  renameDialog: Locator;
  renameName: Locator;
  renameConfirm: Locator;
  renamePlan: Locator;
  renameDormant: Locator;
  renameRepo: (workspaceId: string) => Locator;
  archiveDialog: Locator;
  archiveConfirm: Locator;
  archivePaused: Locator;
  archiveBlocked: Locator;
  unarchiveDialog: Locator;
  unarchiveConfirm: Locator;
  unarchivePaused: Locator;
  deleteDialog: Locator;
  deleteConfirm: Locator;
  deleteEnumeration: Locator;
  deleteArchiveInstead: Locator;
  paletteRow: (name: string) => Locator;
  paletteCreate: Locator;
  paletteAggregate: Locator;
  /** Owner tags — present only in aggregate mode, except on worktree rows. */
  ownerTag: (name: string) => Locator;
  ownerTags: Locator;
  /** The fixed "→ default" chip on a shared creation surface (ADR-005). */
  destinationChip: Locator;
  ownerBanner: Locator;
  ownerBannerSwitch: Locator;
  usageProfileShare: Locator;
}

/**
 * `page` addresses menubar and palette chrome; `scope` addresses the windowed
 * Settings surface and the dialogs it owns. They differ because the menubar is
 * page-level and a settings window is not.
 */
export function profilesOperatorSelectors(
  page: Page,
  scope: Page | Locator = page
): ProfilesOperatorSelectors {
  return {
    switcher: page.getByTestId(profilesTestIds.switcher),
    switcherMenu: page.getByTestId(profilesTestIds.switcherMenu),
    switcherAll: page.getByTestId(profilesTestIds.switcherAll),
    switcherCreate: page.getByTestId(profilesTestIds.switcherCreate),
    switcherOption: (name: string) => page.getByTestId(`profile-switcher-option-${name}`),
    page: scope.getByTestId(profilesTestIds.page),
    pageLine: scope.getByTestId(profilesTestIds.pageLine),
    pageArchived: scope.getByTestId(profilesTestIds.pageArchived),
    pageSelectionMap: scope.getByTestId(profilesTestIds.pageSelectionMap),
    activeList: scope.getByTestId(profilesTestIds.activeList),
    archivedList: scope.getByTestId(profilesTestIds.archivedList),
    row: (name: string) => scope.getByTestId(`profile-row-${name}`),
    editIdentityRow: (name: string) => scope.getByTestId(`profile-edit-identity-${name}`),
    renameRow: (name: string) => scope.getByTestId(`profile-rename-${name}`),
    archiveRow: (name: string) => scope.getByTestId(`profile-archive-${name}`),
    unarchiveRow: (name: string) => scope.getByTestId(`profile-unarchive-${name}`),
    deleteRow: (name: string) => scope.getByTestId(`profile-delete-${name}`),
    needsSetup: (name: string) => scope.getByTestId(`profile-needs-setup-${name}`),
    createOpen: scope.getByTestId(profilesTestIds.createOpen),
    createDialog: page.getByTestId(profilesTestIds.createDialog),
    createName: page.getByTestId(profilesTestIds.createName),
    createConfirm: page.getByTestId(profilesTestIds.createConfirm),
    createPicker: page.getByTestId(profilesTestIds.createPicker),
    identityDialog: page.getByTestId(profilesTestIds.identityDialog),
    identityConfirm: page.getByTestId(profilesTestIds.identityConfirm),
    identityPicker: page.getByTestId(profilesTestIds.identityPicker),
    renameDialog: page.getByTestId(profilesTestIds.renameDialog),
    renameName: page.getByTestId(profilesTestIds.renameName),
    renameConfirm: page.getByTestId(profilesTestIds.renameConfirm),
    renamePlan: page.getByTestId(profilesTestIds.renamePlan),
    renameDormant: page.getByTestId(profilesTestIds.renameDormant),
    renameRepo: (workspaceId: string) => page.getByTestId(`profile-rename-repo-${workspaceId}`),
    archiveDialog: page.getByTestId(profilesTestIds.archiveDialog),
    archiveConfirm: page.getByTestId(profilesTestIds.archiveConfirm),
    archivePaused: page.getByTestId(profilesTestIds.archivePaused),
    archiveBlocked: page.getByTestId(profilesTestIds.archiveBlocked),
    unarchiveDialog: page.getByTestId(profilesTestIds.unarchiveDialog),
    unarchiveConfirm: page.getByTestId(profilesTestIds.unarchiveConfirm),
    unarchivePaused: page.getByTestId(profilesTestIds.unarchivePaused),
    deleteDialog: page.getByTestId(profilesTestIds.deleteDialog),
    deleteConfirm: page.getByTestId(profilesTestIds.deleteConfirm),
    deleteEnumeration: page.getByTestId(profilesTestIds.deleteEnumeration),
    deleteArchiveInstead: page.getByTestId(profilesTestIds.deleteArchiveInstead),
    paletteRow: (name: string) => page.getByTestId(`os-palette-profile-${name}`),
    paletteCreate: page.getByTestId("os-palette-profile-create"),
    paletteAggregate: page.getByTestId("os-palette-profile-aggregate"),
    ownerTag: (name: string) => scope.getByTestId("profile-owner-tag").filter({ hasText: name }),
    ownerTags: scope.getByTestId("profile-owner-tag"),
    destinationChip: scope.getByTestId("profile-destination-chip"),
    ownerBanner: page.getByTestId("profile-owner-banner"),
    ownerBannerSwitch: page.getByTestId("profile-owner-banner-switch"),
    usageProfileShare: scope.getByTestId("home-profile-share"),
  };
}
