import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Client } from "@modelcontextprotocol/sdk/client/index.js";

import {
  connectHostedMcpClient,
  readHostedMcpDescriptor,
  teardownHostedMcp,
  type HostedMcpConnection,
} from "../fixtures/hosted-mcp";
import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { sessionWindow } from "../fixtures/os-navigation";
import { sessionClarifySelectors, sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

// Reuse the proven holding-turn fixture: `hold native approval` reports readiness then blocks until
// cancelled, keeping the session live while a hosted-MCP tool call surfaces its interaction. Clarify
// is read-only (`RiskRead`, no interaction gate), so `approve-reads` does not open a permission prompt.
const MOCK_AGENT = "mock-session-clarify";
const FIXTURE_AGENT = "tool-approval-grants";
const CLARIFY_TOOL = "agh__clarify";
const CLARIFY_CHOICES = ["Staging", "Production"];
const CHOSEN_INDEX = 1;

function repoFixture(name: string): string {
  return path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "../../..",
    "internal/testutil/acpmock/testdata",
    name
  );
}

function assertLaunchRuntime(
  runtime: BrowserRuntime
): asserts runtime is BrowserRuntime & { paths: RuntimePaths } {
  if (runtime.mode !== "launch" || !runtime.paths) {
    throw new Error("session clarify E2E requires a launch-mode runtime with paths");
  }
}

interface SessionEnvelope {
  session: { id: string; agent_name: string; workspace_id: string };
}

interface ClarifyStructuredResult {
  choice: number | null;
  text: string;
  fallback: boolean;
}

interface ClarificationsList {
  clarifications: unknown[];
}

function clarificationsPath(workspaceId: string, sessionId: string): string {
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(
    sessionId
  )}/clarifications`;
}

/**
 * Extract the frozen `{choice, text, fallback}` answer from the MCP tool result. Prefers the typed
 * `structuredContent` (the tool declares an output schema) and falls back to the serialized text block.
 */
function readClarifyResult(
  result: Awaited<ReturnType<Client["callTool"]>>
): ClarifyStructuredResult {
  const structured = result.structuredContent as ClarifyStructuredResult | undefined;
  if (structured && typeof structured === "object") {
    return structured;
  }
  const content = Array.isArray(result.content) ? result.content : [];
  for (const block of content) {
    if (block.type === "text" && typeof block.text === "string") {
      return JSON.parse(block.text) as ClarifyStructuredResult;
    }
  }
  throw new Error("clarify callTool result carried no structured answer");
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: repoFixture("tool_approval_grants_fixture.json"),
          fixtureAgent: FIXTURE_AGENT,
          agentName: MOCK_AGENT,
        },
      ],
    },
  },
});

test("operator answers a running clarification and unblocks the hosted-MCP call", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const clarifyUI = sessionClarifySelectors(appPage);

  await useGlobalWorkspaceIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);

  const created = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: MOCK_AGENT, workspace: workspace.id }),
  });
  const sessionId = created.session.id;

  // Connect the Node MCP client promptly so the single-use bind nonce is still valid.
  const diagnosticsPath = path.join(
    runtime.paths.homeDir,
    "logs",
    "acpmock",
    `${MOCK_AGENT}.jsonl`
  );
  const descriptor = await readHostedMcpDescriptor(diagnosticsPath);

  let connection: HostedMcpConnection | null = null;
  let toolCall: ReturnType<Client["callTool"]> | undefined;
  try {
    connection = await connectHostedMcpClient(descriptor);
    const client = connection.client;

    await appPage.goto(runtime.url(`/agents/${MOCK_AGENT}/sessions/${sessionId}`), {
      waitUntil: "domcontentloaded",
    });
    const sessionWin = sessionWindow(appPage, sessionId);
    const sessionUI = sessionWindowSelectors(sessionWin, appPage);
    // Keep the session live: the holding turn reports readiness then blocks until cancelled.
    await expect(sessionWin).toBeVisible({ timeout: 20_000 });
    await sessionUI.composerTextarea.fill("hold native approval");
    await sessionUI.composerTextarea.press("Enter");
    await expect(appPage.getByText("native approval ready")).toBeVisible({ timeout: 20_000 });
    await expect(sessionUI.stopButton).toBeVisible({ timeout: 20_000 });

    // Ask one bounded question through the hosted MCP client. Read-only: no permission prompt opens;
    // the call blocks until the operator answers in the browser.
    const pendingCall = client.callTool({
      name: CLARIFY_TOOL,
      arguments: { question: "Which environment should deploy first?", choices: CLARIFY_CHOICES },
      _meta: { toolCallId: "e2e-clarify-1" },
    });
    toolCall = pendingCall;

    // The clarify SSE event wakes the live pending query and the question card renders.
    await expect(clarifyUI.card).toBeVisible({ timeout: 30_000 });
    await expect(clarifyUI.question).toContainText("Which environment should deploy first?");
    await expect(clarifyUI.choice(CHOSEN_INDEX)).toBeVisible();
    await expect(sessionUI.permissionPrompt).toBeHidden();

    const answerPromise = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().includes(`/sessions/${encodeURIComponent(sessionId)}/clarifications/`) &&
        response.url().endsWith("/answer")
    );
    await clarifyUI.choice(CHOSEN_INDEX).click();
    expect((await answerPromise).ok()).toBe(true);
    await expect(clarifyUI.card).toBeHidden();

    // Terminal SSE evidence: the resolved event renders a durable, non-interactive receipt carrying the
    // selected label — proving the timeline reflects broker truth, not just local card hiding.
    await expect(clarifyUI.receipt).toBeVisible({ timeout: 20_000 });
    expect(await clarifyUI.receipt.getAttribute("data-status")).toBe("resolved");
    await expect(clarifyUI.receiptAnswer).toHaveText(CLARIFY_CHOICES[CHOSEN_INDEX]!);

    // The original callTool promise resolves with the exact frozen structured answer.
    const callResult = await pendingCall;
    expect(callResult.isError).toBeFalsy();
    expect(readClarifyResult(callResult)).toEqual({
      choice: CHOSEN_INDEX,
      text: "",
      fallback: false,
    });

    // Daemon truth: the live pending projection is empty after the exact answer.
    const pending = await runtime.requestJSON<ClarificationsList>(
      clarificationsPath(workspace.id, sessionId)
    );
    expect(pending.clarifications).toEqual([]);

    // Cancel the blocked holding turn via the public stop control, then wait for an operable state.
    const sessionBase = `/api/workspaces/${encodeURIComponent(
      workspace.id
    )}/sessions/${encodeURIComponent(sessionId)}`;
    const stopResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        (response.url().endsWith(`${sessionBase}/prompt/cancel`) ||
          response.url().endsWith(`${sessionBase}/stop`))
    );
    await expect(sessionUI.stopButton).toBeVisible();
    await sessionUI.stopButton.click();
    expect((await stopResponse).ok()).toBe(true);
    await sessionUI.topbarOverflow.click();
    await expect(sessionUI.composerClearButton).toBeEnabled({ timeout: 60_000 });
  } finally {
    await teardownHostedMcp(connection, toolCall);
  }
});
