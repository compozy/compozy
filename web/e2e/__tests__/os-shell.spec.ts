import { execFile } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Browser, Locator, Page } from "@playwright/test";

import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { tasksOperatorSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const browserLifecycleAgent = "os-shell-agent";
const windowManagerClientStorageKey = "agh.window-manager.client-id";
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

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: browserLifecycleAgent,
          fixtureAgent: "os-shell-multi-session-agent",
          fixturePath: browserLifecycleFixture,
        },
      ],
    },
  },
});

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

test("E2E-001: fresh boot renders the empty desktop without opening a window", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);

  await expect(appPage.getByRole("navigation", { name: "Dock" })).toBeVisible();
  await expect(appPage.getByRole("banner", { name: "System bar" })).toBeVisible();
  await expect(appPage.getByTestId("os-desk-hint")).toContainText("⌘K");
  await expect(appPage.locator('[data-testid^="os-window-"]')).toHaveCount(0);
  await expect(appPage.getByRole("button", { name: "Desktop 1 of 1: Desktop 1" })).toHaveAttribute(
    "aria-current",
    "page"
  );

  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  expect(snapshot.version).toBe(2);
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
  await expect(appPage).toHaveURL(/\/tasks$/);
  const opened = await authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks");

  await dragWindowBy(appPage, tasks, 92, 48);
  await expect
    .poll(() => authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks"))
    .not.toEqual(opened);
  const dragged = await authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks");
  await expect.poll(() => windowRect(appPage, tasks)).toEqual(dragged);
  expect((await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"]?.placement).toBe(
    "floating"
  );

  await appPage.reload({ waitUntil: "domcontentloaded" });
  const restored = appPage.getByTestId("os-window-app:tasks");
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
  await arrangeWindows(
    runtime,
    workspace.id,
    "desktop-default",
    ["app:tasks", "app:settings"],
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
      return focusDesktop?.focus_owner === "app:tasks" &&
        snapshot.windows["app:tasks"]?.desktop_id === focusDesktop.id
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
      return snapshot.windows["app:tasks"]?.desktop_id;
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
  await expect(appPage.getByTestId("os-window-app:tasks")).toBeVisible();

  await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(task.id)}`), {
    waitUntil: "domcontentloaded",
  });
  const tasksUI = tasksOperatorSelectors(appPage);
  const tasksWindow = appPage.getByTestId("os-window-app:tasks");
  await expect(tasksUI.detailContent).toBeVisible();
  const windowPath = tasksWindow.getByRole("navigation", { name: "Window path" });
  await expect(windowPath.getByRole("button", { name: "Tasks", exact: true })).toBeVisible();
  await expect(tasksWindow.getByTestId("tasks-detail-title")).toContainText(task.title);
  await expect(tasksWindow.locator('[data-slot="topbar-title"]')).toContainText(task.title);
  await expect(windowPath).not.toContainText(/^agh\b/);

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
      "app:tasks",
      "--pathname",
      pathname,
      "--search-json",
      JSON.stringify(search),
    ]);

    await expect
      .poll(
        async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"]?.route
      )
      .toEqual({ pathname, search });
    await expect(tasks.getByTestId("tasks-detail-title")).toContainText(task.title);
    await expect(
      peer.getByTestId("os-window-app:tasks").getByTestId("tasks-detail-title")
    ).toContainText(task.title);
    await expect.poll(() => currentRoute(appPage)).toEqual({ pathname, search });

    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(
      appPage.getByTestId("os-window-app:tasks").getByTestId("tasks-detail-title")
    ).toContainText(task.title);
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
    await expect(
      appPage.getByTestId("os-window-app:tasks").getByTestId("tasks-detail-title")
    ).toContainText(task.title);
    await expect.poll(() => currentRoute(appPage)).toEqual({ pathname, search });
    await expect
      .poll(
        async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"]?.route
      )
      .toEqual({ pathname, search });
  } finally {
    await peer.context().close();
  }
});

test("E2E-006: two session windows stream independently through minimize and restore", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const primary = await createNamedSession(runtime, workspace.id, "Primary observer");
  const secondary = await createNamedSession(runtime, workspace.id, "Secondary responder");

  await appPage.getByRole("button", { name: "Sessions" }).click();
  const sessionsModal = appPage.getByTestId("os-sessions-modal");
  await expect(sessionsModal).toBeVisible();
  await sessionsModal.getByTestId(`os-sessions-modal-session-${primary.id}`).first().click();
  await expect(sessionsModal).toHaveCount(0);
  await appPage.getByRole("button", { name: "Sessions" }).click();
  await expect(sessionsModal).toBeVisible();
  await sessionsModal.getByTestId(`os-sessions-modal-session-${secondary.id}`).first().click();
  await expect(sessionsModal).toHaveCount(0);

  const primaryWindow = appPage.getByTestId(`os-window-session:${primary.id}`);
  const secondaryWindow = appPage.getByTestId(`os-window-session:${secondary.id}`);
  await expect(primaryWindow).toBeVisible();
  await expect(secondaryWindow).toBeVisible();

  await moveWindowToNormalizedRect(runtime, workspace.id, `session:${primary.id}`, {
    x: 0.01,
    y: 0.02,
    width: 0.48,
    height: 0.78,
  });
  await moveWindowToNormalizedRect(runtime, workspace.id, `session:${secondary.id}`, {
    x: 0.51,
    y: 0.02,
    width: 0.48,
    height: 0.78,
  });
  await expect
    .poll(() =>
      windowMatchesAuthority(appPage, primaryWindow, runtime, workspace.id, `session:${primary.id}`)
    )
    .toBe(true);
  await expect
    .poll(() =>
      windowMatchesAuthority(
        appPage,
        secondaryWindow,
        runtime,
        workspace.id,
        `session:${secondary.id}`
      )
    )
    .toBe(true);
  const [primaryBox, secondaryBox] = await Promise.all([
    primaryWindow.boundingBox(),
    secondaryWindow.boundingBox(),
  ]);
  if (!primaryBox || !secondaryBox) throw new Error("session windows must have visible bounds");
  expect(primaryBox.x + primaryBox.width).toBeLessThanOrEqual(secondaryBox.x);

  const primaryComposer = primaryWindow.getByTestId("composer-textarea");
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

  const secondaryComposer = secondaryWindow.getByTestId("composer-textarea");
  const secondaryTranscript = secondaryWindow.getByTestId("chat-view");
  await secondaryComposer.fill("reply in secondary window");
  await secondaryComposer.press("Enter");
  await expect(secondaryTranscript).toContainText("Secondary stream started.");
  await expect(secondaryTranscript).toContainText("Secondary stream completed independently.");
  await expect(secondaryWindow).toHaveAttribute("data-focused", "");
  await expect(primaryWindow).not.toHaveAttribute("data-focused", "");
  await expect
    .poll(() => primaryTranscript.evaluate(element => element.scrollTop))
    .toBe(parkedScrollTop);

  await primaryWindow.getByRole("button", { name: "Minimize window" }).click();
  await expect(primaryWindow).toHaveCount(1);
  await expect(primaryWindow).toBeHidden();
  await expect
    .poll(
      async () =>
        (await windowManagerSnapshot(runtime, workspace.id)).windows[`session:${primary.id}`]
    )
    .toMatchObject({ minimized: true });
  await expect
    .poll(() => sessionHistoryContains(runtime, workspace.id, primary.id, "arrived while"))
    .toBe(true);

  await appPage.getByRole("button", { name: "Sessions" }).click();
  const restoredModal = appPage.getByTestId("os-sessions-modal");
  await expect(restoredModal).toBeVisible();
  await restoredModal.getByTestId(`os-sessions-modal-session-${primary.id}`).first().click();
  const restoredPrimary = appPage.getByTestId(`os-window-session:${primary.id}`);
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
  const search = palette.getByPlaceholder("Search apps, sessions, actions…");
  await search.fill("tasks");
  await search.press("Enter");
  await expect(appPage.getByTestId("os-window-app:tasks")).toBeVisible();

  await appPage.keyboard.press("ControlOrMeta+K");
  await search.fill("Palette target");
  await expect(palette.getByTestId(`os-palette-session-${session.id}`)).toBeVisible();
  await search.press("Enter");
  const sessionWindow = appPage.getByTestId(`os-window-session:${session.id}`);
  await expect(sessionWindow).toBeVisible();
  await expect(palette).toHaveCount(0);
  const composer = sessionWindow.getByTestId("composer-textarea");
  await expect(composer).toBeVisible();
  await composer.focus();
  await appPage.keyboard.press("ControlOrMeta+K");
  await expect(palette).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(palette).toHaveCount(0);

  await openMenu(appPage, "Session");
  await appPage.getByTestId("os-menu-new-session").click();
  const createDialog = appPage.getByTestId("session-create-dialog");
  await expect(createDialog).toBeVisible();
  const runtimeTrigger = createDialog.getByTestId("session-create-runtime-select");
  await runtimeTrigger.focus();
  await appPage.keyboard.press("ControlOrMeta+J");
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
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
    const secondWindow = second.getByTestId("os-window-app:tasks");
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
      "app:tasks",
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
          "app:tasks"
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
  await useGlobalWorkspaceIfPrompted(degradedPage);

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
  await expect(degradedPage.getByTestId("os-window-app:tasks")).toBeVisible();
  await expect
    .poll(() => windowPosition(degradedPage, degradedPage.getByTestId("os-window-app:tasks")))
    .toEqual(recovered);
});

test("E2E-014: CLI window move commits semantic normalized geometry live", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const target = { x: 0.42, y: 0.18, width: 0.4, height: 0.52 };

  await moveWindowFromCLI(runtime, workspace.id, "app:tasks", "desktop-default", target);

  await expect
    .poll(() => windowMatchesAuthority(appPage, tasks, runtime, workspace.id, "app:tasks"))
    .toBe(true);
  const snapshot = await windowManagerSnapshot(runtime, workspace.id);
  expect(snapshot.windows["app:tasks"]?.floating_rect).toEqual(target);
  expect(snapshot.windows["app:tasks"]?.placement).toBe("floating");
});

test("E2E-015: bell approval stays live and a CLI-resolved item reports truthful conflict", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  const tasksUI = tasksOperatorSelectors(appPage);
  const first = await createApprovalTask(runtime, "Primary approval");
  const bell = appPage.getByRole("button", { name: "Approvals" });

  await expect(bell).toHaveText("1");
  await bell.click();
  const firstAttentionRow = appPage.getByTestId(`os-attention-task-${first.id}`);
  await expect(firstAttentionRow).toContainText(first.title);
  await firstAttentionRow.click();
  const tasksWindow = appPage.getByTestId("os-window-app:tasks");
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
  const sessionWindow = appPage.getByTestId(`os-window-session:${session.id}`);
  const composer = sessionWindow.getByTestId("composer-textarea");
  await composer.fill("observe primary stream");
  await composer.press("Enter");
  await expect(sessionWindow.getByTestId("chat-view")).toContainText(
    "Primary stream is warming up."
  );

  await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(task.id)}`), {
    waitUntil: "domcontentloaded",
  });
  const tasksWindow = appPage.getByTestId("os-window-app:tasks");
  const tasksUI = tasksOperatorSelectors(appPage);
  await expect(tasksUI.detailContent).toBeVisible();
  await moveWindowToNormalizedRect(runtime, workspace.id, `session:${session.id}`, {
    x: 0.55,
    y: 0.03,
    width: 0.42,
    height: 0.76,
  });
  await moveWindowToNormalizedRect(runtime, workspace.id, "app:tasks", {
    x: 0.02,
    y: 0.03,
    width: 0.5,
    height: 0.74,
  });
  await expect
    .poll(() => windowMatchesAuthority(appPage, tasksWindow, runtime, workspace.id, "app:tasks"))
    .toBe(true);
  await expect
    .poll(() =>
      windowMatchesAuthority(appPage, sessionWindow, runtime, workspace.id, `session:${session.id}`)
    )
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
    sessionWindow
      .locator('[data-slot="os-window-overlays"]')
      .getByTestId("tasks-detail-delete-dialog")
  ).toHaveCount(0);

  await composer.fill("session remains interactive while Tasks confirms");
  await expect(composer).toHaveValue("session remains interactive while Tasks confirms");

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
  await expect(sessionWindow).toBeVisible();
  await expect(composer).toHaveValue("session remains interactive while Tasks confirms");
});

