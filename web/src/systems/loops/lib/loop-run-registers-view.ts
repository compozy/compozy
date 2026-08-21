import type { LoopBriefing, LoopFanoutRollup, LoopRosterNode, LoopTimelineEntry } from "../types";
import type { LoopGraph } from "./loop-graph";
import { type LoopNodePanelModel, buildNodePanel } from "./loop-node-panel-view";
import { type LoopRunOutcomeModel, buildRunOutcome } from "./loop-run-artifacts";
import { type LoopBriefingView, buildBriefingView } from "./loop-run-briefing-view";
import { type LoopRunDagModel, buildRunDag } from "./loop-run-dag-view";
import { type LoopStepsProgressModel, buildStepsProgress } from "./loop-run-progress";
import { type LoopStoryBeat, buildStoryBeats } from "./loop-run-story-beats";

/**
 * Both registers, projected from the same three reads.
 *
 * The default read and the operator register are not two sources of truth with a
 * toggle between them — they are two projections of one roster, one briefing and
 * one timeline. That is what makes it impossible for the graph to disagree with
 * the step count, or for the story to disagree with either.
 */

export interface LoopRunRegistersInput {
  briefing: LoopBriefing | null;
  nodes: readonly LoopRosterNode[];
  rollups: readonly LoopFanoutRollup[];
  timeline: readonly LoopTimelineEntry[];
  graph: LoopGraph | null;
  /** Sessions the run recorded that retention has since removed. */
  prunedSessionIds?: ReadonlySet<string>;
  /** Which round the operator register is drawing; defaults to the briefing's. */
  round?: number;
  /** False while roster pages are still arriving. */
  rosterIsComplete?: boolean;
  /** True when the run holds more rows than the page pulled on its own. */
  rosterIsTruncated?: boolean;
}

/**
 * How much of the run these projections actually read.
 *
 * The roster feeds the step list, the graph, the node table and the per-round
 * usage alike, so a partial read makes all four short at once. Every surface
 * built on it has to be able to say so — a loaded page presented as the whole
 * run is the one thing a legibility surface must never do.
 */
export interface LoopRosterReach {
  isComplete: boolean;
  isTruncated: boolean;
  loadedCount: number;
}

/** The exact continuation, with the run id the command actually requires. */
export const LOOP_ROSTER_CONTINUATION_COMMAND = "compozy loop nodes --run <run id> --all";

export function loopRosterReachNote(reach: LoopRosterReach): string | null {
  if (reach.isTruncated) {
    return `This run has more steps than one read returns. Showing the first ${reach.loadedCount}.`;
  }
  return reach.isComplete ? null : "Still reading this run's steps…";
}

export interface LoopRunRegisters {
  briefing: LoopBriefingView | null;
  outcome: LoopRunOutcomeModel | null;
  /** Null until the briefing arrives — its counts are the only truthful source. */
  progress: LoopStepsProgressModel | null;
  dag: LoopRunDagModel;
  beats: LoopStoryBeat[];
  /** Rounds present in the roster, newest first, for the round filter. */
  rounds: number[];
  round: number;
  nodeCount: number;
  eventCount: number;
  /** How much of the run every roster-derived projection here actually read. */
  reach: LoopRosterReach;
}

function roundsIn(nodes: readonly LoopRosterNode[]): number[] {
  const rounds = new Set<number>();
  for (const node of nodes) rounds.add(node.generation);
  return [...rounds].sort((left, right) => right - left);
}

export function projectLoopRunRegisters(input: LoopRunRegistersInput): LoopRunRegisters {
  const { briefing, nodes, rollups, timeline, graph } = input;
  const rounds = roundsIn(nodes);
  const round = input.round ?? briefing?.progress.round ?? rounds[0] ?? 1;
  return {
    briefing: briefing ? buildBriefingView(briefing) : null,
    outcome: briefing ? buildRunOutcome(briefing) : null,
    progress: briefing
      ? buildStepsProgress({ progress: briefing.progress, nodes, rollups, graph })
      : null,
    dag: buildRunDag({ graph, nodes, rollups, round }),
    beats: buildStoryBeats(timeline),
    rounds,
    round,
    nodeCount: nodes.length,
    eventCount: timeline.length,
    reach: {
      isComplete: input.rosterIsComplete ?? true,
      isTruncated: input.rosterIsTruncated ?? false,
      loadedCount: nodes.length,
    },
  };
}

export interface LoopNodeSelection {
  nodeId: string;
  itemIndex: number;
  generation: number;
}

/**
 * The roster row a selection points at — node, item and round together, because
 * the same node id exists once per round and once per fan-out worker.
 */
export function selectedRosterNode(
  nodes: readonly LoopRosterNode[],
  selection: LoopNodeSelection | null
): LoopRosterNode | null {
  if (!selection) return null;
  return (
    nodes.find(
      entry =>
        entry.node_id === selection.nodeId &&
        entry.item_index === selection.itemIndex &&
        entry.generation === selection.generation
    ) ?? null
  );
}

/** Opens one node from the roster, by the identity the roster itself uses. */
export function selectNodePanel(
  nodes: readonly LoopRosterNode[],
  selection: LoopNodeSelection | null,
  graph: LoopGraph | null,
  prunedSessionIds?: ReadonlySet<string>
): LoopNodePanelModel | null {
  const node = selectedRosterNode(nodes, selection);
  if (!node) return null;
  return buildNodePanel({ node, graph, prunedSessionIds });
}
