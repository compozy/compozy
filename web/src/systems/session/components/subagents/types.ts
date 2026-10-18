import type { SubagentPayload } from "../../adapters/subagent-api";

/**
 * Props-side view of one subagent row. Field types come from the generated
 * `SubagentPayload`; the view narrows the wire's free-string enums, names the
 * row `id`, and adds `created_at` as the roster's ordering key (the wire has
 * none). `subagentViewFromPayload` is the one mapping. These components never
 * fetch or stream.
 */
export type SubagentStatus =
  | "queued"
  | "running"
  | "waiting"
  | "completed"
  | "failed"
  | "canceled"
  | "interrupted";

export type SubagentOrigin = "delegated" | "provider_native";

export type SubagentWorkState = "working" | "waiting_for_children" | "result_available";

export type SubagentRuntimeView = SubagentPayload["runtime"];

/** `worktree`: the subagent runs on its own branch (S5); `shared`: in the parent's checkout. */
export type SubagentIsolation = "shared" | "worktree";

export type SubagentPullRequestState = "open" | "draft" | "merged" | "closed";

/**
 * `unknown`: CompozyOS could not check (no forge provider, lookup failed);
 * `none`: the forge answered with no PR. Never conflate the two (US-018).
 */
export type SubagentPullRequestStatus = "unknown" | "none" | SubagentPullRequestState;

export interface SubagentPullRequestView {
  url: string;
  number: number;
  state: SubagentPullRequestState;
}

/**
 * Daemon-observed facts of an isolated subagent's worktree. Static facts exist
 * from creation; git and PR facts are snapshotted once at settle and stay
 * `null` (absent, never zero) until then or when the read failed.
 */
export interface SubagentWorktreeView {
  id: string;
  name: string;
  branch: string;
  /** Requested ref, or the parent's HEAD sha when none was requested. */
  base_ref: string;
  base_sha: string | null;
  path: string;
  head_sha: string | null;
  commits_ahead: number | null;
  dirty_files: number | null;
  observed_at: string | null;
  /** `null` until the subagent settles. */
  pull_request_status: SubagentPullRequestStatus | null;
  /** Present only when `pull_request_status` is a PR state. */
  pull_request: SubagentPullRequestView | null;
}

export type SubagentView = Pick<
  SubagentPayload,
  | "parent_session_id"
  | "child_session_id"
  | "title"
  | "progress"
  | "result_preview"
  | "error"
  | "runtime"
  | "started_at"
  | "settled_at"
  | "updated_at"
  | "delivery"
> &
  Partial<Pick<SubagentPayload, "role" | "depth">> & {
    id: string;
    status: SubagentStatus;
    origin: SubagentOrigin;
    work_state?: SubagentWorkState | null;
    created_at: string;
    /** Absent reads as `shared` (older daemons, fixtures). */
    isolation?: SubagentIsolation;
    /** Only for `isolation: "worktree"`; shared subagents have no facts. */
    worktree?: SubagentWorktreeView | null;
  };

/** Where a subagent works, shown in the hover card only when it differs from the parent. */
export interface SubagentLocationView {
  workspace?: string | null;
  worktree?: string | null;
}
