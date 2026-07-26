// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import type { Locator } from "@playwright/test";

import {
  automationOperatorSelectors,
  automationOperatorTestIds,
  bridgeOperatorSelectors,
  bridgeOperatorTestIds,
  marketplaceOperatorSelectors,
  marketplaceOperatorTestIds,
  networkOperatorSelectors,
  networkOperatorTestIds,
  settingsExtensionsTestIds,
  settingsGeneralTestIds,
  settingsHooksTestIds,
  settingsMCPServersTestIds,
  settingsOperatorSelectors,
  settingsProvidersTestIds,
  settingsShellTestIds,
  settingsSkillsTestIds,
  sandboxOperatorSelectors,
  sandboxOperatorTestIds,
  sessionLifecycleSelectors,
  sessionLifecycleTestIds,
  sessionWindowSelectors,
  sessionWindowTestIds,
  tasksOperatorSelectors,
  tasksOperatorTestIds,
} from "../selectors";

describe("session lifecycle selectors", () => {
  it("maps the shell-global onboarding and agent-fleet surfaces to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const getByRole = vi.fn(
      (role: string, options?: { name: string }) =>
        `role:${role}:${options?.name}` as unknown as Locator
    );
    const selectors = sessionLifecycleSelectors({
      getByRole,
      getByTestId,
    });

    expect(selectors.osDesktop).toBe(`locator:${sessionLifecycleTestIds.osDesktop}`);
    expect(selectors.workspaceOnboarding).toBe(
      `locator:${sessionLifecycleTestIds.workspaceOnboarding}`
    );
    expect(selectors.workspaceUseGlobal).toBe(
      `locator:${sessionLifecycleTestIds.workspaceUseGlobal}`
    );
    expect(selectors.workspaceManualPathInput).toBe(
      `locator:${sessionLifecycleTestIds.workspaceManualPathInput}`
    );
    expect(selectors.workspaceRegisterManual).toBe(
      `locator:${sessionLifecycleTestIds.workspaceRegisterManual}`
    );
    expect(selectors.agentRow("browser-lifecycle-agent")).toBe(
      "locator:agent-fleet-row-link-browser-lifecycle-agent"
    );
    expect(selectors.agentPageNewSession).toBe("locator:agent-page-new-session");
  });
});

describe("session window selectors", () => {
  it("scopes in-window controls and resolves overflow actions from the owning portal", () => {
    const getByTestId = vi.fn((testId: string) => `win-locator:${testId}` as unknown as Locator);
    const portalGetByTestId = vi.fn(
      (testId: string) => `portal-locator:${testId}` as unknown as Locator
    );
    const getByRole = vi.fn(
      (role: string, options?: { name: string }) =>
        `win-role:${role}:${options?.name}` as unknown as Locator
    );
    const selectors = sessionWindowSelectors(
      {
        getByRole,
        getByTestId,
      } as unknown as Locator,
      { getByTestId: portalGetByTestId }
    );

    expect(selectors.chatView).toBe(`win-locator:${sessionWindowTestIds.chatView}`);
    expect(selectors.composerClearButton).toBe(
      `portal-locator:${sessionWindowTestIds.composerClearButton}`
    );
    expect(selectors.composerSendButton).toBe("win-role:button:Send message");
    expect(selectors.composerTextarea).toBe("win-role:textbox:Session prompt");
    expect(selectors.deleteButton).toBe(`portal-locator:${sessionWindowTestIds.deleteButton}`);
    expect(selectors.permissionAllowAlways).toBe(
      `win-locator:${sessionWindowTestIds.permissionAllowAlways}`
    );
    expect(selectors.permissionAllowOnce).toBe(
      `win-locator:${sessionWindowTestIds.permissionAllowOnce}`
    );
    expect(selectors.permissionPrompt).toBe(`win-locator:${sessionWindowTestIds.permissionPrompt}`);
    expect(selectors.processingIndicator).toBe(
      `win-locator:${sessionWindowTestIds.processingIndicator}`
    );
    expect(selectors.resumeButton).toBe(`win-locator:${sessionWindowTestIds.resumeButton}`);
    expect(selectors.stopButton).toBe(`win-locator:${sessionWindowTestIds.stopButton}`);
    expect(selectors.topbarOverflow).toBe(`win-locator:${sessionWindowTestIds.topbarOverflow}`);
  });
});

