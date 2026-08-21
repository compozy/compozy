import { mkdtemp } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";

import { reloadDaemonServedPage } from "../fixtures/navigation";
import {
  appWindow,
  openAppWindow,
  setGlobalScope,
  switchWorkspace,
} from "../fixtures/os-navigation";
import {
  profilesOperatorSelectors,
  settingsOperatorSelectors,
  sessionLifecycleSelectors,
} from "../fixtures/selectors";
import {
  browserSettingsOperatorFlowScenario,
  cleanupBrowserSettingsFixtures,
  seedBrowserSettingsFixtures,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, completeOnboardingIfPrompted } from "../fixtures/workspace";
import {
  settingsUpdateApplyingFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateManagedFixture,
  settingsUpdateNoAppFixture,
  settingsUpdateRolledBackFixture,
} from "@/systems/settings/mocks/settings-update-fixture";

/** Host-install shapes the update projection can report, shared by the update journeys. */
const updateFixtures = {
  bothAvailable: settingsUpdateBothAvailableFixture,
  managed: settingsUpdateManagedFixture,
  noApp: settingsUpdateNoAppFixture,
  applying: settingsUpdateApplyingFixture,
  rolledBack: settingsUpdateRolledBackFixture,
} as const;

test.use({
  runtimeOptions: {
    env: {
      ...process.env,
      COMPOZY_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
    },
    extensionsAllowUnverified: true,
  },
});

test("operator can navigate the settings shell and complete a restart-aware general save that survives refresh polling", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);

  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/settings/general"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  const settingsUI = settingsOperatorSelectors(settingsWin);
  await expect(settingsUI.shell.shell).toBeVisible({ timeout: 20_000 });
  await expect(settingsUI.shell.sectionNav).toBeVisible({ timeout: 20_000 });

  await expect
    .poll(async () => normalizeTexts(await settingsUI.shell.sectionItems.allTextContents()))
    .toEqual([
      "General",
      "Appearance",
      "Layouts",
      "Palette",
      "Providers",
      "Memory",
      "Roles",
      "Skills",
      "Automation",
      "Network",
      "Notifications",
      "Diagnostics",
      "Remote access",
      "Hooks",
      "Extensions",
    ]);

  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/general");
  await expect(settingsUI.shell.sectionLink("general")).toHaveAttribute("aria-current", "page");
  await expect(settingsUI.general.page).toBeVisible();

  await settingsUI.shell.sectionLink("network").click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/network");
  await expect(settingsUI.shell.sectionLink("network")).toHaveAttribute("aria-current", "page");

  await settingsUI.shell.sectionLink("hooks").click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/hooks");
  await expect(settingsUI.shell.sectionActive("hooks")).toBeVisible();

  await settingsUI.shell.sectionLink("extensions").click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/extensions");
  await expect(settingsUI.shell.sectionActive("extensions")).toBeVisible();

  await appPage.goBack({ waitUntil: "domcontentloaded" });
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/hooks");
  await appPage.goForward({ waitUntil: "domcontentloaded" });
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/extensions");

  await settingsUI.shell.sectionLink("general").click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/general");
  await expect(settingsUI.general.page).toBeVisible();

  const nextTimeoutValue = await nextSessionTimeoutValue(settingsUI.general.sessionTimeoutInput);
  await settingsUI.general.sessionTimeoutInput.fill(nextTimeoutValue);
  await expect(settingsUI.general.saveButton).toBeEnabled();
  await settingsUI.general.saveButton.click();

  await expect(settingsUI.general.restartNotice).toBeVisible();
  await expect(settingsUI.general.restartNotice).toContainText("Restart needed");
  await expect(settingsUI.general.restartTrigger).toBeVisible();
  await browserArtifacts.captureScreenshot("tc-func-001-settings-shell-navigation", appPage);

  const restartResponse = appPage.waitForResponse(
    response =>
      new URL(response.url()).pathname === "/api/settings/actions/restart" &&
      response.request().method() === "POST"
  );
  await settingsUI.general.restartTrigger.click();
  const operationID = ((await (await restartResponse).json()) as { operation_id: string })
    .operation_id;
  expect(operationID).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);

  await expect(settingsUI.general.restartNotice).toContainText("Restarting…");
  await browserArtifacts.captureScreenshot("tc-func-002-general-restart-polling", appPage);

  await reloadDaemonServedPage(appPage, runtime, "/settings/general", {
    readyTestId: "settings-page-general",
  });
  if (!(await settingsUI.general.restartNotice.isVisible().catch(() => false))) {
    const payload = await runtime.requestJSON<{ status: string }>(
      `/api/settings/actions/restart/${encodeURIComponent(operationID)}`
    );
    expect(payload.status).toBe("ready");
  }

  await expect
    .poll(
      async () => {
        const payload = await runtime.requestJSON<{ status: string }>(
          `/api/settings/actions/restart/${encodeURIComponent(operationID)}`
        );
        return payload.status;
      },
      {
        timeout: 45_000,
      }
    )
    .toBe("ready");

  if (await settingsUI.general.restartNotice.isVisible().catch(() => false)) {
    await expect(settingsUI.general.restartNotice).toContainText("Restarted");
  }
  await browserArtifacts.captureScreenshot("tc-int-016-general-restart-ready", appPage);
});

