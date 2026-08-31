import { isOpenWait, type LoopNodeLifecycle } from "./loop-node-lifecycle";
import { isTerminalLoopStatus } from "./loop-formatters";

/**
 * Node + run verb gating (DESIGN-LESSONS L12, SD-007): a control renders only for
 * a state the payload declares. The daemon owns the transition; these lists own
 * nothing but *which request is offerable right now*, so the run page, the row
 * menus, and the inventory can never disagree — and can never surface a verb the
 * daemon would reject.
 *
 * The vocabulary is the public contract's, verbatim:
 * pause `mode` is `drain | cancel`; resume `mode` is
 * `plain | reset_attempts | immediate`; cancel/requeue carry only a reason.
 */

export type LoopNodeVerb =
  | "pause"
  | "resume"
  | "resume-reset-attempts"
  | "resume-immediate"
  | "resume-wait"
  | "cancel"
  | "requeue"
  | "open-quarantine"
  | "amend"
  | "rerun";

export type LoopRunVerb = "pause" | "resume" | "cancel";

/** The `mode` value each resume verb posts. */
export const LOOP_NODE_RESUME_MODES = {
  resume: "plain",
  "resume-reset-attempts": "reset_attempts",
  "resume-immediate": "immediate",
} as const satisfies Record<string, string>;

/** The `mode` value each pause choice posts. */
export const LOOP_NODE_PAUSE_MODES = {
  drain: "drain",
  cancel: "cancel",
} as const satisfies Record<string, string>;

export type LoopNodePauseMode = keyof typeof LOOP_NODE_PAUSE_MODES;

/** The item the daemon still exposes as resumable, excluding settled wait history. */
export function loopNodeWaitResumeItemIndex(node: LoopNodeLifecycle): number | undefined {
  return node.waits.find(isOpenWait)?.itemIndex;
}

/**
 * Verbs offerable for one node, in menu order.
 *
 * - `running` — pause / cancel. No resume (nothing is parked) and no
 *   requeue (nothing was set aside).
 * - `retrying` — same set: pausing a backoff parks the pending retry, and
 *   skipping the backoff is the two-step Pause → Resume now (`immediate`).
 * - `paused` — the three resume modes plus cancel; requeue stays absent
 *   because a paused node was never quarantined.
 * - `waiting` — resume takes a payload validated against the wait; the other
 *   resume modes do not apply to a wait cell.
 * - `quarantined` — requeue is the recovery verb; resume is not offered.
 * - `canceled` — the settled node offers nothing.
 */
export function loopNodeVerbs(
  node: LoopNodeLifecycle,
  runStatus: string | null | undefined,
  timetravel?: LoopNodeTimetravelCapability
): LoopNodeVerb[] {
  // A terminal run has no live work to act on; the daemon answers every verb
  // with a refusal. What survives is time travel — amend and rerun read history
  // rather than steer a run — so a settled run offers those and nothing else.
  const timeTravelVerbs = loopNodeTimetravelVerbs(node, runStatus, timetravel);
  if (isTerminalLoopStatus(runStatus)) return timeTravelVerbs;
  if (node.quarantined) return ["open-quarantine", "requeue", "cancel"];
  if (node.cancelState !== "") return [];
  if (node.paused) {
    return ["resume", "resume-reset-attempts", "resume-immediate", ...timeTravelVerbs, "cancel"];
  }
  if (loopNodeWaitResumeItemIndex(node) !== undefined) {
    return ["resume-wait", ...timeTravelVerbs, "cancel"];
  }
  return ["pause", ...timeTravelVerbs, "cancel"];
}

export interface LoopNodeTimetravelCapability {
  hasOutputShape: boolean;
  isGenerationBusy: boolean;
}

const SETTLED_OUTPUT_STATUSES = new Set(["succeeded", "failed", "partial", "canceled"]);

function loopNodeTimetravelVerbs(
  node: LoopNodeLifecycle,
  runStatus: string | null | undefined,
  timetravel: LoopNodeTimetravelCapability | undefined
): LoopNodeVerb[] {
  if (!timetravel) return [];
  const verbs: LoopNodeVerb[] = [];
  const settled = SETTLED_OUTPUT_STATUSES.has(node.outputStatus);

  if (settled && (node.paused || node.parked) && timetravel.hasOutputShape) {
    verbs.push("amend");
  }

  if (settled && !node.parked && !node.paused && !timetravel.isGenerationBusy) {
    verbs.push("rerun");
  }

  if (isTerminalLoopStatus(runStatus)) return verbs.filter(verb => verb === "rerun");
  return verbs;
}

/**
 * Run-level verbs. Pause is valid only from `running` (the daemon rejects it
 * elsewhere, N-003); resume only from `paused`; cancel stays available for any
 * live run and immediately closes the run.
 */
