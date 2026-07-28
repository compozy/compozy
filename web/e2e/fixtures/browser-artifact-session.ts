import { mkdtemp } from "node:fs/promises";
import path from "node:path";

import type { BrowserContext, ConsoleMessage, Page, Request, Response } from "@playwright/test";

import {
  type ArtifactManifest,
  type ArtifactCollector,
  type BrowserConsoleEntry,
  type BrowserNetworkEntry,
  type BrowserRouteState,
  mirrorBrowserScreenshotForQA,
  persistBrowserArtifacts,
} from "./artifacts";

export interface BrowserArtifactSessionOptions {
  collector: ArtifactCollector;
  context: BrowserContext;
  qaOutputRootDir?: string;
}

export class BrowserArtifactSession {
  private readonly collector: ArtifactCollector;
  private readonly context: BrowserContext;
  private readonly tempDirPromise: Promise<string>;
  private readonly consoleEntries: BrowserConsoleEntry[] = [];
  private readonly networkEntries: BrowserNetworkEntry[] = [];
  private readonly qaOutputRootDir?: string;
  private readonly screenshotPaths: string[] = [];
  private readonly pages = new Set<Page>();

  private persistedManifest: ArtifactManifest | null = null;

  private constructor(options: BrowserArtifactSessionOptions) {
    this.collector = options.collector;
    this.context = options.context;
    this.qaOutputRootDir = options.qaOutputRootDir?.trim() || undefined;
    this.tempDirPromise = mkdtemp(path.join(this.collector.rootDir, ".capture-"));
  }

  static async start(options: BrowserArtifactSessionOptions): Promise<BrowserArtifactSession> {
    const session = new BrowserArtifactSession(options);
    await session.context.tracing.start({ screenshots: false, snapshots: true });

    for (const page of session.context.pages()) {
      session.attachPage(page);
    }
    session.context.on("page", page => {
      session.attachPage(page);
    });

    return session;
  }

  async captureScreenshot(name = "final", page?: Page): Promise<string | null> {
    const targetPage = page ?? this.selectPage();
    if (targetPage === null) {
      return null;
    }

    const tempDir = await this.tempDirPromise;
    const filePath = path.join(tempDir, `${sanitizePathComponent(name)}.png`);
    await targetPage.screenshot({ fullPage: true, path: filePath });
    this.screenshotPaths.push(filePath);
    if (this.qaOutputRootDir) {
      await mirrorBrowserScreenshotForQA(filePath, this.qaOutputRootDir, name);
    }
    return filePath;
  }

  async persist(page?: Page): Promise<ArtifactManifest> {
    if (this.persistedManifest !== null) {
      return this.persistedManifest;
    }

    const targetPage = page ?? this.selectPage();

    if (this.screenshotPaths.length === 0) {
      await this.captureScreenshot("final", targetPage ?? undefined);
    }

    const tempDir = await this.tempDirPromise;
    const tracePath = path.join(tempDir, "trace.zip");
    await this.context.tracing.stop({ path: tracePath });

    const routeState = targetPage ? await captureRouteState(targetPage) : undefined;

    this.persistedManifest = await persistBrowserArtifacts(this.collector, {
      tracePath,
      screenshotPaths: this.screenshotPaths,
      consoleEntries: this.consoleEntries,
      networkEntries: this.networkEntries,
      routeState,
    });
    return this.persistedManifest;
  }

  private attachPage(page: Page): void {
    if (this.pages.has(page)) {
      return;
    }

    this.pages.add(page);
    page.on("console", message => {
      this.consoleEntries.push(consoleEntryFromMessage(message));
    });
    page.on("pageerror", error => {
      this.consoleEntries.push({
        type: "pageerror",
        text: error.message,
      });
    });
    page.on("response", response => {
      this.networkEntries.push(networkEntryFromResponse(response));
    });
    page.on("requestfailed", request => {
      this.networkEntries.push(networkEntryFromFailedRequest(request));
    });
  }

  private selectPage(): Page | null {
    for (const page of [...this.pages].reverse()) {
      if (!page.isClosed()) {
        return page;
      }
    }
    return null;
  }
}