test("Herdr E2E-017: shortcut alternates persist and refresh the live cheatsheet", async ({
  appPage,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-shortcuts-settings-"));
  const workspace = await runtime.resolveWorkspace(workspaceRoot);

  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await appPage.goto(runtime.url("/settings/layouts"), { waitUntil: "domcontentloaded" });

  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin.getByTestId("settings-page-layouts")).toBeVisible({ timeout: 20_000 });

  const newTab = settingsWin.getByTestId("window-manager-shortcut-window.tab.new");
  await newTab.getByRole("button", { name: "Add an alternate shortcut for New tab" }).click();
  const saveResponse = appPage.waitForResponse(
    response =>
      new URL(response.url()).pathname === "/api/settings/window-manager" &&
      response.request().method() === "PATCH"
  );
  await appPage.keyboard.press("Alt+r");
  expect((await saveResponse).ok()).toBe(true);
  await expect(newTab).toContainText("⌥R");

  await appPage.keyboard.press("Shift+/");
  const cheatsheet = appPage.getByTestId("os-shortcuts-dialog");
  await expect(cheatsheet).toBeVisible();
  await expect(cheatsheet.getByTestId("os-shortcut-row-window.tab.new")).toContainText("⌥R");
  await appPage.keyboard.press("Escape");

  await newTab.getByRole("button", { name: "New tab shortcut" }).click();
  await appPage.keyboard.press("Control+Alt+ArrowLeft");
  const conflict = settingsWin.getByTestId("shortcut-conflict-window.tab.new");
  await expect(conflict).toContainText("⌃⌥← is already used by Tile left half");
  await expect(conflict).toContainText("Overwriting leaves Tile left half unbound.");
  await expect(conflict.getByRole("button", { name: "Overwrite" })).toBeVisible();
  await expect(conflict.getByRole("button", { name: "Cancel" })).toBeVisible();
  await expect(settingsWin.getByTestId("window-manager-shortcut-sidebar.toggle")).toContainText(
    "Shadowed"
  );
  const resetResponse = appPage.waitForResponse(
    response =>
      new URL(response.url()).pathname === "/api/settings/window-manager" &&
      response.request().method() === "PATCH"
  );
  await newTab.getByRole("button", { name: "Reset New tab to its default shortcut" }).click();
  expect((await resetResponse).ok()).toBe(true);
});

