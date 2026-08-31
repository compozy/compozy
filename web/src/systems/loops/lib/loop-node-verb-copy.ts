import { formatRelativeTime } from "@compozy/ui";

import { loopStatusLabel } from "./loop-formatters";
import type { LoopNodePauseMode, LoopNodeVerb, LoopRunVerb } from "./loop-node-controls";
import type { LoopNodeLifecycle } from "./loop-node-lifecycle";

/**
 * Confirm copy for every state-changing verb (PRD UI/UX: "every destructive or
 * state-changing action confirms with the current state it acts on").
 *
 * `state` is the strip the dialog shows above its footer — it restates what the
 * daemon currently says about the subject, so the operator confirms against
 * present truth rather than the state they last saw on screen.
 */

export interface LoopVerbConfirmCopy {
  eyebrow: string;
  title: string;
  body: string;
  confirmLabel: string;
  cancelLabel: string;
  tone: "danger" | "warning" | "accent" | "neutral";
  /** Mono trail naming the event the verb will produce. */
  micro: string;
}

function attentionClause(node: LoopNodeLifecycle): string | null {
  if (node.attentionFlag === "") return null;
  const flag = node.attentionReason || node.attentionFlag;
  const evidence =
    node.lastEvidenceAt === null
      ? null
      : `last evidence ${formatRelativeTime(node.lastEvidenceAt)}`;
  return [`flagged: ${flag}`, evidence].filter(Boolean).join(" · ");
}

function withAttention(base: string, node: LoopNodeLifecycle): string {
  const attention = attentionClause(node);
  return attention ? `${base} · ${attention}` : base;
}

/** The one-line current-state strip: what the daemon says right now. */
export function loopNodeStateStrip(node: LoopNodeLifecycle): string {
  switch (node.state) {
    case "quarantined": {
      const attempts = node.quarantineEntry?.attemptCount ?? 0;
      const episodes = node.quarantineEntry?.episodes.length ?? 0;
      return withAttention(
        [
          `${node.nodeId} is quarantined`,
          attempts > 0 ? `${attempts} attempts` : null,
          episodes > 0 ? `episode ${episodes}` : null,
        ]
          .filter(Boolean)
          .join(" · "),
        node
      );
    }
    case "paused":
      return withAttention(
        `${node.nodeId} is paused${
          node.pauseProvenance?.actor_id ? ` by ${node.pauseProvenance.actor_id}` : ""
        }`,
        node
      );
    case "waiting": {
      const wait = node.waits[0];
      return withAttention(`${node.nodeId} is waiting${wait ? ` · ${wait.kind}` : ""}`, node);
    }
    case "retrying":
      return withAttention(
        [
          `${node.nodeId} is retrying`,
          node.attempt ? `attempt ${node.attempt}` : null,
          node.nextAttemptAt ? `next ${formatRelativeTime(node.nextAttemptAt)}` : null,
        ]
          .filter(Boolean)
          .join(" · "),
        node
      );
    case "canceling":
      return withAttention(`${node.nodeId} is ${node.cancelState}`, node);
    case "canceled":
      return withAttention(`${node.nodeId} is canceled`, node);
    default:
      return withAttention(`${node.nodeId} is ${node.outputStatus || "running"}`, node);
  }
}

export interface LoopRunStateStripInput {
  runId: string;
  status: string;
  generation: number;
  inFlightCount?: number;
  waitingOnYouCount?: number;
  elapsedLabel?: string;
}

/** Run confirm strip: identity + non-zero lane counts + elapsed. */
export function loopRunStateStrip(input: LoopRunStateStripInput): string {
  const inFlight = input.inFlightCount ?? 0;
  const waiting = input.waitingOnYouCount ?? 0;
  return [
    `${input.runId} is ${loopStatusLabel(input.status).toLowerCase()}`,
    inFlight > 0 ? `${inFlight} ${inFlight === 1 ? "lane" : "lanes"} in flight` : null,
    waiting > 0 ? `${waiting} waiting on you` : null,
    `generation ${input.generation}`,
    input.elapsedLabel ? input.elapsedLabel : null,
  ]
    .filter(Boolean)
    .join(" · ");
}

