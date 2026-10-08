// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { captureRouteState } from "../browser-artifact-session";

describe("captureRouteState", () => {
  it("captures automation detail route context, topbar title, and session-link state", async () => {
    window.history.replaceState({}, "", "/automations/jobs/job_daily_review");
    document.title = "CompozyOS";
    document.body.innerHTML = `
      <header><h1 data-testid="topbar-title-text">deploy-review</h1></header>
      <section data-testid="automation-detail-panel">
        <div data-slot="page-head-title">deploy-review</div>
        <button data-testid="automation-run-now-btn"></button>
        <button data-testid="automation-detail-overflow"></button>
        <button data-testid="automation-enable-switch"></button>
        <button data-testid="automation-delete-btn"></button>
        <button data-testid="automation-inspect-btn"></button>
      </section>
      <section data-testid="automation-run-list">
        <ul data-testid="automation-run-list-rows">
          <li><button data-testid="automation-run-run_001"></button>
            <div data-testid="automation-run-drawer-run_001">
              <a data-testid="automation-run-open-run_001" href="/session/sess_001"></a>
            </div>
          </li>
          <li><button data-testid="automation-run-run_002"></button>
            <div data-testid="automation-run-drawer-run_002" hidden>
              <a data-testid="automation-run-open-run_002" href="/session/sess_002"></a>
            </div>
          </li>
        </ul>
      </section>
      <form data-entity="job" data-testid="automation-form"></form>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/automations/jobs/job_daily_review",
      title: "CompozyOS",
      automation_view_visible: true,
      automation_active_tab: "jobs",
      automation_delete_visible: true,
      automation_detail_overflow_visible: true,
      automation_enabled_toggle_visible: true,
      automation_editor_kind: "job",
      automation_editor_open: false,
      automation_item_count: 0,
      automation_run_count: 2,
      automation_inspect_visible: true,
      automation_run_list_visible: true,
      automation_selected_item: "deploy-review",
      automation_session_link_count: 1,
      automation_run_now_visible: true,
    });
  });

  it("captures the merged automations listing without a detail tab", async () => {
    window.history.replaceState({}, "", "/automations?start=schedule&scope=global");
    document.title = "CompozyOS";
    document.body.innerHTML = `
      <main data-testid="automations-shell">
        <div data-testid="automations-list-rows">
          <div data-testid="automation-row-job-job_daily_review">
            <p data-testid="automation-sentence-job_daily_review"></p>
          </div>
          <div data-testid="automation-row-trigger-trg_deploy"></div>
        </div>
      </main>
    `;

    const routeState = await captureRouteState({
      evaluate: async (callback: () => unknown) => callback(),
    });

    expect(routeState).toMatchObject({
      pathname: "/automations",
      automation_item_count: 2,
      automation_scope_filter: "global",
      automation_view_visible: true,
    });
    expect(routeState.automation_active_tab).toBeUndefined();
    expect(routeState.automation_selected_item).toBeUndefined();
  });

  it("captures task route context, selected run, and graph/review counts", async () => {
    window.history.replaceState({}, "", "/tasks/task_launch/runs/run_launch");
    document.title = "CompozyOS";
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

  it("captures the installed Skills catalog and unified detail state", async () => {
    window.history.replaceState({}, "", "/marketplace");
    document.title = "CompozyOS";
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
      pathname: "/marketplace",
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

  it("captures Settings route section, provider, restart, and route-independent vault state", async () => {
    window.history.replaceState({}, "", "/settings/automation");
    document.title = "CompozyOS";
    document.body.innerHTML = `
      <main data-testid="settings-shell">
        <nav data-testid="settings-section-nav">
          <a data-testid="settings-section-general"></a>
          <a data-testid="settings-section-automation"></a>
        </nav>
        <section data-testid="settings-page-automation-action-result"></section>
        <form data-testid="settings-vault-editor"></form>
        <section data-testid="settings-vault-delete"></section>
        <section data-testid="settings-page-automation-restart-notice"></section>
        <footer data-testid="settings-page-automation-save-bar"></footer>
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
      pathname: "/settings/automation",
      settings_action_result_visible: true,
      settings_active_section: "automation",
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
    document.title = "CompozyOS";
    document.body.innerHTML = `
      <main data-testid="settings-shell">
        <nav data-testid="settings-section-nav">
          <a data-testid="settings-section-general"></a>
          <a data-testid="settings-section-automation"></a>
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
    document.title = "CompozyOS";
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
            <span data-slot="metric-label">Usage, last 30 days</span>
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
