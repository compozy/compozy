import type { LoopRunEventFrame } from "../types";
import {
  asRecord,
  humanizeLoopNodeId,
  type MutableStoryRow,
  num,
  str,
} from "./loop-run-story-rows";

/**
 * Story rows for the fifteen node-lifecycle event kinds (Spec 1). Each builder
 * turns one replayed frame into a plain-language beat plus the verbatim mono
 * `micro` trail that keeps its machine identity visible (DESIGN-LESSONS L9).
 *
 * Every sentence is a template over fields the payload actually carries. When
 * the daemon omits a detail — no reason on a pause, no hint on a quarantine —
 * the sentence gets shorter; it never gets invented.
 */

function sentenceCase(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function nodeLabel(nodeId: string): string {
  return humanizeLoopNodeId(nodeId);
}

/** `execute_task · task_03[1] · gen 2` — the shared identity tail. */
function nodeTrail(payload: Record<string, unknown>, kind: string): string {
  const nodeId = str(payload.node_id);
  const itemIndex = num(payload.item_index);
  const generation = num(payload.generation);
  const parts = [kind, `${nodeId}${itemIndex === undefined ? "" : `[${itemIndex}]`}`];
  if (generation !== undefined) parts.push(`gen ${generation}`);
  return parts.join(" · ");
}

function actorClause(payload: Record<string, unknown>): string {
  const kind = str(payload.actor_kind);
  const id = str(payload.actor_id);
  if (kind === "" && id === "") return "";
  if (kind === "user" || kind === "operator") return id === "" ? "you" : `you (${id})`;
  // Either half can be absent on the wire; join only what is actually there so
  // the sentence never carries a stray separator.
  return [kind, id].filter(part => part !== "").join(" ");
}

function reasonClause(payload: Record<string, unknown>): string {
  const reason = str(payload.reason);
  return reason === "" ? "" : ` — “${reason}”`;
}

function base(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>,
  kind: string
): Pick<MutableStoryRow, "key" | "kind" | "seq" | "at" | "nodeId" | "generation"> {
  return {
    key: `f-${frame.seq}`,
    kind,
    seq: num(frame.seq) ?? 0,
    at: str(frame.at),
    nodeId: str(payload.node_id) || undefined,
    generation: num(payload.generation),
  };
}

/** `node_retry_scheduled` — the mechanical auto-retry beat (US-008). */
export function nodeRetryScheduledRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const attempt = num(payload.attempt);
  const failureClass = str(payload.failure_class);
  const nextAttemptAt = str(payload.next_attempt_at);
  const attemptClause = attempt === undefined ? "" : ` Attempt ${attempt} is queued.`;
  const classClause =
    failureClass === ""
      ? ""
      : ` Only ${failureClass}-class failures retry on their own — everything else asks or parks.`;
  return {
    ...base(frame, payload, "node_retry_scheduled"),
    tone: "warning",
    icon: "retry",
    title: `Retry scheduled for ${label}`,
    sub: `${attemptClause}${classClause}`.trim() || undefined,
    micro: `${nodeTrail(payload, "node_retry_scheduled")}${
      nextAttemptAt === "" ? "" : ` · next ${nextAttemptAt}`
    }`,
  };
}

/** `node_paused` — manual park or a rule-driven auto-pause (US-019 / US-020). */
export function nodePausedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const ruleId = str(payload.rule_id);
  const mode = str(payload.mode);
  const actor = actorClause(payload);
  const by = ruleId !== "" ? `rule ${ruleId}` : actor;
  const drained =
    mode === "drain"
      ? " The in-flight attempt finished cleanly before parking."
      : mode === "cancel"
        ? " The in-flight attempt was asked to stop."
        : "";
  return {
    ...base(frame, payload, "node_paused"),
    tone: "neutral",
    icon: "paused",
    title: `${sentenceCase(label)} paused`,
    sub: `${by === "" ? "Parked" : `Parked by ${by}`}${reasonClause(payload)}.${drained}`,
    micro: `${nodeTrail(payload, "node_paused")}${mode === "" ? "" : ` · ${mode}`}${
      ruleId === "" ? "" : ` · ${ruleId}`
    }`,
  };
}