test("Herdr E2E-018: Terminal preset previews, applies, reverts, and re-applies idempotently", async ({
  appPage,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-shortcuts-preset-"));
  const workspace = await runtime.resolveWorkspace(workspaceRoot);
  const before = await runtime.requestJSON<{
    config: { shortcuts: Record<string, string[]> } & Record<string, unknown>;
  }>("/api/settings/window-manager");

  try {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(sessionUI);
    await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await switchWorkspace(appPage, workspace.id, workspace.name);
    await appPage.goto(runtime.url("/settings/layouts"), { waitUntil: "domcontentloaded" });

    const settingsWin = appWindow(appPage, "settings");
    await expect(settingsWin.getByTestId("settings-page-layouts")).toBeVisible({
      timeout: 20_000,
    });
    const preset = settingsWin.getByTestId("terminal-shortcut-preset");
    await preset.getByRole("button", { name: "Preview" }).click();
    await expect(preset).toContainText("window.tab.jump");
    await expect(preset).toContainText("desktop.switch");
    await expect(preset).toContainText("⌘1–8");
    await expect(preset).toContainText("⌘1–9");
    await expect(preset).toContainText("Control+Alt can alias AltGr");

    const firstApplyResponse = appPage.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/settings/window-manager" &&
        response.request().method() === "PATCH"
    );
    await preset.getByRole("button", { name: "Apply preset" }).click();
    expect((await firstApplyResponse).ok()).toBe(true);
    await expect(preset).toContainText("Applied");
    await expect(preset.getByRole("button", { name: "Revert" })).toBeVisible();
    const revertResponse = appPage.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/settings/window-manager" &&
        response.request().method() === "PATCH"
    );
    await preset.getByRole("button", { name: "Revert" }).click();
    expect((await revertResponse).ok()).toBe(true);
    await expect(settingsWin.getByTestId("settings-page-layouts-save-bar")).toHaveCount(0);

    await preset.getByRole("button", { name: "Preview" }).click();
    const saveResponse = appPage.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/settings/window-manager" &&
        response.request().method() === "PATCH"
    );
    await preset.getByRole("button", { name: "Apply preset" }).click();
    expect((await saveResponse).ok()).toBe(true);

    await appPage.goto(runtime.url("/settings/general"), { waitUntil: "domcontentloaded" });
    await appPage.goto(runtime.url("/settings/layouts"), { waitUntil: "domcontentloaded" });
    await preset.getByRole("button", { name: "Preview" }).click();
    await expect(preset.getByRole("button", { name: "Apply preset" })).toBeDisabled();
  } finally {
    await runtime.requestJSON("/api/settings/window-manager", {
      method: "PATCH",
      body: JSON.stringify({ config: before.config }),
    });
  }
});

// Invariant: preset definitions are shared, but each active profile reads and
// persists its own default-on enablement exception.
// Owner: notification Settings browser journey.
// Canonical suite: Settings Playwright tests.
test("E2E-026: notification preset enablement follows the active profile", async ({
  appPage,
  runtime,
}) => {
  await runtime.requestJSON("/api/profiles", {
    body: JSON.stringify({ color: "#c26ad6", icon: "megaphone", name: "marketing" }),
    method: "POST",
  });
  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(appPage);
  await appPage.goto(runtime.url("/settings/hooks"), { waitUntil: "domcontentloaded" });

  const settingsWin = appWindow(appPage, "settings");
  const profileLabel = settingsWin.getByTestId("settings-page-hooks-notification-preset-profile");
  const taskTerminalToggle = settingsWin.getByTestId(
    "settings-page-hooks-notification-preset-row-task_terminal-toggle"
  );
  await expect(profileLabel).toContainText("default");
  await expect(taskTerminalToggle).toBeChecked();

  const disabled = appPage.waitForResponse(
    response =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname === "/api/notifications/presets/task_terminal/enablement"
  );
  await taskTerminalToggle.click();
  expect((await disabled).ok()).toBe(true);
  await expect(taskTerminalToggle).not.toBeChecked();

  const profiles = profilesOperatorSelectors(appPage);
  await profiles.switcher.click();
  await profiles.switcherOption("marketing").click();
  await expect(profileLabel).toContainText("marketing");
  await expect(taskTerminalToggle).toBeChecked();

  await profiles.switcher.click();
  await profiles.switcherOption("default").click();
  await expect(profileLabel).toContainText("default");
  await expect(taskTerminalToggle).not.toBeChecked();
});

