import type { Page } from "@playwright/test";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

import { sessionWindow } from "../fixtures/os-navigation";
import { sessionWindowSelectors } from "../fixtures/selectors";
import type { BrowserRuntime } from "../fixtures/runtime";
import { sessionWindow } from "../fixtures/os-navigation";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import { createWorktreeRepo, type WorktreeRepoFixture } from "../fixtures/worktree-repo";

/**
 * Worktree lifecycle against a real daemon and real git. The API is used only to
 * prepare or verify state; every refusal and every destructive step is driven
 * through the UI, because that is the contract this task ships.
 */
let repo: WorktreeRepoFixture;

const worktreeSessionAgent = "browser-lifecycle-agent";
const worktreeSessionFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_lifecycle_fixture.json"
);

// The daemon already gets an isolated HOME; clearing ambient token bindings
// makes the zero-credential browser fallback deterministic as well.
test.use({
  runtimeOptions: {
    env: { ...process.env, GH_TOKEN: "", GITHUB_TOKEN: "" },
    seed: {
      mockAgents: [
        {
          agentName: worktreeSessionAgent,
          fixtureAgent: worktreeSessionAgent,
          fixturePath: worktreeSessionFixture,
        },
      ],
    },
  },
});

test.beforeEach(async () => {
  repo = await createWorktreeRepo();
});

test.afterEach(async () => {
  await repo.cleanup();
});

interface WorktreeRecord {
  id: string;
  name: string;
  branch: string;
  path: string;
  state: string;
  origin: string;
}

async function listWorktrees(runtime: BrowserRuntime, workspaceId: string, refresh = false) {
  return runtime.requestJSON<{ worktrees: WorktreeRecord[]; discovered: Array<{ path: string }> }>(
    `/api/workspaces/${workspaceId}/worktrees${refresh ? "?refresh=true" : ""}`
  );
}

/** Seeds a ready worktree through the API so the spec can start at the UI step. */
async function seedReadyWorktree(
  runtime: BrowserRuntime,
  workspaceId: string,
  name: string
): Promise<WorktreeRecord> {
  await runtime.requestJSON(`/api/workspaces/${workspaceId}/worktrees`, {
    body: JSON.stringify({ name }),
    method: "POST",
  });
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspaceId);
        return listing.worktrees.find(worktree => worktree.name === name)?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("ready");
  const listing = await listWorktrees(runtime, workspaceId);
  const worktree = listing.worktrees.find(entry => entry.name === name);
  if (!worktree) throw new Error(`seeded worktree ${name} is missing from the listing`);
  return worktree;
}

async function openWorkspaceMenu(page: Page) {
  await page.locator('[data-slot="os-menubar-workspace"]').click();
  await expect(page.getByTestId("os-workspace-menu")).toBeVisible();
}

async function openWorkspaceNest(page: Page, workspaceId: string) {
  await openWorkspaceMenu(page);
  // Hovering the workspace row opens the side submenu holding the nest.
  await page.getByTestId(`os-workspace-option-${workspaceId}`).hover();
  await expect(page.getByTestId(`os-worktree-submenu-${workspaceId}`)).toBeVisible();
}

async function selectWorkspace(page: Page, workspaceId: string) {
  await openWorkspaceMenu(page);
  // A pointer click on the workspace row selects it and closes the menu.
  await page.getByTestId(`os-workspace-option-${workspaceId}`).click();
  await expect(page.getByTestId("os-workspace-menu")).toHaveCount(0);
}