describe("network operator selectors", () => {
  it("maps the network navigation, dialog, lists, and detail surfaces to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const locator = vi.fn((selector: string) => `css:${selector}` as unknown as Locator);
    const selectors = networkOperatorSelectors({
      getByTestId,
      locator,
    });

    expect(selectors.workspace).toBe(`locator:${networkOperatorTestIds.workspace}`);
    expect(selectors.channelHeader).toBe(`locator:${networkOperatorTestIds.channelHeader}`);
    expect(selectors.channelTabs).toBe(`locator:${networkOperatorTestIds.channelTabs}`);
    expect(selectors.threadTab).toBe(`locator:${networkOperatorTestIds.threadTab}`);
    expect(selectors.directTab).toBe(`locator:${networkOperatorTestIds.directTab}`);
    expect(selectors.threadList).toBe(`locator:${networkOperatorTestIds.threadList}`);
    expect(selectors.directList).toBe(`locator:${networkOperatorTestIds.directList}`);
    expect(selectors.threadOverlay).toBe(`locator:${networkOperatorTestIds.threadOverlay}`);
    expect(selectors.directRoom).toBe(`locator:${networkOperatorTestIds.directRoom}`);
    expect(selectors.newDirectButton).toBe(`locator:${networkOperatorTestIds.newDirectButton}`);
    expect(selectors.newDirectDialog).toBe(`locator:${networkOperatorTestIds.newDirectDialog}`);
    expect(selectors.channelNameInput).toBe(`locator:${networkOperatorTestIds.channelNameInput}`);
    expect(selectors.messageList).toBe(`locator:${networkOperatorTestIds.messageList}`);
    expect(selectors.agentOption("mock-ops-coordinator")).toBe(
      "locator:network-agent-option-mock-ops-coordinator"
    );
    expect(selectors.channelItem("builders")).toBe("locator:network-channel-row-builders");
    expect(selectors.threadItem("thread_main")).toBe("locator:network-thread-list-row-thread_main");
    expect(selectors.directItem("direct_abc")).toBe("locator:network-direct-list-row-direct_abc");
    expect(selectors.newDirectPeer("peer_ops")).toBe("locator:network-new-direct-peer-peer_ops");
    expect(selectors.channelMessage("browser_msg_say_01")).toBe(
      'css:[data-testid="network-message-row-full"][data-message-id="browser_msg_say_01"], [data-testid="network-message-row-collapsed"][data-message-id="browser_msg_say_01"], [data-testid="network-message-row-system"][data-message-id="browser_msg_say_01"]'
    );
  });
});

