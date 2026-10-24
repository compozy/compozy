import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { sessionWindow, windowFrame } from "../fixtures/os-navigation";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { seedSessionsBoard } from "../fixtures/sessions-board";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

// Invariant: the session window switches between List and the Sessions board, whose
// columns are the daemon's board_column reads with facets.board counts; cards open
// their session in List; columns page by 50; archived sessions live in the Archive
// bar; child sessions link to their parent and can be hidden (E2E-005..E2E-007).
// Owner: public session window + daemon catalog. Durable rows come from the store
// seeder; mapping rules and rendering states are owned by the Vitest suites.

const agentName = "browser-lifecycle-agent";
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_lifecycle_fixture.json"
);

test.use({ runtimeOptions: { seed: { mockAgents: [{ fixturePath, fixtureAgent: agentName }] } } });

type BoardColumn = "needs_you" | "working" | "in_review" | "done";
const COLUMN_LABELS: Record<BoardColumn, string> = {
  needs_you: "Needs you",
  working: "Working",
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

async function openBoardPage(page: Page, runtime: BrowserRuntime, sessionId: string) {
  const workspace = await resolveWorkspace(runtime);
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await ensureProjectWorkspace(page, runtime);
  await openSessionWindow(runtime, workspace.id, sessionId);
  const win = sessionWindow(page, sessionId);
  await expect(win).toBeVisible();
  return { workspace, win, frame: windowFrame(win) };
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

test("E2E-005: operator switches to Kanban, opens a card in List, and keeps the view across reloads", async ({
  appPage,
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
  const { frame } = await openBoardPage(appPage, runtime, seeded.done[0]!);
  const toggle = frame.getByRole("group", { name: "View" });
  const list = toggle.getByRole("button", { name: "List" });
  const kanban = toggle.getByRole("button", { name: "Kanban" });
  await expect(list).toHaveAttribute("aria-pressed", "true");

  // Keyboard: focus the selected item, move with ArrowRight, select with Enter.
  await list.focus();
  await appPage.keyboard.press("ArrowRight");
  await expect(kanban).toBeFocused();
  await appPage.keyboard.press("Enter");
  await expect(frame.getByTestId("sessions-board")).toBeVisible();
  await expect(kanban).toHaveAttribute("aria-pressed", "true");
  await expect(toggle.locator('[aria-pressed="true"]')).toHaveCount(1);

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

  await childCard.click();
  const opened = sessionWindow(appPage, failedChild);
  await expect(opened).toBeVisible();
  const openedFrame = windowFrame(opened);
  await expect(openedFrame.getByTestId("sessions-board")).toHaveCount(0);
  await expect(
    openedFrame.getByRole("group", { name: "View" }).getByRole("button", { name: "List" })
  ).toHaveAttribute("aria-pressed", "true");

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWindow(appPage, failedChild)).toBeVisible();
  await expect(
    windowFrame(sessionWindow(appPage, failedChild))
      .getByRole("group", { name: "View" })
      .getByRole("button", { name: "List" })
  ).toHaveAttribute("aria-pressed", "true");
});

test("E2E-006: Done pages 50 cards with Load more and archived sessions stay in the Archive bar", async ({
  appPage,
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
  const { frame } = await openBoardPage(appPage, runtime, seeded.done[0]!);
  await switchToKanban(frame);

  const done = column(frame, "done");
  await expect(frame.getByRole("region", { name: "Done, 60 sessions" })).toBeVisible();
  await expect(done.locator('[data-testid^="session-board-card-"][data-board-column]')).toHaveCount(
    50
  );
  const more = frame.getByTestId("sessions-board-load-more-done");
  await expect(more).toContainText("50 of 60");
  await more.click();
  await expect(done.locator('[data-testid^="session-board-card-"][data-board-column]')).toHaveCount(
    60
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

test("E2E-007: a parent shows its highlighted rollup and hiding child sessions drops cards and counts", async ({
  appPage,
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
  const { frame } = await openBoardPage(appPage, runtime, parent);
  await switchToKanban(frame);

  const rollup = frame.getByTestId(`session-board-card-children-${parent}`);
  await expect(rollup).toHaveText("3");
  await expect(rollup).toHaveAttribute("data-needs-you", "true");
  await expect(
    column(frame, "needs_you").getByTestId(`session-board-card-parent-${children[0]}`)
  ).toHaveText("Board coordinator");
  await expectColumnCounts(frame, await boardFacets(runtime, workspace.id));

  const hide = frame.getByTestId("sessions-board-hide-children");
  await hide.click();
  await expect(hide).toHaveAttribute("aria-pressed", "true");
  for (const id of children) {
    await expect(frame.getByTestId(`session-board-card-${id}`)).toHaveCount(0);
  }
  await expect(frame.getByTestId(`session-board-card-${parent}`)).toBeVisible();
  await expectColumnCounts(frame, await boardFacets(runtime, workspace.id, "exclude"));
  await browserArtifacts.captureScreenshot("sessions-board-children-hidden", appPage);
});
