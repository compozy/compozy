import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { sessionWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// Subagents (`.compozy/tasks/subagents/_tests.md` E2E-001…E2E-006).
// The parent agent calls `compozy__subagent_capabilities` and `compozy__subagent_delegate`
// for real through its hosted MCP server (acpmock `native_tool_call`), so the delegate call,
// its `subagent_id` result and the card land in the parent transcript exactly as a provider's
// would. Workers answer after a delay, fail their prompt, or block until canceled. E2E-006
// replays the pinned Claude Agent adapter frames (`internal/acp/testdata/
// claude_agent_subagent.jsonl`, session id rebound to the mock's).
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
  prompt: string
): Promise<Scene> {
  if (!runtime.paths) throw new Error("subagent E2E requires an isolated runtime");
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
  await expect(review).toContainText("Running");
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
  await expect(review).toHaveAttribute("data-status", "running");
  const hover = appPage.locator('[data-slot="subagent-hover"]');

  await review.hover();
  await expect(hover).toBeVisible();
  await expect(hover).toContainText(reviewTitle);
  // The target named no model: the hover card says so instead of guessing.
  await expect(hover.locator('[data-slot="subagent-hover-runtime"]')).toContainText("Not reported");
  await expect(hover.locator('[data-slot="subagent-hover-status"]')).toContainText("Running");
  await appPage.mouse.move(0, 0);
  await expect(hover).toHaveCount(0);

  await review.focus();
  await expect(hover).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(hover).toHaveCount(0);
});

test("E2E-004: a card opens its child here with the Subagent of divider, Open parent returns, ⌘-click opens a new window", async ({
  appPage,
  runtime,
}) => {
  const scene = await openSession(appPage, runtime, parentAgent, "get a second opinion");
  const review = card(scene, reviewTitle);
  await expect(review).toBeVisible();
  const childId = await childSessionId(scene, reviewTitle);

  await review.click();
  const child = sessionWindow(appPage, childId);
  await expect(child).toBeVisible();
  const divider = child.locator('[data-testid="session-origin-divider"][data-kind="subagent"]');
  await expect(divider).toContainText("Subagent of");
  await expect(sessionWindowSelectors(child, appPage).composerTextarea).toBeVisible();

  await divider.getByRole("button", { name: "Open parent" }).click();
  await expect(scene.window).toBeVisible();

  await openFolds(scene);
  await card(scene, reviewTitle).click({ modifiers: ["ControlOrMeta"] });
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
    scene.window.locator('[data-slot="subagent-chip"][data-state="delegated"]')
  ).toBeVisible();

  await banner.getByRole("button", { name: "Stop" }).click();
  await expect(banner).toHaveCount(0, { timeout: 30_000 });
});

test("E2E-006: a provider-native subagent is a card whose inner work stays inside it", async ({
  appPage,
  runtime,
}) => {
  const scene = await openSession(appPage, runtime, nativeAgent, "review the diff with a subagent");
  const native = scene.window.locator('[data-slot="subagent-card"][data-origin="provider_native"]');
  await expect(native).toContainText("Review the diff (high effort)");
  // While the Agent call runs the status line counts it once.
  await expect(scene.window.getByText("1 agent running")).toBeVisible();
  // No child session: a disclosure for its inner work, never a drill-in.
  await expect(
    scene.window.getByRole("button", { name: "Open Review the diff (high effort)" })
  ).toHaveCount(0);
  await native.click();
  const nested = scene.window.locator('[data-slot="subagent-card-nested"]');
  await expect(nested).toBeVisible();
  await expect(nested).toContainText("Inspecting the diff.");
  // The parent's own flow never shows the subagent's inner text.
  await expect(scene.window.getByText("Inspecting the diff.")).toHaveCount(1);

  await expect(scene.window.getByText("The reviewer found no issues.")).toBeVisible();
  await openFolds(scene);
  await expect(native).toHaveAttribute("data-status", "completed");
});