async function openSessionCreate(page: Page) {
  await page.getByRole("button", { name: "New session", exact: true }).click();
  const dialog = page.getByTestId("session-create-dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("session-create-agent-select").click();
  await page.getByTestId(`agent-command-item-${worktreeSessionAgent}`).click();
  await expect(dialog.getByTestId("session-create-agent-select")).toContainText(
    worktreeSessionAgent
  );
}

async function openOverviewMenu(page: Page, workspaceId: string) {
  await openWorkspaceMenu(page);
  await page.getByRole("menuitem", { name: /^Workspace picker/ }).click();
  await expect(page.getByTestId("os-workspaces-overview")).toBeVisible();
  // The focused tile anchors the always-visible vertical worktree menu.
  const workspaceTile = page.getByTestId(`os-workspace-tile-${workspaceId}`);
  await expect(workspaceTile).toBeVisible();
  await workspaceTile.hover();
  await expect(page.getByTestId("os-workspaces-worktree-menu")).toBeVisible();
}

/** Overlay kebab: Copy path · Delete worktree… (the S13 remove flow). */
async function deleteFromOverviewRow(page: Page, worktreeId: string) {
  await page.getByTestId(`os-workspaces-worktree-actions-${worktreeId}`).click();
  await page.getByTestId(`os-workspaces-worktree-delete-${worktreeId}`).click();
}

/** Menubar nest actions still own Resolve… and Context… for a worktree row. */
async function chooseNestRowAction(page: Page, worktreeId: string, action: "resolve" | "context") {
  await page.getByTestId(`os-worktree-actions-${worktreeId}`).click();
  await page.getByTestId(`os-worktree-${action}-${worktreeId}`).click();
}

// E2E-006: create dialog — name-first with a generated placeholder, derived
// branch → path preview, the three refusals, and a pending row that can be
// cancelled daemon-side until it is ready.
test("operator creates a worktree name-first and can cancel the pending creation", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  await repo.setSetupCommand("sleep 20");
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  await appPage.getByTestId(`os-worktree-create-${workspace.id}`).click();

  const dialog = appPage.getByTestId("worktree-create-dialog");
  await expect(dialog).toBeVisible();

  // The generated name is a suggestion in the placeholder, never a prefill.
  const nameField = appPage.getByTestId("worktree-create-name");
  await expect(nameField).toHaveValue("");
  await expect(nameField).not.toHaveAttribute("placeholder", "");

  await nameField.fill("payments-retry");
  await expect(appPage.getByTestId("worktree-create-preview")).toContainText("payments-retry");

  await appPage.getByTestId("worktree-create-submit").click();

  // 202 accepted: the row is durable and pending, and Cancel stays live because
  // dismissing the dialog is not aborting the creation.
  const pending = appPage.getByTestId("worktree-create-pending");
  await expect(pending).toBeVisible();
  await expect(appPage.getByTestId("worktree-create-cancel")).toBeEnabled();

  await appPage.getByTestId("worktree-create-cancel-creation").click();

  // Cancelling unwinds the creation daemon-side and drops the row.
  await expect(pending).toBeHidden();
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.some(worktree => worktree.name === "payments-retry");
      },
      { timeout: 30_000 }
    )
    .toBe(false);
});

test("operator sees an accepted worktree remain pending until setup finishes", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  await repo.setSetupCommand("sleep 2");
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  await appPage.getByTestId(`os-worktree-create-${workspace.id}`).click();
  await appPage.getByTestId("worktree-create-name").fill("docs-refresh");
  await appPage.getByTestId("worktree-create-submit").click();

  await expect(appPage.getByTestId("worktree-create-pending")).toBeVisible();
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id);
        return listing.worktrees.find(entry => entry.name === "docs-refresh")?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("ready");
  await expect(appPage.getByTestId("worktree-create-dialog")).toBeHidden();

  const listing = await listWorktrees(runtime, workspace.id);
  const created = listing.worktrees.find(entry => entry.name === "docs-refresh");
  if (!created) throw new Error("ready worktree docs-refresh is missing from the listing");
  await openWorkspaceNest(appPage, workspace.id);
  await expect(appPage.getByTestId(`os-worktree-option-${created.id}`)).toHaveAttribute(
    "aria-checked",
    "true"
  );
});