test("operator can distinguish skills actions that apply now from policy changes that require restart", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const seeded = await seedBrowserSettingsFixtures(runtime, {
    disabledSkills: [browserSettingsOperatorFlowScenario.skills.disabledSkill],
  });

  try {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(sessionUI);
    await appPage.goto(runtime.url("/settings/skills"), { waitUntil: "domcontentloaded" });
    const settingsWin = appWindow(appPage, "settings");
    await expect(settingsWin).toBeVisible({ timeout: 20_000 });
    const settingsUI = settingsOperatorSelectors(settingsWin);

    await expect(settingsUI.skills.page).toBeVisible();
    await expect(settingsUI.skills.disabledList).toBeVisible();
    await expect(
      settingsUI.skills.disabledToggle(browserSettingsOperatorFlowScenario.skills.disabledSkill)
    ).toBeVisible();

    await settingsUI.skills
      .disabledToggle(browserSettingsOperatorFlowScenario.skills.disabledSkill)
      .click();
    await expect(settingsUI.skills.disabledSave).toBeEnabled();
    await settingsUI.skills.disabledSave.click();

    await expect(settingsUI.skills.disabledMessage).toContainText("applied immediately");
    await expect(settingsUI.skills.restartNotice).not.toBeVisible();

    await settingsUI.skills.operationalLink.click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/marketplace/skills");
    await expect.poll(() => new URL(appPage.url()).search).toBe("");
    await openAppWindow(appPage, "Settings", "settings");
    await settingsUI.shell.sectionLink("skills").click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/skills");

    await settingsWin
      .getByTestId("settings-page-skills-advanced")
      .getByTestId("settings-advanced-toggle")
      .click();
    await settingsUI.skills.policyRegistryInput.fill("clawhub");
    await settingsUI.skills.policyBaseURLInput.fill("https://skills.example/browser-updated");
    await expect(settingsUI.skills.save).toBeEnabled();
    await settingsUI.skills.save.click();

    await expect(settingsUI.skills.restartNotice).toBeVisible();
    await expect(settingsUI.skills.restartNotice).toContainText("Restart needed");
    await browserArtifacts.captureScreenshot("tc-func-005-skills-applied-now-vs-restart", appPage);
  } finally {
    await cleanupBrowserSettingsFixtures(runtime, seeded);
  }
});

test("operator can replace a builtin provider with a config overlay and delete it back to builtin fallback", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const builtinProviderName = await pickBuiltinProviderName(runtime);

  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/settings/providers"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  const settingsUI = settingsOperatorSelectors(settingsWin);

  await expect(settingsUI.providers.page).toBeVisible();
  await expect(settingsUI.providers.list).toBeVisible();
  await expect(settingsUI.providers.card(builtinProviderName)).toBeVisible();

  await settingsUI.providers.card(builtinProviderName).click();
  await settingsUI.providers.editorEdit.click();
  await expect(settingsUI.providers.editor).toBeVisible();
  await settingsUI.providers.editorCommandInput.fill(
    browserSettingsOperatorFlowScenario.providers.overlayCommand
  );
  // Runtime and model overrides live in the Advanced tier of the provider editor.
  await settingsUI.providers.editorModeAdvanced.click();
  await settingsUI.providers.editorModelInput.fill(
    browserSettingsOperatorFlowScenario.providers.overlayModel
  );
  await settingsUI.providers.editorSave.click();

  await expect(settingsUI.providers.editor).toBeHidden();
  await expect(settingsUI.providers.actionResult).toContainText(
    `Saved provider "${builtinProviderName}"`
  );
  await expect(settingsUI.providers.actionResult).toContainText("restart required");
  await expect(settingsUI.providers.cardCommand(builtinProviderName)).toContainText(
    browserSettingsOperatorFlowScenario.providers.overlayCommand
  );
  await settingsUI.providers.card(builtinProviderName).click();
  await expect(settingsUI.providers.inspectorSource).toContainText(/config/i);
  await settingsUI.providers.editorDelete.click();
  await expect(settingsUI.providers.deleteDialog).toBeVisible();
  await settingsUI.providers.deleteConfirm.click();

  await expect(settingsUI.providers.actionResult).toContainText(
    `Deleted overlay for "${builtinProviderName}"`
  );
  await expect(settingsUI.providers.actionResult).toContainText("builtin fallback now effective");
  await expect(settingsUI.providers.card(builtinProviderName)).toBeVisible();
  await settingsUI.providers.card(builtinProviderName).click();
  await expect(settingsUI.providers.inspectorSource).toContainText(/builtin/i);
  await browserArtifacts.captureScreenshot(
    "tc-func-008-providers-crud-and-builtin-fallback",
    appPage
  );
});

