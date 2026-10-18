import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { access, mkdir, mkdtemp, readFile, rename, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { expect, test } from "../fixtures/test";
import {
  openAppWindow,
  openCommandPalette,
  openSessionsCatalog,
  paletteView,
  switchWorkspace,
} from "../fixtures/os-navigation";
import { profilesOperatorSelectors } from "../fixtures/selectors";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";
import { createWorktreeRepo } from "../fixtures/worktree-repo";
import type { BrowserRuntime } from "../fixtures/runtime";

/**
 * Profiles — the who-is-working context.
 *
 * Every journey runs against a real daemon, so what these assert is the shipped
 * contract rather than a fixture: the switcher stays quiet until a second
 * profile exists, a switch persists through the canonical selection route, and
 * every lifecycle dialog renders exactly what its plan endpoint returned.
 */

interface ProfileRow {
  name: string;
  state: string;
}

const fixtureRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata"
);
const profilesAgent = "cost-provenance-agent";
const profilesAgentFixture = path.join(fixtureRoot, "cost_provenance_fixture.json");
const profilesUsagePrompt = "Summarize the cost provenance run";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [{ fixtureAgent: profilesAgent, fixturePath: profilesAgentFixture }],
    },
  },
});

async function listProfiles(runtime: BrowserRuntime): Promise<ProfileRow[]> {
  return await runtime.requestJSON<ProfileRow[]>("/api/profiles");
}

async function createProfile(runtime: BrowserRuntime, name: string, color: string, icon: string) {
  await runtime.requestJSON("/api/profiles", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name, color, icon }),
  });
}

async function archiveProfile(runtime: BrowserRuntime, name: string): Promise<void> {
  const plan = await runtime.requestJSON<{ revision: string }>(
    `/api/profiles/${encodeURIComponent(name)}/archive-plan`
  );
  await runtime.requestJSON(`/api/profiles/${encodeURIComponent(name)}/archive`, {
    method: "POST",
    body: JSON.stringify({ plan_revision: plan.revision }),
  });
}

async function openProfilesSettings(page: Parameters<typeof openAppWindow>[0]) {
  const settings = await openAppWindow(page, "Settings", "settings");
  await settings.getByTestId("settings-section-nav").getByText("Profiles", { exact: true }).click();
  return settings;
}

interface DesktopSnapshot {
  revision: number;
  desktops: Array<{ id: string; name: string }>;
}

function windowManagerPath(workspaceId: string, profile: string, suffix = ""): string {
  const query = new URLSearchParams({ profile });
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/window-manager${suffix}?${query.toString()}`;
}

/** Desks as one profile sees them — the same scoped read the shell performs. */
async function desktopsOf(
  runtime: BrowserRuntime,
  workspaceId: string,
  profile: string
): Promise<DesktopSnapshot> {
  return await runtime.requestJSON<DesktopSnapshot>(windowManagerPath(workspaceId, profile));
}

async function addDesktop(
  runtime: BrowserRuntime,
  workspaceId: string,
  profile: string,
  name: string
): Promise<DesktopSnapshot> {
  const snapshot = await desktopsOf(runtime, workspaceId, profile);
  const result = await runtime.requestJSON<{ snapshot: DesktopSnapshot }>(
    windowManagerPath(workspaceId, profile, "/commands"),
    {
      method: "POST",
      body: JSON.stringify({
        workspace_id: workspaceId,
        command_id: "desktop.create",
        expected_revision: snapshot.revision,
        actor: { kind: "e2e", id: "profiles" },
        origin: "web-e2e",
        payload: { desktop_id: "", name },
      }),
    }
  );
  return result.snapshot;
}

function desktopNames(snapshot: DesktopSnapshot): string[] {
  return snapshot.desktops.map(desktop => desktop.name);
}

/**
 * Tabs until the given control has focus, failing if it never arrives.
 *
 * Keyboard journeys assert reachability rather than a hardcoded stop count: the
 * contract is that the operator *can* get there with Tab, not that a particular
 * number of presses does it.
 */
async function tabUntilFocused(
  page: Page,
  target: Locator,
  limit: number,
  key: "Tab" | "Shift+Tab" = "Tab"
): Promise<void> {
  for (let press = 0; press < limit; press += 1) {
    const focused = await target
      .evaluate((node: Element) => node === document.activeElement)
      .catch(() => false);
    if (focused) return;
    await page.keyboard.press(key);
  }
  await expect(target).toBeFocused();
}

/** The workspace the shell is running in — desks are per (workspace, profile). */
async function activeWorkspaceId(runtime: BrowserRuntime): Promise<string> {
  const seeded = runtime.seeded.workspace?.id.trim();
  if (seeded) return seeded;

  const workspaceDir = runtime.paths?.workspaceDir?.trim();
  if (workspaceDir) return (await runtime.resolveWorkspace(workspaceDir)).id;

  const payload = await runtime.requestJSON<{ workspaces: Array<{ id: string }> }>(
    "/api/workspaces"
  );
  const workspace = payload.workspaces[0];
  if (!workspace) throw new Error("the desktop journey requires one resolved workspace");
  return workspace.id;
}

async function availableAgentName(
  runtime: BrowserRuntime,
  workspaceId: string,
  profile: string
): Promise<string> {
  const catalog = await runtime.requestJSON<{ agents: Array<{ name: string }> }>(
    `/api/agents?workspace=${encodeURIComponent(workspaceId)}&profile=${encodeURIComponent(profile)}`
  );
  const agent = catalog.agents.find(candidate => candidate.name === profilesAgent);
  if (!agent) {
    throw new Error(`profile ${profile} has no agent available in workspace ${workspaceId}`);
  }
  return agent.name;
}

async function recordDefaultProfileUsage(
  runtime: BrowserRuntime,
  workspaceId: string
): Promise<void> {
  const agentName = await availableAgentName(runtime, workspaceId, "default");
  const created = await runtime.requestJSON<{ session: { id: string } }>(
    "/api/sessions?profile=default",
    {
      method: "POST",
      body: JSON.stringify({ agent_name: agentName, workspace: workspaceId }),
    }
  );
  const promptResponse = await fetch(
    runtime.url(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(created.session.id)}/prompt`
    ),
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        idempotency_key: randomUUID(),
        message: profilesUsagePrompt,
        message_id: randomUUID(),
      }),
    }
  );
  expect(promptResponse.ok).toBe(true);
  await promptResponse.text();
  await expect
    .poll(async () => {
      const overview = await runtime.requestJSON<{
        overview: { usage: { profiles: Array<{ profile_name: string; tokens: number }> } };
      }>(
        `/api/observe/overview?workspace=${encodeURIComponent(workspaceId)}&usage_window=30&all_profiles=true`
      );
      return (
        overview.overview.usage.profiles.find(entry => entry.profile_name === "default")?.tokens ??
        0
      );
    })
    .toBeGreaterThan(0);
}

