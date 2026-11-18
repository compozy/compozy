import { execFile } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Locator, Page } from "@playwright/test";

import { appWindow, sessionWindow, windowID } from "../fixtures/os-navigation";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const fixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_changes_fixture.json"
);
const agentName = "session-changes-agent";

test.use({
  viewport: { width: 1600, height: 1000 },
  runtimeOptions: {
    seed: { mockAgents: [{ fixturePath: fixture, fixtureAgent: agentName }] },
  },
});

type Runtime = Parameters<Parameters<typeof test>[2]>[0]["runtime"];

async function git(root: string, ...args: string[]) {
  await execFileAsync(
    "git",
    [
      "-c",
      "user.name=e2e",
      "-c",
      "user.email=e2e@example.com",
      "-c",
      "commit.gpgsign=false",
      ...args,
    ],
    { cwd: root }
  );
}

function goSource(changedLine: number | null): string {
  return `${[
    "package src",
    ...Array.from({ length: 60 }, (_, index) =>
      index + 1 === changedLine
        ? `var line${index + 1} = "changed"`
        : `var line${index + 1} = ${index + 1}`
    ),
  ].join("\n")}\n`;
}

/** main: base commit; feat/review: a branch commit, an uncommitted edit, an untracked file. */
async function seedBranchRepository(root: string, options: { mainAhead?: boolean } = {}) {
  await mkdir(path.join(root, "src"), { recursive: true });
  await writeFile(path.join(root, "src/b.go"), goSource(null));
  await writeFile(path.join(root, "README.md"), "# Fixture\n\nFirst line.\n");
  await git(root, "init", "-q", "-b", "main");
  await git(root, "add", ".");
  await git(root, "commit", "-q", "-m", "base");
  if (options.mainAhead) {
    // `main~1` is then an older base than the branch point, with one more file in between.
    await writeFile(path.join(root, "main-only.txt"), "only on main\n");
    await git(root, "add", "main-only.txt");
    await git(root, "commit", "-q", "-m", "main moves");
  }
  await git(root, "checkout", "-q", "-b", "feat/review");
  await writeFile(path.join(root, "src/b.go"), goSource(30));
  await git(root, "commit", "-q", "-am", "change b");
  await writeFile(path.join(root, "README.md"), "# Fixture\n\nFirst line.\nSecond line.\n");
  await writeFile(path.join(root, "notes.txt"), "untracked note\n");
}

async function openSession(appPage: Page, runtime: Runtime, prompt?: string) {
  if (!runtime.paths) throw new Error("changes E2E requires an isolated runtime");
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await completeOnboardingIfPrompted(appPage);
  const created = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspace.id }),
  });
  const sessionId = created.session.id;
  await appPage.goto(runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const window = sessionWindow(appPage, sessionId);
  const ui = sessionWindowSelectors(window, appPage);
  if (prompt) {
    await ui.composerTextarea.click();
    await ui.composerTextarea.pressSequentially(prompt);
    await ui.composerTextarea.press("Enter");
  }
  return { window, ui };
}

async function openChanges(window: Locator) {
  const panel = window.getByTestId("session-review-panel");
  const toggle = window.page().getByTestId("session-review-toggle");
  await expect(toggle).toBeVisible();
  if ((await toggle.getAttribute("data-state")) !== "open") await toggle.click();
  await expect(panel).toBeVisible();
  return panel;
}

function treeRow(panel: Locator, filePath: string) {
  return panel
    .getByTestId("changes-tree")
    .locator(`[data-type="item"][data-item-path="${filePath}"]`);
}

function fileHeader(panel: Locator, filePath: string) {
  return panel.locator(`[data-testid="changes-file-header"][data-path="${filePath}"]`);
}

// Invariant: Changes shows the branch diff (committed, uncommitted, untracked) with its header,
// tree, persisted mode, expandable context, focus from a turn's changed-file row, and Open in
// Files into the project's Files window.
// Owner: session Changes tab; canonical suite: this file.
test("E2E-004: Changes reviews the branch diff and focuses files from the tree and transcript", async ({
  appPage,
  runtime,
}) => {
  await seedBranchRepository(runtime.paths!.workspaceDir);
  const { window } = await openSession(appPage, runtime, "summarize the edits");
  const panel = await openChanges(window);

  await expect(panel.getByTestId("changes-scope")).toHaveText("Branch");
  await expect(panel.getByTestId("changes-totals")).toContainText("+");
  await expect(panel.getByTestId("changes-head")).toHaveText("feat/review");
  await expect(panel.getByTestId("changes-base-selector")).toHaveText("main");
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "3");
  for (const filePath of ["README.md", "notes.txt", "src/b.go"]) {
    await expect(fileHeader(panel, filePath)).toHaveCount(1);
  }

  await treeRow(panel, "notes.txt").click();
  await expect(fileHeader(panel, "notes.txt")).toBeInViewport();
  await expect(fileHeader(panel, "notes.txt")).toHaveAttribute("data-selected", "true");

  await panel.getByTestId("changes-diff-mode").click();
  await expect(panel.locator("[data-slot='diff-viewer']")).toHaveAttribute("data-mode", "split");
  await window.page().reload();
  const reloaded = window.getByTestId("session-review-panel");
  // Changes is the panel's only view: no tab strip.
  await expect(reloaded.getByRole("tab")).toHaveCount(0);
  await expect(reloaded.locator("[data-slot='diff-viewer']")).toHaveAttribute("data-mode", "split");

  await treeRow(reloaded, "src/b.go").click();
  const diff = reloaded.locator("[data-slot='diff-viewer']");
  const lines = diff.locator("[data-line]");
  await expect(lines.first()).toBeVisible();
  const before = await lines.count();
  await diff.getByText("27 unmodified lines").first().click();
  await expect.poll(() => lines.count()).toBeGreaterThan(before);

  await window.getByText(/Edited 1 file/).click();
  await window.getByRole("button", { name: "Review changes in b.go" }).click();
  await expect(fileHeader(reloaded, "src/b.go")).toHaveAttribute("data-selected", "true");
  await expect(fileHeader(reloaded, "src/b.go")).toBeInViewport();

  // Open in Files: the project's Files window opens as a tab beside the session, on that file.
  await fileHeader(reloaded, "src/b.go").getByTestId("changes-open-in-viewer").click();
  const files = appWindow(window.page(), "files");
  await expect(files.getByTestId("file-viewer-header")).toContainText("src/b.go");
  await expect(
    window
      .page()
      .getByTestId("os-tab-bar")
      .locator(`[role="tab"][data-lead-window-id="${await windowID(window)}"]`)
  ).toBeVisible();
});