test("operator can manage MCP servers across global and workspace scopes with visible target semantics", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const settingsUI = settingsOperatorSelectors(appPage);
  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-settings-mcp-workspace-"));
  const workspace = await runtime.resolveWorkspace(workspaceRoot);

  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/marketplace/mcps"), {
    waitUntil: "domcontentloaded",
  });

  await expect(settingsUI.mcpServers.page).toBeVisible();

  await switchWorkspace(appPage, workspace.id, workspace.name);
  await appPage.goto(runtime.url("/marketplace/mcps"), {
    waitUntil: "domcontentloaded",
  });
  await expect(settingsUI.mcpServers.page).toBeVisible();

  await createMCPServerViaUI(settingsUI, {
    name: browserSettingsOperatorFlowScenario.mcpServers.workspace.name,
    command: browserSettingsOperatorFlowScenario.mcpServers.workspace.command,
    target: browserSettingsOperatorFlowScenario.mcpServers.workspace.target,
  });

  await expect(settingsUI.mcpServers.actionResult).toContainText(
    `Saved "${browserSettingsOperatorFlowScenario.mcpServers.workspace.name}" · workspace-config · applied now`
  );
  await expect(
    settingsUI.mcpServers.row(browserSettingsOperatorFlowScenario.mcpServers.workspace.name)
  ).toBeVisible();

  await setGlobalScope(appPage, true);
  await appPage.goto(runtime.url("/marketplace/mcps"), {
    waitUntil: "domcontentloaded",
  });
  await expect(settingsUI.mcpServers.page).toBeVisible();

  await createMCPServerViaUI(settingsUI, {
    name: browserSettingsOperatorFlowScenario.mcpServers.global.name,
    command: browserSettingsOperatorFlowScenario.mcpServers.global.command,
    target: browserSettingsOperatorFlowScenario.mcpServers.global.target,
  });

  await expect(settingsUI.mcpServers.actionResult).toContainText(
    `Saved "${browserSettingsOperatorFlowScenario.mcpServers.global.name}" · global-mcp-sidecar · applied now`
  );
  await expect(
    settingsUI.mcpServers.row(browserSettingsOperatorFlowScenario.mcpServers.global.name)
  ).toBeVisible();

  await setGlobalScope(appPage, false);
  await appPage.goto(runtime.url("/marketplace/mcps"), {
    waitUntil: "domcontentloaded",
  });
  await expect(
    settingsUI.mcpServers.row(browserSettingsOperatorFlowScenario.mcpServers.workspace.name)
  ).toBeVisible();

  await settingsUI.mcpServers
    .editRow(browserSettingsOperatorFlowScenario.mcpServers.workspace.name)
    .click();
  await appPage.getByRole("menuitem", { name: "Remove…" }).click();
  await appPage
    .getByLabel("Type to confirm")
    .fill(browserSettingsOperatorFlowScenario.mcpServers.workspace.name);
  await appPage
    .getByTestId(
      `marketplace-confirm-${browserSettingsOperatorFlowScenario.mcpServers.workspace.name}`
    )
    .click();

  await expect(settingsUI.mcpServers.actionResult).toContainText(
    `${browserSettingsOperatorFlowScenario.mcpServers.workspace.name} removed`
  );
  await expect(
    settingsUI.mcpServers.row(browserSettingsOperatorFlowScenario.mcpServers.workspace.name)
  ).not.toBeVisible();
  await browserArtifacts.captureScreenshot("tc-int-011-mcp-workspace-scope", appPage);
});

