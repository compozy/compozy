import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { sessionWindow, switchWorkspace } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import { createWorktreeRepo } from "../fixtures/worktree-repo";

// Subagents (`.compozy/tasks/subagents/_tests.md` E2E-001…E2E-006).
// The parent agent calls `compozy__subagent_capabilities` and `compozy__subagent_delegate`
// for real through its hosted MCP server (acpmock `native_tool_call`), so the delegate call,
// its `subagent_id` result and the card land in the parent transcript exactly as a provider's
// would. Workers answer after a delay, fail their prompt, or block until canceled. E2E-006
// replays the pinned Claude Agent adapter frames (`internal/acp/testdata/
// claude_agent_subagent.jsonl`, session id rebound to the mock's). The agent-collaboration
// isolated card (`.compozy/tasks/agent-collaboration/_tests.md` E2E-004) delegates with
// `isolation: "worktree"` from a real git workspace; the worker commits for real through an
// ACP terminal in its own worktree.
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_subagent_journeys_fixture.json"
);
const parentAgent = "subagent-journey-parent";
const workerAgent = "subagent-journey-worker";
const nativeAgent = "subagent-native";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath, fixtureAgent: parentAgent },
        { fixturePath, fixtureAgent: workerAgent },
        { fixturePath, fixtureAgent: nativeAgent },
      ],
    },
  },
});

interface Scene {
  page: Page;
  runtime: BrowserRuntime;
  workspaceId: string;
  sessionId: string;
  window: Locator;
}

async function openSession(
  appPage: Page,
  runtime: BrowserRuntime,
  agentName: string,
  prompt: string,
  workspaceDir = runtime.paths?.workspaceDir
): Promise<Scene> {
  if (!workspaceDir) throw new Error("subagent E2E requires an isolated runtime");
  const workspace = await runtime.resolveWorkspace(workspaceDir);
  await completeOnboardingIfPrompted(appPage);
  // A session outside the active project asks before switching; make its project active first.
  if (workspaceDir !== runtime.paths?.workspaceDir) {
    await switchWorkspace(appPage, workspace.id, workspace.name);
  }
  const created = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspace.id }),
  });
  const sessionId = created.session.id;
  await appPage.goto(runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const window = sessionWindow(appPage, sessionId);
  const ui = sessionWindowSelectors(window, appPage);
  await ui.composerTextarea.fill(prompt);
  await ui.composerTextarea.press("Enter");
  return { page: appPage, runtime, workspaceId: workspace.id, sessionId, window };
}

function card(scene: Scene, title: string): Locator {
  return scene.window.locator('[data-slot="subagent-card"]').filter({ hasText: title });
}

async function childSessionId(scene: Scene, title: string): Promise<string> {
  let childId = "";
  await expect
    .poll(async () => {
      const list = await scene.runtime.requestJSON<{
        subagents: { title: string; child_session_id: string | null }[];
      }>(
        `/api/workspaces/${scene.workspaceId}/sessions/${scene.sessionId}/subagents?origin=delegated`
      );
      childId = list.subagents.find(row => row.title === title)?.child_session_id ?? "";
      return childId;
    })
    .not.toBe("");
  return childId;
}

// A settled turn folds its settled cards behind "Worked for …" (UT-W04); open every fold.
async function openFolds(scene: Scene): Promise<void> {
  const closed = scene.window.locator('[data-testid="turn-fold-row"][aria-expanded="false"]');
  while ((await closed.count()) > 0) await closed.first().click();
}

async function subagentStatuses(scene: Scene): Promise<string[]> {
  const list = await scene.runtime.requestJSON<{ subagents: { status: string }[] }>(
    `/api/workspaces/${scene.workspaceId}/sessions/${scene.sessionId}/subagents`
  );
  return list.subagents.map(row => row.status).sort();
}

const reviewTitle = "Review the diff panel default";

test("E2E-001: capabilities and a delegation render as a tool phrase and a live card that settles", async ({
  appPage,
  runtime,
}) => {
  const scene = await openSession(appPage, runtime, parentAgent, "get a second opinion");
  const review = card(scene, reviewTitle);

  await expect(review).toHaveAttribute("data-status", "running");
  await expect(review).toContainText("Working");
  // The async delegation ends the parent turn: its tool work folds, the live card stays out.
  await openFolds(scene);
  await expect(scene.window.getByText("Checked subagent capabilities")).toBeVisible();
  // The card replaces the delegate call: no tool row for it.
  await expect(scene.window.getByText(/Delegat(ed|ing) a subagent/)).toHaveCount(0);
  // Elapsed ticks from the daemon's started_at while the child works.
  const elapsed = review.locator('[data-slot="subagent-elapsed"]');
  const first = await elapsed.textContent();
  await expect.poll(async () => elapsed.textContent(), { timeout: 4_000 }).not.toBe(first);

  // The completion wakes the parent; its reply follows, and the settled card folds.
  await expect(scene.window.getByText("Subagent results received.")).toBeVisible({
    timeout: 30_000,
  });
  await openFolds(scene);
  await expect(review).toHaveAttribute("data-status", "completed");
  await expect(review).toContainText("Recommendation: keep branch as the default.");
  const frozen = await elapsed.textContent();
  await appPage.waitForTimeout(1_500);
  await expect(elapsed).toHaveText(frozen ?? "");
});

