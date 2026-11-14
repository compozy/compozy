import { execFile } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Browser, Locator, Page } from "@playwright/test";

import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import {
  appWindow,
  closeAppWindow,
  destinationPalette,
  menubarCommandItem,
  openAppWindow as openDockApp,
  openCommandPalette,
  paletteEmptyState,
  paletteRow,
  paletteRowReason,
  sessionWindow,
  switchWorkspace,
  windowID,
  windowPane,
} from "../fixtures/os-navigation";
import {
  restartRetiredKnowledgeWindowRuntime,
  seedLegacyWindowLayout,
  seedRetiredKnowledgeWindow,
} from "../fixtures/retired-knowledge-window";
import { createWorktreeRepo } from "../fixtures/worktree-repo";
import {
  osShellSelectors,
  sessionLifecycleSelectors,
  sessionWorkspaceSwitchSelectors,
  tasksOperatorSelectors,
} from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import {
  settingsUpdateApplyingFixture,
  settingsUpdateBothAvailableFixture,
} from "@/systems/settings/mocks/settings-update-fixture";
import { humanizeAgentSlug } from "@/systems/agent/lib/agent-identity";

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

// Every journey runs on the daemon's shipped `tab` new-window policy.
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
 * E2E-003 → E2E-041  ·  E2E-004 → E2E-039  ·  E2E-005 → E2E-032
 * E2E-006 → E2E-033  ·  E2E-007 → E2E-040  ·  E2E-008 → E2E-034
 * E2E-009 → E2E-042  ·  E2E-011 → E2E-043  ·  E2E-013 → E2E-039
 * E2E-014 → E2E-038  ·  E2E-015 → E2E-044  ·  E2E-016 → Tabs-split E2E-014
 * E2E-018 → E2E-045
 *
 * Herdr parity contract coverage (`.compozy/tasks/herdr-parity/_tests.md`) is
 * prefixed `Herdr ` because this suite's physical numbering already claims the
 * same digits: Herdr E2E-016 (⌘E → Sessions view → land) and Herdr E2E-019
 * (nested view push, pop, and reopen-at-root).
 *
 * Memory removal contract coverage (`.compozy/tasks/memory-removal/_tests.md`) is
 * prefixed `Memory removal ` for the same reason.
 *
 * Window tabs and splits contract coverage
 * (`.compozy/tasks/window-tabs-split/_tests.md`) is prefixed `Tabs-split `.
 * Journeys whose v5 behaviour that spec retired (floating geometry, zoom,
 * minimize, decks, snap and frame drags) were removed; drag, drop, seam and
 * unsplit journeys belong to its later slices.
 */

interface WindowRoute {
  pathname: string;
  search: Record<string, unknown>;
}

interface WindowManagerWindow {
  id: string;
  app: string;
  instance_key?: string;
  route: WindowRoute;
  nav_stack: WindowRoute[];
  desktop_id: string;
  tab_id: string;
}

interface WindowManagerLayoutNode {
  id: string;
  kind: "leaf" | "split";
  window_id?: string;
  axis?: "horizontal" | "vertical";
  children?: WindowManagerLayoutNode[];
  weights?: number[];
}

interface WindowManagerTab {
  id: string;
  pinned: boolean;
  root: WindowManagerLayoutNode;
}

interface WindowManagerDesktop {
  id: string;
  name: string;
  order: number;
  tabs: WindowManagerTab[];
  active_tab_id: string;
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
  active_tab?: Record<string, string>;
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

interface SettingsRestartAction {
  operation_id: string;
  status: string;
  status_url: string;
}

interface SettingsRestartStatus {
  status: string;
}

test("E2E-001 / E2E-014: fresh boot shows one New Tab window and a CLI move splits two tabs live", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const shell = osShellSelectors(appPage);

  await test.step("E2E-001: fresh boot renders the default desktop's single New Tab window", async () => {
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
    await expect(
      appPage.getByRole("button", { name: "Desktop 1 of 1: Desktop 1" })
    ).toHaveAttribute("aria-current", "page");

    const snapshot = await windowManagerSnapshot(runtime, workspace.id);
    expect(snapshot.version).toBe(6);
    expect(snapshot.revision).toBe(0);
    expect(snapshot.desktops.map(desktop => [desktop.id, desktop.name])).toEqual([
      ["desktop-default", "Desktop 1"],
    ]);
    const windows = Object.values(snapshot.windows);
    expect(windows.map(window => window.app)).toEqual(["new-tab"]);
    const newTab = windows[0];
    if (!newTab) throw new Error("a fresh desktop must hold one New Tab window");
    expect(desktopTabIds(snapshot, "desktop-default")).toEqual([newTab.tab_id]);
    await expect(shell.tabs).toHaveCount(1);
    await expect(shell.tab(newTab.tab_id)).toHaveAttribute("aria-selected", "true");
    await expect(shell.window(newTab.id).getByTestId("new-tab-empty")).toBeVisible();

    const legacy = await fetch(
      runtime.url(`/api/workspaces/${encodeURIComponent(workspace.id)}/desktop-state`)
    );
    expect(legacy.status).toBe(404);
  });

  await test.step("E2E-014: a CLI window move splits two tabs into one, rendered live", async () => {
    const tasks = await openDockApp(appPage, "Tasks", "tasks");
    const tasksID = await windowID(tasks);
    // A window that joins another tab's split keeps its pane: the same element, never a remount
    // of its app (a peer converging on a split would otherwise pay a full app startup). The probe
    // is set while the pane is shown; the locator only matches a shown pane.
    await tasks.evaluate(element => Reflect.set(element, "__mountProbe", true));
    const agents = await openDockApp(appPage, "Agents", "agents");
    const agentsID = await windowID(agents);
    await expect(tasks).toBeHidden();

    await moveWindowFromCLI(runtime, workspace.id, tasksID, agentsID, "right");

    await expect
      .poll(async () => {
        const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), agentsID);
        return tab?.root.kind === "split" ? nodeWindowIds(tab.root) : null;
      })
      .toEqual([agentsID, tasksID]);
    await expect(tasks).toBeVisible();
    await expect(agents).toBeVisible();
    const [agentsBox, tasksBox] = await Promise.all([agents.boundingBox(), tasks.boundingBox()]);
    if (!agentsBox || !tasksBox) throw new Error("split members must have visible bounds");
    expect(agentsBox.x + agentsBox.width).toBeLessThanOrEqual(tasksBox.x + 1);
    expect(await tasks.evaluate(element => Reflect.get(element, "__mountProbe"))).toBe(true);
  });
});

