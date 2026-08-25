import { execFile } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Browser, Locator, Page } from "@playwright/test";

import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import {
  appWindow,
  destinationPalette,
  menubarCommandItem,
  openAppWindow as openDockApp,
  openCommandPalette,
  paletteEmptyState,
  paletteRow,
  paletteRowReason,
  sessionWindow,
  switchWorkspace,
  windowFrame,
  windowID,
} from "../fixtures/os-navigation";
import { createWorktreeRepo } from "../fixtures/worktree-repo";
import {
  osShellSelectors,
  sessionLifecycleSelectors,
  sessionWorkspaceSwitchSelectors,
  settingsOperatorSelectors,
  tasksOperatorSelectors,
} from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import {
  settingsUpdateApplyingFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateStatusFixture,
} from "@/systems/settings/mocks/settings-update-fixture";

const execFileAsync = promisify(execFile);
const browserLifecycleAgent = "os-shell-agent";
const windowManagerClientStorageKey = "compozy.window-manager.client-id";
const browserLifecycleFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "os_shell_multi_session_fixture.json"
);
const permissionHardeningFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_hardening_fixture.json"
);

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: browserLifecycleAgent,
          fixtureAgent: "os-shell-multi-session-agent",
          fixturePath: browserLifecycleFixture,
        },
        {
          agentName: "permission-hardening-agent",
          fixtureAgent: "permission-hardening-agent",
          fixturePath: permissionHardeningFixture,
        },
      ],
    },
  },
});

/**
 * Window Tabs contract coverage. Logical IDs belong to
 * `.compozy/tasks/window-tabs/_tests.md`; physical IDs stay monotonic in this
 * long-lived suite so unrelated shell scenarios never renumber the contract.
 *
 * E2E-001 → E2E-030  ·  E2E-002 → E2E-031  ·  E2E-003 → E2E-041
 * E2E-004 → E2E-039  ·  E2E-005 → E2E-032  ·  E2E-006 → E2E-033
 * E2E-007 → E2E-040  ·  E2E-008 → E2E-034  ·  E2E-009 → E2E-042
 * E2E-010 → E2E-035 ·  E2E-011 → E2E-043 ·  E2E-012 → E2E-036
 * E2E-013 → E2E-039 ·  E2E-014 → E2E-038 ·  E2E-015 → E2E-044
 * E2E-016 → E2E-037 ·  E2E-018 → E2E-045
 *
 * Herdr parity contract coverage (`.compozy/tasks/herdr-parity/_tests.md`) is
 * prefixed `Herdr ` because this suite's physical numbering already claims the
 * same digits: Herdr E2E-016 (⌘E → Sessions view → land) and Herdr E2E-019
 * (nested view push, pop, and reopen-at-root).
 */

interface NormalizedRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

interface WindowRoute {
  pathname: string;
  search: Record<string, unknown>;
}

interface WindowManagerWindow {
  id: string;
  app: string;
  instance_key?: string;
  route: WindowRoute;
  placement: "tiled" | "stacked" | "floating";
  desktop_id: string;
  floating_rect: NormalizedRect;
  minimized: boolean;
  pinned?: boolean;
  nav_stack?: WindowRoute[];
}

interface WindowManagerLayoutNode {
  id: string;
  kind: "leaf" | "split" | "stack";
  window_id?: string;
  axis?: "horizontal" | "vertical";
  children?: WindowManagerLayoutNode[];
  weights?: number[];
  window_ids?: string[];
  active_id?: string;
}

interface WindowManagerDesktop {
  id: string;
  name: string;
  order: number;
  purpose: "standard" | "focus";
  focus_owner?: string;
  groups: Array<{
    id: string;
    frame: NormalizedRect;
    root: WindowManagerLayoutNode;
  }>;
  floating: string[];
  floating_stacks?: Array<{
    id: string;
    window_ids: string[];
    active_id: string | null;
    rect: NormalizedRect;
    minimized: boolean;
  }>;
}

interface WindowManagerSnapshot {
  version: number;
  workspace_id: string;
  revision: number;
  desktops: WindowManagerDesktop[];
  windows: Record<string, WindowManagerWindow>;
  history: { undo: unknown[]; redo: unknown[] };
  overrides: Record<string, unknown>;
  updated_at: string;
}

interface WindowManagerClientView {
  workspace_id: string;
  client_id: string;
  active_desktop_id: string;
  focused_window_id?: string;
  focus_order: string[];
  stack_active?: Record<string, string>;
}

interface WindowManagerCommandResult {
  snapshot: WindowManagerSnapshot;
  applied: boolean;
  client?: WindowManagerClientView;
}

interface WindowManagerLayoutDocument {
  version: number;
  workspace_id: string;
  desktops: WindowManagerDesktop[];
  windows: Record<string, WindowManagerWindow>;
  overrides: Record<string, unknown>;
}

interface WindowManagerSettingsResponse {
  config: {
    gaps: {
      inner: number;
      top: number;
      right: number;
      bottom: number;
      left: number;
    };
  };
}

interface SettingsRestartAction {
  operation_id: string;
  status: string;
  status_url: string;
}

interface SettingsRestartStatus {
  status: string;
}

test("E2E-001: fresh boot renders the empty desktop without opening a window", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);

  await expect(appPage.getByRole("navigation", { name: "Dock" })).toBeVisible();
  await expect(appPage.getByRole("banner", { name: "System bar" })).toBeVisible();
  expect(
    await appPage.evaluate(() => {
      const overlay = Reflect.get(navigator, "windowControlsOverlay") as
        | { visible: boolean }
        | undefined;
      return overlay?.visible ?? false;
    })
  ).toBe(false);
  await expect(appPage.getByTestId("os-desk-hint")).toContainText("⌘K");
  await expect(appPage.locator('[data-testid^="os-window-"]')).toHaveCount(0);
  await expect(appPage.getByRole("button", { name: "Desktop 1 of 1: Desktop 1" })).toHaveAttribute(
    "aria-current",
    "page"
  );

  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  expect(snapshot.version).toBe(3);
  expect(snapshot.revision).toBe(0);
  expect(snapshot.desktops.map(desktop => [desktop.id, desktop.name, desktop.purpose])).toEqual([
    ["desktop-default", "Desktop 1", "standard"],
  ]);
  expect(snapshot.windows).toEqual({});

  const legacy = await fetch(
    runtime.url(`/api/workspaces/${encodeURIComponent(workspace.id)}/desktop-state`)
  );
  expect(legacy.status).toBe(404);
});

test("E2E-002: floating Tasks drag commits one normalized rect and survives reload", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  await expect(appPage).toHaveURL(/\/tasks$/);
  const opened = await authoritativeWindowRect(appPage, runtime, workspace.id, tasksID);

  await dragWindowBy(appPage, tasks, 92, 48);
  await expect
    .poll(() => authoritativeWindowRect(appPage, runtime, workspace.id, tasksID))
    .not.toEqual(opened);
  const dragged = await authoritativeWindowRect(appPage, runtime, workspace.id, tasksID);
  await expect.poll(() => windowRect(appPage, tasks)).toEqual(dragged);
  expect((await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.placement).toBe(
    "floating"
  );

  await appPage.reload({ waitUntil: "domcontentloaded" });
  const restored = appWindow(appPage, "tasks");
  await expect(restored).toBeVisible();
  await expect.poll(() => windowRect(appPage, restored)).toEqual(dragged);
});

test("E2E-003: zoom uses the focus desktop and restores the exact tiled anchor", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const settings = await openDockApp(appPage, "Settings", "settings");
  const tasksID = await windowID(tasks);
  const settingsID = await windowID(settings);
  await arrangeWindows(
    runtime,
    workspace.id,
    "desktop-default",
    [tasksID, settingsID],
    "horizontal",
    "group-zoom-anchor"
  );
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");
  await expect(settings).toHaveAttribute("data-window-placement", "tiled");

  const before = await windowManagerSnapshot(runtime, workspace.id);
  const anchor = layoutSignature(before, "desktop-default");
  await tasks.getByRole("button", { name: "Zoom window" }).click();

  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      const focusDesktop = snapshot.desktops.find(desktop => desktop.purpose === "focus");
      return focusDesktop?.focus_owner === tasksID &&
        snapshot.windows[tasksID]?.desktop_id === focusDesktop.id
        ? focusDesktop.id
        : null;
    })
    .not.toBeNull();
  const zoomed = await windowManagerSnapshot(runtime, workspace.id);
  const focusDesktop = zoomed.desktops.find(desktop => desktop.purpose === "focus");
  if (!focusDesktop) throw new Error("zoom must create one focus desktop");
  await expect(activeDesktop(appPage, focusDesktop.id)).toHaveAttribute("data-active", "true");
  await expect(settings).toBeHidden();

  await tasks.getByRole("button", { name: "Zoom window" }).click();
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      return snapshot.windows[tasksID]?.desktop_id;
    })
    .toBe("desktop-default");
  const restored = await windowManagerSnapshot(runtime, workspace.id);
  expect(layoutSignature(restored, "desktop-default")).toEqual(anchor);
  await expect(activeDesktop(appPage, "desktop-default")).toHaveAttribute("data-active", "true");
  await expect(tasks).toBeVisible();
  await expect(settings).toBeVisible();
});

test("E2E-004: minimize exposes the dock state and restore remounts content", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");

  await tasks.getByRole("button", { name: "Minimize window" }).click();
  await expect(tasks).toBeHidden();
  const dockItem = appPage.getByRole("button", { name: "Tasks" });
  await expect(dockItem).toHaveAttribute("data-state", "minimized");

  await dockItem.click();
  await expect(tasks).toBeVisible();
  await expect(tasks.getByTestId("tasks-shell")).toBeVisible();
});