test("operator is refused a colliding name, a held branch, and a missing base ref", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const existing = await seedReadyWorktree(runtime, workspace.id, "payments-retry");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  await appPage.getByTestId(`os-worktree-create-${workspace.id}`).click();

  // Name collision: the reason lands on the name field and blocks the primary.
  await appPage.getByTestId("worktree-create-name").fill("payments-retry");
  await appPage.getByTestId("worktree-create-submit").click();
  await expect(appPage.getByTestId("worktree-create-name-error")).toBeVisible();
  await expect(appPage.getByTestId("worktree-create-submit")).toBeDisabled();

  // Editing the offending field clears the refusal rather than latching it.
  await appPage.getByTestId("worktree-create-name").fill("payments-retry-2");
  await expect(appPage.getByTestId("worktree-create-submit")).toBeEnabled();

  // Base ref that does not exist: refused on the base-ref field.
  await appPage.getByTestId("worktree-create-advanced-toggle").click();
  await appPage.getByTestId("worktree-create-base-ref").fill("trunk");
  await appPage.getByTestId("worktree-create-submit").click();
  await expect(appPage.getByTestId("worktree-create-base-ref-error")).toBeVisible();
  await appPage.getByTestId("worktree-create-base-ref").fill("main");

  // Branch already checked out elsewhere: refused with the jump to its holder.
  await appPage.getByTestId("worktree-create-branch").fill(existing.branch);
  await appPage.getByTestId("worktree-create-submit").click();
  await expect(appPage.getByTestId("worktree-create-branch-error")).toBeVisible();
  const jump = appPage.getByTestId("worktree-create-select-holder");
  await expect(jump).toBeVisible();
  await jump.click();
  // Selecting the holder closes the dialog instead of creating a duplicate.
  await expect(appPage.getByTestId("worktree-create-dialog")).toBeHidden();
});

// E2E-007: adopt-on-select — the confirm names the validation, and the refusal
// leaves both the row and the directory untouched.
test("operator adopts a discovered worktree by selecting it", async ({ appPage, runtime }) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await repo.addExternalWorktree("northstar-spike", "spike/sqlite-vacuum");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  const discoveredRow = appPage.getByTestId(/^os-worktree-option-discovered:/).first();
  await expect(discoveredRow).toBeVisible();
  await discoveredRow.click();

  const confirm = appPage.getByTestId("worktree-adopt-confirm");
  await expect(confirm).toBeVisible();
  await expect(confirm).toContainText("resolves to this repository");
  await expect(confirm).toContainText("bootstrap is not re-run");

  await appPage.getByTestId("worktree-adopt-submit").click();
  await expect(confirm).toBeHidden();

  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.some(worktree => worktree.origin === "adopted");
      },
      { timeout: 30_000 }
    )
    .toBe(true);
});

test("operator sees adoption refused for the main checkout with the directory untouched", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const external = await repo.addExternalWorktree("main-alias", "spike/main-alias");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  // Keep the discovered row mounted, then make its path resolve to the main
  // checkout. The daemon must revalidate identity instead of trusting discovery.
  await openWorkspaceNest(appPage, workspace.id);
  const discoveredRow = appPage.getByTestId(/^os-worktree-option-discovered:/).first();
  await expect(discoveredRow).toBeVisible();
  await repo.replaceDirectoryWithSymlink(external, repo.rootDir);
  await discoveredRow.click();
  await expect(appPage.getByTestId("worktree-adopt-confirm")).toBeVisible();
  await appPage.getByTestId("worktree-adopt-submit").click();

  const refusal = appPage.getByTestId("worktree-adopt-refusal");
  await expect(refusal).toBeVisible();
  await expect(appPage.getByTestId("worktree-adopt-refusal-reason")).toContainText(
    "resolves into the main checkout"
  );
  expect(await repo.realPath(external)).toBe(await repo.realPath(repo.rootDir));

  // Nothing was registered and the main checkout stayed intact.
  const listing = await listWorktrees(runtime, workspace.id, true);
  expect(listing.worktrees.some(worktree => worktree.path === repo.rootDir)).toBe(false);
  await expect(refusal).toContainText("directory stays exactly where it is");
});

