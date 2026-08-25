import path from "node:path";
import { fileURLToPath } from "node:url";

import { appWindow, openAppWindow, sessionWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import {
  deleteExposeLink,
  exposeLinkPath,
  pathExists,
  readLinkTarget,
  replaceWithForeignLink,
  seedCustomSkillRoot,
  seedProviderSkill,
  writeSkillDefinition,
} from "../fixtures/skill-sources";
import {
  marketplaceOperatorSelectors,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
  settingsOperatorSelectors,
} from "../fixtures/selectors";
import {
  skillExposeSelectors,
  skillSourceSettingsSelectors,
} from "../fixtures/skill-source-selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

/**
 * Skill sources, end to end against a real daemon.
 *
 * Every assertion here is about daemon truth reaching a surface: counts the
 * scanner measured, origins the catalog attributed, and expose links reconciled
 * from the filesystem. The fixtures write real folders and real symlinks because
 * that is the only way those states can occur.
 */

const mockAgentFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_lifecycle_fixture.json"
);

const MOCK_AGENT = "browser-lifecycle-agent";
const ABSORBED_SKILL = "commit-hygiene";
const CLAUDE_SKILL = "release-notes";
const CUSTOM_SKILL = "burn-review";
const NATIVE_SKILL = "review-checklist";

test.use({
  runtimeOptions: {
    seed: { mockAgents: [{ fixturePath: mockAgentFixture, fixtureAgent: MOCK_AGENT }] },
  },
});

interface SettingsSkillsEnvelope {
  config: { sources: string[]; custom_sources: string[] };
  inherits?: { sources: boolean; custom_sources: boolean } | null;
  sources: {
    slug: string;
    enabled: boolean;
    roots: { path: string; exists: boolean; skill_count?: number | null }[];
  }[];
}

function requirePaths(runtime: BrowserRuntime) {
  if (!runtime.paths) {
    throw new Error("Skill-source browser E2E requires launch-mode runtime paths.");
  }
  return runtime.paths;
}

function settingsPath(query = ""): string {
  return `/api/settings/skills${query}`;
}

async function openSkillsSettings(
  appPage: import("@playwright/test").Page,
  runtime: BrowserRuntime
) {
  await appPage.goto(runtime.url("/settings/skills"), { waitUntil: "domcontentloaded" });
  await expect(settingsOperatorSelectors(appPage).skills.page).toBeVisible();
  return skillSourceSettingsSelectors(appPage);
}

