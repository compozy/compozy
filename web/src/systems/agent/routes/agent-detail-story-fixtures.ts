import type { SessionPayload } from "@/systems/session/types";
import { sessionFixtures } from "@/systems/session/mocks";
import {
  storyAgentNames,
  storySessionIds,
  storyWorkspaceIds,
  storyWorkspacePaths,
} from "@/storybook/fintech-scenario";
import { agentFixtures } from "@/systems/agent/mocks";
import type { AgentPayload } from "@/systems/agent/types";
import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { HttpResponse } from "msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

export const fraudSessions: SessionPayload[] = sessionFixtures.filter(
  session => session.agent_name === storyAgentNames.fraud
);

const fallbackFraudSession: SessionPayload = {
  id: storySessionIds.fraud,
  name: "Payout hold triage",
  agent_name: storyAgentNames.fraud,
  provider: "claude",
  workspace_id: storyWorkspaceIds.risk,
  workspace_path: storyWorkspacePaths.risk,
  state: "active",
  badge: "idle",
  attachable: true,
  available_commands: [],
  created_at: "2026-04-17T16:00:00Z",
  updated_at: "2026-04-17T18:10:00Z",
};

export const failureBaseSession = fraudSessions[0] ?? fallbackFraudSession;

export function sessionPage(sessions: SessionPayload[]) {
  return {
    sessions,
    page: { has_more: false, limit: 50, total: sessions.length },
  };
}

export function failedSessionPage() {
  return sessionPage([
    ...fraudSessions,
    {
      ...failureBaseSession,
      id: "sess_fraud_failed",
      name: "Settlement export retry",
      state: "stopped" as const,
      stop_reason: "agent_crashed" as const,
      failure: {
        kind: "agent_crashed",
        summary: "partner settlement export terminated unexpectedly",
      },
      updated_at: "2026-04-17T18:42:00Z",
    },
  ]);
}

export const fraudAgentRoute = `/agents/${storyAgentNames.fraud}`;

export const complianceAgentRoute = `/agents/${storyAgentNames.compliance}`;

export const missingAgentRoute = "/agents/ghost-risk-agent";

const fraudAgentBase = agentFixtures.find(agent => agent.name === storyAgentNames.fraud)!;

/** Reference-density payload for OpenDesign tab parity captures. */
const denseFraudAgent: AgentPayload = {
  ...fraudAgentBase,
  prompt: `# Role

You are \`${storyAgentNames.fraud}\`, the operator agent for payout holds and reserve anomalies.

## Operating rules

- Prefer verified daemon state over assumptions.
- Never invent metrics or claim a hold cleared unless the runtime confirms it.
- Keep permissions at approve-reads unless the operator raises them.

## Outputs

Produce concrete next steps: files to touch, the hold id, and the exact \`agh\` commands to run.`,
  permissions: "approve-reads",
  tools: ["Bash", "Read", "Edit", "Write", "Glob", "Grep"],
  deny_tools: ["WebFetch", "Bash(rm:*)"],
  toolsets: ["agh-core"],
  mcp_servers: [
    {
      name: "github",
      transport: "stdio",
      command: "npx -y @modelcontextprotocol/server-github",
      env: { GITHUB_TOKEN: "redacted" },
    },
    {
      name: "linear",
      transport: "sse",
      url: "https://mcp.linear.app/sse",
      env: { LINEAR_API_KEY: "redacted" },
    },
  ],
  skills: { disabled: [] },
};

export function denseFraudAgentHandlers() {
  return storybookMswParameters({
    agent: [
      aghApiMock.get("/api/agents/{name}", () => HttpResponse.json({ agent: denseFraudAgent })),
    ],
  });
}

export async function openDetailRuntimeSelector(canvasElement: HTMLElement) {
  const body = within(canvasElement.ownerDocument.body);
  const trigger = await body.findByTestId("agent-detail-runtime-select");
  const openButton =
    within(trigger).queryByRole("button", { name: /^Open runtime selector$/i }) ??
    within(trigger).queryByRole("button", { name: /^Model:/i }) ??
    within(trigger).getByRole("button", { name: /^Provider:/i });
  await waitFor(() => expect(openButton).toBeEnabled(), { timeout: 15_000 });
  await userEvent.click(openButton);
  await expect(await body.findByTestId("runtime-selector-popup")).toBeInTheDocument();
  return body;
}

export async function chooseAlternateRuntimeOption(popupRoot: HTMLElement) {
  const popup = within(popupRoot);
  const option = popup
    .getAllByRole("option")
    .find(
      entry =>
        !entry.hasAttribute("aria-selected") || entry.getAttribute("aria-selected") !== "true"
    );
  if (!option) throw new Error("expected an alternate runtime option");
  await userEvent.click(option);
}
