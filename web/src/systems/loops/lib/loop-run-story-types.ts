import type { PillTone } from "@compozy/ui";

import type { LoopRunGeneration } from "../types";
import type { LoopGraph } from "./loop-graph";

export type LoopStoryIcon =
  | "round"
  | "check-pass"
  | "check-warn"
  | "node-done"
  | "node-failed"
  | "approval"
  | "watching"
  | "paused"
  | "resumed"
  | "started"
  | "done"
  | "circle-slash"
  // Node lifecycle beats (Spec 1). One fixed lucide icon per concept across the
  // domain, per DESIGN-LESSONS L7.
  | "retry"
  | "canceled"
  | "killed"
  | "quarantined"
  | "requeued"
  | "waiting"
  | "attention"
  | "attention-silence"
  | "attention-cleared"
  | "effect"
  | "suppressed"
  | "breaker-open"
  | "breaker-closed"
  | "request-opened"
  | "request-answered"
  | "request-expired"
  | "route-taken"
  | "route-default"
  | "pruned"
  | "amended"
  | "forked";

export interface LoopStoryIssue {
  id: string;
  note?: string;
}

export interface LoopStoryTaskLink {
  taskId: string;
  taskRunId: string;
}

export interface LoopStoryRow {
  key: string;
  kind: string;
  seq: number;
  at: string;
  tone: PillTone;
  icon: LoopStoryIcon;
  title: string;
  sub?: string;
  /** Blocking issues rendered after `sub` (mono `id`, optional note). */
  issues?: LoopStoryIssue[];
  /** Verbatim runtime identifier trail (`gate_verdict · revise`). */
  micro: string;
  /** Source node id when the row belongs to one node (Turns disclosure anchor). */
  nodeId?: string;
  generation?: number;
  /** Persisted metric score associated with this generation, when configured. */
  score?: number;
  /** True only when the daemon projects this row's generation as the best. */
  isBest?: boolean;
  /** Persisted generation origin rendered as an informational chip. */
  originLabel?: string;
  originTone?: PillTone;
}

export interface LoopStoryNow {
  nodeId: string;
  /** Humanized node label (`fix_batches` → "fix batches"). */
  label: string;
  itemIndex?: number;
  generation: number;
  startedAt: string;
  taskLink?: LoopStoryTaskLink;
  /** Set while a `run-loop` node awaits its child run (links `/loop-runs/$runId`). */
  childRunId?: string;
  isGoalNode: boolean;
}

export interface LoopRunStory {
  /** Newest-first story rows. */
  rows: LoopStoryRow[];
  /** The newest running node without a terminal event; null when nothing runs. */
  now: LoopStoryNow | null;
}

export interface LoopRunStoryContext {
  status?: string | null;
  reattemptStrategy?: string;
  graph: LoopGraph | null;
  generations?: readonly LoopRunGeneration[];
  bestGeneration?: number | null;
}
