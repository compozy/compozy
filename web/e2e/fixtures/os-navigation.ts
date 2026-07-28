import { expect, type Locator, type Page } from "@playwright/test";

/**
 * Shared OS window-shell navigation helpers.
 *
 * The current shell has no persistent sidebar: a user opens each app through the
 * Dock (or the menubar for Settings) and every interaction is scoped to the owning
 * OS window (`os-window-app:<app>` / `os-window-session:<sessionId>`). These helpers
 * mirror the proven patterns from `os-shell.spec.ts` so every spec shares one
 * navigation authority instead of re-implementing dock clicks. The canonical suite
 * keeps its own local copies (a spec must not be imported by other specs).
 */

/**
 * Open an OS app window from the Dock or menubar and return its window locator.
 *
 * `title` is the control's accessible name (the app title, e.g. `"Tasks"`); `app`
 * is the lowercase app id used in the window test id (e.g. `"tasks"`). The same
 * role+name click path serves Dock launchers and the menubar Settings cog.
 */
export async function openAppWindow(page: Page, title: string, app: string): Promise<Locator> {
  const launcher =
    app === "settings"
      ? page.locator('[data-slot="os-menubar-settings"]')
      : page
          .locator('[data-slot="os-dock"]:visible, [data-slot="os-dock-tabbar"]:visible')
          .getByRole("button", { exact: true, name: title });
  await launcher.click();
  const win = page.getByTestId(`os-window-app:${app}`);
  await expect(win).toBeVisible();
  return win;
}

/** Locate an already-open app window, or open it without toggling a focused window closed. */
export async function ensureAppWindow(page: Page, title: string, app: string): Promise<Locator> {
  const win = page.getByTestId(`os-window-app:${app}`);
  if (!(await win.isVisible())) {
    return await openAppWindow(page, title, app);
  }
  return win;
}

/**
 * Locator for an open session window. Deep-linking `/agents/<agent>/sessions/<id>`
 * opens `os-window-session:<id>`; scope every session interaction under this so a
 * page-level locator never matches a second window.
 */
export function sessionWindow(page: Page, sessionId: string): Locator {
  return page.getByTestId(`os-window-session:${sessionId}`);
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