test("E2E-005: a direct task detail deep link returns to the catalog with Back", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Deep-link task");
  await appPage.goto(runtime.url("/tasks"), { waitUntil: "domcontentloaded" });
  await expect(appWindow(appPage, "tasks")).toBeVisible();

  await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(task.id)}`), {
    waitUntil: "domcontentloaded",
  });
  const tasksUI = tasksOperatorSelectors(appPage);
  const tasksWindow = appWindow(appPage, "tasks");
  await expect(tasksUI.detailContent).toBeVisible();
  const windowPath = tasksWindow.getByRole("navigation", { name: "Window path" });
  await expect(windowPath.getByRole("button", { name: "Tasks", exact: true })).toBeVisible();
  await expect(tasksWindow.getByTestId("tasks-detail-title")).toContainText(task.title);
  await expect(tasksWindow.locator('[data-slot="topbar-title"]')).toContainText(task.title);
  await expect(windowPath).not.toContainText(/^compozy\b/);

  await appPage.goBack({ waitUntil: "domcontentloaded" });
  await expect(appPage).toHaveURL(/\/tasks$/);
  await expect(tasksWindow.getByTestId("tasks-shell")).toBeVisible();
});

test("E2E-007: window.navigate preserves detail and search across peers, reload, and restart", async ({
  appPage,
  browser,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Durable route task");
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const peer = await openPeerPage(browser, runtime);
  try {
    const clientId = await browserClientId(appPage);
    const pathname = `/tasks/${encodeURIComponent(task.id)}`;
    const search = { inspect: "stream", tab: "activity" };
    await runWindowManagerCLI(runtime, [
      "window",
      "navigate",
      "--workspace",
      workspace.id,
      "--revision",
      String((await windowManagerSnapshot(runtime, workspace.id)).revision),
      "--client",
      clientId,
      "--id",
      tasksID,
      "--pathname",
      pathname,
      "--search-json",
      JSON.stringify(search),
    ]);

    await expect
      .poll(
        async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.route
      )
      .toEqual({ pathname, search });
    await expect(tasks.getByTestId("tasks-detail-title")).toContainText(task.title);
    await expect(appWindow(peer, "tasks").getByTestId("tasks-detail-title")).toContainText(
      task.title
    );
    await expect.poll(() => currentRoute(appPage)).toEqual({ pathname, search });

    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(appWindow(appPage, "tasks").getByTestId("tasks-detail-title")).toContainText(
      task.title
    );
    await expect.poll(() => currentRoute(appPage)).toEqual({ pathname, search });
    const restartLocation = appPage.url();

    const restart = await runtime.requestJSON<SettingsRestartAction>(
      "/api/settings/actions/restart",
      { method: "POST", body: "{}" }
    );
    await expect
      .poll(() => pollRestartStatus(runtime, restart.status_url), { timeout: 45_000 })
      .toBe("ready");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(appPage).toHaveURL(restartLocation);
    await expect(appWindow(appPage, "tasks").getByTestId("tasks-detail-title")).toContainText(
      task.title
    );
    await expect.poll(() => currentRoute(appPage)).toEqual({ pathname, search });
    await expect
      .poll(
        async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.route
      )
      .toEqual({ pathname, search });
  } finally {
    await peer.context().close();
  }
});

test("E2E-006: visible session windows stream without focus and catch up after restore", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const primary = await createNamedSession(runtime, workspace.id, "Primary observer");
  const secondary = await createNamedSession(runtime, workspace.id, "Secondary responder");
  const sessionsDockButton = appPage.getByRole("button", { name: "Sessions", exact: true });

  await sessionsDockButton.click();
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await expect(sessionsModal).toBeVisible();
  await sessionsModal.getByTestId(`os-sessions-modal-session-${primary.id}`).first().click();
  await expect(sessionsModal).toHaveCount(0);
  await sessionsDockButton.click();
  await expect(sessionsModal).toBeVisible();
  await sessionsModal.getByTestId(`os-sessions-modal-session-${secondary.id}`).first().click();
  await expect(sessionsModal).toHaveCount(0);

  const primaryWindow = sessionWindow(appPage, primary.id);
  const secondaryWindow = sessionWindow(appPage, secondary.id);
  await expect(primaryWindow).toBeVisible();
  await expect(secondaryWindow).toBeVisible();
  const primaryWindowID = await windowID(primaryWindow);
  const secondaryWindowID = await windowID(secondaryWindow);

  await moveWindowToNormalizedRect(runtime, workspace.id, primaryWindowID, {
    x: 0.01,
    y: 0.02,
    width: 0.48,
    height: 0.78,
  });
  await moveWindowToNormalizedRect(runtime, workspace.id, secondaryWindowID, {
    x: 0.51,
    y: 0.02,
    width: 0.48,
    height: 0.78,
  });
  await expect
    .poll(() =>
      windowMatchesAuthority(appPage, primaryWindow, runtime, workspace.id, primaryWindowID)
    )
    .toBe(true);
  await expect
    .poll(() =>
      windowMatchesAuthority(appPage, secondaryWindow, runtime, workspace.id, secondaryWindowID)
    )
    .toBe(true);
  const [primaryBox, secondaryBox] = await Promise.all([
    primaryWindow.boundingBox(),
    secondaryWindow.boundingBox(),
  ]);
  if (!primaryBox || !secondaryBox) throw new Error("session windows must have visible bounds");
  expect(primaryBox.x + primaryBox.width).toBeLessThanOrEqual(secondaryBox.x);

  const primaryComposer = primaryWindow.getByRole("textbox", { name: "Session prompt" });
  const primaryTranscript = primaryWindow.getByTestId("chat-view");
  await primaryComposer.fill("observe primary stream");
  await primaryComposer.press("Enter");
  await expect(primaryTranscript).toContainText("Primary stream is warming up.");
  const parkedScrollTop = await primaryTranscript.evaluate(element => {
    const viewport = element as HTMLElement;
    if (viewport.scrollHeight <= viewport.clientHeight) return -1;
    viewport.scrollTop = Math.max(
      1,
      Math.floor((viewport.scrollHeight - viewport.clientHeight) / 3)
    );
    viewport.dispatchEvent(new Event("scroll"));
    return viewport.scrollTop;
  });
  expect(parkedScrollTop).toBeGreaterThan(0);

  const secondaryComposer = secondaryWindow.getByRole("textbox", { name: "Session prompt" });
  const secondaryTranscript = secondaryWindow.getByTestId("chat-view");
  await secondaryComposer.fill("reply in secondary window");
  await secondaryComposer.press("Enter");
  await expect(secondaryTranscript).toContainText("Secondary stream started.");
  await expect(secondaryTranscript).toContainText("Secondary stream completed independently.");
  await expect(windowFrame(secondaryWindow)).toHaveAttribute("data-focused", "");
  await expect(windowFrame(primaryWindow)).not.toHaveAttribute("data-focused", "");
  await expect(primaryTranscript).toContainText(
    "Primary stream event arrived while the window was visible and unfocused."
  );
  await expect
    .poll(() => primaryTranscript.evaluate(element => element.scrollTop))
    .toBe(parkedScrollTop);

  await primaryWindow.getByRole("button", { name: "Minimize window" }).click();
  await expect(primaryWindow).toHaveCount(1);
  await expect(primaryWindow).toBeHidden();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[primaryWindowID])
    .toMatchObject({ minimized: true });
  await expect
    .poll(() => sessionHistoryContains(runtime, workspace.id, primary.id, "arrived while"))
    .toBe(true);

  await sessionsDockButton.click();
  const restoredModal = appPage.getByTestId("os-sessions-modal");
  await expect(restoredModal).toBeVisible();
  await restoredModal.getByTestId(`os-sessions-modal-session-${primary.id}`).first().click();
  const restoredPrimary = sessionWindow(appPage, primary.id);
  await expect(restoredPrimary).toBeVisible();
  await expect(restoredPrimary.getByTestId("chat-view")).toContainText(
    "Primary stream event arrived while the window was minimized."
  );
});

test("E2E-008: palette stays global while RuntimeSelector owns scoped ⌘J", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const session = await createNamedSession(runtime, workspace.id, "Palette target");

  await appPage.keyboard.press("ControlOrMeta+K");
  const palette = appPage.getByTestId("os-command-palette");
  await expect(palette).toBeVisible();
  const search = palette.getByPlaceholder("Search apps, sessions, and actions…");
  await search.fill("tasks");
  await paletteRow(palette, "app.open.tasks").click();
  await expect(appWindow(appPage, "tasks")).toBeVisible();

  await appPage.keyboard.press("ControlOrMeta+K");
  await search.fill("Palette target");
  await expect(palette.getByTestId(`os-palette-session-${session.id}`)).toBeVisible();
  await search.press("Enter");
  const sessionWin = sessionWindow(appPage, session.id);
  await expect(sessionWin).toBeVisible();
  await expect(palette).toHaveCount(0);
  const composer = sessionWin.getByRole("textbox", { name: "Session prompt" });
  await expect(composer).toBeVisible();
  await composer.focus();
  await appPage.keyboard.press("ControlOrMeta+K");
  await expect(palette).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(palette).toHaveCount(0);

  await appPage.goto(runtime.url(`/agents/${browserLifecycleAgent}`), {
    waitUntil: "domcontentloaded",
  });
  const agentsWin = appWindow(appPage, "agents");
  await expect(agentsWin).toBeVisible();
  const runtimeTrigger = agentsWin.getByTestId("agent-detail-runtime-select");
  await expect(runtimeTrigger).toBeEnabled();
  await runtimeTrigger.focus();
  await appPage.keyboard.press("ControlOrMeta+J");
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
});

test("Herdr E2E-016: ⌘E lands on a session while the shared chords keep their rules", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const target = await createNamedSession(runtime, workspace.id, "Palette view target");
  await createNamedSession(runtime, workspace.id, "Palette view neighbour");

  // ⌘E deep-opens the palette inside the Sessions view.
  await appPage.keyboard.press("ControlOrMeta+E");
  const palette = appPage.getByTestId("os-command-palette");
  await expect(palette).toBeVisible();
  await expect(palette).toHaveAttribute("data-palette-view", "sessions");
  await expect(palette.getByTestId("os-palette-breadcrumb")).toContainText("Sessions");

  // Chips narrow by state class and report truthful counts.
  const idleChip = palette.getByTestId("os-palette-session-filter-idle");
  await expect(idleChip).toBeVisible();
  await palette.getByTestId("os-palette-session-filter-finished").click();
  await expect(palette.getByTestId(`os-palette-session-view-${target.id}`)).toHaveCount(0);
  await idleChip.click();
  const targetRow = palette.getByTestId(`os-palette-session-view-${target.id}`);
  await expect(targetRow).toBeVisible();

  // Typing narrows, ⏎ lands on the session window and closes the palette.
  const search = palette.getByPlaceholder("Search sessions…");
  await search.fill("Palette view target");
  await expect(palette.getByTestId(/^os-palette-session-view-/)).toHaveCount(1);
  await search.press("Enter");
  const sessionWin = sessionWindow(appPage, target.id);
  await expect(sessionWin).toBeVisible();
  await expect(palette).toHaveCount(0);

  // Closing the window and activating the row again restores it (US-030.EC-4).
  await sessionWin.getByRole("button", { name: "Close window" }).click();
  await expect(sessionWin).toHaveCount(0);
  await appPage.keyboard.press("ControlOrMeta+E");
  await palette.getByTestId(`os-palette-session-view-${target.id}`).click();
  await expect(sessionWindow(appPage, target.id)).toBeVisible();

  // ⌘B toggles the sessions sidebar.
  const sidebar = appPage.getByTestId("session-sidebar");
  const sidebarVisible = await sidebar.isVisible();
  await appPage.keyboard.press("ControlOrMeta+B");
  await expect(sidebar).toBeVisible({ visible: !sidebarVisible });

  // `?` opens the cheatsheet outside editables and types normally inside one.
  await appPage.keyboard.press("Shift+Slash");
  const cheatsheet = appPage.getByTestId("os-shortcuts-dialog");
  await expect(cheatsheet).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(cheatsheet).toHaveCount(0);

  const composer = sessionWindow(appPage, target.id).getByRole("textbox", {
    name: "Session prompt",
  });
  await composer.focus();
  await appPage.keyboard.press("?");
  await expect(composer).toHaveText("?");
  await expect(appPage.getByTestId("os-shortcuts-dialog")).toHaveCount(0);

  // ⌘K still fires from inside an editable after the keymap migration.
  await appPage.keyboard.press("ControlOrMeta+K");
  await expect(palette).toBeVisible();
  await expect(palette).not.toHaveAttribute("data-palette-view", "sessions");
  await appPage.keyboard.press("ControlOrMeta+K");
  await expect(appPage.getByTestId("os-palette-action-panel")).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("os-palette-action-panel")).toHaveCount(0);
  await appPage.keyboard.press("Escape");
  await expect(palette).toHaveCount(0);

  // Jump-to-attention no-ops calmly when nothing needs the operator.
  await appPage.keyboard.press("Control+Alt+KeyA");
  await expect(appPage.getByText("No session needs attention.")).toBeVisible();
});

test("Herdr E2E-019: palette views push, pop, and always reopen at the root", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  await createNamedSession(runtime, workspace.id, "Nested view session");

  await appPage.keyboard.press("ControlOrMeta+K");
  const palette = appPage.getByTestId("os-command-palette");
  await expect(palette).toBeVisible();
  await expect(palette.getByTestId("os-palette-breadcrumb")).toHaveCount(0);

  // A root view entry pushes the view instead of closing the palette.
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("Sessions");
  await paletteRow(palette, "palette.view.sessions").click();
  await expect(palette).toHaveAttribute("data-palette-view", "sessions");
  await expect(palette.getByTestId("os-palette-breadcrumb")).toContainText("Sessions");
  await expect(paletteRow(palette, "palette.view.sessions")).toHaveCount(0);

  // ⌫ edits the query while it has text, and pops only once it is empty.
  const search = palette.getByPlaceholder("Search sessions…");
  await search.fill("nested");
  await search.press("Backspace");
  await expect(search).toHaveValue("neste");
  await expect(palette).toHaveAttribute("data-palette-view", "sessions");
  await search.fill("");
  await search.press("Backspace");
  await expect(palette.getByTestId("os-palette-breadcrumb")).toHaveCount(0);
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("Sessions");
  await expect(paletteRow(palette, "palette.view.sessions")).toBeVisible();

  // Dismiss closes the whole stack, and reopening starts at the root.
  await paletteRow(palette, "palette.view.sessions").click();
  await expect(palette).toHaveAttribute("data-palette-view", "sessions");
  await appPage.keyboard.press("Escape");
  await expect(palette).toHaveCount(0);

  await appPage.keyboard.press("ControlOrMeta+K");
  await expect(palette).toBeVisible();
  await expect(palette.getByTestId("os-palette-breadcrumb")).toHaveCount(0);
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("Sessions");
  await expect(paletteRow(palette, "palette.view.sessions")).toBeVisible();
});

test("Command palette E2E-009: Tasks reports truthful zero counts and clears one filter", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Palette filter target");

  const palette = await openCommandPalette(appPage);
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("Tasks");
  await paletteRow(palette, "palette.view.tasks").click();
  await expect(palette).toHaveAttribute("data-palette-view", "tasks");
  await expect(palette.getByTestId(`os-palette-domain-task-${task.id}`)).toBeVisible();

  const failed = palette.getByTestId("os-palette-domain-filter-failed");
  await expect(failed).toHaveAccessibleName("Failed, 0");
  await failed.click();
  await expect(palette.getByText("No tasks are failed.")).toBeVisible();

  const search = palette.getByPlaceholder("Search tasks…");
  await search.press("Backspace");
  await expect(palette.getByTestId(`os-palette-domain-task-${task.id}`)).toBeVisible();
  await expect(palette.getByTestId("os-palette-domain-filter-all")).toHaveAttribute(
    "aria-pressed",
    "true"
  );
});

test("E2E-010 and E2E-018: peers converge topology while presentation stays client-local", async ({
  appPage,
  browser,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const second = await openPeerPage(browser, runtime);
  let lastRects: Record<string, unknown> = {};
  try {
    const firstWindow = await openDockApp(appPage, "Tasks", "tasks");
    const tasksID = await windowID(firstWindow);
    const secondWindow = appWindow(second, "tasks");
    await expect(secondWindow).toBeVisible();
    const [firstClientId, secondClientId] = await Promise.all([
      browserClientId(appPage),
      browserClientId(second),
    ]);
    expect(firstClientId).not.toBe(secondClientId);

    const created = await executeWindowManagerCommand(runtime, workspace.id, {
      commandId: "desktop.create",
      payload: { desktop_id: "", name: "Peer target", purpose: "standard" },
    });
    const targetDesktop = created.snapshot.desktops.find(
      desktop => desktop.id !== "desktop-default"
    );
    if (!targetDesktop) throw new Error("desktop.create must return the created desktop");
    await expect(activeDesktop(appPage, targetDesktop.id)).toBeAttached();
    await expect(activeDesktop(second, targetDesktop.id)).toBeAttached();

    await runWindowManagerCLI(runtime, [
      "desktop",
      "switch",
      "--workspace",
      workspace.id,
      "--revision",
      String(created.snapshot.revision),
      "--client",
      firstClientId,
      "--id",
      targetDesktop.id,
    ]);

    await expect
      .poll(() => windowManagerClient(runtime, workspace.id, firstClientId))
      .toMatchObject({ active_desktop_id: targetDesktop.id });
    await expect
      .poll(() => windowManagerClient(runtime, workspace.id, secondClientId))
      .toMatchObject({ active_desktop_id: "desktop-default" });
    await expect(activeDesktop(appPage, targetDesktop.id)).toHaveAttribute("data-active", "true");
    await expect(activeDesktop(second, "desktop-default")).toHaveAttribute("data-active", "true");
    await moveWindowToNormalizedRect(
      runtime,
      workspace.id,
      tasksID,
      { x: 0.2, y: 0.18, width: 0.5, height: 0.58 },
      targetDesktop.id
    );
    await expect(firstWindow).toBeVisible();
    await expect(secondWindow).toBeHidden();
    await second
      .getByRole("button", {
        name: new RegExp(`^Desktop \\d+ of 2: ${escapeRegExp(targetDesktop.name)}$`),
      })
      .click();
    await expect(secondWindow).toBeVisible();

    await Promise.all([
      dragWindowBy(appPage, firstWindow, 58, 24),
      dragWindowBy(second, secondWindow, -42, 64),
    ]);
    await expect
      .poll(async () => {
        const [first, peer] = await Promise.all([
          windowRect(appPage, firstWindow),
          windowRect(second, secondWindow),
        ]);
        const authoritative = await authoritativeWindowRect(
          appPage,
          runtime,
          workspace.id,
          tasksID
        );
        lastRects = {
          authoritative,
          first,
          peer,
          converged:
            rectsClose(first, peer) &&
            rectsClose(first, authoritative) &&
            rectsClose(peer, authoritative),
        };
        return lastRects;
      })
      .toMatchObject({ converged: true });
  } finally {
    await second.context().close();
  }
});

test("E2E-012: blocked window-manager stream degrades without blocking work and recovers", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const degradedPage = await appPage.context().newPage();
  const stream = await routeWindowManagerStream(degradedPage, true);
  await degradedPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(degradedPage);

  const degradedStatus = degradedPage
    .getByRole("status")
    .filter({ hasText: /Layout reconnecting|Live layout disconnected/ });
  await expect(degradedStatus).toBeVisible();
  const tasks = await openDockApp(degradedPage, "Tasks", "tasks");
  const before = await windowPosition(degradedPage, tasks);
  await dragWindowBy(degradedPage, tasks, 76, 38);
  await expect.poll(() => windowPosition(degradedPage, tasks)).not.toEqual(before);

  stream.unblock();
  await expect(degradedStatus).toHaveCount(0);
  const recovered = await windowPosition(degradedPage, tasks);
  await degradedPage.reload({ waitUntil: "domcontentloaded" });
  await expect(tasks).toBeVisible();
  await expect.poll(() => windowPosition(degradedPage, tasks)).toEqual(recovered);
});

test("E2E-014: CLI window move commits semantic normalized geometry live", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const target = { x: 0.42, y: 0.18, width: 0.4, height: 0.52 };

  await moveWindowFromCLI(runtime, workspace.id, tasksID, "desktop-default", target);

  await expect
    .poll(() => windowMatchesAuthority(appPage, tasks, runtime, workspace.id, tasksID))
    .toBe(true);
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  expect(snapshot.windows[tasksID]?.floating_rect).toEqual(target);
  expect(snapshot.windows[tasksID]?.placement).toBe("floating");
});

test("E2E-015: bell approval stays live and a CLI-resolved item reports truthful conflict", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const tasksUI = tasksOperatorSelectors(appPage);
  const first = await createApprovalTask(runtime, "Primary approval");
  const bell = appPage.getByRole("button", { name: "Attention" });

  await expect(bell).toHaveText("1");
  await bell.click();
  const firstAttentionRow = appPage.getByTestId(`os-attention-task-${first.id}`);
  await expect(firstAttentionRow).toContainText(first.title);
  await firstAttentionRow.click();
  const tasksWindow = appWindow(appPage, "tasks");
  await expect(tasksWindow).toBeVisible();
  await expect(tasksUI.detailContent).toBeVisible();
  await expect(
    tasksWindow.getByRole("button", { name: "Approve", exact: true }).first()
  ).toBeVisible();
  await expect(
    tasksWindow.getByRole("button", { name: "Reject", exact: true }).first()
  ).toBeVisible();

  await tasksWindow
    .getByRole("navigation", { name: "Window path" })
    .getByRole("button", { name: "Tasks", exact: true })
    .click();
  await tasksUI.modeInbox.click();
  await expect(tasksUI.inboxItem(first.id)).toBeVisible();
  const approveResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(first.id)}/approve`)
    );
  });
  await tasksUI.inboxApprove(first.id).click();
  const approveResponse = await approveResponsePromise;
  expect(approveResponse.ok()).toBe(true);
  await expect.poll(() => taskApprovalState(runtime, first.id)).toBe("approved");
  await expect(tasksUI.inboxItem(first.id)).toHaveCount(0);
  await expect(bell).toHaveText("");

  const second = await createApprovalTask(runtime, "CLI race approval");
  await expect(bell).toHaveText("1");
  await bell.click();
  await appPage.getByTestId(`os-attention-task-${second.id}`).click();
  await expect(
    tasksWindow.getByRole("button", { name: "Approve", exact: true }).first()
  ).toBeVisible();
  await expect(
    tasksWindow.getByRole("button", { name: "Reject", exact: true }).first()
  ).toBeVisible();
  await tasksWindow
    .getByRole("navigation", { name: "Window path" })
    .getByRole("button", { name: "Tasks", exact: true })
    .click();
  await tasksUI.modeInbox.click();
  await expect(tasksUI.inboxItem(second.id)).toBeVisible();

  await approveTaskFromCLI(runtime, second.id);
  const rejectResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(second.id)}/reject`)
    );
  });
  await tasksUI.inboxReject(second.id).click();
  const rejectResponse = await rejectResponsePromise;
  expect(rejectResponse.status()).toBe(409);
  await expect(appPage.locator("[data-sonner-toast]:last-of-type")).toContainText(
    'cannot transition approval from "approved" to "rejected"'
  );
  await expect.poll(() => taskApprovalState(runtime, second.id)).toBe("approved");
  await expect(bell).toHaveText("");
});

test("E2E-024: a Tasks confirm stays scoped while a session remains interactive", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const session = await createNamedSession(runtime, workspace.id, "Scoped modal session");
  const task = await createTask(runtime, "Window-scoped deletion");

  await appPage.getByRole("button", { name: "Sessions" }).click();
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await expect(sessionsModal).toBeVisible();
  await sessionsModal.getByTestId(`os-sessions-modal-session-${session.id}`).first().click();
  await expect(sessionsModal).toHaveCount(0);
  const sessionWin = sessionWindow(appPage, session.id);
  const sessionWindowID = await windowID(sessionWin);
  const composer = sessionWin.getByRole("textbox", { name: "Session prompt" });
  await composer.fill("observe primary stream");
  await composer.press("Enter");
  await expect(sessionWin.getByTestId("chat-view")).toContainText("Primary stream is warming up.");

  await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(task.id)}`), {
    waitUntil: "domcontentloaded",
  });
  const tasksWindow = appWindow(appPage, "tasks");
  const tasksWindowID = await windowID(tasksWindow);
  const tasksUI = tasksOperatorSelectors(appPage);
  await expect(tasksUI.detailContent).toBeVisible();
  await moveWindowToNormalizedRect(runtime, workspace.id, sessionWindowID, {
    x: 0.55,
    y: 0.03,
    width: 0.42,
    height: 0.76,
  });
  await moveWindowToNormalizedRect(runtime, workspace.id, tasksWindowID, {
    x: 0.02,
    y: 0.03,
    width: 0.5,
    height: 0.74,
  });
  await expect
    .poll(() => windowMatchesAuthority(appPage, tasksWindow, runtime, workspace.id, tasksWindowID))
    .toBe(true);
  await expect
    .poll(() => windowMatchesAuthority(appPage, sessionWin, runtime, workspace.id, sessionWindowID))
    .toBe(true);

  await tasksUI.detailOverflow.click();
  await tasksUI.detailDelete.click();
  const dialog = tasksUI.detailDeleteDialog;
  await expect(dialog).toBeVisible();
  await expect(
    tasksWindow
      .locator('[data-slot="os-window-overlays"]')
      .getByTestId("tasks-detail-delete-dialog")
  ).toBeVisible();
  await expect(
    sessionWin.locator('[data-slot="os-window-overlays"]').getByTestId("tasks-detail-delete-dialog")
  ).toHaveCount(0);

  await composer.fill("session remains interactive while Tasks confirms");
  await expect(composer).toHaveText("session remains interactive while Tasks confirms");

  const [windowBefore, dialogBefore] = await Promise.all([
    windowPosition(appPage, tasksWindow),
    dialog.boundingBox(),
  ]);
  if (!dialogBefore) throw new Error("task delete dialog must have visible bounds");
  await dragWindowBy(appPage, tasksWindow, 68, 34);
  const [windowAfter, dialogAfter] = await Promise.all([
    windowPosition(appPage, tasksWindow),
    dialog.boundingBox(),
  ]);
  if (!dialogAfter) throw new Error("task delete dialog must remain visible after dragging");
  expect({
    x: Math.round(dialogAfter.x - dialogBefore.x),
    y: Math.round(dialogAfter.y - dialogBefore.y),
  }).toEqual({ x: windowAfter.x - windowBefore.x, y: windowAfter.y - windowBefore.y });

  const deleteResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "DELETE" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(task.id)}`)
    );
  });
  await tasksUI.detailDeleteConfirm.click();
  expect((await deleteResponsePromise).ok()).toBe(true);
  await expect(dialog).toHaveCount(0);
  await expect(sessionWin).toBeVisible();
  await expect(composer).toHaveText("session remains interactive while Tasks confirms");
});

test("E2E-017: palette unwinds above the bell one overlay at a time", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  await appPage.getByRole("button", { name: "Attention" }).click();
  await expect(appPage.getByTestId("os-bell-popover")).toBeVisible();

  await appPage.keyboard.press("ControlOrMeta+K");
  await expect(appPage.getByTestId("os-command-palette")).toBeVisible();
  await expect(appPage.getByTestId("os-bell-popover")).toHaveCount(0);

  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("os-command-palette")).toHaveCount(0);
  await appPage.keyboard.press("Escape");
  await expect
    .poll(() =>
      appPage.getByTestId("os-desktop").evaluate(node => node.contains(document.activeElement))
    )
    .toBe(true);
});

test("E2E-019: raw layout validate rejects invalid topology and apply commits atomically", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  await openDockApp(appPage, "Tasks", "tasks");
  const basePath = windowManagerPath(workspace.id);
  const before = await windowManagerSnapshot(runtime, workspace.id);
  const exported = await runtime.requestJSON<WindowManagerLayoutDocument>(`${basePath}/layout`);
  const invalid = structuredClone(exported);
  invalid.desktops = [];

  const validation = await runtime.requestJSON<{
    workspace_id: string;
    valid: boolean;
    diagnostics: Array<{ code: string; path: string; message: string }>;
  }>(`${basePath}/layout/validate`, {
    method: "POST",
    body: JSON.stringify({ workspace_id: workspace.id, document: invalid }),
  });
  expect(validation.valid).toBe(false);
  expect(validation.diagnostics.length).toBeGreaterThan(0);

  const invalidApply = await fetch(runtime.url(`${basePath}/layout`), {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      workspace_id: workspace.id,
      expected_revision: before.revision,
      actor: { kind: "e2e", id: "os-shell" },
      origin: "web-e2e",
      document: invalid,
    }),
  });
  expect(invalidApply.ok).toBe(false);
  expect([400, 422]).toContain(invalidApply.status);
  expect((await invalidApply.json()) as { code: string }).toMatchObject({
    code: "window_manager_invalid_topology",
  });
  expect((await windowManagerSnapshot(runtime, workspace.id)).revision).toBe(before.revision);

  const valid = structuredClone(exported);
  const defaultDesktop = valid.desktops.find(desktop => desktop.id === "desktop-default");
  if (!defaultDesktop) throw new Error("exported layout must include the default desktop");
  defaultDesktop.name = "Applied layout";
  const validCheck = await runtime.requestJSON<{ valid: boolean }>(`${basePath}/layout/validate`, {
    method: "POST",
    body: JSON.stringify({ workspace_id: workspace.id, document: valid }),
  });
  expect(validCheck.valid).toBe(true);

  const applied = await runtime.requestJSON<WindowManagerCommandResult>(`${basePath}/layout`, {
    method: "PUT",
    body: JSON.stringify({
      workspace_id: workspace.id,
      expected_revision: before.revision,
      actor: { kind: "e2e", id: "os-shell" },
      origin: "web-e2e",
      document: valid,
    }),
  });
  expect(applied.applied).toBe(true);
  expect(applied.snapshot.revision).toBe(before.revision + 1);
  await expect(
    appPage.getByRole("button", { name: "Desktop 1 of 1: Applied layout" })
  ).toHaveAttribute("aria-current", "page");
});

test("E2E-022: menubar traverses five menus and operates workspaces, sessions, Desktops, shortcuts, About, and settings", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareProjectShell(appPage, runtime);
  const secondWorkspace = await addSecondWorkspace(runtime, workspace.id);
  await switchWorkspace(appPage, secondWorkspace.id, secondWorkspace.name);

  await openMenu(appPage, "Session");
  await appPage.getByRole("menuitem", { name: /^New session/ }).click();
  const createDialog = appPage.getByTestId("session-create-dialog");
  await expect(createDialog).toBeVisible();
  await createDialog.getByTestId("session-create-mode-advanced").click();
  await createDialog.getByTestId("session-create-agent-select").click();
  await appPage.getByTestId(`agent-command-item-${browserLifecycleAgent}`).click();
  await expect(createDialog.getByTestId("session-create-agent-select")).toContainText(
    browserLifecycleAgent
  );
  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await createDialog.getByTestId("session-create-submit").click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();
  const createPayload = (await createResponse.json()) as { session?: { id?: string } };
  const createdSessionId = createPayload.session?.id ?? "";
  expect(createdSessionId).not.toBe("");
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(
      `/agents/${encodeURIComponent(browserLifecycleAgent)}/sessions/${encodeURIComponent(createdSessionId)}`
    );
  await expect(sessionWindow(appPage, createdSessionId)).toBeVisible();
  await expect(appPage.locator('[data-slot="os-menubar-workspace"]')).toContainText(
    secondWorkspace.name
  );

  // One menubar: arrow keys traverse it and hovering a sibling switches menus.
  await openMenu(appPage, "Session");
  await appPage.keyboard.press("ArrowRight");
  await expect(appPage.getByTestId("os-menu-go")).toBeVisible();
  await appPage.getByRole("menuitem", { name: "Help", exact: true }).hover();
  await expect(appPage.getByTestId("os-menu-help")).toBeVisible();
  await expect(appPage.getByTestId("os-menu-go")).toHaveCount(0);
  await appPage.keyboard.press("Escape");

  await openMenu(appPage, "Window");
  await appPage.getByRole("menuitem", { name: /^Desktops overview/ }).click();
  const desktops = appPage.locator('[data-slot="desktops-overview"]');
  await expect(desktops).toBeVisible();
  await expect(desktops.getByRole("heading", { name: "Desktops" })).toBeVisible();
  await expect(desktops.getByRole("button", { name: "Current desktop Desktop 1" })).toBeVisible();
  await appPage.keyboard.press("Escape");

  await openMenu(appPage, "Go");
  await appPage.getByRole("menuitem", { name: /^Workspace picker/ }).click();
  const workspaces = appPage.getByTestId("os-workspaces-overview");
  await expect(workspaces).toBeVisible();
  // Command-Tab strip: tiles are listbox options; current carries the check.
  await expect(
    workspaces.getByRole("option", {
      name: `${secondWorkspace.name} (current)`,
    })
  ).toBeVisible();
  await expect(workspaces.getByRole("option", { name: workspace.name, exact: true })).toBeVisible();
  await expect(workspaces.getByTestId("os-workspaces-caption")).toContainText(secondWorkspace.name);
  await appPage.keyboard.press("Escape");
  await expect(workspaces).toHaveCount(0);
  // ⇧⌘O is the registered workspace picker chord.
  await appPage.keyboard.press("ControlOrMeta+Shift+O");
  await expect(appPage.getByTestId("os-workspaces-overview")).toBeVisible();
  await appPage.keyboard.press("Escape");
  await appPage.keyboard.press("ControlOrMeta+Shift+D");
  await expect(desktops).toBeVisible();
  await appPage.keyboard.press("Escape");

  await openMenu(appPage, "Help");
  await appPage.getByRole("menuitem", { name: /^Keyboard shortcuts/ }).click();
  const shortcuts = appPage.getByTestId("os-shortcuts-dialog");
  await expect(shortcuts).toBeVisible();
  // Every registry action is listed, bound or not.
  await expect(shortcuts.getByTestId("os-shortcut-row-window.close")).toContainText("⌘W");
  await expect(shortcuts.getByTestId("os-shortcut-row-workspace.picker")).toContainText("⌘⇧O");
  await expect(shortcuts.getByTestId("os-shortcut-row-window.tile.top")).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(shortcuts).toHaveCount(0);

  await openMenu(appPage, "CompozyOS");
  await appPage.getByRole("menuitem", { name: /^About CompozyOS/ }).click();
  const about = appPage.getByTestId("os-about-dialog");
  await expect(about).toBeVisible();
  await expect(about.getByTestId("os-about-row-pid")).not.toBeEmpty();
  await appPage.keyboard.press("Escape");
  await expect(about).toHaveCount(0);

  await appPage.getByRole("button", { name: "Settings" }).click();
  await expect(appWindow(appPage, "settings")).toBeVisible();
});

test("E2E-025: occupied drag previews, cancels cleanly, then commits one structural reflow", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const settings = await openDockApp(appPage, "Settings", "settings");
  const tasksID = await windowID(tasks);
  const settingsID = await windowID(settings);
  await arrangeWindows(
    runtime,
    workspace.id,
    "desktop-default",
    [tasksID],
    "horizontal",
    "group-drag-target",
    { x: 0.25, y: 0.2, width: 0.5, height: 0.6 }
  );
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");
  const preview = appPage.locator('[data-slot="window-manager-command-preview"]');
  const beforeCancel = await windowManagerSnapshot(runtime, workspace.id);
  const tasksBox = await tasks.boundingBox();
  if (!tasksBox) throw new Error("tiled Tasks window must be visible");

  let grip = await windowGrip(settings);
  await appPage.mouse.move(grip.x, grip.y);
  await appPage.mouse.down();
  // The occupied center swaps whole units; grouping as tabs is the deck/head
  // gesture, never a body drop.
  await appPage.mouse.move(tasksBox.x + tasksBox.width / 2, tasksBox.y + tasksBox.height / 2, {
    steps: 8,
  });
  await expect(preview).toContainText("Swap windows");
  await appPage.keyboard.press("Escape");
  await expect(preview).toHaveCount(0);
  await appPage.mouse.up();
  expect((await windowManagerSnapshot(runtime, workspace.id)).revision).toBe(beforeCancel.revision);
  expect((await windowManagerSnapshot(runtime, workspace.id)).windows[settingsID]?.placement).toBe(
    "floating"
  );

  grip = await windowGrip(settings);
  await appPage.mouse.move(grip.x, grip.y);
  await appPage.mouse.down();
  await appPage.mouse.move(tasksBox.x + tasksBox.width * 0.9, tasksBox.y + tasksBox.height / 2, {
    steps: 8,
  });
  await expect(preview).toContainText("Split");
  await appPage.mouse.up();

  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      const group = layoutGroupForWindow(snapshot, tasksID);
      return (
        snapshot.revision > beforeCancel.revision &&
        group !== null &&
        nodeWindowIds(group.root).sort().join(",") === [settingsID, tasksID].sort().join(",") &&
        group.root.kind === "split"
      );
    })
    .toBe(true);
  const committed = await windowManagerSnapshot(runtime, workspace.id);
  expect(committed.windows[tasksID]?.placement).toBe("tiled");
  expect(committed.windows[settingsID]?.placement).toBe("tiled");
  const signature = layoutSignature(committed, "desktop-default");

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(tasks).toBeVisible();
  await expect(settings).toBeVisible();
  expect(
    layoutSignature(await windowManagerSnapshot(runtime, workspace.id), "desktop-default")
  ).toEqual(signature);
});

test("E2E-026: normalized tile topology converges across viewports and CLI arrange updates live", async ({
  appPage,
  browser,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const settings = await runtime.requestJSON<WindowManagerSettingsResponse>(
    "/api/settings/window-manager"
  );
  const second = await openPeerPage(browser, runtime);
  try {
    const firstWindow = await openDockApp(appPage, "Tasks", "tasks");
    const tasksID = await windowID(firstWindow);
    const secondWindow = appWindow(second, "tasks");
    await expect(secondWindow).toBeVisible();

    await appPage.keyboard.press("Control+Alt+ArrowLeft");
    await expect
      .poll(async () => {
        const snapshot = await windowManagerSnapshot(runtime, workspace.id);
        return {
          placement: snapshot.windows[tasksID]?.placement,
          frame: normalizedFrameForWindow(snapshot, tasksID),
        };
      })
      .toEqual({
        placement: "tiled",
        frame: { x: 0, y: 0, width: 0.5, height: 1 },
      });
    await expect(firstWindow).toHaveAttribute("data-window-placement", "tiled");
    await expect(secondWindow).toHaveAttribute("data-window-placement", "tiled");
    const firstRect = await windowRect(appPage, firstWindow);
    const secondRect = await windowRect(second, secondWindow);
    expect(firstRect.w).not.toBe(secondRect.w);

    await arrangeWindowsFromCLI(
      runtime,
      workspace.id,
      "desktop-default",
      [tasksID],
      "horizontal",
      "group-cli-right",
      { x: 0.5, y: 0, width: 0.5, height: 1 }
    );
    await expect
      .poll(async () =>
        normalizedFrameForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
      )
      .toEqual({ x: 0.5, y: 0, width: 0.5, height: 1 });
    await expect
      .poll(async () => {
        const [first, peer, firstLayer, peerLayer] = await Promise.all([
          windowRect(appPage, firstWindow),
          windowRect(second, secondWindow),
          winLayerSize(appPage),
          winLayerSize(second),
        ]);
        return (
          Math.abs(first.x - rightHalfTileX(firstLayer.width, settings.config.gaps)) <= 1 &&
          Math.abs(peer.x - rightHalfTileX(peerLayer.width, settings.config.gaps)) <= 1
        );
      })
      .toBe(true);
  } finally {
    await second.context().close();
  }
});

test("E2E-027: palette tile, drag-away, and reduced-motion gesture each commit one command", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);

  await appPage.keyboard.press("ControlOrMeta+K");
  const palette = appPage.getByTestId("os-command-palette");
  await expect(palette).toBeVisible();
  const search = palette.getByPlaceholder("Search apps, sessions, and actions…");
  await search.fill("Tile left half");
  await search.press("Enter");
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      return {
        placement: snapshot.windows[tasksID]?.placement,
        frame: normalizedFrameForWindow(snapshot, tasksID),
      };
    })
    .toEqual({
      placement: "tiled",
      frame: { x: 0, y: 0, width: 0.5, height: 1 },
    });
  await expect(appPage.locator('[data-slot="dialog-overlay"]')).toHaveCount(0);

  const layerBox = await appPage.locator('[data-slot="os-win-layer"]').boundingBox();
  if (!layerBox) throw new Error("win-layer must be visible");
  const grip = await windowGrip(tasks);
  await appPage.mouse.move(grip.x, grip.y);
  await appPage.mouse.down();
  await appPage.mouse.move(layerBox.x + layerBox.width / 2, layerBox.y + 260, { steps: 10 });
  await appPage.mouse.up();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
    .toMatchObject({ placement: "floating", desktop_id: "desktop-default" });
  await expect(tasks).toHaveAttribute("data-window-placement", "floating");
  await expect
    .poll(() => windowMatchesAuthority(appPage, tasks, runtime, workspace.id, tasksID))
    .toBe(true);

  await appPage.emulateMedia({ reducedMotion: "reduce" });
  const gripAgain = await windowGrip(tasks);
  await appPage.mouse.move(gripAgain.x, gripAgain.y);
  await appPage.mouse.down();
  await appPage.mouse.move(layerBox.x + 12, layerBox.y + layerBox.height / 2, { steps: 8 });
  const preview = appPage.locator('[data-slot="window-manager-command-preview"]');
  await expect(preview).toContainText("Tile");
  await appPage.mouse.up();
  await expect
    .poll(async () =>
      normalizedFrameForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
    )
    .toEqual({ x: 0, y: 0, width: 0.5, height: 1 });
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");
});

test("E2E-028: the structural seam resizes both siblings and persists its weights", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const settings = await openDockApp(appPage, "Settings", "settings");
  const agents = await openDockApp(appPage, "Agents", "agents");
  const tasksID = await windowID(tasks);
  const settingsID = await windowID(settings);
  const agentsID = await windowID(agents);
  await arrangeWindows(
    runtime,
    workspace.id,
    "desktop-default",
    [tasksID, settingsID],
    "horizontal",
    "group-resize"
  );
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");
  await expect(settings).toHaveAttribute("data-window-placement", "tiled");
  await moveWindowToNormalizedRect(runtime, workspace.id, agentsID, {
    x: 0.4,
    y: 0.25,
    width: 0.2,
    height: 0.5,
  });
  await expect(agents).toHaveAttribute("data-window-placement", "floating");

  const beforeWeights = splitWeightsForWindow(
    await windowManagerSnapshot(runtime, workspace.id),
    tasksID
  );
  if (!beforeWeights) throw new Error("two-up arrangement must expose split weights");
  const beforeTasks = await windowRect(appPage, tasks);
  const beforeSettings = await windowRect(appPage, settings);
  const seam = appPage.getByRole("separator", { name: "Resize boundary 1" });
  await expect(seam).toHaveCount(1);
  const seamBox = await seam.boundingBox();
  if (!seamBox) throw new Error("structural seam must be visible");
  const seamCenter = {
    x: seamBox.x + seamBox.width / 2,
    y: seamBox.y + seamBox.height / 2,
  };
  expect(
    await appPage.evaluate(
      ({ x, y, windowID }) =>
        document
          .elementFromPoint(x, y)
          ?.closest(`[data-testid="os-window-${windowID}"]`)
          ?.getAttribute("data-testid"),
      { ...seamCenter, windowID: agentsID }
    )
  ).toBe(`os-window-${agentsID}`);

  const exposedSeamPoint = {
    x: seamBox.x + seamBox.width / 2,
    y: seamBox.y + 20,
  };
  await appPage.mouse.move(exposedSeamPoint.x, exposedSeamPoint.y);
  await appPage.mouse.down();
  await appPage.mouse.move(exposedSeamPoint.x + 150, exposedSeamPoint.y, {
    steps: 8,
  });
  await appPage.mouse.up();
  await expect
    .poll(async () =>
      splitWeightsForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
    )
    .not.toEqual(beforeWeights);
  await expect
    .poll(async () => (await windowRect(appPage, tasks)).w)
    .toBeGreaterThan(beforeTasks.w);
  await expect
    .poll(async () => (await windowRect(appPage, settings)).w)
    .toBeLessThan(beforeSettings.w);

  const persistedWeights = splitWeightsForWindow(
    await windowManagerSnapshot(runtime, workspace.id),
    tasksID
  );
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect
    .poll(async () =>
      splitWeightsForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
    )
    .toEqual(persistedWeights);
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");
});

test("E2E-029: dropping onto a tiled window splits its group and the zoom menu arranges presets", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const settings = await openDockApp(appPage, "Settings", "settings");
  const tasksID = await windowID(tasks);
  const settingsID = await windowID(settings);

  await appPage
    .getByRole("navigation", { name: "Dock" })
    .getByRole("button", { name: "Tasks", exact: true })
    .click();
  await expect(windowFrame(tasks)).toHaveAttribute("data-focused", "");
  await appPage.keyboard.press("Control+Alt+ArrowRight");
  await expect
    .poll(async () =>
      normalizedFrameForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
    )
    .toEqual({ x: 0.5, y: 0, width: 0.5, height: 1 });
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");

  const tasksRect = await tasks.boundingBox();
  if (!tasksRect) throw new Error("snapped tasks window must be visible");
  const grip = await windowGrip(settings);
  await appPage.mouse.move(grip.x, grip.y);
  await appPage.mouse.down();
  await appPage.mouse.move(
    tasksRect.x + tasksRect.width / 2,
    tasksRect.y + tasksRect.height * 0.85,
    { steps: 10 }
  );
  await expect(appPage.locator('[data-slot="window-manager-command-preview"]')).toContainText(
    "Split"
  );
  await appPage.mouse.up();
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      const group = layoutGroupForWindow(snapshot, tasksID);
      return group
        ? {
            kind: group.root.kind,
            axis: group.root.axis,
            windows: nodeWindowIds(group.root).sort(),
          }
        : null;
    })
    .toEqual({
      kind: "split",
      axis: "vertical",
      windows: [settingsID, tasksID].sort(),
    });
  await expect(settings).toHaveAttribute("data-window-placement", "tiled");
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");

  const zoomButton = tasks.getByRole("button", { name: "Zoom window" });
  await zoomButton.hover();
  const menu = appPage.getByTestId("os-zoom-menu");
  await expect(menu).toBeVisible();
  await menu.getByTestId("os-zoom-menu-two-up").click();
  await expect
    .poll(async () => {
      const group = layoutGroupForWindow(
        await windowManagerSnapshot(runtime, workspace.id),
        tasksID
      );
      return group
        ? {
            kind: group.root.kind,
            axis: group.root.axis,
            windows: nodeWindowIds(group.root).sort(),
          }
        : null;
    })
    .toEqual({
      kind: "split",
      axis: "horizontal",
      windows: [settingsID, tasksID].sort(),
    });
});

test("E2E-009: the pager and overview keep desktop arrangements independent", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  await dragWindowBy(appPage, tasks, 140, 90);
  const tasksRect = (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]
    ?.floating_rect;
  if (!tasksRect) throw new Error("Tasks must have an authoritative floating rect");

  for (let index = 2; index <= 8; index += 1) {
    await executeWindowManagerCommand(runtime, workspace.id, {
      commandId: "desktop.create",
      payload: {
        desktop_id: `desktop-e2e-${index}`,
        name: `Desktop ${index}`,
        purpose: "standard",
      },
    });
  }

  const pager = appPage.getByRole("navigation", { name: "Desktops" });
  await expect(pager).toBeVisible();
  await expect(pager.getByRole("button", { name: "Desktop 1 of 8: Desktop 1" })).toHaveAttribute(
    "aria-current",
    "page"
  );
  await expect(pager.getByRole("button", { name: "Show 3 later desktops" })).toBeVisible();
  await assertDesktopPagerLayout(appPage);

  await pager.getByRole("button", { name: "Desktop 2 of 8: Desktop 2" }).click();
  await expect(activeDesktop(appPage, "desktop-e2e-2")).toHaveAttribute("data-active", "true");
  await expect(tasks).toBeHidden();
  await expect(appPage.getByTestId("os-desk-hint")).toBeVisible();
  const vault = await openDockApp(appPage, "Vault", "vault");
  const vaultID = await windowID(vault);
  await expect(vault).toBeVisible();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[vaultID])
    .toMatchObject({ desktop_id: "desktop-e2e-2" });

  await pager.getByRole("button", { name: "Show 3 later desktops" }).click();
  const overview = appPage.locator('[data-slot="desktops-overview"]');
  await expect(overview).toBeVisible();
  await expect(overview.getByRole("button", { name: "Switch to Desktop 6" })).toBeFocused();
  await expect(overview.getByRole("button", { name: "Current desktop Desktop 2" })).toBeVisible();
  await expect(overview.getByRole("list", { name: "Windows on Desktop 1" })).toContainText("Tasks");
  await expect(overview.getByRole("list", { name: "Windows on Desktop 2" })).toContainText("Vault");
  await overview.getByRole("button", { name: "Switch to Desktop 1" }).click();

  const tasksBack = appWindow(appPage, "tasks");
  await expect(tasksBack).toBeVisible();
  await expect(vault).toBeHidden();
  await expect(activeDesktop(appPage, "desktop-default")).toHaveAttribute("data-active", "true");
  expect(
    (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.floating_rect
  ).toEqual(tasksRect);
});

test("E2E-011: the compact stack round-trips with floating rects preserved", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const opened = await authoritativeWindowRect(appPage, runtime, workspace.id, tasksID);
  await dragWindowBy(appPage, tasks, 120, 80);
  await expect
    .poll(() => authoritativeWindowRect(appPage, runtime, workspace.id, tasksID))
    .not.toEqual(opened);
  const floatingRect = await authoritativeWindowRect(appPage, runtime, workspace.id, tasksID);
  await expect.poll(() => windowRect(appPage, tasks)).toEqual(floatingRect);

  // Below the breakpoint: stacked fullscreen presentation with the tab bar.
  await appPage.setViewportSize({ width: 390, height: 844 });
  await expect(windowFrame(tasks)).toHaveAttribute("data-presentation", "compact");
  await expect(appPage.locator('[data-slot="os-dock-tabbar"]')).toBeVisible();
  await expect(tasks.getByRole("button", { name: "Zoom window" })).toHaveCount(0);
  const closeTarget = await tasks.getByRole("button", { name: "Close window" }).boundingBox();
  const minimizeTarget = await tasks.getByRole("button", { name: "Minimize window" }).boundingBox();
  if (!closeTarget || !minimizeTarget) {
    throw new Error("compact window controls must expose measurable touch targets");
  }
  expect(closeTarget.width).toBeGreaterThanOrEqual(44);
  expect(closeTarget.height).toBeGreaterThanOrEqual(44);
  expect(minimizeTarget.width).toBeGreaterThanOrEqual(44);
  expect(minimizeTarget.height).toBeGreaterThanOrEqual(44);
  expect(closeTarget.x + closeTarget.width).toBeLessThanOrEqual(minimizeTarget.x);
  const stackBox = await tasks.boundingBox();
  const viewport = appPage.viewportSize();
  if (!stackBox || !viewport) throw new Error("compact stack window must be measurable");
  expect(Math.round(stackBox.width)).toBe(viewport.width);

  // Back above the breakpoint: the floating rect returns exactly.
  await appPage.setViewportSize({ width: 1280, height: 720 });
  await expect(windowFrame(tasks)).not.toHaveAttribute("data-presentation", "compact");
  await expect
    .poll(async () => rectsClose(await windowRect(appPage, tasks), floatingRect))
    .toBe(true);
});

test("E2E-013: appearance preferences stay client-local while minimize remains authoritative", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasksWindow = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasksWindow);

  await openMenu(appPage, "CompozyOS");
  await appPage.getByRole("menuitem", { name: "Settings → Appearance" }).click();
  await expect(appPage.getByTestId("os-appearance-pane")).toBeVisible();
  const revisionBeforePreferences = (await windowManagerSnapshot(runtime, workspace.id)).revision;
  await appPage.getByTestId("os-wallpaper-option-carbon").click();
  const wallpaper = appPage.locator('[data-slot="os-wallpaper"]');
  await expect(wallpaper).toHaveAttribute("data-wallpaper", "carbon");
  const reduceMotion = appPage.getByRole("switch", { name: "Reduce motion" });
  await reduceMotion.click();
  await expect(reduceMotion).toBeChecked();
  expect((await windowManagerSnapshot(runtime, workspace.id)).revision).toBe(
    revisionBeforePreferences
  );

  await expect(tasksWindow).toBeVisible();
  await appPage
    .getByRole("navigation", { name: "Dock" })
    .getByRole("button", { name: "Tasks", exact: true })
    .click();
  await expect(windowFrame(tasksWindow)).toHaveAttribute("data-focused", "");
  await tasksWindow.getByRole("button", { name: "Minimize window" }).click();
  await expect(tasksWindow).toBeHidden();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
    .toMatchObject({ minimized: true });
  await appPage.getByRole("button", { name: "Tasks", exact: true }).click();
  await expect(tasksWindow).toBeVisible();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
    .toMatchObject({ minimized: false });
});

test("E2E-016 / cross-workspace E2E-008, E2E-009, E2E-011: a foreign session deep link confirms before switching and exposes nothing foreign", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareProjectShell(appPage, runtime);
  const secondWorkspace = await addSecondWorkspace(runtime, workspace.id);
  const session = await createNamedSession(runtime, secondWorkspace.id, "cross-workspace-session");
  const confirmSwitch = sessionWorkspaceSwitchSelectors(appPage);
  const canonicalDeepLink = `/agents/${encodeURIComponent(session.agent_name)}/sessions/${encodeURIComponent(session.id)}`;
  const permalink = `/session/${encodeURIComponent(session.id)}`;
  const menubarWorkspace = appPage.locator('[data-slot="os-menubar-workspace"]');

  // Arrange A so its integrity is checkable across the whole confirmation journey.
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const openedTasks = await authoritativeWindowRect(appPage, runtime, workspace.id, tasksID);
  await dragWindowBy(appPage, tasks, 130, 70);
  await expect
    .poll(() => authoritativeWindowRect(appPage, runtime, workspace.id, tasksID))
    .not.toEqual(openedTasks);
  const arrangedTasks = await authoritativeWindowRect(appPage, runtime, workspace.id, tasksID);
  await expect.poll(() => windowRect(appPage, tasks)).toEqual(arrangedTasks);

  // Every API path the browser touches, so the pre-confirm reads can be proved minimal.
  const apiRequests: string[] = [];
  appPage.on("request", request => {
    const { pathname } = new URL(request.url());
    if (pathname.startsWith("/api/")) apiRequests.push(pathname);
  });

  // Canonical deep link owned by B: the confirmation names B, nothing switches, nothing opens.
  await appPage.goto(runtime.url(canonicalDeepLink), { waitUntil: "domcontentloaded" });
  await expect(confirmSwitch.dialog).toBeVisible();
  await expect(confirmSwitch.dialog).toContainText(secondWorkspace.name);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(canonicalDeepLink);
  await expect.poll(() => new URL(appPage.url()).search).toBe("?workspaceSwitch=confirm");
  await expect(menubarWorkspace).toContainText(workspace.name);
  await expect(sessionWindow(appPage, session.id)).toHaveCount(0);
  await expect(appPage.getByText("cross-workspace-session", { exact: true })).toHaveCount(0);

  // Only the owner projection resolves the session miss. The shell may read B's
  // workspace-scoped loop-request projection for the authorized cross-workspace
  // attention feed, but no foreign session payload is read before confirmation.
  expect(new Set(apiRequests.filter(pathname => pathname.includes(session.id)))).toEqual(
    new Set([
      `/api/workspaces/${workspace.id}/sessions/${session.id}`,
      `/api/sessions/${session.id}/owner`,
    ])
  );
  expect(
    new Set(
      apiRequests.filter(pathname => pathname.startsWith(`/api/workspaces/${secondWorkspace.id}/`))
    )
  ).toEqual(new Set([`/api/workspaces/${secondWorkspace.id}/loop-requests`]));

  // Cancel keeps A active with its arrangement intact and falls back to today's not-found.
  await confirmSwitch.cancel.click();
  await expect(confirmSwitch.dialog).toHaveCount(0);
  await expect(appPage.locator("[data-sonner-toast]:last-of-type")).toContainText(
    "Session not found"
  );
  const fallbackAgentPath = `/agents/${encodeURIComponent(session.agent_name)}`;
  const agentsWindow = appWindow(appPage, "agents");
  await expect(agentsWindow).toBeVisible();
  const agentsID = await windowID(agentsWindow);
  await expect
    .poll(
      async () =>
        (await windowManagerSnapshot(runtime, workspace.id)).windows[agentsID]?.route.pathname
    )
    .toBe(fallbackAgentPath);
  await expect(menubarWorkspace).toContainText(workspace.name);
  await expect(sessionWindow(appPage, session.id)).toHaveCount(0);
  const tasksAfterCancel = appWindow(appPage, "tasks");
  await expect(tasksAfterCancel).toBeVisible();
  await expect
    .poll(async () => rectsClose(await windowRect(appPage, tasksAfterCancel), arrangedTasks))
    .toBe(true);

  // A session that exists nowhere keeps the not-found outcome with no confirmation at all.
  await appPage.goto(
    runtime.url(`/agents/${encodeURIComponent(session.agent_name)}/sessions/sess_missing`),
    { waitUntil: "domcontentloaded" }
  );
  await expect(appPage.locator("[data-sonner-toast]:last-of-type")).toContainText(
    "Session not found"
  );
  await expect(confirmSwitch.dialog).toHaveCount(0);
  await expect(menubarWorkspace).toContainText(workspace.name);

  // The permalink form offers the same confirmation; confirming switches to B and opens the session.
  await appPage.goto(runtime.url(permalink), { waitUntil: "domcontentloaded" });
  await expect(confirmSwitch.dialog).toBeVisible();
  await expect(confirmSwitch.dialog).toContainText(secondWorkspace.name);
  await expect.poll(() => new URL(appPage.url()).search).toBe("?workspaceSwitch=confirm");
  await expect(menubarWorkspace).toContainText(workspace.name);

  await confirmSwitch.confirm.click();
  await expect(confirmSwitch.dialog).toHaveCount(0);
  await expect(menubarWorkspace).toContainText(secondWorkspace.name);
  await expect(sessionWindow(appPage, session.id)).toBeVisible();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(canonicalDeepLink);
  await expect(appPage.getByText("cross-workspace-session").first()).toBeVisible();
});

test("E2E-020: compact keeps deep links, truthful badges, and the rail overlay working", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  await createApprovalTask(runtime, "Compact parity approval");
  const detailTask = await createTask(runtime, "Compact deep link target");

  await appPage.setViewportSize({ width: 390, height: 844 });
  await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(detailTask.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await completeOnboardingIfPrompted(appPage);

  // Deep link lands focused in the stack.
  const tasksWindow = appWindow(appPage, "tasks");
  await expect(tasksWindow).toBeVisible();
  await expect(windowFrame(tasksWindow)).toHaveAttribute("data-presentation", "compact");
  await expect(tasksWindow).toContainText(detailTask.title);

  // Badges render truthfully in the tab bar (awaiting-approval projection).
  const tabbar = appPage.locator('[data-slot="os-dock-tabbar"]');
  await expect(tabbar).toBeVisible();
  await expect(tabbar.locator('[data-app="tasks"] [data-slot="os-dock-badge"]')).toHaveText("1");

  // Tab-bar semantics: tapping the focused app switches to it — never minimizes.
  await tabbar.locator('[data-app="tasks"]').click();
  await expect(tasksWindow).toBeVisible();

  // The sessions catalog presents as a global modal; dismissing returns intact.
  await tabbar.locator('[data-app="session"]').click();
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await expect(sessionsModal).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(sessionsModal).toHaveCount(0);
  await expect(tasksWindow).toBeVisible();
  await expect(tasksWindow).toContainText(detailTask.title);
});

test("E2E-021: the system reduced-motion preference wins over the in-product toggle", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  await appPage.emulateMedia({ reducedMotion: "reduce" });

  // In-product motion stays "full" (toggle off — the default), system says
  // reduce: dock magnification must stay static (US-015.EC-1).
  const tasksWindow = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasksWindow);
  const dockItem = appPage.locator('[data-slot="os-dock"] [data-app="tasks"]');
  const box = await dockItem.boundingBox();
  if (!box) throw new Error("dock item must be visible");
  await appPage.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await appPage.mouse.move(box.x + box.width / 2 + 4, box.y + box.height / 2, { steps: 3 });
  await expect
    .poll(() => dockItem.evaluate(element => (element as HTMLElement).style.transform))
    .toBe("");

  await tasksWindow.getByRole("button", { name: "Minimize window" }).click();
  await expect(tasksWindow).toBeHidden();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
    .toMatchObject({ minimized: true });
});

test("E2E-030 (logical E2E-001): a real drag previews, cancels, then merges a floating window into a deck", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const ids = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
    "marketplace",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, ids[0], [ids[1]]);
  await moveWindowToNormalizedRect(runtime, workspace.id, ids[2], {
    x: 0.56,
    y: 0.14,
    width: 0.4,
    height: 0.58,
  });

  const beforeDrag = await windowManagerSnapshot(runtime, workspace.id);
  const frame = floatingStackForWindow(beforeDrag, ids[0]);
  if (!frame) throw new Error("deck fixture must create a floating stack");
  const shell = osShellSelectors(appPage);
  await expect(shell.deck(frame.id)).toBeVisible();
  const source = shell.window(ids[2]);
  await expect
    .poll(() => windowMatchesAuthority(appPage, source, runtime, workspace.id, ids[2]))
    .toBe(true);

  const target = await shell
    .deck(frame.id)
    .getByRole("tablist", { name: "Open tabs" })
    .boundingBox();
  if (!target) throw new Error("deck tab row must expose a visible drop region");
  const grip = await windowGrip(source);
  await appPage.mouse.move(grip.x, grip.y);
  await appPage.mouse.down();
  await appPage.mouse.move(target.x + target.width / 2, target.y + target.height / 2, { steps: 8 });
  await expect(appPage.locator('[data-slot="os-window-deck-insertion"]')).toBeVisible();
  await appPage.keyboard.press("Escape");
  await appPage.mouse.up();
  expect(
    floatingStackForWindow(await windowManagerSnapshot(runtime, workspace.id), ids[2])
  ).toBeNull();

  const retryGrip = await windowGrip(source);
  await appPage.mouse.move(retryGrip.x, retryGrip.y);
  await appPage.mouse.down();
  await appPage.mouse.move(target.x + target.width / 2, target.y + target.height / 2, { steps: 8 });
  await appPage.mouse.up();

  await expect
    .poll(async () =>
      floatingStackForWindow(await windowManagerSnapshot(runtime, workspace.id), ids[0])
    )
    .toMatchObject({ window_ids: expect.arrayContaining(ids) });
  await expect(shell.tabButton(ids[2])).toHaveAttribute("aria-selected", "true");
});

test("E2E-031 (logical E2E-002): a CLI tiled stack keeps both tabs pointer-reachable and dissolves", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
  ]);
  await arrangeWindowsFromCLI(
    runtime,
    workspace.id,
    "desktop-default",
    [tasksID, agentsID],
    "stack",
    "e2e-tiled-stack"
  );
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const frame = layoutGroupForWindow(snapshot, tasksID);
  if (!frame) throw new Error("CLI stack fixture must create a tiled frame");
  const frameID = frame.root.id;
  const shell = osShellSelectors(appPage);
  await expect(shell.frame(frameID)).toHaveAttribute("data-frame-kind", "tiled");
  await expect(shell.deck(frameID)).toBeVisible();
  await expect(shell.tab(tasksID)).toBeVisible();
  await expect(shell.tab(agentsID)).toBeVisible();
  await shell.tab(agentsID).click();
  await expect(shell.window(agentsID)).toHaveAttribute("data-stack-active", "");
  await expect(shell.window(tasksID)).not.toHaveAttribute("data-stack-active", "");

  await shell.tab(agentsID).getByRole("button", { name: /Close/ }).click();
  await expect(shell.tab(agentsID)).toHaveCount(0);
  await expect(shell.deck(frameID)).toHaveCount(0);
  await expect(
    shell.window(tasksID).getByRole("button", { name: "Minimize window" })
  ).toBeVisible();
});

test("E2E-032 (logical E2E-005): the tab shortcut creates a real destination-backed tab", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsID]);
  const shell = osShellSelectors(appPage);
  await shell.tabButton(tasksID).click();
  await appPage.keyboard.press("ControlOrMeta+T");

  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      return Object.values(snapshot.windows).find(window => window.app === "new-tab")?.id ?? null;
    })
    .not.toBeNull();
  // `expect.poll` does not expose its resolved value; read the authority once
  // it is stable so the rest of the assertion remains ID-opaque.
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const newWindow = Object.values(snapshot.windows).find(window => window.app === "new-tab");
  if (!newWindow) throw new Error("⌘T must create a daemon-backed new-tab window");
  await expect(shell.window(newWindow.id)).toBeVisible();
  await expect(appPage.getByTestId("new-tab-empty")).toBeVisible();
  await expect(shell.tabButton(newWindow.id)).toHaveAttribute("aria-selected", "true");
  const destinationPicker = appPage.getByTestId("os-command-palette");
  await expect(destinationPicker).toHaveAttribute("data-destination", "");
  await destinationPicker.getByPlaceholder("Open in this tab…").fill("Tasks");
  await destinationPicker.getByTestId("os-palette-command-app.open.tasks").click();
  await expect
    .poll(async () => {
      const current = await windowManagerSnapshot(runtime, workspace.id);
      const destination = Object.values(current.windows).find(
        window => window.app === "tasks" && window.id !== tasksID
      );
      return destination !== undefined && current.windows[newWindow.id] === undefined;
    })
    .toBe(true);
  const destinationSnapshot = await windowManagerSnapshot(runtime, workspace.id);
  const destinationWindow = Object.values(destinationSnapshot.windows).find(
    window => window.app === "tasks" && window.id !== tasksID
  );
  if (!destinationWindow) throw new Error("destination picker must create a fresh Tasks surface");
  expect(destinationSnapshot.windows[newWindow.id]).toBeUndefined();
  const extraIDs = await openDeckFixtureWindows(
    runtime,
    workspace.id,
    Array.from({ length: 6 }, () => "tasks")
  );
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [
    destinationWindow.id,
    ...extraIDs,
  ]);
  const lastID = extraIDs.at(-1);
  if (!lastID) throw new Error("shortcut fixture must create a ninth tab");
  await shell.tab(lastID).click();
  await expect(shell.tabButton(lastID)).toHaveAttribute("aria-selected", "true");
  await appPage.keyboard.press("Control+Tab");
  await expect(shell.tabButton(tasksID)).toHaveAttribute("aria-selected", "true");
  await appPage.keyboard.press("ControlOrMeta+3");
  await expect(shell.tabButton(destinationWindow.id)).toHaveAttribute("aria-selected", "true");
  await appPage.keyboard.press("ControlOrMeta+9");
  await expect(shell.tabButton(lastID)).toHaveAttribute("aria-selected", "true");
  const task = await createTask(runtime, "Shortcut back target");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: destinationWindow.id,
      mode: "push",
      route: { pathname: `/tasks/${encodeURIComponent(task.id)}`, search: {} },
    },
  });
  await shell.tab(destinationWindow.id).click();
  await appPage.keyboard.press("ControlOrMeta+[");
  await expect
    .poll(
      async () =>
        (await windowManagerSnapshot(runtime, workspace.id)).windows[destinationWindow.id]?.route
    )
    .toEqual({ pathname: "/tasks", search: {} });
});

test("E2E-033 (logical E2E-006): dock context opens a second app instance as a tab", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const primaryID = await windowID(tasks);
  await expect(tasks).toBeVisible();
  const tasksDockItem = appPage
    .locator('[data-slot="os-dock"]')
    .getByRole("button", { name: "Tasks", exact: true });
  await tasksDockItem.focus();
  await tasksDockItem.press("Shift+F10");
  let menu = appPage.getByTestId("os-dock-app-menu-tasks");
  await expect(menu).toBeVisible();
  await menu.getByText("Open as tab in focused window", { exact: true }).click();

  await tasksDockItem.click({ button: "right" });
  menu = appPage.getByTestId("os-dock-app-menu-tasks");
  await expect(menu).toBeVisible();
  await menu.getByText("Open as tab in focused window", { exact: true }).click();

  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      return Object.values(snapshot.windows).filter(window => window.app === "tasks").length;
    })
    .toBe(3);
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const taskIDs = Object.values(snapshot.windows)
    .filter(window => window.app === "tasks")
    .map(window => window.id);
  const stack = floatingStackForWindow(snapshot, taskIDs[0] ?? "");
  expect(stack?.window_ids).toEqual(expect.arrayContaining(taskIDs));
  const secondaryID = taskIDs.find(id => id !== primaryID);
  if (!secondaryID) throw new Error("dock context must create an independently routable tab");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: secondaryID,
      mode: "replace",
      route: { pathname: "/tasks", search: { mode: "kanban" } },
    },
  });
  await expect
    .poll(
      async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[secondaryID]?.route
    )
    .toEqual({ pathname: "/tasks", search: { mode: "kanban" } });
  const client = await windowManagerClient(runtime, workspace.id, await browserClientId(appPage));
  const stableTaskIDs = [...taskIDs].sort();
  const focusedIndex = stableTaskIDs.indexOf(client.focused_window_id ?? "");
  if (focusedIndex < 0) throw new Error("dock cycle requires one focused Tasks instance");
  const expectedCycleID = stableTaskIDs[(focusedIndex + 1) % stableTaskIDs.length];
  if (!expectedCycleID) throw new Error("dock cycle requires a successor Tasks instance");
  await tasksDockItem.click();
  await expect(osShellSelectors(appPage).tabButton(expectedCycleID)).toHaveAttribute(
    "aria-selected",
    "true"
  );
});

test("E2E-034 (logical E2E-008): a pinned tab refuses Cmd+W until unpinned", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsID]);
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.pin",
    payload: { window_id: tasksID, pinned: true },
  });

  const shell = osShellSelectors(appPage);
  await expect(shell.tab(tasksID)).toHaveAttribute("data-pinned", "");
  await expect(shell.tab(tasksID).getByRole("button", { name: /Close/ })).toHaveCount(0);
  await shell.tab(tasksID).click();
  await appPage.keyboard.press("ControlOrMeta+W");
  await expect(shell.tab(tasksID)).toBeVisible();
  await shell.tabMenu(tasksID).click({ button: "right" });
  await appPage.getByRole("menuitem", { name: "Close other tabs" }).click();
  await expect(shell.tab(tasksID)).toHaveCount(0);
  await expect(shell.window(tasksID)).toBeVisible();
  await expect(shell.tab(agentsID)).toHaveCount(0);

  const agentsReplacement = (await openDeckFixtureWindows(runtime, workspace.id, ["agents"]))[0];
  if (!agentsReplacement) throw new Error("pin fixture must create an unpinned replacement tab");
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsReplacement]);
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.pin",
    payload: { window_id: tasksID, pinned: false },
  });
  await expect(shell.tab(tasksID)).not.toHaveAttribute("data-pinned", "");
  await expect(shell.tab(tasksID).getByRole("button", { name: /Close/ })).toHaveCount(1);
  await shell.tab(tasksID).getByRole("button", { name: /Close/ }).click();
  await expect(shell.tab(tasksID)).toHaveCount(0);
});

test("E2E-035 (logical E2E-010): dragging a tab out makes a standalone window", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Tear-out navigation");
  const [rootTasksID, runTasksID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "tasks",
  ]);
  if (!rootTasksID || !runTasksID) {
    throw new Error("tear-out fixture must create two task windows");
  }
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: runTasksID,
      mode: "push",
      route: { pathname: `/tasks/${encodeURIComponent(task.id)}`, search: {} },
    },
  });
  await groupWindowsInAuthority(runtime, workspace.id, rootTasksID, [runTasksID]);
  const frame = floatingStackForWindow(
    await windowManagerSnapshot(runtime, workspace.id),
    rootTasksID
  );
  if (!frame) throw new Error("tear-out fixture must create a frame");
  const shell = osShellSelectors(appPage);
  await expect(shell.deck(frame.id)).toBeVisible();
  const tab = shell.tab(runTasksID);
  const box = await tab.boundingBox();
  if (!box) throw new Error("tab must be visible before a tear-out gesture");
  await appPage.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await appPage.mouse.down();
  await appPage.mouse.move(box.x + box.width / 2, box.y + box.height + 96, { steps: 8 });
  await appPage.mouse.up();

  await expect
    .poll(
      async () =>
        (await windowManagerSnapshot(runtime, workspace.id)).windows[runTasksID]?.placement
    )
    .toBe("floating");
  await expect(shell.deck(frame.id)).toHaveCount(0);
  await expect(shell.window(runTasksID)).toBeVisible();
  const tornOut = (await windowManagerSnapshot(runtime, workspace.id)).windows[runTasksID];
  expect(tornOut).toMatchObject({
    placement: "floating",
    route: { pathname: `/tasks/${task.id}`, search: {} },
    nav_stack: [{ pathname: "/tasks", search: {} }],
  });
  expect(tornOut?.floating_rect.width).toBeGreaterThan(0);
  expect(tornOut?.floating_rect.height).toBeGreaterThan(0);
  await expect(shell.window(rootTasksID).getByTestId("tasks-shell")).toBeVisible();
});

test("E2E-036 (logical E2E-012): deck menu merges peers in stable order and splits one member back out", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID, marketplaceID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
    "marketplace",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsID]);
  const frame = floatingStackForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
  if (!frame) throw new Error("merge fixture must create a deck");
  const shell = osShellSelectors(appPage);
  await shell.tabMenu(tasksID).click({ button: "right" });
  await appPage.getByRole("menuitem", { name: "Merge all windows" }).click();
  await expect
    .poll(async () =>
      floatingStackForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
    )
    .toMatchObject({ window_ids: [tasksID, agentsID, marketplaceID] });

  await shell.tabMenu(marketplaceID).click({ button: "right" });
  await appPage.getByRole("menuitem", { name: "Move tab to new window" }).click();
  await expect
    .poll(
      async () =>
        (await windowManagerSnapshot(runtime, workspace.id)).windows[marketplaceID]?.placement
    )
    .toBe("floating");
});

test("E2E-037 (logical E2E-016): two clients keep independent active tabs in one frame", async ({
  appPage,
  browser,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsID]);
  const frame = floatingStackForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
  if (!frame) throw new Error("client-independence fixture must create a frame");
  const peer = await openPeerPage(browser, runtime);
  try {
    const [firstClientID, secondClientID] = await Promise.all([
      browserClientId(appPage),
      browserClientId(peer),
    ]);
    await executeWindowManagerCommand(
      runtime,
      workspace.id,
      { commandId: "window.stack.set_active", payload: { window_id: tasksID } },
      firstClientID
    );
    await executeWindowManagerCommand(
      runtime,
      workspace.id,
      { commandId: "window.stack.set_active", payload: { window_id: agentsID } },
      secondClientID
    );

    await expect
      .poll(
        async () =>
          (await windowManagerClient(runtime, workspace.id, firstClientID)).stack_active?.[frame.id]
      )
      .toBe(tasksID);
    await expect
      .poll(
        async () =>
          (await windowManagerClient(runtime, workspace.id, secondClientID)).stack_active?.[
            frame.id
          ]
      )
      .toBe(agentsID);
    const shell = osShellSelectors(appPage);
    await expect(shell.window(tasksID)).toHaveAttribute("data-stack-active", "");
    await expect(peer.getByTestId(`os-window-${agentsID}`)).toHaveAttribute(
      "data-stack-active",
      ""
    );

    const [marketplaceID] = await openDeckFixtureWindows(runtime, workspace.id, ["marketplace"]);
    if (!marketplaceID) throw new Error("two-client fixture must create a membership edit");
    await groupWindowsInAuthority(runtime, workspace.id, tasksID, [marketplaceID]);
    await expect(shell.tab(marketplaceID)).toBeVisible();
    await expect(peer.getByTestId(`os-window-tab-${marketplaceID}`)).toBeVisible();
  } finally {
    await peer.context().close();
  }
});

test("E2E-038 (logical E2E-014): palette Go to tab crosses desktops by opaque identity", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID, duplicateAgentsID] = await openDeckFixtureWindows(
    runtime,
    workspace.id,
    ["tasks", "agents", "agents"]
  );
  if (!tasksID || !agentsID || !duplicateAgentsID) {
    throw new Error("palette fixture must create duplicate tab labels");
  }
  const created = await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "desktop.create",
    payload: { desktop_id: "", name: "Tab destination", purpose: "standard" },
  });
  const destination = created.snapshot.desktops.find(desktop => desktop.id !== "desktop-default");
  if (!destination) throw new Error("fixture must create a second desktop");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.move",
    payload: {
      window_id: agentsID,
      destination_desktop_id: destination.id,
      placement: "floating",
      move_group: false,
    },
  });
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.move",
    payload: {
      window_id: duplicateAgentsID,
      destination_desktop_id: destination.id,
      placement: "floating",
      move_group: false,
    },
  });
  const firstClientID = await browserClientId(appPage);
  await executeWindowManagerCommand(
    runtime,
    workspace.id,
    { commandId: "window.focus", payload: { window_id: tasksID, direction: "" } },
    firstClientID
  );

  await appPage.keyboard.press("ControlOrMeta+K");
  const palette = appPage.getByTestId("os-command-palette");
  await expect(palette).toBeVisible();
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("Agents");
  const agentsRow = palette.getByTestId(`os-palette-tab-${agentsID}`);
  const duplicateAgentsRow = palette.getByTestId(`os-palette-tab-${duplicateAgentsID}`);
  await expect(agentsRow).toContainText("Agents");
  await expect(agentsRow).toContainText("Tab destination");
  await expect(duplicateAgentsRow).toContainText("Agents");
  await expect(duplicateAgentsRow).toContainText("Tab destination");
  await palette.getByTestId(`os-palette-tab-${agentsID}`).click();
  await expect
    .poll(
      async () =>
        (await windowManagerClient(runtime, workspace.id, firstClientID)).active_desktop_id
    )
    .toBe(destination.id);
  await expect
    .poll(
      async () =>
        (await windowManagerClient(runtime, workspace.id, firstClientID)).focused_window_id
    )
    .toBe(agentsID);
});

test("E2E-039 (logical E2E-004/013): crowded decks retain overflow, tooltip, and 15-tab reachability", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const ids = await openDeckFixtureWindows(
    runtime,
    workspace.id,
    Array.from({ length: 15 }, () => "tasks")
  );
  const targetID = ids[0];
  if (!targetID) throw new Error("overflow fixture must create a target tab");
  await groupWindowsInAuthority(runtime, workspace.id, targetID, ids.slice(1));
  const frame = floatingStackForWindow(
    await windowManagerSnapshot(runtime, workspace.id),
    targetID
  );
  if (!frame) throw new Error("overflow fixture must create a deck");
  const shell = osShellSelectors(appPage);
  const deck = shell.deck(frame.id);
  const tabs = deck.getByRole("tab");
  await expect(tabs).toHaveCount(15);
  const dimensions = await deck.getByRole("tablist", { name: "Open tabs" }).evaluate(tablist => {
    return { clientWidth: tablist.clientWidth, scrollWidth: tablist.scrollWidth };
  });
  expect(dimensions.scrollWidth).toBeGreaterThan(dimensions.clientWidth);
  const lastID = ids.at(-1);
  if (!lastID) throw new Error("overflow fixture must retain its last tab");
  await shell.tab(lastID).scrollIntoViewIfNeeded();
  await expect(shell.tab(lastID)).toBeVisible();
  await appPage.mouse.move(1, 1);
  await shell.tabButton(lastID).hover();
  await expect(appPage.locator('[data-slot="tooltip-content"]')).toContainText("Tasks");
});

test("E2E-040 (logical E2E-007): Cmd+W closes an attention-bearing session tab and reopens in reverse order", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const session = await createNamedSessionForAgent(
    runtime,
    workspace.id,
    "Closed-tab survivor",
    "permission-hardening-agent"
  );
  const secondSession = await createNamedSession(runtime, workspace.id, "Closed-tab reverse order");
  const [tasksID] = await openDeckFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  const sessionID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  const secondSessionID = await openSessionWindowInAuthority(runtime, workspace.id, secondSession);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [sessionID, secondSessionID]);
  const shell = osShellSelectors(appPage);
  const layoutUnavailable = appPage
    .getByRole("status")
    .filter({ hasText: /Layout reconnecting|Live layout disconnected/ });
  await expect(shell.tab(sessionID)).toBeVisible();
  await shell.tab(sessionID).click();
  const composer = shell.window(sessionID).getByRole("textbox", { name: "Session prompt" });
  await composer.fill("exercise permission hardening");
  await composer.press("Enter");
  await expect(shell.window(sessionID).getByTestId("permission-dock")).toBeVisible();
  await expect(
    appPage.getByRole("button", { name: "Sessions" }).locator('[data-slot="os-dock-badge"]')
  ).toBeVisible();
  await expect(layoutUnavailable).toHaveCount(0);
  await appPage.keyboard.press("ControlOrMeta+W");
  await expect(shell.window(sessionID)).toHaveCount(0);
  await expect(
    appPage.getByRole("button", { name: "Sessions" }).locator('[data-slot="os-dock-badge"]')
  ).toBeVisible();
  await shell.tab(secondSessionID).click();
  await expect(layoutUnavailable).toHaveCount(0);
  await appPage.keyboard.press("ControlOrMeta+W");
  await expect(shell.window(secondSessionID)).toHaveCount(0);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(shell.window(tasksID)).toBeVisible();
  await expect(layoutUnavailable).toHaveCount(0);
  await appPage.keyboard.press("ControlOrMeta+Shift+T");
  await expect(shell.window(secondSessionID)).toBeVisible();
  await appPage.keyboard.press("ControlOrMeta+Shift+T");
  await expect(shell.window(sessionID)).toBeVisible();
  await expect(
    shell.window(sessionID).getByRole("textbox", { name: "Session prompt" })
  ).toBeVisible();
});

test("E2E-041 (logical E2E-003): Task root, Task Run, and session document heads survive deck switches", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Deck head identity");
  const run = await createTaskRun(runtime, task.id);
  const session = await createNamedSession(runtime, workspace.id, "Draft preservation");
  const [tasksID, taskRunID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "tasks",
  ]);
  if (!tasksID || !taskRunID) throw new Error("head fixture must create root and Task Run members");
  const sessionID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [taskRunID, sessionID]);
  const shell = osShellSelectors(appPage);

  await shell.tab(tasksID).click();
  await expect(shell.window(tasksID).locator('[data-slot="topbar-title"]')).toContainText("Tasks");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: taskRunID,
      mode: "push",
      route: {
        pathname: `/tasks/${encodeURIComponent(task.id)}/runs/${encodeURIComponent(run.id)}`,
        search: {},
      },
    },
  });
  await shell.tab(taskRunID).click();
  await expect(shell.window(taskRunID).getByTestId("tasks-run-detail-content")).toBeVisible();
  await expect(shell.window(taskRunID).locator('[data-slot="topbar-crumbs"]')).toContainText(
    task.title
  );
  await shell.tab(sessionID).click();
  const composer = shell.window(sessionID).getByRole("textbox", { name: "Session prompt" });
  await composer.fill("draft survives a deck switch");

  await expect(shell.window(sessionID).locator('[data-slot="topbar-title"]')).toContainText(
    "Draft preservation"
  );
  await shell.tab(tasksID).click();
  await expect(shell.window(tasksID).locator('[data-slot="topbar-title"]')).toContainText("Tasks");
  await shell.tab(sessionID).click();
  await expect(composer).toHaveText("draft survives a deck switch");
});

test("E2E-042 (logical E2E-009): breadcrumb back pops only the active tab navigation stack", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Deck-local navigation");
  const run = await createTaskRun(runtime, task.id);
  const [tasksID, agentsID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsID]);
  const shell = osShellSelectors(appPage);
  await shell.tabButton(tasksID).click();
  await expect(shell.tabButton(tasksID)).toHaveAttribute("aria-selected", "true");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: tasksID,
      mode: "push",
      route: { pathname: `/tasks/${encodeURIComponent(task.id)}`, search: {} },
    },
  });
  await expect(shell.window(tasksID).getByTestId("tasks-detail-title")).toContainText(task.title);
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: tasksID,
      mode: "push",
      route: {
        pathname: `/tasks/${encodeURIComponent(task.id)}/runs/${encodeURIComponent(run.id)}`,
        search: {},
      },
    },
  });
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.route)
    .toEqual({
      pathname: `/tasks/${task.id}/runs/${run.id}`,
      search: {},
    });
  await shell.window(tasksID).locator('[data-slot="topbar-back"]').click();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.route)
    .toEqual({ pathname: `/tasks/${task.id}`, search: {} });
  expect((await windowManagerSnapshot(runtime, workspace.id)).windows[agentsID]?.route).toEqual({
    pathname: "/agents",
    search: {},
  });
});

test("E2E-043 (logical E2E-011): CLI close removes an attention tab without losing its session", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const session = await createNamedSessionForAgent(
    runtime,
    workspace.id,
    "CLI managed attention",
    "permission-hardening-agent"
  );
  const [tasksID] = await openDeckFixtureWindows(runtime, workspace.id, ["tasks"]);
  const sessionID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [sessionID]);
  const shell = osShellSelectors(appPage);
  const sessionWindow = shell.window(sessionID);
  await shell.tab(sessionID).click();
  const composer = sessionWindow.getByRole("textbox", { name: "Session prompt" });
  await composer.fill("exercise permission hardening");
  await composer.press("Enter");
  await expect(sessionWindow.getByTestId("permission-dock")).toBeVisible();
  await expect(shell.tab(sessionID).locator('[data-slot="os-window-tab-badge"]')).toBeVisible();

  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  await runWindowManagerCLI(runtime, [
    "window",
    "close",
    "--workspace",
    workspace.id,
    "--revision",
    String(snapshot.revision),
    "--id",
    sessionID,
  ]);
  await expect(sessionWindow).toHaveCount(0);
  await appPage.getByRole("button", { name: "Sessions" }).click();
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  const sessionRow = sessionsModal.getByTestId(`os-sessions-modal-session-${session.id}`).first();
  await expect(sessionRow).toBeVisible();
  await sessionRow.click();
  const restored = appPage.locator('[data-testid^="os-window-"][data-app="session"]:visible');
  await expect(restored).toHaveCount(1);
  await expect(restored.getByTestId("permission-dock")).toBeVisible();
  await restored.getByTestId("permission-reject-menu-trigger").click();
  await restored.getByTestId("permission-reject-always").click();
  await expect(restored.getByTestId("permission-dock")).toBeHidden();
  await expect(
    appPage.getByRole("button", { name: "Sessions" }).locator('[data-slot="os-dock-badge"]')
  ).toHaveCount(0);
});

test("E2E-044 (logical E2E-015): daemon restart preserves frame membership, pin, and active tab", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
  ]);
  await groupWindowsInAuthority(runtime, workspace.id, tasksID, [agentsID]);
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.pin",
    payload: { window_id: tasksID, pinned: true },
  });
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: tasksID,
      mode: "push",
      route: { pathname: "/tasks/example-task", search: {} },
    },
  });
  const shell = osShellSelectors(appPage);
  await shell.tab(agentsID).click();
  await expect(shell.tabButton(agentsID)).toHaveAttribute("aria-selected", "true");
  await expect
    .poll(
      async () =>
        floatingStackForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)
          ?.active_id
    )
    .toBe(agentsID);
  const before = await windowManagerSnapshot(runtime, workspace.id);
  const beforeFrame = floatingStackForWindow(before, tasksID);
  if (!beforeFrame) throw new Error("restart fixture must persist a floating stack");

  const restart = await runtime.requestJSON<SettingsRestartAction>(
    "/api/settings/actions/restart",
    {
      method: "POST",
      body: "{}",
    }
  );
  await expect
    .poll(() => pollRestartStatus(runtime, restart.status_url), { timeout: 45_000 })
    .toBe("ready");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  const after = await windowManagerSnapshot(runtime, workspace.id);
  expect(floatingStackForWindow(after, tasksID)).toMatchObject({
    id: beforeFrame.id,
    window_ids: beforeFrame.window_ids,
    active_id: agentsID,
  });
  expect(after.windows[tasksID]).toMatchObject({ pinned: true });
  expect(after.windows[tasksID]?.nav_stack).toEqual([{ pathname: "/tasks", search: {} }]);
  await expect(osShellSelectors(appPage).deck(beforeFrame.id)).toBeVisible();
  await expect(osShellSelectors(appPage).tabButton(agentsID)).toHaveAttribute(
    "aria-selected",
    "true"
  );
});

test("E2E-045 (logical E2E-018): Tasks and Marketplace publish views in their window strips", async ({
  appPage,
  runtime,
}, testInfo) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Window strip drill state");
  const [tasksID, marketplaceID] = await openDeckFixtureWindows(runtime, workspace.id, [
    "tasks",
    "marketplace",
  ]);
  await moveWindowToNormalizedRect(runtime, workspace.id, tasksID, {
    x: 0.02,
    y: 0.06,
    width: 0.46,
    height: 0.78,
  });
  await moveWindowToNormalizedRect(runtime, workspace.id, marketplaceID, {
    x: 0.52,
    y: 0.06,
    width: 0.46,
    height: 0.78,
  });
  const shell = osShellSelectors(appPage);
  const tasksWindow = shell.window(tasksID);
  const marketplaceWindow = shell.window(marketplaceID);
  const tasksViews = tasksWindow.getByRole("navigation", { name: "Tasks views" });
  const marketplaceViews = marketplaceWindow.getByTestId("marketplace-kind-navigation");

  await expect(tasksViews).toBeVisible();
  await expect(marketplaceViews).toBeVisible();
  await expect(tasksWindow.locator('[data-slot="os-window-toolbar"]')).toContainText("List");
  await expect(marketplaceWindow.locator('[data-slot="os-window-toolbar"]')).toContainText(
    "Skills"
  );
  await expect(tasksWindow.locator('[data-slot="os-window-head"]')).not.toContainText("Kanban");
  await expect(marketplaceWindow.locator('[data-slot="os-window-head"]')).not.toContainText("MCPs");
  await expect(tasksWindow.getByTestId("topbar-title-text")).toHaveText("Tasks");
  await expect(marketplaceWindow.getByTestId("topbar-title-text")).toHaveText("Skills");
  await expect(
    tasksWindow.locator('[data-slot="os-window-head"] [data-slot="topbar-crumbs"]')
  ).toHaveCount(0);
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: {
      window_id: tasksID,
      mode: "push",
      route: { pathname: `/tasks/${encodeURIComponent(task.id)}`, search: {} },
    },
  });
  await expect(tasksWindow.getByTestId("tasks-detail-title")).toContainText(task.title);
  await expect(tasksWindow.getByTestId("topbar-title-text")).toHaveText(task.title);
  await expect(tasksWindow.locator('[data-slot="topbar-crumbs"]')).toBeVisible();
  await expect(tasksWindow.locator('[data-slot="os-window-head"]')).not.toContainText("List");
  await testInfo.attach("e2e-045-window-strips", {
    body: await appPage.screenshot(),
    contentType: "image/png",
  });
});

const PERF_APPS = [
  "dashboard",
  "tasks",
  "agents",
  "network",
  "loops",
  "jobs",
  "triggers",
  "marketplace",
  "bridges",
  "knowledge",
  "sandbox",
  "vault",
] as const;

test("E2E-023: the 12-window envelope holds for drag frames, restore, and convergence", async ({
  appPage,
  browser,
  runtime,
}, testInfo) => {
  const workspace = await prepareShell(appPage, runtime);
  const perfWindowIDs = new Map<string, string>();

  for (const [index, app] of PERF_APPS.entries()) {
    const id = await openWindowInAuthority(
      runtime,
      workspace.id,
      app,
      { pathname: app === "dashboard" ? "/" : `/${app}`, search: {} },
      {
        x: 0.02 + index * 0.015,
        y: 0.02 + (index % 4) * 0.04,
        width: 0.42,
        height: 0.5,
      }
    );
    perfWindowIDs.set(app, id);
  }

  await appPage.addInitScript(() => {
    const perf = {
      snapshotResponseEnd: null as number | null,
      windowsPlaced: null as number | null,
    };
    Reflect.set(window, "__osPerf", perf);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        const url = new URL(entry.name);
        if (url.pathname.endsWith("/window-manager")) {
          perf.snapshotResponseEnd = entry.startTime + entry.duration;
        }
      }
    }).observe({ type: "resource", buffered: true });
    const placed = () => {
      if (perf.windowsPlaced !== null) return;
      const count = document.querySelectorAll('[data-slot="os-window-surface"]').length;
      if (count >= 12) perf.windowsPlaced = performance.now();
    };
    // Init scripts run at document start — observe `document` itself so the
    // hook works before <html>/<body> exist.
    new MutationObserver(placed).observe(document, { childList: true, subtree: true });
    document.addEventListener("DOMContentLoaded", placed, { once: true });
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  for (const app of PERF_APPS) {
    const id = perfWindowIDs.get(app);
    if (!id) throw new Error(`performance fixture must retain the ${app} window ID`);
    await expect(osShellSelectors(appPage).window(id)).toBeAttached();
  }
  await expect
    .poll(() =>
      appPage.evaluate(() => {
        const perf = Reflect.get(window, "__osPerf") as {
          snapshotResponseEnd: number | null;
          windowsPlaced: number | null;
        };
        if (perf.snapshotResponseEnd === null || perf.windowsPlaced === null) return null;
        return perf.windowsPlaced - perf.snapshotResponseEnd;
      })
    )
    .not.toBeNull();
  const restore = await appPage.evaluate(() => {
    const perf = Reflect.get(window, "__osPerf") as {
      snapshotResponseEnd: number;
      windowsPlaced: number;
    };
    return perf.windowsPlaced - perf.snapshotResponseEnd;
  });
  expect(restore).toBeGreaterThanOrEqual(0);
  expect(restore).toBeLessThan(500);

  // The envelope measures steady-state pointer fluidity: wait until the main
  // thread has been long-task quiet so the 12 window bodies' initial content
  // burst can't masquerade as drag jank. (networkidle never settles here — the
  // shell keeps WebSocket/SSE connections open by design.)
  await installLongTaskQuietProbe(appPage);
  await waitForLongTaskQuiet(appPage);

  const dragged = appWindow(appPage, "vault");
  await focusWindow(appPage, dragged);
  const grip = await windowGrip(dragged);
  await waitForLongTaskQuiet(appPage);

  // Long-task probe during a 3s continuous drag of one window. Install it
  // after focus settles so the envelope measures drag frames, not activation.
  // Dispatch trusted Chromium input directly: Playwright's mouse helper takes
  // a full DOM snapshot for every traced action, and that recorder work runs
  // on the renderer thread that this envelope is meant to measure.
  const dragInput = await appPage.context().newCDPSession(appPage);
  await appPage.evaluate(() => {
    const tasks: number[] = [];
    Reflect.set(window, "__osLongTasks", tasks);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) tasks.push(entry.duration);
    }).observe({ type: "longtask", buffered: false });
  });
  let currentX = grip.x;
  let currentY = grip.y;
  let pressed = false;
  let longTasks: number[] = [];
  try {
    await dragInput.send("Input.dispatchMouseEvent", {
      type: "mouseMoved",
      x: currentX,
      y: currentY,
      button: "none",
    });
    await dragInput.send("Input.dispatchMouseEvent", {
      type: "mousePressed",
      x: currentX,
      y: currentY,
      button: "left",
      buttons: 1,
      clickCount: 1,
    });
    pressed = true;
    const start = Date.now();
    let step = 0;
    while (Date.now() - start < 3000) {
      const angle = (step / 20) * Math.PI * 2;
      const targetX = grip.x + 120 + Math.cos(angle) * 90;
      const targetY = grip.y + 100 + Math.sin(angle) * 60;
      for (const progress of [0.5, 1]) {
        await dragInput.send("Input.dispatchMouseEvent", {
          type: "mouseMoved",
          x: currentX + (targetX - currentX) * progress,
          y: currentY + (targetY - currentY) * progress,
          button: "left",
          buttons: 1,
        });
      }
      currentX = targetX;
      currentY = targetY;
      step += 1;
    }
    longTasks = await appPage.evaluate(() => Reflect.get(window, "__osLongTasks") as number[]);
  } finally {
    try {
      if (pressed) {
        await dragInput.send("Input.dispatchMouseEvent", {
          type: "mouseReleased",
          x: currentX,
          y: currentY,
          button: "left",
          buttons: 0,
          clickCount: 1,
        });
      }
    } finally {
      await dragInput.detach();
    }
  }
  const worstFrame = longTasks.length > 0 ? Math.max(...longTasks) : 0;

  const peerA = await openPeerPage(browser, runtime);
  const peerB = await openPeerPage(browser, runtime);
  const sandboxID = perfWindowIDs.get("sandbox");
  if (!sandboxID) throw new Error("performance fixture must retain the sandbox window ID");
  let worstPeerTask = 0;
  try {
    for (const peer of [peerA, peerB]) {
      for (const app of PERF_APPS) {
        const id = perfWindowIDs.get(app);
        if (!id) throw new Error(`performance fixture must retain the ${app} window ID`);
        await expect(osShellSelectors(peer).window(id)).toBeAttached();
      }
      await installLongTaskQuietProbe(peer);
      await waitForLongTaskQuiet(peer);
      await peer.evaluate(() => {
        const tasks: number[] = [];
        Reflect.set(window, "__osLongTasks", tasks);
        new PerformanceObserver(list => {
          for (const entry of list.getEntries()) tasks.push(entry.duration);
        }).observe({ type: "longtask", buffered: false });
      });
    }
    await moveWindowFromCLI(runtime, workspace.id, sandboxID, "desktop-default", {
      x: 0.32,
      y: 0.24,
      width: 0.38,
      height: 0.46,
    });
    for (const peer of [peerA, peerB]) {
      await expect
        .poll(() =>
          windowMatchesAuthority(peer, appWindow(peer, "sandbox"), runtime, workspace.id, sandboxID)
        )
        .toBe(true);
    }
    const peerLongTasks = await Promise.all(
      [peerA, peerB].map(peer =>
        peer.evaluate(() => Reflect.get(window, "__osLongTasks") as number[])
      )
    );
    worstPeerTask = Math.max(0, ...peerLongTasks.flat());
  } finally {
    await peerA.context().close();
    await peerB.context().close();
  }

  const envelope = {
    restoreMsFromSnapshotResponse: restore,
    dragLongTasksOver50ms: longTasks,
    worstDragFrameMs: worstFrame,
    worstPeerConvergenceTaskMs: worstPeerTask,
  };
  // Surfaced on stdout so completion notes can record the measured numbers.
  console.log(`[perf-envelope] ${JSON.stringify(envelope)}`);
  await testInfo.attach("perf-envelope", {
    body: JSON.stringify(envelope, null, 2),
    contentType: "application/json",
  });

  // Envelope: no shell frame beyond 50ms during the drag, no peer thrash.
  expect(worstFrame).toBeLessThanOrEqual(50);
  expect(worstPeerTask).toBeLessThanOrEqual(50);
});

async function prepareShell(page: Page, runtime: BrowserRuntime): Promise<WorkspacePayload> {
  await completeOnboardingIfPrompted(page);
  await expect(page.getByTestId("os-desktop")).toBeVisible();
  await page.getByRole("menuitem", { name: "Window", exact: true }).click();
  const closeWindow = page.getByTestId("os-menubar-command-window.close");
  await expect(closeWindow).toBeVisible();
  await expect(closeWindow).not.toContainText("requires an attached shell");
  await page.keyboard.press("Escape");
  const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>("/api/workspaces");
  const workspace = payload.workspaces[0];
  if (!workspace) throw new Error("OS shell E2E requires one resolved workspace");
  await switchWorkspace(page, workspace.id, workspace.name);
  return workspace;
}

async function prepareProjectShell(page: Page, runtime: BrowserRuntime): Promise<WorkspacePayload> {
  const runtimeWorkspace = await prepareShell(page, runtime);
  const projectWorkspace = await addWorkspace(runtime, runtimeWorkspace.id, "primary");
  await switchWorkspace(page, projectWorkspace.id, projectWorkspace.name);
  return projectWorkspace;
}

async function openPeerPage(browser: Browser, runtime: BrowserRuntime): Promise<Page> {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(page);
  const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>("/api/workspaces");
  const workspace = payload.workspaces[0];
  if (!workspace) throw new Error("OS shell peer E2E requires one resolved workspace");
  await switchWorkspace(page, workspace.id, workspace.name);
  return page;
}

async function installLongTaskQuietProbe(page: Page): Promise<void> {
  await page.evaluate(() => {
    const settle = { last: performance.now() };
    Reflect.set(window, "__osSettle", settle);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        settle.last = Math.max(settle.last, entry.startTime + entry.duration);
      }
    }).observe({ type: "longtask", buffered: true });
  });
}

async function waitForLongTaskQuiet(page: Page): Promise<void> {
  await page.waitForFunction(() => {
    const settle = Reflect.get(window, "__osSettle") as { last: number };
    return performance.now() - settle.last > 600;
  });
}

async function dragWindowBy(page: Page, win: ReturnType<Page["locator"]>, dx: number, dy: number) {
  const { x, y } = await windowGrip(win);
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx, y + dy, { steps: 6 });
  await page.mouse.up();
}

async function focusWindow(page: Page, win: ReturnType<Page["locator"]>) {
  const head = win.locator('[data-slot="os-window-head"]');
  const box = await head.boundingBox();
  if (!box) throw new Error("window head must have a visible bounding box before focusing");
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
}

async function windowPosition(page: Page, win: ReturnType<Page["locator"]>) {
  const [windowBox, layerBox] = await Promise.all([
    windowFrame(win).boundingBox(),
    page.locator('[data-slot="os-win-layer"]').boundingBox(),
  ]);
  if (!windowBox || !layerBox) throw new Error("window and win-layer must be visible");
  return { x: Math.round(windowBox.x - layerBox.x), y: Math.round(windowBox.y - layerBox.y) };
}

async function windowRect(page: Page, win: ReturnType<Page["locator"]>) {
  const [windowBox, layerBox] = await Promise.all([
    windowFrame(win).boundingBox(),
    page.locator('[data-slot="os-win-layer"]').boundingBox(),
  ]);
  if (!windowBox || !layerBox) throw new Error("window and win-layer must be visible");
  return {
    x: Math.round(windowBox.x - layerBox.x),
    y: Math.round(windowBox.y - layerBox.y),
    w: Math.round(windowBox.width),
    h: Math.round(windowBox.height),
  };
}

/**
 * A guaranteed drag surface on the window head: the flexible spacer remains
 * mounted while route identity is published and never overlaps interactive
 * head controls. Title nodes can be replaced during lazy route startup.
 */
async function windowGrip(win: ReturnType<Page["locator"]>): Promise<{ x: number; y: number }> {
  const spacer = win.locator('[data-slot="topbar-flex"]');
  await expect(spacer).toBeVisible();
  const box = await spacer.boundingBox();
  if (!box) throw new Error("window head drag surface must expose a visible bounding box");
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

function rectsClose(
  first: { x: number; y: number; w: number; h: number },
  second: { x: number; y: number; w: number; h: number },
  tolerance = 2
): boolean {
  return (
    Math.abs(first.x - second.x) <= tolerance &&
    Math.abs(first.y - second.y) <= tolerance &&
    Math.abs(first.w - second.w) <= tolerance &&
    Math.abs(first.h - second.h) <= tolerance
  );
}

async function winLayerSize(page: Page): Promise<{ width: number; height: number }> {
  const box = await page.locator('[data-slot="os-win-layer"]').boundingBox();
  if (!box) throw new Error("win-layer must be visible");
  return { width: box.width, height: box.height };
}

function windowManagerPath(workspaceId: string): string {
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/window-manager`;
}