const NODE_VERB_COPY: Record<
  Exclude<LoopNodeVerb, "amend" | "open-quarantine" | "rerun">,
  (node: LoopNodeLifecycle, pauseMode: LoopNodePauseMode) => LoopVerbConfirmCopy
> = {
  pause: (node, pauseMode) => ({
    eyebrow: "Pause node",
    title: `Pause ${node.nodeId}?`,
    body: "The lane stops being scheduled. The rest of the run keeps working. Choose what happens to the attempt already in flight.",
    confirmLabel: "Pause lane",
    cancelLabel: "Keep running",
    tone: "accent",
    micro: `mode: ${pauseMode}`,
  }),
  resume: node => ({
    eyebrow: "Resume node",
    title: `Resume ${node.nodeId}?`,
    body: "The schedule and attempt counter from pause time are restored, and the lane picks up where it left off.",
    confirmLabel: "Resume lane",
    cancelLabel: "Keep paused",
    tone: "accent",
    micro: "node_resumed · mode plain",
  }),
  "resume-reset-attempts": node => ({
    eyebrow: "Resume node",
    title: `Resume ${node.nodeId} with a clean attempt count?`,
    body: "The retry delay is cleared and the attempt counter restarts at one. Run caps and budgets still apply.",
    confirmLabel: "Reset and resume",
    cancelLabel: "Keep paused",
    tone: "warning",
    micro: "node_resumed · mode reset_attempts",
  }),
  "resume-immediate": node => ({
    eyebrow: "Resume node",
    title: `Resume ${node.nodeId} now?`,
    body: "The remaining backoff is skipped and the next attempt runs immediately. The attempt counter carries over.",
    confirmLabel: "Resume now",
    cancelLabel: "Keep paused",
    tone: "accent",
    micro: "node_resumed · mode immediate",
  }),
  "resume-wait": node => ({
    eyebrow: "Resume wait",
    title: `Resume ${node.nodeId} by hand?`,
    body: "Resuming by hand stands in for the event this lane is waiting for — the payload must match what the wait expects.",
    confirmLabel: "Resume lane",
    cancelLabel: "Keep waiting",
    tone: "accent",
    micro: "node_wait_resumed · by hand",
  }),
  cancel: node => ({
    eyebrow: "Cancel node",
    title: `Cancel ${node.nodeId}?`,
    body: "The lane closes immediately. Active work is stopped automatically, and finished work is kept.",
    confirmLabel: "Cancel lane",
    cancelLabel: "Keep it",
    tone: "danger",
    micro: "node_canceled · stops immediately",
  }),
  requeue: node => {
    const episodes = node.quarantineEntry?.episodes.length;
    const nextEpisode =
      episodes === undefined
        ? ""
        : ` If it fails again, it returns here as episode ${episodes + 1}.`;
    return {
      eyebrow: "Requeue node",
      title: `Send ${node.nodeId} back to the queue?`,
      body: `The lane leaves quarantine and runs again through the normal path, with all limits applying.${nextEpisode}`,
      confirmLabel: "Requeue lane",
      cancelLabel: "Keep quarantined",
      tone: "accent",
      micro: "node_requeued · origin requeue · with provenance",
    };
  },
};

export function loopNodeVerbConfirmCopy(
  verb: LoopNodeVerb,
  node: LoopNodeLifecycle,
  options?: { pauseMode?: LoopNodePauseMode }
): LoopVerbConfirmCopy | null {
  if (verb === "open-quarantine" || verb === "amend" || verb === "rerun") return null;
  return NODE_VERB_COPY[verb](node, options?.pauseMode ?? "drain");
}

const RUN_VERB_COPY: Record<
  Extract<LoopRunVerb, "cancel">,
  (runId: string, status: string) => LoopVerbConfirmCopy
> = {
  cancel: (runId, status) => ({
    eyebrow: "Cancel run",
    title: `Cancel run ${runId}?`,
    body: "The run closes immediately. Active sessions are stopped automatically, and finished work is kept.",
    confirmLabel: "Cancel run",
    cancelLabel: "Keep running",
    tone: "danger",
    micro: `status_changed · ${status} → canceled · cause operator_cancel`,
  }),
};

export function loopRunVerbConfirmCopy(
  verb: "cancel",
  runId: string,
  status: string
): LoopVerbConfirmCopy {
  return RUN_VERB_COPY[verb](runId, status);
}
