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
  windowID,
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
  await expect(window.getByRole("button", { name: "Open main.tsx in Files" })).toBeVisible();
  return { window, sessionId };
}

function reviewPanel(window: Locator) {
  return window.getByTestId("session-review-panel");
}

/** A transcript path's plain click: the project's Files window, as the frame's active tab. */
async function openPathInFiles(window: Locator, name: string) {
  await window.getByRole("button", { name: `Open ${name} in Files` }).click();
  const files = appWindow(window.page(), "files");
  await expect(files).toBeVisible();
  return files;
}

/** Brings the session's tab back to the front of its frame. */
async function showSessionTab(window: Locator) {
  await window
    .page()
    .getByTestId(`os-window-tab-${await windowID(window)}`)
    .locator('[data-slot="os-window-tab-activate"]')
    .click();
  await expect(window).toBeVisible();
}

// Invariant: a plain click on a transcript path opens the project's Files window as a tab of the
// session's frame, at the path and line with the tree revealing it, without opening the Review
// panel; a second path retargets that same window. The Review panel (Changes) still replaces the
// Inspector and keeps its width across a reload. Owner: transcript link routing + Files app;
// canonical suite: this file.
test("E2E-001: a transcript path opens the Files app as a tab at its line", async ({
  appPage,
  runtime,
}) => {
  const { window } = await openSessionWithReply(appPage, runtime);
  const sessionTab = appPage.getByTestId(`os-window-tab-${await windowID(window)}`);

  const files = await openPathInFiles(window, "main.tsx");
  // Same frame: the session keeps its tab beside the Files tab, and the head stays "Files".
  await expect(sessionTab).toBeVisible();
  await expect(appPage.getByTestId(`os-window-tab-${await windowID(files)}`)).toHaveAttribute(
    "data-active",
    ""
  );
  await expect(files.getByTestId("topbar-title-text")).toHaveText("Files");
  await expect(files.getByTestId("file-viewer-header")).toContainText("web/src/main.tsx");
  await expect(files.locator("[data-slot='code-viewer']")).toHaveAttribute(
    "data-highlighted-line",
    "12"
  );
  await expect(
    files.locator('[data-type="item"][data-item-path="web/src/main.tsx"]')
  ).toHaveAttribute("aria-selected", "true");

  await showSessionTab(window);
  await expect(reviewPanel(window)).toHaveCount(0);
  await openPathInFiles(window, ".env");
  await expect(appPage.locator('[data-slot="os-window-surface"][data-app="files"]')).toHaveCount(1);
  await expect(appWindow(appPage, "files").getByTestId("file-viewer-header")).toContainText(".env");

  await showSessionTab(window);
  await window.page().getByTestId("session-inspector-toggle").click();
  await expect(window.getByTestId("session-inspector")).toBeVisible();
  await window.page().getByTestId("session-review-toggle").click();
  const panel = reviewPanel(window);
  await expect(panel).toBeVisible();
  await expect(panel.getByRole("tab")).toHaveCount(0);
  await expect(window.getByTestId("session-inspector")).toHaveCount(0);
  const widthBefore = await panel.evaluate(element => element.getBoundingClientRect().width);
  await window.page().reload();
  await expect(reviewPanel(window)).toBeVisible();
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

  const files = await openPathInFiles(window, ".env");
  await expect(files.getByText("Sensitive file")).toBeVisible();
  await expect(files).not.toContainText("fixture-secret");
  await files.getByRole("button", { name: "Show contents" }).click();
  await expect(files.locator("[data-slot='code-viewer']")).toContainText(
    "API_TOKEN=fixture-secret"
  );

  await showSessionTab(window);
  await openPathInFiles(window, "logo.png");
  await expect(files.getByTestId("file-viewer-image").locator("img")).toBeVisible();

  await showSessionTab(window);
  await openPathInFiles(window, "build.log");
  await expect(files.getByTestId("file-viewer-truncated")).toHaveText(
    "Showing the first 1 MB of 2 MB"
  );

  // No desktop target control (S7's "Open in …" split button) in the browser.
  await expect(files.getByRole("button", { name: /^Open in / })).toHaveCount(0);
  await expect(files.getByRole("button", { name: "Choose application" })).toHaveCount(0);
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
    // One head row: the Files identity, then the project and root pickers. Only the window's
    // tab names the open file, else the project.
    const windowTitle = files.getByTestId("topbar-title-text");
    await expect(windowTitle).toHaveText("Files");
    await expect(
      files.locator('[data-slot="topbar-context"] [data-testid="files-root-picker"]')
    ).toBeVisible();
    // A lone window shows no tab strip; the palette's "Go to tab" row carries the same label.
    const filesWindowID = await windowID(files);
    const expectTabLabel = async (label: string) => {
      await appPage.keyboard.press("ControlOrMeta+K");
      const palette = appPage.getByTestId("os-command-palette");
      await palette.getByPlaceholder("Search apps, sessions, and actions…").fill(label);
      await expect(palette.getByTestId(`os-palette-tab-${filesWindowID}`)).toContainText(label);
      await appPage.keyboard.press("Escape");
      await expect(palette).toBeHidden();
    };
    await expectTabLabel(workspace.name);

    await files.locator('[data-type="item"][data-item-path="web/"]').click();
    await files.locator('[data-type="item"][data-item-path="web/README.md"]').click();
    await expect(files.getByTestId("file-viewer-header")).toContainText("web/README.md");
    await expect(files.locator("[data-slot='code-viewer']")).toContainText("Files app fixture.");
    await expectTabLabel("README.md");
    await expect(windowTitle).toHaveText("Files");

    await appPage.getByTestId("files-root-picker").click();
    await appPage.getByTestId(`files-root-option-${worktree.id}`).click();
    await expect(files.getByTestId("files-root-name")).toContainText(worktree.branch);
    await expectTabLabel("README.md");

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

const CHANGES_TAB = [
  'import { Tabs } from "@compozy/ui";',
  "",
  'import { useSessionChanges } from "../hooks/use-session-changes";',
  ...Array.from({ length: 36 }, (_, index) => `// filler ${index + 4}`),
  "export function SessionChangesTab({ sessionId }: { sessionId: string }) {",
  "  const changes = useSessionChanges(sessionId);",
  "  return changes;",
  "}",
];
const USE_CHANGES = [
  ...Array.from({ length: 13 }, (_, index) => `// header ${index + 1}`),
  "export function useSessionChanges(sessionId: string) {",
  "  return sessionId;",
  "}",
];

/** Files for the search journeys; written before the session's first name lookup. */
async function seedSearchFiles(root: string) {
  const session = path.join(root, "web/src/systems/session");
  await mkdir(path.join(session, "components"), { recursive: true });
  await mkdir(path.join(session, "hooks"), { recursive: true });
  await mkdir(path.join(root, "docs"), { recursive: true });
  const long = Array.from({ length: 5_000 }, (_, index) =>
    index + 1 === 4_800
      ? "export const needleOnlyHere = 4800;"
      : `export const row${index + 1} = 0;`
  );
  await Promise.all([
    writeFile(
      path.join(session, "components/session-review-panel.tsx"),
      "export function SessionReviewPanel() {}\n"
    ),
    writeFile(path.join(session, "components/session-row.tsx"), "export const row = 1;\n"),
    writeFile(
      path.join(session, "components/session-changes-tab.tsx"),
      `${CHANGES_TAB.join("\n")}\n`
    ),
    writeFile(path.join(session, "hooks/use-session-changes.ts"), `${USE_CHANGES.join("\n")}\n`),
    writeFile(path.join(root, "docs/long.ts"), `${long.join("\n")}\n`),
  ]);
}

/** Opens the Files window from a transcript path and puts focus inside the explorer. */
async function focusExplorer(window: Locator) {
  const files = await openPathInFiles(window, "main.tsx");
  await expect(files.getByTestId("files-tree")).toBeVisible();
  await files.getByTestId("files-filter").click();
  return files;
}

// Invariant: Go to file ranks names on the daemon, opens the chosen file in the viewer with the
// tree revealing it, and `name:line` opens at that line. Owner: files explorer; this file.
test("E2E-009: Go to file opens a fuzzy match and a name:line target", async ({
  appPage,
  runtime,
}) => {
  if (!runtime.paths) throw new Error("files E2E requires an isolated runtime");
  await seedSearchFiles(runtime.paths.workspaceDir);
  const { window } = await openSessionWithReply(appPage, runtime);
  const panel = await focusExplorer(window);

  await window.page().keyboard.press("ControlOrMeta+P");
  const box = panel.getByTestId("go-to-file");
  await box.getByTestId("go-to-file-input").fill("sesrevpan");
  const first = box.getByTestId("go-to-file-row").first();
  await expect(first).toHaveAttribute(
    "data-path",
    "web/src/systems/session/components/session-review-panel.tsx"
  );
  await expect(first.locator("mark").first()).toBeVisible();
  await window.page().keyboard.press("Enter");
  await expect(box).toHaveCount(0);
  await expect(panel.getByTestId("file-viewer-header")).toContainText("session-review-panel.tsx");
  await expect(
    panel.locator(
      '[data-type="item"][data-item-path="web/src/systems/session/components/session-review-panel.tsx"]'
    )
  ).toHaveAttribute("aria-selected", "true");

  await panel.getByTestId("files-go-to-file").click();
  await panel.getByTestId("go-to-file-input").fill("main.tsx:12");
  await expect(panel.getByTestId("go-to-file-footer")).toContainText("Opens at line 12");
  await expect(panel.getByTestId("go-to-file-row").first()).toHaveAttribute(
    "data-path",
    "web/src/main.tsx"
  );
  await window.page().keyboard.press("Enter");
  await expect(panel.getByTestId("file-viewer-header")).toContainText("web/src/main.tsx");
  await expect(panel.locator("[data-slot='code-viewer']")).toHaveAttribute(
    "data-highlighted-line",
    "12"
  );

  // Escape closes the box and hands focus back, so explorer shortcuts keep working.
  await window.page().keyboard.press("ControlOrMeta+P");
  await expect(panel.getByTestId("go-to-file")).toBeVisible();
  await window.page().keyboard.press("Escape");
  await expect(panel.getByTestId("go-to-file")).toHaveCount(0);
  await window.page().keyboard.press("ControlOrMeta+Shift+F");
  await expect(panel.getByTestId("files-search-input")).toBeFocused();
});

// Invariant: content search groups daemon results by file, a match opens the viewer at its line
// with Find in file seeded, and an invalid regex shows inline. Owner: files explorer; this file.
test("E2E-010: Search finds content, opens a match at its line, and reports a bad regex", async ({
  appPage,
  runtime,
}) => {
  if (!runtime.paths) throw new Error("files E2E requires an isolated runtime");
  await seedSearchFiles(runtime.paths.workspaceDir);
  const { window } = await openSessionWithReply(appPage, runtime);
  const panel = await focusExplorer(window);

  await window.page().keyboard.press("ControlOrMeta+Shift+F");
  const input = panel.getByTestId("files-search-input");
  await expect(input).toBeFocused();
  await input.fill("useSessionChanges");
  await expect(panel.getByTestId("files-search-summary")).toHaveText("3 results in 2 files");
  const tab = panel.locator(
    '[data-testid="files-search-file"][data-path="web/src/systems/session/components/session-changes-tab.tsx"]'
  );
  await expect(tab).toContainText("2");
  await panel.getByTestId("files-search-match").and(panel.locator('[data-line="41"]')).click();
  await expect(panel.getByTestId("file-viewer-header")).toContainText("session-changes-tab.tsx");
  const code = panel.locator("[data-slot='code-viewer']");
  await expect(code).toHaveAttribute("data-highlighted-line", "41");
  await expect(code).toHaveAttribute("data-find-active-line", "41");
  await expect(panel.getByTestId("viewer-find-input")).toHaveValue("useSessionChanges");
  await expect(panel.getByTestId("viewer-find-count")).toHaveText("2 of 2");

  await panel.getByRole("button", { name: "Use regular expression" }).click();
  await input.fill("foo(");
  await expect(panel.getByTestId("files-search-error")).toBeVisible();
  await expect(input).toHaveAttribute("aria-invalid", "true");
});

// Invariant: Find in file searches the viewer's full text, so a match on a never-rendered line
// scrolls into view. Owner: files viewer; this file.
test("E2E-011: Find in file reaches a match deep in a long file", async ({ appPage, runtime }) => {
  if (!runtime.paths) throw new Error("files E2E requires an isolated runtime");
  await seedSearchFiles(runtime.paths.workspaceDir);
  const { window } = await openSessionWithReply(appPage, runtime);
  const panel = await focusExplorer(window);

  await window.page().keyboard.press("ControlOrMeta+P");
  await panel.getByTestId("go-to-file-input").fill("docs/long.ts");
  await expect(panel.getByTestId("go-to-file-row").first()).toHaveAttribute(
    "data-path",
    "docs/long.ts"
  );
  await window.page().keyboard.press("Enter");
  const code = panel.locator("[data-slot='code-viewer']");
  await expect(code).toHaveAttribute("data-line-count", "5000");

  await panel.getByTestId("file-viewer-focus").focus();
  await window.page().keyboard.press("ControlOrMeta+F");
  await panel.getByTestId("viewer-find-input").fill("needleOnlyHere");
  await expect(panel.getByTestId("viewer-find-count")).toHaveText("1 of 1");
  await expect(code).toHaveAttribute("data-find-active-line", "4800");
  await expect(code.locator('[data-line="4800"]')).toBeInViewport();
});