test("E2E-136 / ENG-136: Session menu owns the catalog while the dock creates or focuses sessions", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await expect(appPage.getByRole("button", { name: /^Desktop 1 of 1:/ })).toBeEnabled();
  const sessionsDockButton = appPage.getByRole("button", { name: "Sessions", exact: true });

  await openSessionsCatalog(appPage);
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await expect(sessionsModal).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(sessionsModal).toHaveCount(0);

  await sessionsDockButton.click();
  const createDialog = appPage.getByTestId("session-create-dialog");
  await expect(createDialog).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(createDialog).toHaveCount(0);

  const session = await createNamedSession(runtime, workspace.id, "Dock focus target");
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{ sessions: { id: string }[] }>(
        `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
      );
      return payload.sessions.some(row => row.id === session.id);
    })
    .toBe(true);
  await sessionsDockButton.click();
  const seededWindow = sessionWindow(appPage, session.id);
  await expect(seededWindow).toBeVisible();
  await expect(createDialog).toHaveCount(0);

  // Another tab takes the view; the dock brings the session's tab back and focuses it.
  await openDockApp(appPage, "Tasks", "tasks");
  await expect(seededWindow).toBeHidden();
  await sessionsDockButton.click();
  await expect(seededWindow).toBeVisible();
  await expect(windowPane(seededWindow)).toHaveAttribute("data-focused", "");
});

test("E2E-825: a new session lands in the existing Session window instead of opening another", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const seeded = await createNamedSession(runtime, workspace.id, "Reused window seed");
  await appPage.goto(
    runtime.url(
      `/agents/${encodeURIComponent(browserLifecycleAgent)}/sessions/${encodeURIComponent(seeded.id)}`
    ),
    { waitUntil: "domcontentloaded" }
  );
  const seededWindow = sessionWindow(appPage, seeded.id);
  await expect(seededWindow).toBeVisible();
  const hostID = await windowID(seededWindow);
  const sessionWindows = appPage.locator('[data-slot="os-window-surface"][data-app="session"]');
  const hostWindow = appPage.getByTestId(`os-window-${hostID}`);

  // Focused Session window: the new session replaces the conversation in place, focus stays.
  const first = await createSessionFromMenubar(appPage);
  await expect(sessionWindow(appPage, first)).toBeVisible();
  expect(await windowID(sessionWindow(appPage, first))).toBe(hostID);
  await expect(sessionWindow(appPage, seeded.id)).toHaveCount(0);
  await expect(sessionWindows).toHaveCount(1);
  await expect(windowPane(hostWindow)).toHaveAttribute("data-focused", "");

  // Session window in a background tab: its tab comes forward with the new session.
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  await expect(windowPane(tasks)).toHaveAttribute("data-focused", "");
  const second = await createSessionFromMenubar(appPage);
  await expect(sessionWindow(appPage, second)).toBeVisible();
  expect(await windowID(sessionWindow(appPage, second))).toBe(hostID);
  await expect(sessionWindows).toHaveCount(1);
  await expect(windowPane(hostWindow)).toHaveAttribute("data-focused", "");

  await expect
    .poll(
      async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[hostID]?.instance_key
    )
    .toBe(second);
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

test("Issue 1655: dock Files writes one history entry and one Back returns to the window it left", async ({
  appPage,
  runtime,
}) => {
  await prepareProjectShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  await expect(windowPane(tasks)).toHaveAttribute("data-focused", "");
  await expect(appPage).toHaveURL(/\/tasks$/);

  // Files is not open yet, then it is: both open-or-focus paths write exactly one entry.
  for (const pass of ["opens", "focuses"]) {
    const files = await openDockApp(appPage, "Files", "files");
    await expect(windowPane(files), pass).toHaveAttribute("data-focused", "");
    await expect(appPage).toHaveURL(/\/files(?:\/|$|\?)/);

    await appPage.goBack({ waitUntil: "domcontentloaded" });
    await expect(appPage, pass).toHaveURL(/\/tasks$/);
    await expect(windowPane(appWindow(appPage, "tasks")), pass).toHaveAttribute("data-focused", "");
  }
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
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await expect(appPage.getByRole("button", { name: /^Desktop 1 of 1:/ })).toBeEnabled();
  const primary = await createNamedSession(runtime, workspace.id, "Primary observer");
  const secondary = await createNamedSession(runtime, workspace.id, "Secondary responder");

  await openSessionsCatalog(appPage);
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await sessionsModal.getByTestId(`os-sessions-modal-session-${primary.id}`).first().click();
  await expect(sessionsModal).toHaveCount(0);
  await expect(sessionWindow(appPage, primary.id)).toBeVisible();
  const secondaryWindowID = await openSessionWindowInAuthority(runtime, workspace.id, secondary);

  const primaryWindow = sessionWindow(appPage, primary.id);
  const secondaryWindow = sessionWindow(appPage, secondary.id);
  await expect(primaryWindow).toBeVisible();
  const primaryWindowID = await windowID(primaryWindow);
  // The secondary window joins the primary's tab as the right half of a split.
  await splitWindowBeside(runtime, workspace.id, secondaryWindowID, primaryWindowID, "right");
  await expect(secondaryWindow).toBeVisible();

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
    // A reader gesture hands the transcript over; a bare scroll alone keeps following.
    viewport.dispatchEvent(new WheelEvent("wheel", { deltaY: -120 }));
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
  await expect(windowPane(secondaryWindow)).toHaveAttribute("data-focused", "");
  await expect(windowPane(primaryWindow)).not.toHaveAttribute("data-focused", "");
  await expect(primaryTranscript).toContainText(
    "Primary stream event arrived while the window was visible and unfocused."
  );
  await expect
    .poll(() => primaryTranscript.evaluate(element => element.scrollTop))
    .toBe(parkedScrollTop);

  // Another tab takes the view: the split stays mounted but hidden.
  await openDockApp(appPage, "Tasks", "tasks");
  await expect(primaryWindow).toHaveCount(1);
  await expect(primaryWindow).toBeHidden();
  await expect
    .poll(() => sessionHistoryContains(runtime, workspace.id, primary.id, "arrived while"))
    .toBe(true);

  await openSessionsCatalog(appPage);
  const restoredModal = appPage.getByTestId("os-sessions-modal");
  await restoredModal.getByTestId(`os-sessions-modal-session-${primary.id}`).first().click();
  const restoredPrimary = sessionWindow(appPage, primary.id);
  await expect(restoredPrimary).toBeVisible();
  await expect(restoredPrimary.getByTestId("chat-view")).toContainText(
    "Primary stream event arrived while the window was minimized."
  );
});

test("Issue 843: each session window owns its sessions rail across ⌘B, reload, and close", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  await expect(appPage.getByRole("button", { name: /^Desktop 1 of 1:/ })).toBeEnabled();
  const left = await createNamedSession(runtime, workspace.id, "Left rail owner");
  const right = await createNamedSession(runtime, workspace.id, "Right rail owner");
  const leftWindowID = await openSessionWindowInAuthority(runtime, workspace.id, left);
  const rightWindowID = await openSessionWindowInAuthority(runtime, workspace.id, right);
  await splitWindowBeside(runtime, workspace.id, rightWindowID, leftWindowID, "right");
  await showTab(appPage, runtime, workspace.id, leftWindowID);
  const leftWindow = sessionWindow(appPage, left.id);
  const rightWindow = sessionWindow(appPage, right.id);
  const leftRail = leftWindow.getByTestId("session-sidebar");
  const rightRail = rightWindow.getByTestId("session-sidebar");
  await expect(leftRail).toBeVisible();
  await expect(rightRail).toBeVisible();

  // ⌘B toggles the focused window's rail only.
  await focusWindow(appPage, leftWindow);
  await expect(windowPane(leftWindow)).toHaveAttribute("data-focused", "");
  await appPage.keyboard.press("ControlOrMeta+B");
  await expect(leftRail).toBeHidden();
  await expect(rightRail).toBeVisible();

  // The in-window toggle is just as local.
  const rightToggle = rightWindow.getByTestId("session-sidebar-toggle");
  await rightToggle.click();
  await expect(rightRail).toBeHidden();
  await rightToggle.click();
  await expect(rightRail).toBeVisible();
  await expect(leftRail).toBeHidden();

  // A reload restores each window's own rail.
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(rightRail).toBeVisible();
  await expect(leftWindow).toBeVisible();
  await expect(leftRail).toBeHidden();

  // Closing a window drops its persisted rail state; the survivor keeps its own.
  await closeAppWindow(appPage, leftWindow);
  await rightToggle.click();
  await expect(rightRail).toBeHidden();
  await expect
    .poll(() => persistedSessionRailScopes(appPage))
    .toEqual(expect.arrayContaining([rightWindowID]));
  expect(await persistedSessionRailScopes(appPage)).not.toContain(leftWindowID);
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
  await search.fill("terminal");
  await paletteRow(palette, "app.open.terminal").click();
  await expect(appWindow(appPage, "terminal")).toBeVisible();
  await expect(palette).toHaveCount(0);
  await expect(appPage.locator('[data-slot="dialog-overlay"]')).toHaveCount(0);

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
  const runtimeTrigger = appPage
    .getByTestId("agent-setup-model")
    .getByTestId("agent-setup-model-select");
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
  await expect(sidebar).toBeVisible();
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

test("Command palette E2E-009 / ENG-131: Tasks filters clear and a loop row opens its detail route", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const loopName = "palette-direct-loop";
  // The first root search caches every domain catalog. Seed the loop before
  // opening the palette because runtime API writes do not invalidate that cache.
  await createPaletteLoop(runtime, workspace.id, loopName);

  await test.step("Command palette E2E-009: Tasks reports truthful zero counts and clears one filter", async () => {
    const task = await createTask(runtime, "Palette filter target", workspace.id);

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
    await appPage.keyboard.press("Escape");
    await expect(palette).toHaveCount(0);
  });

  await test.step("Command palette ENG-131: a loop row opens its detail route", async () => {
    const palette = await openCommandPalette(appPage);
    const search = palette.getByPlaceholder("Search apps, sessions, and actions…");
    await search.fill(loopName);
    const loopRow = palette.getByTestId(`os-palette-domain-row-loop:${workspace.id}:${loopName}`);
    await expect(loopRow).toBeVisible();
    await loopRow.click();

    const loops = appWindow(appPage, "loops");
    await expect(loops).toBeVisible();
    await expect
      .poll(async () => {
        const snapshot = await windowManagerSnapshot(runtime, workspace.id);
        return Object.values(snapshot.windows).find(window => window.app === "loops")?.route
          .pathname;
      })
      .toBe(`/loops/${encodeURIComponent(loopName)}`);
    await expect(loops.getByTestId("loop-detail")).toBeVisible();
  });
});

test("E2E-010 and E2E-018: peers converge topology while the active tab stays client-local", async ({
  appPage,
  browser,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const second = await openPeerPage(browser, runtime);
  try {
    const firstWindow = await openDockApp(appPage, "Tasks", "tasks");
    const tasksID = await windowID(firstWindow);
    const secondWindow = appWindow(second, "tasks");
    // The peer's bar gains the tab without switching to it.
    await expect(osShellSelectors(second).tabOfWindow(tasksID)).toBeVisible();
    await expect(secondWindow).toBeHidden();
    const [firstClientId, secondClientId] = await Promise.all([
      browserClientId(appPage),
      browserClientId(second),
    ]);
    expect(firstClientId).not.toBe(secondClientId);

    const created = await executeWindowManagerCommand(runtime, workspace.id, {
      commandId: "desktop.create",
      payload: { desktop_id: "", name: "Peer target" },
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
    // Moving the Tasks tab converges for every client; which tab each one shows stays its own.
    await executeWindowManagerCommand(runtime, workspace.id, {
      commandId: "window.move",
      payload: {
        window_id: tasksID,
        destination_desktop_id: targetDesktop.id,
        placement: "tab",
        move_group: true,
      },
    });
    await expect
      .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
      .toMatchObject({ desktop_id: targetDesktop.id });
    const firstShell = osShellSelectors(appPage);
    const peerShell = osShellSelectors(second);
    await expect(firstShell.tabOfWindow(tasksID)).toBeVisible();
    await expect(secondWindow).toBeHidden();
    await second
      .getByRole("button", {
        name: new RegExp(`^Desktop \\d+ of 2: ${escapeRegExp(targetDesktop.name)}$`),
      })
      .click();
    await expect(peerShell.tabOfWindow(tasksID)).toBeVisible();

    await firstShell.tabOfWindow(tasksID).click();
    await expect(firstWindow).toBeVisible();
    await expect(firstShell.tabOfWindow(tasksID)).toHaveAttribute("aria-selected", "true");
    await expect(peerShell.tabOfWindow(tasksID)).not.toHaveAttribute("aria-selected", "true");
    await expect(secondWindow).toBeHidden();
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
    .filter({ hasText: /Reconnecting…|Can.t save window layout/ });
  await expect(degradedStatus).toBeVisible();
  // Commands travel over HTTP, so opening a tab still works while the stream is down.
  const tasks = await openDockApp(degradedPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  await expect(osShellSelectors(degradedPage).tabOfWindow(tasksID)).toBeVisible();

  stream.unblock();
  await expect(degradedStatus).toHaveCount(0);
  await degradedPage.reload({ waitUntil: "domcontentloaded" });
  await expect(appWindow(degradedPage, "tasks")).toBeVisible();
  await expect(osShellSelectors(degradedPage).tabOfWindow(tasksID)).toHaveAttribute(
    "aria-selected",
    "true"
  );
});

test("E2E-015: bell approval stays live and a CLI-resolved item reports truthful conflict", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasksUI = tasksOperatorSelectors(appPage);
  const first = await createApprovalTask(runtime, "Primary approval", workspace.id);
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

  const second = await createApprovalTask(runtime, "CLI race approval", workspace.id);
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
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await expect(appPage.getByRole("button", { name: /^Desktop 1 of 1:/ })).toBeEnabled();
  const session = await createNamedSession(runtime, workspace.id, "Scoped modal session");
  const task = await createTask(runtime, "Window-scoped deletion");

  await openSessionsCatalog(appPage);
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
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
  // The session joins the Tasks tab as the right half of a split.
  await splitWindowBeside(runtime, workspace.id, sessionWindowID, tasksWindowID, "right");
  await expect(tasksWindow).toBeVisible();
  await expect(sessionWin).toBeVisible();

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
    humanizeAgentSlug(browserLifecycleAgent)
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
  await appPage.getByRole("menuitem", { name: /^All desktops/ }).click();
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
  await expect(shortcuts.getByTestId("os-shortcut-row-window.close")).toContainText(/[⌘⌃]W/u);
  await expect(shortcuts.getByTestId("os-shortcut-row-workspace.picker")).toContainText(/[⌘⌃]⇧O/u);
  await expect(shortcuts.getByTestId("os-shortcut-row-window.tile.top")).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(shortcuts).toHaveCount(0);

  await openMenu(appPage, "CompozyOS");
  await appPage.getByRole("menuitem", { name: /^About CompozyOS/ }).click();
  const about = appPage.getByTestId("os-about-dialog");
  await expect(about).toBeVisible();
  await expect(about.getByTestId("os-about-row-version")).not.toBeEmpty();
  await about.getByRole("button", { name: "Technical details" }).click();
  await expect(about.getByTestId("os-about-row-pid")).not.toBeEmpty();
  await appPage.keyboard.press("Escape");
  await expect(about).toHaveCount(0);

  await appPage.getByRole("button", { name: "Settings" }).click();
  await expect(appWindow(appPage, "settings")).toBeVisible();
});

test("E2E-026: a CLI arrange builds one split tab whose weights render the same shares on every client", async ({
  appPage,
  browser,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const second = await openPeerPage(browser, runtime);
  try {
    const tasks = await openDockApp(appPage, "Tasks", "tasks");
    const tasksID = await windowID(tasks);
    const agents = await openDockApp(appPage, "Agents", "agents");
    const agentsID = await windowID(agents);

    await arrangeSplitFromCLI(runtime, workspace.id, "desktop-default", [tasksID, agentsID]);
    await expect
      .poll(async () => {
        const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
        return tab?.root.kind === "split" ? nodeWindowIds(tab.root) : null;
      })
      .toEqual([tasksID, agentsID]);
    const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
    if (!tab) throw new Error("the arrange must leave both windows in one tab");

    // Each client lays the same weights over its own width (1440 and 1280 wide).
    const share = async (page: Page) => {
      const shell = osShellSelectors(page);
      const [left, right] = await Promise.all([
        shell.window(tasksID).boundingBox(),
        shell.window(agentsID).boundingBox(),
      ]);
      if (!left || !right) throw new Error("both split members must have visible bounds");
      return left.width / (left.width + right.width);
    };
    for (const page of [appPage, second]) {
      await osShellSelectors(page).tab(tab.id).click();
      await expect(osShellSelectors(page).window(tasksID)).toBeVisible();
      await expect(osShellSelectors(page).window(agentsID)).toBeVisible();
      await expect.poll(() => share(page)).toBeCloseTo(0.5, 1);
    }

    await resizeSplitOf(runtime, workspace.id, tasksID, -0.2);
    for (const page of [appPage, second]) {
      await expect.poll(() => share(page)).toBeCloseTo(0.3, 1);
    }
    const [firstBox, peerBox] = await Promise.all([
      osShellSelectors(appPage).window(tasksID).boundingBox(),
      osShellSelectors(second).window(tasksID).boundingBox(),
    ]);
    if (!firstBox || !peerBox) throw new Error("split members must have visible bounds");
    expect(Math.round(firstBox.width)).not.toBe(Math.round(peerBox.width));
  } finally {
    await second.context().close();
  }
});

// Invariant: the sessions rail width is one persisted operator preference, set by
// pointer or keyboard within 200–480 px, reset by double-click, and never wider
// than half of a narrow session window.
test("Issue 836: the sessions rail resizes by drag and keyboard, persists, and resets", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const session = await createNamedSession(runtime, workspace.id, "Resizable rail target");
  const windowId = await openSessionWindowInAuthority(runtime, workspace.id, session);
  await showTab(appPage, runtime, workspace.id, windowId);
  const sessionWin = sessionWindow(appPage, session.id);
  await expect(sessionWin).toBeVisible();

  const rail = sessionWin.getByTestId("session-sidebar");
  const handle = sessionWin.getByRole("separator", { name: "Resize sessions sidebar" });
  const railWidth = async () => Math.round((await rail.boundingBox())?.width ?? 0);
  const dragHandleBy = async (deltaX: number) => {
    const box = await handle.boundingBox();
    if (!box) throw new Error("the rail resize handle must be visible");
    const start = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
    await appPage.mouse.move(start.x, start.y);
    await appPage.mouse.down();
    await appPage.mouse.move(start.x + deltaX, start.y, { steps: 8 });
    await appPage.mouse.up();
  };
  await expect.poll(railWidth).toBe(360);
  await browserArtifacts.captureScreenshot("rail-default", appPage);

  await dragHandleBy(96);
  await expect.poll(railWidth).toBeCloseTo(456, -1);
  await browserArtifacts.captureScreenshot("rail-dragged", appPage);
  await dragHandleBy(400);
  await expect.poll(railWidth).toBe(480);
  await browserArtifacts.captureScreenshot("rail-max", appPage);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWin).toBeVisible();
  await expect.poll(railWidth).toBe(480);

  await handle.focus();
  await appPage.keyboard.press("ArrowLeft");
  await expect.poll(railWidth).toBeLessThan(480);
  const stepped = await railWidth();
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWin).toBeVisible();
  await expect.poll(railWidth).toBe(stepped);

  // Pointer double-click at the handle, as an operator does: the transcript may cover the 1px line.
  const handleBox = await handle.boundingBox();
  if (!handleBox) throw new Error("the rail resize handle must be visible");
  await appPage.mouse.dblclick(
    handleBox.x + handleBox.width / 2,
    handleBox.y + handleBox.height / 2
  );
  await expect.poll(railWidth).toBe(360);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWin).toBeVisible();
  await expect.poll(railWidth).toBe(360);

  // Collapse and reopen keep the chosen width; the handle leaves with the rail.
  await dragHandleBy(-24);
  await expect.poll(railWidth).toBeCloseTo(336, -1);
  const chosen = await railWidth();
  const railToggle = sessionWin.getByTestId("session-sidebar-toggle");
  await railToggle.click();
  await expect(rail).toBeHidden();
  await expect(handle).toBeHidden();
  await railToggle.click();
  await expect.poll(railWidth).toBe(chosen);

  // A narrow window keeps at least half its width for the session pane: split
  // Tasks beside the session, then move the seam toward the session.
  const [tasksID] = await openFixtureWindows(runtime, workspace.id, ["tasks"]);
  if (!tasksID) throw new Error("the narrowing fixture must open a Tasks window");
  await splitWindowBeside(runtime, workspace.id, tasksID, windowId, "right");
  await resizeSplitOf(runtime, workspace.id, windowId, -0.2);
  await expect
    .poll(async () => {
      const windowWidth = (await sessionWin.boundingBox())?.width ?? 0;
      return (await railWidth()) <= Math.ceil(windowWidth / 2);
    })
    .toBe(true);
  await browserArtifacts.captureScreenshot("rail-narrow-window", appPage);

  // Narrower than two rail floors (288px): the rail's floor yields, so the
  // session pane still holds half the window instead of being squeezed.
  await resizeSplitOf(runtime, workspace.id, windowId, -0.1);
  const panes = sessionWin.locator("[data-panel]");
  await expect
    .poll(async () => {
      const windowWidth = (await sessionWin.boundingBox())?.width ?? 0;
      const paneWidth = (await panes.last().boundingBox())?.width ?? 0;
      return windowWidth < 400 && paneWidth >= Math.floor(windowWidth / 2) - 2;
    })
    .toBe(true);
});

test("E2E-009: the pager and overview keep desktop arrangements independent", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const tasksTab = (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.tab_id;
  if (!tasksTab) throw new Error("Tasks must sit in an authoritative tab");

  for (let index = 2; index <= 8; index += 1) {
    await executeWindowManagerCommand(runtime, workspace.id, {
      commandId: "desktop.create",
      payload: { desktop_id: `desktop-e2e-${index}`, name: `Desktop ${index}` },
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
  // A fresh desktop holds one New Tab window. Every desktop keeps its own New tab window
  // mounted behind `inert`, so scope to the desktop the pager just activated.
  const desktopTwo = activeDesktop(appPage, "desktop-e2e-2");
  await expect(desktopTwo.locator('[data-app="new-tab"]')).toHaveCount(1);
  await expect(desktopTwo.getByTestId("new-tab-empty")).toBeVisible();
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
  // Each card lists exactly the tabs on its desktop.
  const vaultTab = (await windowManagerSnapshot(runtime, workspace.id)).windows[vaultID]?.tab_id;
  if (!vaultTab) throw new Error("Vault must sit in an authoritative tab");
  for (const [desktop, present, absent] of [
    ["Desktop 1", tasksTab, vaultTab],
    ["Desktop 2", vaultTab, tasksTab],
  ] as const) {
    const card = overview.getByRole("listitem", { name: desktop, exact: true });
    await expect(card.locator(`[data-tab-id="${present}"]`)).toBeVisible();
    await expect(card.locator(`[data-tab-id="${absent}"]`)).toHaveCount(0);
  }
  await expect(overview).toBeVisible();
  await overview.getByRole("button", { name: "Switch to Desktop 1" }).click();

  const tasksBack = appWindow(appPage, "tasks");
  await expect(tasksBack).toBeVisible();
  await expect(vault).toBeHidden();
  await expect(activeDesktop(appPage, "desktop-default")).toHaveAttribute("data-active", "true");
  expect((await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]).toMatchObject({
    desktop_id: "desktop-default",
    tab_id: tasksTab,
  });
});

test("E2E-011: a compact viewport shows one split member at a time and keeps its weights", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const [agentsID] = await openFixtureWindows(runtime, workspace.id, ["agents"]);
  if (!agentsID) throw new Error("compact fixture must open a second window");
  await splitWindowBeside(runtime, workspace.id, agentsID, tasksID, "right");
  const shell = osShellSelectors(appPage);
  await expect(shell.window(tasksID)).toBeVisible();
  await expect(shell.window(agentsID)).toBeVisible();
  const before = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
  if (before?.root.kind !== "split") throw new Error("compact fixture must hold one split tab");
  const members = appPage
    .locator(`[data-testid="os-window-${tasksID}"], [data-testid="os-window-${agentsID}"]`)
    .filter({ visible: true });

  // Below the breakpoint the split shows one member at a time behind its switcher.
  await appPage.setViewportSize({ width: 390, height: 844 });
  await expect(appPage.locator('[data-slot="os-dock-tabbar"]')).toBeVisible();
  await expect(appPage.getByLabel("Windows in this split")).toBeVisible();
  await expect(members).toHaveCount(1);
  const memberBox = await members.boundingBox();
  const viewport = appPage.viewportSize();
  if (!memberBox || !viewport) throw new Error("the shown member must be measurable");
  expect(Math.round(memberBox.width)).toBe(viewport.width);
  expect(
    tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)?.root.weights
  ).toEqual(before.root.weights);

  // Back above the breakpoint both members return side by side.
  await appPage.setViewportSize({ width: 1280, height: 720 });
  await expect(members).toHaveCount(2);
  await expect(appPage.getByLabel("Windows in this split")).toHaveCount(0);
});

// The browser owns snapshot animation: exercise the shared vocabulary with the real API,
// while the Appearance control and OS media preference supply the actual shell signal.
async function contentMotionState(page: Page) {
  return page.evaluate(async () => {
    const probe = document.createElement("div");
    probe.style.cssText = "position:fixed;inset:0;width:20px;height:20px;background:red";
    probe.style.viewTransitionName = "motion-preference-probe";
    probe.style.setProperty("view-transition-class", "vt-fade");
    const skeleton = document.createElement("div");
    skeleton.className = "skeleton-shimmer animate-skeleton-reveal";
    document.body.append(probe, skeleton);
    try {
      const transition = document.startViewTransition(() => {
        probe.style.backgroundColor = "blue";
      });
      await transition.ready;
      const animations = document.getAnimations().flatMap(animation => {
        const effect = animation.effect;
        if (!(effect instanceof KeyframeEffect)) return [];
        const pseudo = effect.pseudoElement;
        return pseudo?.includes("motion-preference-probe") ? [pseudo] : [];
      });
      const reveal = getComputedStyle(skeleton);
      const sweepName = getComputedStyle(skeleton, "::after").animationName;
      const revealDelay = parseFloat(reveal.animationDelay);
      const revealDuration = parseFloat(reveal.animationDuration);
      await transition.finished;
      return { animations, sweepName, revealDelay, revealDuration };
    } finally {
      probe.remove();
      skeleton.remove();
    }
  });
}

test("E2E-004: appearance stays client-local and never writes the window layout", async ({
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
  await appPage.emulateMedia({ reducedMotion: "no-preference" });
  await expect(reduceMotion).not.toBeChecked();
  const animated = await contentMotionState(appPage);
  expect(animated.animations.length).toBeGreaterThan(0);
  expect(animated.sweepName).toBe("skeleton-sweep");
  expect(animated.revealDelay).toBeGreaterThan(0);
  await reduceMotion.click();
  await expect(reduceMotion).toBeChecked();
  await expect(appPage.locator("html")).toHaveAttribute("data-reduce-motion", "true");
  const appReduced = await contentMotionState(appPage);
  expect(appReduced.animations).toEqual([]);
  expect(appReduced.sweepName).toBe("none");
  expect(appReduced.revealDelay).toBe(animated.revealDelay);
  expect(appReduced.revealDuration).toBeLessThan(animated.revealDuration);
  await appPage.emulateMedia({ reducedMotion: "reduce" });
  const bothReduced = await contentMotionState(appPage);
  expect(bothReduced.animations).toEqual([]);
  expect(bothReduced.sweepName).toBe("none");
  expect(bothReduced.revealDelay).toBe(animated.revealDelay);
  await reduceMotion.click();
  await expect(reduceMotion).not.toBeChecked();
  const systemReduced = await contentMotionState(appPage);
  expect(systemReduced.animations).toEqual([]);
  expect(systemReduced.sweepName).toBe("none");
  expect(systemReduced.revealDelay).toBe(animated.revealDelay);
  await appPage.emulateMedia({ reducedMotion: "no-preference" });
  await expect(appPage.locator("html")).toHaveAttribute("data-reduce-motion", "false");
  const restored = await contentMotionState(appPage);
  expect(restored.animations.length).toBeGreaterThan(0);
  expect(restored.sweepName).toBe(animated.sweepName);
  expect(restored.revealDelay).toBe(animated.revealDelay);
  await reduceMotion.click();
  await expect(reduceMotion).toBeChecked();
  expect((await windowManagerSnapshot(runtime, workspace.id)).revision).toBe(
    revisionBeforePreferences
  );

  // Settings holds the active tab; Tasks keeps its own tab and shows again when chosen.
  const shell = osShellSelectors(appPage);
  await expect(tasksWindow).toBeHidden();
  await shell.tabOfWindow(tasksID).click();
  await expect(shell.tabOfWindow(tasksID)).toHaveAttribute("aria-selected", "true");
  await expect(tasksWindow).toBeVisible();
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
  const arrangedTasksTab = (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]
    ?.tab_id;
  if (!arrangedTasksTab) throw new Error("Tasks must sit in an authoritative tab");

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
  // attention feed (the aggregate route serves it now), but nothing else of B's and
  // no foreign session payload is read before confirmation.
  expect(new Set(apiRequests.filter(pathname => pathname.includes(session.id)))).toEqual(
    new Set([
      `/api/workspaces/${workspace.id}/sessions/${session.id}`,
      `/api/sessions/${session.id}/owner`,
    ])
  );
  expect(
    apiRequests.filter(
      pathname =>
        pathname.startsWith(`/api/workspaces/${secondWorkspace.id}/`) &&
        pathname !== `/api/workspaces/${secondWorkspace.id}/loop-requests`
    )
  ).toEqual([]);

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
  // The fallback opened its own tab; A's Tasks tab is untouched.
  await expect(osShellSelectors(appPage).tab(arrangedTasksTab)).toBeVisible();
  expect((await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID]?.tab_id).toBe(
    arrangedTasksTab
  );

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
  const workspace = await prepareShell(appPage, runtime);
  await createApprovalTask(runtime, "Compact parity approval", workspace.id);
  const detailTask = await createTask(runtime, "Compact deep link target", workspace.id);

  await appPage.setViewportSize({ width: 390, height: 844 });
  await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(detailTask.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await completeOnboardingIfPrompted(appPage);

  // Deep link lands focused in the stack.
  const tasksWindow = appWindow(appPage, "tasks");
  await expect(tasksWindow).toBeVisible();
  await expect(tasksWindow).toContainText(detailTask.title);

  // Badges render truthfully in the tab bar (awaiting-approval projection).
  const tabbar = appPage.locator('[data-slot="os-dock-tabbar"]');
  await expect(tabbar).toBeVisible();
  await expect(tabbar.locator('[data-app="tasks"] [data-slot="os-dock-badge"]')).toHaveText("1");

  // Tab-bar semantics: tapping the focused app switches to it — never hides it.
  await tabbar.locator('[data-app="tasks"]').click();
  await expect(tasksWindow).toBeVisible();

  // Compact chrome has no menubar, so the palette owns the dedicated catalog entry.
  await openSessionsCatalogFromPalette(appPage);
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await appPage.keyboard.press("Escape");
  await expect(sessionsModal).toHaveCount(0);
  await expect(tasksWindow).toBeVisible();
  await expect(tasksWindow).toContainText(detailTask.title);
});

test("E2E-032 (logical E2E-005): the tab shortcut creates a real destination-backed tab", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const shell = osShellSelectors(appPage);
  const existing = new Set(
    Object.keys((await windowManagerSnapshot(runtime, workspace.id)).windows)
  );
  const freshNewTab = (snapshot: WindowManagerSnapshot) =>
    Object.values(snapshot.windows).find(
      window => window.app === "new-tab" && !existing.has(window.id)
    );
  await appPage.keyboard.press("ControlOrMeta+T");

  await expect
    .poll(async () => freshNewTab(await windowManagerSnapshot(runtime, workspace.id))?.id ?? null)
    .not.toBeNull();
  // `expect.poll` does not expose its resolved value; read the authority once
  // it is stable so the rest of the assertion remains ID-opaque.
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const newWindow = freshNewTab(snapshot);
  if (!newWindow) throw new Error("⌘T must create a daemon-backed new-tab window");
  const order = desktopTabIds(snapshot, "desktop-default");
  expect(order.indexOf(newWindow.tab_id)).toBe(
    order.indexOf(snapshot.windows[tasksID]?.tab_id ?? "") + 1
  );
  await expect(shell.window(newWindow.id)).toBeVisible();
  await expect(shell.window(newWindow.id).getByTestId("new-tab-empty")).toBeVisible();
  await expect(shell.tab(newWindow.tab_id)).toHaveAttribute("aria-selected", "true");
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
  await expect(shell.tabOfWindow(destinationWindow.id)).toHaveAttribute("aria-selected", "true");
  // The previous-tab shortcut steps back to the tab the new one opened after.
  await appPage.keyboard.press("Control+Shift+Tab");
  await expect(shell.tabOfWindow(tasksID)).toHaveAttribute("aria-selected", "true");
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
  await menu.getByTestId("os-dock-app-menu-tasks-tab").click();

  await tasksDockItem.click({ button: "right" });
  menu = appPage.getByTestId("os-dock-app-menu-tasks");
  await expect(menu).toBeVisible();
  await menu.getByTestId("os-dock-app-menu-tasks-tab").click();

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
  // Every instance is its own tab of the desktop's bar.
  expect(new Set(taskIDs.map(id => snapshot.windows[id]?.tab_id)).size).toBe(3);
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
  await expect(osShellSelectors(appPage).tabOfWindow(expectedCycleID)).toHaveAttribute(
    "aria-selected",
    "true"
  );
});

test("Shell-rail P6: the rail opens a second app as a tab after the active one and ⌥-click splits", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const dock = appPage.locator('[data-slot="os-dock"]');
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const tasksID = await windowID(tasks);
  const agents = await openDockApp(appPage, "Agents", "agents");
  const agentsID = await windowID(agents);

  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      const order = desktopTabIds(snapshot, "desktop-default");
      const tasksTab = snapshot.windows[tasksID]?.tab_id ?? "";
      const agentsTab = snapshot.windows[agentsID]?.tab_id ?? "";
      return tasksTab !== agentsTab && order.indexOf(agentsTab) === order.indexOf(tasksTab) + 1;
    })
    .toBe(true);
  const shell = osShellSelectors(appPage);
  await expect(shell.tabOfWindow(tasksID)).toBeVisible();
  await expect(shell.tabOfWindow(agentsID)).toHaveAttribute("aria-selected", "true");

  await dock.getByRole("button", { name: "Tasks", exact: true }).click({ modifiers: ["Alt"] });
  let splitID = "";
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      const split = Object.values(snapshot.windows).find(
        window => window.app === "tasks" && window.id !== tasksID
      );
      splitID = split?.id ?? "";
      if (split === undefined) return null;
      const tab = tabForWindow(snapshot, split.id);
      return {
        sameTab: tab !== null && tab.id === snapshot.windows[agentsID]?.tab_id,
        rootKind: tab?.root.kind,
      };
    })
    .toEqual({ sameTab: true, rootKind: "split" });
  await expect(shell.window(splitID)).toBeVisible();
  await expect(shell.window(agentsID)).toBeVisible();
});

test("E2E-034 (logical E2E-008): a pinned tab refuses Cmd+W and offers Unpin until unpinned", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("pin fixture must open two tabs");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.pin",
    payload: { window_id: tasksID, pinned: true },
  });
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const tasksTab = snapshot.windows[tasksID]?.tab_id ?? "";
  const agentsTab = snapshot.windows[agentsID]?.tab_id ?? "";
  // Pinned tabs sit at the start of the bar.
  expect(desktopTabIds(snapshot, "desktop-default")[0]).toBe(tasksTab);

  const shell = osShellSelectors(appPage);
  await expect(shell.tab(tasksTab)).toHaveAttribute("data-pinned", "");
  await expect(shell.tabClose(tasksTab)).toHaveCount(0);
  await shell.tab(tasksTab).click();
  await expect(shell.window(tasksID)).toBeVisible();
  await appPage.keyboard.press("ControlOrMeta+W");
  await expect(shell.tab(tasksTab)).toBeVisible();
  await expect(shell.window(tasksID)).toBeVisible();

  await shell.tab(tasksTab).click({ button: "right" });
  await expect(appPage.getByRole("menuitem", { name: "Unpin tab" })).toBeVisible();
  await expect(appPage.getByRole("menuitem", { name: "Close tab", exact: true })).toHaveCount(0);
  // Bulk close (Close ▸ Other tabs) skips pinned tabs.
  await appPage.getByTestId(`os-tab-close-menu-${tasksTab}`).click();
  await appPage.getByRole("menuitem", { name: "Other tabs", exact: true }).click();
  await expect(shell.tab(agentsTab)).toHaveCount(0);
  await expect(shell.tab(tasksTab)).toBeVisible();

  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.pin",
    payload: { window_id: tasksID, pinned: false },
  });
  await expect(shell.tab(tasksTab)).not.toHaveAttribute("data-pinned", "");
  await shell.tab(tasksTab).hover();
  await expect(shell.tabClose(tasksTab)).toHaveCount(1);
  await shell.tabClose(tasksTab).click();
  await expect(shell.tab(tasksTab)).toHaveCount(0);
});

// Invariant: opening a window adds a tab right after the opener's active tab and activates it
// for the opener (US-001). Owner: tab bar + daemon open placement; canonical suite: this file.
test("Tabs-split E2E-001: opening two sessions adds two tabs, the second active right after the first", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const first = await createNamedSession(runtime, workspace.id, "First tab session");
  const second = await createNamedSession(runtime, workspace.id, "Second tab session");
  const shell = osShellSelectors(appPage);

  for (const session of [first, second]) {
    await openSessionsCatalog(appPage);
    const sessionsModal = appPage.getByTestId("os-sessions-modal");
    await sessionsModal.getByTestId(`os-sessions-modal-session-${session.id}`).first().click();
    await expect(sessionsModal).toHaveCount(0);
    await expect(sessionWindow(appPage, session.id)).toBeVisible();
  }

  const sessionWindowOf = (snapshot: WindowManagerSnapshot, sessionId: string) =>
    Object.values(snapshot.windows).find(
      window => window.app === "session" && window.instance_key === sessionId
    );
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      return [sessionWindowOf(snapshot, first.id), sessionWindowOf(snapshot, second.id)].every(
        Boolean
      );
    })
    .toBe(true);
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const firstWindow = sessionWindowOf(snapshot, first.id);
  const secondWindow = sessionWindowOf(snapshot, second.id);
  if (!firstWindow || !secondWindow) throw new Error("both sessions must own a window");
  expect(firstWindow.tab_id).not.toBe(secondWindow.tab_id);
  const order = desktopTabIds(snapshot, firstWindow.desktop_id);
  expect(order.indexOf(secondWindow.tab_id)).toBe(order.indexOf(firstWindow.tab_id) + 1);

  await expect(shell.tab(secondWindow.tab_id)).toHaveAttribute("aria-selected", "true");
  await expect(shell.tab(firstWindow.tab_id)).not.toHaveAttribute("aria-selected", "true");
  await expect(sessionWindow(appPage, first.id)).toBeHidden();
  // The rendered bar follows the authority's order.
  await expect
    .poll(() => shell.tabs.evaluateAll(tabs => tabs.map(tab => tab.getAttribute("data-testid"))))
    .toEqual(order.map(id => `os-tab-${id}`));
});

// Invariant: closing the active tab activates its right neighbour (else the left one), reopen
// restores the tab at its former position with its route, and a middle click closes a tab
// (US-002, US-006). Owner: tab bar + daemon close/reopen; canonical suite: this file.
test("Tabs-split E2E-002: close, reopen, and middle-click keep the bar's order and routes", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const ids = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents", "marketplace"]);
  const shell = osShellSelectors(appPage);
  const opened = await windowManagerSnapshot(runtime, workspace.id);
  const order = desktopTabIds(opened, "desktop-default");
  // Close the fixture tab that has a neighbour on each side.
  const target = ids
    .map(id => ({ id, index: order.indexOf(opened.windows[id]?.tab_id ?? "") }))
    .find(entry => entry.index > 0 && entry.index < order.length - 1);
  if (!target) throw new Error("the close fixture needs a tab between two others");
  const closedTab = order[target.index] ?? "";
  const rightNeighbour = order[target.index + 1] ?? "";
  const routes: Record<string, WindowRoute> = {
    tasks: { pathname: "/tasks", search: { mode: "kanban" } },
    agents: { pathname: `/agents/${encodeURIComponent(browserLifecycleAgent)}`, search: {} },
    marketplace: { pathname: "/marketplace", search: { q: "reopen" } },
  };
  const route = routes[opened.windows[target.id]?.app ?? ""];
  if (!route) throw new Error("the close fixture's target must have a known route");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.navigate",
    payload: { window_id: target.id, mode: "replace", route },
  });

  await shell.tab(closedTab).click();
  await expect(shell.window(target.id)).toBeVisible();
  await appPage.keyboard.press("ControlOrMeta+W");
  await expect(shell.tab(closedTab)).toHaveCount(0);
  await expect(shell.tab(rightNeighbour)).toHaveAttribute("aria-selected", "true");

  await appPage.keyboard.press("ControlOrMeta+Shift+T");
  await expect(shell.window(target.id)).toBeVisible();
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      const reopened = snapshot.windows[target.id];
      return {
        index: desktopTabIds(snapshot, "desktop-default").indexOf(reopened?.tab_id ?? ""),
        route: reopened?.route,
      };
    })
    .toEqual({ index: target.index, route });
  await expect(shell.tabOfWindow(target.id)).toHaveAttribute("aria-selected", "true");

  // A middle click closes a tab without activating it first.
  await shell.tab(rightNeighbour).click({ button: "middle" });
  await expect(shell.tab(rightNeighbour)).toHaveCount(0);
  await expect(shell.tabOfWindow(target.id)).toHaveAttribute("aria-selected", "true");
});

// Invariant: jump, next, and previous shortcuts activate the expected tab of the active
// desktop, and a jump past the last tab changes nothing (US-003). Owner: tab shortcuts;
// canonical suite: this file.
test("Tabs-split E2E-003: jump and next/previous shortcuts activate the expected tabs", async ({
  appPage,
  runtime,
}) => {
  // `meta` chords follow the platform's primary modifier. On Linux runners that is Control,
  // which `desktop.switch.N` also binds; pin the Mac primary so the tab chords are unambiguous.
  await appPage.addInitScript(() => {
    Object.defineProperty(Navigator.prototype, "platform", { get: () => "MacIntel" });
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  const workspace = await prepareShell(appPage, runtime);
  await openFixtureWindows(runtime, workspace.id, ["tasks", "agents", "marketplace"]);
  const shell = osShellSelectors(appPage);
  const order = desktopTabIds(
    await windowManagerSnapshot(runtime, workspace.id),
    "desktop-default"
  );
  expect(order.length).toBeGreaterThanOrEqual(4);
  expect(order.length).toBeLessThan(8);
  const expectActive = async (tabId: string | undefined) => {
    if (!tabId) throw new Error("the shortcut fixture must hold the expected tab");
    await expect(shell.tab(tabId)).toHaveAttribute("aria-selected", "true");
  };
  await expect(shell.tabs).toHaveCount(order.length);

  await appPage.keyboard.press("Meta+Digit1");
  await expectActive(order[0]);
  await appPage.keyboard.press("Meta+Digit3");
  await expectActive(order[2]);
  await appPage.keyboard.press("Control+Tab");
  await expectActive(order[3]);
  await appPage.keyboard.press("Control+Shift+Tab");
  await expectActive(order[2]);
  await appPage.keyboard.press("Control+Shift+Tab");
  await expectActive(order[1]);
  // Jumping past the last tab changes nothing.
  await appPage.keyboard.press("Meta+Digit8");
  await expectActive(order[1]);
  await appPage.keyboard.press("Meta+Digit9");
  await expectActive(order.at(-1));
});

// Invariant: the active tab is per client: switching tabs in one client leaves another's
// unchanged, and a tab opened in one appears in the other's bar without activating there
// (US-024). Owner: per-client tab activation; canonical suite: this file.
test("Tabs-split E2E-014 / E2E-037: two clients keep independent active tabs and share the bar", async ({
  appPage,
  browser,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("client-independence fixture must open two tabs");
  const peer = await openPeerPage(browser, runtime);
  try {
    const shell = osShellSelectors(appPage);
    const peerShell = osShellSelectors(peer);
    const [firstClientID, secondClientID] = await Promise.all([
      browserClientId(appPage),
      browserClientId(peer),
    ]);
    const snapshot = await windowManagerSnapshot(runtime, workspace.id);
    const tasksTab = snapshot.windows[tasksID]?.tab_id ?? "";
    const agentsTab = snapshot.windows[agentsID]?.tab_id ?? "";

    await shell.tab(tasksTab).click();
    await peerShell.tab(agentsTab).click();
    await expect(shell.window(tasksID)).toBeVisible();
    await expect(peerShell.window(agentsID)).toBeVisible();
    await expect
      .poll(
        async () =>
          (await windowManagerClient(runtime, workspace.id, firstClientID)).active_tab?.[
            "desktop-default"
          ]
      )
      .toBe(tasksTab);
    await expect
      .poll(
        async () =>
          (await windowManagerClient(runtime, workspace.id, secondClientID)).active_tab?.[
            "desktop-default"
          ]
      )
      .toBe(agentsTab);

    // Switching tabs in one client leaves the other where it was.
    await shell.tab(agentsTab).click();
    await expect(shell.window(agentsID)).toBeVisible();
    await expect(peerShell.tab(agentsTab)).toHaveAttribute("aria-selected", "true");
    await peerShell.tab(tasksTab).click();
    await expect(peerShell.window(tasksID)).toBeVisible();
    await expect(shell.tab(agentsTab)).toHaveAttribute("aria-selected", "true");
    await expect(shell.window(tasksID)).toBeHidden();

    // A tab opened in one client appears in the other's bar without taking over its view.
    const marketplace = await openDockApp(appPage, "Marketplace", "marketplace");
    const marketplaceID = await windowID(marketplace);
    await expect(peerShell.tabOfWindow(marketplaceID)).toBeVisible();
    await expect(peerShell.tabOfWindow(marketplaceID)).not.toHaveAttribute("aria-selected", "true");
    await expect(peerShell.tab(tasksTab)).toHaveAttribute("aria-selected", "true");
    await expect(peer.getByTestId(`os-window-${marketplaceID}`)).toBeHidden();
  } finally {
    await peer.context().close();
  }
});

// Invariant: dragging a tab along the bar reorders it within its range and the order persists;
// the tab menu's Move ▸ Left takes it one place back; a pinned tab sits icon-only at the left after
// reload (US-004, US-005). Owner: tab drag store + `window.tab.reorder`; canonical suite: this file.
test("Tabs-split E2E-004: dragging a tab two positions persists its order; a pinned tab stays left", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  await openFixtureWindows(runtime, workspace.id, ["tasks", "agents", "marketplace"]);
  const shell = osShellSelectors(appPage);
  const order = desktopTabIds(
    await windowManagerSnapshot(runtime, workspace.id),
    "desktop-default"
  );
  expect(order.length).toBeGreaterThanOrEqual(4);
  // The boot tab leads and is active; drag an inactive fixture tab past two others.
  const moved = order[1] ?? "";
  const landing = order[3] ?? "";
  await expect(shell.tabs).toHaveCount(order.length);

  const from = await boxCenter(shell.tab(moved));
  const landingBox = await requiredBox(shell.tab(landing));
  await startPointerDrag(appPage, from);
  // Past the middle of the tab two places on: the dragged tab lands right after it.
  await movePointer(appPage, { x: landingBox.x + landingBox.width * 0.8, y: from.y });
  await expect(appPage.locator('[data-slot="os-tab-insertion"]')).toBeVisible();
  // Nothing activates during the drag.
  await expect(shell.tab(moved)).not.toHaveAttribute("aria-selected", "true");
  await appPage.mouse.up();

  const expected = [order[0], order[2], order[3], moved, ...order.slice(4)];
  await expect
    .poll(async () =>
      desktopTabIds(await windowManagerSnapshot(runtime, workspace.id), "desktop-default")
    )
    .toEqual(expected);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(shell.tabs).toHaveCount(order.length);
  expect(await barTabIds(appPage)).toEqual(expected);

  // The tab menu's Move ▸ Left takes it one place back.
  await shell.tab(moved).click({ button: "right" });
  await appPage.getByTestId(`os-tab-move-menu-${moved}`).click();
  await appPage.getByRole("menuitem", { name: "Left", exact: true }).click();
  const movedLeft = [order[0], order[2], moved, order[3], ...order.slice(4)];
  await expect
    .poll(async () =>
      desktopTabIds(await windowManagerSnapshot(runtime, workspace.id), "desktop-default")
    )
    .toEqual(movedLeft);
  await expect.poll(() => barTabIds(appPage)).toEqual(movedLeft);

  await shell.tab(landing).click({ button: "right" });
  await appPage.getByRole("menuitem", { name: "Pin tab" }).click();
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(shell.tab(landing)).toHaveAttribute("data-pinned", "");
  await expect(shell.tabClose(landing)).toHaveCount(0);
  expect((await barTabIds(appPage))[0]).toBe(landing);
});

// Invariant: a tab dragged below the bar splits into the active tab at the nearest edge of the
// window under the pointer, and within 24px of the content's outer edge it becomes a full-height
// member at that root edge (US-007). Owner: drop-edge + drag store + `window.move`.
test("Tabs-split E2E-005: dropping tabs on a window's edge and on the content's outer edge splits them", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID, marketplaceID] = await openFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
    "marketplace",
  ]);
  if (!tasksID || !agentsID || !marketplaceID) throw new Error("split fixture needs three tabs");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);

  const tasksBox = await requiredBox(shell.window(tasksID));
  await startPointerDrag(appPage, await boxCenter(shell.tabOfWindow(agentsID)));
  await movePointer(appPage, {
    x: tasksBox.x + tasksBox.width * 0.85,
    y: tasksBox.y + tasksBox.height / 2,
  });
  await expect(appPage.locator('[data-slot="os-drop-zone"][data-edge="right"]')).toBeVisible();
  await appPage.mouse.up();
  await expect
    .poll(async () => {
      const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
      return tab?.root.kind === "split"
        ? { axis: tab.root.axis, members: nodeWindowIds(tab.root), weights: tab.root.weights }
        : null;
    })
    .toEqual({ axis: "horizontal", members: [tasksID, agentsID], weights: [0.5, 0.5] });
  await expect(shell.window(tasksID)).toBeVisible();
  await expect(shell.window(agentsID)).toBeVisible();

  const layer = await requiredBox(appPage.locator('[data-slot="os-desk-layer"]'));
  await startPointerDrag(appPage, await boxCenter(shell.tabOfWindow(marketplaceID)));
  await movePointer(appPage, { x: layer.x + 8, y: layer.y + layer.height / 2 });
  await expect(appPage.locator('[data-slot="os-drop-zone-root"][data-edge="left"]')).toBeVisible();
  await appPage.mouse.up();
  await expect
    .poll(async () => {
      const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
      return tab?.root.kind === "split"
        ? { axis: tab.root.axis, members: (tab.root.children ?? []).map(nodeWindowIds) }
        : null;
    })
    .toEqual({ axis: "horizontal", members: [[marketplaceID], [tasksID], [agentsID]] });
});

// Invariant: dragging the active tab below the bar previews the previously active tab as the
// drop target, and Escape restores the active tab and order without a command (US-007.EC-1/EC-4).
test("Tabs-split E2E-006: dragging the active tab previews the previous tab and Escape restores it", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("preview fixture needs two tabs");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);
  await showTab(appPage, runtime, workspace.id, agentsID);
  const before = await windowManagerSnapshot(runtime, workspace.id);

  const from = await boxCenter(shell.tabOfWindow(agentsID));
  const layer = await requiredBox(appPage.locator('[data-slot="os-desk-layer"]'));
  await startPointerDrag(appPage, from);
  await movePointer(appPage, { x: layer.x + layer.width * 0.6, y: layer.y + layer.height / 2 });
  await expect(shell.window(tasksID)).toBeVisible();
  await expect(shell.window(agentsID)).toBeHidden();
  await expect(appPage.locator('[data-slot="os-drag-ghost"]')).toBeVisible();

  await appPage.keyboard.press("Escape");
  await appPage.mouse.up();
  await expect(appPage.locator('[data-slot="os-drag-ghost"]')).toHaveCount(0);
  await expect(shell.tabOfWindow(agentsID)).toHaveAttribute("aria-selected", "true");
  await expect(shell.window(agentsID)).toBeVisible();
  await expect(shell.window(tasksID)).toBeHidden();
  const after = await windowManagerSnapshot(runtime, workspace.id);
  expect(desktopTabIds(after, "desktop-default")).toEqual(desktopTabIds(before, "desktop-default"));
  expect(tabForWindow(after, agentsID)?.root.kind).toBe("leaf");
});

// Invariant: a member's head dropped on a sibling's edge moves it there; with the swap modifier
// the two members trade places (US-008). Owner: drag store + `window.move` / `window.swap`.
test("Tabs-split E2E-007: a head dropped on a sibling's top edge stacks them, and Shift swaps them", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("rearrange fixture needs two windows");
  await splitWindowBeside(runtime, workspace.id, agentsID, tasksID, "right");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);
  await expect(shell.window(agentsID)).toBeVisible();

  // The title is the head's grab surface; the head's centre may land on a picker or an action.
  const head = (windowId: string) =>
    shell.window(windowId).locator('[data-slot="os-window-head"] [data-slot="topbar-title"]');
  const tasksBox = await requiredBox(shell.window(tasksID));
  await startPointerDrag(appPage, await boxCenter(head(agentsID)));
  await movePointer(appPage, {
    x: tasksBox.x + tasksBox.width / 2,
    y: tasksBox.y + tasksBox.height * 0.2,
  });
  await expect(appPage.locator('[data-slot="os-drop-zone"][data-edge="top"]')).toBeVisible();
  await appPage.mouse.up();
  const arrangement = async () => {
    const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
    return tab?.root.kind === "split"
      ? { axis: tab.root.axis, members: nodeWindowIds(tab.root) }
      : null;
  };
  await expect.poll(arrangement).toEqual({ axis: "vertical", members: [agentsID, tasksID] });

  const target = await requiredBox(shell.window(tasksID));
  await startPointerDrag(appPage, await boxCenter(head(agentsID)));
  await appPage.keyboard.down("Shift");
  await movePointer(appPage, { x: target.x + target.width / 2, y: target.y + target.height / 2 });
  await expect(appPage.locator('[data-slot="os-drop-zone"][data-mode="swap"]')).toBeVisible();
  await appPage.mouse.up();
  await appPage.keyboard.up("Shift");
  await expect.poll(arrangement).toEqual({ axis: "vertical", members: [tasksID, agentsID] });
});

// Invariant: the palette's directional swap trades the focused member with its on-screen
// neighbour on that side, focus stays on the moved window, and one layout undo restores the places
// (#1658). Owner: `window.swap.*` client ops + `window.swap` / `layout.undo`.
test("Tabs-split #1658: palette Swap right trades the focused member with its neighbour, and undo restores it", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID, marketplaceID] = await openFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
    "marketplace",
  ]);
  if (!tasksID || !agentsID || !marketplaceID) throw new Error("swap fixture needs three windows");
  await splitWindowBeside(runtime, workspace.id, agentsID, tasksID, "right");
  await splitWindowBeside(runtime, workspace.id, marketplaceID, agentsID, "right");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);
  const members = async () => {
    const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
    return tab ? nodeWindowIds(tab.root) : [];
  };
  await expect.poll(members).toEqual([tasksID, agentsID, marketplaceID]);
  const clientID = await browserClientId(appPage);
  const focused = async () =>
    (await windowManagerClient(runtime, workspace.id, clientID)).focused_window_id;
  await focusWindow(appPage, shell.window(tasksID));
  await expect.poll(focused).toBe(tasksID);

  const runPaletteCommand = async (query: string, commandId: string) => {
    await appPage.keyboard.press("ControlOrMeta+K");
    const palette = appPage.getByTestId("os-command-palette");
    await expect(palette).toBeVisible();
    await palette.getByPlaceholder("Search apps, sessions, and actions…").fill(query);
    await paletteRow(palette, commandId).click();
    await expect(palette).toHaveCount(0);
  };

  await runPaletteCommand("swap", "window.swap.right");
  await expect.poll(members).toEqual([agentsID, tasksID, marketplaceID]);
  await expect.poll(focused).toBe(tasksID);

  await runPaletteCommand("undo layout", "layout.undo");
  await expect.poll(members).toEqual([tasksID, agentsID, marketplaceID]);
});

// Invariant: a seam drag changes only its two sides' shares and persists; a double-click
// balances them; arrow keys move a focused seam; Escape mid-drag cancels it (US-009). Owner: seam hook + `layout.resize`
// and `layout.balance`.
test("Tabs-split E2E-008: seam drag persists, double-click balances, and arrow keys resize", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("seam fixture needs two windows");
  await splitWindowBeside(runtime, workspace.id, agentsID, tasksID, "right");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);
  const seam = appPage.locator('[data-slot="os-split-seam"][role="separator"]:visible');
  await expect(seam).toHaveCount(1);
  const leading = async () =>
    splitHoldingWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)?.weights?.[0] ??
    0;

  const from = await boxCenter(seam);
  await startPointerDrag(appPage, from);
  await movePointer(appPage, { x: from.x + 150, y: from.y });
  await expect(seam).toHaveAttribute("data-state", "drag");
  await appPage.mouse.up();
  await expect.poll(leading).toBeGreaterThan(0.55);
  const committed = await leading();
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(shell.window(tasksID)).toBeVisible();
  const share = async () => {
    const [left, right] = await Promise.all([
      requiredBox(shell.window(tasksID)),
      requiredBox(shell.window(agentsID)),
    ]);
    return left.width / (left.width + right.width);
  };
  await expect.poll(share).toBeCloseTo(committed, 1);

  await seam.dblclick();
  await expect.poll(leading).toBeCloseTo(0.5, 3);

  await seam.focus();
  await appPage.keyboard.press("ArrowRight");
  await expect.poll(leading).toBeCloseTo(0.52, 3);
  await expect(seam).toHaveAttribute("aria-valuenow", "52");

  // Escape mid-drag cancels: the panes return to their shares and the release commits nothing.
  const revision = async () => (await windowManagerSnapshot(runtime, workspace.id)).revision;
  const before = await revision();
  const pressAt = await boxCenter(seam);
  await startPointerDrag(appPage, pressAt);
  await movePointer(appPage, { x: pressAt.x + 150, y: pressAt.y });
  await expect(seam).toHaveAttribute("data-state", "drag");
  await expect.poll(share).toBeGreaterThan(0.57);
  await appPage.keyboard.press("Escape");
  await expect(seam).not.toHaveAttribute("data-state", "drag");
  await appPage.mouse.up();
  await expect.poll(share).toBeCloseTo(0.52, 1);
  expect(await revision()).toBe(before);
  expect(await leading()).toBeCloseTo(0.52, 3);
});

// Invariant: a split member leaves for its own tab, right after the split, by its head button,
// the Window menu or a head dropped on the bar; closing a member collapses the split and reopening
// restores it on the same side (US-010, US-011, US-006.AC-3). Owner: window head, tab drag store
// and `window.move` placement tab; canonical suite: this file.
test("Tabs-split E2E-009: Move to new tab unsplits a member, and a closed member reopens on its side", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("unsplit fixture needs two windows");
  const shell = osShellSelectors(appPage);
  const members = async () => {
    const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID);
    return tab ? nodeWindowIds(tab.root) : [];
  };
  const unsplit = async () => {
    const snapshot = await windowManagerSnapshot(runtime, workspace.id);
    const order = desktopTabIds(snapshot, "desktop-default");
    const tasksTab = tabForWindow(snapshot, tasksID);
    const agentsTab = tabForWindow(snapshot, agentsID);
    return {
      tasks: tasksTab ? nodeWindowIds(tasksTab.root) : [],
      agents: agentsTab ? nodeWindowIds(agentsTab.root) : [],
      next: order.indexOf(agentsTab?.id ?? "") === order.indexOf(tasksTab?.id ?? "") + 1,
    };
  };
  const twoTabs = { tasks: [tasksID], agents: [agentsID], next: true };
  const splitAgain = async () => {
    await splitWindowBeside(runtime, workspace.id, agentsID, tasksID, "right");
    await expect.poll(members).toEqual([tasksID, agentsID]);
    await showTab(appPage, runtime, workspace.id, tasksID);
    await expect(shell.window(agentsID)).toBeVisible();
  };
  const moveToNewTab = (windowId: string) =>
    shell.window(windowId).getByRole("button", { name: "Move to new tab" });

  // The head's button.
  await splitAgain();
  await expect(moveToNewTab(tasksID)).toBeVisible();
  await moveToNewTab(agentsID).click();
  await expect.poll(unsplit).toEqual(twoTabs);
  await showTab(appPage, runtime, workspace.id, tasksID);
  // A window alone in its tab offers no way out of a split.
  await expect(moveToNewTab(tasksID)).toHaveCount(0);

  // The Window menu acts on the focused member.
  await splitAgain();
  await focusWindow(appPage, shell.window(agentsID));
  await openMenu(appPage, "Window");
  await menubarCommandItem(appPage, "window.unsplit").click();
  await expect.poll(unsplit).toEqual(twoTabs);

  // A head dropped on the bar becomes its own tab at the marker.
  await splitAgain();
  const lastTab = await requiredBox(shell.tabs.last());
  // Grab the head by its title: its controls (pickers, actions) never start a drag.
  const head = shell
    .window(agentsID)
    .locator('[data-slot="os-window-head"] [data-slot="topbar-title"]');
  await startPointerDrag(appPage, await boxCenter(head));
  await movePointer(appPage, {
    x: lastTab.x + lastTab.width * 0.8,
    y: lastTab.y + lastTab.height / 2,
  });
  await expect(appPage.locator('[data-slot="os-tab-insertion"]')).toBeVisible();
  await appPage.mouse.up();
  await expect.poll(unsplit).toEqual(twoTabs);

  // Closing a member collapses the split; reopening puts it back on the same side.
  await splitAgain();
  await shell.window(agentsID).getByRole("button", { name: "Close window" }).click();
  await expect.poll(members).toEqual([tasksID]);
  await expect(shell.window(agentsID)).toHaveCount(0);
  await expect(shell.window(tasksID)).toBeVisible();
  await appPage.keyboard.press("ControlOrMeta+Shift+T");
  await expect.poll(members).toEqual([tasksID, agentsID]);
  await expect(shell.window(agentsID)).toBeVisible();
  const [left, right] = await Promise.all([
    requiredBox(shell.window(tasksID)),
    requiredBox(shell.window(agentsID)),
  ]);
  expect(left.x).toBeLessThan(right.x);
});

// Invariant: inside a split the arrows move focus between members, the tile shortcut sends the
// focused member to the tab's root edge, "Split with ▸" joins another tab, and undo/redo replay a
// split (US-012). Owner: shortcut dispatch, tab menu and `window.move`/`layout.undo`.
test("Tabs-split E2E-010: directional focus, tile left, Split with, and undo/redo of a split", async ({
  appPage,
  runtime,
}) => {
  // `meta+KeyZ` follows the platform's primary modifier; pin the Mac one as E2E-003 does.
  await appPage.addInitScript(() => {
    Object.defineProperty(Navigator.prototype, "platform", { get: () => "MacIntel" });
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
    "marketplace",
  ]);
  if (!tasksID || !agentsID) throw new Error("split shortcut fixture needs three tabs");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);
  const opened = await windowManagerSnapshot(runtime, workspace.id);
  const tasksTab = opened.windows[tasksID]?.tab_id ?? "";
  const agentsTab = opened.windows[agentsID]?.tab_id ?? "";
  const tabMembers = async (windowId: string) => {
    const tab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), windowId);
    return tab ? nodeWindowIds(tab.root) : [];
  };

  // Split with ▸ joins the chosen tab to this one's right.
  await shell.tab(tasksTab).click({ button: "right" });
  await appPage.getByTestId(`os-tab-split-with-menu-${tasksTab}`).click();
  await appPage.getByTestId(`os-tab-split-with-${agentsTab}`).click();
  await expect.poll(() => tabMembers(tasksID)).toEqual([tasksID, agentsID]);
  await expect(shell.tab(agentsTab)).toHaveCount(0);
  await expect(shell.window(agentsID)).toBeVisible();

  // Undo takes the split back; redo replays it.
  await focusWindow(appPage, shell.window(tasksID));
  await appPage.keyboard.press("Meta+KeyZ");
  await expect.poll(() => tabMembers(tasksID)).toEqual([tasksID]);
  await expect.poll(() => tabMembers(agentsID)).toEqual([agentsID]);
  await appPage.keyboard.press("Meta+Shift+KeyZ");
  await expect.poll(() => tabMembers(tasksID)).toEqual([tasksID, agentsID]);
  await expect(shell.window(agentsID)).toBeVisible();

  // The arrows move focus between members.
  const clientID = await browserClientId(appPage);
  const focused = async () =>
    (await windowManagerClient(runtime, workspace.id, clientID)).focused_window_id;
  await focusWindow(appPage, shell.window(tasksID));
  await expect.poll(focused).toBe(tasksID);
  await appPage.keyboard.press("Control+ArrowRight");
  await expect.poll(focused).toBe(agentsID);
  await appPage.keyboard.press("Control+ArrowLeft");
  await expect.poll(focused).toBe(tasksID);

  // Tile left sends the focused member to the tab's left edge.
  await appPage.keyboard.press("Control+ArrowRight");
  await expect.poll(focused).toBe(agentsID);
  await appPage.keyboard.press("Control+Alt+ArrowLeft");
  await expect.poll(() => tabMembers(tasksID)).toEqual([agentsID, tasksID]);
});

// Invariant: a tab dragged onto a top-bar desktop indicator moves to that desktop and each bar
// reflects it; the overview lists every desktop's tabs; a tab row dragged onto another card moves
// the whole tab there; choosing a tab switches to its desktop, activates it and closes the
// overview; a card dragged onto another reorders desktops; Escape closes and leaves the desktop
// as it was; closing a desktop with tabs asks for a destination and moves them, closing none
// (US-014, US-015, US-016). Owner: desktop indicators + desktops overview + tab drag store
// `desktop` target + desktop commands; canonical suite: this file.
test("Tabs-split E2E-011: a tab dragged onto a desktop indicator moves there; the overview lists tabs per desktop, moves a dragged tab, jumps, reorders and closes into a destination", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("the overview fixture needs two windows");
  const second = "desktop-e2e-second";
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "desktop.create",
    payload: { desktop_id: second, name: "Second" },
  });
  const shell = osShellSelectors(appPage);
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  const agentsTab = tabForWindow(snapshot, agentsID)?.id;
  if (!agentsTab) throw new Error("Agents must sit in an authoritative tab");
  const firstTabs = desktopTabIds(snapshot, "desktop-default");

  const tasksTab = tabForWindow(snapshot, tasksID)?.id;
  if (!tasksTab) throw new Error("Tasks must sit in an authoritative tab");
  const indicators = appPage.getByRole("navigation", { name: "Desktops" });
  const dragTabOntoIndicator = async (tabId: string, desktopName: string) => {
    await startPointerDrag(appPage, await boxCenter(shell.tab(tabId)));
    // During the drag every indicator grows into a labelled drop target.
    const target = indicators.getByRole("button", { name: `Move tab to ${desktopName}` });
    await movePointer(appPage, await boxCenter(target));
    await expect(target).toHaveAttribute("data-drop-target", "true");
    await appPage.mouse.up();
  };

  await test.step("a tab dragged onto a top-bar indicator moves there and each bar shows it", async () => {
    await dragTabOntoIndicator(tasksTab, "Second");
    await expect
      .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
      .toMatchObject({ desktop_id: second, tab_id: tasksTab });
    await expect(shell.tab(tasksTab)).toHaveCount(0);
    await indicators.getByRole("button", { name: "Desktop 2 of 2: Second" }).click();
    await expect(activeDesktop(appPage, second)).toHaveAttribute("data-active", "true");
    await expect(shell.tab(tasksTab)).toBeVisible();

    // And back, so the overview steps start from the fixture's own tabs.
    await dragTabOntoIndicator(tasksTab, "Desktop 1");
    await expect
      .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[tasksID])
      .toMatchObject({ desktop_id: "desktop-default", tab_id: tasksTab });
    await expect(shell.tab(tasksTab)).toHaveCount(0);
    await indicators.getByRole("button", { name: "Desktop 1 of 2: Desktop 1" }).click();
    await expect(activeDesktop(appPage, "desktop-default")).toHaveAttribute("data-active", "true");
    await expect(shell.tab(tasksTab)).toBeVisible();
  });

  const overview = appPage.locator('[data-slot="desktops-overview"]');
  const openOverview = async () => {
    await openMenu(appPage, "Window");
    await appPage.getByRole("menuitem", { name: /^All desktops/ }).click();
    await expect(overview).toBeVisible();
  };
  const card = (name: string) => overview.getByRole("listitem", { name, exact: true });
  const tabRow = (tabId: string) =>
    overview.locator(`[data-slot="desktop-overview-tab"][data-tab-id="${tabId}"]`);

  await test.step("the overview opens as an overlay listing each desktop's tabs", async () => {
    await openOverview();
    await expect(card("Desktop 1").locator('[data-slot="desktop-overview-tab"]')).toHaveCount(
      Math.min(firstTabs.length, 4)
    );
    await expect(card("Second").locator('[data-slot="desktop-overview-tab"]')).toHaveCount(1);
    await expect(card("Second").getByText("1 tab", { exact: true })).toBeVisible();
  });

  await test.step("a tab row dragged onto another card moves the tab there", async () => {
    await startPointerDrag(appPage, await boxCenter(tabRow(agentsTab)));
    await movePointer(appPage, await boxCenter(card("Second")));
    await expect(card("Second")).toHaveAttribute("data-drop-target", "true");
    await appPage.mouse.up();
    await expect
      .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows[agentsID])
      .toMatchObject({ desktop_id: second, tab_id: agentsTab });
    await expect(card("Second").locator(`[data-tab-id="${agentsTab}"]`)).toBeVisible();
    await expect(card("Desktop 1").locator(`[data-tab-id="${agentsTab}"]`)).toHaveCount(0);
    await expect(overview).toBeVisible();
  });

  await test.step("choosing a tab jumps to it and closes the overview", async () => {
    await tabRow(agentsTab).click();
    await expect(overview).toBeHidden();
    await expect(activeDesktop(appPage, second)).toHaveAttribute("data-active", "true");
    await expect(shell.tab(agentsTab)).toHaveAttribute("aria-selected", "true");
    await expect(shell.window(agentsID)).toBeVisible();
  });

  await test.step("dragging a card reorders desktops", async () => {
    await openOverview();
    const thumbnail = card("Second").getByRole("button", { name: "Current desktop Second" });
    await startPointerDrag(appPage, await boxCenter(thumbnail));
    await movePointer(appPage, await boxCenter(card("Desktop 1")));
    await expect(overview.locator('[data-slot="desktop-gap"]')).toHaveCount(1);
    await appPage.mouse.up();
    await expect
      .poll(async () =>
        [...(await windowManagerSnapshot(runtime, workspace.id)).desktops]
          .sort((left, right) => left.order - right.order)
          .map(desktop => desktop.id)
      )
      .toEqual([second, "desktop-default"]);
    // The release was a drop, not a click: the overview stays open on the same desktop.
    await expect(overview).toBeVisible();
  });

  await test.step("Escape closes the overview and leaves the desktop as it was", async () => {
    await appPage.keyboard.press("Escape");
    await expect(overview).toBeHidden();
    await expect(activeDesktop(appPage, second)).toHaveAttribute("data-active", "true");
    await expect(shell.tab(agentsTab)).toHaveAttribute("aria-selected", "true");
  });

  await test.step("closing a desktop with tabs asks for a destination and moves them", async () => {
    const before = await windowManagerSnapshot(runtime, workspace.id);
    // A tab holding only a New Tab window is a placeholder, not content: the delete drops it
    // (spec "Empty desktop"; reducer UT-039) and moves every other tab.
    const secondTabs = before.desktops.find(desktop => desktop.id === second)?.tabs ?? [];
    const isPlaceholder = (tab: WindowManagerTab) => {
      const ids = nodeWindowIds(tab.root);
      return ids.length === 1 && before.windows[ids[0] ?? ""]?.app === "new-tab";
    };
    const moving = secondTabs.filter(tab => !isPlaceholder(tab)).map(tab => tab.id);
    const dropped = secondTabs.filter(isPlaceholder).flatMap(tab => nodeWindowIds(tab.root));
    expect(moving).toContain(agentsTab);
    await openOverview();
    const closing = card("Second");
    await closing.getByRole("button", { name: "Close Second" }).click();
    const confirm = closing.getByRole("button", { name: "Confirm: Close Second" });
    await expect(confirm).toBeDisabled();
    await closing.getByRole("combobox").selectOption({ label: "Desktop 1" });
    await confirm.click();
    await expect
      .poll(async () =>
        (await windowManagerSnapshot(runtime, workspace.id)).desktops.map(desktop => desktop.id)
      )
      .toEqual(["desktop-default"]);
    const after = await windowManagerSnapshot(runtime, workspace.id);
    // Nothing closed: every tab of the closed desktop now ends the destination's bar.
    expect(desktopTabIds(after, "desktop-default").slice(-moving.length)).toEqual(moving);
    expect(after.windows[agentsID]).toMatchObject({ desktop_id: "desktop-default" });
    for (const id of dropped) expect(after.windows[id]).toBeUndefined();
    await expect(card("Desktop 1").getByRole("button", { name: "Close Desktop 1" })).toHaveCount(0);
  });
});

// Invariant: a stored v5 layout opens as tabs: floating and minimized windows become their own
// tabs, a side-by-side snapped pair one split tab and a split stays a split; the success is told
// in the bell (an item counting what moved, toasted once), never inline, and reading it in one
// client clears it in every client and after a reload (US-017). The failure variants stay inline
// and are owned by the notice suite. Owner: v5 converter + migration status + bell; canonical
// suite: this file.
test("Tabs-split E2E-012: a v5 layout opens as tabs and the bell's upgrade item is read for every client", async ({
  appPage,
  browser,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [floatingID, snappedID, minimizedID, splitLeadID, splitMemberID] = await openFixtureWindows(
    runtime,
    workspace.id,
    ["tasks", "agents", "marketplace", "agents", "tasks"]
  );
  if (!floatingID || !snappedID || !minimizedID || !splitLeadID || !splitMemberID) {
    throw new Error("v5 fixture needs five windows");
  }
  // The split lives on its own desktop: a snapped pair only joins when it is the desktop's sole
  // tiled content.
  const created = await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "desktop.create",
    payload: { desktop_id: "", name: "Split desk" },
  });
  const splitDesk = created.snapshot.desktops.find(desktop => desktop.id !== "desktop-default");
  if (!splitDesk) throw new Error("fixture must create a second desktop");
  for (const id of [splitLeadID, splitMemberID]) {
    await executeWindowManagerCommand(runtime, workspace.id, {
      commandId: "window.move",
      payload: {
        window_id: id,
        destination_desktop_id: splitDesk.id,
        placement: "tab",
        move_group: false,
      },
    });
  }
  await splitWindowBeside(runtime, workspace.id, splitMemberID, splitLeadID, "right");
  const opened = await windowManagerSnapshot(runtime, workspace.id);
  const fixtureIDs = [floatingID, snappedID, minimizedID];
  const others = Object.values(opened.windows)
    .filter(window => window.desktop_id === "desktop-default" && !fixtureIDs.includes(window.id))
    .map(window => window.id);
  const [snappedLeftID, ...floatingOthers] = others;
  if (!snappedLeftID) throw new Error("the default desktop keeps its boot window");
  const floating = [floatingID, ...floatingOthers];

  await appPage.goto("about:blank");
  const seeded = await seedLegacyWindowLayout(runtime, {
    workspaceId: workspace.id,
    floating,
    minimized: [minimizedID],
    snapped: [snappedLeftID, snappedID],
    pinned: [snappedID],
  });
  expect(seeded.version).toBe(5);
  expect(seeded.windows[floatingID]?.placement).toBe("floating");
  await restartRetiredKnowledgeWindowRuntime(runtime);
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await prepareShell(appPage, runtime);

  const loaded = await windowManagerSnapshot(runtime, workspace.id);
  expect(loaded.version).toBe(6);
  const arrangement = (windowId: string) => {
    const tab = tabForWindow(loaded, windowId);
    return tab?.root.kind === "split"
      ? { axis: tab.root.axis, members: nodeWindowIds(tab.root), pinned: tab.pinned }
      : { members: tab ? nodeWindowIds(tab.root) : [], pinned: tab?.pinned };
  };
  expect(arrangement(snappedID)).toEqual({
    axis: "horizontal",
    members: [snappedLeftID, snappedID],
    pinned: false,
  });
  expect(arrangement(splitLeadID)).toMatchObject({ members: [splitLeadID, splitMemberID] });
  for (const id of [...floating, minimizedID]) {
    expect(arrangement(id)).toEqual({ members: [id], pinned: false });
  }

  const description = `${floating.length} floating ${floating.length === 1 ? "window" : "windows"} and 1 minimized window are tabs now. 1 pin inside a split was removed.`;
  const bellTrigger = '[data-slot="os-menubar-bell"]';
  const upgradeRow = `os-attention-layout-upgrade-${workspace.id}`;
  const notice = appPage.locator('[data-slot="os-upgrade-notice"]');
  const toast = appPage.getByTestId("os-attention-toast-layout-upgrade");
  await expect(toast).toContainText("Your windows are now tabs");
  await expect(toast).toContainText(description);
  await expect(notice).toHaveCount(0);

  // Told once per client: a reload that lists the still-unread item again raises no second toast.
  // A page-side observer records any toast that mounts, so one that shows and times out before the
  // check still fails it. Any read of the list the new page makes, its first or a later poll, can
  // only toast into that record.
  await appPage.addInitScript(() => {
    const record = window as typeof window & { __layoutUpgradeToasted?: boolean };
    record.__layoutUpgradeToasted = false;
    new MutationObserver(() => {
      if (document.querySelector('[data-testid="os-attention-toast-layout-upgrade"]')) {
        record.__layoutUpgradeToasted = true;
      }
    }).observe(document, { childList: true, subtree: true });
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await appPage.waitForResponse(
    response => new URL(response.url()).pathname === "/api/notifications/attention" && response.ok()
  );
  await expect(osShellSelectors(appPage).tabs.first()).toBeVisible();
  // Leave the read room to render and run the toast effect before reading the record.
  const toastedAfterReload = await appPage.evaluate(
    () =>
      new Promise<boolean>(resolve =>
        setTimeout(
          () =>
            resolve(
              (window as typeof window & { __layoutUpgradeToasted?: boolean })
                .__layoutUpgradeToasted === true
            ),
          1_000
        )
      )
  );
  expect(toastedAfterReload).toBe(false);

  const peer = await openPeerPage(browser, runtime);
  try {
    await peer.locator(bellTrigger).click();
    const peerRow = peer.getByTestId(upgradeRow);
    await expect(peerRow).toContainText("Your windows are now tabs");
    await expect(peerRow).toContainText(description);
    await expect(peer.locator('[data-slot="os-upgrade-notice"]')).toHaveCount(0);

    await appPage.locator(bellTrigger).click();
    const row = appPage.getByTestId(upgradeRow);
    await expect(row).toContainText(description);
    await row.click();
    // Reading it acknowledges the move daemon-side; the bell stays open on what is left.
    await expect(row).toHaveCount(0);
    await expect(appPage.getByTestId("os-bell-popover")).toBeVisible();
    // The peer's open bell drops the item on its next read of the list.
    await expect(peerRow).toHaveCount(0, { timeout: 15_000 });
  } finally {
    await peer.context().close();
  }
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(osShellSelectors(appPage).tabs.first()).toBeVisible();
  await appPage.locator(bellTrigger).click();
  await expect(appPage.getByTestId("os-bell-popover")).toBeVisible();
  await expect(appPage.getByTestId(upgradeRow)).toHaveCount(0);
  await expect(notice).toHaveCount(0);
});

const RETIRED_CONTROL =
  /minimi[sz]e|zoom|float|top[- ]left|top[- ]right|bottom[- ]left|bottom[- ]right/i;
const RETIRED_COMMANDS = [
  "window.minimize",
  "window.zoom",
  "window.toggle_floating",
  "window.tile.top-left",
  "window.tile.top-right",
  "window.tile.bottom-left",
  "window.tile.bottom-right",
  "window.merge_all",
  "window.tab.detach",
];

// Invariant: no surface offers a retired control: the window head, tab menu, Window menu, dock and
// shortcut cheatsheet carry no minimize, zoom, float or corner tile (US-018). Owner: those
// surfaces plus the daemon catalog (the cheatsheet rows follow the S6 catalog).
test("Tabs-split E2E-013: no minimize, zoom, float or corner-tile control is left in the shell", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("retired-control fixture needs two windows");
  await splitWindowBeside(runtime, workspace.id, agentsID, tasksID, "right");
  const shell = osShellSelectors(appPage);
  await showTab(appPage, runtime, workspace.id, tasksID);
  await expect(shell.window(agentsID)).toBeVisible();

  // Window head: Move to new tab and Close, nothing else.
  for (const id of [tasksID, agentsID]) {
    const controls = shell.window(id).locator('[data-slot="os-window-control"]');
    await expect
      .poll(() => controls.evaluateAll(nodes => nodes.map(node => node.getAttribute("aria-label"))))
      .toEqual(["Move to new tab", "Close window"]);
  }

  // Tab menu.
  const tasksTab = tabForWindow(await windowManagerSnapshot(runtime, workspace.id), tasksID)?.id;
  if (!tasksTab) throw new Error("the split must sit in a tab");
  await shell.tab(tasksTab).click({ button: "right" });
  const tabMenu = appPage.getByRole("menu");
  await expect(tabMenu).toBeVisible();
  await expect(tabMenu).not.toContainText(RETIRED_CONTROL);
  await appPage.keyboard.press("Escape");

  // Window menu and its Arrange submenu.
  await openMenu(appPage, "Window");
  await expect(appPage.getByTestId("os-menu-window")).not.toContainText(RETIRED_CONTROL);
  for (const id of RETIRED_COMMANDS) {
    await expect(menubarCommandItem(appPage, id)).toHaveCount(0);
  }
  await appPage.getByTestId("os-menu-arrange").click();
  const arrange = appPage.getByRole("menu").last();
  await expect(arrange).toContainText("Balance sizes");
  await expect(arrange).not.toContainText(RETIRED_CONTROL);
  await appPage.keyboard.press("Escape");
  await appPage.keyboard.press("Escape");

  // Dock buttons and an app's menu.
  const dock = appPage.locator('[data-slot="os-dock"]:visible');
  await expect(dock.getByRole("button", { name: RETIRED_CONTROL })).toHaveCount(0);
  await dock.getByRole("button", { name: "Tasks" }).click({ button: "right" });
  const dockMenu = appPage.getByTestId("os-dock-app-menu-tasks");
  await expect(dockMenu).toBeVisible();
  await expect(dockMenu).not.toContainText(RETIRED_CONTROL);
  await appPage.keyboard.press("Escape");

  // Shortcut cheatsheet.
  await openMenu(appPage, "Help");
  await appPage.getByRole("menuitem", { name: /^Keyboard shortcuts/ }).click();
  const shortcuts = appPage.getByTestId("os-shortcuts-dialog");
  await expect(shortcuts.getByTestId("os-shortcut-row-window.close")).toBeVisible();
  for (const id of RETIRED_COMMANDS) {
    await expect(shortcuts.getByTestId(`os-shortcut-row-${id}`)).toHaveCount(0);
  }
  const windowRows = await shortcuts
    .locator('[data-testid^="os-shortcut-row-window."]')
    .allTextContents();
  expect(windowRows.filter(row => RETIRED_CONTROL.test(row))).toEqual([]);
});

test("E2E-038 (logical E2E-014): palette Go to tab crosses desktops by opaque identity", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID, duplicateAgentsID] = await openFixtureWindows(runtime, workspace.id, [
    "tasks",
    "agents",
    "agents",
  ]);
  if (!tasksID || !agentsID || !duplicateAgentsID) {
    throw new Error("palette fixture must create duplicate tab labels");
  }
  const created = await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "desktop.create",
    payload: { desktop_id: "", name: "Tab destination" },
  });
  const destination = created.snapshot.desktops.find(desktop => desktop.id !== "desktop-default");
  if (!destination) throw new Error("fixture must create a second desktop");
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.move",
    payload: {
      window_id: agentsID,
      destination_desktop_id: destination.id,
      placement: "tab",
      move_group: false,
    },
  });
  await executeWindowManagerCommand(runtime, workspace.id, {
    commandId: "window.move",
    payload: {
      window_id: duplicateAgentsID,
      destination_desktop_id: destination.id,
      placement: "tab",
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

test("E2E-039 (logical E2E-004/013): a crowded tab bar keeps overflow, tooltip, and last-tab reachability", async ({
  appPage,
  runtime,
}) => {
  // Tabs shrink to fit down to the 40px floor before the bar scrolls (spec "Tab width"): at
  // 1280px the strip holds about 26 floor tabs, so 30 windows plus the New Tab overflow it.
  await appPage.setViewportSize({ width: 1280, height: 720 });
  const workspace = await prepareShell(appPage, runtime);
  const ids = await openFixtureWindows(
    runtime,
    workspace.id,
    Array.from({ length: 30 }, () => "tasks")
  );
  const shell = osShellSelectors(appPage);
  const order = desktopTabIds(
    await windowManagerSnapshot(runtime, workspace.id),
    "desktop-default"
  );
  expect(order.length).toBeGreaterThanOrEqual(30);
  await expect(shell.tabs).toHaveCount(order.length);
  for (const id of ids) await expect(shell.tabOfWindow(id)).toHaveCount(1);
  // The strip scrolls; the bar box around it holds the overflow list's trigger.
  const overflow = shell.tabBar.getByRole("button", { name: `All ${order.length} tabs` });
  await expect(overflow).toBeVisible();
  await expect
    .poll(() => shell.tabStrip.evaluate(strip => strip.scrollWidth > strip.clientWidth))
    .toBe(true);
  const lastID = ids.at(-1);
  if (!lastID) throw new Error("overflow fixture must retain its last tab");
  const lastTab = shell.tabOfWindow(lastID);
  await lastTab.scrollIntoViewIfNeeded();
  await expect(lastTab).toBeVisible();
  await appPage.mouse.move(1, 1);
  await lastTab.hover();
  await expect(appPage.locator('[data-slot="tooltip-content"]')).toContainText("Tasks");
  await lastTab.click();
  await expect(lastTab).toHaveAttribute("aria-selected", "true");
  await expect(shell.window(lastID)).toBeVisible();
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
  const [tasksID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID) throw new Error("reopen fixture must open a Tasks tab");
  const sessionID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  const secondSessionID = await openSessionWindowInAuthority(runtime, workspace.id, secondSession);
  const shell = osShellSelectors(appPage);
  const layoutUnavailable = appPage
    .getByRole("status")
    .filter({ hasText: /Reconnecting…|Can.t save window layout/ });
  await expect(shell.tabOfWindow(sessionID)).toBeVisible();
  await shell.tabOfWindow(sessionID).click();
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
  await expect(shell.tabOfWindow(sessionID)).toHaveCount(0);
  await expect(
    appPage.getByRole("button", { name: "Sessions" }).locator('[data-slot="os-dock-badge"]')
  ).toBeVisible();
  await shell.tabOfWindow(secondSessionID).click();
  await expect(layoutUnavailable).toHaveCount(0);
  await appPage.keyboard.press("ControlOrMeta+W");
  await expect(shell.window(secondSessionID)).toHaveCount(0);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(shell.tabOfWindow(tasksID)).toBeVisible();
  await expect(layoutUnavailable).toHaveCount(0);
  await appPage.keyboard.press("ControlOrMeta+Shift+T");
  await expect(shell.window(secondSessionID)).toBeVisible();
  await appPage.keyboard.press("ControlOrMeta+Shift+T");
  await expect(shell.window(sessionID)).toBeVisible();
  await expect(shell.tabOfWindow(sessionID)).toHaveAttribute("aria-selected", "true");
  await expect(
    shell.window(sessionID).getByRole("textbox", { name: "Session prompt" })
  ).toBeVisible();
});

test("E2E-041 (logical E2E-003): Task root, Task Run, and session document heads survive tab switches", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Tab head identity");
  const run = await createTaskRun(runtime, task.id);
  const session = await createNamedSession(runtime, workspace.id, "Draft preservation");
  const [tasksID, taskRunID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "tasks"]);
  if (!tasksID || !taskRunID) throw new Error("head fixture must open root and Task Run tabs");
  const sessionID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  const shell = osShellSelectors(appPage);

  await shell.tabOfWindow(tasksID).click();
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
  await shell.tabOfWindow(taskRunID).click();
  await expect(shell.window(taskRunID).getByTestId("tasks-run-detail-content")).toBeVisible();
  await expect(shell.window(taskRunID).locator('[data-slot="topbar-crumbs"]')).toContainText(
    task.title
  );
  await shell.tabOfWindow(sessionID).click();
  const composer = shell.window(sessionID).getByRole("textbox", { name: "Session prompt" });
  await composer.fill("draft survives a tab switch");

  await expect(shell.window(sessionID).locator('[data-slot="topbar-title"]')).toContainText(
    "Draft preservation"
  );
  await shell.tabOfWindow(tasksID).click();
  await expect(shell.window(tasksID).locator('[data-slot="topbar-title"]')).toContainText("Tasks");
  await expect(shell.window(sessionID)).toBeHidden();
  await shell.tabOfWindow(sessionID).click();
  await expect(composer).toHaveText("draft survives a tab switch");
});

test("E2E-042 (logical E2E-009): breadcrumb back pops only the active tab navigation stack", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Tab-local navigation");
  const run = await createTaskRun(runtime, task.id);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("navigation fixture must open two tabs");
  const shell = osShellSelectors(appPage);
  await shell.tabOfWindow(tasksID).click();
  await expect(shell.tabOfWindow(tasksID)).toHaveAttribute("aria-selected", "true");
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
    .toEqual({ pathname: `/tasks/${task.id}`, search: { tab: "overview" } });
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
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await expect(appPage.getByRole("button", { name: /^Desktop 1 of 1:/ })).toBeEnabled();
  const session = await createNamedSessionForAgent(
    runtime,
    workspace.id,
    "CLI managed attention",
    "permission-hardening-agent"
  );
  await openFixtureWindows(runtime, workspace.id, ["tasks"]);
  const sessionID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  const shell = osShellSelectors(appPage);
  const sessionWindow = shell.window(sessionID);
  const sessionTab = shell.tabOfWindow(sessionID);
  await sessionTab.click();
  const composer = sessionWindow.getByRole("textbox", { name: "Session prompt" });
  await composer.fill("exercise permission hardening");
  await composer.press("Enter");
  await expect(sessionWindow.getByTestId("permission-dock")).toBeVisible();
  await expect(sessionTab.getByRole("img", { name: "Session needs input" })).toBeVisible();

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
  await expect(sessionTab).toHaveCount(0);
  await openSessionsCatalog(appPage);
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  const sessionRow = sessionsModal.getByTestId(`os-sessions-modal-session-${session.id}`).first();
  await expect(sessionRow).toBeVisible();
  await sessionRow.click();
  const restored = appPage.locator('[data-testid^="os-window-"][data-app="session"]:visible');
  await expect(restored).toHaveCount(1);
  await expect(restored.getByTestId("permission-dock")).toBeVisible();
  await restored.getByTestId("permission-reject-menu-trigger").click();
  await appPage.getByTestId("permission-reject-always").click();
  await expect(restored.getByTestId("permission-dock")).toBeHidden();
  await expect(
    appPage.getByRole("button", { name: "Sessions" }).locator('[data-slot="os-dock-badge"]')
  ).toHaveCount(0);
});

test("E2E-044 (logical E2E-015): daemon restart preserves tab order, pin, navigation, and the active tab", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const [tasksID, agentsID] = await openFixtureWindows(runtime, workspace.id, ["tasks", "agents"]);
  if (!tasksID || !agentsID) throw new Error("restart fixture must open two tabs");
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
  const clientID = await browserClientId(appPage);
  await shell.tabOfWindow(agentsID).click();
  await expect(shell.tabOfWindow(agentsID)).toHaveAttribute("aria-selected", "true");
  const before = await windowManagerSnapshot(runtime, workspace.id);
  const agentsTab = before.windows[agentsID]?.tab_id ?? "";
  const tasksTab = before.windows[tasksID]?.tab_id ?? "";
  await expect
    .poll(
      async () =>
        (await windowManagerClient(runtime, workspace.id, clientID)).active_tab?.["desktop-default"]
    )
    .toBe(agentsTab);
  const beforeOrder = desktopTabIds(before, "desktop-default");

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
  expect(after.version).toBe(6);
  expect(desktopTabIds(after, "desktop-default")).toEqual(beforeOrder);
  expect(tabForWindow(after, tasksID)).toMatchObject({ id: tasksTab, pinned: true });
  expect(after.windows[tasksID]?.nav_stack).toEqual([{ pathname: "/tasks", search: {} }]);
  await expect(shell.tab(tasksTab)).toHaveAttribute("data-pinned", /.*/);
  await expect(shell.tab(agentsTab)).toHaveAttribute("aria-selected", "true");
  await expect(shell.window(agentsID)).toBeVisible();
  await expect
    .poll(
      async () =>
        (await windowManagerClient(runtime, workspace.id, clientID)).active_tab?.["desktop-default"]
    )
    .toBe(agentsTab);
});

test("Memory removal E2E-008: a saved layout holding a retired knowledge window opens with its session window and stays stable on reload", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const session = await createNamedSession(runtime, workspace.id, "Layout upgrade session");
  const sessionWindowID = await openSessionWindowInAuthority(runtime, workspace.id, session);
  // The client-state store is exclusively locked by the daemon. Seed the previous
  // release's raw layout (version 5, a floating knowledge window) during a cold
  // restart, without a reconciling API read.
  await appPage.goto("about:blank");
  const knowledgeID = `${sessionWindowID}-retired-knowledge`;
  const seeded = await seedRetiredKnowledgeWindow(runtime, {
    workspaceId: workspace.id,
    sessionWindowId: sessionWindowID,
    knowledgeWindowId: knowledgeID,
  });
  expect(seeded.version).toBe(5);
  expect(seeded.windows[knowledgeID]?.app).toBe("knowledge");
  expect(seeded.windows[knowledgeID]?.placement).toBe("floating");
  expect(seeded.windows[sessionWindowID]?.app).toBe("session");
  const sessionDesktopID = seeded.windows[sessionWindowID]?.desktop_id;
  const sessionRoute = seeded.windows[sessionWindowID]?.route;
  expect(seeded.windows[knowledgeID]?.desktop_id).toBe(sessionDesktopID);
  expect(seeded.desktops.find(desktop => desktop.id === sessionDesktopID)?.floating).toContain(
    knowledgeID
  );

  // Only boot the daemon after inspecting the actual stored legacy shape. A fresh
  // page then observes the upgraded tab layout and its console output.
  await restartRetiredKnowledgeWindowRuntime(runtime);
  const consoleErrors: string[] = [];
  appPage.on("console", message => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });
  appPage.on("pageerror", error => {
    consoleErrors.push(error.message);
  });
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });

  const loaded = await windowManagerSnapshot(runtime, workspace.id);
  expect(loaded.version).toBe(6);
  expect(loaded.windows[knowledgeID]).toBeUndefined();
  expect(
    loaded.desktops.flatMap(desktop => desktop.tabs.flatMap(tab => nodeWindowIds(tab.root)))
  ).not.toContain(knowledgeID);
  expect(loaded.windows[sessionWindowID]).toMatchObject({
    app: "session",
    desktop_id: sessionDesktopID,
    route: sessionRoute,
  });
  const sessionTab = tabForWindow(loaded, sessionWindowID);
  if (!sessionTab) throw new Error("the upgraded session window must sit in a tab");
  expect(loaded.windows[sessionWindowID]?.tab_id).toBe(sessionTab.id);
  expect(desktopTabIds(loaded, sessionDesktopID ?? "")).toContain(sessionTab.id);
  const shell = osShellSelectors(appPage);
  await expect(shell.tab(sessionTab.id)).toBeVisible();
  await shell.tab(sessionTab.id).click();
  const restoredSession = sessionWindow(appPage, session.id);
  await expect(restoredSession).toBeVisible();

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(shell.tab(sessionTab.id)).toHaveAttribute("aria-selected", "true");
  await expect(restoredSession).toBeVisible();
  const reloaded = await windowManagerSnapshot(runtime, workspace.id);
  expect(reloaded.windows[knowledgeID]).toBeUndefined();
  expect(reloaded.windows[sessionWindowID]?.tab_id).toBe(sessionTab.id);
  expect(desktopTabIds(reloaded, sessionDesktopID ?? "")).toEqual(
    desktopTabIds(loaded, sessionDesktopID ?? "")
  );
  expect(consoleErrors).toEqual([]);
});

test("E2E-045 (logical E2E-018): Tasks views and Marketplace search stay in their window strips", async ({
  appPage,
  runtime,
}, testInfo) => {
  const workspace = await prepareShell(appPage, runtime);
  const task = await createTask(runtime, "Window strip drill state");
  const [tasksID, marketplaceID] = await openFixtureWindows(runtime, workspace.id, [
    "tasks",
    "marketplace",
  ]);
  if (!tasksID || !marketplaceID) throw new Error("strip fixture must open two windows");
  await splitWindowBeside(runtime, workspace.id, marketplaceID, tasksID, "right");
  await showTab(appPage, runtime, workspace.id, tasksID);
  const shell = osShellSelectors(appPage);
  const tasksWindow = shell.window(tasksID);
  const marketplaceWindow = shell.window(marketplaceID);
  const tasksViews = tasksWindow.getByRole("navigation", { name: "Tasks views" });
  const marketplaceSearch = marketplaceWindow.getByRole("searchbox", { name: "Search extensions" });

  await expect(tasksViews).toBeVisible();
  await expect(marketplaceSearch).toBeVisible();
  await expect(tasksWindow.locator('[data-slot="os-window-toolbar"]')).toContainText("List");
  await expect(
    marketplaceWindow
      .locator('[data-slot="os-window-toolbar"]')
      .getByRole("searchbox", { name: "Search extensions" })
  ).toBeVisible();
  await expect(tasksWindow.locator('[data-slot="os-window-head"]')).not.toContainText("Kanban");
  await expect(marketplaceWindow.locator('[data-slot="os-window-head"]')).not.toContainText("MCPs");
  await expect(tasksWindow.getByTestId("topbar-title-text")).toHaveText("Tasks");
  await expect(marketplaceWindow.getByTestId("topbar-title-text")).toHaveText("Marketplace");
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
  "new-tab",
  "loops",
  "automations",
  "marketplace",
  "terminal",
  "settings",
  "vault",
] as const;

test("E2E-023: the 11-tab envelope holds for tab switches, restore, and convergence", async ({
  appPage,
  browser,
  runtime,
}, testInfo) => {
  const workspace = await prepareShell(appPage, runtime);
  const perfWindowIDs = new Map<string, string>();

  const opened = await openFixtureWindows(runtime, workspace.id, PERF_APPS);
  for (const [index, app] of PERF_APPS.entries()) {
    const id = opened[index];
    if (!id) throw new Error(`performance fixture must open ${app}`);
    perfWindowIDs.set(app, id);
  }
  // Jobs and Triggers merged into one Automations app and Knowledge was removed; a
  // second Tasks window keeps the envelope at 11 windows.
  const [extraTasksID] = await openFixtureWindows(runtime, workspace.id, ["tasks"]);
  if (!extraTasksID) throw new Error("performance fixture must open the second tasks window");
  perfWindowIDs.set("tasks#2", extraTasksID);

  await appPage.addInitScript(() => {
    const perf = {
      snapshotResponseEnd: null as number | null,
      tabsPlaced: null as number | null,
    };
    Reflect.set(window, "__osPerf", perf);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        const url = new URL(entry.name);
        if (url.pathname.endsWith("/window-manager") && perf.snapshotResponseEnd === null) {
          perf.snapshotResponseEnd = entry.startTime + entry.duration;
        }
      }
    }).observe({ type: "resource", buffered: true });
    const placed = () => {
      if (perf.tabsPlaced !== null) return;
      const count = document.querySelectorAll('[data-testid="os-tab-bar"] [role="tab"]').length;
      if (count >= 11) perf.tabsPlaced = performance.now();
    };
    // Init scripts run at document start — observe `document` itself so the
    // hook works before <html>/<body> exist.
    new MutationObserver(placed).observe(document, { childList: true, subtree: true });
    document.addEventListener("DOMContentLoaded", placed, { once: true });
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  expect(perfWindowIDs.size).toBe(11);
  const shell = osShellSelectors(appPage);
  for (const id of perfWindowIDs.values()) {
    await expect(shell.tabOfWindow(id)).toHaveCount(1);
  }
  await expect
    .poll(() =>
      appPage.evaluate(() => {
        const perf = Reflect.get(window, "__osPerf") as {
          snapshotResponseEnd: number | null;
          tabsPlaced: number | null;
        };
        if (perf.snapshotResponseEnd === null || perf.tabsPlaced === null) return null;
        return perf.tabsPlaced - perf.snapshotResponseEnd;
      })
    )
    .not.toBeNull();
  const restore = await appPage.evaluate(() => {
    const perf = Reflect.get(window, "__osPerf") as {
      snapshotResponseEnd: number;
      tabsPlaced: number;
    };
    return perf.tabsPlaced - perf.snapshotResponseEnd;
  });
  expect(restore).toBeGreaterThanOrEqual(0);
  expect(restore).toBeLessThan(500);

  // The envelope measures steady-state switching: wait until the main thread has
  // been long-task quiet so the first content burst can't masquerade as switch
  // jank. (networkidle never settles here — the shell keeps WebSocket/SSE
  // connections open by design.)
  await installLongTaskQuietProbe(appPage);
  await waitForLongTaskQuiet(appPage);

  // Switch among the tabs the bar shows without scrolling. Each one is visited
  // once as warm-up: a tab's first mount is lazy app startup, not a switch.
  // The strip is the bar's scrolling viewport: a tab outside its box needs a scroll.
  const barBox = await shell.tabStrip.boundingBox();
  if (!barBox) throw new Error("the tab strip must be measurable");
  const targets: Array<{ x: number; y: number }> = [];
  for (const id of perfWindowIDs.values()) {
    const box = await shell.tabOfWindow(id).boundingBox();
    if (!box || box.x < barBox.x || box.x + box.width > barBox.x + barBox.width) continue;
    targets.push({ x: box.x + box.width / 2, y: box.y + box.height / 2 });
  }
  expect(targets.length).toBeGreaterThanOrEqual(3);
  const switchInput = await appPage.context().newCDPSession(appPage);
  const clickAt = async (point: { x: number; y: number }) => {
    for (const type of ["mousePressed", "mouseReleased"] as const) {
      await switchInput.send("Input.dispatchMouseEvent", {
        type,
        x: point.x,
        y: point.y,
        button: "left",
        buttons: type === "mousePressed" ? 1 : 0,
        clickCount: 1,
      });
    }
  };

  // Long-task probe during 3s of continuous tab switching. Dispatch trusted
  // Chromium input directly: Playwright's mouse helper takes a full DOM snapshot
  // for every traced action, and that recorder work runs on the renderer thread
  // that this envelope is meant to measure.
  let longTasks: number[] = [];
  let switchProbe: SwitchProbeRecords = { switches: [], tasks: [], tooltipsShown: 0, frames: [] };
  let switchCount = 0;
  try {
    for (const target of targets) await clickAt(target);
    await waitForLongTaskQuiet(appPage);
    await installSwitchProbe(appPage);
    const start = Date.now();
    while (Date.now() - start < 3000) {
      const target = targets[switchCount % targets.length];
      if (target) await clickAt(target);
      switchCount += 1;
    }
    switchProbe = await readSwitchProbe(appPage);
    longTasks = switchProbe.tasks.map(task => task.duration);
  } finally {
    await switchInput.detach();
  }
  const worstFrame = longTasks.length > 0 ? Math.max(...longTasks) : 0;
  // Temporary attribution only: replay the same warmed 11-tab layout after scoring and
  // before any peer split changes it. The canonical frame-budget invariant stays above/below.
  const diagnostic = await appPage.context().newCDPSession(appPage);
  try {
    await waitForLongTaskQuiet(appPage);
    await diagnostic.send("Debugger.enable");
    await diagnostic.send("Profiler.enable");
    const layoutEvents: unknown[] = [];
    diagnostic.on("Tracing.dataCollected", ({ value }) => {
      layoutEvents.push(...value);
    });
    await diagnostic.send("Tracing.start", {
      categories:
        "devtools.timeline,disabled-by-default-devtools.timeline.stack,disabled-by-default-devtools.timeline.invalidationTracking",
      transferMode: "ReportEvents",
    });
    await diagnostic.send("Profiler.start");
    for (let index = 0; index < 200; index += 1) {
      const target = targets[index % targets.length];
      if (!target) continue;
      for (const type of ["mousePressed", "mouseReleased"] as const) {
        await diagnostic.send("Input.dispatchMouseEvent", {
          type,
          x: target.x,
          y: target.y,
          button: "left",
          buttons: type === "mousePressed" ? 1 : 0,
          clickCount: 1,
        });
      }
    }
    const { profile } = await diagnostic.send("Profiler.stop");
    const tracingComplete = new Promise<void>(resolve => {
      diagnostic.once("Tracing.tracingComplete", () => resolve());
    });
    await diagnostic.send("Tracing.end");
    await tracingComplete;
    await testInfo.attach("switch-layout-replay.cpuprofile", {
      body: JSON.stringify(profile),
      contentType: "application/json",
    });
    await testInfo.attach("switch-layout-replay-events", {
      body: JSON.stringify(layoutEvents),
      contentType: "application/json",
    });
    const scripts = new Map<string, string>();
    for (const node of profile.nodes) {
      if (node.callFrame.url.includes("/assets/")) {
        scripts.set(node.callFrame.scriptId, node.callFrame.url);
      }
    }
    for (const [scriptId, url] of scripts) {
      const { scriptSource } = await diagnostic.send("Debugger.getScriptSource", { scriptId });
      await testInfo.attach(`switch-layout-replay-${new URL(url).pathname.split("/").pop()}`, {
        body: scriptSource,
        contentType: "text/javascript",
      });
    }
  } catch (error) {
    console.log("[switch-layout-diagnostic]", String(error));
  } finally {
    await diagnostic.detach();
  }
  const appOfWindow = new Map([...perfWindowIDs].map(([app, id]) => [id, app]));

  const peerA = await openPeerPage(browser, runtime);
  const peerB = await openPeerPage(browser, runtime);
  const settingsID = perfWindowIDs.get("settings");
  const vaultID = perfWindowIDs.get("vault");
  const marketplaceID = perfWindowIDs.get("marketplace");
  const loopsID = perfWindowIDs.get("loops");
  if (!settingsID || !vaultID || !marketplaceID || !loopsID) {
    throw new Error("performance fixture must retain its split window IDs");
  }
  let worstPeerTask = 0;
  let peerAttribution: ReturnType<typeof attributePeerLongTasks> = [];
  try {
    const peers = [peerA, peerB];
    for (const peer of peers) {
      for (const id of perfWindowIDs.values()) {
        await expect(osShellSelectors(peer).tabOfWindow(id)).toHaveCount(1);
      }
      await installLongTaskQuietProbe(peer);
      await waitForLongTaskQuiet(peer);
    }
    // Warm the shared topology/convergence path before starting either peer's
    // observer: a split merges two tabs into one. The second split is measured.
    const tabCount = () =>
      windowManagerSnapshot(runtime, workspace.id).then(
        snapshot => desktopTabIds(snapshot, "desktop-default").length
      );
    const warmCount = (await tabCount()) - 1;
    await moveWindowFromCLI(runtime, workspace.id, settingsID, vaultID, "right");
    for (const peer of peers) {
      await expect(osShellSelectors(peer).tabs).toHaveCount(warmCount);
      await waitForLongTaskQuiet(peer);
    }
    const measuredCount = warmCount - 1;
    await Promise.all(peers.map(peer => installPeerConvergenceProbe(peer, measuredCount)));
    await moveWindowFromCLI(runtime, workspace.id, marketplaceID, loopsID, "right");
    const peerProbes = await Promise.all(peers.map(readPeerConvergenceProbe));
    const peerLongTasks = peerProbes.map(probe => probe.tasks.map(task => task.duration));
    peerAttribution = peerProbes.flatMap((probe, index) =>
      attributePeerLongTasks(probe, index === 0 ? "A" : "B")
    );
    for (const peer of peers) {
      await expect(osShellSelectors(peer).tabs).toHaveCount(measuredCount);
    }
    expect(
      tabForWindow(await windowManagerSnapshot(runtime, workspace.id), marketplaceID)?.id
    ).toBe(tabForWindow(await windowManagerSnapshot(runtime, workspace.id), loopsID)?.id);
    worstPeerTask = Math.max(0, ...peerLongTasks.flat());
    // Attribute the response continuation separately, after scoring. Restore only the measured
    // pair's pre-merge topology on each replay; these profiles never replace envelope scores.
    const sampledPeer = peers[peerLongTasks.findIndex(tasks => tasks.includes(worstPeerTask))] ?? peerB;
    const peerDiagnostic = await sampledPeer.context().newCDPSession(sampledPeer);
    try {
      await peerDiagnostic.send("Debugger.enable");
      await peerDiagnostic.send("Profiler.enable");
      const resources = await sampledPeer.evaluate(() => {
        return performance.getEntriesByType("resource").map(entry => {
          const resource = entry as PerformanceResourceTiming;
          return {
            path: new URL(resource.name).pathname,
            start: resource.startTime,
            end: resource.responseEnd,
            bytes: resource.decodedBodySize,
          };
        });
      });
      await testInfo.attach("peer-scored-resources", {
        body: JSON.stringify(resources),
        contentType: "application/json",
      });
      const scripts = new Map<string, string>();
      for (let replay = 0; replay < 3; replay += 1) {
        await moveWindowFromCLI(runtime, workspace.id, marketplaceID, loopsID, "tab");
        for (const peer of peers) {
          await expect(osShellSelectors(peer).tabs).toHaveCount(warmCount);
          await waitForLongTaskQuiet(peer);
        }
        await Promise.all(peers.map(peer => installPeerConvergenceProbe(peer, measuredCount)));
        await peerDiagnostic.send("Profiler.start");
        await moveWindowFromCLI(runtime, workspace.id, marketplaceID, loopsID, "right");
        await Promise.all(peers.map(readPeerConvergenceProbe));
        const { profile } = await peerDiagnostic.send("Profiler.stop");
        await testInfo.attach(`peer-response-replay-${replay}.cpuprofile`, {
          body: JSON.stringify(profile),
          contentType: "application/json",
        });
        for (const node of profile.nodes) {
          if (node.callFrame.url.includes("/assets/")) {
            scripts.set(node.callFrame.scriptId, node.callFrame.url);
          }
        }
      }
      for (const [scriptId, url] of scripts) {
        const { scriptSource } = await peerDiagnostic.send("Debugger.getScriptSource", { scriptId });
        await testInfo.attach(`peer-response-replay-${new URL(url).pathname.split("/").pop()}`, {
          body: scriptSource,
          contentType: "text/javascript",
        });
      }
    } catch (error) {
      console.log("[peer-response-diagnostic]", String(error));
    } finally {
      await peerDiagnostic.detach();
    }
  } finally {
    await peerA.context().close();
    await peerB.context().close();
  }

  const envelope = {
    restoreMsFromSnapshotResponse: restore,
    switchLongTasksOver50ms: longTasks,
    worstSwitchFrameMs: worstFrame,
    switchesDispatched: switchCount,
    // Which switch each long task followed, and what the frames over 50ms spent their time on.
    switchAttribution: attributeSwitchLongTasks(switchProbe, appOfWindow),
    tooltipsShownDuringSwitches: switchProbe.tooltipsShown,
    worstPeerConvergenceTaskMs: worstPeerTask,
    // Each peer long task with the frame it ran in, and how long before the bar converged.
    peerAttribution,
  };
  // Surfaced on stdout so completion notes can record the measured numbers.
  console.log(`[perf-envelope] ${JSON.stringify(envelope)}`);
  await testInfo.attach("perf-envelope", {
    body: JSON.stringify(envelope, null, 2),
    contentType: "application/json",
  });

  // Envelope: no shell frame beyond 50ms while switching tabs, no peer thrash.
  expect(worstFrame).toBeLessThanOrEqual(50);
  expect(worstPeerTask).toBeLessThanOrEqual(50);
});

/** A long animation frame over 50ms, with its slowest scripts. */
interface LongFrameRecord {
  start: number;
  duration: number;
  blockingDuration: number;
  /** Time from the frame's render start to its end: rAF, style, layout, paint. */
  renderMs: number | null;
  /** Time from style and layout start to the frame's end. */
  styleAndLayoutMs: number | null;
  scripts: Array<{
    invoker: string;
    invokerType: string;
    source: string;
    duration: number;
    forcedStyleAndLayoutMs: number;
  }>;
}

interface SwitchProbeRecords {
  /** Each tab press, by the lead window of the pressed tab. */
  switches: Array<{ at: number; windowId: string | null }>;
  tasks: Array<{ start: number; duration: number }>;
  /** Tooltip popups mounted during the run. */
  tooltipsShown: number;
  frames: LongFrameRecord[];
}

const SWITCH_FRAMES = "__osSwitchFrames";
const PEER_FRAMES = "__osPeerFrames";

/**
 * Records, page side under `key`, every long animation frame over 50ms with its five slowest
 * scripts and the share of render and of style and layout. It only observes.
 */
async function installLongFrameRecorder(page: Page, key: string): Promise<void> {
  await page.evaluate(key => {
    const recorder = { frames: [] as unknown[], flush: () => {} };
    Reflect.set(window, key, recorder);
    if (!PerformanceObserver.supportedEntryTypes.includes("long-animation-frame")) return;
    const rounded = (value: number) => Math.round(value * 10) / 10;
    const record = (entries: PerformanceEntryList) => {
      for (const entry of entries) {
        if (entry.duration <= 50) continue;
        const frame = entry as PerformanceEntry & {
          blockingDuration: number;
          renderStart: number;
          styleAndLayoutStart: number;
          scripts: Array<{
            invoker: string;
            invokerType: string;
            sourceURL: string;
            sourceFunctionName: string;
            sourceCharPosition: number;
            duration: number;
            forcedStyleAndLayoutDuration: number;
          }>;
        };
        const end = frame.startTime + frame.duration;
        recorder.frames.push({
          start: rounded(frame.startTime),
          duration: rounded(frame.duration),
          blockingDuration: rounded(frame.blockingDuration),
          renderMs: frame.renderStart > 0 ? rounded(end - frame.renderStart) : null,
          styleAndLayoutMs:
            frame.styleAndLayoutStart > 0 ? rounded(end - frame.styleAndLayoutStart) : null,
          scripts: [...frame.scripts]
            .sort((left, right) => right.duration - left.duration)
            .slice(0, 5)
            .map(script => ({
              invoker: script.invoker,
              invokerType: script.invokerType,
              source: `${script.sourceURL.split("/").pop() ?? ""}:${script.sourceFunctionName}@${script.sourceCharPosition}`,
              duration: rounded(script.duration),
              forcedStyleAndLayoutMs: rounded(script.forcedStyleAndLayoutDuration),
            })),
        });
      }
    };
    const observer = new PerformanceObserver(list => record(list.getEntries()));
    observer.observe({ type: "long-animation-frame", buffered: false });
    recorder.flush = () => record(observer.takeRecords());
  }, key);
}

/** The frames recorded under `key`, including those the observer has not delivered yet. */
async function readLongFrames(page: Page, key: string): Promise<LongFrameRecord[]> {
  return await page.evaluate(key => {
    const recorder = Reflect.get(window, key) as
      | { frames: LongFrameRecord[]; flush: () => void }
      | undefined;
    if (!recorder) return [];
    recorder.flush();
    return recorder.frames;
  }, key);
}

/** The frame a long task ran in, if one was recorded. */
function frameOfTask(
  frames: readonly LongFrameRecord[],
  task: { start: number }
): LongFrameRecord | null {
  return (
    frames.find(
      entry => entry.start <= task.start + 1 && task.start < entry.start + entry.duration
    ) ?? null
  );
}

/**
 * Records, page side, every tab press of the switching run with the long tasks and long
 * animation frames it produced. It only observes: the envelope's assertions read the same
 * long-task durations as before.
 */
async function installSwitchProbe(page: Page): Promise<void> {
  await page.evaluate(() => {
    const records = {
      switches: [] as Array<{ at: number; windowId: string | null }>,
      tasks: [] as Array<{ start: number; duration: number }>,
      tooltipsShown: 0,
    };
    Reflect.set(window, "__osSwitchProbe", records);
    // A tooltip popup mounted while switching: presses that open and position a popup.
    new MutationObserver(changes => {
      for (const change of changes) {
        for (const node of change.addedNodes) {
          if (!(node instanceof Element)) continue;
          if (node.matches('[data-slot="tooltip-content"]')) records.tooltipsShown += 1;
          else
            records.tooltipsShown += node.querySelectorAll('[data-slot="tooltip-content"]').length;
        }
      }
    }).observe(document.body, { childList: true, subtree: true });
    document.addEventListener(
      "pointerdown",
      event => {
        const tab = event.target instanceof Element ? event.target.closest('[role="tab"]') : null;
        if (!(tab instanceof HTMLElement)) return;
        records.switches.push({
          at: event.timeStamp,
          windowId: tab.dataset.leadWindowId ?? null,
        });
      },
      { capture: true }
    );
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        records.tasks.push({ start: entry.startTime, duration: entry.duration });
      }
    }).observe({ type: "longtask", buffered: false });
  });
  await installLongFrameRecorder(page, SWITCH_FRAMES);
}

async function readSwitchProbe(page: Page): Promise<SwitchProbeRecords> {
  // Read at once, as the envelope always has: the measured window ends with the last press.
  const probe = await page.evaluate(
    () => Reflect.get(window, "__osSwitchProbe") as Omit<SwitchProbeRecords, "frames">
  );
  return { ...probe, frames: await readLongFrames(page, SWITCH_FRAMES) };
}

/** Each long task with the tab press it followed (the latest press at or before its start). */
function attributeSwitchLongTasks(
  probe: SwitchProbeRecords,
  appOfWindow: ReadonlyMap<string, string>
) {
  return probe.tasks.map(task => {
    let pressIndex = -1;
    probe.switches.forEach((entry, index) => {
      if (entry.at <= task.start) pressIndex = index;
    });
    const press = pressIndex < 0 ? undefined : probe.switches[pressIndex];
    const previous = pressIndex > 0 ? probe.switches[pressIndex - 1] : undefined;
    const frame = frameOfTask(probe.frames, task);
    const appOf = (windowId: string | null | undefined) =>
      windowId ? (appOfWindow.get(windowId) ?? windowId) : null;
    return {
      durationMs: Math.round(task.duration),
      afterSwitchTo: appOf(press?.windowId),
      switchedFrom: appOf(previous?.windowId),
      msAfterPress: press === undefined ? null : Math.round(task.start - press.at),
      pressIndex,
      frame,
    };
  });
}

async function prepareShell(page: Page, runtime: BrowserRuntime): Promise<WorkspacePayload> {
  await completeOnboardingIfPrompted(page);
  await expect(page.getByTestId("os-desktop")).toBeVisible();
  await page.getByRole("menuitem", { name: "Window", exact: true }).click();
  const closeWindow = page.getByTestId("os-menubar-command-window.close");
  await expect(closeWindow).toBeVisible();
  await expect(closeWindow).not.toContainText("requires an attached shell");
  await page.keyboard.press("Escape");
  const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>("/api/workspaces");
  const workspace = runtime.seeded.workspace ?? payload.workspaces[0];
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

/**
 * Records long tasks on the peer until its tab bar renders `expectedTabs` tabs, i.e. until a
 * remote layout command has converged into this client's bar.
 */
interface PeerConvergenceRecords {
  tasks: Array<{ start: number; duration: number }>;
  /** When the bar showed the expected tabs, two frames later. */
  convergedAt: number;
  frames: LongFrameRecord[];
}

async function installPeerConvergenceProbe(page: Page, expectedTabs: number): Promise<void> {
  await page.evaluate(expectedTabs => {
    const bar = document.querySelector<HTMLElement>('[data-testid="os-tab-bar"]');
    if (!bar) throw new Error("the tab bar must be mounted before measurement");

    const tasks: Array<{ start: number; duration: number }> = [];
    const push = (entries: PerformanceEntryList) => {
      for (const entry of entries) tasks.push({ start: entry.startTime, duration: entry.duration });
    };
    const longTasks = new PerformanceObserver(list => push(list.getEntries()));
    longTasks.observe({ type: "longtask", buffered: false });

    let resolveProbe: (records: Omit<PeerConvergenceRecords, "frames">) => void = () => {};
    const result = new Promise<Omit<PeerConvergenceRecords, "frames">>(resolve => {
      resolveProbe = resolve;
    });
    Reflect.set(window, "__osPeerConvergence", result);

    const converged = () => bar.querySelectorAll('[role="tab"]').length === expectedTabs;
    const mutations = new MutationObserver(() => {
      if (!converged()) return;
      mutations.disconnect();
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          push(longTasks.takeRecords());
          longTasks.disconnect();
          resolveProbe({ tasks, convergedAt: performance.now() });
        });
      });
    });
    mutations.observe(bar, { childList: true, subtree: true });
  }, expectedTabs);
  await installLongFrameRecorder(page, PEER_FRAMES);
}

async function readPeerConvergenceProbe(page: Page): Promise<PeerConvergenceRecords> {
  const probe = await page.evaluate(async () => {
    const result = Reflect.get(window, "__osPeerConvergence") as
      | Promise<Omit<PeerConvergenceRecords, "frames">>
      | undefined;
    if (!result) throw new Error("peer convergence probe must be installed before it is read");
    return await result;
  });
  // The measured window ends where the long tasks stopped being collected.
  const frames = (await readLongFrames(page, PEER_FRAMES)).filter(
    frame => frame.start <= probe.convergedAt
  );
  return { ...probe, frames };
}

/** Each peer long task with the frame it ran in and how long before the bar converged. */
function attributePeerLongTasks(probe: PeerConvergenceRecords, peer: string) {
  return probe.tasks.map(task => ({
    peer,
    durationMs: Math.round(task.duration),
    msBeforeConverged: Math.round(probe.convergedAt - task.start),
    frame: frameOfTask(probe.frames, task),
  }));
}

/** Window scopes holding sessions-rail state in the persisted per-window store. */
async function persistedSessionRailScopes(page: Page): Promise<string[]> {
  return await page.evaluate(() => {
    const raw = window.localStorage.getItem("compozy:session:sidebar:v1");
    if (raw === null) return [];
    const parsed = JSON.parse(raw) as { context?: { byScope?: Record<string, unknown> } };
    return Object.keys(parsed.context?.byScope ?? {});
  });
}

async function focusWindow(page: Page, win: ReturnType<Page["locator"]>) {
  const head = win.locator('[data-slot="os-window-head"]');
  const box = await head.boundingBox();
  if (!box) throw new Error("window head must have a visible bounding box before focusing");
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
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
    key => window.sessionStorage.getItem(key)?.trim() ?? "",
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

type SplitPlacement = "left" | "right" | "top" | "bottom";

/** `compozy window move`: a tab after the target's tab, or a split at the target's edge. */
async function moveWindowFromCLI(
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string,
  targetWindowId: string,
  placement: "tab" | SplitPlacement,
  desktopId = "desktop-default"
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
    "--target",
    targetWindowId,
    "--placement",
    placement,
  ]);
}

/** `compozy layout arrange`: the windows share one horizontal split tab. */
async function arrangeSplitFromCLI(
  runtime: BrowserRuntime,
  workspaceId: string,
  desktopId: string,
  windowIds: readonly string[]
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
  args.push("--arrangement", "horizontal");
  await runWindowManagerCLI(runtime, args);
}

/** Moves `windowId` out of its tab into a split at `placement` of `targetWindowId`. */
async function splitWindowBeside(
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string,
  targetWindowId: string,
  placement: SplitPlacement
): Promise<void> {
  const snapshot = await windowManagerSnapshot(runtime, workspaceId);
  const destination = snapshot.windows[targetWindowId]?.desktop_id;
  if (!destination) throw new Error(`window ${targetWindowId} has no desktop`);
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "window.move",
    payload: {
      window_id: windowId,
      destination_desktop_id: destination,
      target_window_id: targetWindowId,
      placement,
      move_group: false,
    },
  });
}

/** Moves the first seam of the split holding `windowId` by `delta` of the split's extent. */
async function resizeSplitOf(
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string,
  delta: number
): Promise<void> {
  const snapshot = await windowManagerSnapshot(runtime, workspaceId);
  const split = splitHoldingWindow(snapshot, windowId);
  if (!split) throw new Error(`window ${windowId} is not in a split`);
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "layout.resize",
    payload: { split_id: split.id, boundary_index: 0, delta },
  });
}

/**
 * Creates opaque window identities through the real command endpoint. Test
 * fixtures must never infer IDs from `app`, because multi-instance apps and
 * reopened tabs deliberately share the same app identity. Without a client the
 * tabs land in the bar but no client's active tab changes.
 */
let tabFixtureSequence = 0;

async function openFixtureWindows(
  runtime: BrowserRuntime,
  workspaceId: string,
  apps: readonly string[],
  clientId?: string
): Promise<string[]> {
  const batch = ++tabFixtureSequence;
  const ids: string[] = [];
  for (const [index, app] of apps.entries()) {
    const id = `e2e-tab-b${batch}-${app}-${index}`;
    await executeWindowManagerCommand(
      runtime,
      workspaceId,
      {
        commandId: "window.open",
        payload: {
          window: {
            id,
            app,
            route: { pathname: app === "dashboard" ? "/" : `/${app}`, search: {} },
            desktop_id: "desktop-default",
          },
        },
      },
      clientId
    );
    ids.push(id);
  }
  return ids;
}

async function openSessionWindowInAuthority(
  runtime: BrowserRuntime,
  workspaceId: string,
  session: { id: string; agent_name: string }
): Promise<string> {
  const id = `e2e-tab-session-${session.id}`;
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
      },
    },
  });
  return id;
}

/** Activates the tab holding `windowId` by clicking it in the bar. */
async function showTab(
  page: Page,
  runtime: BrowserRuntime,
  workspaceId: string,
  windowId: string
): Promise<void> {
  const tabId = tabForWindow(await windowManagerSnapshot(runtime, workspaceId), windowId)?.id;
  if (!tabId) throw new Error(`window ${windowId} is in no tab`);
  const tab = osShellSelectors(page).tab(tabId);
  await tab.click();
  await expect(tab).toHaveAttribute("aria-selected", "true");
}

function nodeWindowIds(node: WindowManagerLayoutNode): string[] {
  if (node.kind === "leaf") return node.window_id ? [node.window_id] : [];
  return (node.children ?? []).flatMap(nodeWindowIds);
}

function tabForWindow(snapshot: WindowManagerSnapshot, windowId: string): WindowManagerTab | null {
  for (const desktop of snapshot.desktops) {
    const tab = desktop.tabs.find(candidate => nodeWindowIds(candidate.root).includes(windowId));
    if (tab) return tab;
  }
  return null;
}

function desktopTabIds(snapshot: WindowManagerSnapshot, desktopId: string): string[] {
  const desktop = snapshot.desktops.find(candidate => candidate.id === desktopId);
  if (!desktop) throw new Error(`desktop ${desktopId} is missing from authority`);
  return desktop.tabs.map(tab => tab.id);
}

/** The innermost split node whose children hold `windowId`. */
function splitHoldingWindow(
  snapshot: WindowManagerSnapshot,
  windowId: string
): WindowManagerLayoutNode | null {
  const visit = (node: WindowManagerLayoutNode): WindowManagerLayoutNode | null => {
    if (node.kind !== "split") return null;
    for (const child of node.children ?? []) {
      const nested = visit(child);
      if (nested) return nested;
    }
    return (node.children ?? []).some(
      child => child.kind === "leaf" && child.window_id === windowId
    )
      ? node
      : null;
  };
  const tab = tabForWindow(snapshot, windowId);
  return tab ? visit(tab.root) : null;
}

async function requiredBox(locator: Locator): Promise<{
  x: number;
  y: number;
  width: number;
  height: number;
}> {
  const box = await locator.boundingBox();
  if (!box) throw new Error("the drag fixture needs a visible element");
  return box;
}

async function boxCenter(locator: Locator): Promise<{ x: number; y: number }> {
  const box = await requiredBox(locator);
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

/** Presses at `point` and moves past the drag threshold, leaving the button down. */
async function startPointerDrag(page: Page, point: { x: number; y: number }): Promise<void> {
  await page.mouse.move(point.x, point.y);
  await page.mouse.down();
  await page.mouse.move(point.x + 8, point.y + 2, { steps: 4 });
}

async function movePointer(page: Page, point: { x: number; y: number }): Promise<void> {
  await page.mouse.move(point.x, point.y, { steps: 12 });
}

/** The active desktop's tab IDs in rendered bar order. */
async function barTabIds(page: Page): Promise<string[]> {
  return osShellSelectors(page).tabs.evaluateAll(tabs =>
    tabs.map(tab => (tab.getAttribute("data-testid") ?? "").replace(/^os-tab-/, ""))
  );
}

async function assertDesktopPagerLayout(page: Page): Promise<void> {
  const pager = page.getByRole("navigation", { name: "Desktops" });
  const bar = page.getByRole("banner", { name: "System bar" });
  const dock = page.getByRole("navigation", { name: "Dock" });
  const desktopControls = pager.getByRole("button", { name: /^Desktop \d+ of \d+:/ });
  const [pagerBox, barBox, dockBox, firstBox, secondBox] = await Promise.all([
    pager.boundingBox(),
    bar.boundingBox(),
    dock.boundingBox(),
    desktopControls.nth(0).boundingBox(),
    desktopControls.nth(1).boundingBox(),
  ]);
  if (!pagerBox || !barBox || !dockBox || !firstBox || !secondBox) {
    throw new Error("desktop pager, topbar, and rail must expose measurable bounds");
  }
  const firstCenterY = firstBox.y + firstBox.height / 2;
  const secondCenterY = secondBox.y + secondBox.height / 2;
  const barCenterY = barBox.y + barBox.height / 2;
  const alignmentTolerance = Math.max(2, barBox.height * 0.1);

  // The pager is a row of dots in the topbar tray, clear of the left rail.
  expect(pagerBox.y).toBeGreaterThanOrEqual(barBox.y);
  expect(pagerBox.y + pagerBox.height).toBeLessThanOrEqual(barBox.y + barBox.height);
  expect(pagerBox.x).toBeGreaterThan(dockBox.x + dockBox.width);
  expect(firstBox.x + firstBox.width).toBeLessThanOrEqual(secondBox.x);
  expect(Math.abs(firstCenterY - secondCenterY)).toBeLessThanOrEqual(alignmentTolerance);
  expect(Math.abs(firstCenterY - barCenterY)).toBeLessThanOrEqual(alignmentTolerance);
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
  title: string,
  workspaceId: string
): Promise<{ id: string; title: string }> {
  const payload = await runtime.requestJSON<{ task: { id: string; title: string } }>("/api/tasks", {
    method: "POST",
    body: JSON.stringify({
      approval_policy: "manual",
      description: "OS shell attention E2E approval fixture.",
      owner: { kind: "human", ref: "os-shell-operator" },
      priority: "high",
      scope: "workspace",
      title,
      workspace: workspaceId,
    }),
  });
  return payload.task;
}

async function createTask(
  runtime: BrowserRuntime,
  title: string,
  workspaceId?: string
): Promise<{ id: string; title: string }> {
  const payload = await runtime.requestJSON<{ task: { id: string; title: string } }>("/api/tasks", {
    method: "POST",
    body: JSON.stringify({
      description: "OS shell window-controller E2E fixture.",
      owner: { kind: "human", ref: "os-shell-operator" },
      priority: "medium",
      scope: workspaceId ? "workspace" : "global",
      title,
      ...(workspaceId ? { workspace: workspaceId } : {}),
    }),
  });
  return payload.task;
}

async function createPaletteLoop(
  runtime: BrowserRuntime,
  workspaceId: string,
  name: string
): Promise<void> {
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspaceId)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({
      definition: {
        apiVersion: "compozy.loop/v1",
        kind: "Loop",
        meta: {
          name,
          description: "Command palette direct-route fixture.",
          catalog: { category: "Testing" },
        },
        concurrency: "allow",
        contract: {
          goal: "Open one concrete loop from the command palette.",
          definition_of_done: "The loop is available in the workspace catalog.",
          stop_when: "nodes.finish.status == 'succeeded'",
          iteration_cap: 1,
          no_progress: { window: 2 },
          budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
          terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
        },
        graph: {
          nodes: [
            {
              id: "finish",
              class: "action",
              kind: "transform",
              params: { map: { ready: { value: true } } },
            },
          ],
          edges: [],
        },
        start: [{ kind: "http" }],
      },
    }),
  });
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

type MenubarMenu = "CompozyOS" | "Session" | "Go" | "Window" | "Help";

/** Menubar triggers are `role="menuitem"` inside the shell's single menubar. */
async function openMenu(page: Page, name: MenubarMenu): Promise<void> {
  const trigger = page.getByRole("menuitem", { name, exact: true });
  await trigger.click();
  const menuTestID = name === "CompozyOS" ? "os-menu-compozy" : `os-menu-${name.toLowerCase()}`;
  await expect(page.getByTestId(menuTestID)).toBeVisible();
  // A popup already closing is still visible: the trigger says whether the menu is open.
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
}

/** Creates a session through Session > New session and resolves its id. */
async function createSessionFromMenubar(page: Page): Promise<string> {
  await openMenu(page, "Session");
  await page.getByRole("menuitem", { name: /^New session/ }).click();
  const createDialog = page.getByTestId("session-create-dialog");
  await expect(createDialog).toBeVisible();
  await createDialog.getByTestId("session-create-mode-advanced").click();
  await createDialog.getByTestId("session-create-agent-select").click();
  await page.getByTestId(`agent-command-item-${browserLifecycleAgent}`).click();
  const created = page.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await createDialog.getByTestId("session-create-submit").click();
  const response = await created;
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as { session?: { id?: string } };
  const sessionId = payload.session?.id ?? "";
  expect(sessionId).not.toBe("");
  await expect(createDialog).toHaveCount(0);
  return sessionId;
}

async function openSessionsCatalog(page: Page): Promise<void> {
  await openMenu(page, "Session");
  await page.getByTestId("os-menubar-command-shell.sessions.toggle").click();
  await expect(page.getByTestId("os-sessions-modal")).toBeVisible();
}

async function openSessionsCatalogFromPalette(page: Page): Promise<void> {
  const palette = await openCommandPalette(page);
  const search = palette.getByPlaceholder("Search apps, sessions, and actions…");
  await search.fill("Toggle sessions");
  await palette.getByTestId("os-palette-command-shell.sessions.toggle").click();
  await expect(page.getByTestId("os-sessions-modal")).toBeVisible();
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
    const workspaceRow = appPage.getByTestId(`os-workspace-option-${workspace.id}`);
    await expect(workspaceRow).toHaveAttribute("aria-haspopup", "menu");
    await workspaceRow.press("ArrowRight");
    const submenu = appPage.getByTestId(`os-worktree-submenu-${workspace.id}`);
    await expect(submenu).toBeVisible();
    const menuRow = submenu.getByTestId(/^os-worktree-option-/).first();
    await expect(menuRow).toBeVisible();
    await expect(menuRow).toContainText("payments-retry");

    await menuRow.focus();
    await appPage.keyboard.press("Enter");

    // The chip states where new work will run: `workspace / worktree`.
    await expect(appPage.locator('[data-slot="os-menubar-worktree"]')).toHaveText(
      "/ payments-retry"
    );

    // A fresh window inherits the shell selection and issues scoped reads.
    await openDockApp(appPage, "Tasks", "tasks");
    await expect(appPage.locator('[data-slot="os-menubar-worktree"]')).toHaveText(
      "/ payments-retry"
    );

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

// Presence across update states belongs to menubar-update-indicator.test.tsx.
// This journey owns the real shell wiring: one activation of the menubar pill,
// by keyboard or pointer, applies every eligible target without a Settings detour.
test("E2E-021 / E2E-022: the menubar Update pill updates CompozyOS in one activation by keyboard or pointer", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(sessionLifecycleSelectors(appPage));

  let updatePayload: unknown = settingsUpdateBothAvailableFixture;
  const applyRequests: string[][] = [];
  await appPage.route("**/api/settings/update", async route => {
    await route.fulfill({ json: updatePayload });
  });
  await appPage.route("**/api/settings/update/apply", async route => {
    const body = route.request().postDataJSON() as { targets: string[] };
    applyRequests.push(body.targets);
    // Apply only acknowledges acquisition; the GET above becomes the progress feed.
    updatePayload = settingsUpdateApplyingFixture;
    await route.fulfill({
      json: {
        targets: body.targets,
        status: "accepted",
        operation_id: "op-e2e-menubar",
        message: "Started the requested updates.",
        holder: null,
      },
    });
  });

  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("os-desktop")).toBeVisible({ timeout: 20_000 });

  const indicator = appPage.getByTestId("os-menubar-update");
  await expect(indicator).toHaveText("Update", { timeout: 20_000 });

  // Keyboard: focus without a pointer, then activate.
  await indicator.focus();
  await expect(indicator).toBeFocused();
  await appPage.keyboard.press("Enter");
  await expect.poll(() => applyRequests).toEqual([["runtime", "app"]]);
  await expect(indicator).toHaveAttribute("data-state", "updating", { timeout: 20_000 });
  await expect(indicator).toBeDisabled();
  // One click means no detour: Settings never opens.
  await expect(appWindow(appPage, "settings")).toHaveCount(0);

  // Pointer: a fresh offer applies the same way.
  updatePayload = settingsUpdateBothAvailableFixture;
  await expect(indicator).toHaveAttribute("data-state", "available", { timeout: 20_000 });
  await indicator.click();
  await expect.poll(() => applyRequests).toHaveLength(2);
  expect(applyRequests[1]).toEqual(["runtime", "app"]);
  await expect(appWindow(appPage, "settings")).toHaveCount(0);
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
  await expect(paletteRowReason(palette, "window.close")).toHaveText(
    "CompozyOS isn't reachable right now"
  );
  await expect(closeRow).toHaveAccessibleDescription("CompozyOS isn't reachable right now");
  await palette.getByPlaceholder("Search apps, sessions, and actions…").press("Home");
  await expect(closeRow).toHaveAttribute("data-selected", "true");
  await appPage.keyboard.press("Enter");
  await expect(appPage.locator("[data-sonner-toast]:last-of-type")).toContainText(
    "Close window — CompozyOS isn't reachable right now"
  );
  await expect(palette).toBeVisible();

  await appPage.keyboard.press("ControlOrMeta+KeyK");
  const unavailablePanel = appPage.getByTestId("os-palette-action-panel");
  await expect(unavailablePanel.getByTestId("os-palette-action-primary.run")).toHaveCount(0);
  await expect(unavailablePanel.getByTestId("os-palette-action-meta.pin")).toBeVisible();
  await expect(unavailablePanel.getByTestId("os-palette-action-reason")).toHaveText(
    "CompozyOS isn't reachable right now"
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
  // Recovery replaces the transport failure with the live context: a desktop always holds a
  // tab, so this fresh shell focuses its New Tab and Close window is available again.
  const reopened = await openCommandPalette(appPage);
  await expect(paletteRow(reopened, "window.close")).toBeEnabled();
  await expect(paletteRowReason(reopened, "window.close")).toHaveCount(0);
  await expect(paletteRow(reopened, "window.close")).toHaveAccessibleDescription("");
});

test("E2E-019 / E2E-017: menubar matches the palette and overlays unwind above the bell", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);

  await test.step("E2E-019: menubar items project the same id, label, chord and reason as the palette row", async () => {
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

  await test.step("E2E-017: palette unwinds above the bell one overlay at a time", async () => {
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
});
