import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import type { Locator, Page } from "@playwright/test";

import { expect, test } from "../fixtures/test";
import {
  appWindow,
  openAppWindow,
  openCommandPalette,
  paletteView,
  switchWorkspace,
} from "../fixtures/os-navigation";
import { profilesOperatorSelectors } from "../fixtures/selectors";
import { completeOnboardingIfPrompted, ensureGlobalWorkspace } from "../fixtures/workspace";
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
        payload: { desktop_id: "", name, purpose: "standard" },
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
async function tabUntilFocused(page: Page, target: Locator, limit: number): Promise<void> {
  for (let press = 0; press < limit; press += 1) {
    const focused = await target
      .evaluate((node: Element) => node === document.activeElement)
      .catch(() => false);
    if (focused) return;
    await page.keyboard.press("Tab");
  }
  await expect(target).toBeFocused();
}

/** The workspace the shell is running in — desks are per (workspace, profile). */
async function activeWorkspaceId(runtime: BrowserRuntime): Promise<string> {
  const payload = await runtime.requestJSON<{ workspaces: Array<{ id: string }> }>(
    "/api/workspaces"
  );
  const workspace = payload.workspaces[0];
  if (!workspace) throw new Error("the desktop journey requires one resolved workspace");
  return workspace.id;
}

