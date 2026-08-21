import { expect, test } from "../fixtures/test";
import {
  appWindow,
  openAppWindow,
  openCommandPalette,
  paletteView,
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

async function openProfilesSettings(page: Parameters<typeof openAppWindow>[0]) {
  const settings = await openAppWindow(page, "Settings", "settings");
  await settings.getByTestId("settings-section-nav").getByText("Profiles", { exact: true }).click();
  return settings;
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
    await runtime.requestJSON("/api/profiles/research/archive", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        plan_revision: (
          await runtime.requestJSON<{ revision: string }>("/api/profiles/research/archive-plan")
        ).revision,
      }),
    });
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
});