async function windowManagerSnapshot(
  runtime: BrowserRuntime,
  workspaceId: string
): Promise<WindowManagerSnapshot> {
  return await runtime.requestJSON<WindowManagerSnapshot>(windowManagerPath(workspaceId));
}

async function executeWindowManagerCommand(
  runtime: BrowserRuntime,
  workspaceId: string,
  command: { commandId: string; payload: Record<string, unknown> },
  clientId?: string
): Promise<WindowManagerCommandResult> {
  const snapshot = await windowManagerSnapshot(runtime, workspaceId);
  return await runtime.requestJSON<WindowManagerCommandResult>(
    `${windowManagerPath(workspaceId)}/commands`,
    {
      method: "POST",
      body: JSON.stringify({
        workspace_id: workspaceId,
        command_id: command.commandId,
        expected_revision: snapshot.revision,
        ...(clientId ? { client_id: clientId } : {}),
        actor: { kind: "e2e", id: "os-shell" },
        origin: "web-e2e",
        payload: command.payload,
      }),
    }
  );
}

async function windowManagerClient(
  runtime: BrowserRuntime,
  workspaceId: string,
  clientId: string
): Promise<WindowManagerClientView> {
  const payload = await runtime.requestJSON<{
    workspace_id: string;
    clients: WindowManagerClientView[];
  }>(`${windowManagerPath(workspaceId)}/clients`);
  const client = payload.clients.find(candidate => candidate.client_id === clientId);
  if (!client) throw new Error(`window-manager client ${clientId} is not registered`);
  return client;
}