test("operator can manage restart-aware hooks and extension policy on split settings routes", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const seeded = await seedBrowserSettingsFixtures(runtime, {
    hooks: [
      {
        name: browserSettingsOperatorFlowScenario.hooks.hookName,
        declaration: {
          name: browserSettingsOperatorFlowScenario.hooks.hookName,
          event: "turn.end",
          mode: "sync",
          command: "/bin/echo",
          args: ["settings-hook"],
          matcher: {},
          required: true,
          enabled: true,
        },
      },
    ],
  });

  try {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(sessionUI);
    await appPage.goto(runtime.url("/settings/hooks"), {
      waitUntil: "domcontentloaded",
    });
    const settingsWin = appWindow(appPage, "settings");
    await expect(settingsWin).toBeVisible({ timeout: 20_000 });
    const settingsUI = settingsOperatorSelectors(settingsWin);

    await expect(settingsUI.hooks.page).toBeVisible();
    await expect(
      settingsUI.hooks.hookToggle(browserSettingsOperatorFlowScenario.hooks.hookName)
    ).toBeVisible();

    await settingsUI.hooks.hookToggle(browserSettingsOperatorFlowScenario.hooks.hookName).click();
    await expect(
      settingsUI.hooks.hookToggle(browserSettingsOperatorFlowScenario.hooks.hookName)
    ).not.toBeChecked();
    await expect(settingsUI.hooks.restartNotice).toBeVisible();

    // Prove failure-fatality (`required`) and dispatch enablement (`enabled`) are
    // independent fields: disabling the hook flips `enabled` to false while
    // `required` stays true in the real daemon response.
    const hooksResponse = await runtime.requestJSON<{
      hooks: Array<{ name: string; declaration: { enabled?: boolean; required?: boolean } }>;
    }>("/api/settings/hooks");
    const toggledHook = hooksResponse.hooks.find(
      hook => hook.name === browserSettingsOperatorFlowScenario.hooks.hookName
    );
    expect(toggledHook?.declaration.enabled).toBe(false);
    expect(toggledHook?.declaration.required).toBe(true);

    await appPage.goto(runtime.url("/settings/extensions"), {
      waitUntil: "domcontentloaded",
    });
    await expect(settingsUI.extensions.page).toBeVisible();
    await expect(settingsUI.hooks.hooksList).toHaveCount(0);

    await expect(settingsUI.extensions.githubEnabled).toBeChecked();
    await settingsUI.extensions.githubBaseURLInput.fill("https://github.example/api/v3");
    await expect(settingsUI.extensions.save).toBeEnabled();
    await settingsUI.extensions.save.click();

    await expect(settingsUI.extensions.restartNotice).toBeVisible();
    await browserArtifacts.captureScreenshot("tc-func-012-extensions-policy", appPage);
  } finally {
    await cleanupBrowserSettingsFixtures(runtime, seeded);
  }
});

