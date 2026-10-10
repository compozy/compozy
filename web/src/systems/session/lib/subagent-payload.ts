import type { SubagentWirePayload, SubagentWorktreePayload } from "../adapters/subagent-api";
import type {
  SubagentOrigin,
  SubagentPullRequestState,
  SubagentStatus,
  SubagentView,
  SubagentWorkState,
  SubagentWorktreeView,
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

const PULL_REQUEST_STATES: ReadonlySet<string> = new Set<SubagentPullRequestState>([
  "open",
  "draft",
  "merged",
  "closed",
]);

const SAFE_PR_URL = /^https?:\/\//i;

const text = (value: string | undefined): string | null => value?.trim() || null;
const count = (value: number | undefined): number | null =>
  typeof value === "number" && Number.isFinite(value) ? value : null;

/**
 * PR status is the authority. A status that names a PR state but arrives
 * without a readable PR (no number, non-http URL), or a status this client
 * does not know, reads as `unknown`: the UI can not vouch for it, and it must
 * never claim `none`.
 */
function pullRequestFacts(
  worktree: SubagentWorktreePayload
): Pick<SubagentWorktreeView, "pull_request_status" | "pull_request"> {
  const status = text(worktree.pull_request_status);
  if (status === null) return { pull_request_status: null, pull_request: null };
  if (status === "unknown" || status === "none") {
    return { pull_request_status: status, pull_request: null };
  }
  const pr = worktree.pull_request;
  if (!PULL_REQUEST_STATES.has(status) || !pr || !SAFE_PR_URL.test(pr.url) || !(pr.number > 0)) {
    return { pull_request_status: "unknown", pull_request: null };
  }
  const state = status as SubagentPullRequestState;
  return { pull_request_status: state, pull_request: { url: pr.url, number: pr.number, state } };
}

function worktreeView(worktree: SubagentWorktreePayload): SubagentWorktreeView {
  return {
    id: worktree.id,
    name: worktree.name,
    branch: worktree.branch,
    base_ref: worktree.base_ref,
    base_sha: text(worktree.base_sha),
    path: worktree.path,
    head_sha: text(worktree.head_sha),
    commits_ahead: count(worktree.commits_ahead),
    dirty_files: count(worktree.dirty_files),
    observed_at: text(worktree.observed_at),
    ...pullRequestFacts(worktree),
  };
}

/** Shared subagents carry no facts, even if a payload sends some. */
function isolationFields(
  payload: SubagentWirePayload
): Pick<SubagentView, "isolation" | "worktree"> {
  if (payload.isolation !== "worktree") return { isolation: "shared", worktree: null };
  return {
    isolation: "worktree",
    worktree: payload.worktree ? worktreeView(payload.worktree) : null,
  };
}

/**
 * The wire → view mapping preserves the daemon creation timestamp for stable ordering.
 */
export function subagentViewFromPayload(payload: SubagentWirePayload): SubagentView {
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
    ...isolationFields(payload),
  };
}
