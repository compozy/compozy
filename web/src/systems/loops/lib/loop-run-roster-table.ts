import type { LoopFanoutRollup, LoopRosterNode } from "../types";
import type { LoopGraph } from "./loop-graph";
import { loopDagKindLabel } from "./loop-node-labels";
import { loopFanOutContainerState, resolveFanOutBranches } from "./loop-run-fanout-band";
import {
  type LoopStateChip,
  isUnsettledRosterState,
  loopRosterStateChip,
} from "./loop-run-state-copy";
import { deriveCostEstimate } from "./loop-run-usage";

/**
 * Every node of every round — healthy ones included.
 *
 * The run page shows exceptions; this shows the whole run. That difference is
 * the entire point: an operator asking "what is my run doing right now" was
 * previously answered only when something had gone wrong, which is why a healthy
 * run looked identical to a stalled one.
 *
 * Fan-out workers group under the node that spread them rather than flooding the
 * table as siblings, and attempts stay metadata on the row they belong to.
 */

/**
 * What the row's timing actually means.
 *
 * A node that started and has not ended is *running*, and printing "not started"
 * against it — the single most common state on a live run — inverts the one fact
 * the reader came for. Only a node with no start at all has not started.
 */
export type LoopRosterProgressState = "settled" | "in-progress" | "not-started";

export interface LoopRosterRow {
  key: string;
  nodeId: string;
  /** True for a fan-out worker; the table indents it under its parent. */
  isBranch: boolean;
  kindLabel: string | null;
  /** "3 workers" on a fan-out container, null elsewhere. */
  fanOutLabel: string | null;
  chip: LoopStateChip;
  generation: number;
  itemIndex: number;
  /** "2 of 2", or "10 of 12 · next 18:47" while a retry is scheduled. */
  attemptLabel: string;
  nextRetryAt: string | null;
  startedAt: string | null;
  endedAt: string | null;
  progressState: LoopRosterProgressState;
  /** 0..1 of the run's longest node, for the row's duration micro-bar. */
  durationRatio: number | null;
  /** Settled duration, or elapsed so far while the step is still running. */
  durationMs: number | null;
  sessionId: string | null;
  usageTokens: number | null;
  /** `~$0.05` — an estimate, and the leading `~` says so wherever it lands. */
  usageCostLabel: string | null;
}

export interface LoopRosterTableModel {
  /** Built once and never mutated after: the table reads, it does not reorder. */
  rows: readonly LoopRosterRow[];
  rounds: number[];
  /** True when the run reached no action node at all — stated, not blank. */
  reachedNothing: boolean;
}

interface RosterTiming {
  progressState: LoopRosterProgressState;
  durationMs: number | null;
}

function parseTime(value: string | null | undefined): number | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : parsed;
}

function timingOf(
  startedAt: string | null | undefined,
  endedAt: string | null | undefined,
  nowMs: number
): RosterTiming {
  const start = parseTime(startedAt);
  if (start === null) return { progressState: "not-started", durationMs: null };
  const end = parseTime(endedAt);
  if (end !== null) return { progressState: "settled", durationMs: Math.max(end - start, 0) };
  // Elapsed, not a frozen zero: a step that has been running eleven minutes and a
  // step that started a second ago are the two answers people are looking for.
  return { progressState: "in-progress", durationMs: Math.max(nowMs - start, 0) };
}

function attemptLabel(node: LoopRosterNode): string {
  if (node.attempt <= 0) return "—";
  const attempts = node.attempts.length;
  const total = Math.max(attempts, node.attempt);
  return `${node.attempt} of ${total}`;
}

function costLabel(tokens: number | null): string | null {
  return tokens === null || tokens <= 0 ? null : deriveCostEstimate(tokens);
}

/**
 * A roster row's identity: round, step and item together.
 *
 * The same `node_id` exists once per round and once per fan-out worker, so any
 * two of the three are not enough to tell two rows apart — in the model or in
 * the DOM.
 */
export function rosterRowKey(node: Pick<LoopRosterNode, "generation" | "node_id" | "item_index">) {
  return `${node.generation}:${node.node_id}:${node.item_index}`;
}

/**
 * Earliest start and latest end across a fan-out's workers.
 *
 * Settlement is a question about state, not about timestamps. A branch the run
 * declined at a route carries no clock at all, and a branch canceled before it
 * started carries none either — both are finished, and treating a missing
 * `ended_at` as unfinished kept completed fan-outs reading as still running.
 */
function branchSpan(branches: readonly LoopRosterNode[]): {
  startedAt: string | null;
  endedAt: string | null;
} {
  let startedAt: string | null = null;
  let earliest = Number.POSITIVE_INFINITY;
  let endedAt: string | null = null;
  let latest = Number.NEGATIVE_INFINITY;
  let allSettled = true;
  for (const branch of branches) {
    const start = parseTime(branch.started_at);
    if (start !== null && start < earliest) {
      earliest = start;
      startedAt = branch.started_at ?? null;
    }
    if (isUnsettledRosterState(branch.state)) allSettled = false;
    const end = parseTime(branch.ended_at);
    if (end === null) continue;
    if (end > latest) {
      latest = end;
      endedAt = branch.ended_at ?? null;
    }
  }
  // One worker still owed work means the fan-out is still running, whatever its
  // finished siblings say.
  return { startedAt, endedAt: allSettled ? endedAt : null };
}

