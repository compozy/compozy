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
  if (app === "settings") {
    await expect(page.locator('[data-slot="os-menubar-command"]')).toHaveAttribute(
      "title",
      /^Command palette · /u
    );
  }
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
  if (!(await option.isVisible().catch(() => false))) {
    await page.keyboard.press("Escape");
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("os-desktop")).toBeVisible();
    await workspaceControl.click();
  }
  await expect(option).toBeVisible();
  await expect(option).toContainText(workspaceName);
  await option.click();
  await expect(workspaceControl).toContainText(workspaceName);
}

/** Turn Global scope on or off from the menubar globe toggle. */
export async function setGlobalScope(page: Page, on: boolean): Promise<void> {
  const toggle = page.getByTestId("os-global-scope-toggle");
  await expect(toggle).toBeVisible();
  const pressed = (await toggle.getAttribute("aria-pressed")) === "true";
  if (pressed !== on) {
    await toggle.click();
  }
  await expect(toggle).toHaveAttribute("aria-pressed", on ? "true" : "false");
}

/** The window's topbar title element (also carries `data-testid="topbar-title-text"`). */
export function windowTitle(win: Locator): Locator {
  return win.locator('[data-slot="topbar-title"]');
}

/* ── Command palette ──────────────────────────────────────────────────────
 * Every row is a projection of the daemon registry, so specs address rows by
 * command id rather than by the component that used to hand-write them. These
 * helpers are the one place that knows the palette's DOM contract.
 */

export function commandPalette(page: Page): Locator {
  return page.getByTestId("os-command-palette");
}

/** Opens ⌘K and waits for the overlay; the palette must not block on the daemon. */
export async function openCommandPalette(page: Page): Promise<Locator> {
  const palette = commandPalette(page);
  await expect(page.locator('[data-slot="os-menubar-command"]')).toHaveAttribute(
    "title",
    /^Command palette · /u
  );
  await page.keyboard.press("ControlOrMeta+KeyK");
  await expect(palette).toBeVisible();
  return palette;
}

/** Opens the dedicated Sessions catalog through its current public menu command. */
export async function openSessionsCatalog(page: Page): Promise<Locator> {
  await page.getByRole("menuitem", { name: "Session", exact: true }).click();
  await expect(page.getByTestId("os-menu-session")).toBeVisible();
  await page.getByTestId("os-menubar-command-shell.sessions.toggle").click();
  const catalog = page.getByTestId("os-sessions-modal");
  await expect(catalog).toBeVisible();
  return catalog;
}

/** One command row, addressed by its registry id. */
export function paletteRow(palette: Locator, commandId: string): Locator {
  return palette.getByTestId(`os-palette-command-${commandId}`);
}

/** The verbatim runtime reason a disabled row carries (BR-8). */
export function paletteRowReason(palette: Locator, commandId: string): Locator {
  return paletteRow(palette, commandId).locator('[data-slot="os-palette-reason"]');
}

/** A capped group's exact "showing N of M" note; silent truncation is forbidden. */
export function paletteOverflowNote(palette: Locator, section: string): Locator {
  return palette.getByTestId(`os-palette-overflow-${section.toLowerCase()}`);
}

/** The palette while it is picking the surface a new tab becomes (US-036). */
export function destinationPalette(page: Page): Locator {
  return page.locator('[data-testid="os-command-palette"][data-destination]');
}

/** The palette's own empty state — including the zero-eligible destination copy. */
export function paletteEmptyState(palette: Locator): Locator {
  return palette.getByTestId("os-palette-empty");
}

/** The active pushed palette view, regardless of its List/Detail/Form/Grid body. */
export function paletteView(page: Page, viewId: string): Locator {
  return page.locator(`[data-testid="os-command-palette"][data-palette-view="${viewId}"]`);
}

/** One built-in domain row addressed by the normalized domain key. */
export function paletteDomainRow(palette: Locator, key: string): Locator {
  return palette.getByTestId(`os-palette-domain-${key.replaceAll(":", "-")}`);
}

/** One truthful domain filter, including filters whose current count is zero. */
export function paletteDomainFilter(palette: Locator, filter: string): Locator {
  return palette.getByTestId(`os-palette-domain-filter-${filter}`);
}

/** One tile in a Grid view. */
export function paletteGridTile(palette: Locator, tileId: string): Locator {
  return palette.getByTestId(`palette-grid-tile-${tileId}`);
}

/** A menubar item projected from the registry, addressed by command id. */
export function menubarCommandItem(page: Page, commandId: string): Locator {
  return page.getByTestId(`os-menubar-command-${commandId}`);
}