describe("automation operator selectors", () => {
  it("maps the jobs/triggers navigation, editor, detail, and run-history surfaces to stable test IDs", () => {
    const getByLabel = vi.fn((label: string) => `label:${label}` as unknown as Locator);
    const getByRole = vi.fn(
      (role: string, options?: { name?: string | RegExp }) =>
        `role:${role}:${String(options?.name)}` as unknown as Locator
    );
    const editorDialog = {
      getByLabel: vi.fn((label: string) => `editor-label:${label}` as unknown as Locator),
      getByRole: vi.fn(
        (role: string, options?: { name?: string | RegExp }) =>
          `editor-role:${role}:${String(options?.name)}` as unknown as Locator
      ),
    } as unknown as Locator;
    const getByTestId = vi.fn((testId: string) =>
      testId === automationOperatorTestIds.automationEditorDialog
        ? editorDialog
        : (`locator:${testId}` as unknown as Locator)
    );
    const portalGetByTestId = vi.fn(
      (testId: string) => `portal-locator:${testId}` as unknown as Locator
    );
    const selectors = automationOperatorSelectors(
      {
        getByLabel,
        getByRole,
        getByTestId,
      },
      { getByTestId: portalGetByTestId }
    );

    expect(selectors.jobsShell).toBe(`locator:${automationOperatorTestIds.jobsShell}`);
    expect(selectors.automationSuggestionsCard).toBe(
      `locator:${automationOperatorTestIds.automationSuggestionsCard}`
    );
    expect(selectors.triggersShell).toBe(`locator:${automationOperatorTestIds.triggersShell}`);
    expect(selectors.jobsListRows).toBe(`locator:${automationOperatorTestIds.jobsListRows}`);
    expect(selectors.triggersListRows).toBe(
      `locator:${automationOperatorTestIds.triggersListRows}`
    );
    expect(selectors.createJobButton).toBe(`locator:${automationOperatorTestIds.createJobButton}`);
    expect(selectors.suggestion("suggestion-1")).toBe("locator:automation-suggestion-suggestion-1");
    expect(selectors.createTriggerButton).toBe(
      `locator:${automationOperatorTestIds.createTriggerButton}`
    );
    expect(selectors.automationDeleteDialog).toBe(
      `locator:${automationOperatorTestIds.automationDeleteDialog}`
    );
    expect(selectors.automationDeleteConfirmTyping).toBe(
      `locator:${automationOperatorTestIds.automationDeleteConfirmTyping}`
    );
    expect(selectors.confirmDeleteAutomationButton).toBe(
      `locator:${automationOperatorTestIds.confirmDeleteAutomationButton}`
    );
    expect(selectors.detailPanel).toBe(
      `locator:${automationOperatorTestIds.automationDetailPanel}`
    );
    expect(selectors.editAutomationButton).toBe(
      `portal-locator:${automationOperatorTestIds.editAutomationButton}`
    );
    expect(selectors.toggleAutomationButton).toBe(
      `portal-locator:${automationOperatorTestIds.toggleAutomationButton}`
    );
    expect(selectors.deleteAutomationButton).toBe(
      `portal-locator:${automationOperatorTestIds.deleteAutomationButton}`
    );
    expect(selectors.jobForm).toBe(`locator:${automationOperatorTestIds.automationJobForm}`);
    expect(selectors.jobNameInput).toBe(`locator:${automationOperatorTestIds.jobNameInput}`);
    expect(selectors.jobScheduleExpr).toBe("editor-label:Cron expression");
    expect(selectors.jobScheduleCustom).toBe("editor-role:button:Custom");
    expect(selectors.jobGovernanceToggle).toBe(
      `locator:${automationOperatorTestIds.jobGovernanceToggle}`
    );
    expect(selectors.submitJobForm).toBe(`locator:${automationOperatorTestIds.submitJobForm}`);
    expect(selectors.runHistory).toBe(`locator:${automationOperatorTestIds.automationRunHistory}`);
    expect(selectors.triggerJobButton).toBe(
      `locator:${automationOperatorTestIds.triggerJobButton}`
    );
    expect(selectors.triggerEventOption("webhook")).toBe("locator:trigger-event-webhook");
    expect(selectors.triggerFilterAdd).toBe("role:button:Add condition");
    expect(selectors.triggerFilterKey(0)).toBe("locator:trigger-filter-key-0");
    expect(selectors.triggerFilterValue(0)).toBe("locator:trigger-filter-value-0");
    expect(selectors.item("job_daily_review")).toBe("locator:automation-item-job_daily_review");
    expect(selectors.run("run_001")).toBe("locator:automation-run-run_001");
    expect(selectors.runSessionLink("run_001")).toBe("locator:automation-run-run_001");
  });
});