// E2E-016: dirty removal refuses in the UI with quantified loss, force re-states
// it, and the missing dialog exercises dismissal and restore separately.
test("operator must pass the force doorway before a dirty worktree is removed", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const worktree = await seedReadyWorktree(runtime, workspace.id, "payments-retry");
  await repo.dirtyWorktree(worktree.path);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openOverviewMenu(appPage, workspace.id);
  await deleteFromOverviewRow(appPage, worktree.id);

  const dialog = appPage.getByTestId("worktree-remove-dialog");
  await expect(dialog).toBeVisible();
  await appPage.getByTestId("worktree-remove-submit").click();

  // The daemon refuses and the dialog names exactly what is at risk.
  await expect(dialog).toHaveAttribute("data-stage", "dirty-refusal");
  await expect(appPage.getByTestId("worktree-remove-risks")).toContainText("changed");

  // Force is a doorway to a second confirm, never the direct action.
  await appPage.getByTestId("worktree-remove-force-doorway").click();
  await expect(dialog).toHaveAttribute("data-stage", "force");
  // The second step re-states the quantities before destroying them.
  await expect(dialog).toContainText("deleted permanently");

  await appPage.getByTestId("worktree-remove-submit").click();

  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.find(entry => entry.id === worktree.id)?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("removed");
  await expect(appPage.getByTestId(`os-workspaces-worktree-row-${worktree.id}`)).toHaveCount(0);
});

test("operator dismisses a missing worktree record without losing its history", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const worktree = await seedReadyWorktree(runtime, workspace.id, "hotfix-cors");

  await repo.removeDirectory(worktree.path);
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.find(entry => entry.id === worktree.id)?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("missing");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  await chooseNestRowAction(appPage, worktree.id, "resolve");

  const dialog = appPage.getByTestId("worktree-missing-dialog");
  await expect(dialog).toBeVisible();
  // History preservation is stated before either choice is made.
  await expect(dialog).toContainText("Run history is preserved");

  await appPage.getByTestId("worktree-missing-dismiss").click();

  // Dismissal drops the record only; the outcome is rendered verbatim.
  await expect(appPage.getByTestId("worktree-missing-outcome")).toBeVisible();
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.some(entry => entry.id === worktree.id);
      },
      { timeout: 30_000 }
    )
    .toBe(false);
});

test("operator restores a missing worktree when its checkout comes back", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const worktree = await seedReadyWorktree(runtime, workspace.id, "hotfix-cors");

  // Pruned out of band, then restored at the very same path.
  await repo.removeDirectory(worktree.path);
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.find(entry => entry.id === worktree.id)?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("missing");
  await repo.restoreWorktreeAt(worktree.path, worktree.branch);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  await chooseNestRowAction(appPage, worktree.id, "resolve");
  await expect(appPage.getByTestId("worktree-missing-dialog")).toBeVisible();

  await appPage.getByTestId("worktree-missing-restore").click();

  // Adoption is the restore path: the same record returns to ready, and a new
  // record is never minted for it.
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.find(entry => entry.id === worktree.id)?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("ready");
  const listing = await listWorktrees(runtime, workspace.id);
  expect(listing.worktrees.filter(entry => entry.name === "hotfix-cors")).toHaveLength(1);
});

