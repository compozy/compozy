import type { SubagentPayload } from "../adapters/subagent-api";
import type {
  SubagentOrigin,
  SubagentStatus,
  SubagentView,
  SubagentWorkState,
} from "../components/subagents/types";

const SUBAGENT_STATUSES: ReadonlySet<string> = new Set<SubagentStatus>([
  "queued",
  "running",
  "waiting",
  "completed",
  "failed",
  "canceled",
  "interrupted",
]);

const SUBAGENT_WORK_STATES: ReadonlySet<string> = new Set<SubagentWorkState>([
  "working",
  "waiting_for_children",
  "result_available",
]);

/**
 * An unknown status from a newer daemon reads as `interrupted`: settled and not
 * stoppable, so the UI never offers an action it cannot honour.
 */
function toSubagentStatus(status: string): SubagentStatus {
  return SUBAGENT_STATUSES.has(status) ? (status as SubagentStatus) : "interrupted";
}

function toSubagentOrigin(origin: string): SubagentOrigin {
  return origin === "provider_native" ? "provider_native" : "delegated";
}

/**
 * The wire → view mapping preserves the daemon creation timestamp for stable ordering.
 */
export function subagentViewFromPayload(payload: SubagentPayload): SubagentView {
  return {
    id: payload.subagent_id,
    parent_session_id: payload.parent_session_id,
    child_session_id: payload.child_session_id,
    origin: toSubagentOrigin(payload.origin),
    title: payload.title,
    role: payload.role,
    status: toSubagentStatus(payload.status),
    work_state: SUBAGENT_WORK_STATES.has(payload.work_state)
      ? (payload.work_state as SubagentWorkState)
      : null,
    progress: payload.progress,
    result_preview: payload.result_preview,
    error: payload.error,
    runtime: payload.runtime,
    depth: payload.depth,
    started_at: payload.started_at,
    settled_at: payload.settled_at,
    created_at: payload.created_at,
    updated_at: payload.updated_at,
    delivery: payload.delivery,
  };
}
