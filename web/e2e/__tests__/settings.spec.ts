import { mkdtemp } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";

import { reloadDaemonServedPage } from "../fixtures/navigation";
import { openAppWindow, switchWorkspace } from "../fixtures/os-navigation";
import { settingsOperatorSelectors, sessionLifecycleSelectors } from "../fixtures/selectors";
import {
  browserSettingsOperatorFlowScenario,
  cleanupBrowserSettingsFixtures,
  seedBrowserSettingsFixtures,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

test.use({
  runtimeOptions: {
    env: {
      ...process.env,
      AGH_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
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
  await useGlobalWorkspaceIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/settings/general"), { waitUntil: "domcontentloaded" });
  const settingsWin = appPage.getByTestId("os-window-app:settings");
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
      "Providers",
      "Memory",
      "Roles",
      "Skills",
      "Automation",
      "Network",
      "Observability",
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
    await useGlobalWorkspaceIfPrompted(sessionUI);
    await appPage.goto(runtime.url("/settings/skills"), { waitUntil: "domcontentloaded" });
    const settingsWin = appPage.getByTestId("os-window-app:settings");
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
    await expect.poll(() => new URL(appPage.url()).search).toBe("?tab=installed");
    await openAppWindow(appPage, "Settings", "settings");
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
  await useGlobalWorkspaceIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/settings/providers"), { waitUntil: "domcontentloaded" });
  const settingsWin = appPage.getByTestId("os-window-app:settings");
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
  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "agh-settings-mcp-workspace-"));
  const workspace = await runtime.resolveWorkspace(workspaceRoot);

  await ensureGlobalWorkspace(runtime);
  await useGlobalWorkspaceIfPrompted(sessionUI);
  await appPage.goto(runtime.url("/marketplace/mcps?tab=installed"), {
    waitUntil: "domcontentloaded",
  });

  await expect(settingsUI.mcpServers.page).toBeVisible();
  await expect(settingsUI.mcpServers.scopeWorkspace).toBeVisible();
  await expect(settingsUI.mcpServers.scopeGlobal).toBeVisible();

  await switchWorkspace(appPage, workspace.id, workspace.name);
  await appPage.goto(runtime.url("/marketplace/mcps?tab=installed"), {
    waitUntil: "domcontentloaded",
  });
  await expect(settingsUI.mcpServers.page).toBeVisible();
  await settingsUI.mcpServers.scopeWorkspace.click();
  await expect(settingsUI.mcpServers.scopeWorkspace).toHaveAttribute("aria-pressed", "true");

  await settingsUI.mcpServers.scopeGlobal.click();
  await expect(settingsUI.mcpServers.scopeGlobal).toHaveAttribute("aria-pressed", "true");

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

  await settingsUI.mcpServers.scopeWorkspace.click();
  await expect(settingsUI.mcpServers.scopeWorkspace).toHaveAttribute("aria-pressed", "true");

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

  await expect(
    settingsUI.mcpServers.row(browserSettingsOperatorFlowScenario.mcpServers.global.name)
  ).toBeVisible();
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
    await useGlobalWorkspaceIfPrompted(sessionUI);
    await appPage.goto(runtime.url("/settings/hooks"), {
      waitUntil: "domcontentloaded",
    });
    const settingsWin = appPage.getByTestId("os-window-app:settings");
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

    await settingsUI.extensions.policyRegistryInput.fill("github");
    await settingsUI.extensions.policyBaseURLInput.fill(
      "https://extensions.example/browser-updated"
    );
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

  await ensureGlobalWorkspace(runtime);
  await useGlobalWorkspaceIfPrompted(sessionUI);
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
  // An inherit-mode role pins no provider, and a custom model id is never
  // emitted without one — target a real provider through the rail first.
  await appPage
    .locator(
      '[data-testid="runtime-selector-popup"] [role="radio"][data-rail]:not([data-rail="all"]):not([data-rail="fav"])'
    )
    .first()
    .click();
  await appPage.getByTestId("runtime-selector-search").fill(nextModel);
  await appPage.getByTestId("runtime-selector-custom").click();
  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("runtime-selector-popup")).toHaveCount(0);
  await expect(settingsUI.roles.runtimeSelect("auto_title")).toContainText(nextModel);
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
  await expect(settingsUI.roles.routeSummary("auto_title")).toContainText(nextModel);

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
