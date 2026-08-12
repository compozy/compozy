import type { Page } from "@playwright/test";

import type { BrowserRuntime } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import { createWorktreeRepo, type WorktreeRepoFixture } from "../fixtures/worktree-repo";

/**
 * Worktree lifecycle against a real daemon and real git. The API is used only to
 * prepare or verify state; every refusal and every destructive step is driven
 * through the UI, because that is the contract this task ships.
 */
let repo: WorktreeRepoFixture;

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

async function openOverviewNest(page: Page, workspaceId: string) {
  await openWorkspaceMenu(page);
  await page.getByTestId("os-workspace-overview").click();
  const toggle = page.getByTestId(`os-workspace-worktrees-toggle-${workspaceId}`);
  await expect(toggle).toBeVisible();
  await toggle.click();
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

  await openWorkspaceMenu(appPage);
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

  await openWorkspaceMenu(appPage);
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
  await expect(appPage.getByTestId("worktree-create-pending")).toBeHidden();
});

test("operator is refused a colliding name, a held branch, and a missing base ref", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(repo.rootDir);
  const existing = await seedReadyWorktree(runtime, workspace.id, "payments-retry");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openWorkspaceMenu(appPage);
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

  await openWorkspaceMenu(appPage);
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
  await openWorkspaceMenu(appPage);
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

  await openOverviewNest(appPage, workspace.id);
  await appPage.getByTestId(`os-workspace-nest-${workspace.id}-remove-${worktree.id}`).click();

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
        return listing.worktrees.some(entry => entry.id === worktree.id);
      },
      { timeout: 30_000 }
    )
    .toBe(false);
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

  await openOverviewNest(appPage, workspace.id);
  await appPage.getByTestId(`os-workspace-nest-${workspace.id}-resolve-${worktree.id}`).click();

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
  await repo.restoreWorktreeAt(worktree.path, "pedro/hotfix-cors");
  await appPage.reload({ waitUntil: "domcontentloaded" });

  await openOverviewNest(appPage, workspace.id);
  await appPage.getByTestId(`os-workspace-nest-${workspace.id}-resolve-${worktree.id}`).click();
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