test("E2E-017: palette unwinds above the bell one overlay at a time", async ({
  appPage,
  runtime,
}) => {
  await prepareShell(appPage, runtime);
  await appPage.getByRole("button", { name: "Approvals" }).click();
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
  const workspace = await prepareShell(appPage, runtime);
  const secondWorkspace = await addSecondWorkspace(runtime);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await appPage.locator('[data-slot="os-menubar-workspace"]').click();
  await expect(appPage.getByTestId(`os-workspace-option-${workspace.id}`)).toBeVisible();
  await appPage.getByTestId(`os-workspace-option-${secondWorkspace.id}`).click();
  await expect(appPage.locator('[data-slot="os-menubar-workspace"]')).toContainText(
    secondWorkspace.name
  );

  await openMenu(appPage, "Session");
  await appPage.getByTestId("os-menu-new-session").click();
  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();
  await appPage.keyboard.press("Escape");

  // One menubar: arrow keys traverse it and hovering a sibling switches menus.
  await openMenu(appPage, "Session");
  await appPage.keyboard.press("ArrowRight");
  await expect(appPage.getByTestId("os-menu-go")).toBeVisible();
  await appPage.getByRole("menuitem", { name: "Help", exact: true }).hover();
  await expect(appPage.getByTestId("os-menu-help")).toBeVisible();
  await expect(appPage.getByTestId("os-menu-go")).toHaveCount(0);
  await appPage.keyboard.press("Escape");

  await openMenu(appPage, "Window");
  await appPage.getByTestId("os-menu-desktops-overview").click();
  const desktops = appPage.locator('[data-slot="desktops-overview"]');
  await expect(desktops).toBeVisible();
  await expect(desktops.getByRole("heading", { name: "Desktops" })).toBeVisible();
  await expect(desktops.getByRole("button", { name: "Current desktop Desktop 1" })).toBeVisible();
  await appPage.keyboard.press("Escape");

  await openMenu(appPage, "Go");
  await appPage.getByTestId("os-menu-workspaces-overview").click();
  const workspaces = appPage.getByTestId("os-workspaces-overview");
  await expect(workspaces).toBeVisible();
  await expect(
    workspaces.getByRole("button", {
      name: new RegExp(`Current workspace ${escapeRegExp(secondWorkspace.name)}`),
    })
  ).toBeVisible();
  await expect(
    workspaces.getByRole("button", {
      name: new RegExp(`Switch to ${escapeRegExp(workspace.name)}`),
    })
  ).toBeVisible();
  await appPage.keyboard.press("Escape");
  await appPage.keyboard.press("ControlOrMeta+Shift+S");
  await expect(desktops).toBeVisible();
  await appPage.keyboard.press("Escape");

  await openMenu(appPage, "Help");
  await appPage.getByTestId("os-menu-shortcuts").click();
  const shortcuts = appPage.getByTestId("os-shortcuts-dialog");
  await expect(shortcuts).toBeVisible();
  // Every registry action is listed, bound or not.
  await expect(shortcuts.getByTestId("os-shortcut-row-window.close")).toContainText("⌘W");
  await expect(shortcuts.getByTestId("os-shortcut-row-window.tile.top")).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(shortcuts).toHaveCount(0);

  await openMenu(appPage, "AGH");
  await appPage.getByTestId("os-menu-about").click();
  const about = appPage.getByTestId("os-about-dialog");
  await expect(about).toBeVisible();
  await expect(about.getByTestId("os-about-row-pid")).not.toBeEmpty();
  await appPage.keyboard.press("Escape");
  await expect(about).toHaveCount(0);

  await appPage.getByRole("button", { name: "Settings" }).click();
  await expect(appPage.getByTestId("os-window-app:settings")).toBeVisible();
});

