import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// Subagents (`.compozy/tasks/subagents/_tests.md` E2E-001…E2E-006).
// E2E-006 replays the pinned Claude Agent adapter frames (`internal/acp/testdata/
// claude_agent_subagent.jsonl`, session id rebound to the mock's) through acpmock.
// E2E-004/005 create delegated subagents through the operator tool route, the same
// `compozy__subagent_delegate` call an agent makes. E2E-001…003 are not here: they need the
// parent agent itself to call the hosted native tool so the delegate call and its result land
// in the parent transcript, and acpmock has no step that calls a hosted tool.
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_native_subagent_fixture.json"
);
const nativeAgent = "subagent-native";
const parentAgent = "subagent-parent";
const workerAgent = "subagent-worker";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath, fixtureAgent: nativeAgent },
        { fixturePath, fixtureAgent: parentAgent },
        { fixturePath, fixtureAgent: workerAgent },
      ],
    },
  },
});

interface CreatedSession {
  session: { id: string; name?: string };
}

async function createSession(runtime: BrowserRuntime, agentName: string, workspaceId: string) {
  const created = await runtime.requestJSON<CreatedSession>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspaceId }),
  });
  return created.session.id;
}

async function delegate(
  runtime: BrowserRuntime,
  workspaceId: string,
  parentSessionId: string,
  title: string
): Promise<void> {
  await runtime.requestJSON("/api/tools/compozy__subagent_delegate/invoke", {
    method: "POST",
    body: JSON.stringify({
      session_id: parentSessionId,
      workspace_id: workspaceId,
      input: {
        task: `${title}. Report what you find.`,
        title,
        target: { agent: workerAgent },
        mode: "async",
        idempotency_key: title,
      },
    }),
  });
}

test("E2E-006: a provider-native subagent is a card whose inner work stays inside it", async ({
  appPage,
  runtime,
}) => {
  if (!runtime.paths) throw new Error("subagent E2E requires an isolated runtime");
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await completeOnboardingIfPrompted(appPage);
  const sessionId = await createSession(runtime, nativeAgent, workspace.id);
  await appPage.goto(runtime.url(`/agents/${nativeAgent}/sessions/${sessionId}`));
  const window = sessionWindow(appPage, sessionId);
  const ui = sessionWindowSelectors(window, appPage);
  await ui.composerTextarea.fill("review the diff with a subagent");
  await ui.composerTextarea.press("Enter");

  const card = window.locator('[data-slot="subagent-card"][data-origin="provider_native"]');
  await expect(card).toContainText("Review the diff (high effort)");
  // While the Agent call runs the status line counts it once.
  await expect(window.getByText("1 agent running")).toBeVisible();
  // No child session: a disclosure for its inner work, never a drill-in.
  await expect(
    window.getByRole("button", { name: "Open Review the diff (high effort)" })
  ).toHaveCount(0);
  await card.click();
  const nested = window.locator('[data-slot="subagent-card-nested"]');
  await expect(nested).toBeVisible();
  await expect(nested).toContainText("Inspecting the diff.");
  // The parent's own flow never shows the subagent's inner text.
  await expect(window.getByText("Inspecting the diff.")).toHaveCount(1);

  await expect(window.getByText("The reviewer found no issues.")).toBeVisible();
  await expect(card).toHaveAttribute("data-status", "completed");
});

test("E2E-005 and E2E-004: an idle parent waits on its subagents, opens one, and stops them", async ({
  appPage,
  runtime,
}) => {
  if (!runtime.paths) throw new Error("subagent E2E requires an isolated runtime");
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await completeOnboardingIfPrompted(appPage);
  const parentId = await createSession(runtime, parentAgent, workspace.id);
  await appPage.goto(runtime.url(`/agents/${parentAgent}/sessions/${parentId}`));
  const window = sessionWindow(appPage, parentId);
  const ui = sessionWindowSelectors(window, appPage);
  await ui.composerTextarea.fill("start the audit");
  await ui.composerTextarea.press("Enter");
  await expect(window.getByText("Two subagents are on it.")).toBeVisible();

  await delegate(runtime, workspace.id, parentId, "Audit payment webhooks");
  await delegate(runtime, workspace.id, parentId, "Audit refund handlers");

  const banner = window.locator('[data-slot="subagent-waiting-banner"]');
  await expect(banner).toContainText("Waiting on 2 subagents");
  await expect(
    banner.getByRole("button", { name: "Open subagent Audit payment webhooks" })
  ).toBeVisible();
  await expect(
    banner.getByRole("button", { name: "Open subagent Audit refund handlers" })
  ).toBeVisible();

  // E2E-004: drill into a subagent in this window, then return to the parent.
  await banner.getByRole("button", { name: "Open subagent Audit payment webhooks" }).click();
  const divider = appPage.locator('[data-testid="session-origin-divider"][data-kind="subagent"]');
  await expect(divider).toBeVisible();
  await expect(divider).toContainText("Subagent of");
  await divider.getByRole("button", { name: "Open parent" }).click();
  await expect(window.locator('[data-slot="subagent-waiting-banner"]')).toBeVisible();

  await window
    .locator('[data-slot="subagent-waiting-banner"]')
    .getByRole("button", { name: "Stop" })
    .click();
  await expect(window.locator('[data-slot="subagent-waiting-banner"]')).toHaveCount(0);
});
