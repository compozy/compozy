import type { LucideIcon } from "lucide-react";

import type { LoopFanoutRollup, LoopRosterNode } from "../types";
import { type LoopGraph, type LoopGraphNode, nodeClassLabel, topoOrder } from "./loop-graph";
import { loopNodeClassIcon } from "./loop-node-kind-icons";
import {
  type LoopDagEdge,
  type LoopDagEdgeState,
  edgesCrossingGutter,
  gutterState,
  rankNodes,
  resolveDagEdges,
} from "./loop-run-dag-edges";
import { type LoopFanOutBand, buildFanOutBand } from "./loop-run-fanout-band";
import { type LoopStateChip, loopRosterStateChip } from "./loop-run-state-copy";

/**
 * The run's authored shape, wearing its current state.
 *
 * This is a read-only observability surface. It shares no chrome with the
 * builder canvas and imports nothing from it — the authored topology arrives as
 * a plain graph, the ordering is a topological sort, and the drawing is a row.
 * No layout engine is involved, which is the cheapest possible guarantee that
 * authoring affordances cannot leak in.
 *
 * Structure and status ride separate channels throughout: a node's *kind* is a
 * neutral glyph, its *state* is a signal chip. A failed agent is still an agent,
 * so the glyph never changes colour with the run.
 */

export interface LoopDagNode {
  key: string;
  nodeId: string;
  /** Structure channel: what kind of node this is. Never carries status. */
  kindIcon: LucideIcon | null;
  kindLabel: string;
  /** Status channel: tone + glyph + the literal state word, always together. */
  chip: LoopStateChip;
  /** One plain sentence when the state needs explaining; null when it does not. */
  note: string | null;
  attemptLabel: string | null;
  fanOut: LoopFanOutBand | null;
  isFocus: boolean;
}

/** One column of the layered lane, plus the gutter drawn to its right. */
export interface LoopDagColumn {
  rank: number;
  nodes: LoopDagNode[];
  /** Edges passing this column's right-hand gutter; empty on the last column. */
  gutter: LoopDagEdge[];
  gutterState: LoopDagEdgeState;
}

export interface LoopRunDagModel {
  nodes: LoopDagNode[];
  /** Topologically layered columns — a chain renders as one node per column. */
  columns: LoopDagColumn[];
  edges: LoopDagEdge[];
  /** What the lane centres on: whatever needs a human, else whatever is running. */
  focusNodeId: string | null;
  focusReason: string | null;
  round: number;
  /** True when the definition could not be read — the roster is the fallback view. */
  empty: boolean;
}

/** How many branches a DAG card names before it folds to lanes plus a sentence. */
const DAG_NAMED_BRANCH_LIMIT = 4;

/** Plain words for the authored kind. The DSL spelling stays in the definition. */
const KIND_LABELS: Record<string, string> = {
  "run-agent": "agent",
  "fan-out": "fan-out",
  gate: "gate",
  collect: "collect",
  route: "route",
  ask: "ask",
  wait: "wait",
  goal: "goal",
  transform: "transform",
  "run-loop": "child run",
  "sub-loop": "sub-loop",
  "watch-source": "watch",
  "watch-events": "watch",
  "file-import": "file import",
  input: "input",
};

export function loopDagKindLabel(node: LoopGraphNode | undefined): string {
  if (!node) return "step";
  return KIND_LABELS[node.kind] ?? nodeClassLabel(node);
}

/**
 * The distinction this view exists to make.
 *
 * `pending` is reachable and unsettled — the run may still come this way.
 * `not_taken` is durable evidence that it did not. Conflating them tells an
 * operator that a decided branch is still owed work (Safety Invariant 14).
 */
const STATE_NOTES: Record<string, string> = {
  pending: "Reachable. Nothing has reached it yet.",
  not_taken: "The run took another route.",
  quarantined: "Held back after repeated failures.",
  control_pending: "Waiting for your decision.",
};

