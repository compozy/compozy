import type { LoopRosterAttempt, LoopRosterNode } from "../types";
import { type LoopGraph, type LoopGraphNode } from "./loop-graph";
import { loopDagKindLabel } from "./loop-node-labels";
import {
  type LoopStateChip,
  isNeverMaterializedRosterState,
  loopRosterStateChip,
} from "./loop-run-state-copy";

/**
 * One node, opened.
 *
 * Navigation from here must never dead-end (US-015). A session that outlived the
 * run still opens; a session retention has pruned degrades to a sentence rather
 * than an error page; and a node that never materialised offers no links at all,
 * because a link to a record that was never written is a lie the operator only
 * discovers by clicking it.
 */

export interface LoopNodeLink {
  kind: "session" | "record" | "child-run";
  label: string;
  /** The identifier the link resolves; never a fabricated one. */
  id: string;
}

/** A link that cannot be followed any more, with the reason it cannot. */
export interface LoopNodeDegradedLink {
  kind: "session" | "record" | "child-run";
  note: string;
}

export interface LoopNodeAttemptRow {
  key: string;
  attempt: number;
  chip: LoopStateChip;
  /** The failure class in plain words, when the attempt recorded one. */
  failureLabel: string | null;
  startedAt: string;
  endedAt: string | null;
}

export interface LoopNodeCancellationView {
  /** Strategy-canceled and operator-canceled are different dispositions. */
  disposition: string;
  label: string;
  actorLabel: string | null;
  cause: string | null;
}

export interface LoopNodePanelModel {
  nodeId: string;
  itemIndex: number;
  generation: number;
  kindLabel: string;
  chip: LoopStateChip;
  /** "attempt 2 of 3" when the definition declares a ceiling, "attempt 2" otherwise. */
  attemptLabel: string | null;
  nextRetryAt: string | null;
  startedAt: string | null;
  endedAt: string | null;
  attempts: LoopNodeAttemptRow[];
  cancellation: LoopNodeCancellationView | null;
  links: LoopNodeLink[];
  degradedLinks: LoopNodeDegradedLink[];
  /** True for a node the run never reached — no links, no fabricated timing. */
  neverMaterialized: boolean;
}

const FAILURE_LABELS: Record<string, string> = {
  tool_error: "tool error",
  timeout: "timed out",
  model_refusal: "the model refused",
  invalid_output: "invalid output",
  canceled: "canceled",
  budget_exceeded: "budget exceeded",
};

function failureLabel(attempt: LoopRosterAttempt): string | null {
  const raw = attempt.failure_class?.trim();
  if (!raw) return null;
  // An unmapped class still reads better as spaced words than as a wire value.
  return FAILURE_LABELS[raw] ?? raw.replaceAll("_", " ");
}

const CANCELLATION_LABELS: Record<string, string> = {
  strategy: "Canceled by the loop's strategy",
  operator: "Canceled by an operator",
  run_terminal: "Canceled when the run ended",
  drained: "Canceled when its siblings settled",
};

/**
 * Reason codes the runtime writes for its own cancellations.
 *
 * The daemon puts these in `cause` so an agent can match on them, but the label
 * above already says the same thing in words — rendering both would print
 * `canceled_by_strategy` underneath "Canceled by the loop's strategy". A named
 * set rather than a snake_case regex: an operator's own reason may contain
 * underscores, and swallowing their sentence would be worse than the leak.
 */
const RUNTIME_CANCELLATION_CAUSES = new Set(["canceled_by_strategy", "canceled_never_started"]);

/**
 * Exported because the roster needs the same reading.
 *
 * `nr-cancel` locks cause and actor to the row itself — "not a tooltip" — so a
 * strategy cancellation and an operator one have to read differently without
 * opening anything. Sharing this builder keeps the runtime-reason suppression
 * above from having to be remembered twice.
 */
export function buildNodeCancellationView(node: LoopRosterNode): LoopNodeCancellationView | null {
  const cancellation = node.cancellation;
  if (!cancellation) return null;
  const actorRef = cancellation.actor_ref?.trim();
  const actorKind = cancellation.actor_kind?.trim();
  const cause = cancellation.cause?.trim() ?? "";
  return {
    disposition: cancellation.disposition,
    label: CANCELLATION_LABELS[cancellation.disposition] ?? "Canceled",
    actorLabel: actorRef ? actorRef : actorKind || null,
    cause: cause && !RUNTIME_CANCELLATION_CAUSES.has(cause) ? cause : null,
  };
}

function attemptRows(node: LoopRosterNode): LoopNodeAttemptRow[] {
  return [...node.attempts]
    .sort((left, right) => right.attempt - left.attempt)
    .map(attempt => ({
      key: `${node.node_id}:${node.item_index}:${attempt.attempt}`,
      attempt: attempt.attempt,
      chip: loopRosterStateChip(attempt.state),
      failureLabel: failureLabel(attempt),
      startedAt: attempt.started_at,
      endedAt: attempt.ended_at ?? null,
    }));
}

function attemptLabel(node: LoopRosterNode, authored: LoopGraphNode | undefined): string | null {
  if (node.attempt <= 1) return null;
  const ceiling = authored?.retryMaxAttempts;
  // "of 3" may only be said when an author wrote the 3. The run payload carries
  // no ceiling, so inventing a denominator would be inventing a policy.
  return ceiling === undefined
    ? `attempt ${node.attempt}`
    : `attempt ${node.attempt} of ${ceiling}`;
}

export interface BuildNodePanelInput {
  node: LoopRosterNode;
  graph: LoopGraph | null;
  /** Sessions the run recorded but retention has since removed. */
  prunedSessionIds?: ReadonlySet<string>;
}

export function buildNodePanel({
  node,
  graph,
  prunedSessionIds,
}: BuildNodePanelInput): LoopNodePanelModel {
  const authored = graph?.nodes.find(entry => entry.id === node.node_id);
  const neverMaterialized = isNeverMaterializedRosterState(node.state);
  const links: LoopNodeLink[] = [];
  const degradedLinks: LoopNodeDegradedLink[] = [];

  if (!neverMaterialized) {
    const sessionId = node.session_id?.trim();
    if (sessionId) {
      // The link survives the run: "open session" works during and after
      // (US-015.AC-1). Only retention takes it away, and then it says so.
      if (prunedSessionIds?.has(sessionId)) {
        degradedLinks.push({ kind: "session", note: "Session no longer available" });
      } else {
        links.push({ kind: "session", label: "Open session", id: sessionId });
      }
    }
    const cellTaskId = node.cell_task_id?.trim();
    if (cellTaskId) links.push({ kind: "record", label: "Open record", id: cellTaskId });
    const childRunId = node.child_loop_run_id?.trim();
    if (childRunId) links.push({ kind: "child-run", label: "View child run", id: childRunId });
  }

  return {
    nodeId: node.node_id,
    itemIndex: node.item_index,
    generation: node.generation,
    kindLabel: loopDagKindLabel(authored),
    chip: loopRosterStateChip(node.state),
    attemptLabel: attemptLabel(node, authored),
    nextRetryAt: node.next_retry_at ?? null,
    startedAt: neverMaterialized ? null : (node.started_at ?? null),
    endedAt: neverMaterialized ? null : (node.ended_at ?? null),
    attempts: neverMaterialized ? [] : attemptRows(node),
    cancellation: buildNodeCancellationView(node),
    links,
    degradedLinks,
    neverMaterialized,
  };
}
