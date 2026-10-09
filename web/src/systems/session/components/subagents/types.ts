/**
 * Props-side view of one subagent row, shaped after `_dx.md` `SubagentPayload`
 * (snake_case, as the wire carries it). The roster owner aliases this to the
 * generated contract type; these components never fetch or stream.
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

export interface SubagentRuntimeView {
  agent?: string | null;
  provider?: string | null;
  model?: string | null;
  reasoning_effort?: string | null;
  speed?: string | null;
}

export interface SubagentView {
  id: string;
  parent_session_id: string;
  /** `null` for provider-native rows: there is no Compozy session to open. */
  child_session_id: string | null;
  origin: SubagentOrigin;
  title: string;
  role?: string | null;
  status: SubagentStatus;
  work_state?: SubagentWorkState | null;
  /** Latest child step, coalesced to 1/s by the daemon; `""` until it reports. */
  progress?: string | null;
  result_preview?: string | null;
  error?: string | null;
  runtime: SubagentRuntimeView;
  depth?: number;
  started_at: string | null;
  settled_at: string | null;
  created_at: string;
  updated_at: string;
}

/** Where a subagent works, shown in the hover card only when it differs from the parent. */
export interface SubagentLocationView {
  workspace?: string | null;
  worktree?: string | null;
}
