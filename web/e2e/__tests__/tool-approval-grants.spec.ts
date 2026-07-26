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
import { sessionWindowSelectors, toolApprovalGrantsSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const MOCK_AGENT = "mock-tool-approval-grants";
const FIXTURE_AGENT = "tool-approval-grants";
const REVOKE_TOOL = "agh__tool_approvals_revoke";
const WIDER_TOOL = "agh__workspace_list";

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
    throw new Error("tool approval grants E2E requires a launch-mode runtime with paths");
  }
}

interface SessionEnvelope {
  session: { id: string; agent_name: string; workspace_id: string };
}

interface ToolApprovalGrant {
  id: string;
  workspace_id: string;
  agent_name?: string;
  tool_id: string;
  input_digest?: string;
  decision: "allow" | "reject";
}

interface ToolApprovalGrantList {
  grants: ToolApprovalGrant[];
  total: number;
}

function grantsPath(workspaceId: string): string {
  return `/api/tool-approval-grants?workspace_id=${encodeURIComponent(workspaceId)}`;
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

test("operator remembers a native-tool decision and revokes it end to end", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const grantsUI = toolApprovalGrantsSelectors(appPage);

  // The session's workspace is the global workspace, which is also the browser's active
  // workspace, so the remembered decision is visible in General Settings.
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
    // The native-tool permission event only renders while a prompt stream is active
    // (emitPromptEvent needs a non-nil active prompt). Submit a prompt that reports readiness
    // then blocks until cancelled, so the hosted-tool permission surfaces in that live stream.
    await expect(sessionWin).toBeVisible({ timeout: 20_000 });
    await sessionUI.composerTextarea.fill("hold native approval");
    await sessionUI.composerTextarea.press("Enter");
    await expect(appPage.getByText("native approval ready")).toBeVisible({ timeout: 20_000 });
    await expect(sessionUI.stopButton).toBeVisible({ timeout: 20_000 });

    // A session-bound native tool that requires approval (approve-reads gates this mutating
    // builtin). A nonexistent grant id returns a not-found error AFTER approval, so the run
    // creates no unrelated state. The call blocks until the operator answers in the browser.
    const pendingCall = client.callTool({
      name: REVOKE_TOOL,
      arguments: { id: "e2e-nonexistent-grant", workspace_id: workspace.id },
      _meta: { toolCallId: "e2e-tool-approval-revoke" },
    });
    toolCall = pendingCall;

    await expect(sessionUI.permissionPrompt).toBeVisible({ timeout: 30_000 });
    // The prompt shows a humanized tool title; assert the unique tool input instead so this
    // is unambiguously our hosted-tool call.
    await expect(sessionUI.permissionPrompt).toContainText("e2e-nonexistent-grant");
    const approvePromise = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response
          .url()
          .endsWith(
            `/api/workspaces/${encodeURIComponent(workspace.id)}/sessions/${encodeURIComponent(
              sessionId
            )}/approve`
          )
    );
    await sessionUI.permissionAllowAlways.click();
    expect((await approvePromise).ok()).toBe(true);
    await expect(sessionUI.permissionPrompt).toBeHidden();

    const callResult = await pendingCall;
    expect(callResult.isError).toBe(true);

    // Cancel the blocked holding turn via the public stop control, then wait for an operable
    // session state (do not infer cancellation from the stop button's visibility).
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

    // Daemon truth: exactly one remembered decision, exact agent + exact sha256 input.
    const seeded = await runtime.requestJSON<ToolApprovalGrantList>(grantsPath(workspace.id));
    expect(seeded.total).toBe(1);
    const grant = seeded.grants[0]!;
    expect(grant.tool_id).toBe(REVOKE_TOOL);
    expect(grant.decision).toBe("allow");
    expect(grant.agent_name).toBe(MOCK_AGENT);
    expect(grant.input_digest?.startsWith("sha256:")).toBe(true);

    // The exact grant appears in General Settings for the active workspace.
    await appPage.goto(runtime.url("/settings/general"), { waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("settings-page-general")).toBeVisible({ timeout: 20_000 });
    await expect(grantsUI.row(grant.id)).toBeVisible();
    await expect(grantsUI.decision(grant.id)).toHaveText(/allow/i);

    // Revoke through the confirmation flow; the row disappears after the daemon refetch.
    await grantsUI.revoke(grant.id).click();
    await grantsUI.revokeConfirm.click();
    await expect(grantsUI.row(grant.id)).toBeHidden();
    await expect(grantsUI.empty).toBeVisible();

    // Daemon truth is empty via the public API seam (not SQLite).
    const afterRevoke = await runtime.requestJSON<ToolApprovalGrantList>(grantsPath(workspace.id));
    expect(afterRevoke.total).toBe(0);
    expect(afterRevoke.grants).toEqual([]);

    // Explicit wider creation is a separate public path: choose scope and decision in Web,
    // then verify the stored daemon key through the workspace-scoped API.
    await grantsUI.setOpen.click();
    await expect(grantsUI.setDialog).toBeVisible();
    await expect(grantsUI.setConfirm).toBeDisabled();
    await grantsUI.setScopeAgent.click();
    await grantsUI.setToolID.fill(WIDER_TOOL);
    await grantsUI.setAgentName.fill(MOCK_AGENT);
    await grantsUI.setDecisionAllow.click();
    await grantsUI.setConfirm.click();
    await expect(grantsUI.setDialog).toBeHidden();

    const afterWiderSet = await runtime.requestJSON<ToolApprovalGrantList>(
      grantsPath(workspace.id)
    );
    expect(afterWiderSet.total).toBe(1);
    const widerGrant = afterWiderSet.grants[0]!;
    expect(widerGrant.tool_id).toBe(WIDER_TOOL);
    expect(widerGrant.decision).toBe("allow");
    expect(widerGrant.agent_name).toBe(MOCK_AGENT);
    expect(widerGrant.input_digest).toBeUndefined();
    await expect(grantsUI.row(widerGrant.id)).toBeVisible();
    await expect(appPage.getByTestId(`tool-approval-grant-scope-${widerGrant.id}`)).toHaveText(
      "agent-wide"
    );
  } finally {
    await teardownHostedMcp(connection, toolCall);
  }
});