test("E2E-025: occupied drag previews, cancels cleanly, then commits one structural reflow", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const settings = await openDockApp(appPage, "Settings", "settings");
  await arrangeWindows(
    runtime,
    workspace.id,
    "desktop-default",
    ["app:tasks"],
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
  await appPage.mouse.move(tasksBox.x + tasksBox.width / 2, tasksBox.y + tasksBox.height / 2, {
    steps: 8,
  });
  await expect(preview).toContainText("Add to stack");
  await appPage.keyboard.press("Escape");
  await expect(preview).toHaveCount(0);
  await appPage.mouse.up();
  expect((await windowManagerSnapshot(runtime, workspace.id)).revision).toBe(beforeCancel.revision);
  expect(
    (await windowManagerSnapshot(runtime, workspace.id)).windows["app:settings"]?.placement
  ).toBe("floating");

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
      const group = layoutGroupForWindow(snapshot, "app:tasks");
      return (
        snapshot.revision > beforeCancel.revision &&
        group !== null &&
        nodeWindowIds(group.root).sort().join(",") === "app:settings,app:tasks" &&
        group.root.kind === "split"
      );
    })
    .toBe(true);
  const committed = await windowManagerSnapshot(runtime, workspace.id);
  expect(committed.windows["app:tasks"]?.placement).toBe("tiled");
  expect(committed.windows["app:settings"]?.placement).toBe("tiled");
  const signature = layoutSignature(committed, "desktop-default");

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("os-window-app:tasks")).toBeVisible();
  await expect(appPage.getByTestId("os-window-app:settings")).toBeVisible();
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
  const second = await openPeerPage(browser, runtime);
  try {
    const firstWindow = await openDockApp(appPage, "Tasks", "tasks");
    const secondWindow = second.getByTestId("os-window-app:tasks");
    await expect(secondWindow).toBeVisible();

    await appPage.keyboard.press("Control+Alt+ArrowLeft");
    await expect
      .poll(async () => {
        const snapshot = await windowManagerSnapshot(runtime, workspace.id);
        return {
          placement: snapshot.windows["app:tasks"]?.placement,
          frame: normalizedFrameForWindow(snapshot, "app:tasks"),
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
      ["app:tasks"],
      "horizontal",
      "group-cli-right",
      { x: 0.5, y: 0, width: 0.5, height: 1 }
    );
    await expect
      .poll(async () =>
        normalizedFrameForWindow(await windowManagerSnapshot(runtime, workspace.id), "app:tasks")
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
          Math.abs(first.x - firstLayer.width / 2) <= 1 &&
          Math.abs(peer.x - peerLayer.width / 2) <= 1
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

  await appPage.keyboard.press("ControlOrMeta+K");
  const palette = appPage.getByTestId("os-command-palette");
  await expect(palette).toBeVisible();
  const search = palette.getByPlaceholder("Search apps, sessions, actions…");
  await search.fill("Tile left half");
  await search.press("Enter");
  await expect
    .poll(async () => {
      const snapshot = await windowManagerSnapshot(runtime, workspace.id);
      return {
        placement: snapshot.windows["app:tasks"]?.placement,
        frame: normalizedFrameForWindow(snapshot, "app:tasks"),
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
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"])
    .toMatchObject({ placement: "floating", desktop_id: "desktop-default" });
  await expect(tasks).toHaveAttribute("data-window-placement", "floating");
  await expect
    .poll(() => windowMatchesAuthority(appPage, tasks, runtime, workspace.id, "app:tasks"))
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
      normalizedFrameForWindow(await windowManagerSnapshot(runtime, workspace.id), "app:tasks")
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
  await arrangeWindows(
    runtime,
    workspace.id,
    "desktop-default",
    ["app:tasks", "app:settings"],
    "horizontal",
    "group-resize"
  );
  await expect(tasks).toHaveAttribute("data-window-placement", "tiled");
  await expect(settings).toHaveAttribute("data-window-placement", "tiled");

  const beforeWeights = splitWeightsForWindow(
    await windowManagerSnapshot(runtime, workspace.id),
    "app:tasks"
  );
  if (!beforeWeights) throw new Error("two-up arrangement must expose split weights");
  const beforeTasks = await windowRect(appPage, tasks);
  const beforeSettings = await windowRect(appPage, settings);
  const seam = appPage.getByRole("separator", { name: "Resize boundary 1" });
  await expect(seam).toHaveCount(1);
  const seamBox = await seam.boundingBox();
  if (!seamBox) throw new Error("structural seam must be visible");
  await appPage.mouse.move(seamBox.x + seamBox.width / 2, seamBox.y + seamBox.height / 2);
  await appPage.mouse.down();
  await appPage.mouse.move(seamBox.x + seamBox.width / 2 + 150, seamBox.y + seamBox.height / 2, {
    steps: 8,
  });
  await appPage.mouse.up();
  await expect
    .poll(async () =>
      splitWeightsForWindow(await windowManagerSnapshot(runtime, workspace.id), "app:tasks")
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
    "app:tasks"
  );
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect
    .poll(async () =>
      splitWeightsForWindow(await windowManagerSnapshot(runtime, workspace.id), "app:tasks")
    )
    .toEqual(persistedWeights);
  await expect(appPage.getByTestId("os-window-app:tasks")).toHaveAttribute(
    "data-window-placement",
    "tiled"
  );
});

test("E2E-029: dropping onto a tiled window splits its group and the zoom menu arranges presets", async ({
  appPage,
  runtime,
}) => {
  await appPage.setViewportSize({ width: 1440, height: 900 });
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const settings = await openDockApp(appPage, "Settings", "settings");

  await appPage
    .getByRole("navigation", { name: "Dock" })
    .getByRole("button", { name: "Tasks", exact: true })
    .click();
  await expect(tasks).toHaveAttribute("data-focused", "");
  await appPage.keyboard.press("Control+Alt+ArrowRight");
  await expect
    .poll(async () =>
      normalizedFrameForWindow(await windowManagerSnapshot(runtime, workspace.id), "app:tasks")
    )
    .toEqual({ x: 0.5, y: 0, width: 0.5, height: 1 });

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
      const group = layoutGroupForWindow(snapshot, "app:tasks");
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
      windows: ["app:settings", "app:tasks"],
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
        "app:tasks"
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
      windows: ["app:settings", "app:tasks"],
    });
});

test("E2E-009: the pager and overview keep desktop arrangements independent", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  await dragWindowBy(appPage, tasks, 140, 90);
  const tasksRect = (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"]
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
  await expect(vault).toBeVisible();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:vault"])
    .toMatchObject({ desktop_id: "desktop-e2e-2" });

  await pager.getByRole("button", { name: "Show 3 later desktops" }).click();
  const overview = appPage.locator('[data-slot="desktops-overview"]');
  await expect(overview).toBeVisible();
  await expect(overview.getByRole("button", { name: "Switch to Desktop 6" })).toBeFocused();
  await expect(overview.getByRole("button", { name: "Current desktop Desktop 2" })).toBeVisible();
  await expect(overview.getByRole("list", { name: "Windows on Desktop 1" })).toContainText("Tasks");
  await expect(overview.getByRole("list", { name: "Windows on Desktop 2" })).toContainText("Vault");
  await overview.getByRole("button", { name: "Switch to Desktop 1" }).click();

  const tasksBack = appPage.getByTestId("os-window-app:tasks");
  await expect(tasksBack).toBeVisible();
  await expect(vault).toBeHidden();
  await expect(activeDesktop(appPage, "desktop-default")).toHaveAttribute("data-active", "true");
  expect(
    (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"]?.floating_rect
  ).toEqual(tasksRect);
});

test("E2E-011: the compact stack round-trips with floating rects preserved", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const opened = await authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks");
  await dragWindowBy(appPage, tasks, 120, 80);
  await expect
    .poll(() => authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks"))
    .not.toEqual(opened);
  const floatingRect = await authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks");
  await expect.poll(() => windowRect(appPage, tasks)).toEqual(floatingRect);

  // Below the breakpoint: stacked fullscreen presentation with the tab bar.
  await appPage.setViewportSize({ width: 390, height: 844 });
  await expect(tasks).toHaveAttribute("data-presentation", "compact");
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
  await expect(tasks).not.toHaveAttribute("data-presentation", "compact");
  await expect
    .poll(async () => rectsClose(await windowRect(appPage, tasks), floatingRect))
    .toBe(true);
});

test("E2E-013: appearance preferences stay client-local while minimize remains authoritative", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  await openDockApp(appPage, "Tasks", "tasks");

  await openMenu(appPage, "AGH");
  await appPage.getByTestId("os-menu-appearance").click();
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

  const tasksWindow = appPage.getByTestId("os-window-app:tasks");
  await expect(tasksWindow).toBeVisible();
  await appPage
    .getByRole("navigation", { name: "Dock" })
    .getByRole("button", { name: "Tasks", exact: true })
    .click();
  await expect(tasksWindow).toHaveAttribute("data-focused", "");
  await tasksWindow.getByRole("button", { name: "Minimize window" }).click();
  await expect(tasksWindow).toBeHidden();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"])
    .toMatchObject({ minimized: true });
  await appPage.getByRole("button", { name: "Tasks", exact: true }).click();
  await expect(tasksWindow).toBeVisible();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"])
    .toMatchObject({ minimized: false });
});

test("E2E-016: a cross-workspace session deep link switches workspaces and leaves both topologies intact", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareShell(appPage, runtime);
  const secondWorkspace = await addSecondWorkspace(runtime);
  const session = await createNamedSession(runtime, secondWorkspace.id, "cross-workspace-session");

  // Arrange A so its integrity is checkable after the round trip.
  const tasks = await openDockApp(appPage, "Tasks", "tasks");
  const openedTasks = await authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks");
  await dragWindowBy(appPage, tasks, 130, 70);
  await expect
    .poll(() => authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks"))
    .not.toEqual(openedTasks);
  const arrangedTasks = await authoritativeWindowRect(appPage, runtime, workspace.id, "app:tasks");
  await expect.poll(() => windowRect(appPage, tasks)).toEqual(arrangedTasks);

  // Follow the session link owned by B: the shell switches to workspace B and
  // opens that session focused there — never cross-workspace in place.
  await appPage.goto(
    runtime.url(
      `/agents/${encodeURIComponent(session.agent_name)}/sessions/${encodeURIComponent(session.id)}`
    ),
    { waitUntil: "domcontentloaded" }
  );
  await expect(appPage.locator('[data-slot="os-menubar-workspace"]')).toContainText(
    secondWorkspace.name
  );
  const sessionWindow = appPage.getByTestId(`os-window-session:${session.id}`);
  await expect(sessionWindow).toBeVisible();
  await expect(appPage.getByTestId("os-window-app:tasks")).toHaveCount(0);

  // Switching back shows A untouched: same windows, same position, no leak.
  await appPage.locator('[data-slot="os-menubar-workspace"]').click();
  await appPage.getByTestId(`os-workspace-option-${workspace.id}`).click();
  const tasksBack = appPage.getByTestId("os-window-app:tasks");
  await expect(tasksBack).toBeVisible();
  await expect(appPage.getByTestId(`os-window-session:${session.id}`)).toHaveCount(0);
  await expect
    .poll(async () => rectsClose(await windowRect(appPage, tasksBack), arrangedTasks))
    .toBe(true);
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
  await useGlobalWorkspaceIfPrompted(appPage);

  // Deep link lands focused in the stack.
  const tasksWindow = appPage.getByTestId("os-window-app:tasks");
  await expect(tasksWindow).toBeVisible();
  await expect(tasksWindow).toHaveAttribute("data-presentation", "compact");
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
  await openDockApp(appPage, "Tasks", "tasks");
  const dockItem = appPage.locator('[data-slot="os-dock"] [data-app="tasks"]');
  const box = await dockItem.boundingBox();
  if (!box) throw new Error("dock item must be visible");
  await appPage.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await appPage.mouse.move(box.x + box.width / 2 + 4, box.y + box.height / 2, { steps: 3 });
  await expect
    .poll(() => dockItem.evaluate(element => (element as HTMLElement).style.transform))
    .toBe("");

  const tasksWindow = appPage.getByTestId("os-window-app:tasks");
  await tasksWindow.getByRole("button", { name: "Minimize window" }).click();
  await expect(tasksWindow).toBeHidden();
  await expect
    .poll(async () => (await windowManagerSnapshot(runtime, workspace.id)).windows["app:tasks"])
    .toMatchObject({ minimized: true });
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

  for (const [index, app] of PERF_APPS.entries()) {
    await openWindowInAuthority(
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
      const count = document.querySelectorAll('[data-testid^="os-window-app:"]').length;
      if (count >= 12) perf.windowsPlaced = performance.now();
    };
    // Init scripts run at document start — observe `document` itself so the
    // hook works before <html>/<body> exist.
    new MutationObserver(placed).observe(document, { childList: true, subtree: true });
    document.addEventListener("DOMContentLoaded", placed, { once: true });
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  for (const app of PERF_APPS) {
    await expect(appPage.getByTestId(`os-window-app:${app}`)).toBeAttached();
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
  // thread has been long-task quiet for 600ms so the 12 window bodies' initial
  // content burst can't masquerade as drag jank. (networkidle never settles
  // here — the shell keeps WebSocket/SSE connections open by design.)
  await appPage.evaluate(() => {
    const settle = { last: performance.now() };
    Reflect.set(window, "__osSettle", settle);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        settle.last = Math.max(settle.last, entry.startTime + entry.duration);
      }
    }).observe({ type: "longtask", buffered: true });
  });
  await appPage.waitForFunction(() => {
    const settle = Reflect.get(window, "__osSettle") as { last: number };
    return performance.now() - settle.last > 600;
  });

  // Long-task probe during a 3s continuous drag of one window.
  await appPage.evaluate(() => {
    const tasks: number[] = [];
    Reflect.set(window, "__osLongTasks", tasks);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) tasks.push(entry.duration);
    }).observe({ type: "longtask", buffered: false });
  });
  const dragged = appPage.getByTestId("os-window-app:vault");
  await focusWindow(appPage, dragged);
  const grip = await windowGrip(dragged);
  await appPage.mouse.move(grip.x, grip.y);
  await appPage.mouse.down();
  const start = Date.now();
  let step = 0;
  while (Date.now() - start < 3000) {
    const angle = (step / 20) * Math.PI * 2;
    await appPage.mouse.move(
      grip.x + 120 + Math.cos(angle) * 90,
      grip.y + 100 + Math.sin(angle) * 60,
      { steps: 2 }
    );
    step += 1;
  }
  await appPage.mouse.up();
  const longTasks = await appPage.evaluate(() => Reflect.get(window, "__osLongTasks") as number[]);
  const worstFrame = longTasks.length > 0 ? Math.max(...longTasks) : 0;

  const peerA = await openPeerPage(browser, runtime);
  const peerB = await openPeerPage(browser, runtime);
  let worstPeerTask = 0;
  try {
    for (const peer of [peerA, peerB]) {
      await expect(peer.getByTestId("os-window-app:sandbox")).toBeAttached();
      await peer.evaluate(() => {
        const tasks: number[] = [];
        Reflect.set(window, "__osLongTasks", tasks);
        new PerformanceObserver(list => {
          for (const entry of list.getEntries()) tasks.push(entry.duration);
        }).observe({ type: "longtask", buffered: false });
      });
    }
    await moveWindowFromCLI(runtime, workspace.id, "app:sandbox", "desktop-default", {
      x: 0.32,
      y: 0.24,
      width: 0.38,
      height: 0.46,
    });
    for (const peer of [peerA, peerB]) {
      await expect
        .poll(() =>
          windowMatchesAuthority(
            peer,
            peer.getByTestId("os-window-app:sandbox"),
            runtime,
            workspace.id,
            "app:sandbox"
          )
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
  await useGlobalWorkspaceIfPrompted(page);
  await expect(page.getByTestId("os-desktop")).toBeVisible();
  const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>("/api/workspaces");
  const workspace = payload.workspaces[0];
  if (!workspace) throw new Error("OS shell E2E requires one resolved workspace");
  return workspace;
}

async function openPeerPage(browser: Browser, runtime: BrowserRuntime): Promise<Page> {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(page);
  return page;
}

async function openDockApp(page: Page, name: string, app: string) {
  await page.getByRole("button", { name }).click();
  const win = page.getByTestId(`os-window-app:${app}`);
  await expect(win).toBeVisible();
  return win;
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
    win.boundingBox(),
    page.locator('[data-slot="os-win-layer"]').boundingBox(),
  ]);
  if (!windowBox || !layerBox) throw new Error("window and win-layer must be visible");
  return { x: Math.round(windowBox.x - layerBox.x), y: Math.round(windowBox.y - layerBox.y) };
}

async function windowRect(page: Page, win: ReturnType<Page["locator"]>) {
  const [windowBox, layerBox] = await Promise.all([
    win.boundingBox(),
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
 * A guaranteed drag surface on the window head: the identity (glyph + title)
 * area is never inside the drag-cancel selectors, unlike the head center,
 * which can land on the mode tabs (`topbar-nav`) once an app publishes them.
 */
async function windowGrip(win: ReturnType<Page["locator"]>): Promise<{ x: number; y: number }> {
  const title = win.locator('[data-slot="topbar-title"]');
  const box = await title.boundingBox();
  if (!box) throw new Error("window title must be visible to start a head drag");
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
    env: { ...process.env, AGH_HOME: runtime.paths.homeDir, HOME: runtime.paths.homeDir },
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
): Promise<void> {
  await executeWindowManagerCommand(runtime, workspaceId, {
    commandId: "window.open",
    payload: {
      window: {
        id: `app:${app}`,
        app,
        route,
        desktop_id: "desktop-default",
        floating_rect: rect,
        insert_tiled: false,
      },
    },
  });
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
  const payload = await runtime.requestJSON<{
    session: { id: string; agent_name: string };
  }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: browserLifecycleAgent, name, workspace: workspaceId }),
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
      env: { ...process.env, AGH_HOME: runtime.paths.homeDir, HOME: runtime.paths.homeDir },
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

async function addSecondWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  if (!runtime.paths) throw new Error("workspace switch E2E requires launch-mode runtime paths");
  const rootDir = path.join(runtime.paths.homeDir, "os-shell-second-workspace");
  const { mkdir } = await import("node:fs/promises");
  await mkdir(rootDir, { recursive: true });
  return await runtime.resolveWorkspace(rootDir);
}

type MenubarMenu = "AGH" | "Session" | "Go" | "Window" | "Help";

/** Menubar triggers are `role="menuitem"` inside the shell's single menubar. */
async function openMenu(page: Page, name: MenubarMenu): Promise<void> {
  await page.getByRole("menuitem", { name, exact: true }).click();
  await expect(page.getByTestId(`os-menu-${name.toLowerCase()}`)).toBeVisible();
}