describe("bridge operator selectors", () => {
  it("maps the bridge list, edit, secret-binding, and test-delivery surfaces to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const getByRoleWithinWindowPath = vi.fn(
      (role: string, options?: { name: string }) =>
        `breadcrumb-role:${role}:${options?.name}` as unknown as Locator
    );
    const getByRole = vi.fn((role: string, options?: { name: string }) =>
      role === "navigation" && options?.name === "Window path"
        ? ({ getByRole: getByRoleWithinWindowPath } as unknown as Locator)
        : (`role:${role}:${options?.name}` as unknown as Locator)
    );
    const selectors = bridgeOperatorSelectors({
      getByRole,
      getByTestId,
    });

    expect(selectors.listPanel).toBe(`locator:${bridgeOperatorTestIds.bridgeListPanel}`);
    expect(selectors.detailPanel).toBe(`locator:${bridgeOperatorTestIds.bridgeDetailPanel}`);
    expect(selectors.backToList).toBe("breadcrumb-role:button:Bridges");
    expect(selectors.createDialog).toBe(`locator:${bridgeOperatorTestIds.bridgeCreateDialog}`);
    expect(selectors.createDisplayNameInput).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeDisplayNameInput}`
    );
    expect(selectors.createProviderConfigInput).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeProviderConfigInput}`
    );
    expect(selectors.createProviderConfigError).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeProviderConfigError}`
    );
    expect(selectors.createScopeSelect).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeScopeSelect}`
    );
    expect(selectors.createDeliveryModeSelect).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeDeliveryModeSelect}`
    );
    expect(selectors.createDeliveryPeerInput).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeDeliveryPeerInput}`
    );
    expect(selectors.createDeliveryThreadInput).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeDeliveryThreadInput}`
    );
    expect(selectors.createRoutingIncludePeer).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeRoutingIncludePeer}`
    );
    expect(selectors.createRoutingIncludeThread).toBe(
      `locator:${bridgeOperatorTestIds.createBridgeRoutingIncludeThread}`
    );
    expect(selectors.submitBridgeCreate).toBe(
      `locator:${bridgeOperatorTestIds.submitBridgeCreate}`
    );
    expect(selectors.editDialog).toBe(`locator:${bridgeOperatorTestIds.bridgeEditDialog}`);
    expect(selectors.editBridgeButton).toBe(`locator:${bridgeOperatorTestIds.editBridgeButton}`);
    expect(selectors.disableBridgeButton).toBe(
      `locator:${bridgeOperatorTestIds.disableBridgeButton}`
    );
    expect(selectors.enableBridgeButton).toBe(
      `locator:${bridgeOperatorTestIds.enableBridgeButton}`
    );
    expect(selectors.restartBridgeButton).toBe(
      `locator:${bridgeOperatorTestIds.restartBridgeButton}`
    );
    expect(selectors.restartRequired).toBe(
      `locator:${bridgeOperatorTestIds.bridgeRestartRequired}`
    );
    expect(selectors.addListFilter).toBe(`locator:${bridgeOperatorTestIds.bridgeListFiltersAdd}`);
    expect(selectors.activeRoutesMetric).toBe(
      `locator:${bridgeOperatorTestIds.bridgeMetricActiveRoutes}`
    );
    expect(selectors.openTestDeliveryButton).toBe(
      `locator:${bridgeOperatorTestIds.openTestDeliveryButton}`
    );
    expect(selectors.deliveryTestPanel).toBe(
      `locator:${bridgeOperatorTestIds.bridgeDeliveryTestPanel}`
    );
    expect(selectors.testDeliveryResult).toBe(
      `locator:${bridgeOperatorTestIds.bridgeTestDeliveryResult}`
    );
    expect(selectors.item("brg_browser")).toBe("locator:bridge-item-brg_browser");
    expect(selectors.providerCard("telegram-reference::telegram")).toBe(
      "locator:bridge-provider-card-telegram-reference::telegram"
    );
    expect(selectors.secretBinding("bot_token")).toBe("locator:bridge-secret-binding-bot_token");
    expect(selectors.secretEnvInput("bot_token")).toBe("locator:bridge-secret-env-input-bot_token");
    expect(selectors.saveSecret("bot_token")).toBe("locator:save-bridge-secret-bot_token");
    expect(selectors.deleteSecret("bot_token")).toBe("locator:delete-bridge-secret-bot_token");
    expect(selectors.route("sess_bridge_01")).toBe("locator:bridge-route-sess_bridge_01");
  });
});

