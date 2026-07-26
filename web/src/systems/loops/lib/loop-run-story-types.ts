import type { PillTone } from "@agh/ui";

import type { LoopRunGeneration } from "../types";
import type { LoopGraph } from "./loop-graph";

/**
 * The story data shapes (redesign spec §5.3): the plain-language story rows, the
 * "Happening now" projection, and the story context the frame fold reads. Every
 * string is a template over real payload fields — the runtime identifiers survive
 * verbatim in the mono `micro` labels (§5.1).
 */

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
  | "stopped"
  | "done";

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
}
