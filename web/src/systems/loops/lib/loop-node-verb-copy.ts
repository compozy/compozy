import type { LoopNodeVerb, LoopRunVerb } from "./loop-node-controls";
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

/** The one-line current-state strip: what the daemon says right now. */
export function loopNodeStateStrip(node: LoopNodeLifecycle): string {
  switch (node.state) {
    case "quarantined": {
      const attempts = node.quarantineEntry?.attemptCount ?? 0;
      const episodes = node.quarantineEntry?.episodes.length ?? 0;
      return [
        `${node.nodeId} is quarantined`,
        attempts > 0 ? `${attempts} attempts` : null,
        episodes > 0 ? `episode ${episodes}` : null,
      ]
        .filter(Boolean)
        .join(" · ");
    }
    case "paused":
      return `${node.nodeId} is paused${
        node.pauseProvenance?.actor_id ? ` by ${node.pauseProvenance.actor_id}` : ""
      }`;
    case "waiting": {
      const wait = node.waits[0];
      return `${node.nodeId} is waiting${wait ? ` · ${wait.kind}` : ""}`;
    }
    case "retrying":
      return `${node.nodeId} is retrying${node.attempt ? ` · attempt ${node.attempt}` : ""}`;
    case "canceling":
      return `${node.nodeId} is ${node.cancelState}`;
    case "canceled":
      return `${node.nodeId} is canceled`;
    default:
      return `${node.nodeId} is ${node.outputStatus || "running"}`;
  }
}

const NODE_VERB_COPY: Record<
  Exclude<LoopNodeVerb, "open-quarantine">,
  (node: LoopNodeLifecycle) => LoopVerbConfirmCopy
> = {
  pause: node => ({
    eyebrow: "Pause node",
    title: `Pause ${node.nodeId}?`,
    body: "The lane stops being scheduled. The rest of the run keeps working. Choose what happens to the attempt already in flight.",
    confirmLabel: "Pause lane",
    cancelLabel: "Keep running",
    tone: "accent",
    micro: "node_paused · with provenance",
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
    body: "The lane winds down cooperatively: in-flight work drains, its cancel reactions run, and finished work is kept.",
    confirmLabel: "Cancel lane",
    cancelLabel: "Keep it",
    tone: "warning",
    micro: "node_canceled · drains first",
  }),
  kill: node => ({
    eyebrow: "Kill node",
    title: `Kill ${node.nodeId}?`,
    body: "The lane stops immediately — no drain, no cleanup reactions. Its work so far is kept but the lane ends as canceled.",
    confirmLabel: "Kill lane",
    cancelLabel: "Keep it",
    tone: "danger",
    micro: "node_killed · no on_* reactions",
  }),
  requeue: node => ({
    eyebrow: "Requeue node",
    title: `Send ${node.nodeId} back to the queue?`,
    body: `The lane leaves quarantine and runs again through the normal path, with all limits applying. If it fails again, it returns here as episode ${
      (node.quarantineEntry?.episodes.length ?? 1) + 1
    }.`,
    confirmLabel: "Requeue lane",
    cancelLabel: "Keep quarantined",
    tone: "accent",
    micro: "node_requeued · origin requeue · with provenance",
  }),
};

export function loopNodeVerbConfirmCopy(
  verb: LoopNodeVerb,
  node: LoopNodeLifecycle
): LoopVerbConfirmCopy | null {
  if (verb === "open-quarantine") return null;
  return NODE_VERB_COPY[verb](node);
}

const RUN_VERB_COPY: Record<
  Extract<LoopRunVerb, "cancel" | "kill">,
  (runId: string, status: string) => LoopVerbConfirmCopy
> = {
  cancel: (runId, status) => ({
    eyebrow: "Cancel run",
    title: `Cancel run ${runId}?`,
    body: "The run winds down cleanly: in-flight lanes drain, their cleanup reactions run, and finished work is kept.",
    confirmLabel: "Cancel run",
    cancelLabel: "Keep running",
    tone: "warning",
    micro: `status_changed · ${status} → canceled · cause operator_cancel`,
  }),
  kill: (runId, status) => ({
    eyebrow: "Kill run",
    title: `Kill run ${runId}?`,
    body: "The run stops immediately: in-flight work is interrupted mid-step and its reactions are skipped. Finished work is still kept. Use kill when a run is misbehaving and can't be trusted to wind down.",
    confirmLabel: "Kill run",
    cancelLabel: "Keep running",
    tone: "danger",
    micro: `status_changed · ${status} → canceled · cause operator_kill`,
  }),
};

export function loopRunVerbConfirmCopy(
  verb: "cancel" | "kill",
  runId: string,
  status: string
): LoopVerbConfirmCopy {
  return RUN_VERB_COPY[verb](runId, status);
}