describe("marketplace operator selectors", () => {
  it("maps acquisition, detail, and overlay surfaces to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const selectors = marketplaceOperatorSelectors({ getByTestId });

    expect(selectors.kindNavigation).toBe(`locator:${marketplaceOperatorTestIds.kindNavigation}`);
    expect(selectors.detail).toBe(`locator:${marketplaceOperatorTestIds.detail}`);
    expect(selectors.detailAction).toBe(`locator:${marketplaceOperatorTestIds.detailAction}`);
    expect(selectors.mcpInstallDialog).toBe(
      `locator:${marketplaceOperatorTestIds.mcpInstallDialog}`
    );
    expect(selectors.mcpInstallConfirm).toBe(
      `locator:${marketplaceOperatorTestIds.mcpInstallConfirm}`
    );
    expect(selectors.bundleActivationDialog).toBe(
      `locator:${marketplaceOperatorTestIds.bundleActivationDialog}`
    );
    expect(selectors.bundleActivateConfirm).toBe(
      `locator:${marketplaceOperatorTestIds.bundleActivateConfirm}`
    );
    expect(selectors.extensionTrustDialog).toBe(
      `locator:${marketplaceOperatorTestIds.extensionTrustDialog}`
    );
    expect(selectors.extensionTrustConfirm).toBe(
      `locator:${marketplaceOperatorTestIds.extensionTrustConfirm}`
    );
    expect(selectors.card("browser-skill")).toBe("locator:marketplace-card-browser-skill");
    expect(selectors.action("browser-skill")).toBe("locator:marketplace-action-browser-skill");
    expect(selectors.kind("skill")).toBe("locator:marketplace-kind-skill");
    expect(selectors.mcpVaultSelector("BROWSER_TOKEN")).toBe(
      "locator:mcp-vault-selector-BROWSER_TOKEN"
    );
    expect(selectors.mcpCreateSecret("BROWSER_TOKEN")).toBe(
      "locator:mcp-create-secret-BROWSER_TOKEN"
    );
  });
});

describe("sandbox operator selectors", () => {
  it("maps the sandbox profile lifecycle surfaces to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const selectors = sandboxOperatorSelectors({
      getByTestId,
    });

    expect(selectors.osDesktop).toBe(`locator:${sandboxOperatorTestIds.osDesktop}`);
    expect(selectors.shell).toBe(`locator:${sandboxOperatorTestIds.shell}`);
    expect(selectors.total).toBe(`locator:${sandboxOperatorTestIds.total}`);
    expect(selectors.workspaceReferences).toBe(
      `locator:${sandboxOperatorTestIds.workspaceReferences}`
    );
    expect(selectors.createButton).toBe(`locator:${sandboxOperatorTestIds.createButton}`);
    expect(selectors.editor).toBe(`locator:${sandboxOperatorTestIds.editor}`);
    expect(selectors.editorNameInput).toBe(`locator:${sandboxOperatorTestIds.editorNameInput}`);
    expect(selectors.editorBackendInput).toBe(
      `locator:${sandboxOperatorTestIds.editorBackendInput}`
    );
    expect(selectors.editorSave).toBe(`locator:${sandboxOperatorTestIds.editorSave}`);
    expect(selectors.deleteDialog).toBe(`locator:${sandboxOperatorTestIds.deleteDialog}`);
    expect(selectors.deleteConfirm).toBe(`locator:${sandboxOperatorTestIds.deleteConfirm}`);
    expect(selectors.deleteUsage).toBe(`locator:${sandboxOperatorTestIds.deleteUsage}`);
    expect(selectors.actionResult).toBe(`locator:${sandboxOperatorTestIds.actionResult}`);
    expect(selectors.profile("browser-local-sandbox")).toBe(
      "locator:sandbox-page-card-browser-local-sandbox"
    );
    expect(selectors.profileMetadata("browser-local-sandbox")).toBe(
      "locator:sandbox-page-card-browser-local-sandbox-profile"
    );
    expect(selectors.editProfile("browser-local-sandbox")).toBe(
      "locator:sandbox-page-card-browser-local-sandbox-edit"
    );
    expect(selectors.deleteProfile("browser-local-sandbox")).toBe(
      "locator:sandbox-page-card-browser-local-sandbox-delete"
    );
  });
});