export function loopRunVerbs(
  status: string | null | undefined,
  pauseRequested: boolean
): LoopRunVerb[] {
  if (isTerminalLoopStatus(status)) return [];
  const verbs: LoopRunVerb[] = [];
  if (status === "running" && !pauseRequested) verbs.push("pause");
  if (status === "paused") verbs.push("resume");
  verbs.push("cancel");
  return verbs;
}

export interface LoopNodeVerbPresentation {
  label: string;
  /** Mono trail shown beside the item — the `mode` the request actually posts. */
  mode?: string;
  destructive: boolean;
}

/**
 * Menu copy for each verb (COPY.md sentence case; a trailing ellipsis means the
 * verb opens a confirm or a form rather than firing immediately).
 */
export const LOOP_NODE_VERB_PRESENTATION: Record<LoopNodeVerb, LoopNodeVerbPresentation> = {
  pause: { label: "Pause…", destructive: false },
  resume: { label: "Resume", mode: "plain", destructive: false },
  "resume-reset-attempts": {
    label: "Resume, reset attempts…",
    mode: "reset_attempts",
    destructive: false,
  },
  "resume-immediate": { label: "Resume now", mode: "immediate", destructive: false },
  "resume-wait": { label: "Resume with payload…", destructive: false },
  cancel: { label: "Cancel…", destructive: true },
  requeue: { label: "Requeue…", destructive: false },
  "open-quarantine": { label: "Open quarantine entry", destructive: false },
  amend: { label: "Amend output…", destructive: false },
  rerun: { label: "Rerun from here…", destructive: false },
};

/**
 * The deterministic answer the daemon returns when a verb no longer applies
 * (PRD UI/UX: "idempotent answers render as informative, not as errors").
 *
 * The daemon's reason codes are the vocabulary — `node_not_paused`,
 * `node_not_quarantined`, `run_terminal`, `already_decided` — and its `details`
 * carry the state that won plus, for a lost race, the winning actor. Nothing
 * here is inferred: an unrecognized code falls through to the daemon's own
 * message rather than being dressed up in copy that might not be true.
 */
export interface LoopControlAnswer {
  tone: "info" | "success" | "warning";
  title: string;
  detail: string;
  /** Verbs the daemon says are valid from the actual state; may be empty. */
  allowedTransitions: string[];
  /** Mono trail: the reason code and the state that rejected the verb. */
  micro: string;
}

export interface LoopControlAnswerInput {
  code: string;
  message: string;
  actualState: string;
  allowedTransitions: string[];
  winnerActorKind: string;
  winnerActorId: string;
  winnerReason: string;
  winnerRequestedAt: string;
  /** Human label for the subject — a node id, or the run id for run verbs. */
  subject: string;
}

function winnerLabel(kind: string, id: string): string {
  if (id !== "") return id;
  return kind === "" ? "someone else" : kind;
}

function relativeVerbList(verbs: string[]): string {
  if (verbs.length === 0) return "";
  if (verbs.length === 1) return verbs[0];
  return `${verbs.slice(0, -1).join(", ")} or ${verbs[verbs.length - 1]}`;
}

export function loopControlAnswer(input: LoopControlAnswerInput): LoopControlAnswer {
  const micro = [input.code || "rejected", input.actualState ? `state ${input.actualState}` : null]
    .filter(Boolean)
    .join(" · ");
  const base = { allowedTransitions: input.allowedTransitions, micro };
  switch (input.code) {
    case "node_not_paused": {
      const next = relativeVerbList(input.allowedTransitions);
      return {
        ...base,
        tone: "info",
        title: "Nothing to resume",
        detail:
          `${input.subject} isn't paused — it's ${input.actualState || "not parked"}.` +
          (next === "" ? "" : ` From here you can ${next} it.`),
      };
    }
    case "node_not_quarantined":
      return {
        ...base,
        tone: "info",
        title: "Already out of quarantine",
        detail: `${input.subject} left quarantine and is ${
          input.actualState || "no longer set aside"
        } — nothing left to requeue.`,
      };
    case "run_terminal":
      return {
        ...base,
        tone: "info",
        title: "This run is closed",
        detail: `The run ended as ${
          input.actualState || "terminal"
        } — node controls no longer apply. Starting again is a new run.`,
      };
    case "already_decided": {
      const who = winnerLabel(input.winnerActorKind, input.winnerActorId);
      const because = input.winnerReason === "" ? "" : ` — “${input.winnerReason}”`;
      return {
        ...base,
        tone: "success",
        title: "Already handled",
        detail: `Someone got there first: ${who} already acted on ${input.subject}${because}. Nothing was done twice.`,
        micro: [micro, input.winnerRequestedAt === "" ? null : `winner ${input.winnerRequestedAt}`]
          .filter(Boolean)
          .join(" · "),
      };
    }
    default:
      return {
        ...base,
        tone: "warning",
        title: "The request did not go through",
        detail: input.message,
      };
  }
}