async function browserClientId(page: Page): Promise<string> {
  const clientId = await page.evaluate(
    key => window.localStorage.getItem(key)?.trim() ?? "",
    windowManagerClientStorageKey
  );
  if (!clientId) throw new Error("browser must publish a stable window-manager client ID");
  return clientId;
}

function activeDesktop(page: Page, desktopId: string): Locator {
  return page.locator(`[data-desktop-id="${desktopId}"]`);
}

async function currentRoute(page: Page): Promise<WindowRoute> {
  const url = new URL(page.url());
  return {
    pathname: url.pathname,
    search: Object.fromEntries(url.searchParams.entries()),
  };
}

async function pollRestartStatus(runtime: BrowserRuntime, statusURL: string): Promise<string> {
  try {
    return (await runtime.requestJSON<SettingsRestartStatus>(statusURL)).status;
  } catch {
    return "restarting";
  }
}

async function routeWindowManagerStream(page: Page, initiallyBlocked: boolean) {
  let blocked = initiallyBlocked;
  await page.routeWebSocket("**/window-manager/stream**", async socket => {
    if (blocked) {
      await socket.close({ code: 1013, reason: "E2E stream blocked" });
      return;
    }
    socket.connectToServer();
  });
  return { unblock: () => (blocked = false) };
}