/** `node_resumed` — the chosen resume semantics are recorded (US-019 AC-3). */
export function nodeResumedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const mode = str(payload.mode);
  const sub =
    mode === "reset_attempts"
      ? "Attempt counter restarted; the retry delay was cleared."
      : mode === "immediate"
        ? "The retry delay was skipped; the attempt counter carried over."
        : "The schedule and attempt counter from pause time were restored.";
  return {
    ...base(frame, payload, "node_resumed"),
    tone: "neutral",
    icon: "resumed",
    title: `${sentenceCase(label)} resumed`,
    sub,
    micro: `${nodeTrail(payload, "node_resumed")}${mode === "" ? "" : ` · ${mode}`}`,
  };
}

/** `node_canceled` / `node_killed` — cooperative wind-down vs immediate stop. */
export function nodeCanceledRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>,
  killed: boolean
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const actor = actorClause(payload);
  const by = actor === "" ? "" : ` by ${actor}`;
  return {
    ...base(frame, payload, killed ? "node_killed" : "node_canceled"),
    tone: killed ? "danger" : "neutral",
    icon: killed ? "killed" : "canceled",
    title: killed ? `${sentenceCase(label)} killed` : `${sentenceCase(label)} canceled`,
    sub: killed
      ? `Stopped immediately${by}${reasonClause(payload)}. Its reactions were skipped.`
      : `Walked its cancel steps — requested → delivering → draining → canceled${by}${reasonClause(payload)}.`,
    micro: nodeTrail(payload, killed ? "node_killed" : "node_canceled"),
  };
}

/** `node_quarantined` — the node is set aside; the run keeps going (US-024). */
export function nodeQuarantinedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const attempt = num(payload.attempt);
  const failure = asRecord(payload.failure);
  const cause = str(failure?.cause);
  const attemptClause = attempt === undefined ? "" : ` after ${attempt} attempts`;
  return {
    ...base(frame, payload, "node_quarantined"),
    tone: "danger",
    icon: "quarantined",
    title: `${sentenceCase(label)} was set aside${attemptClause}`,
    sub:
      `${cause === "" ? "" : `${cause} — `}` +
      "Its clock is suspended and no attempts are being spent; the rest of the run keeps going.",
    micro: nodeTrail(payload, "node_quarantined"),
  };
}

/** `node_requeued` — repair-and-continue re-enters through normal succession. */
export function nodeRequeuedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const actor = actorClause(payload);
  const by = actor === "" ? "" : ` by ${actor}`;
  return {
    ...base(frame, payload, "node_requeued"),
    tone: "info",
    icon: "requeued",
    title: `${sentenceCase(label)} requeued`,
    sub: `Re-entered${by}${reasonClause(payload)}. All caps and budgets still apply.`,
    micro: nodeTrail(payload, "node_requeued"),
  };
}

const WAIT_KIND_CLAUSE: Record<string, string> = {
  timer: "Sleeping until its timer is due",
  event: "Waiting for a matching event",
  approval_escalation: "Waiting for your decision",
  dependency: "Waiting on another lane",
};

/** `node_wait_started` — a durable wait cell opened (US-021/US-022). */
export function nodeWaitStartedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const waitKind = str(payload.wait_kind);
  const resumeAt = str(payload.resume_at);
  const clause = WAIT_KIND_CLAUSE[waitKind] ?? "Waiting";
  const resumeClause = resumeAt === "" ? "" : ` — resumes ${resumeAt}`;
  return {
    ...base(frame, payload, "node_wait_started"),
    tone: "info",
    icon: "waiting",
    title: `${sentenceCase(label)} started waiting`,
    sub: `${clause}${resumeClause}. A lane's wait is its own; nothing else is held up.`,
    micro: `${nodeTrail(payload, "node_wait_started")}${waitKind === "" ? "" : ` · ${waitKind}`}`,
  };
}

