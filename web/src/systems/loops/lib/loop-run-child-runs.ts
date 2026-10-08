import type { LoopRosterNode, LoopRunRecord, LoopStepProgress } from "../types";
import { isTerminalLoopStatus } from "./loop-formatters";
import { type LoopStateChip, loopRosterStateChip } from "./loop-run-state-copy";
import { runElapsedSeconds } from "./loop-run-usage";

/**
 * A child run, as its parent step sees it.
 *
 * A step that starts another loop records the child's run id on its roster row.
 * The parent never copies the child's progress into its own read: the child's
 * own detail and roster are the truth, read lazily when someone opens the step,
 * so a nested run stays one hop from the parent instead of one navigation away.
 */
export interface LoopStepChildRun {
  key: string;
  runId: string;
  /** The fan-out slot ("item 0") a branch child belongs to; null on a plain step. */
  slotLabel: string | null;
}

export interface LoopStepChildRunSlot {
  key: string;
  childRunId: string | null;
  slotLabel: string | null;
  itemIndex: number;
}

/**
 * The child-run references a step owns, in branch order.
 *
 * A branch that never ran has no child to point at, and a slot that recorded
 * none (it has not dispatched yet) is simply absent — never a placeholder row.
 * When a fan-out spreads one authored node over several items, every branch
 * carries the same name, so the item slot is added: three rows reading
 * "fix batch" would leave the reader guessing which child is which.
 */
export function stepChildRuns(rows: readonly LoopStepChildRunSlot[]): LoopStepChildRun[] {
  const labelCounts = new Map<string, number>();
  for (const row of rows) {
    if (row.slotLabel) labelCounts.set(row.slotLabel, (labelCounts.get(row.slotLabel) ?? 0) + 1);
  }
  const children: LoopStepChildRun[] = [];
  for (const row of rows) {
    const runId = row.childRunId?.trim();
    if (!runId) continue;
    const shared = row.slotLabel !== null && (labelCounts.get(row.slotLabel) ?? 0) > 1;
    children.push({
      key: row.key,
      runId,
      slotLabel: shared ? `${row.slotLabel} · item ${row.itemIndex}` : row.slotLabel,
    });
  }
  return children;
}

/** "1 child run" / "3 child runs" — the disclosure names what it holds. */
export function childRunsToggleLabel(count: number): string {
  return count === 1 ? "1 child run" : `${count} child runs`;
}

/**
 * Where a live child is, read from its own roster.
 *
 * Several nodes can be live at once, so one has to lead. Work holding for a
 * person outranks work parked on something else, which outranks work merely
 * moving: the row exists to surface the child that is stuck, not the one that
 * is fine.
 */
const CURRENT_STEP_PRECEDENCE = [
  "control_pending",
  "awaiting_goal",
  "awaiting_child",
  "waiting",
  "retrying",
  "paused",
  "running",
  "queued",
] as const;

const CURRENT_STEP_RANK = new Map<string, number>(
  CURRENT_STEP_PRECEDENCE.map((state, index) => [state, index])
);

export interface LoopChildRunCurrentStep {
  nodeId: string;
  chip: LoopStateChip;
  /** Other steps live at the same time, counted rather than listed. */
  alsoActive: number;
}

/** The served round when the briefing has answered; the roster's latest otherwise. */
function currentRound(progress: LoopStepProgress | null, nodes: readonly LoopRosterNode[]): number {
  if (progress && progress.round > 0) return progress.round;
  return nodes.reduce((latest, node) => Math.max(latest, node.generation), 0);
}

export function childRunCurrentStep(
  status: string,
  progress: LoopStepProgress | null,
  nodes: readonly LoopRosterNode[]
): LoopChildRunCurrentStep | null {
  if (isTerminalLoopStatus(status)) return null;
  const round = currentRound(progress, nodes);
  const live = new Map<string, LoopRosterNode>();
  for (const node of nodes) {
    if (node.generation !== round) continue;
    const rank = CURRENT_STEP_RANK.get(node.state);
    if (rank === undefined) continue;
    const held = live.get(node.node_id);
    if (!held || rank < (CURRENT_STEP_RANK.get(held.state) ?? rank)) live.set(node.node_id, node);
  }
  let lead: LoopRosterNode | null = null;
  for (const node of live.values()) {
    if (
      !lead ||
      (CURRENT_STEP_RANK.get(node.state) ?? 0) < (CURRENT_STEP_RANK.get(lead.state) ?? 0)
    ) {
      lead = node;
    }
  }
  if (!lead) return null;
  return {
    nodeId: lead.node_id,
    chip: loopRosterStateChip(lead.state),
    alsoActive: live.size - 1,
  };
}

export interface LoopChildRunSummary {
  runId: string;
  loopName: string;
  /** The raw run status; `LoopStatusMark` owns its glyph and word. */
  status: string;
  currentStep: LoopChildRunCurrentStep | null;
  /** "1 of 4 steps" from the child's briefing; empty before it has any. */
  progressLabel: string;
  elapsedSeconds: number;
}

/**
 * The step counts come from the child's briefing, the read that serves them:
 * the run record's own `progress` is not populated on the detail route, and a
 * label built from it would read "0 of 0" for a child that is halfway through.
 */
function progressLabel(progress: LoopStepProgress | null): string {
  if (!progress || progress.steps_total <= 0) return "";
  return `${progress.steps_done} of ${progress.steps_total} steps`;
}

export function buildChildRunSummary(
  run: LoopRunRecord,
  progress: LoopStepProgress | null,
  nodes: readonly LoopRosterNode[],
  nowMs: number
): LoopChildRunSummary {
  return {
    runId: run.id,
    loopName: run.loop_name,
    status: run.status,
    currentStep: childRunCurrentStep(run.status, progress, nodes),
    progressLabel: progressLabel(progress),
    elapsedSeconds: runElapsedSeconds(run, nowMs),
  };
}
