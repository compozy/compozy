import { parseBranchPruned } from "./loop-graph-events";
import { strategySummary, type LoopGraph, type LoopGraphNode } from "./loop-graph";
import { isLoopCompletionState, type LoopCompletionState } from "./loop-request-vocabulary";
import type { LoopRunEventFrame, LoopRunGeneration, LoopRunRecord } from "../types";

export interface LoopStrategyCounts {
  settled: number;
  succeeded: number;
  failed: number;

  canceledByStrategy: number;

  active: number;

  pending: number;

  neverMaterialized: number;

  opened: number;
}

export interface LoopStrategyProgressModel {
  nodeId: string;

  joinNodeId: string;

  strategyLabel: string | null;
  strategyKind: string;
  threshold: string | null;

  missingAcceptable: boolean;
  counts: LoopStrategyCounts;

  coverageLabel: string;
  coverageRate: number;

  completionState: LoopCompletionState;
  isPartial: boolean;

  triggerLane: number | null;

  winningLane: number | null;

  isWide: boolean;
}

export const LOOP_STRATEGY_WIDE_THRESHOLD = 24;

const TERMINAL_LANE_STATUSES = new Set(["succeeded", "failed", "canceled", "partial", "skipped"]);
const ACTIVE_LANE_STATUSES = new Set(["running", "retrying"]);

function fanOutNodes(graph: LoopGraph | null): LoopGraphNode[] {
  const nodes: LoopGraphNode[] = [];
  for (const node of graph?.nodes ?? []) {
    if (node.kind === "fan-out") nodes.push(node);
  }
  return nodes;
}

function joinNodeFor(graph: LoopGraph | null, fanOutId: string): string {
  if (!graph) return "";
  const downstream = new Set<string>();
  for (const edge of graph.edges) {
    if (edge.from === fanOutId) downstream.add(edge.to);
  }
  const reachable = new Set<string>();
  for (const edge of graph.edges) {
    if (downstream.has(edge.from)) reachable.add(edge.to);
  }
  const collect = graph.nodes.find(
    node => node.kind === "collect" && (downstream.has(node.id) || reachable.has(node.id))
  );
  return collect?.id ?? "";
}

function laneNodeFor(graph: LoopGraph | null, fanOutId: string): string {
  if (!graph) return "";
  const target = graph.edges.find(edge => edge.from === fanOutId)?.to;
  return target ?? "";
}

function prunedLanes(frames: readonly LoopRunEventFrame[], nodeId: string): Set<number> {
  const lanes = new Set<number>();
  for (const frame of frames) {
    if (frame.kind !== "branch_pruned") continue;
    const payload = parseBranchPruned(frame);
    if (!payload || payload.nodeId !== nodeId) continue;
    for (const index of payload.itemIndexes) lanes.add(index);
  }
  return lanes;
}

function countLanes(
  generation: LoopRunGeneration | undefined,
  laneNodeId: string,
  declaredWidth: number | undefined
): LoopStrategyCounts {
  const byLane = new Map<number, string>();
  for (const row of generation?.outputs ?? []) {
    if (row.node_id === laneNodeId) byLane.set(row.item_index ?? 0, row.status);
  }
  let succeeded = 0;
  let failed = 0;
  let canceledByStrategy = 0;
  let active = 0;
  let pending = 0;
  for (const status of byLane.values()) {
    if (status === "succeeded" || status === "partial") succeeded += 1;
    else if (status === "failed") failed += 1;
    else if (status === "canceled" || status === "skipped") canceledByStrategy += 1;
    else if (ACTIVE_LANE_STATUSES.has(status)) active += 1;
    else pending += 1;
  }
  let settled = 0;
  for (const status of byLane.values()) {
    if (TERMINAL_LANE_STATUSES.has(status)) settled += 1;
  }
  const opened = byLane.size;
  const neverMaterialized = declaredWidth === undefined ? 0 : Math.max(0, declaredWidth - opened);
  return {
    settled,
    succeeded,
    failed,
    canceledByStrategy,
    active,
    pending,
    neverMaterialized,
    opened,
  };
}

function coverage(counts: LoopStrategyCounts): { rate: number; label: string } {
  const denominator = counts.succeeded + counts.failed + counts.active + counts.pending;
  if (denominator === 0) return { rate: 0, label: "no lanes" };
  const rate = counts.succeeded / denominator;
  return {
    rate,
    label: `${counts.succeeded} of ${denominator} ${denominator === 1 ? "lane" : "lanes"}`,
  };
}

export interface LoopStrategyProgressInput {
  run: Pick<LoopRunRecord, "completion_state" | "generation">;
  graph: LoopGraph | null;
  generations: readonly LoopRunGeneration[];
  frames: readonly LoopRunEventFrame[];
}

export function buildStrategyProgress({
  run,
  graph,
  generations,
  frames,
}: LoopStrategyProgressInput): LoopStrategyProgressModel[] {
  const nodes = fanOutNodes(graph);
  if (nodes.length === 0) return [];
  const newest = [...generations].sort((a, b) => b.generation - a.generation).at(0);
  const completionState = isLoopCompletionState(run.completion_state)
    ? run.completion_state
    : "complete";
  return nodes.map(node => {
    const laneNodeId = laneNodeFor(graph, node.id);
    const pruned = prunedLanes(frames, laneNodeId);
    const counts = countLanes(newest, laneNodeId, node.maxFanOut);
    const { rate, label } = coverage(counts);
    const partialJoin = (newest?.outputs ?? []).some(
      output => output.node_id === joinNodeFor(graph, node.id) && output.status === "partial"
    );
    return {
      nodeId: node.id,
      joinNodeId: joinNodeFor(graph, node.id),
      strategyLabel: strategySummary(node),
      strategyKind: node.strategy?.kind ?? "wait_all",
      threshold: node.strategy?.threshold ?? null,
      missingAcceptable: node.strategy?.missing === "acceptable",
      counts,
      coverageLabel: label,
      coverageRate: rate,
      completionState,

      isPartial: completionState === "partial" || partialJoin,
      triggerLane: node.strategy?.kind === "fail_fast" ? firstLane(pruned) : null,
      winningLane: node.strategy?.kind === "race" ? winningLane(newest, laneNodeId, pruned) : null,
      isWide: counts.opened + counts.neverMaterialized > LOOP_STRATEGY_WIDE_THRESHOLD,
    } satisfies LoopStrategyProgressModel;
  });
}

function firstLane(pruned: ReadonlySet<number>): number | null {
  const lanes = [...pruned].sort((a, b) => a - b);
  return lanes.at(0) ?? null;
}

function winningLane(
  generation: LoopRunGeneration | undefined,
  laneNodeId: string,
  pruned: ReadonlySet<number>
): number | null {
  const winner = (generation?.outputs ?? []).find(
    output =>
      output.node_id === laneNodeId &&
      output.status === "succeeded" &&
      !pruned.has(output.item_index ?? 0)
  );
  return winner?.item_index ?? null;
}