// E2E-008: the session-create environment control — root default, ready
// worktrees only, "New worktree…", absence on a non-git workspace, and a reset
// when the workspace changes.
test("operator picks a session environment under Workspace", async ({ appPage, runtime }) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const nonGitWorkspace = await runtime.resolveWorkspace(repo.nonGitDir);
  const ready = await seedReadyWorktree(runtime, workspace.id, "payments-retry");
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await selectWorkspace(appPage, workspace.id);

  await openSessionCreate(appPage);
  await appPage.getByTestId("session-create-mode-advanced").click();

  const environment = appPage.getByTestId("session-create-environment");
  await expect(environment).toBeVisible();
  await expect(environment).toContainText("Workspace root");

  await environment.click();
  await expect(appPage.getByRole("option", { name: new RegExp(ready.name) })).toBeVisible();
  await expect(appPage.getByRole("option", { name: "New worktree…" })).toBeVisible();
  await appPage.getByRole("option", { name: new RegExp(ready.name) }).click();
  await expect(environment).toContainText(ready.name);

  // Session creation is scoped by the shell. A non-git workspace has no
  // environment control at all, and reopening in the original workspace
  // cannot retain a choice owned by the prior dialog.
  const dialog = appPage.getByTestId("session-create-dialog");
  await dialog.getByRole("button", { name: "Close" }).click();
  await selectWorkspace(appPage, nonGitWorkspace.id);
  await openSessionCreate(appPage);
  await appPage.getByTestId("session-create-mode-advanced").click();
  await expect(appPage.getByTestId("session-create-environment")).toHaveCount(0);

  // Worktrees belong to one workspace, so switching back discards the choice.
  await appPage.getByTestId("session-create-dialog").getByRole("button", { name: "Close" }).click();
  await selectWorkspace(appPage, workspace.id);
  await openSessionCreate(appPage);
  await appPage.getByTestId("session-create-mode-advanced").click();
  const reopenedEnvironment = appPage.getByTestId("session-create-environment");
  await expect(reopenedEnvironment).toBeVisible();
  await expect(reopenedEnvironment).toContainText("Workspace root");
});

// E2E-009 (US-004 AC-2): starting a session materializes its chosen environment;
// the first message is then authored and sent by the durable, bound session.
test("operator starts a bound environment and sends its first message", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await selectWorkspace(appPage, workspace.id);
  await openSessionCreate(appPage);

  await appPage.getByTestId("session-create-mode-advanced").click();
  await appPage.getByTestId("session-create-environment").click();
  await appPage.getByRole("option", { name: "New worktree…" }).click();

  // Choosing is not creating: an abandoned dialog must leave no checkout behind.
  await expect(appPage.locator('[data-slot="session-environment-phase"]')).toHaveCount(0);
  expect((await listWorktrees(runtime, workspace.id, true)).worktrees).toHaveLength(0);

  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await appPage.getByRole("button", { name: /start session/i }).click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBe(true);
  const createPayload = (await createResponse.json()) as { session?: { id?: string } };
  const sessionId = createPayload.session?.id;
  if (!sessionId) throw new Error("Created session response did not include an id.");

  const created = await pollForBoundSession(runtime, workspace.id);
  expect(created.id).toBe(sessionId);
  const sessionWin = sessionWindow(appPage, sessionId);
  const sessionUI = sessionWindowSelectors(sessionWin, appPage);
  const chip = appPage.locator('[data-slot="session-environment-chip"]');
  await expect(chip).toBeVisible();
  await expect(chip).toHaveAttribute("data-locked", "");
  await expect(chip).toHaveAttribute("data-binding", "worktree");

  await sessionUI.composerTextarea.fill("Investigate the checkout latency regression");
  await sessionUI.composerTextarea.press("Enter");

  // The bound session owns the first message and persists it in its transcript.
  await expect
    .poll(async () => {
      const transcript = await runtime.requestJSON<{
        entries: Array<{
          message: { role: string; parts: Array<{ type: string; text?: string }> };
        }>;
      }>(`/api/workspaces/${workspace.id}/sessions/${created.id}/transcript`);
      return transcript.entries.filter(
        entry =>
          entry.message.role === "user" &&
          entry.message.parts.some(
            part =>
              part.type === "text" &&
              part.text?.includes("Investigate the checkout latency regression")
          )
      ).length;
    })
    .toBe(1);
});

