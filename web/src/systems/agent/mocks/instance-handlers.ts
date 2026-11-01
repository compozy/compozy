import { http, HttpResponse, type HttpHandler } from "msw";

import type {
  AgentActivityPayload,
  AgentInstanceCheckDigestItem,
  AgentInstanceCheckIn,
  AgentInstancePayload,
  AgentInstanceAway,
  AgentInstanceReport,
} from "../types";
import { agentInstanceHoldsSlot } from "../lib/agent-instance";
import { agentInstanceFixture } from "./instance-fixtures";

let instancesState: AgentInstancePayload[] = [];
let checkInsState: AgentInstanceCheckIn[] = [];
let digestState: AgentInstanceCheckDigestItem[] = [];
let activityState: AgentActivityPayload | null = null;
let reportsState: AgentInstanceReport[] = [];
let awayState: AgentInstanceAway | null = null;
let capacityLimit = 10;

export function resetAgentInstanceMock(): void {
  instancesState = [];
  checkInsState = [];
  digestState = [];
  activityState = null;
  capacityLimit = 10;
}

/** `agents.stays_on.max_active_instances`, as the list envelope reports it. */
export function seedAgentInstanceLimit(limit: number): void {
  capacityLimit = limit;
}

/** Without a seed the activity route answers 404, the way a daemon without it does. */
export function seedAgentActivityMock(activity: AgentActivityPayload | null): void {
  activityState = activity;
  reportsState = [];
  awayState = null;
}

export function seedAgentInstanceMock(
  instances: readonly AgentInstancePayload[],
  checkIns: readonly AgentInstanceCheckIn[] = [],
  digest: readonly AgentInstanceCheckDigestItem[] = []
): void {
  instancesState = structuredClone([...instances]);
  checkInsState = structuredClone([...checkIns]);
  digestState = structuredClone([...digest]);
}

/** What the reports route and the away read return; no away seeded answers 404, as today. */
export function seedAgentInstanceReports(
  reports: readonly AgentInstanceReport[],
  away: AgentInstanceAway | null = null
): void {
  reportsState = structuredClone([...reports]);
  awayState = structuredClone(away);
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
  http.get("*/api/agents/:name/activity", () =>
    activityState
      ? HttpResponse.json(activityState)
      : HttpResponse.json({ code: "not_found" }, { status: 404 })
  ),
  http.get("*/api/agent-instances/:id/reports", () =>
    HttpResponse.json({ reports: reportsState, next_cursor: "" })
  ),
  http.get("*/api/agent-instances/:id/away", () =>
    awayState
      ? HttpResponse.json(awayState)
      : HttpResponse.json({ code: "not_found" }, { status: 404 })
  ),
  http.post("*/api/agent-instances/:id/away", () => new HttpResponse(null, { status: 204 })),
  http.get("*/api/agent-instances", () =>
    HttpResponse.json({
      instances: instancesState,
      next_cursor: "",
      max_active_instances: capacityLimit,
      active_count: instancesState.filter(agentInstanceHoldsSlot).length,
    })
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
  http.patch("*/api/agent-instances/:id", async ({ params, request }) => {
    const { checks_enabled } = (await request.json()) as { checks_enabled: boolean };
    return respond(params.id, { checks_enabled });
  }),
  http.post("*/api/agent-instances/:id/check", async ({ request }) => {
    const { dry_run } = (await request.json()) as { dry_run: boolean };
    return dry_run
      ? HttpResponse.json({ gate: "would_run", digest: digestState })
      : HttpResponse.json({ occurrence_id: "occ-3a91" }, { status: 202 });
  }),
  http.get("*/api/agent-instances/:id/occurrences/:occurrence", () =>
    HttpResponse.json({
      state: "finished",
      result: "reported",
      reason: "reported",
      report_id: "arpt-7c1e",
    })
  ),
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