/** `node_wait_resumed` — the wait resolved and the lane picked back up. */
export function nodeWaitResumedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const waitKind = str(payload.wait_kind);
  const actor = actorClause(payload);
  return {
    ...base(frame, payload, "node_wait_resumed"),
    tone: "neutral",
    icon: "resumed",
    title: `${sentenceCase(label)} stopped waiting`,
    sub:
      actor === ""
        ? "The wait resolved and the lane picked back up."
        : `Resolved by ${actor}; the lane picked back up.`,
    micro: `${nodeTrail(payload, "node_wait_resumed")}${waitKind === "" ? "" : ` · ${waitKind}`}`,
  };
}

const ATTENTION_FLAG_COPY: Record<string, { title: string; sub: string }> = {
  silence: {
    title: "silent for a while",
    sub: "No output, no tool call, no heartbeat — and no confirmed death either. Watch flag only: it clears itself on any evidence of life.",
  },
  dependency_quarantined: {
    title: "blocked on a quarantined lane",
    sub: "Forward progress needs a producer that is set aside. The run parks here and names it — it never waits silently.",
  },
  expired_wait: {
    title: "waiting past its deadline",
    sub: "The wait expired and the loop declares no timeout route, so nothing decided what happens next.",
  },
};

/** `node_attention_flagged` — evidence-only flag, never a stop (US-007 EC-2). */
export function nodeAttentionFlaggedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  // The liveness path writes the flag into `reason`; the escalation path uses a
  // dedicated `attention_flag`. Read both so neither emitter loses its copy.
  const flag = str(payload.attention_flag) || str(payload.reason);
  const copy = ATTENTION_FLAG_COPY[flag];
  return {
    ...base(frame, payload, "node_attention_flagged"),
    tone: "warning",
    icon: "attention",
    title: copy ? `Flagged: ${label} is ${copy.title}` : `Flagged: ${label} needs attention`,
    sub: copy?.sub ?? (str(payload.reason) || undefined),
    micro: `${nodeTrail(payload, "node_attention_flagged")}${flag === "" ? "" : ` · ${flag}`}`,
  };
}

/** `node_attention_cleared` — a self-clearing flag resolved on its own. */
export function nodeAttentionClearedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const label = nodeLabel(str(payload.node_id));
  const flag = str(payload.attention_flag) || str(payload.reason);
  return {
    ...base(frame, payload, "node_attention_cleared"),
    tone: "success",
    icon: "attention-cleared",
    title: `Flag cleared: ${label} is fine`,
    sub: "The evidence the flag was waiting for arrived; no one had to touch it.",
    micro: `${nodeTrail(payload, "node_attention_cleared")}${flag === "" ? "" : ` · ${flag}`}`,
  };
}

/** `effect_results` — a reaction's delivery outcome; never touches the node. */
export function effectResultsRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const outcome = str(payload.outcome);
  const trigger = str(payload.trigger);
  const cause = str(payload.cause);
  const failed = outcome !== "" && outcome !== "ok";
  const triggerClause = trigger === "" ? "A reaction" : `The ${trigger} reaction`;
  return {
    ...base(frame, payload, "effect_results"),
    tone: failed ? "warning" : "neutral",
    icon: "effect",
    title: failed ? `${triggerClause} couldn't be delivered` : `${triggerClause} was delivered`,
    sub: failed
      ? `${cause === "" ? "The delivery failed." : cause} Recorded here and nothing else — reactions never touch the step itself.`
      : undefined,
    micro: `effect_results · ${trigger || "effect"}${outcome === "" ? "" : ` · ${outcome}`}`,
  };
}

