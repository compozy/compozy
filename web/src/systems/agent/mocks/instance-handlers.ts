import { http, HttpResponse, type HttpHandler } from "msw";

import type { AgentInstanceCheckIn, AgentInstancePayload } from "../types";
import { agentInstanceFixture } from "./instance-fixtures";

let instancesState: AgentInstancePayload[] = [];
let checkInsState: AgentInstanceCheckIn[] = [];

export function resetAgentInstanceMock(): void {
  instancesState = [];
  checkInsState = [];
}

export function seedAgentInstanceMock(
  instances: readonly AgentInstancePayload[],
  checkIns: readonly AgentInstanceCheckIn[] = []
): void {
  instancesState = structuredClone([...instances]);
  checkInsState = structuredClone([...checkIns]);
}

function respond(id: unknown, patch: Partial<AgentInstancePayload>) {
  const current = instancesState.find(instance => instance.id === id);
  if (!current) return HttpResponse.json({ code: "not_found" }, { status: 404 });
  const instance = { ...current, ...patch };
  instancesState = instancesState.map(row => (row.id === id ? instance : row));
  return HttpResponse.json({ instance });
}

/** `_dx.md`'s instance routes: plain `http` until they join the OpenAPI document. */
export const instanceHandlers: HttpHandler[] = [
  http.get("*/api/agent-instances", () =>
    HttpResponse.json({ instances: instancesState, next_cursor: "" })
  ),
  http.post("*/api/agent-instances", async ({ request }) => {
    const { agent, workspace } = (await request.json()) as { agent: string; workspace: string };
    const global = workspace === "@user";
    const instance = agentInstanceFixture({
      id: `ainst-${agent}-${workspace}`,
      agent_name: agent,
      label: agent,
      workspace_id: global ? "" : workspace,
      scope: workspace,
      main_session_id: `sess-main-${agent}`,
    });
    instancesState = [...instancesState, instance];
    return HttpResponse.json({ instance }, { status: 201 });
  }),
  http.post("*/api/agent-instances/:id/pause", ({ params }) =>
    respond(params.id, { state: "paused", pause_reason: "manual" })
  ),
  http.post("*/api/agent-instances/:id/resume", ({ params }) =>
    respond(params.id, { state: "active", pause_reason: "" })
  ),
  http.post("*/api/agent-instances/:id/ensure-main", ({ params }) =>
    respond(params.id, { main_session_id: `sess-main-${String(params.id)}` })
  ),
  http.delete("*/api/agent-instances/:id", ({ params }) => {
    instancesState = instancesState.filter(instance => instance.id !== params.id);
    return new HttpResponse(null, { status: 204 });
  }),
  http.get("*/api/agent-instances/:id/check-ins", () =>
    HttpResponse.json({ check_ins: checkInsState, next_cursor: "" })
  ),
  http.post("*/api/agents/draft-instructions", async ({ request }) => {
    const { goal } = (await request.json()) as { goal: string };
    return HttpResponse.json({
      instructions: `${goal}.\n\n- Say what changed and why it matters\n- Stay quiet when nothing changed`,
    });
  }),
];