function stateNote(state: string): string | null {
  return STATE_NOTES[state] ?? null;
}

/** Trouble first, then activity, then calm — a fan never hides a failed worker. */
const CONTAINER_PRECEDENCE = [
  "failed",
  "quarantined",
  "control_pending",
  "waiting",
  "retrying",
  "paused",
  "running",
  "pending",
  "canceled",
  "succeeded",
];

function containerState(rows: readonly LoopRosterNode[], rollup: LoopFanoutRollup): string {
  for (const candidate of CONTAINER_PRECEDENCE) {
    if (rows.some(row => row.state === candidate)) return candidate;
  }
  return rollup.total > 0 && rollup.done === rollup.total ? "succeeded" : "pending";
}

/**
 * Which node the lane centres on, and why.
 *
 * Ordered by what an operator came here to find: something that needs them,
 * then something that broke, then whatever is moving. A run with none of those
 * is calm, and saying so is more useful than centring on nothing in particular.
 */
const FOCUS_RULES: { states: string[]; reason: (nodeId: string) => string }[] = [
  {
    states: ["control_pending", "quarantined"],
    reason: nodeId => `Centred on ${nodeId} — the step waiting on you.`,
  },
  { states: ["failed"], reason: nodeId => `Centred on ${nodeId} — the step that failed.` },
  { states: ["running"], reason: nodeId => `Centred on ${nodeId} — running now.` },
  { states: ["retrying", "waiting"], reason: nodeId => `Centred on ${nodeId} — waiting.` },
];

function resolveFocus(nodes: readonly LoopDagNode[]): {
  focusNodeId: string | null;
  focusReason: string | null;
} {
  for (const rule of FOCUS_RULES) {
    const match = nodes.find(node => rule.states.includes(node.chip.state));
    if (match) return { focusNodeId: match.nodeId, focusReason: rule.reason(match.nodeId) };
  }
  return { focusNodeId: null, focusReason: null };
}

const SETTLED_FOR_FLOW = new Set(["succeeded", "partial"]);

function edgeState(from: LoopDagNode | undefined, to: LoopDagNode | undefined): LoopDagEdgeState {
  if (!from || !to) return "idle";
  // Route evidence beats everything: a branch the run provably declined is drawn
  // as declined, whatever its neighbours are doing.
  if (to.chip.state === "not_taken") return "not_taken";
  // Liveness travels on the edge *into* whatever is working, so the eye is
  // pulled toward the front of the run rather than to a node that has stopped.
  if (to.chip.state === "running") return "live";
  if (SETTLED_FOR_FLOW.has(from.chip.state) && to.chip.state !== "pending") return "taken";
  if (SETTLED_FOR_FLOW.has(from.chip.state) && SETTLED_FOR_FLOW.has(to.chip.state)) return "taken";
  return "idle";
}

export interface BuildRunDagInput {
  graph: LoopGraph | null;
  nodes: readonly LoopRosterNode[];
  rollups: readonly LoopFanoutRollup[];
  /** The round being drawn; the DAG shows one round's shape at a time. */
  round: number;
}