test("operator routes a background role, persists it across reload, and keeps builtins out of the Agents fleet", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const settingsUI = settingsOperatorSelectors(appPage);
  const nextModel = "claude-haiku-4-5";
  const nextModelLabel = "Claude Haiku 4.5";

  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/settings/roles"), { waitUntil: "domcontentloaded" });

  await expect(settingsUI.roles.page).toBeVisible({ timeout: 20_000 });
  // Coordinator ships disabled — the header switch projects that truthfully
  // without expanding the row.
  await expect(settingsUI.roles.enabledSwitch("coordinator")).toHaveAttribute(
    "aria-checked",
    "false"
  );
  // Nothing is pinned yet, so no route chip is invented for auto_title.
  await expect(settingsUI.roles.routeSummary("auto_title")).toHaveCount(0);

  await settingsUI.roles.toggle("auto_title").click();
  await settingsUI.roles.runtimeSelect("auto_title").click();
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
  // Pick the provider-backed catalog row so the selector owns the complete
  // provider/model decision instead of treating a known model as a custom ID.
  await appPage.getByTestId("runtime-selector-search").fill(nextModel);
  await appPage
    .locator(`[role="option"][data-provider="opencode"][data-model="${nextModel}"]`)
    .click();
  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("runtime-selector-popup")).toHaveCount(0);
  await expect(settingsUI.roles.runtimeSelect("auto_title")).toContainText(nextModelLabel);
  await expect(settingsUI.roles.saveButton).toBeEnabled();

  const applyResponse = appPage.waitForResponse(
    response =>
      new URL(response.url()).pathname === "/api/settings/roles" &&
      response.request().method() === "PATCH"
  );
  await settingsUI.roles.saveButton.click();
  await applyResponse;
  await expect(settingsUI.roles.saveMessage).toContainText("applied immediately");
  await browserArtifacts.captureScreenshot("e2e-006-roles-auto-title-saved", appPage);

  // The routed model survives a reload — panel and daemon config agree.
  await reloadDaemonServedPage(appPage, runtime, "/settings/roles", {
    readyTestId: "settings-page-roles",
  });
  await expect(settingsUI.roles.routeSummary("auto_title")).toContainText(
    `opencode · ${nextModel}`
  );

  const section = await runtime.requestJSON<{ config: { auto_title: { model: string } } }>(
    "/api/settings/roles"
  );
  expect(section.config.auto_title.model).toBe(nextModel);

  // Virtual builtins never enter the Agents fleet.
  await appPage.goto(runtime.url("/agents"), { waitUntil: "domcontentloaded" });
  await expect(sessionUI.agentRow("general")).toBeVisible();
  await expect(sessionUI.agentRow("coordinator")).toHaveCount(0);
  await expect(sessionUI.agentRow("dreaming-curator")).toHaveCount(0);
  await browserArtifacts.captureScreenshot("e2e-006-agents-fleet-no-builtins", appPage);
});

/**
 * E2E-019 — the Updates section renders daemon truth for every two-track shape a
 * host can be in, in a plain browser with zero desktop-awareness (US-029 AC-1/AC-3,
 * EC-1, EC-3). The update projection describes the host install, and no real feed
 * exists in the harness, so each shape is served through the API boundary; every
 * assertion below is on what the SPA does with that truth.
 */
