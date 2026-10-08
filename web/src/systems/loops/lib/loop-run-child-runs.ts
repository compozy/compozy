import type { LoopNodeWait, LoopRosterNode, LoopRunRecord, LoopStepProgress } from "../types";
import { isTerminalLoopStatus } from "./loop-formatters";
import { humanizeLoopNodeId } from "./loop-node-labels";
import { type LoopStateChip, loopParkReason, loopRosterStateChip } from "./loop-run-state-copy";
import { formatClockDuration, runElapsedSeconds } from "./loop-run-usage";

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
  /** The step as a reader names it: "hold for release". */
  name: string;
  chip: LoopStateChip;
  /** Other steps live at the same time, counted rather than listed. */
  alsoActive: number;
  /** " — paused · 1 more active": why it sits there, when it is parked. */
  detail: string;
  /**
   * How long the child has been on this step, from the step's own start. It
   * keeps counting while the step is parked: a child stuck for an hour must read
   * an hour, which the run clock cannot say once it freezes at the last progress.
   */
  onStepSeconds: number | null;
}

/** The served round when the briefing has answered; the roster's latest otherwise. */
function currentRound(progress: LoopStepProgress | null, nodes: readonly LoopRosterNode[]): number {
  if (progress && progress.round > 0) return progress.round;
  return nodes.reduce((latest, node) => Math.max(latest, node.generation), 0);
}

function secondsSince(iso: string | null | undefined, nowMs: number): number | null {
  if (!iso) return null;
  const started = Date.parse(iso);
  if (Number.isNaN(started)) return null;
  return Math.max(0, Math.round((nowMs - started) / 1000));
}

/** The durable wait cells a step parks in; a wait step has no roster start of its own. */
export type LoopChildRunWait = Pick<LoopNodeWait, "node_id" | "item_index" | "created_at">;

/**
 * When the step began: the roster's start, or — for a step parked in a durable
 * wait, which the roster does not time — when its wait cell was created.
 */
function stepStartedAt(node: LoopRosterNode, waits: readonly LoopChildRunWait[]): string | null {
  if (node.started_at) return node.started_at;
  const wait = waits.find(
    cell => cell.node_id === node.node_id && cell.item_index === node.item_index
  );
  return wait?.created_at ?? null;
}

export function childRunCurrentStep(
  status: string,
  progress: LoopStepProgress | null,
  nodes: readonly LoopRosterNode[],
  nowMs: number,
  waits: readonly LoopChildRunWait[] = []
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
  const alsoActive = live.size - 1;
  const parkReason = loopParkReason(lead.state);
  return {
    nodeId: lead.node_id,
    name: humanizeLoopNodeId(lead.node_id),
    chip: loopRosterStateChip(lead.state),
    alsoActive,
    detail: `${parkReason ? ` — ${parkReason}` : ""}${alsoActive > 0 ? ` · ${alsoActive} more active` : ""}`,
    onStepSeconds: secondsSince(stepStartedAt(lead, waits), nowMs),
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
  /** "1 of 4 steps · 6m 00s" — the count and the clock, as the row prints them. */
  metaLabel: string;
  /** "12m 03s on this step" while the child sits on a step; empty otherwise. */
  onStepLabel: string;
  /**
   * Loop runs the child itself started in its current round — the next level of
   * a nested loop, opened the same way one level down.
   */
  childRuns: LoopStepChildRun[];
}

/** The runs a child started in the round it is on, named by the step that started them. */
function grandchildRuns(round: number, nodes: readonly LoopRosterNode[]): LoopStepChildRun[] {
  const slots: LoopStepChildRunSlot[] = [];
  for (const node of nodes) {
    if (node.generation !== round) continue;
    slots.push({
      key: `${node.node_id}:${node.item_index}`,
      childRunId: node.child_loop_run_id ?? null,
      slotLabel: node.node_id,
      itemIndex: node.item_index,
    });
  }
  return stepChildRuns(slots);
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
  nowMs: number,
  waits: readonly LoopChildRunWait[] = []
): LoopChildRunSummary {
  const label = progressLabel(progress);
  const elapsedSeconds = runElapsedSeconds(run, nowMs);
  const clock = formatClockDuration(elapsedSeconds);
  const currentStep = childRunCurrentStep(run.status, progress, nodes, nowMs, waits);
  return {
    runId: run.id,
    loopName: run.loop_name,
    status: run.status,
    currentStep,
    progressLabel: label,
    elapsedSeconds,
    metaLabel: label ? `${label} · ${clock}` : clock,
    onStepLabel:
      currentStep?.onStepSeconds == null
        ? ""
        : `${formatClockDuration(currentStep.onStepSeconds)} on this step`,
    childRuns: grandchildRuns(currentRound(progress, nodes), nodes),
  };
}