function consoleEntryFromMessage(message: ConsoleMessage): BrowserConsoleEntry {
  const location = message.location();
  return {
    type: message.type(),
    text: message.text(),
    location:
      location.url === "" && location.lineNumber === 0 && location.columnNumber === 0
        ? undefined
        : {
            url: location.url || undefined,
            line_number: location.lineNumber || undefined,
            column_number: location.columnNumber || undefined,
          },
  };
}

function networkEntryFromResponse(response: Response): BrowserNetworkEntry {
  const request = response.request();
  return {
    event: "response",
    url: response.url(),
    method: request.method(),
    resource_type: request.resourceType(),
    status: response.status(),
    ok: response.ok(),
  };
}

function networkEntryFromFailedRequest(request: Request): BrowserNetworkEntry {
  const failure = request.failure();
  return {
    event: "requestfailed",
    url: request.url(),
    method: request.method(),
    resource_type: request.resourceType(),
    failure: failure?.errorText ?? "unknown request failure",
  };
}

function sanitizePathComponent(value: string): string {
  const trimmed = value.trim().toLowerCase();
  if (trimmed === "") {
    return "artifact";
  }
  return trimmed.replace(/[^a-z0-9._-]+/g, "-");
}

export async function captureRouteState(page: Pick<Page, "evaluate">): Promise<BrowserRouteState> {
  return await page.evaluate(() => {
    const readText = (testId: string) =>
      document.querySelector<HTMLElement>(`[data-testid="${testId}"]`)?.textContent?.trim() ||
      undefined;
    const readHomeMetricValue = (labelPrefix: string) => {
      const prefix = labelPrefix.toLowerCase();
      const metrics = document.querySelectorAll<HTMLElement>(
        '[data-slot="home-kpi-strip"] [data-slot="metric"]'
      );
      for (const metric of metrics) {
        const label = metric
          .querySelector<HTMLElement>('[data-slot="metric-label"]')
          ?.textContent?.trim()
          .toLowerCase();
        if (label?.startsWith(prefix)) {
          return (
            metric.querySelector<HTMLElement>('[data-slot="metric-value"]')?.textContent?.trim() ||
            undefined
          );
        }
      }
      return undefined;
    };
    const countByPrefix = (prefix: string) =>
      document.querySelectorAll(`[data-testid^="${prefix}"]`).length;
    const countSettingsProviderCards = () =>
      [
        ...document.querySelectorAll<HTMLElement>('[data-testid^="settings-page-providers-card-"]'),
      ].filter(element => element.querySelector('[data-testid$="-edit"]') !== null).length;
    const countSettingsMCPServerRows = () =>
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-testid^="settings-page-mcp-servers-row-"]'
        ),
      ].filter(
        element =>
          element.tagName.toLowerCase() === "tr" ||
          element.querySelector('[data-testid$="-delete"]') !== null
      ).length;
    const readPathContainerId = (pattern: RegExp) => {
      const match = window.location.pathname.match(pattern);
      const value = match?.[1];
      return value ? decodeURIComponent(value) : undefined;
    };
    const countAutomationRunCards = () =>
      [...document.querySelectorAll<HTMLElement>("[data-testid]")]
        .map(element => element.dataset.testid || "")
        .filter(
          testId =>
            testId.startsWith("automation-run-") &&
            testId !== "automation-run-history" &&
            testId !== "automation-run-history-loading" &&
            testId !== "automation-run-history-error" &&
            testId !== "automation-run-history-empty" &&
            !testId.startsWith("automation-run-session-link-")
        ).length;
    const networkPathTab = window.location.pathname.match(
      /\/network\/[^/]+\/(threads|directs|activity)(?:\/|$)/
    )?.[1] as "threads" | "directs" | "activity" | undefined;
    const networkActiveTab =
      networkPathTab ??
      (document.querySelector('[data-testid="network-threads-tab"]')
        ? ("threads" as const)
        : document.querySelector('[data-testid="network-directs-tab"]')
          ? ("directs" as const)
          : document.querySelector('[data-testid="network-activity-tab"]')
            ? ("activity" as const)
            : undefined);
    const selectedChannelLink = document.querySelector<HTMLElement>(
      '[data-testid^="network-channel-link-"][aria-current="page"]'
    );
    const selectedChannelTestId = selectedChannelLink?.getAttribute("data-testid") ?? "";
    const networkSelectedChannel =
      selectedChannelTestId.replace(/^network-channel-link-/, "") ||
      document
        .querySelector<HTMLElement>('[data-testid="network-channel-header"] h1')
        ?.textContent?.trim()
        .replace(/^#/, "") ||
      undefined;
    const networkSelectedThread = readPathContainerId(/\/network\/[^/]+\/threads\/([^/?#]+)/);
    const networkSelectedDirect =
      document
        .querySelector<HTMLElement>('[data-testid="network-direct-detail-slot"]')
        ?.getAttribute("aria-label")
        ?.match(/^Direct room (\S+)/)?.[1] ??
      readPathContainerId(/\/network\/[^/]+\/directs\/([^/?#]+)/);
    const automationPathTab = window.location.pathname.match(/^\/(jobs|triggers)(?:\/|$)/)?.[1] as
      | "jobs"
      | "triggers"
      | undefined;
    const automationActiveTab =
      automationPathTab ??
      (document.querySelector('[data-testid="jobs-shell"]')
        ? "jobs"
        : document.querySelector('[data-testid="triggers-shell"]')
          ? "triggers"
          : undefined);
    // Catalog scope now lives in the list route URL (`?scope=`); absent means "all".
    const automationScopeParam = new URLSearchParams(window.location.search).get("scope");
    const automationScopeFilter = /^\/(jobs|triggers)$/.test(window.location.pathname)
      ? automationScopeParam === "global" || automationScopeParam === "workspace"
        ? automationScopeParam
        : ("all" as const)
      : undefined;
    const topbarTitle = readText("topbar-title-text");
    const automationSelectedItem = document.querySelector('[data-testid="automation-detail-panel"]')
      ? topbarTitle
      : undefined;
    const automationEditorKind = document.querySelector('[data-testid="automation-job-form"]')
      ? "job"
      : document.querySelector('[data-testid="automation-trigger-form"]')
        ? "trigger"
        : undefined;
    const bridgeScopeSearch = new URLSearchParams(window.location.search).get("scope");
    const bridgeScopeFilter = document.querySelector('[data-testid="bridge-list-panel"]')
      ? bridgeScopeSearch === "global" || bridgeScopeSearch === "workspace"
        ? bridgeScopeSearch
        : "all"
      : undefined;
    const bridgeSelectedItem = document.querySelector('[data-testid="bridge-detail-panel"]')
      ? topbarTitle
      : undefined;
    const tasksActiveMode = (["dashboard", "inbox", "kanban", "list"] as const).find(
      mode =>
        document.querySelector(`[data-testid="tasks-mode-${mode}"][aria-current="page"]`) !== null
    );
    const tasksSelectedTask = readPathContainerId(/\/tasks\/([^/?#]+)/);
    const tasksSelectedRun = readPathContainerId(/\/tasks\/[^/]+\/runs\/([^/?#]+)/);
    const tasksViewVisible =
      document.querySelector('[data-testid="tasks-dashboard-view"]') !== null ||
      document.querySelector('[data-testid="tasks-inbox-view"]') !== null ||
      document.querySelector('[data-testid="task-list-surface"]') !== null ||
      document.querySelector('[data-testid="tasks-detail-content"]') !== null ||
      document.querySelector('[data-testid="tasks-run-detail-content"]') !== null ||
      document.querySelector('[data-testid="task-editor-surface"]') !== null;
    const tasksReviewCount = countByPrefix("tasks-run-review-");
    const knowledgeScope = document.querySelector('[data-testid="tab-global"][aria-pressed="true"]')
      ? "global"
      : document.querySelector('[data-testid="tab-workspace"][aria-pressed="true"]')
        ? "workspace"
        : document.querySelector('[data-testid="tab-agent"][aria-pressed="true"]')
          ? "agent"
          : undefined;
    const knowledgeSelectedItem =
      document
        .querySelector<HTMLElement>('[data-testid^="memory-item-"][data-state="selected"]')
        ?.textContent?.trim() || undefined;
    const skillsDetailRouteItem = readPathContainerId(/\/marketplace\/skill\/([^/?#]+)/);
    const skillsSelectedItem = skillsDetailRouteItem;
    const skillsEnabledText = document
      .querySelector<HTMLElement>("#marketplace-skill-enabled-label")
      ?.textContent?.trim()
      .toLowerCase();
    const skillsEnabledState = skillsEnabledText?.includes("disabled")
      ? "disabled"
      : skillsEnabledText?.includes("enabled")
        ? "enabled"
        : undefined;
    const sandboxProfiles = [
      ...document.querySelectorAll<HTMLElement>('[data-testid^="sandbox-page-card-"][data-name]'),
    ];
    const sandboxProfileNames = sandboxProfiles
      .map(element => element.dataset.name)
      .filter((value): value is string => Boolean(value));
    const settingsActiveSection = readPathContainerId(/\/settings\/([^/?#]+)/);

    return {
      url: window.location.href,
      pathname: window.location.pathname,
      title: document.title,
      automation_active_tab: automationActiveTab,
      automation_delete_visible:
        document.querySelector('[data-testid="delete-automation-btn"]') !== null,
      automation_detail_overflow_visible:
        document.querySelector('[data-testid="automation-detail-overflow"]') !== null,
      automation_enabled_toggle_visible:
        document.querySelector('[data-testid="toggle-automation-btn"]') !== null,
      automation_editor_kind: automationEditorKind,
      automation_editor_open:
        document.querySelector('[data-testid="automation-editor-dialog"]') !== null,
      automation_item_count: countByPrefix("automation-item-"),
      automation_run_count: countAutomationRunCards(),
      automation_run_history_visible:
        document.querySelector('[data-testid="automation-run-history"]') !== null,
      automation_scheduler_visible:
        document.querySelector('[data-testid="automation-job-scheduler"]') !== null,
      automation_scope_filter: automationScopeFilter,
      automation_selected_item: automationSelectedItem,
      automation_session_link_count: document.querySelectorAll(
        'a[data-testid^="automation-run-"][href^="/session/"]'
      ).length,
      automation_trigger_visible:
        document.querySelector('[data-testid="trigger-job-btn"]') !== null,
      automation_view_visible:
        document.querySelector('[data-testid="jobs-shell"]') !== null ||
        document.querySelector('[data-testid="triggers-shell"]') !== null ||
        document.querySelector('[data-testid="automation-detail-panel"]') !== null,
      bridge_create_dialog_open:
        document.querySelector('[data-testid="bridge-create-dialog"]') !== null,
      bridge_detail_visible: document.querySelector('[data-testid="bridge-detail-panel"]') !== null,
      bridge_edit_dialog_open:
        document.querySelector('[data-testid="bridge-edit-dialog"]') !== null,
      bridge_item_count: countByPrefix("bridge-item-"),
      bridge_route_count: countByPrefix("bridge-route-"),
      bridge_scope_filter: bridgeScopeFilter,
      bridge_secret_binding_count: countByPrefix("bridge-secret-binding-"),
      bridge_selected_item: bridgeSelectedItem,
      bridge_test_delivery_open:
        document.querySelector('[data-testid="bridge-delivery-test-panel"]') !== null,
      bridge_test_delivery_result_visible:
        document.querySelector('[data-testid="bridge-test-delivery-result"]') !== null,
      bridge_view_visible:
        document.querySelector('[data-testid="bridge-list-panel"]') !== null ||
        document.querySelector('[data-testid="bridges-empty-state"]') !== null ||
        document.querySelector('[data-testid="bridge-detail-panel"]') !== null,
      chat_view_visible: document.querySelector('[data-testid="chat-view"]') !== null,
      composer_clear_button_enabled:
        document.querySelector<HTMLButtonElement>('[data-testid="composer-clear-button"]')
          ?.disabled === false,
      composer_clear_button_visible:
        document.querySelector('[data-testid="composer-clear-button"]') !== null,
      delete_button_visible: document.querySelector('[data-testid="delete-button"]') !== null,
      session_topbar_overflow_visible:
        document.querySelector('[data-testid="session-topbar-overflow"]') !== null,
      home_activity_count: document.querySelectorAll('[data-slot="home-activity-row"]').length,
      home_agent_count: document.querySelectorAll('[data-slot="home-agent-row"]').length,
      home_completed_today_value: readHomeMetricValue("Completed today"),
      home_connection_status: document.querySelector<HTMLElement>(
        '[data-testid="home-connection-indicator"]'
      )?.dataset.status,
      home_metric_count: document.querySelectorAll(
        '[data-slot="home-kpi-strip"] [data-slot="metric"]'
      ).length,
      home_needs_you_value: readHomeMetricValue("Needs you"),
      home_run_count: document.querySelectorAll('[data-slot="home-run-card"]').length,
      home_scope_text:
        document.querySelector<HTMLElement>('[data-slot="home-page-meta"]')?.textContent?.trim() ||
        undefined,
      home_usage_value: readHomeMetricValue("Usage"),
      home_view_visible: document.querySelector('[data-testid="home-body"]') !== null,
      home_working_now_value: readHomeMetricValue("Working now"),
      knowledge_create_dialog_open:
        document.querySelector('[data-testid="knowledge-create-dialog"]') !== null,
      knowledge_decisions_count: countByPrefix("knowledge-decision-"),
      knowledge_delete_dialog_open:
        document.querySelector('[data-testid="knowledge-delete-dialog"]') !== null,
      knowledge_detail_visible:
        document.querySelector('[data-testid="knowledge-detail-panel"]') !== null,
      knowledge_edit_dialog_open:
        document.querySelector('[data-testid="knowledge-edit-dialog"]') !== null,
      knowledge_item_count: countByPrefix("memory-item-"),
      knowledge_revert_button_count: countByPrefix("revert-memory-decision-"),
      knowledge_scope: knowledgeScope,
      knowledge_search_active:
        document.querySelector('[data-testid="knowledge-search-info"]') !== null,
      knowledge_selected_item: knowledgeSelectedItem,
      knowledge_view_visible: document.querySelector('[data-testid="knowledge-shell"]') !== null,
      skills_content_visible: document.querySelector('[data-testid="content-body"]') !== null,
      skills_detail_visible:
        skillsDetailRouteItem !== undefined &&
        document.querySelector('[data-testid="marketplace-detail"]') !== null,
      skills_enabled_state: skillsEnabledState,
      skills_item_count: countByPrefix("marketplace-installed-card-"),
      skills_search_active:
        (document.querySelector<HTMLInputElement>('[data-testid="marketplace-kind-search-skill"]')
          ?.value ?? "") !== "",
      skills_selected_item: skillsSelectedItem,
      skills_view_visible:
        document.querySelector('[data-testid="marketplace-kind-skill"]') !== null ||
        (skillsDetailRouteItem !== undefined &&
          document.querySelector('[data-testid="marketplace-detail"]') !== null),
      sandbox_action_result_visible:
        document.querySelector('[data-testid="sandbox-page-action-result"]') !== null,
      sandbox_delete_dialog_open:
        document.querySelector('[data-testid="settings-sandboxes-delete"]') !== null,
      sandbox_editor_open:
        document.querySelector('[data-testid="settings-sandbox-editor"]') !== null,
      sandbox_empty_visible: document.querySelector('[data-testid="sandbox-page-empty"]') !== null,
      sandbox_profile_count: sandboxProfiles.length,
      sandbox_profile_names: sandboxProfileNames,
      sandbox_restart_notice_visible:
        document.querySelector('[data-testid="settings-page-sandbox-restart-notice"]') !== null,
      sandbox_total_text: readText("sandbox-page-total"),
      sandbox_view_visible: document.querySelector('[data-testid="sandbox-shell"]') !== null,
      sandbox_workspace_references_text: readText("sandbox-page-workspaces"),
      settings_action_result_visible:
        document.querySelector('[data-testid^="settings-page-"][data-testid$="-action-result"]') !==
        null,
      settings_active_section: settingsActiveSection,
      settings_mcp_server_count: countSettingsMCPServerRows(),
      settings_provider_card_count: countSettingsProviderCards(),
      settings_restart_notice_visible:
        document.querySelector(
          '[data-testid^="settings-page-"][data-testid$="-restart-notice"]'
        ) !== null,
      settings_save_bar_visible:
        document.querySelector('[data-testid^="settings-page-"][data-testid$="-save-bar"]') !==
        null,
      settings_section_count: document.querySelectorAll(
        '[data-testid="settings-section-nav"] a[data-testid^="settings-section-"]'
      ).length,
      settings_view_visible: document.querySelector('[data-testid="settings-shell"]') !== null,
      vault_delete_dialog_open:
        document.querySelector('[data-testid="settings-vault-delete"]') !== null,
      vault_editor_open: document.querySelector('[data-testid="settings-vault-editor"]') !== null,
      vault_secret_count: countByPrefix("vault-secrets-row"),
      message_count: document.querySelectorAll(
        '[data-testid="message-bubble-user"], [data-testid="message-bubble-assistant"]'
      ).length,
      network_active_tab: networkActiveTab,
      network_activity_count: countByPrefix("network-activity-entry-"),
      network_channel_count: countByPrefix("network-channel-row-"),
      network_create_dialog_open:
        document.querySelector('[data-testid="network-create-channel-dialog"]') !== null,
      network_thread_count: countByPrefix("network-thread-list-row-"),
      network_direct_count: countByPrefix("network-direct-list-row-"),
      network_disabled_visible:
        document.querySelector('[data-testid="network-disabled-state"]') !== null,
      network_message_count: countByPrefix("network-message-"),
      network_no_channels_visible:
        document.querySelector('[data-testid="network-no-channels-state"]') !== null,
      network_selected_channel: networkSelectedChannel,
      network_selected_thread: networkSelectedThread,
      network_selected_direct: networkSelectedDirect,
      network_view_visible: document.querySelector('[data-testid="network-shell"]') !== null,
      network_work_count: countByPrefix("network-work-inspector-row-"),
      permission_prompt_visible:
        document.querySelector('[data-testid="permission-prompt"]') !== null,
      processing_indicator_visible:
        document.querySelector('[data-testid="processing-indicator"]') !== null,
      resume_button_visible: document.querySelector('[data-testid="resume-button"]') !== null,
      session_name: readText("session-name"),
      stop_button_visible: document.querySelector('[data-testid="stop-button"]') !== null,
      tasks_active_mode: tasksActiveMode,
      tasks_children_count: countByPrefix("tasks-detail-subtask-"),
      tasks_dependencies_count: countByPrefix("tasks-detail-dependency-"),
      tasks_detail_cancel_visible:
        document.querySelector('[data-testid="tasks-detail-cancel"]') !== null,
      tasks_detail_delete_dialog_open:
        document.querySelector('[data-testid="tasks-detail-delete-dialog"]') !== null,
      tasks_detail_visible: document.querySelector('[data-testid="tasks-detail-content"]') !== null,
      tasks_inbox_count: document.querySelectorAll('[data-testid^="tasks-inbox-item-"][data-lane]')
        .length,
      tasks_review_count: tasksReviewCount,
      tasks_run_cancel_visible: document.querySelector('[data-testid="tasks-run-cancel"]') !== null,
      tasks_run_detail_visible:
        document.querySelector('[data-testid="tasks-run-detail-content"]') !== null,
      tasks_selected_run: tasksSelectedRun,
      tasks_selected_task: tasksSelectedTask,
      tasks_task_count: countByPrefix("task-card-"),
      tasks_view_visible: tasksViewVisible,
      tool_card_count: countByPrefix("tool-call-card"),
    };
  });
}
