import type { LoopRosterNode } from "../types";
// Straight from the module that owns it; the story projection only re-exported it.
import { humanizeLoopNodeId } from "./loop-node-labels";
import type { LoopNodeLifecycle } from "./loop-node-lifecycle";
import { isNeverMaterializedRosterState } from "./loop-run-state-copy";

/**
 * Bridges a roster selection to the row the verb rules read.
 *
 * The operator register selects nodes by roster identity — node, item, round —
 * while `loopNodeVerbs` reads a lifecycle row. Those are two views of the same
 * node, and this is the only place they are reconciled.
 *
 * The reconciliation has to stay honest in both directions. Where a durable
 * lifecycle row exists it always wins: it carries control, wait and quarantine
 * truth the roster does not model, and a synthesized stand-in would quietly
 * strip a pause or a quarantine and offer verbs the daemon will refuse.
 *
 * Where no row exists, that absence is itself information. `projectNodeLifecycles`
 * skips a node precisely when it has no control, no open wait and no scheduled
 * retry — a healthy node — so a stand-in built from the roster asserts nothing
 * the runtime has not already said. It is still the daemon, through
 * `loopNodeVerbs`, that decides which verbs that state permits.
 */

function latestAttempt(node: LoopRosterNode) {
  return node.attempts.reduce<LoopRosterNode["attempts"][number] | null>(
    (latest, attempt) => (latest === null || attempt.attempt > latest.attempt ? attempt : latest),
    null
  );
}

/**
 * A lifecycle row for a node the projection skipped, carrying only what the
 * roster genuinely knows. Every control-shaped field is empty because the node
 * having none is exactly why it was skipped.
 */
function synthesizeFromRoster(node: LoopRosterNode): LoopNodeLifecycle {
  const attempt = latestAttempt(node);
  return {
    nodeId: node.node_id,
    label: humanizeLoopNodeId(node.node_id),
    state: null,
    parked: false,
    paused: false,
    pauseProvenance: null,
    quarantined: false,
    quarantinedAt: null,
    quarantineEntry: null,
    attentionFlag: "",
    attentionReason: "",
    attentionProducerNodeId: "",
    cancelState: "",
    cancelProvenance: null,
    lastEvidenceAt: node.ended_at ?? node.started_at ?? null,
    deathResumeStreak: 0,
    revision: 1,
    waits: [],
    attempt: node.attempt > 0 ? node.attempt : null,
    nextAttemptAt: node.next_retry_at ?? null,
    failureClass: attempt?.failure_class ?? "",
    disposition: attempt?.disposition ?? "",
    outputStatus: node.state,
    generation: node.generation,
    itemIndex: node.item_index,
    sessionId: node.session_id?.trim() || null,
  };
}

export interface NodeVerbSelection {
  nodeId: string;
  itemIndex: number;
  generation: number;
}

export function resolveNodeVerbTarget(
  selection: NodeVerbSelection | null,
  lifecycles: readonly LoopNodeLifecycle[],
  nodes: readonly LoopRosterNode[]
): LoopNodeLifecycle | null {
  if (!selection) return null;
  const rosterNode = nodes.find(
    node =>
      node.node_id === selection.nodeId &&
      node.item_index === selection.itemIndex &&
      node.generation === selection.generation
  );
  // A node the run never reached has no session, no record and nothing to
  // intervene on. Offering verbs here would invent a control over work that
  // does not exist (US-015.EC-2).
  if (rosterNode && isNeverMaterializedRosterState(rosterNode.state)) return null;

  // Identity is node + round + item, never the node id alone: a fan-out worker
  // and its sibling share an id, and so does the same step in a later round.
  // Matching on the id would hand a verb to a different cell than the one that
  // was selected, which is the worst possible kind of "works most of the time".
  const durable = lifecycles.find(
    entry =>
      entry.nodeId === selection.nodeId &&
      entry.generation === selection.generation &&
      (entry.itemIndex ?? 0) === selection.itemIndex
  );
  if (durable) return durable;
  return rosterNode ? synthesizeFromRoster(rosterNode) : null;
}