/** The session created by the first send, once the daemon reports it bound. */
async function pollForBoundSession(runtime: BrowserRuntime, workspaceId: string) {
  let bound: { id: string; worktree_id?: string } | undefined;
  await expect
    .poll(async () => {
      const sessions = await runtime.requestJSON<{
        sessions: Array<{ id: string; worktree_id?: string }>;
      }>(`/api/sessions?workspace_id=${workspaceId}`);
      bound = sessions.sessions.find(session => Boolean(session.worktree_id));
      return bound?.worktree_id;
    })
    .not.toBeUndefined();
  if (!bound) throw new Error("Expected a session bound to a worktree.");
  return bound;
}

// E2E-010: /worktree on a live session — the three facts, one new session per
// confirmation, the original untouched, and the daemon's refusal mid-turn.
test("operator forks a live session into a worktree", async ({ appPage, runtime }) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  const target = await seedReadyWorktree(runtime, workspace.id, "payments-retry");
  await selectWorkspace(appPage, workspace.id);

  await openSessionCreate(appPage);
  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await appPage.getByRole("button", { name: /start session/i }).click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBe(true);
  const createPayload = (await createResponse.json()) as { session?: { id?: string } };
  const originSessionID = createPayload.session?.id;
  if (!originSessionID) throw new Error("Created origin session response did not include an id.");

  let sessionsBefore: { sessions: Array<{ id: string; worktree_id?: string }> } | undefined;
  await expect
    .poll(async () => {
      sessionsBefore = await runtime.requestJSON<{
        sessions: Array<{ id: string; worktree_id?: string }>;
      }>(`/api/sessions?workspace_id=${workspace.id}`);
      return sessionsBefore.sessions.map(session => session.id);
    })
    .toEqual([originSessionID]);
  if (!sessionsBefore) throw new Error("Created origin session was missing from the catalog.");
  expect(sessionsBefore.sessions).toHaveLength(1);
  const originSession = sessionsBefore.sessions[0];

  const chip = appPage.locator('[data-slot="session-environment-chip"]');
  await expect(chip).toBeVisible();

  await appPage.locator('[data-testid="composer-input"] [contenteditable="true"]').fill("/");
  const command = appPage
    .getByTestId("composer-command-item")
    .filter({ hasText: "Worktree" })
    .first();
  await expect(command).toBeVisible();
  await expect(command).not.toHaveAttribute("data-unavailable", "");

  await command.click();
  const facts = appPage.locator('[data-slot="session-fork-facts"]');
  await expect(facts.locator('[data-fact="new-session"]')).toBeVisible();
  await expect(facts.locator('[data-fact="untouched"]')).toBeVisible();
  await expect(facts.locator('[data-fact="changes-stay"]')).toBeVisible();

  await appPage.getByTestId("session-fork-target").click();
  await appPage.getByRole("option", { name: new RegExp(target.name) }).click();
  await appPage.getByRole("button", { name: /fork session/i }).click();

  await expect
    .poll(async () => {
      const sessions = await runtime.requestJSON<{
        sessions: Array<{ id: string; worktree_id?: string }>;
      }>(`/api/sessions?workspace_id=${workspace.id}`);
      return {
        originWorktree: sessions.sessions.find(session => session.id === originSession.id)
          ?.worktree_id,
        total: sessions.sessions.length,
        target: sessions.sessions.filter(session => session.worktree_id === target.id).length,
      };
    })
    .toEqual({ originWorktree: undefined, total: sessionsBefore.sessions.length + 1, target: 1 });
});