async function createDefaultProfileSession(
  runtime: BrowserRuntime,
  workspaceId: string
): Promise<void> {
  const agentName = await availableAgentName(runtime, workspaceId, "default");
  await runtime.requestJSON("/api/sessions?profile=default", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspaceId }),
  });
}

test.describe("Profiles", () => {
  // Invariant: cold entry exposes the reserved owner's remedy without silently switching it.
  // Owner: Web profile recovery; canonical suite: Profiles E2E-031.
  test("E2E-031: unavailable profile entry explains recovery and permits an explicit switch", async ({
    appPage,
    runtime,
  }) => {
    const homeDir = runtime.paths?.homeDir;
    if (!homeDir) throw new Error("Profile recovery requires the managed runtime home.");
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "recovery-notes", "#527b67", "notebook-pen");
    const ui = profilesOperatorSelectors(appPage);
    await ui.switcher.click();
    await ui.switcherOption("recovery-notes").click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: recovery-notes");

    const destination = path.join(homeDir, "profiles", "recovery-guides");
    await mkdir(destination, { recursive: true });
    const plan = await runtime.requestJSON<{ revision: string }>(
      "/api/profiles/recovery-notes/rename-plan?new_name=recovery-guides"
    );
    const refused = await fetch(runtime.url("/api/profiles/recovery-notes/rename"), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        new_name: "recovery-guides",
        plan_revision: plan.revision,
        repos: [],
      }),
    });
    expect(refused.ok).toBe(false);
    const operations =
      await runtime.requestJSON<Array<{ id: string; status: string; profile: string }>>(
        "/api/profiles/ops"
      );
    const failed = operations.find(
      op => op.profile === "recovery-guides" && op.status === "failed"
    );
    expect(failed).toBeDefined();

    await appPage.goto(runtime.url("/settings/profiles"));
    await expect(ui.switcher).toHaveAccessibleName("Profile: recovery-guides");
    const status = appPage.getByTestId("os-window-manager-status");
    await expect(status).toContainText(/profile needs recovery/i);
    await status.hover();
    const detail = appPage.locator('[data-slot="tooltip-content"]');
    await expect(detail).toBeVisible();
    await expect(detail).toContainText(failed!.id);
    await expect(detail).toContainText("profile ops");
    const selections =
      await runtime.requestJSON<Array<{ profile: string }>>("/api/profiles/selection");
    expect(selections.some(selection => selection.profile === "recovery-guides")).toBe(true);

    await ui.switcher.click();
    await ui.switcherOption("default").click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
    await expect(
      (await openProfilesSettings(appPage)).getByRole("button", { name: "Create profile" })
    ).toBeVisible();

    await rename(destination, path.join(homeDir, "preserved-import"));
    await runtime.requestJSON(`/api/profiles/ops/${encodeURIComponent(failed!.id)}/retry`, {
      method: "POST",
      body: "{}",
    });
    await ui.switcher.click();
    await ui.switcherOption("recovery-guides").click();
    await appPage.reload();
    await expect(ui.switcher).toHaveAccessibleName("Profile: recovery-guides");
    await expect(
      (await openProfilesSettings(appPage)).getByRole("button", { name: "Create profile" })
    ).toBeVisible();
    await expect(status).toHaveCount(0);
  });

  // The global lifecycle feed must survive the removed desktop's authority loss.
  test("E2E-030: external deletion recovers the viewed profile without reloading", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "retained", "#4cb782", "briefcase");
    await createProfile(runtime, "release-drafts", "#4ea7fc", "notebook-pen");
    const ui = profilesOperatorSelectors(appPage);
    await ui.switcher.click();
    await ui.switcherOption("release-drafts").click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: release-drafts");
    await openAppWindow(appPage, "Home", "dashboard");

    const plan = await runtime.requestJSON<{ revision: string }>(
      "/api/profiles/release-drafts/delete-plan"
    );
    const deleted = await runtime.requestJSON<{ deleted: boolean }>(
      `/api/profiles/release-drafts?plan_revision=${encodeURIComponent(plan.revision)}`,
      { method: "DELETE" }
    );
    expect(deleted.deleted).toBe(true);
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
    await ui.switcher.click();
    await expect(ui.switcherOption("release-drafts")).toHaveCount(0);
    await expect(ui.switcherOption("retained")).toBeVisible();
    expect((await listProfiles(runtime)).map(profile => profile.name)).not.toContain(
      "release-drafts"
    );
  });

  // Invariant: project entry restores its remembered profile, including after aggregate viewing.
  // Owner: Web profile selection; canonical suite: Profiles E2E-013.
  test("E2E-013: switcher stays quiet, then carries identity, switch, and per-project remembered profile", async ({
    appPage,
    runtime,
  }) => {
    const originalId = await activeWorkspaceId(runtime);
    const { workspaces } = await runtime.requestJSON<{
      workspaces: Array<{ id: string; name: string }>;
    }>("/api/workspaces");
    const originalWorkspace = workspaces.find(workspace => workspace.id === originalId);
    if (!originalWorkspace) throw new Error("the profile journey requires its initial project");
    const secondRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-profile-switch-"));
    const secondWorkspace = await runtime.resolveWorkspace(secondRoot);

    // Bundled extensions can create profiles; arrange the single-profile state.
    for (const profile of await listProfiles(runtime)) {
      if (profile.name !== "default" && profile.state === "active") {
        await archiveProfile(runtime, profile.name);
      }
    }
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    const ui = profilesOperatorSelectors(appPage);

    // Quiet single: default alone renders a neutral icon button, no name.
    await expect(ui.switcher).toBeVisible();
    await expect(ui.switcher).toHaveAccessibleName("Profile");

    await ui.switcher.click();
    await expect(ui.switcherMenu).toBeVisible();
    await expect(ui.switcherMenu).toContainText(
      "Work is separate per profile. Project folders and machine tools are shared."
    );
    await ui.switcherCreate.click();
    await expect(ui.createDialog).toBeVisible();
    await ui.createName.fill("marketing");
    await ui.createPicker.getByRole("button", { name: "Icons" }).click();
    await ui.createPicker.getByRole("option", { name: "Violet" }).click();
    const created = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/profiles")
    );
    await ui.createConfirm.click();
    expect((await created).ok()).toBe(true);
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");

    const workspaceControl = appPage.locator('[data-slot="os-menubar-workspace"]');
    await workspaceControl.click();
    const workspaceOptions = appPage.locator('[data-testid^="os-workspace-option-"]');
    await expect(workspaceOptions.first()).toBeVisible();
    const marketingWorkspaces = await workspaceOptions.allTextContents();
    await appPage.keyboard.press("Escape");
    await expect(workspaceOptions).toHaveCount(0);

    await ui.switcher.click();
    await ui.switcherOption("default").click();
    await workspaceControl.click();
    await expect(workspaceOptions.first()).toBeVisible();
    expect(await workspaceOptions.allTextContents()).toEqual(marketingWorkspaces);
    await appPage.keyboard.press("Escape");
    await expect(workspaceOptions).toHaveCount(0);

    await ui.switcher.click();
    const selectionResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" && response.url().endsWith("/api/profiles/selection")
    );
    await ui.switcherOption("marketing").click();
    expect((await selectionResponse).ok()).toBe(true);
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");

    // The remembered choice is daemon state, so the terminal sees it too.
    const remembered =
      await runtime.requestJSON<Array<{ profile: string }>>("/api/profiles/selection");
    expect(remembered.some(entry => entry.profile === "marketing")).toBe(true);

    // A different project starts from its own slot, and returning restores this one.
    await switchWorkspace(appPage, secondWorkspace.id, secondWorkspace.name);
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
    await switchWorkspace(appPage, originalWorkspace.id, originalWorkspace.name);
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");

    // Aggregate is an ephemeral view; re-entry uses the remembered real identity.
    await ui.switcher.click();
    await ui.switcherAll.click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: All profiles");
    await switchWorkspace(appPage, secondWorkspace.id, secondWorkspace.name);
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
    await switchWorkspace(appPage, originalWorkspace.id, originalWorkspace.name);
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");

    // A fresh client resolves the same remembered choice.
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");

    // An external choice for an inactive project applies on entry, even with cached data.
    await switchWorkspace(appPage, secondWorkspace.id, secondWorkspace.name);
    await runtime.requestJSON("/api/profiles/selection", {
      method: "PUT",
      body: JSON.stringify({
        scope: "workspace",
        workspace_id: originalWorkspace.id,
        profile: "default",
      }),
    });
    await switchWorkspace(appPage, originalWorkspace.id, originalWorkspace.name);
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
  });

  test("E2E-014: settings lists, creates, and edits identity behind disclosure", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "consulting", "#4ea7fc", "briefcase");

    const settings = await openProfilesSettings(appPage);
    const ui = profilesOperatorSelectors(appPage, settings);

    await expect(ui.page).toBeVisible();
    // The page's only prose is the honest sentence about what a profile is not.
    await expect(ui.pageLine).toHaveText(
      "Profiles keep work separate. They are not a security boundary."
    );
    await expect(ui.row("default")).toBeVisible();
    await expect(ui.row("consulting")).toBeVisible();

    await ui.editIdentityRow("consulting").click();
    await expect(ui.identityDialog).toBeVisible();
    const color = ui.identityPicker.getByLabel("Custom color", { exact: true });
    await color.fill("12ZZ");
    await expect(ui.identityDialog).toContainText("Enter a color like #4ea7fc.");
    await expect(ui.identityConfirm).toBeDisabled();

    await color.fill("4CB782");
    await ui.identityPicker.getByRole("button", { name: "Emojis" }).click();
    // The composed picker must navigate results and commit identity from the keyboard.
    const emojiSearch = ui.identityPicker.getByRole("searchbox", { name: "Search emojis" });
    await emojiSearch.fill("book");
    await expect(
      ui.identityPicker.getByRole("gridcell", { name: "Notebook with decorative cover" })
    ).toHaveAttribute("aria-selected", "true");
    await emojiSearch.press("ArrowRight");
    await expect(ui.identityPicker.getByRole("gridcell", { name: "Closed book" })).toHaveAttribute(
      "aria-selected",
      "true"
    );
    await emojiSearch.press("ArrowRight");
    await expect(ui.identityPicker.getByRole("gridcell", { name: "Open book" })).toHaveAttribute(
      "aria-selected",
      "true"
    );
    await emojiSearch.press("Enter");
    const updated = appPage.waitForResponse(
      response =>
        response.request().method() === "PATCH" &&
        response.url().endsWith("/api/profiles/consulting")
    );
    await ui.identityConfirm.click();
    const identityResponse = await updated;
    expect(identityResponse.ok()).toBe(true);
    expect(await identityResponse.json()).toMatchObject({ emoji: "📖", icon: null });
    await expect(ui.identityDialog).not.toBeVisible();

    await ui.createOpen.click();
    await expect(ui.createDialog).toBeVisible();
    // Creation is never blank: a starter identity is already picked.
    await expect(ui.createPicker).toBeVisible();

    // An empty name is refused inline rather than at the daemon.
    await ui.createConfirm.click();
    await expect(ui.createDialog).toContainText("Give the profile a name.");

    // Server refusals stay in the dialog, without a second error notification.
    for (const name of ["default", "consulting"]) {
      const refused = appPage.waitForResponse(
        response =>
          response.request().method() === "POST" && response.url().endsWith("/api/profiles")
      );
      await ui.createName.fill(name);
      await ui.createConfirm.click();
      expect((await refused).ok()).toBe(false);
      await expect(ui.createName).toHaveAttribute("aria-invalid", "true");
      await expect(ui.createDialog.getByRole("alert")).toBeVisible();
      // Check while the inline refusal is visible, before any toast can expire.
      expect(await appPage.getByRole("button", { name: "Close toast", exact: true }).count()).toBe(
        0
      );
    }

    const created = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/profiles")
    );
    await ui.createName.fill("research");
    await ui.createConfirm.click();
    expect((await created).ok()).toBe(true);
    // Creation activates the new profile immediately. Desktops are profile
    // partitions, so reopen Settings in research instead of asserting against
    // the now-inactive consulting window.
    await expect(profilesOperatorSelectors(appPage).switcher).toHaveAccessibleName(
      "Profile: research"
    );
    const researchSettings = await openProfilesSettings(appPage);
    const researchUI = profilesOperatorSelectors(appPage, researchSettings);
    await expect(researchUI.row("research")).toBeVisible();

    // Archived profiles are demoted to a disclosure, not shown by default.
    await archiveProfile(runtime, "research");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    const reopened = await openProfilesSettings(appPage);
    const after = profilesOperatorSelectors(appPage, reopened);
    await expect(after.pageArchived).toBeVisible();
    await after.pageArchived.click();
    await expect(after.archivedList).toContainText("research");
  });

  // Invariant: rename offers start selected, preserve explicit declines across
  // name edits, and move only accepted repository folders.
  // Owner: profile lifecycle browser composition; canonical suite: E2E-016.
  test("E2E-016: rename selects repository offers and preserves declined folders", async ({
    appPage,
    runtime,
  }) => {
    const repos = [await createWorktreeRepo(), await createWorktreeRepo()];
    try {
      for (const repo of repos) {
        const folder = path.join(repo.rootDir, ".compozy", "profiles", "dev");
        await mkdir(folder, { recursive: true });
        await writeFile(path.join(folder, "README.md"), "Development notes\n", "utf8");
        const options = {
          cwd: repo.rootDir,
          env: { ...process.env, GIT_CONFIG_GLOBAL: "/dev/null", GIT_CONFIG_SYSTEM: "/dev/null" },
        };
        execFileSync("git", ["add", ".compozy/profiles/dev/README.md"], options);
        execFileSync("git", ["commit", "-m", "add development profile notes"], options);
      }
      const workspaces = await Promise.all(
        repos.map(repo => runtime.resolveWorkspace(repo.rootDir))
      );
      await ensureProjectWorkspace(appPage, runtime);
      await completeOnboardingIfPrompted(appPage);
      await createProfile(runtime, "dev", "#4cb782", "wrench");

      const settings = await openProfilesSettings(appPage);
      const ui = profilesOperatorSelectors(appPage, settings);
      await ui.renameRow("dev").click();
      await expect(ui.renameDialog).toBeVisible();
      await ui.renameName.fill("eng");

      const plan = await runtime.requestJSON<{
        machine_folders: string[];
        repo_candidates: Array<{ workspace_id: string }>;
        revision: string;
      }>("/api/profiles/dev/rename-plan?new_name=eng");
      expect(plan.revision).not.toBe("");
      expect(plan.repo_candidates.map(candidate => candidate.workspace_id).sort()).toEqual(
        workspaces.map(workspace => workspace.id).sort()
      );
      await expect(ui.renamePlan).toBeVisible();
      if (plan.machine_folders.length > 0) {
        await expect(ui.renamePlan).toContainText("Machine folders");
      }

      const accepted = ui.renameRepo(workspaces[0]!.id).getByRole("checkbox");
      const declined = ui.renameRepo(workspaces[1]!.id).getByRole("checkbox");
      await expect(accepted).toBeChecked();
      await expect(declined).toBeChecked();
      await declined.uncheck();
      await ui.renameName.fill("engineering");
      await expect(ui.renameConfirm).toBeEnabled();
      await expect(accepted).toBeChecked();
      await expect(declined).not.toBeChecked();
      const currentPlan = await runtime.requestJSON<{ revision: string }>(
        "/api/profiles/dev/rename-plan?new_name=engineering"
      );

      const renamed = appPage.waitForResponse(
        response =>
          response.request().method() === "POST" &&
          response.url().endsWith("/api/profiles/dev/rename")
      );
      await ui.renameConfirm.click();
      const response = await renamed;
      expect(response.ok()).toBe(true);
      expect(response.request().postDataJSON()).toMatchObject({
        new_name: "engineering",
        plan_revision: currentPlan.revision,
        repos: [workspaces[0]!.id],
      });
      const profiles = await listProfiles(runtime);
      expect(profiles.map(profile => profile.name)).toContain("engineering");
      expect(profiles.map(profile => profile.name)).not.toContain("dev");
      await expect(
        readFile(path.join(repos[0]!.rootDir, ".compozy/profiles/engineering/README.md"), "utf8")
      ).resolves.toBe("Development notes\n");
      await expect(
        access(path.join(repos[0]!.rootDir, ".compozy/profiles/dev"))
      ).rejects.toMatchObject({ code: "ENOENT" });
      await expect(
        readFile(path.join(repos[1]!.rootDir, ".compozy/profiles/dev/README.md"), "utf8")
      ).resolves.toBe("Development notes\n");
      await expect(
        access(path.join(repos[1]!.rootDir, ".compozy/profiles/engineering"))
      ).rejects.toMatchObject({ code: "ENOENT" });
      await appPage.reload({ waitUntil: "domcontentloaded" });
      const reopened = await openProfilesSettings(appPage);
      await expect(profilesOperatorSelectors(appPage, reopened).row("engineering")).toBeVisible();
    } finally {
      for (const repo of repos) await repo.cleanup();
    }
  });

  test("E2E-017: archive names what pauses, blocks on running work, and unarchive lists reactivation", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "finance", "#e8b04a", "gem");

    const settings = await openProfilesSettings(appPage);
    const ui = profilesOperatorSelectors(appPage, settings);

    await ui.archiveRow("finance").click();
    await expect(ui.archiveDialog).toBeVisible();
    // Archive destroys nothing, so it reads calm rather than dangerous.
    await expect(ui.archiveDialog).toContainText("Nothing is deleted.");

    // Invariant: browser history dismisses a lifecycle flow without mutating its owner.
    // Owner: shell/dialog browser composition; canonical suite: E2E-017.
    await appPage.goBack();
    await expect(appPage).toHaveURL(/\/settings\/general$/);
    await expect(ui.archiveDialog).not.toBeVisible();
    expect((await listProfiles(runtime)).find(profile => profile.name === "finance")).toMatchObject(
      {
        state: "active",
      }
    );
    await appPage.goForward();
    await expect(appPage).toHaveURL(/\/settings\/profiles$/);
    await expect(ui.archiveDialog).not.toBeVisible();
    await ui.archiveRow("finance").click();
    await expect(ui.archiveDialog).toBeVisible();

    const archived = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().endsWith("/api/profiles/finance/archive")
    );
    await ui.archiveConfirm.click();
    expect((await archived).ok()).toBe(true);

    await ui.pageArchived.click();
    await expect(ui.archivedList).toContainText("finance");

    // Unarchive confirms first, then reports what stays paused — the runtime has
    // no unarchive plan to preview, so the dialog never invents one.
    await ui.unarchiveRow("finance").click();
    await expect(ui.unarchiveDialog).toBeVisible();
    await expect(ui.unarchiveDialog).toContainText("stay paused until you");
    const unarchived = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().endsWith("/api/profiles/finance/unarchive")
    );
    await ui.unarchiveConfirm.click();
    expect((await unarchived).ok()).toBe(true);
    await expect(ui.unarchiveDialog).toContainText("finance is back");

    await appPage.goBack();
    await expect(appPage).toHaveURL(/\/settings\/general$/);
    await expect(ui.unarchiveDialog).not.toBeVisible();
    expect((await listProfiles(runtime)).find(profile => profile.name === "finance")).toMatchObject(
      {
        state: "active",
      }
    );
    await appPage.goForward();
    await expect(appPage).toHaveURL(/\/settings\/profiles$/);
    await expect(ui.unarchiveDialog).not.toBeVisible();
  });

  test("E2E-027: the palette Profiles view switches and hands lifecycle to the canonical dialogs", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const palette = await openCommandPalette(appPage);
    await palette.getByRole("combobox").fill("Profiles");
    await palette.getByTestId("os-palette-command-palette.view.profiles").click();

    const view = paletteView(appPage, "profiles");
    await expect(view).toBeVisible();

    const ui = profilesOperatorSelectors(appPage);
    // The current profile is marked; every row keeps its identity.
    await expect(ui.paletteRow("default")).toBeVisible();
    await expect(ui.paletteRow("marketing")).toBeVisible();

    const selectionResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" && response.url().endsWith("/api/profiles/selection")
    );
    await ui.paletteRow("marketing").click();
    expect((await selectionResponse).ok()).toBe(true);
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");
    await expect(palette).toBeHidden();

    // A lifecycle action collects only its target, then opens the canonical
    // dialog; the palette owns no plan or mutation behavior.
    const reopened = await openCommandPalette(appPage);
    await reopened.getByRole("combobox").fill("Archive profile");
    await reopened.getByTestId("os-palette-command-profile.archive").click();
    const profileArgument = appPage.getByTestId("os-palette-arg-profile");
    await profileArgument.fill("marketing");
    await profileArgument.press("Enter");
    await expect(ui.archiveDialog).toBeVisible();
    await expect(ui.archiveDialog).toContainText("marketing");

    // Invariant: a consumed palette flow cannot reopen after cancellation and reload.
    // Owner: window-route/dialog composition; canonical suite: E2E-027.
    await ui.archiveDialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(ui.archiveDialog).toBeHidden();
    await expect(appPage).toHaveURL(/\/settings\/profiles$/);
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("settings-page-profiles-content")).toBeVisible();
    await expect(ui.archiveDialog).toBeHidden();
    expect(
      (await listProfiles(runtime)).find(profile => profile.name === "marketing")
    ).toMatchObject({ state: "active" });

    // Invariant: consuming a profile intent keeps both attached clients responsive.
    // Owner: window-route/dialog composition; canonical suite: E2E-027.
    const workspace = await activeWorkspaceId(runtime);
    const observer = await appPage.context().newPage();
    await observer.goto(appPage.url(), { waitUntil: "domcontentloaded" });
    await expect(profilesOperatorSelectors(observer).switcher).toHaveAccessibleName(
      "Profile: marketing"
    );
    await expect(observer.getByTestId("settings-page-profiles-content")).toBeVisible();
    await expect
      .poll(async () => {
        const clients = await runtime.requestJSON<Array<{ client_id: string }>>(
          `/api/cmd-palette/clients?workspace=${encodeURIComponent(workspace)}`
        );
        return clients.length;
      })
      .toBe(2);

    // Invariant: supplied lifecycle names reach the canonical form and are consumed on cancel.
    // Owner: palette/window/dialog composition; canonical suite: E2E-027.
    const createPalette = await openCommandPalette(appPage);
    await createPalette.getByRole("combobox").fill("Create profile");
    await createPalette.getByTestId("os-palette-command-profile.create").click();
    const nameArgument = appPage.getByTestId("os-palette-arg-name");
    await nameArgument.fill("dispatch-notes");
    await nameArgument.press("Enter");
    await expect(ui.createDialog).toBeVisible();
    await expect(ui.createName).toHaveValue("dispatch-notes");
    await ui.createDialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(appPage).toHaveURL(/\/settings\/profiles$/);
    await expect(ui.createDialog).toBeHidden();
    await expect(observer).toHaveURL(/\/settings\/profiles$/);
    await expect(observer.getByTestId("settings-page-profiles-content")).toBeVisible();
    await observer.close();

    const renamePalette = await openCommandPalette(appPage);
    await renamePalette.getByRole("combobox").fill("Rename profile");
    await renamePalette.getByTestId("os-palette-command-profile.rename").click();
    await appPage.getByTestId("os-palette-arg-profile").fill("marketing");
    const newNameArgument = appPage.getByTestId("os-palette-arg-new_name");
    await newNameArgument.fill("dispatch-team");
    await newNameArgument.press("Enter");
    await expect(ui.renameDialog).toBeVisible();
    await expect(ui.renameName).toHaveValue("dispatch-team");
    await expect(ui.renameConfirm).toBeEnabled();
    await ui.renameName.fill("");
    await expect(ui.renameName).toHaveValue("");
    await expect(ui.renameConfirm).toBeDisabled();
    await ui.renameDialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(appPage).toHaveURL(/\/settings\/profiles$/);
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("settings-page-profiles-content")).toBeVisible();
    await expect(ui.createDialog).toBeHidden();
    await expect(ui.renameDialog).toBeHidden();
    expect((await listProfiles(runtime)).some(profile => profile.name === "dispatch-notes")).toBe(
      false
    );

    // Invariant: delegated profile selection returns its result before rebinding the client.
    // Owner: real client-command/selection composition; canonical suite: E2E-027.
    await createProfile(runtime, "dispatch-client", "#22c55e", "folder");
    const attached = await runtime.requestJSON<Array<{ client_id: string }>>(
      `/api/cmd-palette/clients?workspace=${encodeURIComponent(workspace)}`
    );
    expect(attached).toHaveLength(1);
    if (!runtime.requestOperatorJSON) throw new Error("delegated selection requires operator UDS");
    const invoked = await runtime.requestOperatorJSON<{ status: string }>(
      "/api/cmd-palette/commands/profile.use/invoke?profile=marketing",
      {
        method: "POST",
        body: JSON.stringify({
          workspace,
          client: attached[0]!.client_id,
          args: { profile: "dispatch-client" },
        }),
      }
    );
    expect(invoked.status).toBe("ok");
    await expect(ui.switcher).toHaveAccessibleName("Profile: dispatch-client");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(ui.switcher).toHaveAccessibleName("Profile: dispatch-client");
  });

  test("E2E-018: a deep link into another profile's session names its owner and offers the switch", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "consulting", "#4ea7fc", "briefcase");
    const workspaceId = await activeWorkspaceId(runtime);
    const agentName = await availableAgentName(runtime, workspaceId, "consulting");
    const foreign = await runtime.requestJSON<{ session: { id: string; agent_name: string } }>(
      "/api/sessions?profile=consulting",
      {
        method: "POST",
        body: JSON.stringify({ agent_name: agentName, workspace: workspaceId }),
      }
    );
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const ui = profilesOperatorSelectors(appPage);
    await appPage.goto(runtime.url(`/session/${foreign.session.id}`), {
      waitUntil: "domcontentloaded",
    });

    // Informed, not blocked: the item resolves through the labeled aggregate read.
    await expect(ui.ownerBanner).toContainText("belongs to consulting");
    await ui.ownerBannerSwitch.click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: consulting");
  });

  test("E2E-021: usage is scoped per profile and breaks down by owner in the aggregate", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await recordDefaultProfileUsage(runtime, await activeWorkspaceId(runtime));
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const home = await openAppWindow(appPage, "Home", "dashboard");
    const scoped = profilesOperatorSelectors(appPage, home);
    // Scoped: figures cover this profile only, so no breakdown is offered.
    await expect(scoped.usageProfileShare).toHaveCount(0);

    const ui = profilesOperatorSelectors(appPage);
    await ui.switcher.click();
    await ui.switcherAll.click();
    await expect(scoped.usageProfileShare).toBeVisible();
    await expect(scoped.usageProfileShare).toContainText("default");
  });

  // Invariant: a repository profile declaration stays dormant until the
  // operator creates that profile, after which its content binds immediately.
  // Owner: workspace profile-adoption browser journey.
  // Canonical suite: profile Playwright tests.
  test("E2E-022: a repository profile hint adopts, binds, and disappears", async ({
    appPage,
    runtime,
  }) => {
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-profile-hint-"));
    const agentDir = path.join(
      workspaceRoot,
      ".compozy",
      "profiles",
      "dev",
      "agents",
      "browser-dev"
    );
    await mkdir(agentDir, { recursive: true });
    await writeFile(
      path.join(agentDir, "AGENT.md"),
      "---\nname: browser-dev\ncategory_path: [Browser]\n---\n\nOwn browser development work.\n",
      "utf8"
    );
    const workspace = await runtime.resolveWorkspace(workspaceRoot);

    await completeOnboardingIfPrompted(appPage);
    await switchWorkspace(appPage, workspace.id, workspace.name);

    const hint = appPage.getByTestId("workspace-profiles-hint");
    await expect(hint).toContainText(
      "This project includes settings for the profile “dev”. Create it to use them."
    );
    await hint.getByRole("button", { name: "Create dev" }).click();

    const profiles = profilesOperatorSelectors(appPage);
    await expect(profiles.createDialog).toBeVisible();
    await expect(profiles.createName).toHaveValue("dev");
    const created = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/profiles")
    );
    await profiles.createConfirm.click();
    expect((await created).ok()).toBe(true);
    await expect(hint).toHaveCount(0);

    await expect
      .poll(async () => {
        const detail = await runtime.requestJSON<{ agents: Array<{ name: string }> }>(
          `/api/agents?workspace=${encodeURIComponent(workspace.id)}&profile=dev`
        );
        return detail.agents.map(agent => agent.name);
      })
      .toContain("browser-dev");
  });

  // Invariant: scoped and aggregate profile views carry owner labels and the create destination.
  // Owner: browser/daemon profile composition; canonical suite: Profiles E2E-028 / E2E-015.
  test("E2E-028 / E2E-015: palette and listings re-scope, label owners, and state the destination", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await createDefaultProfileSession(runtime, await activeWorkspaceId(runtime));
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const ui = profilesOperatorSelectors(appPage);

    await test.step("E2E-028: cold scoped palette rebinds to aggregate sessions", async () => {
      const scopedCatalog = appPage.waitForResponse(
        response =>
          response.url().includes("/api/cmd-palette/commands") &&
          response.url().includes("profile=default")
      );
      await openCommandPalette(appPage);
      expect((await scopedCatalog).ok()).toBe(true);
      await appPage.keyboard.press("Escape");
      await expect(appPage.getByTestId("os-command-palette")).toBeHidden();

      const aggregateCatalog = appPage.waitForResponse(
        response =>
          response.url().includes("/api/cmd-palette/commands") &&
          response.url().includes("all_profiles=true")
      );
      await ui.switcher.click();
      await ui.switcherAll.click();
      const palette = await openCommandPalette(appPage);
      expect((await aggregateCatalog).ok()).toBe(true);
      await palette.getByRole("combobox").fill("Sessions");
      await palette.getByTestId("os-palette-command-palette.view.sessions").click();
      await expect(palette).toHaveAttribute("data-palette-view", "sessions");
      // The aggregate speaks the same owner vocabulary as the listings.
      await expect(profilesOperatorSelectors(appPage, palette).ownerTags.first()).toBeVisible();
    });

    await appPage.keyboard.press("Escape");
    await expect(appPage.getByTestId("os-command-palette")).toBeHidden();
    await ui.switcher.click();
    await ui.switcherOption("default").click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
    await createProfile(runtime, "old-agency", "#b58e5f", "folder");
    await archiveProfile(runtime, "old-agency");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    await test.step("E2E-015: aggregate listings label owners and creation destination", async () => {
      await ui.switcher.click();
      await ui.switcherAll.click();
      await expect(ui.switcher).toHaveAccessibleName("Profile: All profiles");

      // S3: every aggregate row names its owner, and an archived owner says so.
      const sessions = await openSessionsCatalog(appPage);
      const rows = profilesOperatorSelectors(appPage, sessions);
      await expect(rows.ownerTags.first()).toBeVisible();

      // S2: the destination is stated before the commit, as fixed text.
      await sessions.getByTestId("os-sessions-modal-new-session").click();
      const chip = profilesOperatorSelectors(appPage).destinationChip;
      await expect(chip).toBeVisible();
      await expect(chip).toContainText("default");
      await expect(chip.locator("button, select, input")).toHaveCount(0);
      await appPage.getByRole("button", { name: "Cancel", exact: true }).click();
      await sessions.getByRole("button", { name: "Close sessions" }).click();

      // S11: the two axes compose — the globe stays independent of the profile.
      await appPage.getByTestId("os-global-scope-toggle").click();
      await expect(ui.switcher).toHaveAccessibleName("Profile: All profiles");

      // Leaving the aggregate lands on a real profile, never on the aggregate.
      await ui.switcher.click();
      await ui.switcherOption("marketing").click();
      await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");
      await expect(rows.ownerTags).toHaveCount(0);
    });
  });

  // Invariant: keyboard profile selection reaches a listing empty for that identity.
  // Owner: browser profile navigation; canonical suite: Profiles E2E-029 / E2E-019.
  test("E2E-029 / E2E-019: keyboard profile switching preserves tray order and scopes empty listings", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    await test.step("E2E-029: tray order and keyboard-only profile switch", async () => {
      // S1: the topbar tray keeps notifications before the palette; the rail foot
      // stacks the profile switcher above the theme toggle above Settings (D6).
      const railFoot = appPage.locator('[data-slot="os-rail-foot"]');
      for (const control of [
        appPage.locator('[data-slot="os-menubar-bell"]'),
        appPage.locator('[data-slot="os-menubar-command"]'),
        railFoot.getByTestId("os-menubar-profile"),
        railFoot.getByRole("button", { name: /^Switch to (?:light|dark) mode$/ }),
        railFoot.locator('[data-slot="os-rail-settings"]'),
      ]) {
        await expect(control).toBeVisible();
      }
      const order = await appPage.evaluate(() => {
        const rect = (selector: string) =>
          document.querySelector(selector)?.getBoundingClientRect();
        const foot = '[data-slot="os-rail-foot"]';
        return {
          tray: ['[data-slot="os-menubar-bell"]', '[data-slot="os-menubar-command"]'].map(
            selector => rect(selector)?.left ?? -1
          ),
          foot: [
            `${foot} [data-testid="os-menubar-profile"]`,
            `${foot} [aria-label^="Switch to"]`,
            `${foot} [data-slot="os-rail-settings"]`,
          ].map(selector => rect(selector)?.top ?? -1),
        };
      });
      for (const positions of [order.tray, order.foot]) {
        expect(
          positions.every(position => position >= 0),
          JSON.stringify(order)
        ).toBe(true);
        expect(positions).toEqual([...positions].sort((left, right) => left - right));
      }

      const ui = profilesOperatorSelectors(appPage);
      const palette = await openCommandPalette(appPage);
      await palette.getByRole("combobox").fill("Profiles");
      await palette.getByTestId("os-palette-command-palette.view.profiles").click();
      await expect(paletteView(appPage, "profiles")).toBeVisible();
      await palette.getByRole("combobox").fill("marketing");
      await expect(ui.paletteRow("marketing")).toBeVisible();
      await appPage.keyboard.press("Enter");
      await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");
    });

    await test.step("E2E-019: the empty task listing names its profile", async () => {
      const tasks = await openAppWindow(appPage, "Tasks", "tasks");
      await expect(
        tasks.getByRole("heading", { name: /No tasks in marketing yet/i })
      ).toBeVisible();
    });
  });
  test("E2E-020: two clients hold their own active profile and share only the remembered choice", async ({
    appPage,
    browser,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const second = await browser.newContext();
    const peerPage = await second.newPage();
    try {
      await peerPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
      await ensureProjectWorkspace(peerPage, runtime);
      await completeOnboardingIfPrompted(peerPage);
      const ui = profilesOperatorSelectors(appPage);
      const peer = profilesOperatorSelectors(peerPage);
      await expect(peer.switcher).toHaveAccessibleName("Profile: default");

      // The switch is this client's gesture. It persists the remembered choice…
      const remembered = appPage.waitForResponse(
        response =>
          response.request().method() === "PUT" &&
          response.url().endsWith("/api/profiles/selection")
      );
      await ui.switcher.click();
      await ui.switcherOption("marketing").click();
      expect((await remembered).ok()).toBe(true);
      await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");

      // …and leaves the open peer exactly where it was: the remembered choice is a
      // default for the next entry into the lens, never a remote control over an
      // open client (US-010.EC-4, ADR-014).
      await peerPage.waitForTimeout(500);
      await expect(peer.switcher).toHaveAccessibleName("Profile: default");

      // Entering the lens again is when the shared choice applies.
      await peerPage.reload({ waitUntil: "domcontentloaded" });
      await expect(peer.switcher).toHaveAccessibleName("Profile: marketing");
    } finally {
      await second.close();
    }
  });

  test("E2E-024: each profile keeps its own desktops and a new profile starts clean", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    const workspaceId = await activeWorkspaceId(runtime);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    const ui = profilesOperatorSelectors(appPage);

    await addDesktop(runtime, workspaceId, "default", "Default deck");
    await addDesktop(runtime, workspaceId, "marketing", "Campaign deck");

    // Neither profile ever shows the other's arrangement (US-026.EC-2).
    expect(desktopNames(await desktopsOf(runtime, workspaceId, "default"))).toEqual([
      "Desktop 1",
      "Default deck",
    ]);
    expect(desktopNames(await desktopsOf(runtime, workspaceId, "marketing"))).toEqual([
      "Desktop 1",
      "Campaign deck",
    ]);

    // Switching restores the target profile's desks in the shell, exactly as left.
    await ui.switcher.click();
    await ui.switcherOption("marketing").click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: marketing");
    const campaign = (await desktopsOf(runtime, workspaceId, "marketing")).desktops[1];
    await expect(appPage.locator(`[data-desktop-id="${campaign.id}"]`)).toBeAttached();
    const defaultDeck = (await desktopsOf(runtime, workspaceId, "default")).desktops[1];
    await expect(appPage.locator(`[data-desktop-id="${defaultDeck.id}"]`)).toHaveCount(0);

    // And back the other way, with no leakage in either direction.
    await ui.switcher.click();
    await ui.switcherOption("default").click();
    await expect(ui.switcher).toHaveAccessibleName("Profile: default");
    await expect(appPage.locator(`[data-desktop-id="${defaultDeck.id}"]`)).toBeAttached();
    await expect(appPage.locator(`[data-desktop-id="${campaign.id}"]`)).toHaveCount(0);

    // A brand-new profile enters on the seeded default desk (US-026.AC-2).
    await createProfile(runtime, "research", "#5fbf85", "compass");
    expect(desktopNames(await desktopsOf(runtime, workspaceId, "research"))).toEqual(["Desktop 1"]);
  });

  test("E2E-025: the identity picker is fully operable from the keyboard", async ({
    appPage,
    runtime,
  }) => {
    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(appPage);
    const ui = profilesOperatorSelectors(appPage);

    // Nothing below uses the pointer: an operator who never touches a mouse has to
    // be able to create a profile, and every step here is the keystroke they press.
    await ui.switcher.focus();
    await appPage.keyboard.press("Enter");
    await expect(ui.switcherMenu).toBeVisible();
    await appPage.keyboard.press("End");
    await expect(ui.switcherCreate).toHaveAttribute("data-selected", "true");
    await appPage.keyboard.press("Enter");
    await expect(ui.createDialog).toBeVisible();
    // Focus is inside the dialog the moment it opens (the popup itself counts:
    // dialogs focus their popup first), and stays there.
    await expect
      .poll(() => ui.createDialog.evaluate(dialog => dialog.contains(document.activeElement)))
      .toBe(true);

    // Every control the picker offers is named, so a screen reader can tell them apart.
    const iconsTab = ui.createDialog.getByRole("button", { name: "Icons" });
    const emojisTab = ui.createDialog.getByRole("button", { name: "Emojis" });
    const icons = ui.createDialog.getByRole("listbox", { name: "Icons" });
    const emojiSearch = ui.createDialog.getByRole("searchbox", { name: "Search emojis" });
    const swatches = ui.createDialog.getByRole("listbox", { name: "Suggested colors" });
    const hex = ui.createDialog.getByLabel("Custom color", { exact: true });
    await expect(iconsTab).toBeVisible();
    await expect(emojisTab).toBeVisible();

    await tabUntilFocused(appPage, ui.createName, 4);
    await appPage.keyboard.type("research");
    await expect(ui.createName).toHaveValue("research");

    // The kind pills are ordinary buttons, so they answer Tab and Enter.
    await tabUntilFocused(appPage, emojisTab, 4);
    await appPage.keyboard.press("Enter");
    await expect(emojiSearch).toBeVisible();
    await appPage.keyboard.press("Shift+Tab");
    await expect(iconsTab).toBeFocused();
    await appPage.keyboard.press("Enter");
    await expect(icons).toBeVisible();

    // Search filters by typing, and the grid shrinks to what matched.
    const search = ui.createDialog.getByLabel("Search icons");
    await tabUntilFocused(appPage, search, 16);
    const allIcons = await icons.getByRole("option").count();
    await appPage.keyboard.type("compass");
    await expect.poll(async () => icons.getByRole("option").count()).toBeLessThan(allIcons);

    // Arrow keys move the cursor inside the grid; Enter commits the option under it.
    await tabUntilFocused(appPage, icons, 4);
    await appPage.keyboard.press("Home");
    const activeIconId = await icons.getAttribute("aria-activedescendant");
    expect(activeIconId).not.toBeNull();
    await appPage.keyboard.press("Enter");
    await expect(appPage.locator(`#${activeIconId}`)).toHaveAttribute("aria-selected", "true");

    // The result grid and color palette are each one tab stop; moving backward
    // reaches the earlier color controls without circling the whole dialog.
    await tabUntilFocused(appPage, swatches, 4, "Shift+Tab");
    await appPage.keyboard.press("End");
    const activeSwatchId = await swatches.getAttribute("aria-activedescendant");
    expect(activeSwatchId).not.toBeNull();
    await appPage.keyboard.press("Enter");
    await expect(appPage.locator(`#${activeSwatchId}`)).toHaveAttribute("aria-selected", "true");
    await appPage.keyboard.press("Tab");
    await expect(hex).toBeFocused();
    await appPage.keyboard.press("ControlOrMeta+a");
    await appPage.keyboard.type("5fbf85");
    await expect(hex).toHaveValue("5fbf85");

    // Focus never escapes the dialog while it is open.
    for (let press = 0; press < 12; press += 1) {
      await appPage.keyboard.press("Tab");
      await expect(ui.createDialog.locator(":focus")).toHaveCount(1);
    }

    const created = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/profiles")
    );
    await tabUntilFocused(appPage, ui.createConfirm, 12);
    await appPage.keyboard.press("Enter");
    expect((await created).ok()).toBe(true);
    await expect(ui.switcher).toHaveAccessibleName("Profile: research");
  });
});