test("E2E-002: three same-turn delegations group, keep their expansion, and summarize settles", async ({
  appPage,
  runtime,
}) => {
  const scene = await openSession(appPage, runtime, parentAgent, "split the audit three ways");
  const group = scene.window.locator('[data-slot="subagent-group"]');
  await expect(group).toContainText("3 subagents");
  await expect(group.locator('[data-slot="subagent-group-summary"]')).toHaveText("3 working");

  const trigger = group.getByRole("button", { name: /3 subagents/ });
  await trigger.click();
  await expect(group.locator('[data-slot="subagent-card"]')).toHaveCount(3);
  await trigger.click();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");

  await expect
    .poll(async () => subagentStatuses(scene), { timeout: 40_000 })
    .toEqual(["completed", "completed", "failed"]);
  await openFolds(scene);
  await expect(group.locator('[data-slot="subagent-group-summary"]')).toHaveText(
    "2 done · 1 failed"
  );
  await expect(group.locator('[data-slot="subagent-group-summary"]')).toHaveAttribute(
    "data-tone",
    "failed"
  );
  // The reader's collapse held while the members settled.
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
});

test("E2E-003: hover and keyboard focus open the hover card; Escape closes it", async ({
  appPage,
  runtime,
}) => {
  const scene = await openSession(appPage, runtime, parentAgent, "get a second opinion");
  const review = card(scene, reviewTitle);
  // Hover once the parent's turn ended and its message reconciled, while the child still works.
  await expect(
    scene.window.getByText("A subagent is reviewing it; I'll pick up its answer when it finishes.")
  ).toBeVisible();
  await expect(review).toHaveAttribute("data-status", "running");
  const hover = appPage.locator('[data-slot="subagent-hover"]');

  await review.hover();
  await expect(hover).toBeVisible();
  await expect(hover).toContainText(reviewTitle);
  // The target named no model: the hover card says so instead of guessing.
  await expect(hover.locator('[data-slot="subagent-hover-runtime"]')).toContainText("Not reported");
  await expect(hover.locator('[data-slot="subagent-hover-status"]')).toContainText("Working");
  await appPage.mouse.move(0, 0);
  await expect(hover).toHaveCount(0);

  // The card is a container; its open control is what keyboard focus lands on.
  await review.locator('[data-slot="subagent-card-open"]').focus();
  await expect(hover).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(hover).toHaveCount(0);
});

