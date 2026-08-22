import { isNeverMaterializedRosterState } from "../lib/loop-run-state-copy";
import type { LoopRosterNode, LoopRun } from "../types";
import { graphEngRunFixtures } from "./fixture-graph-eng-runs";
import { loopRunFixtures } from "./fixtures";

/**
 * The shared vocabulary the run-read fixtures are built from.
 *
 * Split out so each staged run owns its own file: the seeds are what every one
 * of them agrees on, and agreeing in one place is what keeps a roster node in
 * the terminal fixture shaped like a roster node in the running one.
 */

export const runsById = new Map<string, LoopRun>(
  [...loopRunFixtures, ...graphEngRunFixtures].map(run => [run.id, run])
);

/**
 * The activity and chatter tiers of `internal/loop/timeline.go`. Everything else
 * is notable, which is the view the page reads by default.
 */
export const TIMELINE_NOISE_KINDS: ReadonlySet<string> = new Set([
  "node_running",
  "channel_msg",
  "token_tick",
  "goal_turn_started",
  "goal_turn_completed",
  "runtime_applied",
  "predicate_diagnostic",
  "node_wait_started",
  "node_wait_resumed",
  "effect_results",
  "custom_event",
  "duplicate_suppressed",
  "stale_schedule_dropped",
  "late_arrival",
]);

export type RosterNodeSeed = Partial<LoopRosterNode> & Pick<LoopRosterNode, "node_id" | "state">;

export function rosterNode(
  runId: string,
  generation: number,
  seed: RosterNodeSeed
): LoopRosterNode {
  const itemIndex = seed.item_index ?? 0;
  // The same predicate the node panel and the verb resolver read, so a fixture
  // can never disagree with the UI about whether a node was ever reached.
  const materialized = !isNeverMaterializedRosterState(seed.state);
  return {
    generation,
    item_index: itemIndex,
    attempt: 0,
    attempts: [],
    // The workspace-task id the daemon mints per node cell; absent until the
    // node actually ran.
    ...(materialized
      ? { cell_task_id: `loop.${runId}.g${generation}.node.${seed.node_id}.${itemIndex}` }
      : {}),
    ...seed,
  };
}