describe("settings operator selectors", () => {
  it("maps shell, restart-aware sections, collections, hooks, and extension policy to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const locator = vi.fn((selector: string) => `locator:${selector}` as unknown as Locator);
    const selectors = settingsOperatorSelectors({
      getByTestId,
      locator,
    });

    expect(selectors.shell.shell).toBe(`locator:${settingsShellTestIds.shell}`);
    expect(selectors.shell.sectionNav).toBe(`locator:${settingsShellTestIds.sectionNav}`);
    expect(selectors.shell.sectionLink("general")).toBe("locator:settings-section-general");
    expect(selectors.shell.sectionActive("network")).toBe(
      'locator:[data-testid="settings-section-network"][aria-current="page"]'
    );

    expect(selectors.general.page).toBe(`locator:${settingsGeneralTestIds.page}`);
    expect(selectors.general.saveButton).toBe(`locator:${settingsGeneralTestIds.saveButton}`);
    expect(selectors.general.restartNotice).toBe(`locator:${settingsGeneralTestIds.restartNotice}`);
    expect(selectors.general.restartTrigger).toBe(
      `locator:${settingsGeneralTestIds.restartTrigger}`
    );
    expect(selectors.general.sessionTimeoutInput).toBe(
      `locator:${settingsGeneralTestIds.sessionTimeoutInput}`
    );

    expect(selectors.skills.page).toBe(`locator:${settingsSkillsTestIds.page}`);
    expect(selectors.skills.disabledList).toBe(`locator:${settingsSkillsTestIds.disabledList}`);
    expect(selectors.skills.disabledToggle("browser-disabled-skill")).toBe(
      "locator:settings-page-skills-disabled-toggle-browser-disabled-skill"
    );
    expect(selectors.skills.policyRegistryInput).toBe(
      `locator:${settingsSkillsTestIds.policyRegistryInput}`
    );
    expect(selectors.skills.policyBaseURLInput).toBe(
      `locator:${settingsSkillsTestIds.policyBaseURLInput}`
    );
    expect(selectors.providers.page).toBe(`locator:${settingsProvidersTestIds.page}`);
    expect(selectors.providers.create).toBe(`locator:${settingsProvidersTestIds.create}`);
    expect(selectors.providers.editor).toBe(`locator:${settingsProvidersTestIds.editor}`);
    expect(selectors.providers.editorEdit).toBe(`locator:${settingsProvidersTestIds.editorEdit}`);
    expect(selectors.providers.editorDelete).toBe(
      `locator:${settingsProvidersTestIds.editorDelete}`
    );
    expect(selectors.providers.editorSave).toBe(`locator:${settingsProvidersTestIds.editorSave}`);
    expect(selectors.providers.card("codex")).toBe("locator:settings-page-providers-card-codex");
    expect(selectors.providers.cardCommand("codex")).toBe(
      "locator:settings-page-providers-card-codex-command"
    );
    expect(selectors.providers.inspectorSource).toBe("locator:inspect-source");

    expect(selectors.mcpServers.page).toBe(`locator:${settingsMCPServersTestIds.page}`);
    expect(selectors.mcpServers.create).toBe(`locator:${settingsMCPServersTestIds.create}`);
    expect(selectors.mcpServers.scopeGlobal).toBe(
      `locator:${settingsMCPServersTestIds.scopeGlobal}`
    );
    expect(selectors.mcpServers.scopeWorkspace).toBe(
      `locator:${settingsMCPServersTestIds.scopeWorkspace}`
    );
    expect(selectors.mcpServers.row("browser-global-mcp")).toBe(
      "locator:marketplace-installed-card-browser-global-mcp"
    );
    expect(selectors.mcpServers.rowSource("browser-global-mcp")).toBe(
      'locator:[data-testid="marketplace-installed-card-browser-global-mcp"] [data-slot="pill"]'
    );
    expect(selectors.mcpServers.editRow("browser-global-mcp")).toBe(
      'locator:[data-testid="marketplace-installed-card-browser-global-mcp"] button[aria-label^="More for"]'
    );
    expect(selectors.mcpServers.editorRemove).toBe("locator:settings-mcp-servers-editor-remove");

    expect(selectors.hooks.page).toBe(`locator:${settingsHooksTestIds.page}`);
    expect(selectors.hooks.hookToggle("browser-turn-end")).toBe(
      "locator:settings-page-hooks-row-browser-turn-end-toggle"
    );

    expect(selectors.extensions.page).toBe(`locator:${settingsExtensionsTestIds.page}`);
    expect(selectors.extensions.policyControls).toBe(
      `locator:${settingsExtensionsTestIds.policyControls}`
    );
    expect(selectors.extensions.policyBaseURLInput).toBe(
      `locator:${settingsExtensionsTestIds.policyBaseURLInput}`
    );
    expect(selectors.extensions.allowUnverified).toBe(
      `locator:${settingsExtensionsTestIds.allowUnverified}`
    );
  });
});