test("E2E-004: a card opens a read-only drawer, Open as session navigates, and modifier-click splits", async ({
  appPage,
  runtime,
  browserArtifacts,
}) => {
  await appPage.setViewportSize({ width: 1600, height: 1000 });
  const scene = await openSession(appPage, runtime, parentAgent, "get a second opinion");
  const review = card(scene, reviewTitle);
  await expect(review).toBeVisible();
  const childId = await childSessionId(scene, reviewTitle);

  await sessionWindowSelectors(scene.window, appPage).composerTextarea.fill(
    "Keep this unsent draft"
  );

  // Invariant: inspection preserves the parent; explicit navigation retains the child session.
  // Owner/canonical suite: this browser/daemon journey.
  await review.getByRole("button", { name: `Open ${reviewTitle}`, exact: true }).click();
  const drawer = appPage.locator('[data-slot="resizable-drawer"]');
  await expect(drawer).toBeVisible();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await expect(drawer).toHaveCSS("opacity", "1");
  await expect(scene.window).toBeVisible();
  await expect(drawer.getByRole("textbox")).toHaveCount(0);
  await browserArtifacts.captureScreenshot("subagent-card-drawer-dark", appPage);
  const resize = scene.window.getByRole("separator", { name: "Resize drawer" });
  const beforeResize = await drawer.boundingBox();
  const handle = await resize.boundingBox();
  await appPage.mouse.move(handle!.x + handle!.width / 2, handle!.y + 120);
  await appPage.mouse.down();
  await appPage.mouse.move(handle!.x - 100, handle!.y + 120, { steps: 8 });
  await appPage.mouse.up();
  await expect
    .poll(async () => (await drawer.boundingBox())?.width ?? 0)
    .toBeGreaterThan(beforeResize!.width);
  const resizedWidth = (await drawer.boundingBox())!.width;
  await browserArtifacts.captureScreenshot("subagent-drawer-resized", appPage);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await expect(sessionWindowSelectors(scene.window, appPage).composerTextarea).toHaveText(
    "Keep this unsent draft"
  );
  await scene.window.getByTestId("composer-subagents-button").click();
  await browserArtifacts.captureScreenshot("subagent-composer-roster-dark", appPage);
  await appPage
    .getByRole("button", { name: `Open ${reviewTitle}`, exact: true })
    .last()
    .click();
  await expect(drawer).toBeVisible();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await expect(drawer).toHaveCSS("opacity", "1");
  expect((await drawer.boundingBox())!.width).toBeCloseTo(resizedWidth, 0);
  await browserArtifacts.captureScreenshot("subagent-composer-drawer-dark", appPage);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await appPage.getByRole("button", { name: "Switch to light mode" }).click();
  await openFolds(scene);
  await card(scene, reviewTitle)
    .getByRole("button", { name: `Open ${reviewTitle}`, exact: true })
    .click();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await browserArtifacts.captureScreenshot("subagent-card-drawer-light", appPage);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await scene.window.getByTestId("composer-subagents-button").click();
  await browserArtifacts.captureScreenshot("subagent-composer-roster-light", appPage);
  await appPage
    .getByRole("button", { name: `Open ${reviewTitle}`, exact: true })
    .last()
    .click();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await expect(drawer).toHaveCSS("opacity", "1");
  await browserArtifacts.captureScreenshot("subagent-composer-drawer-light", appPage);
  await appPage.setViewportSize({ width: 1000, height: 800 });
  await expect(drawer).toHaveAttribute("data-mode", "overlay");
  await expect(drawer).toHaveCSS("opacity", "1");
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await browserArtifacts.captureScreenshot("subagent-drawer-narrow-light", appPage);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await appPage.getByRole("button", { name: "Switch to dark mode" }).click();
  await openFolds(scene);
  await card(scene, reviewTitle)
    .getByRole("button", { name: `Open ${reviewTitle}`, exact: true })
    .click();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await expect(drawer).toHaveCSS("opacity", "1");
  await browserArtifacts.captureScreenshot("subagent-drawer-narrow-dark", appPage);
  await drawer.getByRole("button", { name: "Open as session" }).click();
  const child = sessionWindow(appPage, childId);
  await expect(child).toBeVisible();
  const divider = child.locator('[data-testid="session-origin-divider"][data-kind="subagent"]');
  await expect(divider).toContainText("Subagent of");
  await expect(sessionWindowSelectors(child, appPage).composerTextarea).toBeVisible();

  await divider.getByRole("button", { name: "Open parent" }).click();
  await expect(scene.window).toBeVisible();

  await openFolds(scene);
  await card(scene, reviewTitle)
    .getByRole("button", { name: `Open ${reviewTitle}`, exact: true })
    .click({ modifiers: ["ControlOrMeta"] });
  await expect(sessionWindow(appPage, childId)).toBeVisible();
  await expect(scene.window).toBeVisible();
});

test("E2E-005: an idle parent waits on two subagents, shows the delegated glyph, and Stop cancels both", async ({
  appPage,
  runtime,
}) => {
  const scene = await openSession(appPage, runtime, parentAgent, "start two long audits");
  await expect(scene.window.getByText("Two subagents are on it.")).toBeVisible();

  const banner = scene.window.locator('[data-slot="subagent-waiting-banner"]');
  await expect(banner).toContainText("Waiting on 2 subagents");
  await expect(
    banner.getByRole("button", { name: "Open subagent Long audit of webhooks" })
  ).toBeVisible();
  await expect(
    banner.getByRole("button", { name: "Open subagent Long audit of refunds" })
  ).toBeVisible();
  // The sidebar's parent row reads delegated, not done.
  await expect(
    scene.window.locator('[data-segment="subagents"][data-state="delegated"]')
  ).toBeVisible();

  await banner.getByRole("button", { name: "Stop" }).click();
  await expect(banner).toHaveCount(0, { timeout: 30_000 });
});