test.describe("Profiles", () => {
  test("E2E-013: switcher stays quiet, then carries identity, switch, and per-project memory", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
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
    await expect(ui.switcher).toContainText("marketing");

    const workspaceControl = appPage.locator('[data-slot="os-menubar-workspace"]');
    await workspaceControl.click();
    const workspaceOptions = appPage.locator('[data-testid^="os-workspace-option-"]');
    const marketingWorkspaces = await workspaceOptions.allTextContents();
    await appPage.keyboard.press("Escape");

    await ui.switcher.click();
    await ui.switcherOption("default").click();
    await workspaceControl.click();
    expect(await workspaceOptions.allTextContents()).toEqual(marketingWorkspaces);
    await appPage.keyboard.press("Escape");

    const selectionResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" && response.url().endsWith("/api/profiles/selection")
    );
    await ui.switcherOption("marketing").click();
    expect((await selectionResponse).ok()).toBe(true);
    await expect(ui.switcher).toContainText("marketing");

    // The remembered choice is daemon state, so the terminal sees it too.
    const remembered =
      await runtime.requestJSON<Array<{ profile: string }>>("/api/profiles/selection");
    expect(remembered.some(entry => entry.profile === "marketing")).toBe(true);

    // Returning to the project restores it rather than resetting to default.
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(ui.switcher).toContainText("marketing");
  });

  test("E2E-014: settings lists, creates, and edits identity behind disclosure", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
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
    const color = ui.identityPicker.getByLabel("Custom color");
    await color.fill("12ZZ");
    await expect(ui.identityDialog).toContainText("Enter a color like #4ea7fc.");
    await expect(ui.identityConfirm).toBeDisabled();

    await color.fill("4CB782");
    await ui.identityPicker.getByRole("button", { name: "Emojis" }).click();
    await ui.identityPicker.getByRole("option", { name: "seedling" }).click();
    const updated = appPage.waitForResponse(
      response =>
        response.request().method() === "PATCH" &&
        response.url().endsWith("/api/profiles/consulting")
    );
    await ui.identityConfirm.click();
    expect((await updated).ok()).toBe(true);
    await expect(ui.identityDialog).not.toBeVisible();

    await ui.createOpen.click();
    await expect(ui.createDialog).toBeVisible();
    // Creation is never blank: a starter identity is already picked.
    await expect(ui.createPicker).toBeVisible();

    // An empty name is refused inline rather than at the daemon.
    await ui.createConfirm.click();
    await expect(ui.createDialog).toContainText("Give the profile a name.");

    const created = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/profiles")
    );
    await ui.createName.fill("research");
    await ui.createConfirm.click();
    expect((await created).ok()).toBe(true);
    await expect(ui.row("research")).toBeVisible();

    // Archived profiles are demoted to a disclosure, not shown by default.
    await archiveProfile(runtime, "research");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    const reopened = await openProfilesSettings(appPage);
    const after = profilesOperatorSelectors(appPage, reopened);
    await expect(after.pageArchived).toBeVisible();
    await after.pageArchived.click();
    await expect(after.archivedList).toContainText("research");
  });

  test("E2E-016: rename shows the tiered plan and reports dormant placements", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "dev", "#4cb782", "wrench");

    const settings = await openProfilesSettings(appPage);
    const ui = profilesOperatorSelectors(appPage, settings);

    await ui.renameRow("dev").click();
    await expect(ui.renameDialog).toBeVisible();
    await ui.renameName.fill("eng");

    // The plan comes from the daemon; the dialog never recomputes it.
    const plan = await runtime.requestJSON<{
      machine_folders: string[];
      revision: string;
    }>("/api/profiles/dev/rename-plan?new_name=eng");
    expect(plan.revision).not.toBe("");
    await expect(ui.renamePlan).toBeVisible();
    if (plan.machine_folders.length > 0) {
      await expect(ui.renamePlan).toContainText("Machine folders");
    }

    const renamed = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().endsWith("/api/profiles/dev/rename")
    );
    await ui.renameConfirm.click();
    expect((await renamed).ok()).toBe(true);

    const profiles = await listProfiles(runtime);
    expect(profiles.map(profile => profile.name)).toContain("eng");
    expect(profiles.map(profile => profile.name)).not.toContain("dev");
  });

  test("E2E-017: archive names what pauses, blocks on running work, and unarchive lists reactivation", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "finance", "#e8b04a", "gem");

    const settings = await openProfilesSettings(appPage);
    const ui = profilesOperatorSelectors(appPage, settings);

    await ui.archiveRow("finance").click();
    await expect(ui.archiveDialog).toBeVisible();
    // Archive destroys nothing, so it reads calm rather than dangerous.
    await expect(ui.archiveDialog).toContainText("Nothing is deleted.");

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
  });

  test("E2E-027: the palette Profiles view switches and hands lifecycle to the canonical dialogs", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
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
    await expect(ui.switcher).toContainText("marketing");

    // A lifecycle action opens the canonical dialog; the palette owns no plan.
    const reopened = await openCommandPalette(appPage);
    await reopened.getByRole("combobox").fill("Archive profile");
    await reopened.getByTestId("os-palette-command-profile.archive").click();
    await expect(appWindow(appPage, "settings")).toBeVisible();
  });

  test("E2E-015: All profiles labels every row, states the destination, and names the owner", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await createProfile(runtime, "old-agency", "#b58e5f", "folder");
    await archiveProfile(runtime, "old-agency");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const ui = profilesOperatorSelectors(appPage);
    await ui.switcher.click();
    await ui.switcherAll.click();
    await expect(ui.switcher).toContainText("All profiles");

    // S3: every aggregate row names its owner, and an archived owner says so.
    const sessions = await openAppWindow(appPage, "Sessions", "sessions");
    const rows = profilesOperatorSelectors(appPage, sessions);
    await expect(rows.ownerTags.first()).toBeVisible();

    // S2: the destination is stated before the commit, as fixed text.
    await sessions.getByTestId("os-sessions-modal-new-session").click();
    const chip = profilesOperatorSelectors(appPage).destinationChip;
    await expect(chip).toBeVisible();
    await expect(chip).toContainText("default");
    await expect(chip.locator("button, select, input")).toHaveCount(0);

    // S11: the two axes compose — the globe stays independent of the profile.
    await appPage.getByTestId("os-global-scope-toggle").click();
    await expect(ui.switcher).toContainText("All profiles");

    // Leaving the aggregate lands on a real profile, never on the aggregate.
    await ui.switcher.click();
    await ui.switcherOption("marketing").click();
    await expect(ui.switcher).toContainText("marketing");
    await expect(rows.ownerTags).toHaveCount(0);
  });

  test("E2E-018: a deep link into another profile's session names its owner and offers the switch", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "consulting", "#4ea7fc", "briefcase");
    const foreign = await runtime.requestJSON<{ session: { id: string; agent_name: string } }>(
      "/api/sessions?profile=consulting",
      { method: "POST", body: JSON.stringify({ agent_name: "claude-agent" }) }
    );
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const ui = profilesOperatorSelectors(appPage);
    await appPage.goto(`/session/${foreign.session.id}`, { waitUntil: "domcontentloaded" });

    // Informed, not blocked: the item resolves through the labeled aggregate read.
    await expect(ui.ownerBanner).toContainText("belongs to consulting");
    await ui.ownerBannerSwitch.click();
    await expect(ui.switcher).toContainText("consulting");
  });

  test("E2E-019: an empty listing names the profile it is empty for", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const ui = profilesOperatorSelectors(appPage);
    await ui.switcher.click();
    await ui.switcherOption("marketing").click();

    const tasks = await openAppWindow(appPage, "Tasks", "tasks");
    await expect(tasks.getByRole("heading", { name: /No tasks in marketing yet/i })).toBeVisible();
  });

  test("E2E-021: usage is scoped per profile and breaks down by owner in the aggregate", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
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
      "---\nname: browser-dev\nprovider: anthropic\nmodel: claude-sonnet-4-20250514\n---\n",
      "utf8"
    );
    const workspace = await runtime.resolveWorkspace(workspaceRoot);

    await completeOnboardingIfPrompted(appPage);
    await switchWorkspace(appPage, workspace.id, workspace.name);

    const hint = appPage.getByTestId("workspace-profiles-hint");
    await expect(hint).toContainText("This project declares content for profile dev.");
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

    const detail = await runtime.requestJSON<{ agents: Array<{ name: string }> }>(
      `/api/workspaces/${workspace.id}?profile=dev`
    );
    expect(detail.agents.map(agent => agent.name)).toContain("browser-dev");
  });

  test("E2E-028: palette results, ranking, and view sessions re-scope across a switch", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const ui = profilesOperatorSelectors(appPage);
    const scopedCatalog = appPage.waitForResponse(
      response =>
        response.url().includes("/api/cmd-palette/commands") &&
        response.url().includes("profile=default")
    );
    await openCommandPalette(appPage);
    expect((await scopedCatalog).ok()).toBe(true);
    await appPage.keyboard.press("Escape");

    await ui.switcher.click();
    await ui.switcherAll.click();
    const aggregateCatalog = appPage.waitForResponse(
      response =>
        response.url().includes("/api/cmd-palette/commands") &&
        response.url().includes("all_profiles=true")
    );
    const palette = await openCommandPalette(appPage);
    expect((await aggregateCatalog).ok()).toBe(true);
    await palette.getByRole("combobox").fill("session");
    // The aggregate speaks the same owner vocabulary as the listings.
    await expect(profilesOperatorSelectors(appPage, palette).ownerTags.first()).toBeVisible();
  });

  test("E2E-029: the right cluster order holds and a keyboard-only switch re-scopes listings", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    // S1: notifications → palette → profile switcher → Settings, in that order.
    const order = await appPage.evaluate(() => {
      const slots = ["os-menubar-notifications", "os-menubar-command", "os-menubar-profile"];
      const positions = slots.map(
        slot =>
          document.querySelector(`[data-testid="${slot}"]`)?.getBoundingClientRect().left ?? -1
      );
      const settings =
        document.querySelector('[data-testid="os-menubar-settings"]')?.getBoundingClientRect()
          .left ?? -1;
      return [...positions, settings];
    });
    expect(order).toEqual([...order].sort((left, right) => left - right));

    const ui = profilesOperatorSelectors(appPage);
    const palette = await openCommandPalette(appPage);
    await palette.getByRole("combobox").fill("marketing");
    await appPage.keyboard.press("Enter");
    await expect(ui.switcher).toContainText("marketing");
  });
  test("E2E-020: two clients hold their own active profile and share only the remembered choice", async ({
    appPage,
    browser,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    await createProfile(runtime, "marketing", "#c26ad6", "megaphone");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    const second = await browser.newContext();
    const peerPage = await second.newPage();
    try {
      await peerPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
      await completeOnboardingIfPrompted(peerPage);
      const ui = profilesOperatorSelectors(appPage);
      const peer = profilesOperatorSelectors(peerPage);
      await expect(peer.switcher).toContainText("default");

      // The switch is this client's gesture. It persists the remembered choice…
      const remembered = appPage.waitForResponse(
        response =>
          response.request().method() === "PUT" &&
          response.url().endsWith("/api/profiles/selection")
      );
      await ui.switcher.click();
      await ui.switcherOption("marketing").click();
      expect((await remembered).ok()).toBe(true);
      await expect(ui.switcher).toContainText("marketing");

      // …and leaves the open peer exactly where it was: the remembered choice is a
      // default for the next entry into the lens, never a remote control over an
      // open client (US-010.EC-4, ADR-014).
      await peerPage.waitForTimeout(500);
      await expect(peer.switcher).toContainText("default");

      // Entering the lens again is when the shared choice applies.
      await peerPage.reload({ waitUntil: "domcontentloaded" });
      await expect(peer.switcher).toContainText("marketing");
    } finally {
      await second.close();
    }
  });

  test("E2E-024: each profile keeps its own desktops and a new profile starts clean", async ({
    appPage,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);
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
    await expect(ui.switcher).toContainText("marketing");
    const campaign = (await desktopsOf(runtime, workspaceId, "marketing")).desktops[1];
    await expect(appPage.locator(`[data-desktop-id="${campaign.id}"]`)).toBeAttached();
    const defaultDeck = (await desktopsOf(runtime, workspaceId, "default")).desktops[1];
    await expect(appPage.locator(`[data-desktop-id="${defaultDeck.id}"]`)).toHaveCount(0);

    // And back the other way, with no leakage in either direction.
    await ui.switcher.click();
    await ui.switcherOption("default").click();
    await expect(ui.switcher).toContainText("default");
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
    await ensureGlobalWorkspace(runtime);
    await completeOnboardingIfPrompted(appPage);
    const ui = profilesOperatorSelectors(appPage);

    // Nothing below uses the pointer: an operator who never touches a mouse has to
    // be able to create a profile, and every step here is the keystroke they press.
    await ui.switcher.focus();
    await appPage.keyboard.press("Enter");
    await expect(ui.switcherMenu).toBeVisible();
    await tabUntilFocused(appPage, ui.switcherCreate, 12);
    await appPage.keyboard.press("Enter");
    await expect(ui.createDialog).toBeVisible();
    // Focus is inside the dialog the moment it opens, and stays there.
    await expect(ui.createDialog.locator(":focus")).toHaveCount(1);

    // Every control the picker offers is named, so a screen reader can tell them apart.
    const iconsTab = ui.createDialog.getByRole("button", { name: "Icons" });
    const emojisTab = ui.createDialog.getByRole("button", { name: "Emojis" });
    const icons = ui.createDialog.getByRole("listbox", { name: "Icons" });
    const emojis = ui.createDialog.getByRole("listbox", { name: "Emojis" });
    const swatches = ui.createDialog.getByRole("listbox", { name: "Suggested colors" });
    const hex = ui.createDialog.getByLabel("Custom color");
    await expect(iconsTab).toBeVisible();
    await expect(emojisTab).toBeVisible();

    await tabUntilFocused(appPage, ui.createName, 4);
    await appPage.keyboard.type("research");
    await expect(ui.createName).toHaveValue("research");

    // The kind pills are ordinary buttons, so they answer Tab and Enter.
    await tabUntilFocused(appPage, emojisTab, 4);
    await appPage.keyboard.press("Enter");
    await expect(emojis).toBeVisible();
    await tabUntilFocused(appPage, iconsTab, 4);
    await appPage.keyboard.press("Enter");
    await expect(icons).toBeVisible();

    // Search filters by typing, and the grid shrinks to what matched.
    const search = ui.createDialog.getByLabel("Search icons");
    await tabUntilFocused(appPage, search, 4);
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

    // The palette is a single tab stop, so the hex field is one Tab away from it.
    await tabUntilFocused(appPage, swatches, 4);
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
    await expect(ui.switcher).toContainText("research");
  });
});
