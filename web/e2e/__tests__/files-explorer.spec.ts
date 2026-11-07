import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import type { BrowserRuntime } from "../fixtures/runtime";
import {
  appWindow,
  openAppWindow,
  sessionWindow,
  switchWorkspace,
} from "../fixtures/os-navigation";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import { createWorktreeRepo } from "../fixtures/worktree-repo";

const fixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_files_explorer_fixture.json"
);
const agentName = "files-explorer-agent";

test.use({
  // Wide enough for the Inspector to sit inline beside the transcript (below 1440 px it is a
  // modal sheet over it), so the Review panel ↔ Inspector exchange is observable.
  viewport: { width: 1600, height: 900 },
  runtimeOptions: {
    seed: { mockAgents: [{ fixturePath: fixture, fixtureAgent: agentName }] },
  },
});

// A 1x1 PNG.
const LOGO = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "base64"
);

async function seedProjectFiles(root: string) {
  await mkdir(path.join(root, "web/src"), { recursive: true });
  await mkdir(path.join(root, "docs"), { recursive: true });
  await mkdir(path.join(root, "logs"), { recursive: true });
  const main = Array.from(
    { length: 30 },
    (_, index) => `export const line${index + 1} = ${index + 1};`
  );
  await Promise.all([
    writeFile(path.join(root, "web/src/main.tsx"), `${main.join("\n")}\n`),
    writeFile(path.join(root, ".env"), "API_TOKEN=fixture-secret\n"),
    writeFile(path.join(root, "docs/logo.png"), LOGO),
    // 2 MiB of text: over the 1 MiB read cap.
    writeFile(path.join(root, "logs/build.log"), `${"build step ok\n".repeat(150_000)}`),
  ]);
}

async function openSessionWithReply(
  appPage: Page,
  runtime: Parameters<Parameters<typeof test>[2]>[0]["runtime"]
) {
  if (!runtime.paths) throw new Error("files E2E requires an isolated runtime");
  await seedProjectFiles(runtime.paths.workspaceDir);
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
  await ui.composerTextarea.click();
  await ui.composerTextarea.pressSequentially("where are the files?");
  await ui.composerTextarea.press("Enter");
  await expect(window.getByRole("button", { name: "Open main.tsx in viewer" })).toBeVisible();
  return { window, sessionId };
}

function reviewPanel(window: Locator) {
  return window.getByTestId("session-review-panel");
}

// Invariant: a transcript path opens the session's Review panel on Files with the tree revealing
// the file and the viewer on its line; the panel replaces the Inspector and its width and tab
// persist across a reload. Owner: session Review panel + link routing; canonical suite: this file.
test("E2E-001: a transcript path opens the Files tab at its line and the panel persists", async ({
  appPage,
  runtime,
}) => {
  const { window } = await openSessionWithReply(appPage, runtime);
  const inspectorToggle = window.page().getByTestId("session-inspector-toggle");
  await inspectorToggle.click();
  await expect(window.getByTestId("session-inspector")).toBeVisible();

  await window.getByRole("button", { name: "Open main.tsx in viewer" }).click();
  const panel = reviewPanel(window);
  await expect(panel).toBeVisible();
  await expect(panel.getByRole("tab", { name: "Files" })).toHaveAttribute("aria-selected", "true");
  await expect(window.getByTestId("session-inspector")).toHaveCount(0);
  await expect(panel.getByTestId("file-viewer-header")).toContainText("web/src/main.tsx");
  await expect(panel.locator("[data-slot='code-viewer']")).toHaveAttribute(
    "data-highlighted-line",
    "12"
  );
  await expect(
    panel.locator('[data-type="item"][data-item-path="web/src/main.tsx"]')
  ).toHaveAttribute("aria-selected", "true");

  const widthBefore = await panel.evaluate(element => element.getBoundingClientRect().width);
  await window.page().reload();
  await expect(reviewPanel(window)).toBeVisible();
  await expect(reviewPanel(window).getByRole("tab", { name: "Files" })).toHaveAttribute(
    "aria-selected",
    "true"
  );
  await expect
    .poll(() => reviewPanel(window).evaluate(element => element.getBoundingClientRect().width))
    .toBeCloseTo(widthBefore, -1);
});

