import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { reloadDaemonServedPage } from "../fixtures/navigation";
import { sessionWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { seedSessionsBoard } from "../fixtures/sessions-board";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

// Invariant: the session window switches between List and the Sessions board, whose
// columns are the daemon's board_column reads with facets.board counts; cards open
// their session in a drawer over the live board (#1052) that Escape closes back to the
// card, Back reopens, a reload restores, and "Open full session" turns into List; columns
// page by 50; archived sessions live in the Archive bar; child sessions start hidden, link
// to their parent once shown (E2E-005..E2E-007); cards multi-select and run the List's bulk
// verbs against the daemon (E2E-008).
// Owner: public session window + daemon catalog. Durable rows come from the store
// seeder; mapping rules and rendering states are owned by the Vitest suites.
// Use the blank page fixture: the seeder restarts the daemon, so loading the app
// before seeding races its module downloads. openBoardPage owns the first navigation.

const agentName = "browser-lifecycle-agent";
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_lifecycle_fixture.json"
);

// Floating windows keep the board window in its own frame, on top, instead of a background tab.
test.use({
  runtimeOptions: {
    newWindowPolicy: "floating",
    seed: { mockAgents: [{ fixturePath, fixtureAgent: agentName }] },
  },
});

type BoardColumn = "needs_you" | "working" | "waiting" | "in_review" | "done";
const COLUMN_LABELS: Record<BoardColumn, string> = {
  needs_you: "Needs you",
  working: "Working",
  waiting: "Waiting",
  in_review: "In review",
  done: "Done",
};

async function resolveWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  const workspace =
    runtime.seeded.workspace ??
    (runtime.paths?.workspaceDir
      ? await runtime.resolveWorkspace(runtime.paths.workspaceDir)
      : null);
  if (!workspace) throw new Error("Sessions board walk requires a seeded workspace");
  return workspace;
}

async function boardFacets(
  runtime: BrowserRuntime,
  workspaceId: string,
  children: "include" | "exclude" = "include"
): Promise<Record<BoardColumn, number>> {
  const payload = await runtime.requestJSON<{ facets: { board: Record<BoardColumn, number> } }>(
    `/api/sessions/facets?workspace_id=${encodeURIComponent(workspaceId)}&children=${children}`
  );
  return payload.facets.board;
}