export interface BuildRosterTableInput {
  nodes: readonly LoopRosterNode[];
  rollups: readonly LoopFanoutRollup[];
  graph: LoopGraph | null;
  /** `null` shows every round; a number filters to one. */
  round: number | null;
  /** The clock in-progress rows measure against; stories pin it for capture. */
  nowMs: number;
  /**
   * Whether the roster read has finished. Required, because "this run reached no
   * step" is a claim about the run, and a table built from a partial or failed
   * read has no standing to make it.
   */
  isComplete: boolean;
}

export function buildRosterTable({
  nodes,
  rollups,
  graph,
  round,
  nowMs,
  isComplete,
}: BuildRosterTableInput): LoopRosterTableModel {
  const scoped = round === null ? nodes : nodes.filter(node => node.generation === round);
  const rounds = [...new Set(nodes.map(node => node.generation))].sort((a, b) => b - a);

  // Each container keeps the rollup it was resolved from. The association is
  // known at this point, and reconstructing it later from a composite string key
  // both rescanned the rollups and broke on any node id containing a colon.
  const containers: { rollup: LoopFanoutRollup; branches: LoopRosterNode[] }[] = [];
  const claimed = new Set<string>();
  for (const rollup of rollups) {
    if (round !== null && rollup.generation !== round) continue;
    const branches = resolveFanOutBranches(rollup, scoped, graph);
    containers.push({ rollup, branches });
    // Claim identity carries the round. Without it, round 2's fan-out claims the
    // identically-named worker in round 1, and that row vanishes from "All
    // rounds" — grouped under a container that never spread it.
    for (const branch of branches) claimed.add(rosterRowKey(branch));
  }

  const rowFor = (node: LoopRosterNode, isBranch: boolean): LoopRosterRow => {
    const timing = timingOf(node.started_at, node.ended_at, nowMs);
    const authored = graph?.nodes.find(entry => entry.id === node.node_id);
    const tokens = node.usage?.tokens ?? null;
    return {
      key: rosterRowKey(node),
      nodeId: node.node_id,
      isBranch,
      kindLabel: isBranch ? null : loopDagKindLabel(authored),
      fanOutLabel: null,
      chip: loopRosterStateChip(node.state),
      generation: node.generation,
      itemIndex: node.item_index,
      attemptLabel: attemptLabel(node),
      nextRetryAt: node.next_retry_at ?? null,
      startedAt: node.started_at ?? null,
      endedAt: node.ended_at ?? null,
      progressState: timing.progressState,
      durationRatio: null,
      durationMs: timing.durationMs,
      sessionId: node.session_id?.trim() || null,
      usageTokens: tokens,
      usageCostLabel: costLabel(tokens),
    };
  };

  const rows: LoopRosterRow[] = [];
  for (const node of scoped) {
    if (claimed.has(rosterRowKey(node))) continue;
    rows.push(rowFor(node, false));
  }
  for (const { rollup, branches } of containers) {
    const { generation, node_id: nodeId } = rollup;
    // The container has no roster row of its own — only a rollup — so its state
    // and its span are the ones its workers put it in.
    const span = branchSpan(branches);
    const timing = timingOf(span.startedAt, span.endedAt, nowMs);
    const tokens = branches.reduce((total, branch) => total + (branch.usage?.tokens ?? 0), 0);
    const containerRow: LoopRosterRow = {
      key: `${generation}:${nodeId}:container`,
      nodeId,
      isBranch: false,
      kindLabel: loopDagKindLabel(graph?.nodes.find(entry => entry.id === nodeId)),
      fanOutLabel: `${rollup.total} ${rollup.total === 1 ? "worker" : "workers"}`,
      chip: loopRosterStateChip(
        loopFanOutContainerState(
          branches.map(branch => branch.state),
          rollup
        )
      ),
      generation,
      itemIndex: 0,
      attemptLabel: "—",
      nextRetryAt: null,
      startedAt: span.startedAt,
      endedAt: span.endedAt,
      progressState: timing.progressState,
      durationRatio: null,
      durationMs: timing.durationMs,
      sessionId: null,
      usageTokens: tokens,
      usageCostLabel: costLabel(tokens),
    };
    rows.push(containerRow, ...branches.map(branch => rowFor(branch, true)));
  }

  // Relative to the longest row in view, so the bar compares like with like —
  // and computed after the fan-out containers exist, since a container's span
  // covers its workers and is usually the longest thing on the table.
  const longest = rows.reduce((max, row) => Math.max(max, row.durationMs ?? 0), 0);
  const measured = rows.map(row => ({
    ...row,
    durationRatio: row.durationMs !== null && longest > 0 ? row.durationMs / longest : null,
  }));

  return {
    rows: measured,
    rounds,
    // A run that ended before round 1 has no action nodes at all. Saying so
    // beats an empty table, which reads as a loading failure — but only once the
    // read is complete. An in-flight or failed roster is also rowless, and it
    // means "we cannot say yet", which is the opposite sentence.
    reachedNothing: measured.length === 0 && isComplete,
  };
}