// Invariant: sensitive, image, and oversized files each render their truthful state in the browser
// client, and no Open in control exists there. Owner: files viewer; canonical suite: this file.
test("E2E-003: the browser viewer gates sensitive files, previews images, and truncates large text", async ({
  appPage,
  runtime,
}) => {
  const { window } = await openSessionWithReply(appPage, runtime);
  const panel = reviewPanel(window);

  await window.getByRole("button", { name: "Open .env in viewer" }).click();
  await expect(panel.getByText("Sensitive file")).toBeVisible();
  await expect(panel).not.toContainText("fixture-secret");
  await panel.getByRole("button", { name: "Show contents" }).click();
  await expect(panel.locator("[data-slot='code-viewer']")).toContainText(
    "API_TOKEN=fixture-secret"
  );

  await window.getByRole("button", { name: "Open logo.png in viewer" }).click();
  await expect(panel.getByTestId("file-viewer-image").locator("img")).toBeVisible();

  await window.getByRole("button", { name: "Open build.log in viewer" }).click();
  await expect(panel.getByTestId("file-viewer-truncated")).toHaveText(
    "Showing the first 1 MB of 2 MB"
  );

  await expect(panel.getByRole("button", { name: /^Open in/ })).toHaveCount(0);
});

/** Seeds a ready worktree through the API so the spec starts at the Files app step. */
async function seedReadyWorktree(runtime: BrowserRuntime, workspaceId: string, name: string) {
  await runtime.requestJSON(`/api/workspaces/${workspaceId}/worktrees`, {
    body: JSON.stringify({ name }),
    method: "POST",
  });
  const ready = async () => {
    const listing = await runtime.requestJSON<{
      worktrees: Array<{ id: string; name: string; branch: string; state: string }>;
    }>(`/api/workspaces/${workspaceId}/worktrees`);
    return listing.worktrees.find(worktree => worktree.name === name && worktree.state === "ready");
  };
  await expect.poll(async () => (await ready())?.state, { timeout: 30_000 }).toBe("ready");
  const worktree = await ready();
  if (!worktree) throw new Error(`seeded worktree ${name} is missing from the listing`);
  return worktree;
}

// Invariant: the dock's Files icon (right after Agents) opens one Files window per project on
// the active project; its root picker re-roots the tree on a worktree, and a second dock click
// focuses that same window. Owner: Files OS app + dock; canonical suite: this file.
test("E2E-002: the Files dock app browses the active project and switches to a worktree root", async ({
  appPage,
  runtime,
}) => {
  const repo = await createWorktreeRepo();
  try {
    await mkdir(path.join(repo.rootDir, "web"), { recursive: true });
    await writeFile(path.join(repo.rootDir, "web/README.md"), "# web\n\nFiles app fixture.\n");
    await completeOnboardingIfPrompted(appPage);
    const workspace = await runtime.resolveWorkspace(repo.rootDir);
    const worktree = await seedReadyWorktree(runtime, workspace.id, "files-app-root");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await switchWorkspace(appPage, workspace.id, workspace.name);

    const dock = appPage.locator('[data-slot="os-dock"]:visible');
    const launchers = dock.getByRole("button");
    const names = await launchers.evaluateAll(buttons =>
      buttons.map(button => button.getAttribute("aria-label") ?? button.textContent ?? "")
    );
    const agentsIndex = names.findIndex(name => name.startsWith("Agents"));
    expect(names[agentsIndex + 1]).toMatch(/^Files/);

    const files = await openAppWindow(appPage, "Files", "files");
    await expect(files.getByTestId("files-root-name")).toContainText(workspace.name);

    await files.locator('[data-type="item"][data-item-path="web/"]').click();
    await files.locator('[data-type="item"][data-item-path="web/README.md"]').click();
    await expect(files.getByTestId("file-viewer-header")).toContainText("web/README.md");
    await expect(files.locator("[data-slot='code-viewer']")).toContainText("Files app fixture.");

    await appPage.getByTestId("files-root-picker").click();
    await appPage.getByTestId(`files-root-option-${worktree.id}`).click();
    await expect(files.getByTestId("files-root-name")).toContainText(worktree.branch);

    await dock.getByRole("button", { name: "Files" }).click();
    await expect(appPage.locator('[data-slot="os-window-surface"][data-app="files"]')).toHaveCount(
      1
    );
    await expect(appWindow(appPage, "files").getByTestId("files-root-name")).toContainText(
      worktree.branch
    );
  } finally {
    await repo.cleanup();
  }
});