async function runWindowManagerCLI(runtime: BrowserRuntime, args: string[]): Promise<void> {
  if (!runtime.paths) throw new Error("window-manager CLI E2E requires launch-mode runtime paths");
  await execFileAsync(runtime.paths.cliShim, [...args, "-o", "json"], {
    env: {
      ...process.env,
      COMPOZY_HOME: runtime.paths.homeDir,
      HOME: runtime.paths.operatorHomeDir,
    },
    maxBuffer: 10 * 1024 * 1024,
  });
}

async function moveWindowFromCLI(
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string,
  desktopId: string,
  rect: NormalizedRect
): Promise<void> {
  const snapshot = await windowManagerSnapshot(runtime, workspaceId);
  await runWindowManagerCLI(runtime, [
    "window",
    "move",
    "--workspace",
    workspaceId,
    "--revision",
    String(snapshot.revision),
    "--id",
    windowId,
    "--desktop",
    desktopId,
    "--placement",
    "floating",
    "--rect",
    normalizedRectFlag(rect),
  ]);
}

async function arrangeWindowsFromCLI(
  runtime: BrowserRuntime,
  workspaceId: string,
  desktopId: string,
  windowIds: string[],
  arrangement: "horizontal" | "vertical" | "grid" | "stack",
  groupId: string,
  frame: NormalizedRect = { x: 0, y: 0, width: 1, height: 1 }
): Promise<void> {
  const snapshot = await windowManagerSnapshot(runtime, workspaceId);
  const args = [
    "layout",
    "arrange",
    "--workspace",
    workspaceId,
    "--revision",
    String(snapshot.revision),
    "--desktop",
    desktopId,
  ];
  for (const windowId of windowIds) args.push("--window", windowId);
  args.push("--arrangement", arrangement, "--frame", normalizedRectFlag(frame), "--group", groupId);
  await runWindowManagerCLI(runtime, args);
}