describe("tasks operator selectors", () => {
  it("maps the tasks shell, editor, detail, aggregate, and inbox surfaces to stable test IDs", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const getByRoleWithinWindowPath = vi.fn(
      (role: string, options?: { name: string }) =>
        `breadcrumb-role:${role}:${options?.name}` as unknown as Locator
    );
    const getByRole = vi.fn((role: string, options?: { name: string }) =>
      role === "navigation" && options?.name === "Window path"
        ? ({ getByRole: getByRoleWithinWindowPath } as unknown as Locator)
        : (`role:${role}:${options?.name}` as unknown as Locator)
    );
    const selectors = tasksOperatorSelectors({
      getByRole,
      getByTestId,
    });

    expect(selectors.modeList).toBe(`locator:${tasksOperatorTestIds.modeList}`);
    expect(selectors.modeKanban).toBe(`locator:${tasksOperatorTestIds.modeKanban}`);
    expect(selectors.modeDashboard).toBe(`locator:${tasksOperatorTestIds.modeDashboard}`);
    expect(selectors.modeInbox).toBe(`locator:${tasksOperatorTestIds.modeInbox}`);
    expect(selectors.openCreate).toBe(`locator:${tasksOperatorTestIds.openCreate}`);
    expect(selectors.createEditorSurface).toBe(
      `locator:${tasksOperatorTestIds.createEditorSurface}`
    );
    expect(selectors.createTitle).toBe(`locator:${tasksOperatorTestIds.createTitle}`);
    expect(selectors.createDescription).toBe(`locator:${tasksOperatorTestIds.createDescription}`);
    expect(selectors.createModeAdvanced).toBe(`locator:${tasksOperatorTestIds.createModeAdvanced}`);
    expect(selectors.createModeSimple).toBe(`locator:${tasksOperatorTestIds.createModeSimple}`);
    expect(selectors.createScopeGlobal).toBe(`locator:${tasksOperatorTestIds.createScopeGlobal}`);
    expect(selectors.createScopeWorkspace).toBe(
      `locator:${tasksOperatorTestIds.createScopeWorkspace}`
    );
    expect(selectors.createWorkspaceSelect).toBe(
      `locator:${tasksOperatorTestIds.createWorkspaceSelect}`
    );
    expect(selectors.createWorkspaceOption("ws_beta")).toBe("locator:task-workspace-item-ws_beta");
    expect(selectors.createSaveDraft).toBe(`locator:${tasksOperatorTestIds.createSaveDraft}`);
    expect(selectors.createSubmit).toBe(`locator:${tasksOperatorTestIds.createSubmit}`);
    expect(selectors.createTemplate("one_shot")).toBe("locator:task-template-one_shot");
    expect(selectors.createPriority("high")).toBe("locator:task-priority-high");
    expect(selectors.taskCard("task_browser_01")).toBe("locator:task-card-task_browser_01");
    expect(selectors.taskCardPublish("task_browser_01")).toBe(
      "locator:task-card-publish-task_browser_01"
    );
    expect(selectors.detailPreviewPanel).toBe(`locator:${tasksOperatorTestIds.detailPreviewPanel}`);
    expect(selectors.detailPreviewPublish).toBe(
      `locator:${tasksOperatorTestIds.detailPreviewPublish}`
    );
    expect(selectors.detailPreviewDeeplink).toBe(
      `locator:${tasksOperatorTestIds.detailPreviewDeeplink}`
    );
    expect(selectors.detailPublish).toBe(`locator:${tasksOperatorTestIds.detailPublish}`);
    expect(selectors.detailStatus).toBe(`locator:${tasksOperatorTestIds.detailStatus}`);
    expect(selectors.detailContent).toBe(`locator:${tasksOperatorTestIds.detailContent}`);
    expect(selectors.detailApprovalPill).toBe(`locator:${tasksOperatorTestIds.detailApprovalPill}`);
    expect(selectors.detailNowApproval).toBe(`locator:${tasksOperatorTestIds.detailNowApproval}`);
    expect(selectors.detailNowRun).toBe(`locator:${tasksOperatorTestIds.detailNowRun}`);
    expect(selectors.detailBreadcrumbTasks).toBe("breadcrumb-role:button:Tasks");
    expect(selectors.detailTitle).toBe(`locator:${tasksOperatorTestIds.detailTitle}`);
    expect(selectors.detailTabRuns).toBe(`locator:${tasksOperatorTestIds.detailTabRuns}`);
    expect(selectors.detailTab("activity")).toBe("locator:tasks-detail-tab-activity");
    expect(selectors.dashboardView).toBe(`locator:${tasksOperatorTestIds.dashboardView}`);
    expect(selectors.dashboardActiveRun("run_browser_01")).toBe(
      "locator:tasks-dashboard-active-run-run_browser_01"
    );
    expect(selectors.dashboardActiveRunLink("run_browser_01")).toBe(
      "locator:tasks-dashboard-active-run-link-run_browser_01"
    );
    expect(selectors.runDetailOverflow).toBe(`locator:${tasksOperatorTestIds.runDetailOverflow}`);
    expect(selectors.inboxView).toBe(`locator:${tasksOperatorTestIds.inboxView}`);
    expect(selectors.inboxLane("approvals")).toBe("locator:tasks-inbox-group-needs_review");
    expect(selectors.inboxItem("task_browser_approval")).toBe(
      "locator:tasks-inbox-item-task_browser_approval"
    );
    expect(selectors.inboxApprove("task_browser_approval")).toBe(
      "locator:tasks-inbox-item-approve-task_browser_approval"
    );
    expect(selectors.inboxOpenTask("task_browser_approval")).toBe(
      "locator:tasks-inbox-item-open-task_browser_approval"
    );
    expect(selectors.runDetailContent).toBe(`locator:${tasksOperatorTestIds.runDetailContent}`);
    expect(selectors.runSessionDrilldown).toBe(
      `locator:${tasksOperatorTestIds.runSessionDrilldown}`
    );
    expect(selectors.multiAgentEmpty).toBe(`locator:${tasksOperatorTestIds.multiAgentEmpty}`);
    expect(selectors.multiAgentNoActive).toBe(`locator:${tasksOperatorTestIds.multiAgentNoActive}`);
    expect(selectors.multiAgentDisconnected).toBe(
      `locator:${tasksOperatorTestIds.multiAgentDisconnected}`
    );
    expect(selectors.detailInspectDrawer).toBe(
      `locator:${tasksOperatorTestIds.detailInspectDrawer}`
    );
    expect(selectors.detailInspectStream).toBe(
      `locator:${tasksOperatorTestIds.detailInspectStream}`
    );
    expect(selectors.detailCoordination).toBe(`locator:${tasksOperatorTestIds.detailCoordination}`);
    expect(selectors.detailEnqueue).toBe(`locator:${tasksOperatorTestIds.detailEnqueue}`);
    expect(selectors.detailRunsEmpty).toBe(`locator:${tasksOperatorTestIds.detailRunsEmpty}`);
    expect(selectors.detailRunsChannel("run_browser_01")).toBe(
      "locator:tasks-detail-runs-channel-run_browser_01"
    );
    expect(selectors.detailActiveRunChannel).toBe(
      `locator:${tasksOperatorTestIds.detailActiveRunChannel}`
    );
    expect(selectors.detailActiveRunEmpty).toBe(
      `locator:${tasksOperatorTestIds.detailActiveRunEmpty}`
    );
    expect(selectors.detailActiveRunEmptyHint).toBe(
      `locator:${tasksOperatorTestIds.detailActiveRunEmptyHint}`
    );
    expect(selectors.detailPreviewLifecycle).toBe(
      `locator:${tasksOperatorTestIds.detailPreviewLifecycle}`
    );
    expect(selectors.detailPreviewCoordination).toBe(
      `locator:${tasksOperatorTestIds.detailPreviewCoordination}`
    );
    expect(selectors.detailSetupOpen).toBe(`locator:${tasksOperatorTestIds.detailSetupOpen}`);
    expect(selectors.detailSetupSheet).toBe(`locator:${tasksOperatorTestIds.detailSetupSheet}`);
    expect(selectors.detailSetupEdit).toBe(`locator:${tasksOperatorTestIds.detailSetupEdit}`);
    expect(selectors.detailSetupForm).toBe(`locator:${tasksOperatorTestIds.detailSetupForm}`);
    expect(selectors.detailSetupWorkerRuntime).toBe(
      `locator:${tasksOperatorTestIds.detailSetupWorkerRuntime}`
    );
    expect(selectors.runsRow("run_browser_01")).toBe("locator:tasks-runs-row-run_browser_01");
    expect(selectors.runReviews).toBe(`locator:${tasksOperatorTestIds.runReviews}`);
  });
});