/** `custom_event` — an authored `emit` reaction that landed on the event bus. */
export function customEventRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const name = str(payload.name) || str(payload.event) || str(payload.kind);
  return {
    ...base(frame, payload, "custom_event"),
    tone: "neutral",
    icon: "effect",
    title: name === "" ? "A custom event was emitted" : `Emitted ${name}`,
    sub: "An authored reaction published this; it does not change the run's own state.",
    micro: `custom_event${name === "" ? "" : ` · ${name}`}`,
  };
}

/** `duplicate_suppressed` — one delivered event starts exactly one run (US-025). */
export function duplicateSuppressedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const count = num(payload.suppressed_count) ?? 0;
  const eventKey = str(payload.event_key);
  return {
    ...base(frame, payload, "duplicate_suppressed"),
    tone: "info",
    icon: "suppressed",
    title:
      count > 1
        ? `${count} duplicate deliveries were suppressed`
        : "A duplicate delivery was suppressed",
    sub: "The same event arrived again. This run already claimed it, so no second run started.",
    micro: `duplicate_suppressed${eventKey === "" ? "" : ` · ${eventKey}`}`,
  };
}

/**
 * The kinds whose beat ends the node's current attempt, so the orchestrator
 * clears it from the "Happening now" projection exactly as `node_failed` does.
 */
export const LIFECYCLE_KINDS_ENDING_ATTEMPT: ReadonlySet<string> = new Set([
  "node_canceled",
  "node_killed",
  "node_quarantined",
  "node_paused",
  "node_wait_started",
]);

/**
 * Single dispatch for every node-lifecycle kind. Returns null for a kind this
 * module does not own, so the orchestrator's own switch stays the authority for
 * the pre-existing kinds.
 */
export function lifecycleStoryRow(
  kind: string,
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow | null {
  switch (kind) {
    case "node_retry_scheduled":
      return nodeRetryScheduledRow(frame, payload);
    case "node_paused":
      return nodePausedRow(frame, payload);
    case "node_resumed":
      return nodeResumedRow(frame, payload);
    case "node_canceled":
      return nodeCanceledRow(frame, payload, false);
    case "node_killed":
      return nodeCanceledRow(frame, payload, true);
    case "node_quarantined":
      return nodeQuarantinedRow(frame, payload);
    case "node_requeued":
      return nodeRequeuedRow(frame, payload);
    case "node_wait_started":
      return nodeWaitStartedRow(frame, payload);
    case "node_wait_resumed":
      return nodeWaitResumedRow(frame, payload);
    case "node_attention_flagged":
      return nodeAttentionFlaggedRow(frame, payload);
    case "node_attention_cleared":
      return nodeAttentionClearedRow(frame, payload);
    case "effect_results":
      return effectResultsRow(frame, payload);
    case "custom_event":
      return customEventRow(frame, payload);
    case "duplicate_suppressed":
      return duplicateSuppressedRow(frame, payload);
    case "target_breaker_transition":
      return targetBreakerTransitionRow(frame, payload);
    default:
      return null;
  }
}

/** `target_breaker_transition` — a shared target opened or closed its breaker. */
export function targetBreakerTransitionRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const target = str(payload.target);
  const state = str(payload.state);
  const opened = state === "open";
  const name = target === "" ? "a target" : target;
  return {
    ...base(frame, payload, "target_breaker_transition"),
    tone: opened ? "warning" : "success",
    icon: opened ? "breaker-open" : "breaker-closed",
    title: opened ? `Paused calls to ${name}` : `Calls to ${name} resumed`,
    sub: opened
      ? "Repeated connection failures opened its circuit. Calls hold until a probe succeeds; other targets are not affected."
      : "A probe succeeded, so the circuit closed and calls flow again.",
    micro: `target_breaker_transition · ${target || "target"}${state === "" ? "" : ` · ${state}`}`,
  };
}