async function moveWindowToNormalizedRect(
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string,
  rect: NormalizedRect,
  desktopId?: string
): Promise<void> {
  const snapshot = await windowManagerSnapshot(runtime, workspaceId);
  const destination = desktopId ?? snapshot.windows[windowId]?.desktop_id;
  if (!destination) throw new Error(`window ${windowId} has no destination desktop`);
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "window.move",
    payload: {
      window_id: windowId,
      destination_desktop_id: destination,
      placement: "floating",
      floating_rect: rect,
      move_group: false,
    },
  });
}

async function openWindowInAuthority(
  runtime: BrowserRuntime,
  workspaceId: string,
  app: string,
  route: WindowRoute,
  rect: NormalizedRect
): Promise<string> {
  const id = `e2e-authority-${app}`;
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "window.open",
    payload: {
      window: {
        id,
        app,
        route,
        desktop_id: "desktop-default",
        floating_rect: rect,
        insert_tiled: false,
      },
    },
  });
  return id;
}

/**
 * Creates opaque window identities through the real command endpoint. Test
 * fixtures must never infer IDs from `app`, because multi-instance apps and
 * reopened tabs deliberately share the same app identity.
 */
let deckFixtureSequence = 0;

async function openDeckFixtureWindows(
  runtime: BrowserRuntime,
  workspaceId: string,
  apps: readonly string[]
): Promise<string[]> {
  const batch = ++deckFixtureSequence;
  const ids: string[] = [];
  for (const [index, app] of apps.entries()) {
    const id = `e2e-deck-b${batch}-${app}-${index}`;
    await executeWindowManagerCommand(runtime, workspaceId, {
      commandId: "window.open",
      payload: {
        window: {
          id,
          app,
          route: { pathname: app === "dashboard" ? "/" : `/${app}`, search: {} },
          desktop_id: "desktop-default",
          floating_rect: {
            x: 0.04 + index * 0.08,
            y: 0.06 + index * 0.04,
            width: 0.48,
            height: 0.62,
          },
          insert_tiled: false,
        },
      },
    });
    ids.push(id);
  }
  return ids;
}