export function buildRunDag({ graph, nodes, rollups, round }: BuildRunDagInput): LoopRunDagModel {
  if (!graph || graph.nodes.length === 0) {
    return {
      nodes: [],
      columns: [],
      edges: [],
      focusNodeId: null,
      focusReason: null,
      round,
      empty: true,
    };
  }
  const inRound = nodes.filter(node => node.generation === round);
  const roundRollups = rollups.filter(rollup => rollup.generation === round);
  const byNode = new Map<string, LoopRosterNode[]>();
  for (const node of inRound) {
    const bucket = byNode.get(node.node_id);
    if (bucket) bucket.push(node);
    else byNode.set(node.node_id, [node]);
  }

  const dagNodes: LoopDagNode[] = [];
  for (const nodeId of topoOrder(graph)) {
    const authored = graph.nodes.find(entry => entry.id === nodeId);
    const rollup = roundRollups.find(entry => entry.node_id === nodeId);
    const rows = byNode.get(nodeId) ?? [];
    const kindIcon = authored?.nodeClass
      ? loopNodeClassIcon({
          nodeClass: authored.nodeClass,
          isFanOut: authored.kind === "fan-out",
          isGate: authored.isGate,
          isRoute: authored.kind === "route",
          isAsk: authored.kind === "ask",
        })
      : null;

    if (rollup) {
      const band = buildFanOutBand({
        rollup,
        nodes: inRound,
        graph,
        namedLimit: DAG_NAMED_BRANCH_LIMIT,
      });
      const branchKeys = new Set(band.branches.map(branch => branch.key));
      const branchRows = inRound.filter(row => branchKeys.has(`${row.node_id}:${row.item_index}`));
      dagNodes.push({
        key: nodeId,
        nodeId,
        kindIcon,
        kindLabel: loopDagKindLabel(authored),
        chip: loopRosterStateChip(containerState(branchRows, rollup)),
        note: band.wide ? band.summary : null,
        attemptLabel: null,
        fanOut: band,
        isFocus: false,
      });
      continue;
    }

    // An authored node with no roster row has not been reached. It is pending —
    // never `not_taken`, which only durable route evidence may say.
    const row = rows[0];
    const state = row?.state ?? "pending";
    dagNodes.push({
      key: nodeId,
      nodeId,
      kindIcon,
      kindLabel: loopDagKindLabel(authored),
      chip: loopRosterStateChip(state),
      note: stateNote(state),
      attemptLabel: row && row.attempt > 1 ? `attempt ${row.attempt}` : null,
      fanOut: null,
      isFocus: false,
    });
  }

  // Branches drawn inside a fan-out never reappear as siblings in the lane.
  const claimed = new Set(
    dagNodes.flatMap(node => node.fanOut?.branches.map(branch => branch.nodeId) ?? [])
  );
  const laneNodes = dagNodes.filter(node => !claimed.has(node.nodeId));

  const { focusNodeId, focusReason } = resolveFocus(laneNodes);
  const focused = laneNodes.map(node => ({ ...node, isFocus: node.nodeId === focusNodeId }));

  const byId = new Map(focused.map(node => [node.nodeId, node]));
  const visible = new Set(focused.map(node => node.nodeId));
  // Every hidden worker points at the band that now stands for it, so an
  // authored path running through it survives the collapse.
  const collapsedInto = new Map<string, string>();
  for (const node of dagNodes) {
    for (const branch of node.fanOut?.branches ?? []) {
      collapsedInto.set(branch.nodeId, node.nodeId);
    }
  }
  const order = topoOrder(graph);
  const ranks = rankNodes(graph, order);
  const edges = resolveDagEdges({
    graph,
    visible,
    collapsedInto,
    ranks,
    edgeState: (from, to) => edgeState(byId.get(from), byId.get(to)),
  });

  const byRank = new Map<number, LoopDagNode[]>();
  for (const node of focused) {
    const rank = ranks.get(node.nodeId) ?? 0;
    const bucket = byRank.get(rank);
    if (bucket) bucket.push(node);
    else byRank.set(rank, [node]);
  }
  const rankValues = [...byRank.keys()].sort((left, right) => left - right);
  const columns: LoopDagColumn[] = rankValues.map((rank, index) => {
    const gutter = index === rankValues.length - 1 ? [] : edgesCrossingGutter(edges, rank);
    return { rank, nodes: byRank.get(rank) ?? [], gutter, gutterState: gutterState(gutter) };
  });

  return {
    nodes: focused,
    columns,
    edges,
    focusNodeId,
    focusReason: focusReason ?? (focused.length > 0 ? "Nothing needs you now." : null),
    round,
    empty: false,
  };
}
