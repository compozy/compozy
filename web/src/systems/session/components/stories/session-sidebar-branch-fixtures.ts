import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type { SessionPayload } from "../../types";
import { minutesAgo, sidebarSession, withSubagents } from "./session-sidebar-fixtures";

/** Issue #919: one parent per branch shape — children only, subagents only, both, many. */
export const BRANCH_SESSIONS: SessionPayload[] = [
  minutesAgo(sidebarSession("sess-docs", "Rewrite onboarding docs", "writer", "done"), 2),
  minutesAgo(
    sidebarSession("sess-docs-api", "Draft API reference", "writer", "running", "sess-docs"),
    3
  ),
  minutesAgo(
    sidebarSession("sess-docs-faq", "Collect FAQ answers", "research", "failed", "sess-docs"),
    5
  ),
  minutesAgo(
    withSubagents(
      sidebarSession("sess-audit", "Audit payment webhooks", "claude", "running"),
      2,
      5
    ),
    1
  ),
  minutesAgo(
    withSubagents(sidebarSession("sess-ship", "Ship checkout v2", "claude", "running"), 3, 8),
    0
  ),
  minutesAgo(
    sidebarSession("sess-ship-qa", "QA the checkout flow", "qa", "running", "sess-ship"),
    1
  ),
  minutesAgo(
    sidebarSession("sess-ship-copy", "Polish checkout copy", "writer", "done", "sess-ship"),
    6
  ),
  minutesAgo(
    withSubagents(
      sidebarSession("sess-migrate", "Migrate the session store", "codex", "done"),
      6,
      24,
      1
    ),
    9
  ),
];

const TITLES = [
  "Map webhook retry paths",
  "Check idempotency keys",
  "Review refund edge cases",
  "Profile the ledger writes",
  "Trace failed captures",
  "Verify currency rounding",
  "Draft the migration plan",
  "Backfill audit rows",
];
// A mixed set: working, waiting, done, failed, idle (delivered, child quiet), queued.
const STATES = [
  { status: "running" },
  { status: "waiting" },
  { status: "running" },
  { status: "completed" },
  { status: "failed" },
  { status: "completed", child_activity: "idle", delivery: "delivered" },
  { status: "queued" },
] as const;

const LOADED_AT = Date.now();

function subagent(parentId: string, index: number) {
  const state = STATES[index % STATES.length]!;
  const { status } = state;
  // Relative to story load, so Working ticks from minutes ago and Done freezes at a real span.
  const startMs = LOADED_AT - (index + 2) * 90_000;
  const at = new Date(startMs).toISOString();
  const settledAt = new Date(startMs + 45_000 + index * 7_000).toISOString();
  return {
    subagent_id: `${parentId}-sub-${index}`,
    workspace_id: "ws-product",
    parent_session_id: parentId,
    parent_turn_id: "turn-1",
    child_session_id: `${parentId}-child-${index}`,
    origin: "delegated",
    provider_tool_call_id: null,
    title: `${TITLES[index % TITLES.length]} ${index + 1}`,
    role: "",
    status,
    work_state: status === "completed" ? "result_available" : "working",
    progress: "",
    result: null,
    result_preview: "",
    result_truncated: false,
    error: null,
    delivery: "delivery" in state ? state.delivery : "none",
    ...("child_activity" in state ? { child_activity: state.child_activity } : {}),
    depth: 1,
    wait_timed_out: false,
    runtime: { agent: "coder", provider: "claude", model: "", reasoning_effort: "", speed: "" },
    started_at: status === "queued" ? null : at,
    settled_at: status === "completed" || status === "failed" ? settledAt : null,
    created_at: at,
    updated_at: at,
  };
}

/** The list route, paged by `limit` and an offset cursor, sized by each parent's summary. */
export const SUBAGENT_BRANCH_HANDLERS = [
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/subagents",
    ({ params, request }) => {
      const parent = BRANCH_SESSIONS.find(session => session.id === params.session_id);
      const total = parent?.subagent_summary?.total ?? 0;
      const search = new URL(request.url).searchParams;
      const limit = Number(search.get("limit") ?? 200);
      const from = Number(search.get("cursor") || 0);
      const to = Math.min(total, from + limit);
      return HttpResponse.json({
        subagents: Array.from({ length: to - from }, (_, offset) =>
          subagent(String(params.session_id), from + offset)
        ),
        ...(to < total ? { next_cursor: String(to) } : {}),
      } as never);
    }
  ),
];