async function openSessionWindowInAuthority(
  runtime: BrowserRuntime,
  workspaceId: string,
  session: { id: string; agent_name: string }
): Promise<string> {
  const id = `e2e-deck-session-${session.id}`;
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "window.open",
    payload: {
      window: {
        id,
        app: "session",
        instance_key: session.id,
        route: {
          pathname: `/agents/${encodeURIComponent(session.agent_name)}/sessions/${encodeURIComponent(session.id)}`,
          search: {},
        },
        desktop_id: "desktop-default",
        floating_rect: { x: 0.16, y: 0.1, width: 0.48, height: 0.62 },
        insert_tiled: false,
      },
    },
  });
  return id;
}

async function groupWindowsInAuthority(
  runtime: BrowserRuntime,
  workspaceId: string,
  targetWindowID: string,
  joiningWindowIDs: readonly string[]
): Promise<void> {
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "window.stack.group",
    payload: { target_window_id: targetWindowID, window_ids: [...joiningWindowIDs] },
  });
}

function floatingStackForWindow(
  snapshot: WindowManagerSnapshot,
  windowId: string
): NonNullable<WindowManagerDesktop["floating_stacks"]>[number] | null {
  for (const desktop of snapshot.desktops) {
    const stack = desktop.floating_stacks?.find(candidate =>
      candidate.window_ids.includes(windowId)
    );
    if (stack) return stack;
  }
  return null;
}

async function arrangeWindows(
  runtime: BrowserRuntime,
  workspaceId: string,
  desktopId: string,
  windowIds: string[],
  arrangement: "horizontal" | "vertical" | "grid" | "stack",
  groupId: string,
  frame: NormalizedRect = { x: 0, y: 0, width: 1, height: 1 }
): Promise<void> {
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "layout.arrange",
    payload: {
      desktop_id: desktopId,
      window_ids: windowIds,
      arrangement,
      frame,
      group_id: groupId,
    },
  });
}

async function authoritativeWindowRect(
  page: Page,
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string
): Promise<{ x: number; y: number; w: number; h: number }> {
  const [snapshot, layer] = await Promise.all([
    windowManagerSnapshot(runtime, workspaceId),
    winLayerSize(page),
  ]);
  const window = snapshot.windows[windowId];
  if (!window) throw new Error(`window ${windowId} is missing from authority`);
  if (window.placement !== "floating") {
    throw new Error(`window ${windowId} must be floating to derive its authoritative rect`);
  }
  return {
    x: Math.round(window.floating_rect.x * layer.width),
    y: Math.round(window.floating_rect.y * layer.height),
    w: Math.round(window.floating_rect.width * layer.width),
    h: Math.round(window.floating_rect.height * layer.height),
  };
}

async function windowMatchesAuthority(
  page: Page,
  window: Locator,
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string
): Promise<boolean> {
  try {
    return rectsClose(
      await windowRect(page, window),
      await authoritativeWindowRect(page, runtime, workspaceId, windowId)
    );
  } catch {
    return false;
  }
}

function normalizedRectFlag(rect: NormalizedRect): string {
  return [rect.x, rect.y, rect.width, rect.height].join(",");
}

function nodeWindowIds(node: WindowManagerLayoutNode): string[] {
  if (node.kind === "leaf") return node.window_id ? [node.window_id] : [];
  if (node.kind === "stack") return [...(node.window_ids ?? [])];
  return (node.children ?? []).flatMap(nodeWindowIds);
}

function layoutGroupForWindow(
  snapshot: WindowManagerSnapshot,
  windowId: string
): WindowManagerDesktop["groups"][number] | null {
  for (const desktop of snapshot.desktops) {
    for (const group of desktop.groups) {
      if (nodeWindowIds(group.root).includes(windowId)) return group;
    }
  }
  return null;
}

function normalizedFrameForWindow(
  snapshot: WindowManagerSnapshot,
  windowId: string
): NormalizedRect | null {
  return layoutGroupForWindow(snapshot, windowId)?.frame ?? null;
}

function splitWeightsForWindow(snapshot: WindowManagerSnapshot, windowId: string): number[] | null {
  const root = layoutGroupForWindow(snapshot, windowId)?.root;
  return root?.kind === "split" && root.weights ? [...root.weights] : null;
}

function layoutSignature(snapshot: WindowManagerSnapshot, desktopId: string): unknown {
  const desktop = snapshot.desktops.find(candidate => candidate.id === desktopId);
  if (!desktop) throw new Error(`desktop ${desktopId} is missing from authority`);
  return {
    groups: desktop.groups,
    floating: desktop.floating,
  };
}

async function assertDesktopPagerLayout(page: Page): Promise<void> {
  const pager = page.getByRole("navigation", { name: "Desktops" });
  const dock = page.getByRole("navigation", { name: "Dock" });
  const desktopControls = pager.getByRole("button", { name: /^Desktop \d+ of \d+:/ });
  const [pagerBox, dockBox, firstBox, secondBox] = await Promise.all([
    pager.boundingBox(),
    dock.boundingBox(),
    desktopControls.nth(0).boundingBox(),
    desktopControls.nth(1).boundingBox(),
  ]);
  if (!pagerBox || !dockBox || !firstBox || !secondBox) {
    throw new Error("desktop pager and bottom chrome must expose measurable bounds");
  }
  const firstCenterY = firstBox.y + firstBox.height / 2;
  const secondCenterY = secondBox.y + secondBox.height / 2;
  const dockCenterY = dockBox.y + dockBox.height / 2;
  const alignmentTolerance = Math.max(2, dockBox.height * 0.1);

  expect(pagerBox.x).toBeLessThan(dockBox.x);
  expect(firstBox.x + firstBox.width).toBeLessThanOrEqual(secondBox.x);
  expect(Math.abs(firstCenterY - secondCenterY)).toBeLessThanOrEqual(alignmentTolerance);
  expect(Math.abs(firstCenterY - dockCenterY)).toBeLessThanOrEqual(alignmentTolerance);
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

async function createNamedSession(runtime: BrowserRuntime, workspaceId: string, name: string) {
  return await createNamedSessionForAgent(runtime, workspaceId, name, browserLifecycleAgent);
}

async function createNamedSessionForAgent(
  runtime: BrowserRuntime,
  workspaceId: string,
  name: string,
  agentName: string
) {
  const payload = await runtime.requestJSON<{
    session: { id: string; agent_name: string };
  }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, name, workspace: workspaceId }),
  });
  return payload.session;
}