test("browser operator reads both update tracks, a managed runtime, and a headless host from daemon truth", async ({
  appPage,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);

  let updatePayload: unknown = updateFixtures.bothAvailable;
  await appPage.route("**/api/settings/update", async route => {
    await route.fulfill({ json: updatePayload });
  });

  await appPage.goto(runtime.url("/settings/general"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  const settingsUI = settingsOperatorSelectors(settingsWin);
  await expect(settingsUI.general.updates).toBeVisible({ timeout: 20_000 });

  // Both tracks available: two rows, both versions, both apply affordances.
  await expect(settingsUI.general.updateTrack("runtime")).toContainText("0.5.0");
  await expect(settingsUI.general.updateTrack("runtime")).toContainText("0.5.1");
  await expect(settingsUI.general.updateTrack("app")).toContainText("0.5.1");
  await expect(settingsUI.general.updateApply("runtime")).toBeVisible();
  await expect(settingsUI.general.updateApply("app")).toBeVisible();
  await expect(settingsUI.general.updateRelease("runtime")).toHaveAttribute(
    "href",
    "https://github.com/compozy/compozy/releases/tag/v0.5.1"
  );

  // Managed runtime: the recommendation is verbatim and apply is ABSENT, not disabled.
  updatePayload = updateFixtures.managed;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(settingsUI.general.updates).toBeVisible({ timeout: 20_000 });
  await expect(settingsUI.general.updateRecommendation).toContainText("brew upgrade compozy");
  await expect(settingsUI.general.updateApply("runtime")).toHaveCount(0);

  // Headless host: the app row is absent entirely, not an empty row.
  updatePayload = updateFixtures.noApp;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(settingsUI.general.updates).toBeVisible({ timeout: 20_000 });
  await expect(settingsUI.general.updateTrack("runtime")).toBeVisible();
  await expect(settingsUI.general.updateTrack("app")).toHaveCount(0);
});

/**
 * E2E-020 — applying the runtime from a plain browser: the apply call is issued
 * with its target, progress renders from the polled projection, and the terminal
 * outcome is read back rather than assumed (US-029 EC-2/EC-5).
 */
test("browser operator applies the runtime update and reads staged progress and terminal truth from the daemon", async ({
  appPage,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(sessionUI);

  let updatePayload: unknown = updateFixtures.bothAvailable;
  const applyTargets: string[] = [];
  await appPage.route("**/api/settings/update", async route => {
    await route.fulfill({ json: updatePayload });
  });
  await appPage.route("**/api/settings/update/apply", async route => {
    const body = route.request().postDataJSON() as { target: string };
    applyTargets.push(body.target);
    // Apply only acknowledges acquisition; the GET above becomes the progress feed.
    updatePayload = updateFixtures.applying;
    await route.fulfill({
      json: {
        target: body.target,
        status: "accepted",
        operation_id: "op-e2e",
        message: "Started the runtime update.",
        holder: null,
      },
    });
  });

  await appPage.goto(runtime.url("/settings/general"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  const settingsUI = settingsOperatorSelectors(settingsWin);
  await expect(settingsUI.general.updateApply("runtime")).toBeVisible({ timeout: 20_000 });

  await settingsUI.general.updateApply("runtime").click();
  await expect.poll(() => applyTargets).toEqual(["runtime"]);

  // Progress is the daemon's named phase, not an invented spinner label.
  await expect(settingsUI.general.updateProgress("runtime")).toContainText("install", {
    timeout: 20_000,
  });
  await expect(settingsUI.general.updateApply("runtime")).toHaveCount(0);

  // Terminal truth arrives through the polled read, including rollback.
  updatePayload = updateFixtures.rolledBack;
  await expect(settingsUI.general.updateRollback).toContainText("0.5.0", { timeout: 20_000 });
  await expect(settingsUI.general.updateLastError).toContainText("health check failed after swap");
});

function normalizeTexts(values: string[]): string[] {
  return values.map(value => value.trim()).filter(value => value !== "");
}

async function nextSessionTimeoutValue(
  input: ReturnType<typeof settingsOperatorSelectors>["general"]["sessionTimeoutInput"]
): Promise<string> {
  const currentValue = Number.parseInt((await input.inputValue()) || "0", 10);
  const primary = browserSettingsOperatorFlowScenario.general.primarySessionTimeoutSeconds;
  const fallback = browserSettingsOperatorFlowScenario.general.fallbackSessionTimeoutSeconds;
  return String(currentValue === primary ? fallback : primary);
}

async function pickBuiltinProviderName(runtime: {
  requestJSON<T>(pathname: string, init?: RequestInit): Promise<T>;
}) {
  const payload = await runtime.requestJSON<{
    providers: Array<{
      name: string;
      source_metadata: { effective_source: { kind: string } };
    }>;
  }>("/api/settings/providers");
  const builtinProvider =
    payload.providers.find(
      provider =>
        provider.name === "codex" &&
        provider.source_metadata.effective_source.kind === "builtin-provider"
    ) ??
    payload.providers.find(
      provider => provider.source_metadata.effective_source.kind === "builtin-provider"
    );

  if (!builtinProvider) {
    throw new Error("Expected at least one builtin provider in the settings providers list.");
  }

  return builtinProvider.name;
}

async function createMCPServerViaUI(
  settingsUI: ReturnType<typeof settingsOperatorSelectors>,
  input: {
    command: string;
    name: string;
    target: "auto" | "config" | "sidecar";
  }
) {
  await settingsUI.mcpServers.create.click();
  await expect(settingsUI.mcpServers.editor).toBeVisible();
  await settingsUI.mcpServers.editorNameInput.fill(input.name);
  await settingsUI.mcpServers.editorCommandInput.fill(input.command);
  await settingsUI.mcpServers.editorTargetInput.selectOption(input.target);
  await settingsUI.mcpServers.editorSave.click();
  await expect(settingsUI.mcpServers.editor).toBeHidden();
}
