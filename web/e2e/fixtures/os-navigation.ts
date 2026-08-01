import { expect, type Locator, type Page } from "@playwright/test";

/**
 * Shared OS window-shell navigation helpers.
 *
 * The current shell has no persistent sidebar: a user opens each app through the
 * Dock (or the menubar for Settings) and every interaction is scoped to the owning
 * OS window. Runtime window IDs are opaque; app and instance selectors come from
 * the authority-backed attributes rendered on each window surface. These helpers
 * mirror the proven patterns from `os-shell.spec.ts` so every spec shares one
 * navigation authority instead of re-implementing dock clicks.
 */

/** Locate the active member for an app without reconstructing its opaque window ID. */
export function appWindow(page: Page, app: string): Locator {
  return page.locator(`[data-slot="os-window-surface"][data-app="${app}"][data-stack-active]`);
}

/**
 * Open an OS app window from the Dock or menubar and return its window locator.
 *
 * `title` is the control's accessible name (the app title, e.g. `"Tasks"`); `app`
 * is the lowercase app identity (e.g. `"tasks"`). The same role+name click path
 * serves Dock launchers and the menubar Settings cog.
 */
export async function openAppWindow(page: Page, title: string, app: string): Promise<Locator> {
  const launcher =
    app === "settings"
      ? page.locator('[data-slot="os-menubar-settings"]')
      : page
          .locator('[data-slot="os-dock"]:visible, [data-slot="os-dock-tabbar"]:visible')
          .getByRole("button", { exact: true, name: title });
  await launcher.click();
  const win = appWindow(page, app);
  await expect(win).toBeVisible();
  return win;
}

/** Locate an already-open app window, or open it without toggling a focused window closed. */
export async function ensureAppWindow(page: Page, title: string, app: string): Promise<Locator> {
  const win = appWindow(page, app);
  if (!(await win.isVisible())) {
    return await openAppWindow(page, title, app);
  }
  return win;
}

/**
 * Locator for an open session window. The session ID is the authority's
 * `instance_key`, not the window ID; this remains unambiguous when multiple
 * session windows are mounted.
 */
export function sessionWindow(page: Page, sessionId: string): Locator {
  return page.locator(
    `[data-slot="os-window-surface"][data-app="session"][data-instance-key="${sessionId}"]`
  );
}

/** Locate the chrome frame that owns a window surface. */
export function windowFrame(win: Locator): Locator {
  return win.locator("xpath=ancestor::*[@data-slot='os-window-frame'][1]");
}

/** Read the authority-issued opaque ID carried by a rendered window surface. */
export async function windowID(win: Locator): Promise<string> {
  const testID = await win.getAttribute("data-testid");
  const prefix = "os-window-";
  if (!testID?.startsWith(prefix) || testID.length === prefix.length) {
    throw new Error("window surface must expose its opaque ID");
  }
  return testID.slice(prefix.length);
}

/** Switch the active workspace and prove the menubar committed the selection. */
export async function switchWorkspace(
  page: Page,
  workspaceId: string,
  workspaceName: string
): Promise<void> {
  const workspaceControl = page.locator('[data-slot="os-menubar-workspace"]');
  await workspaceControl.click();
  const option = page.getByTestId(`os-workspace-option-${workspaceId}`);
  await expect(option).toBeVisible();
  await expect(option).toContainText(workspaceName);
  await option.click();
  await expect(workspaceControl).toContainText(workspaceName);
}

/** The window's topbar title element (also carries `data-testid="topbar-title-text"`). */
export function windowTitle(win: Locator): Locator {
  return win.locator('[data-slot="topbar-title"]');
}