async function createApprovalTask(
  runtime: BrowserRuntime,
  title: string
): Promise<{ id: string; title: string }> {
  const payload = await runtime.requestJSON<{ task: { id: string; title: string } }>("/api/tasks", {
    method: "POST",
    body: JSON.stringify({
      approval_policy: "manual",
      description: "OS shell attention E2E approval fixture.",
      owner: { kind: "human", ref: "os-shell-operator" },
      priority: "high",
      scope: "global",
      title,
    }),
  });
  return payload.task;
}

async function createTask(
  runtime: BrowserRuntime,
  title: string
): Promise<{ id: string; title: string }> {
  const payload = await runtime.requestJSON<{ task: { id: string; title: string } }>("/api/tasks", {
    method: "POST",
    body: JSON.stringify({
      description: "OS shell window-controller E2E fixture.",
      owner: { kind: "human", ref: "os-shell-operator" },
      priority: "medium",
      scope: "global",
      title,
    }),
  });
  return payload.task;
}

async function createTaskRun(runtime: BrowserRuntime, taskId: string): Promise<{ id: string }> {
  const payload = await runtime.requestJSON<{ run: { id: string } }>(
    `/api/tasks/${encodeURIComponent(taskId)}/runs`,
    {
      method: "POST",
      body: JSON.stringify({ idempotency_key: `os-shell-deck-run-${taskId}` }),
    }
  );
  return payload.run;
}

async function taskApprovalState(runtime: BrowserRuntime, taskId: string): Promise<string> {
  const payload = await runtime.requestJSON<{
    task: {
      summary?: { approval_state?: string | null };
      task?: { approval_state?: string | null };
    };
  }>(`/api/tasks/${encodeURIComponent(taskId)}`);
  return payload.task.summary?.approval_state ?? payload.task.task?.approval_state ?? "";
}

async function approveTaskFromCLI(runtime: BrowserRuntime, taskId: string): Promise<void> {
  if (!runtime.paths) throw new Error("E2E-015 requires launch-mode runtime paths");
  await execFileAsync(
    runtime.paths.cliShim,
    ["task", "approve", taskId, "--idempotency-key", `os-shell-cli-${taskId}`, "-o", "json"],
    {
      env: {
        ...process.env,
        COMPOZY_HOME: runtime.paths.homeDir,
        HOME: runtime.paths.operatorHomeDir,
      },
      maxBuffer: 10 * 1024 * 1024,
    }
  );
}

async function sessionHistoryContains(
  runtime: BrowserRuntime,
  workspaceId: string,
  sessionId: string,
  expected: string
): Promise<boolean> {
  const history = await runtime.requestJSON<unknown>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(sessionId)}/history`
  );
  return JSON.stringify(history).includes(expected);
}

async function addSecondWorkspace(
  runtime: BrowserRuntime,
  firstWorkspaceId: string
): Promise<WorkspacePayload> {
  return await addWorkspace(runtime, firstWorkspaceId, "second");
}

async function addWorkspace(
  runtime: BrowserRuntime,
  excludedWorkspaceId: string,
  suffix: string
): Promise<WorkspacePayload> {
  if (!runtime.paths) throw new Error("workspace switch E2E requires launch-mode runtime paths");
  const rootDir = `${runtime.paths.homeDir}-os-shell-${suffix}-workspace`;
  const { mkdir } = await import("node:fs/promises");
  await mkdir(rootDir, { recursive: true });
  const workspace = await runtime.resolveWorkspace(rootDir);
  if (workspace.id === excludedWorkspaceId) {
    throw new Error("cross-workspace E2E setup must resolve a distinct workspace");
  }
  return workspace;
}

function rightHalfTileX(
  layerWidth: number,
  gaps: WindowManagerSettingsResponse["config"]["gaps"]
): number {
  const layoutWidth = Math.max(0, layerWidth - gaps.left - gaps.right);
  return gaps.left + Math.round(layoutWidth / 2) + Math.ceil(gaps.inner / 2);
}

type MenubarMenu = "CompozyOS" | "Session" | "Go" | "Window" | "Help";

/** Menubar triggers are `role="menuitem"` inside the shell's single menubar. */
async function openMenu(page: Page, name: MenubarMenu): Promise<void> {
  await page.getByRole("menuitem", { name, exact: true }).click();
  const menuTestID = name === "CompozyOS" ? "os-menu-compozy" : `os-menu-${name.toLowerCase()}`;
  await expect(page.getByTestId(menuTestID)).toBeVisible();
}

/**
 * E2E-005: the command switcher, the menubar menu, and the overview render one
 * nested tree from one query; keyboard-only traversal reaches nested entries;
 * and selecting a worktree scopes session and task reads server-side.
 */
test("operator sees one nested worktree tree across all three workspace-listing surfaces", async ({
  appPage,
  runtime,
}) => {
  const repo = await createWorktreeRepo();
  try {
    await completeOnboardingIfPrompted(appPage);
    const workspace = await runtime.resolveWorkspace(repo.rootDir);
    await runtime.requestJSON(`/api/workspaces/${workspace.id}/worktrees`, {
      body: JSON.stringify({ name: "payments-retry" }),
      method: "POST",
    });
    await expect
      .poll(
        async () => {
          const listing = await runtime.requestJSON<{
            worktrees: Array<{ id: string; name: string; state: string }>;
          }>(`/api/workspaces/${workspace.id}/worktrees`);
          return listing.worktrees[0]?.state;
        },
        { timeout: 30_000 }
      )
      .toBe("ready");
    const worktree = (
      await runtime.requestJSON<{
        worktrees: Array<{ id: string; name: string; state: string }>;
      }>(`/api/workspaces/${workspace.id}/worktrees`)
    ).worktrees.find(entry => entry.name === "payments-retry");
    if (!worktree) throw new Error("ready worktree is missing from the catalog");
    await appPage.reload({ waitUntil: "domcontentloaded" });

    // Observe the scoped reads the UI issues, rather than trusting the chip.
    const scopedRequests: string[] = [];
    const parentRequests: string[] = [];
    appPage.on("request", request => {
      const url = request.url();
      if (!url.includes("/api/sessions") && !url.includes("/api/tasks")) return;
      const worktreeID = new URL(url).searchParams.get("worktree");
      if (worktreeID === worktree.id) scopedRequests.push(url);
      if (worktreeID === null) parentRequests.push(url);
    });

    // Surface 1 — the menubar workspace menu. Keyboard-only traversal opens
    // the side submenu and selects the nested entry.
    await appPage.locator('[data-slot="os-menubar-workspace"]').click();
    await appPage.getByTestId(`os-workspace-option-${workspace.id}`).focus();
    await appPage.keyboard.press("ArrowRight");
    const menuRow = appPage.getByTestId(/^os-worktree-option-/).first();
    await expect(menuRow).toBeVisible();
    await expect(menuRow).toContainText("payments-retry");

    await menuRow.focus();
    await appPage.keyboard.press("Enter");

    // The chip states where new work will run: `workspace / worktree`.
    await expect(appPage.locator('[data-slot="os-menubar-worktree"]')).toHaveText("payments-retry");

    // A fresh window inherits the shell selection and issues scoped reads.
    await openDockApp(appPage, "Tasks", "tasks");
    await expect(appPage.locator('[data-slot="os-menubar-worktree"]')).toHaveText("payments-retry");

    // Surface 2 — the real command palette switcher renders and selects the
    // same ready row; this is not another read of the menubar menu.
    await appPage.keyboard.press("ControlOrMeta+K");
    const palette = appPage.getByTestId("os-command-palette");
    await expect(palette).toBeVisible();
    const paletteRow = palette
      .getByRole("group", { name: "Worktrees" })
      .getByRole("option", { name: /payments-retry/ });
    await expect(paletteRow).toContainText("payments-retry");
    await paletteRow.click();
    await expect(palette).toHaveCount(0);

    // Surface 3 — the overview's vertical worktree menu, always visible for
    // the focused git-backed workspace: same content and same order.
    await appPage.locator('[data-slot="os-menubar-workspace"]').click();
    await appPage.getByRole("menuitem", { name: /^Workspace picker/ }).click();
    await expect(appPage.getByTestId("os-workspaces-overview")).toBeVisible();
    await expect(appPage.getByTestId("os-workspaces-worktree-menu")).toBeVisible();
    const overviewRows = await appPage
      .getByTestId(new RegExp(`^os-workspaces-worktree-row-`))
      .allTextContents();
    expect(overviewRows.join(" ")).toContain("payments-retry");
    // Both surfaces project the same tree, so their row order matches.
    expect(overviewRows[0]).toContain("payments-retry");
    await appPage.keyboard.press("Escape");

    // Selection scopes the reads themselves, server-side, for sessions and tasks.
    await expect
      .poll(() => scopedRequests.some(url => url.includes("/api/sessions")), { timeout: 15_000 })
      .toBe(true);
    await expect
      .poll(() => scopedRequests.some(url => url.includes("/api/tasks")), { timeout: 15_000 })
      .toBe(true);

    // Selecting the parent clears the worktree scope and returns both views to
    // the whole workspace instead of keeping a hidden client-side filter.
    await appPage.locator('[data-slot="os-menubar-workspace"]').click();
    await appPage.getByTestId(`os-workspace-option-${workspace.id}`).click();
    await expect(appPage.locator('[data-slot="os-menubar-worktree"]')).toHaveCount(0);
    await expect
      .poll(() => parentRequests.some(url => url.includes("/api/sessions")), { timeout: 15_000 })
      .toBe(true);
    await expect
      .poll(() => parentRequests.some(url => url.includes("/api/tasks")), { timeout: 15_000 })
      .toBe(true);
  } finally {
    await repo.cleanup();
  }
});

/**
 * E2E-021 — the menubar indicator's whole lifecycle in a plain browser: absent by
 * default, present only while an update is genuinely offered, absent again the
 * moment an operation takes the channel, and it lands the operator on the Updates
 * section (US-029 AC-2, EC-4). The update projection describes the host install,
 * which the harness has no real feed for, so each shape is served at the API
 * boundary; every assertion is on what the SPA renders from it.
 */
test("E2E-021: the menubar update indicator appears only while an update is offered and opens the Updates section", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(sessionLifecycleSelectors(appPage));

  let updatePayload: unknown = settingsUpdateStatusFixture;
  await appPage.route("**/api/settings/update", async route => {
    await route.fulfill({ json: updatePayload });
  });

  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("os-desktop")).toBeVisible({ timeout: 20_000 });

  const indicator = appPage.getByTestId("os-menubar-update");

  // Up to date: absent from the DOM, not hidden with CSS.
  await expect(indicator).toHaveCount(0);

  // Offered: the indicator appears, with no count and no dropdown.
  updatePayload = settingsUpdateBothAvailableFixture;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(indicator).toBeVisible({ timeout: 20_000 });
  await expect(indicator).not.toHaveAttribute("aria-haspopup", "true");

  // Applying: progress belongs to Settings, so the menubar goes quiet again.
  updatePayload = settingsUpdateApplyingFixture;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(indicator).toHaveCount(0, { timeout: 20_000 });

  // Failed: still no menubar error surface.
  updatePayload = settingsUpdateRolledBackFixture;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(indicator).toHaveCount(0, { timeout: 20_000 });

  // Offered again: activation lands on the Updates section.
  updatePayload = settingsUpdateBothAvailableFixture;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(indicator).toBeVisible({ timeout: 20_000 });
  await indicator.click();

  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/general");
  await expect(settingsOperatorSelectors(settingsWin).general.updates).toBeVisible({
    timeout: 20_000,
  });
});

/**
 * E2E-022 — the same journey with no pointer at all: the indicator is reachable by
 * keyboard, activates on Enter, and the apply affordance is reachable and
 * activatable in the Settings tab order (US-029 AC-4).
 */
test("E2E-022: a keyboard-only operator reaches the indicator, opens Updates, and applies without a pointer", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(sessionLifecycleSelectors(appPage));

  const applyTargets: string[] = [];
  await appPage.route("**/api/settings/update", async route => {
    await route.fulfill({ json: settingsUpdateBothAvailableFixture });
  });
  await appPage.route("**/api/settings/update/apply", async route => {
    const body = route.request().postDataJSON() as { targets: string[] };
    applyTargets.push(...body.targets);
    await route.fulfill({
      json: {
        targets: body.targets,
        status: "accepted",
        operation_id: "op-e2e-keyboard",
        message: "Started the requested updates.",
        holder: null,
      },
    });
  });

  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("os-desktop")).toBeVisible({ timeout: 20_000 });

  const indicator = appPage.getByTestId("os-menubar-update");
  await expect(indicator).toBeVisible({ timeout: 20_000 });

  // Focus without a pointer, then activate with the keyboard.
  await indicator.focus();
  await expect(indicator).toBeFocused();
  await appPage.keyboard.press("Enter");

  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  const settingsUI = settingsOperatorSelectors(settingsWin);
  await expect(settingsUI.general.updates).toBeVisible({ timeout: 20_000 });

  // The apply affordance is a real button in the tab order, activatable by keyboard.
  const apply = settingsUI.general.updateApply();
  await expect(apply).toBeVisible();
  await apply.focus();
  await expect(apply).toBeFocused();
  await appPage.keyboard.press("Enter");
  await expect.poll(() => applyTargets).toEqual(["runtime", "app"]);
});

test("E2E-015: destination mode offers only eligible targets and exits clean when none are", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);

  // ⌘T opens an empty tab and hands the palette its destination intent.
  await appPage.keyboard.press("ControlOrMeta+KeyT");
  const destination = destinationPalette(appPage);
  await expect(destination).toBeVisible();
  await expect(destination.getByPlaceholder("Open in this tab…")).toBeVisible();

  // Only navigable targets are offered; shell operations are absent, not disabled.
  await destination.getByPlaceholder("Open in this tab…").fill("Tasks");
  await expect(paletteRow(destination, "app.open.tasks")).toBeVisible();
  await expect(paletteRow(destination, "window.close")).toHaveCount(0);

  await paletteRow(destination, "app.open.tasks").click();
  await expect(appWindow(appPage, "tasks")).toBeVisible();
  await expect(destination).toHaveCount(0);

  // Zero-eligible variant: an impossible query says so and Esc leaves cleanly.
  await appPage.keyboard.press("ControlOrMeta+KeyT");
  const reopened = destinationPalette(appPage);
  await expect(reopened).toBeVisible();
  await reopened.getByPlaceholder("Open in this tab…").fill("zzzzzzzz");
  await expect(paletteEmptyState(reopened)).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(reopened).toHaveCount(0);
});

test("E2E-018: a cold daemon explains and refuses primary actions while exempt commands keep working", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const palette = await openCommandPalette(appPage);
  await expect(paletteRow(palette, "window.close")).toBeEnabled();

  // Pinning through the palette invalidates its catalog after the public
  // mutation succeeds. Aborting that read proves the same query boundary used
  // when the daemon disappears, without substituting browser connectivity for
  // daemon reachability.
  await appPage.route("**/api/cmd-palette/commands*", route => route.abort());
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("Close window");
  await appPage.keyboard.press("ControlOrMeta+KeyK");
  const pin = appPage.getByTestId("os-palette-action-meta.pin");
  await expect(pin).toHaveText("Pin");
  await pin.click();

  // The row remains keyboard-selectable because its meta-actions still work,
  // but the unavailable primary action is described and refused at the shared
  // dispatch seam without closing the palette or changing the window topology.
  const closeRow = paletteRow(palette, "window.close");
  await expect(paletteRowReason(palette, "window.close")).toHaveText("runtime unavailable");
  await expect(closeRow).toHaveAccessibleDescription("runtime unavailable");
  await palette.getByPlaceholder("Search apps, sessions, and actions…").press("Home");
  await expect(closeRow).toHaveAttribute("data-selected", "true");
  await appPage.keyboard.press("Enter");
  await expect(appPage.locator("[data-sonner-toast]:last-of-type")).toContainText(
    "Close window — runtime unavailable"
  );
  await expect(palette).toBeVisible();

  await appPage.keyboard.press("ControlOrMeta+KeyK");
  const unavailablePanel = appPage.getByTestId("os-palette-action-panel");
  await expect(unavailablePanel.getByTestId("os-palette-action-primary.run")).toHaveCount(0);
  await expect(unavailablePanel.getByTestId("os-palette-action-meta.pin")).toBeVisible();
  await expect(unavailablePanel.getByTestId("os-palette-action-reason")).toHaveText(
    "runtime unavailable"
  );
  await appPage.keyboard.press("Escape");
  // Availability-exempt commands survive the reconnect (US-001.EC-1).
  await palette.getByPlaceholder("Search apps, sessions, and actions…").fill("");
  const cheatsheet = paletteRow(palette, "shortcuts.cheatsheet");
  await expect(cheatsheet).toBeEnabled();
  await cheatsheet.click();
  await expect(appPage.getByTestId("os-shortcuts-dialog")).toBeVisible();
  await appPage.keyboard.press("Escape");

  await appPage.unroute("**/api/cmd-palette/commands*");
  await runtime.requestJSON(
    `/api/cmd-palette/pins/${encodeURIComponent("window.close")}?workspace=${encodeURIComponent(workspace.id)}`,
    { method: "DELETE" }
  );
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await prepareShell(appPage, runtime);
  // Recovery replaces the transport failure with the live contextual reason;
  // this fresh shell has no focused window yet.
  const reopened = await openCommandPalette(appPage);
  await expect(paletteRowReason(reopened, "window.close")).toHaveText("requires a focused window");
  await expect(paletteRow(reopened, "window.close")).toHaveAccessibleDescription(
    "requires a focused window"
  );
});

test("E2E-019: menubar items project the same id, label, chord and reason as the palette row", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const palette = await openCommandPalette(appPage);
  const paletteLabel = await paletteRow(palette, "window.close").innerText();
  await appPage.keyboard.press("Escape");

  await appPage.getByRole("menuitem", { name: "Window", exact: true }).click();
  const menuItem = menubarCommandItem(appPage, "window.close");
  await expect(menuItem).toBeVisible();
  // Same registry row, same words: BR-17 curates order only.
  expect(paletteLabel).toContain(await menuItem.innerText());
  await appPage.keyboard.press("Escape");
});