test("E2E-006: native sidebar and transcript cards open live activity without a child session", async ({
  appPage,
  runtime,
  browserArtifacts,
}) => {
  await appPage.setViewportSize({ width: 1600, height: 1000 });
  const scene = await openSession(appPage, runtime, nativeAgent, "review the diff with a subagent");
  const native = scene.window.locator('[data-slot="subagent-card"][data-origin="provider_native"]');
  await expect(native).toContainText("Review the diff (high effort)");
  // Invariant: native inspection shows only its attributed work, without a child session.
  // Owner/canonical suite: this real daemon journey with pinned Claude adapter frames.
  const sidebar = scene.window.getByTestId("session-sidebar");
  await sidebar.getByRole("button", { name: /1 (?:of 1 )?subagent(?:s running)?$/ }).click();
  const branch = sidebar.getByTestId(`session-sidebar-subagents-${scene.sessionId}`);
  await branch.getByRole("button", { name: "Open Review the diff (high effort)" }).click();
  const drawer = appPage.locator('[data-slot="resizable-drawer"]');
  await expect(drawer).toBeVisible();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await expect(drawer).toHaveCSS("opacity", "1");
  await expect(drawer.getByTestId("native-subagent-activity")).toContainText(
    "Inspecting the diff."
  );
  await expect(drawer.getByRole("button", { name: "Open as session" })).toHaveCount(0);
  await openNativeResult(drawer);
  await browserArtifacts.captureScreenshot("native-subagent-sidebar-drawer", appPage);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await openFolds(scene);
  await native.getByRole("button", { name: "Open Review the diff (high effort)" }).click();
  await expect(drawer).toBeVisible();
  await expect(drawer).toBeInViewport({ ratio: 1 });
  await expect(drawer).toHaveCSS("opacity", "1");
  await expect(drawer.getByTestId("native-subagent-activity")).toContainText(
    "Inspecting the diff."
  );
  await openNativeResult(drawer);
  await browserArtifacts.captureScreenshot("native-subagent-card-drawer", appPage);
  await expect(scene.window.getByText("The reviewer found no issues.")).toBeVisible();
  await expect(drawer).toContainText("Done");
  await expect(drawer.getByRole("textbox")).toHaveCount(0);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await appPage.getByRole("button", { name: "Switch to light mode" }).click();
  await branch.getByRole("button", { name: "Open Review the diff (high effort)" }).click();
  await expect(drawer).toContainText("Inspecting the diff.");
  await openNativeResult(drawer);
  await browserArtifacts.captureScreenshot("native-subagent-sidebar-drawer-light", appPage);
  await appPage.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await openFolds(scene);
  await native.getByRole("button", { name: "Open Review the diff (high effort)" }).click();
  await expect(drawer).toContainText("Inspecting the diff.");
  await openNativeResult(drawer);
  await browserArtifacts.captureScreenshot("native-subagent-card-drawer-light", appPage);
});

async function openNativeResult(drawer: Locator) {
  await drawer.getByRole("button", { name: /^Ran agent/ }).click();
  await expect(drawer.getByText("No issues found.", { exact: true })).toBeVisible();
}

interface IsolatedWorktree {
  branch: string;
  base_sha?: string;
}

async function isolatedWorktree(scene: Scene, title: string): Promise<IsolatedWorktree> {
  let worktree: IsolatedWorktree | undefined;
  await expect
    .poll(async () => {
      const list = await scene.runtime.requestJSON<{
        subagents: { title: string; isolation: string; worktree?: IsolatedWorktree }[];
      }>(`/api/workspaces/${scene.workspaceId}/sessions/${scene.sessionId}/subagents`);
      const row = list.subagents.find(candidate => candidate.title === title);
      worktree = row?.isolation === "worktree" ? row.worktree : undefined;
      return worktree?.branch ?? "";
    })
    .not.toBe("");
  return worktree!;
}

const isolatedTitle = "Extract billing client";

test("Agent collaboration E2E-004: an isolated subagent shows its branch while running, then its observed facts", async ({
  appPage,
  runtime,
}) => {
  const repo = await createWorktreeRepo();
  try {
    const scene = await openSession(
      appPage,
      runtime,
      parentAgent,
      "deliver the client on its own branch",
      repo.rootDir
    );
    const isolated = card(scene, isolatedTitle);
    await expect(isolated).toHaveAttribute("data-status", "running");
    const worktree = await isolatedWorktree(scene, isolatedTitle);
    expect(worktree.branch).toMatch(/^run\/extract-billing-client-[0-9a-f]{8}$/);
    // US-016.AC-1: the branch is on line two from creation, full name in its title.
    await expect(isolated.locator('[data-slot="subagent-branch"]')).toHaveAttribute(
      "title",
      worktree.branch
    );

    await expect(scene.window.getByText("Subagent results received.")).toBeVisible({
      timeout: 60_000,
    });
    await openFolds(scene);
    const settled = card(scene, isolatedTitle);
    await expect(settled).toHaveAttribute("data-status", "completed");
    // The fixture has no forge provider: no PR link, and the hover says unknown, never none.
    await expect(settled.locator('[data-slot="subagent-pr-link"]')).toHaveCount(0);

    await settled.hover();
    const facts = appPage.locator('[data-slot="subagent-hover-facts"]');
    await expect(facts).toBeVisible();
    await expect(facts).toContainText(worktree.branch);
    await expect(facts).toContainText(`Base${worktree.base_sha!.slice(0, 7)}`);
    await expect(facts).toContainText("1 ahead·clean");
    await expect(facts).toContainText("PR status unknown");
    await expect(facts).toContainText("Observed");
  } finally {
    await repo.cleanup();
  }
});