// Invariant: scope and base drive the summary: Uncommitted drops committed files, another base
// changes the totals. Owner: session Changes tab; canonical suite: this file.
test("E2E-005: switching scope and base changes what Changes compares", async ({
  appPage,
  runtime,
}) => {
  await seedBranchRepository(runtime.paths!.workspaceDir, { mainAhead: true });
  const { window } = await openSession(appPage, runtime);
  const panel = await openChanges(window);
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "3");

  await panel.getByTestId("changes-scope").click();
  await window.page().getByTestId("changes-scope-uncommitted").click();
  await expect(panel.getByTestId("changes-scope")).toHaveText("Uncommitted");
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "2");
  await expect(fileHeader(panel, "src/b.go")).toHaveCount(0);

  await panel.getByTestId("changes-scope").click();
  await window.page().getByTestId("changes-scope-branch").click();
  const totals = panel.getByTestId("changes-totals");
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "3");
  const branchTotals = await totals.textContent();
  await panel.getByTestId("changes-base-selector").click();
  await window.page().getByRole("combobox", { name: "Filter branches" }).fill("main~1");
  await window.page().getByTestId("changes-base-custom").click();
  await expect(panel.getByTestId("changes-base-selector")).toHaveText("main~1");
  await expect(totals).not.toHaveText(branchTotals ?? "");
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "4");
  await expect(fileHeader(panel, "main-only.txt")).toHaveCount(1);
});

// Invariant: past 200 files Changes shows one file at a time with steppers, and the tree still
// lists every file. Owner: session Changes tab; canonical suite: this file.
test("E2E-006: a 250-file diff shows one file at a time", async ({ appPage, runtime }) => {
  const root = runtime.paths!.workspaceDir;
  await writeFile(path.join(root, "README.md"), "# Fixture\n");
  await git(root, "init", "-q", "-b", "main");
  await git(root, "add", ".");
  await git(root, "commit", "-q", "-m", "base");
  await mkdir(path.join(root, "bulk"), { recursive: true });
  await Promise.all(
    Array.from({ length: 250 }, (_, index) =>
      writeFile(
        path.join(root, `bulk/f${String(index + 1).padStart(3, "0")}.txt`),
        `file ${index + 1}\n`
      )
    )
  );
  const { window } = await openSession(appPage, runtime);
  const panel = await openChanges(window);

  await expect(panel.getByTestId("changes-large-banner")).toContainText(
    "This diff is large, showing one file at a time"
  );
  await expect(panel.getByTestId("changes-large-position")).toHaveText("1 of 250");
  await expect(panel.getByTestId("changes-file-header")).toHaveCount(1);
  await panel.getByTestId("changes-next-file").click();
  await expect(panel.getByTestId("changes-large-position")).toHaveText("2 of 250");
  await expect(fileHeader(panel, "bulk/f002.txt")).toHaveCount(1);
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "250");
});

// Invariant: a completed agent tool call refreshes Changes without a manual refresh.
// Owner: session Changes tab live refresh; canonical suite: this file.
test("E2E-007: an agent edit appears in Changes without a manual refresh", async ({
  appPage,
  runtime,
}) => {
  await seedBranchRepository(runtime.paths!.workspaceDir);
  const { window, ui } = await openSession(appPage, runtime);
  const panel = await openChanges(window);
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "3");

  await ui.composerTextarea.click();
  await ui.composerTextarea.pressSequentially("add e.go");
  await ui.composerTextarea.press("Enter");
  await expect(window.getByText("Added e.go.")).toBeVisible();
  // US-010.AC-1: within 2 s of the tool call, plus the stream's own delivery.
  await expect(fileHeader(panel, "e.go")).toHaveCount(1, { timeout: 4_000 });
  await expect(panel.getByTestId("changes-tree")).toHaveAttribute("data-file-count", "4");
});