test.describe("skill sources", () => {
  test("E2E-007: toggling a source applies live, counts follow, and the picker follows", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const paths = requirePaths(runtime);
    await seedProviderSkill(paths.operatorHomeDir, "claude", CLAUDE_SKILL);
    await ensureProjectWorkspace(appPage, runtime);

    const sources = await openSkillsSettings(appPage, runtime);
    // Claude ships off, so its skills are absent before the toggle.
    await expect(sources.row("claude")).toBeVisible();
    await expect(sources.count("claude")).toHaveCount(0);

    await sources.toggle("claude").click();
    await expect(sources.save).toBeEnabled();
    await sources.save.click();
    await expect(sources.message).toContainText("applied immediately");

    // The count is the scanner's, so it only appears once a real pass measured it.
    await expect(sources.count("claude")).toContainText("1 skill", {
      timeout: 20_000,
    });
    await browserArtifacts.captureScreenshot("e2e-007-claude-source-enabled", appPage);

    const enabled = await runtime.requestJSON<SettingsSkillsEnvelope>(settingsPath());
    expect(enabled.config.sources).toContain("claude");

    await sources.toggle("claude").click();
    await sources.save.click();
    await expect(sources.message).toContainText("applied immediately");
    await expect
      .poll(
        async () =>
          (await runtime.requestJSON<SettingsSkillsEnvelope>(settingsPath())).config.sources,
        { timeout: 20_000 }
      )
      .not.toContain("claude");
    await expectSkillAbsentFromPicker(appPage, CLAUDE_SKILL);
  });

  test("E2E-008: a custom folder is added, refuses a duplicate, and can be removed", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const paths = requirePaths(runtime);
    const customRoot = path.join(paths.operatorHomeDir, "team-skills");
    await seedCustomSkillRoot(customRoot, [CUSTOM_SKILL]);
    await ensureProjectWorkspace(appPage, runtime);

    const sources = await openSkillsSettings(appPage, runtime);
    await sources.customInput.fill(customRoot);
    await sources.customAdd.click();
    await sources.save.click();
    await expect(sources.message).toContainText("applied immediately");
    await expect(sources.count("team-skills")).toContainText("1 skill", {
      timeout: 20_000,
    });

    // The same folder twice is refused next to the input, before any request.
    await sources.customInput.fill(customRoot);
    await sources.customAdd.click();
    await expect(sources.customError).toContainText("already on the list");
    await expect(sources.customError).toContainText("duplicate_skill_source");
    await browserArtifacts.captureScreenshot("e2e-008-duplicate-source-refused", appPage);

    await sources.remove("team-skills").click();
    await sources.save.click();
    await expect
      .poll(
        async () =>
          (await runtime.requestJSON<SettingsSkillsEnvelope>(settingsPath())).config.custom_sources,
        { timeout: 20_000 }
      )
      .toEqual([]);
    await expectSkillAbsentFromPicker(appPage, CUSTOM_SKILL);
  });

  test("E2E-009: a workspace overrides one key and returns it to inheritance", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const paths = requirePaths(runtime);
    await seedProviderSkill(paths.operatorHomeDir, "claude", CLAUDE_SKILL);
    await ensureProjectWorkspace(appPage, runtime);
    const workspace = await runtime.resolveWorkspace(paths.workspaceDir);

    const sources = await openSkillsSettings(appPage, runtime);
    await sources.scopeWorkspace.click();
    await expect(sources.keyPosture("sources")).toContainText("inherited");
    await expect(sources.keyPosture("custom_sources")).toContainText("inherited");

    await sources.toggle("claude").click();
    await sources.save.click();
    await expect(sources.message).toContainText("applied immediately");
    await expect(sources.keyPosture("sources")).toContainText("custom for this workspace");
    // The untouched key never left inheritance.
    await expect(sources.keyPosture("custom_sources")).toContainText("inherited");
    await browserArtifacts.captureScreenshot("e2e-009-workspace-override", appPage);

    const workspaceView = await runtime.requestJSON<SettingsSkillsEnvelope>(
      settingsPath(`?scope=workspace&workspace_id=${encodeURIComponent(workspace.id)}`)
    );
    expect(workspaceView.inherits).toMatchObject({ sources: false, custom_sources: true });
    expect(workspaceView.config.sources).toContain("claude");

    // The user layer never moved, so every other workspace still inherits it.
    const userView = await runtime.requestJSON<SettingsSkillsEnvelope>(settingsPath());
    expect(userView.config.sources).not.toContain("claude");
    const otherRoot = path.join(paths.homeDir, "other-workspace");
    await writeSkillDefinition(
      path.join(otherRoot, ".compozy", "skills", "other-native"),
      "other-native",
      "Other workspace fixture"
    );
    const otherWorkspace = await runtime.resolveWorkspace(otherRoot);
    const otherWorkspaceView = await runtime.requestJSON<SettingsSkillsEnvelope>(
      settingsPath(`?scope=workspace&workspace_id=${encodeURIComponent(otherWorkspace.id)}`)
    );
    expect(otherWorkspaceView.inherits).toMatchObject({ sources: true, custom_sources: true });
    expect(otherWorkspaceView.config.sources).not.toContain("claude");

    await sources.keyUseInherited("sources").click();
    await expect(sources.keyPosture("sources")).toContainText("inherited", {
      timeout: 20_000,
    });
    const restored = await runtime.requestJSON<SettingsSkillsEnvelope>(
      settingsPath(`?scope=workspace&workspace_id=${encodeURIComponent(workspace.id)}`)
    );
    expect(restored.inherits).toMatchObject({ sources: true, custom_sources: true });
  });

  test("E2E-010: the picker labels an absorbed skill and leaves a native one unlabeled", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const paths = requirePaths(runtime);
    await seedProviderSkill(paths.operatorHomeDir, "agents", ABSORBED_SKILL);
    await writeSkillDefinition(
      path.join(paths.workspaceDir, ".compozy", "skills", NATIVE_SKILL),
      NATIVE_SKILL,
      "Pre-merge review checklist"
    );
    await ensureProjectWorkspace(appPage, runtime);

    const sessionId = await createMockSession(appPage);
    const sessionWin = sessionWindow(appPage, sessionId);
    const sessionUi = sessionWindowSelectors(sessionWin, appPage);
    await expect(sessionUi.composerTextarea).toBeVisible();

    await sessionUi.composerTextarea.fill("/");
    const menu = sessionWin.getByTestId("composer-command-menu");
    await expect(menu).toBeVisible();

    const absorbed = menu.getByTestId("composer-command-item").filter({ hasText: ABSORBED_SKILL });
    await expect(absorbed.locator("[data-slot='composer-command-origin']")).toHaveText("agents", {
      timeout: 20_000,
    });

    const native = menu.getByTestId("composer-command-item").filter({ hasText: NATIVE_SKILL });
    await expect(native).toBeVisible();
    await expect(native.locator("[data-slot='composer-command-origin']")).toHaveCount(0);
    await browserArtifacts.captureScreenshot("e2e-010-picker-origin-label", appPage);
  });

  test("E2E-011: an exposed skill reports missing, repairs, and never touches a foreign entry", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const paths = requirePaths(runtime);
    await writeSkillDefinition(
      path.join(paths.workspaceDir, ".compozy", "skills", NATIVE_SKILL),
      NATIVE_SKILL,
      "Pre-merge review checklist"
    );
    await ensureProjectWorkspace(appPage, runtime);

    await openInstalledSkillDetail(appPage, runtime, NATIVE_SKILL);
    const expose = skillExposeSelectors(appPage);
    await expect(expose.panel).toBeVisible();

    await expose.pickerTrigger.click();
    await expose.pickerOption("agents").click();
    await expose.pickerConfirm.click();

    await expect(expose.rowStatus("agents")).toHaveText("active", { timeout: 20_000 });
    const linkPath = exposeLinkPath(paths.operatorHomeDir, "agents", NATIVE_SKILL);
    expect(await readLinkTarget(linkPath)).not.toBeNull();
    await browserArtifacts.captureScreenshot("e2e-011-exposed-healthy", appPage);

    // Someone removes our link outside CompozyOS; the record outlives it.
    await deleteExposeLink(linkPath);
    await openInstalledSkillDetail(appPage, runtime, NATIVE_SKILL);
    await expect(expose.rowStatus("agents")).toHaveText("the link was deleted", {
      timeout: 20_000,
    });

    await expose.exposeAgain("agents").click();
    await expect(expose.rowStatus("agents")).toHaveText("active", { timeout: 20_000 });
    expect(await pathExists(linkPath)).toBe(true);

    // Another app takes the path. CompozyOS reports it and offers nothing.
    await replaceWithForeignLink(linkPath, path.join(paths.operatorHomeDir, "foreign-skill"));
    await openInstalledSkillDetail(appPage, runtime, NATIVE_SKILL);
    await expect(expose.row("agents")).toContainText("another app's file is there", {
      timeout: 20_000,
    });
    await expect(expose.row("agents").getByRole("button")).toHaveCount(0);
    await browserArtifacts.captureScreenshot("e2e-011-foreign-conflict", appPage);
  });
});