// E2E-014: the worktree context — truthful strip, a daemon-rendered ladder, the
// commit dialog's scope and honest placeholder, and streamed progress phases.
test("operator runs the assisted exit from the worktree context", async ({ appPage, runtime }) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  const worktree = await seedReadyWorktree(runtime, workspace.id, "payments-retry");
  await repo.dirtyWorktree(worktree.path);

  await openWorkspaceNest(appPage, workspace.id);
  await chooseNestRowAction(appPage, worktree.id, "context");

  const strip = appPage.locator('[data-slot="worktree-status-strip"]');
  await expect(strip).toBeVisible();
  await expect(strip.locator('[data-field="branch"]')).toContainText(worktree.branch);
  await expect(strip.locator('[data-field="dirty"]')).toHaveAttribute("data-tone", "warning");

  const control = appPage.locator('[data-slot="worktree-exit-control"]');
  await expect(control).toBeVisible();
  const primary = control.locator('[data-slot="split-button-action"]');
  await expect(primary).toBeEnabled();

  await primary.click();
  const commit = appPage.locator('[data-slot="worktree-commit-dialog"]');
  await expect(commit).toBeVisible();
  await expect(commit.locator('[data-slot="worktree-scope-block"]')).toContainText("files");
  await expect(appPage.getByPlaceholder("Leave blank to use a default message.")).toBeVisible();

  await commit.locator('[data-slot="split-button-action"]').click();
  const progressSurface = appPage.locator('[data-slot="worktree-exit-progress-surface"]');
  await expect(progressSurface).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(appPage.locator('[data-slot="worktree-detail-dialog"]')).toBeHidden();
  await expect(progressSurface).toBeVisible();
  await expect(progressSurface.locator('[data-slot="worktree-exit-progress"]')).toHaveAttribute(
    "data-status",
    "completed",
    { timeout: 30_000 }
  );
  await expect(progressSurface.getByRole("button", { name: "Remove worktree" })).toHaveCount(1);
});

// E2E-015: merged / safe-to-clean evidence leads into the standard removal flow.
test("operator reads cleanup evidence before removing a finished worktree", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  const worktree = await seedReadyWorktree(runtime, workspace.id, "fix-flaky-e2e");

  await openWorkspaceNest(appPage, workspace.id);
  await chooseNestRowAction(appPage, worktree.id, "context");

  const evidence = appPage.locator('[data-slot="worktree-merged-evidence"]');
  await expect(evidence).toBeVisible();
  await expect(evidence).toHaveAttribute("data-safe", "");
  const cleanUp = evidence.getByRole("button", { name: "Clean up" });
  await expect(cleanUp).toBeVisible();
  await cleanUp.click();

  const removal = appPage.getByTestId("worktree-remove-dialog");
  await expect(removal).toBeVisible();
  await appPage.getByTestId("worktree-remove-submit").click();
  await expect
    .poll(
      async () => {
        const listing = await listWorktrees(runtime, workspace.id, true);
        return listing.worktrees.find(entry => entry.id === worktree.id)?.state;
      },
      { timeout: 30_000 }
    )
    .toBe("removed");
  await expect(appPage.getByTestId(`os-worktree-option-${worktree.id}`)).toHaveCount(0);
});

// E2E-017: zero-credential remote — PR affordances are absent, not disabled,
// and the browser compare path is the whole PR step.
test("operator sees no PR affordances without a credential", async ({ appPage, runtime }) => {
  await completeOnboardingIfPrompted(appPage);
  await repo.setCredentiallessGitHubOrigin();
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const worktree = await seedReadyWorktree(runtime, workspace.id, "auth-refresh");
  await repo.publishWorktreeForBrowser(worktree.path);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceNest(appPage, workspace.id);
  await chooseNestRowAction(appPage, worktree.id, "context");

  const strip = appPage.locator('[data-slot="worktree-status-strip"]');
  await expect(strip).toBeVisible();
  await expect(strip.locator('[data-field="pr"]')).toHaveCount(0);

  const control = appPage.locator('[data-slot="worktree-exit-control"]');
  await expect(control).toHaveAttribute("data-forge", "absent");
  const browserStep = control.locator('[data-slot="split-button-action"]');
  await expect(browserStep).toContainText("Open in browser");
  await browserStep.click();
  const prDialog = appPage.locator('[data-slot="worktree-pr-dialog"]');
  await expect(prDialog).toHaveAttribute("data-mode", "browser-only");
  await expect(
    prDialog.locator('[data-slot="worktree-pr-action"][data-action="browser"]')
  ).toBeVisible();
  await expect(prDialog.getByLabel("Title")).toHaveCount(0);
  await expect(prDialog.getByLabel("Description")).toHaveCount(0);
});
