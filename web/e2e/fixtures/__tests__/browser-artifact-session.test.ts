// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { captureRouteState } from "../browser-artifact-session";

describe("captureRouteState", () => {
  it("captures network shell context with the channel-pivot information architecture", async () => {
    window.history.replaceState({}, "", "/network/builders/threads");
    document.title = "AGH";
    document.body.innerHTML = `
      <div data-testid="network-shell">
        <aside data-testid="network-channel-rail">
          <div data-testid="network-channel-row-builders">
            <a data-testid="network-channel-link-builders" aria-current="page">
              <span>builders</span><span>just now</span>
            </a>
          </div>
          <div data-testid="network-channel-row-design">
            <a data-testid="network-channel-link-design">design</a>
          </div>
        </aside>
        <main data-testid="network-main-pane">
          <header data-testid="network-channel-header"><h1>#builders</h1></header>
          <section data-testid="network-threads-tab">
            <article data-testid="network-thread-list-row-thread_one"></article>
            <article data-testid="network-thread-list-row-thread_two"></article>
          </section>
          <section data-testid="network-activity-feed">
            <a data-testid="network-activity-entry-thread:thread_one"></a>
          </section>
          <section data-testid="network-work-inspector">
            <li data-testid="network-work-inspector-row-work_one"></li>
          </section>
        </main>
      </div>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/network/builders/threads",
      title: "AGH",
      chat_view_visible: false,
      message_count: 0,
      network_view_visible: true,
      network_active_tab: "threads",
      network_channel_count: 2,
      network_thread_count: 2,
      network_direct_count: 0,
      network_activity_count: 1,
      network_message_count: 0,
      network_work_count: 1,
      network_selected_channel: "builders",
    });
    expect(routeState).not.toHaveProperty("network_selected_peer");
    expect(routeState.network_selected_thread).toBeUndefined();
  });

  it("captures disabled and no-channel network route states for launch diagnostics", async () => {
    window.history.replaceState({}, "", "/network");
    document.title = "AGH";
    document.body.innerHTML = `
      <div data-testid="network-shell">
        <section data-testid="network-no-channels-state"></section>
      </div>
      <section data-testid="network-disabled-state"></section>
      <form data-testid="network-create-channel-dialog"></form>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/network",
      network_create_dialog_open: true,
      network_disabled_visible: true,
      network_no_channels_visible: true,
      network_view_visible: true,
    });
  });

  it("captures the selected thread overlay container id without leaking direct fields", async () => {
    window.history.replaceState({}, "", "/network/builders/threads/thread_launch_command");
    document.title = "AGH";
    document.body.innerHTML = `
      <div data-testid="network-shell">
        <main data-testid="network-main-pane">
          <section data-testid="network-threads-tab">
            <article data-testid="network-thread-list-row-thread_launch_command"></article>
          </section>
          <aside data-testid="network-thread-overlay" aria-label="Thread"></aside>
        </main>
      </div>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      network_view_visible: true,
      network_active_tab: "threads",
      network_selected_thread: "thread_launch_command",
    });
    expect(routeState.network_selected_direct).toBeUndefined();
  });

  it("captures the selected direct room container id without leaking thread fields", async () => {
    window.history.replaceState({}, "", "/network/builders/directs/direct_abc123");
    document.title = "AGH";
    document.body.innerHTML = `
      <div data-testid="network-shell">
        <main data-testid="network-main-pane">
          <section data-testid="network-direct-detail-slot" aria-label="Direct room direct_abc123 in #builders">
            <article data-testid="network-direct-room" aria-label="Direct room with @peer"></article>
          </section>
        </main>
      </div>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      network_view_visible: true,
      network_active_tab: "directs",
      network_selected_direct: "direct_abc123",
    });
    expect(routeState.network_selected_thread).toBeUndefined();
  });

  it("captures automation detail route context, topbar title, and session-link state", async () => {
    window.history.replaceState({}, "", "/jobs/job_daily_review");
    document.title = "AGH";
    document.body.innerHTML = `
      <header><h1 data-testid="topbar-title-text">deploy-review</h1></header>
      <section data-testid="automation-detail-panel">
        <div data-slot="page-head-title">deploy-review</div>
        <button data-testid="automation-detail-overflow"></button>
        <button data-testid="toggle-automation-btn"></button>
        <button data-testid="trigger-job-btn"></button>
        <button data-testid="delete-automation-btn"></button>
        <div data-testid="automation-job-scheduler"></div>
      </section>
      <section data-testid="automation-run-history">
        <a data-testid="automation-run-run_001" href="/session/sess_001"></a>
        <article data-testid="automation-run-run_002"></article>
      </section>
      <form data-testid="automation-job-form"></form>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/jobs/job_daily_review",
      title: "AGH",
      automation_view_visible: true,
      automation_active_tab: "jobs",
      automation_delete_visible: true,
      automation_detail_overflow_visible: true,
      automation_enabled_toggle_visible: true,
      automation_editor_kind: "job",
      automation_editor_open: false,
      automation_item_count: 0,
      automation_run_count: 2,
      automation_run_history_visible: true,
      automation_scheduler_visible: true,
      automation_selected_item: "deploy-review",
      automation_session_link_count: 1,
      automation_trigger_visible: true,
    });
  });

  it("captures bridge detail route context, topbar title, and dialog state", async () => {
    window.history.replaceState({}, "", "/bridges/brg_ops");
    document.title = "AGH";
    document.body.innerHTML = `
      <header><h1 data-testid="topbar-title-text">Telegram Bridge Ops</h1></header>
      <section data-testid="bridge-detail-panel">
        <div data-slot="page-head-title">Telegram Bridge Ops</div>
      </section>
      <article data-testid="bridge-secret-binding-bot_token"></article>
      <article data-testid="bridge-route-sess_bridge_01"></article>
      <form data-testid="bridge-edit-dialog"></form>
      <section data-testid="bridge-test-delivery-result"></section>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/bridges/brg_ops",
      title: "AGH",
      bridge_view_visible: true,
      bridge_item_count: 0,
      bridge_selected_item: "Telegram Bridge Ops",
      bridge_secret_binding_count: 1,
      bridge_route_count: 1,
      bridge_edit_dialog_open: true,
      bridge_test_delivery_result_visible: true,
    });
  });

  it("captures task route context, selected run, and graph/review counts", async () => {
    window.history.replaceState({}, "", "/tasks/task_launch/runs/run_launch");
    document.title = "AGH";
    document.body.innerHTML = `
      <div data-testid="tasks-dashboard-view">
        <a data-testid="tasks-mode-dashboard" href="/tasks?mode=dashboard"></a>
        <a data-testid="tasks-mode-inbox" href="/tasks?mode=inbox"></a>
        <a data-testid="tasks-mode-kanban" href="/tasks?mode=kanban"></a>
        <a data-testid="tasks-mode-list" aria-current="page" href="/tasks"></a>
        <article data-testid="task-card-task_launch"></article>
        <article data-testid="task-card-task_review"></article>
      </div>
      <section data-testid="tasks-detail-content">
        <button data-testid="tasks-detail-cancel"></button>
        <table data-testid="tasks-detail-subtasks">
          <tr data-testid="tasks-detail-subtask-task_child"></tr>
        </table>
        <table data-testid="tasks-detail-dependencies">
          <tr data-testid="tasks-detail-dependency-task_dependency"></tr>
        </table>
      </section>
      <section data-testid="tasks-run-detail-content">
        <button data-testid="tasks-run-cancel"></button>
        <table data-testid="tasks-run-reviews">
          <tr data-testid="tasks-run-review-review_001"></tr>
        </table>
      </section>
      <article data-testid="tasks-inbox-item-task_launch" data-lane="failed_runs"></article>
      <button data-testid="tasks-inbox-item-retry-task_launch"></button>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/tasks/task_launch/runs/run_launch",
      tasks_active_mode: "list",
      tasks_children_count: 1,
      tasks_dependencies_count: 1,
      tasks_detail_cancel_visible: true,
      tasks_detail_visible: true,
      tasks_inbox_count: 1,
      tasks_review_count: 1,
      tasks_run_cancel_visible: true,
      tasks_run_detail_visible: true,
      tasks_selected_run: "run_launch",
      tasks_selected_task: "task_launch",
      tasks_task_count: 2,
      tasks_view_visible: true,
    });
  });

  it("captures knowledge route scope, dialogs, decisions, and selected item", async () => {
    window.history.replaceState({}, "", "/knowledge");
    document.title = "AGH";
    document.body.innerHTML = `
      <main data-testid="knowledge-shell">
        <button data-testid="tab-global" aria-pressed="false"></button>
        <button data-testid="tab-workspace" aria-pressed="true"></button>
        <button data-testid="tab-agent" aria-pressed="false"></button>
        <aside data-testid="knowledge-list-panel">
          <p data-testid="knowledge-search-info">Recall 1 of top-K</p>
          <button data-testid="memory-item-workspace:launch-memory.md" data-state="selected">
            Launch Memory
          </button>
          <button data-testid="memory-item-workspace:other-memory.md">Other Memory</button>
        </aside>
        <section data-testid="knowledge-detail-panel">
          <article data-testid="knowledge-decision-dec_write"></article>
          <button data-testid="revert-memory-decision-dec_write"></button>
        </section>
        <form data-testid="knowledge-create-dialog"></form>
        <form data-testid="knowledge-edit-dialog"></form>
        <form data-testid="knowledge-delete-dialog"></form>
      </main>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/knowledge",
      knowledge_create_dialog_open: true,
      knowledge_decisions_count: 1,
      knowledge_delete_dialog_open: true,
      knowledge_detail_visible: true,
      knowledge_edit_dialog_open: true,
      knowledge_item_count: 2,
      knowledge_revert_button_count: 1,
      knowledge_scope: "workspace",
      knowledge_search_active: true,
      knowledge_selected_item: "Launch Memory",
      knowledge_view_visible: true,
    });
  });

  it("captures the installed Skills catalog and unified detail state", async () => {
    window.history.replaceState({}, "", "/marketplace/skills?tab=installed");
    document.title = "AGH";
    document.body.innerHTML = `
      <main data-testid="marketplace-kind-skill">
        <input data-testid="marketplace-kind-search-skill" value="browser-context" />
        <article data-testid="marketplace-installed-card-browser-context-skill">
          Browser Context Skill
        </article>
        <article data-testid="marketplace-installed-card-browser-other-skill">Other Skill</article>
      </main>
    `;

    const installedState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(installedState).toMatchObject({
      pathname: "/marketplace/skills",
      skills_content_visible: false,
      skills_detail_visible: false,
      skills_item_count: 2,
      skills_search_active: true,
      skills_view_visible: true,
    });

    window.history.replaceState({}, "", "/marketplace/skill/browser-context-skill");
    document.body.innerHTML = `
      <section data-testid="marketplace-detail">
        <span id="marketplace-skill-enabled-label">Enabled</span>
        <article data-testid="content-body">Skill content</article>
      </section>
    `;

    const detailState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(detailState).toMatchObject({
      pathname: "/marketplace/skill/browser-context-skill",
      skills_content_visible: true,
      skills_detail_visible: true,
      skills_enabled_state: "enabled",
      skills_search_active: false,
      skills_selected_item: "browser-context-skill",
      skills_view_visible: true,
    });
  });

  it("captures sandbox route profile counts, dialogs, and restart state", async () => {
    window.history.replaceState({}, "", "/sandbox");
    document.title = "AGH";
    document.body.innerHTML = `
      <main data-testid="sandbox-shell">
        <p data-testid="sandbox-page-total">2 profiles</p>
        <p data-testid="sandbox-page-workspaces">1 workspace reference</p>
        <table data-testid="sandbox-page-list">
          <tbody>
            <tr data-name="browser-local-sandbox" data-testid="sandbox-page-card-browser-local-sandbox">
              <td data-testid="sandbox-page-card-browser-local-sandbox-profile">local / reuse</td>
              <td data-testid="sandbox-page-card-browser-local-sandbox-source">CONFIG</td>
              <td data-testid="sandbox-page-card-browser-local-sandbox-usage">1 workspace</td>
            </tr>
            <tr data-name="browser-blocked-sandbox" data-testid="sandbox-page-card-browser-blocked-sandbox"></tr>
          </tbody>
        </table>
        <form data-testid="settings-sandbox-editor"></form>
        <section data-testid="settings-sandboxes-delete"></section>
        <section data-testid="sandbox-page-action-result"></section>
        <section data-testid="settings-page-sandbox-restart-notice"></section>
      </main>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/sandbox",
      sandbox_action_result_visible: true,
      sandbox_delete_dialog_open: true,
      sandbox_editor_open: true,
      sandbox_profile_count: 2,
      sandbox_profile_names: ["browser-local-sandbox", "browser-blocked-sandbox"],
      sandbox_restart_notice_visible: true,
      sandbox_total_text: "2 profiles",
      sandbox_view_visible: true,
      sandbox_workspace_references_text: "1 workspace reference",
    });
  });

  it("captures Settings route section, provider, restart, and route-independent vault state", async () => {
    window.history.replaceState({}, "", "/settings/network");
    document.title = "AGH";
    document.body.innerHTML = `
      <main data-testid="settings-shell">
        <nav data-testid="settings-section-nav">
          <a data-testid="settings-section-general"></a>
          <a data-testid="settings-section-network"></a>
        </nav>
        <section data-testid="settings-page-network-action-result"></section>
        <form data-testid="settings-vault-editor"></form>
        <section data-testid="settings-vault-delete"></section>
        <section data-testid="settings-page-network-restart-notice"></section>
        <footer data-testid="settings-page-network-save-bar"></footer>
        <table data-testid="vault-page-table">
          <tr data-testid="vault-secrets-row"></tr>
          <tr data-testid="vault-secrets-row"></tr>
        </table>
        <article data-testid="settings-page-providers-card-codex">
          <button data-testid="settings-page-providers-card-codex-edit"></button>
        </article>
        <article data-testid="settings-page-providers-card-claude">
          <button data-testid="settings-page-providers-card-claude-edit"></button>
        </article>
        <table>
          <tbody>
            <tr data-testid="settings-page-mcp-servers-row-filesystem">
              <td><button data-testid="settings-page-mcp-servers-row-filesystem-delete"></button></td>
            </tr>
          </tbody>
        </table>
      </main>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/settings/network",
      settings_action_result_visible: true,
      settings_active_section: "network",
      settings_mcp_server_count: 1,
      settings_provider_card_count: 2,
      settings_restart_notice_visible: true,
      settings_save_bar_visible: true,
      settings_section_count: 2,
      settings_view_visible: true,
      vault_delete_dialog_open: true,
      vault_editor_open: true,
      vault_secret_count: 2,
    });
  });

  it("captures clean Settings section state without modal or restart affordances", async () => {
    window.history.replaceState({}, "", "/settings/general");
    document.title = "AGH";
    document.body.innerHTML = `
      <main data-testid="settings-shell">
        <nav data-testid="settings-section-nav">
          <a data-testid="settings-section-general"></a>
          <a data-testid="settings-section-network"></a>
        </nav>
      </main>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/settings/general",
      settings_active_section: "general",
      settings_mcp_server_count: 0,
      settings_provider_card_count: 0,
      settings_restart_notice_visible: false,
      settings_save_bar_visible: false,
      settings_section_count: 2,
      settings_view_visible: true,
      vault_delete_dialog_open: false,
      vault_editor_open: false,
      vault_secret_count: 0,
    });
    expect(routeState.settings_action_result_visible).toBe(false);
  });

  it("captures the current Home overview route context", async () => {
    window.history.replaceState({}, "", "/");
    document.title = "AGH";
    document.body.innerHTML = `
      <main data-testid="home-body">
        <div data-testid="home-connection-indicator" data-status="connected"></div>
        <p data-slot="home-page-meta">Thursday, Jul 24 · workspace launch-hq</p>
        <section data-slot="home-kpi-strip">
          <article data-slot="metric">
            <span data-slot="metric-label">Working now</span>
            <span data-slot="metric-value">1</span>
          </article>
          <article data-slot="metric">
            <span data-slot="metric-label">Needs you</span>
            <span data-slot="metric-value">2</span>
          </article>
          <article data-slot="metric">
            <span data-slot="metric-label">Completed today</span>
            <span data-slot="metric-value">3</span>
          </article>
          <article data-slot="metric">
            <span data-slot="metric-label">Usage · 30d</span>
            <span data-slot="metric-value">4K</span>
          </article>
        </section>
        <article data-slot="home-run-card"></article>
        <article data-slot="home-agent-row"></article>
        <article data-slot="home-agent-row"></article>
        <article data-slot="home-activity-row"></article>
      </main>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/",
      home_view_visible: true,
      home_connection_status: "connected",
      home_metric_count: 4,
      home_working_now_value: "1",
      home_needs_you_value: "2",
      home_completed_today_value: "3",
      home_usage_value: "4K",
      home_run_count: 1,
      home_agent_count: 2,
      home_activity_count: 1,
      home_scope_text: "Thursday, Jul 24 · workspace launch-hq",
    });
  });
});
