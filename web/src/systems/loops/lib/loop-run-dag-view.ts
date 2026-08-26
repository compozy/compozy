import type { LucideIcon } from "lucide-react";

import type { LoopFanoutRollup, LoopRosterNode } from "../types";
import { type LoopGraph, topoOrder } from "./loop-graph";
import { loopNodeClassIcon } from "./loop-node-kind-icons";
import {
  type LoopDagEdge,
  type LoopDagEdgeState,
  edgesCrossingGutter,
  gutterState,
  rankNodes,
  resolveDagEdges,
} from "./loop-run-dag-edges";
import {
  type LoopFanOutBand,
  buildFanOutBand,
  loopAttemptLabel,
  loopFanOutContainerState,
} from "./loop-run-fanout-band";
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
  /** Status channel: tone + glyph + the literal state word, always together. */
  chip: LoopStateChip;
  /** One plain sentence when the state needs explaining; null when it does not. */
  note: string | null;
  attemptLabel: string | null;
  fanOut: LoopFanOutBand | null;
  /**
   * The roster item this card opens — a server-owned `item_index`, never a
   * guess.
   *
   * A fan-out draws one card for many workers, so "the card" has to name which
   * worker the panel should open. Assuming item 0 targeted a row that need not
   * exist: fan-out item indexes come from the daemon and are not guaranteed to
   * start at zero or be contiguous. `null` means no roster row exists for this
   * node yet, which is the one case where there is nothing to target.
   */
  itemIndex: number | null;
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
  return "idle";
}

/**
 * The branches a fan-out card stands in for — never the card itself.
 *
 * A fan-out reaches the roster in one of two authored shapes. It may fan into
 * separately named downstream nodes, in which case those nodes must leave the
 * lane because the card now represents them. Or it may spread one node across
 * item slots, in which case every branch carries the fan-out's own id and there
 * is no sibling to hide — the card *is* the fan.
 */
function branchesOutside(node: LoopDagNode) {
  return (node.fanOut?.branches ?? []).filter(branch => branch.nodeId !== node.nodeId);
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
  const authoredByNode = new Map(graph.nodes.map(node => [node.id, node]));
  const rollupByNode = new Map(roundRollups.map(rollup => [rollup.node_id, rollup]));
  const byNode = new Map<string, LoopRosterNode[]>();
  for (const node of inRound) {
    const bucket = byNode.get(node.node_id);
    if (bucket) bucket.push(node);
    else byNode.set(node.node_id, [node]);
  }

  /**
   * Which worker a collapsed fan-out card opens.
   *
   * The one a person would have gone looking for: the worst-off worker first —
   * a failure or a hold is why anybody opens a fan-out — and otherwise the
   * lowest item the daemon actually recorded. Tone comes from the shared state
   * model, so this ranking cannot drift from what the chips already say.
   */
  const representativeItem = (rows: readonly LoopRosterNode[]): number | null => {
    if (rows.length === 0) return null;
    const rank = (state: string): number => {
      const tone = loopRosterStateChip(state).tone;
      return tone === "danger" ? 0 : tone === "warning" ? 1 : 2;
    };
    return [...rows].sort(
      (left, right) => rank(left.state) - rank(right.state) || left.item_index - right.item_index
    )[0].item_index;
  };

  const dagNodes: LoopDagNode[] = [];
  for (const nodeId of topoOrder(graph)) {
    const authored = authoredByNode.get(nodeId);
    const rollup = rollupByNode.get(nodeId);
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
      dagNodes.push({
        key: nodeId,
        nodeId,
        kindIcon,
        chip: loopRosterStateChip(
          loopFanOutContainerState(
            band.branches.map(branch => branch.chip.state),
            rollup
          )
        ),
        note: band.wide ? band.summary : null,
        attemptLabel: null,
        fanOut: band,
        itemIndex: representativeItem(rows),
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
      chip: loopRosterStateChip(state),
      note: stateNote(state),
      attemptLabel: row ? loopAttemptLabel(row.attempt) : null,
      fanOut: null,
      itemIndex: row?.item_index ?? null,
    });
  }

  // Branches drawn inside a fan-out never reappear as siblings in the lane —
  // except the fan-out's own card. A fan-out that spreads one node across item
  // slots gives every branch that node's id, so claiming by bare id would make
  // the container claim itself and delete the only card the fan has.
  const claimed = new Set(
    dagNodes.flatMap(node => branchesOutside(node).map(branch => branch.nodeId))
  );
  const laneNodes = dagNodes.filter(node => !claimed.has(node.nodeId));

  const { focusNodeId, focusReason } = resolveFocus(laneNodes);

  const byId = new Map(laneNodes.map(node => [node.nodeId, node]));
  const visible = new Set(laneNodes.map(node => node.nodeId));
  // Every hidden worker points at the band that now stands for it, so an
  // authored path running through it survives the collapse. A same-node fan-out
  // hides no sibling, so it maps nothing.
  const collapsedInto = new Map<string, string>();
  for (const node of dagNodes) {
    for (const branch of branchesOutside(node)) {
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
  for (const node of laneNodes) {
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
    nodes: laneNodes,
    columns,
    edges,
    focusNodeId,
    focusReason: focusReason ?? (laneNodes.length > 0 ? "Nothing needs you now." : null),
    round,
    empty: false,
  };
}