async function createMockSession(appPage: import("@playwright/test").Page): Promise<string> {
  const ui = sessionLifecycleSelectors(appPage);
  await completeOnboardingIfPrompted(ui);
  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const fleet = sessionLifecycleSelectors(agentsWin);
  await fleet.agentRow(MOCK_AGENT).click();
  await expect(fleet.agentPageNewSession).toBeVisible();
  await fleet.agentPageNewSession.click();

  const createResponse = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await appPage.getByTestId("session-create-submit").click();
  const payload = (await (await createResponse).json()) as { session?: { id?: string } };
  const sessionId = payload.session?.id ?? "";
  expect(sessionId).not.toBe("");
  return sessionId;
}

async function expectSkillAbsentFromPicker(
  appPage: import("@playwright/test").Page,
  skillName: string
): Promise<void> {
  const sessionId = await createMockSession(appPage);
  const sessionWin = sessionWindow(appPage, sessionId);
  const sessionUi = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionUi.composerTextarea).toBeVisible();
  await sessionUi.composerTextarea.fill("/");
  const menu = sessionWin.getByTestId("composer-command-menu");
  await expect(menu).toBeVisible();
  await expect(
    menu.getByTestId("composer-command-item").filter({ hasText: skillName })
  ).toHaveCount(0, { timeout: 20_000 });
}

async function openInstalledSkillDetail(
  appPage: import("@playwright/test").Page,
  runtime: BrowserRuntime,
  name: string
): Promise<void> {
  await appPage.goto(runtime.url("/marketplace/skills"), { waitUntil: "domcontentloaded" });
  const marketplaceWin = appWindow(appPage, "marketplace");
  await expect(marketplaceWin).toBeVisible();
  const marketplace = marketplaceOperatorSelectors(marketplaceWin);
  await expect(marketplace.kind("skill")).toBeVisible();
  await appPage.getByTestId("marketplace-kind-search-skill").fill(name);
  await appPage
    .getByTestId(/^marketplace-installed-card-/)
    .filter({ hasText: name })
    .first()
    .getByRole("link", { name: `View ${name} details` })
    .click();
  await expect(marketplace.detail).toContainText(name);
}
