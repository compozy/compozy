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
> &
  Partial<Pick<SubagentPayload, "role" | "depth">> & {
    id: string;
    status: SubagentStatus;
    origin: SubagentOrigin;
    work_state?: SubagentWorkState | null;
    created_at: string;
  };

/** Where a subagent works, shown in the hover card only when it differs from the parent. */
export interface SubagentLocationView {
  workspace?: string | null;
  worktree?: string | null;
}