/** Opens a session window for `sessionId` through the window-manager authority. */
async function openSessionWindow(
  runtime: BrowserRuntime,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  const base = `/api/workspaces/${encodeURIComponent(workspaceId)}/window-manager`;
  const snapshot = await runtime.requestJSON<{ revision: number }>(base);
  await runtime.requestJSON(`${base}/commands`, {
    method: "POST",
    body: JSON.stringify({
      workspace_id: workspaceId,
      command_id: "window.open",
      expected_revision: snapshot.revision,
      actor: { kind: "e2e", id: "sessions-board" },
      origin: "web-e2e",
      payload: {
        window: {
          id: `e2e-board-${sessionId}`,
          app: "session",
          instance_key: sessionId,
          route: {
            pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(sessionId)}`,
            search: {},
          },
          desktop_id: "desktop-default",
          floating_rect: { x: 0.02, y: 0.06, width: 0.96, height: 0.88 },
          insert_tiled: false,
        },
      },
    }),
  });
}

/**
 * A live session the window can show: seeded rows are catalog-only, so the
 * by-id read a session window resolves would retire a window pointed at one.
 */
async function createLiveSession(runtime: BrowserRuntime, workspaceId: string, name: string) {
  const { session } = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, name, workspace: workspaceId }),
  });
  await expect
    .poll(async () => {
      const result = await runtime.requestJSON<{ session: { state: string } }>(
        `/api/workspaces/${workspaceId}/sessions/${session.id}`
      );
      return result.session.state;
    })
    .toBe("active");
  return session.id;
}

async function openBoardPage(page: Page, runtime: BrowserRuntime, sessionId: string) {
  const workspace = await resolveWorkspace(runtime);
  await reloadDaemonServedPage(page, runtime, "/", { readyTestId: "os-desktop" });
  await ensureProjectWorkspace(page, runtime);
  await openSessionWindow(runtime, workspace.id, sessionId);
  // The window surface holds both its head (topbar) and its body.
  const frame = sessionWindow(page, sessionId);
  await expect(frame).toBeVisible();
  return { workspace, frame };
}

async function switchToKanban(frame: Locator) {
  const toggle = frame.getByRole("group", { name: "View" });
  await toggle.getByRole("button", { name: "Kanban" }).click();
  await expect(frame.getByTestId("sessions-board")).toBeVisible();
}

function column(frame: Locator, id: BoardColumn): Locator {
  return frame.getByTestId(`sessions-board-column-${id}`);
}

async function expectColumnCounts(frame: Locator, counts: Record<BoardColumn, number>) {
  for (const id of Object.keys(COLUMN_LABELS) as BoardColumn[]) {
    const label = `${COLUMN_LABELS[id]}, ${counts[id]} ${counts[id] === 1 ? "session" : "sessions"}`;
    await expect(frame.getByRole("region", { name: label })).toBeVisible();
  }
}

test("E2E-005: operator switches to Kanban, opens a card in the drawer over the board, and keeps both across reloads", async ({
  page: appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const seeded = await seedSessionsBoard(runtime, {
    workspaceId: workspace.id,
    agentName,
    prefix: "sess-e2e005",
    done: 2,
    family: true,
  });
  const host = await createLiveSession(runtime, workspace.id, "Board host session");
  const target = await createLiveSession(runtime, workspace.id, "Board target session");
  const { frame } = await openBoardPage(appPage, runtime, host);
  const toggle = frame.getByRole("group", { name: "View" });
  const list = toggle.getByRole("button", { name: "List" });
  const kanban = toggle.getByRole("button", { name: "Kanban" });
  await expect(list).toHaveAttribute("aria-pressed", "true");
  await browserArtifacts.captureScreenshot("sessions-board-list-head", appPage);

  // Keyboard: focus the selected item, move with ArrowRight, select with Enter.
  await list.focus();
  await appPage.keyboard.press("ArrowRight");
  await expect(kanban).toBeFocused();
  await appPage.keyboard.press("Enter");
  await expect(frame.getByTestId("sessions-board")).toBeVisible();
  await expect(kanban).toHaveAttribute("aria-pressed", "true");
  await expect(toggle.locator('[aria-pressed="true"]')).toHaveCount(1);

  await expectColumnCounts(frame, await boardFacets(runtime, workspace.id, "exclude"));
  await frame.getByTestId("sessions-board-hide-children").click();
  await expectColumnCounts(frame, await boardFacets(runtime, workspace.id));
  const failedChild = seeded.children![0]!;
  const childCard = column(frame, "needs_you").getByTestId(`session-board-card-${failedChild}`);
  await expect(childCard.getByTestId(`session-board-card-status-${failedChild}`)).toHaveText(
    "Failed"
  );
  await expect(childCard.getByTestId(`session-board-card-parent-${failedChild}`)).toHaveText(
    "Board coordinator"
  );
  await browserArtifacts.captureScreenshot("sessions-board-kanban", appPage);

  // The view belongs to the window: a reload keeps the board.
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(frame.getByTestId("sessions-board")).toBeVisible();

  // A card opens its session in the drawer: the window keeps the board and shows that session.
  const card = frame.getByTestId(`session-board-card-${target}`);
  await card.click();
  const openedFrame = sessionWindow(appPage, target);
  const drawer = openedFrame.getByRole("dialog", { name: "Board target session" });
  await expect(drawer).toBeVisible();
  await expect(openedFrame.getByTestId("sessions-board")).toBeVisible();
  await expect(openedFrame.getByTestId(`session-board-card-${target}`)).toHaveAttribute(
    "aria-current",
    "true"
  );
  await expect(drawer.getByRole("button", { name: "Send message" })).toBeVisible();
  await expect(appPage).toHaveURL(/[?&]drawer=open/);
  await browserArtifacts.captureScreenshot("sessions-board-drawer", appPage);

  // Escape closes it back to the card; Back reopens it; a reload restores it.
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await expect(openedFrame.getByTestId(`session-board-card-${target}`)).toBeFocused();
  await appPage.goBack();
  await expect(drawer).toBeVisible();
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(drawer).toBeVisible();
  await expect(openedFrame.getByTestId("sessions-board")).toBeVisible();

  // Open full session: the same window shows that session in List, and keeps it on reload.
  await drawer.getByTestId("session-board-drawer-expand").click();
  await expect(openedFrame.getByTestId("sessions-board")).toHaveCount(0);
  const openedList = openedFrame
    .getByRole("group", { name: "View" })
    .getByRole("button", { name: "List" });
  await expect(openedList).toHaveAttribute("aria-pressed", "true");
  await expect(appPage).not.toHaveURL(/[?&]drawer=open/);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWindow(appPage, target)).toBeVisible();
  await expect(openedList).toHaveAttribute("aria-pressed", "true");
});

test("E2E-006: Done pages 50 cards with Load more and archived sessions stay in the Archive bar", async ({
  page: appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const seeded = await seedSessionsBoard(runtime, {
    workspaceId: workspace.id,
    agentName,
    prefix: "sess-e2e006",
    done: 60,
    archived: 3,
  });
  const host = await createLiveSession(runtime, workspace.id, "Board host session");
  const { frame } = await openBoardPage(appPage, runtime, host);
  await switchToKanban(frame);

  const done = column(frame, "done");
  // The runtime fixture owns a few sessions of its own; counts come from the daemon.
  const doneCount = (await boardFacets(runtime, workspace.id)).done;
  expect(doneCount).toBeGreaterThanOrEqual(60);
  await expect(frame.getByRole("region", { name: `Done, ${doneCount} sessions` })).toBeVisible();
  await expect(done.locator('[data-testid^="session-board-card-"][data-board-column]')).toHaveCount(
    50
  );
  const more = frame.getByTestId("sessions-board-load-more-done");
  await expect(more).toContainText(`50 of ${doneCount}`);
  await more.click();
  await expect(done.locator('[data-testid^="session-board-card-"][data-board-column]')).toHaveCount(
    Math.min(doneCount, 100)
  );
  await expect(more).toHaveCount(0);

  const archive = frame.getByTestId("sessions-board-archive-toggle");
  await expect(archive).toHaveText(/Archive\s*3/);
  await expect(archive).toHaveAttribute("aria-expanded", "false");
  await archive.click();
  const body = frame.getByTestId("sessions-board-archive-body");
  for (const id of seeded.archived) {
    await expect(body.getByTestId(`session-board-card-${id}`)).toBeVisible();
    await expect(done.getByTestId(`session-board-card-${id}`)).toHaveCount(0);
  }
  await browserArtifacts.captureScreenshot("sessions-board-archive", appPage);
});

test("E2E-007: child sessions start hidden under a highlighted rollup, and showing them adds cards and counts that survive a reload", async ({
  page: appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const seeded = await seedSessionsBoard(runtime, {
    workspaceId: workspace.id,
    agentName,
    prefix: "sess-e2e007",
    family: true,
  });
  const parent = seeded.parent!;
  const children = seeded.children!;
  const host = await createLiveSession(runtime, workspace.id, "Board host session");
  const { frame } = await openBoardPage(appPage, runtime, host);
  await switchToKanban(frame);

  const hide = frame.getByTestId("sessions-board-hide-children");
  await expect(hide).toHaveAttribute("aria-pressed", "true");
  for (const id of children) {
    await expect(frame.getByTestId(`session-board-card-${id}`)).toHaveCount(0);
  }
  const rollup = frame.getByTestId(`session-board-card-children-${parent}`);
  await expect(rollup).toHaveText("3");
  await expect(rollup).toHaveAttribute("data-needs-you", "true");
  await expectColumnCounts(frame, await boardFacets(runtime, workspace.id, "exclude"));
  await browserArtifacts.captureScreenshot("sessions-board-children-hidden", appPage);

  await hide.click();
  await expect(hide).toHaveAttribute("aria-pressed", "false");
  await expect(
    column(frame, "needs_you").getByTestId(`session-board-card-parent-${children[0]}`)
  ).toHaveText("Board coordinator");
  await expectColumnCounts(frame, await boardFacets(runtime, workspace.id));

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(hide).toHaveAttribute("aria-pressed", "false");
});

test("E2E-008: operator multi-selects board cards and stops and archives them in one batch", async ({
  page: appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const host = await createLiveSession(runtime, workspace.id, "Board host session");
  const first = await createLiveSession(runtime, workspace.id, "Board bulk first");
  const second = await createLiveSession(runtime, workspace.id, "Board bulk second");
  const { frame } = await openBoardPage(appPage, runtime, host);
  await switchToKanban(frame);

  const firstCard = frame.getByTestId(`session-board-card-${first}`);
  const secondCard = frame.getByTestId(`session-board-card-${second}`);
  await firstCard.click({ modifiers: ["ControlOrMeta"] });
  // An open selection turns a plain click into a toggle; nothing opens.
  await secondCard.click();
  await expect(frame.getByTestId("sessions-board-selection-count")).toHaveText("2 selected");
  await expect(firstCard).toHaveAttribute("aria-pressed", "true");
  await expect(frame.getByTestId("sessions-board-new-session")).toHaveCount(0);

  await secondCard.click({ button: "right" });
  const menu = appPage.getByTestId("sessions-board-selection-menu");
  await expect(menu.getByTestId("sessions-board-selection-menu-count")).toHaveText("2 sessions");
  await browserArtifacts.captureScreenshot("sessions-board-selection-menu", appPage);
  await menu.getByTestId("sessions-board-selection-menu-archive").click();

  // Both cards leave the columns for the Archive bar, and the selection empties with them.
  await expect(firstCard).toHaveCount(0);
  await expect(secondCard).toHaveCount(0);
  await expect(frame.getByTestId("sessions-board-archive-toggle")).toHaveText(/Archive\s*2/);
  await expect(frame.getByTestId("sessions-board-selection-bar")).toHaveCount(0);
  await expect(frame.getByTestId("sessions-board-new-session")).toBeVisible();
});
