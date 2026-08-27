/**
 * Everything call detail renders, decided in one place.
 *
 * The rule this module exists to enforce: **a control appears only when the
 * operation behind it exists right now.** Not greyed, not "coming soon" —
 * absent. Cancel belongs to a call in flight; call-again belongs to one that has
 * settled; messaging belongs to a child that is still addressable. Rendering a
 * disabled button would advertise an operation the runtime would refuse, which
 * is the same lie as inventing one — and so would rendering a control for an
 * operation with no operator surface at all. Those three are the whole set.
 *
 * The second rule: **absence is stated, never filled.** A resultless terminal
 * gets a sentence saying so, not an empty JSON pane. A call with no deadline
 * shows no timer chrome at all — most calls have none, and a dormant clock
 * implies a bound the daemon is not keeping.
 */
import {
  isTerminalCallState,
  toCallState,
  toCallVerdict,
  type CallState,
  type CallVerdict,
} from "./call-state";
import { buildCallResultShape, type CallResultShape } from "./call-result-rows";
import { buildCallTimeline, type CallTimelineEvent } from "./call-detail-timeline";
import type { CallPayload } from "../types";

/**
 * What the idle clock is doing.
 *
 * The TTL is an idle ceiling, not a lifetime: it runs only while the child is
 * parked and suspends the moment a call is in flight, so a working child is
 * never reaped by the clock. Saying "suspended" is therefore a fact about the
 * runtime, not a UI nicety.
 */
export type CallIdleTtl =
  | { kind: "suspended" }
  | { kind: "counting"; expiresAt: string }
  | { kind: "none" };

export interface CallDetailControls {
  /** In flight only. */
  cancel: boolean;
  /** Terminal only — a fresh call to the same agent. */
  callAgain: boolean;
  /** The child is still addressable; contacting it is what revives it. */
  messageChild: boolean;
  /** The counterpart session still exists to open. */
  openChildSession: boolean;
}

export interface CallDetailInput {
  call: CallPayload;
  /**
   * Whether the child session still resolves. Retention prunes sessions while
   * call records are kept indefinitely, so an old call can name a session that
   * no longer exists; the jump link degrades rather than dangling. Callers that
   * have not checked pass `true` — the link then fails loudly on navigation
   * instead of being hidden on a guess.
   */
  counterpartExists?: boolean;
}

export interface CallDetailView {
  callId: string;
  agentName: string | null;
  /** Null when the daemon sent a state word the web does not know. */
  state: CallState | null;
  /** Raw state, always renderable even when narrowing failed. */
  stateLabel: string;
  verdict: CallVerdict | null;
  terminal: boolean;
  callerId: string;
  /** Wire `caller.kind` — shown as `(session)` when the caller is a session. */
  callerKind: CallPayload["caller"]["kind"];
  childSessionId: string | null;
  depth: number;
  expectDigest: string | null;
  /** Admission budget from the record — lives on the contract, not the result. */
  resultBudgetBytes: number;
  resultOverflow: string;
  idleTtl: CallIdleTtl;
  /** Present only when someone opted into a deadline. */
  deadlineAt: string | null;
  settledAt: string | null;
  createdAt: string;
  timeline: CallTimelineEvent[];
  result: CallResultView;
  controls: CallDetailControls;
  /**
   * The ask, bounded by the daemon.
   *
   * `prompt_bytes` is the real size; the preview may be shorter. When it is, the
   * pane says so and offers the whole thing rather than silently truncating.
   */
  prompt: { preview: string | null; bytes: number; bounded: boolean } | null;
  /**
   * A late answer that arrived after the call had already settled.
   *
   * Kept as evidence, never as a state change — a result landing after a cancel
   * does not reopen the call, and this is the only place it is ever shown.
   */
  superseded: { preview: unknown; bytes: number } | null;
  /** Owner label for aggregate reads; the row's own profile, verbatim. */
  profileName: string;
  /** `Global` calls carry no workspace — never fabricate one. */
  workspaceId: string | null;
}

/**
 * The result pane has three honest shapes, and "empty" is not one of them.
 */
export type CallResultView =
  /** A typed result was admitted. */
  | {
      kind: "value";
      shape: CallResultShape;
      bytes: number | null;
      budgetBytes: number;
      overflow: string;
      /** The preview is smaller than what is stored. */
      bounded: boolean;
    }
  /**
   * A result exists and is stored, but nothing of it was inlined.
   *
   * Distinct from `none` on purpose: a 800 KB payload the daemon chose not to
   * preview is not an absent answer, and calling it one would tell the operator
   * their agent produced nothing when it produced a great deal.
   */
  | { kind: "stored"; bytes: number; budgetBytes: number; overflow: string }
  /** The child finished and recorded nothing. Its transcript is still intact. */
  | { kind: "none"; strict: boolean; prosePreview: string | null }
  /** The contract was not satisfied. Attempt evidence lives beside this. */
  | {
      kind: "invalid";
      repairAttempts: number;
      firstIssueText: string | null;
      secondIssueText: string | null;
    }
  /** Nothing has settled yet. */
  | { kind: "pending" };

function buildResultView(call: CallPayload, state: CallState | null): CallResultView {
  if (state !== null && !isTerminalCallState(state)) return { kind: "pending" };
  if (state === "invalid-result") {
    return {
      kind: "invalid",
      repairAttempts: call.repair_attempts,
      // Both tries reach the wire as their own fields. `failure_detail` repeats
      // the second one, so it is not read here — one source per fact.
      firstIssueText: call.first_issue_text ?? null,
      secondIssueText: call.second_issue_text ?? call.failure_detail ?? null,
    };
  }
  if (state === "completed-without-result") {
    return {
      kind: "none",
      strict: call.strict,
      prosePreview: call.final_prose_preview ?? null,
    };
  }
  const bytes = call.result_bytes ?? 0;
  const shape = buildCallResultShape(call.result_preview);
  if (shape.kind === "absent") {
    // Size decides, not the preview. A stored payload with no inline preview
    // reads as `stored`; only a genuinely empty one reads as `none`.
    if (bytes > 0) {
      return {
        kind: "stored",
        bytes,
        budgetBytes: call.result_budget_bytes,
        overflow: call.result_overflow,
      };
    }
    return { kind: "none", strict: call.strict, prosePreview: call.final_prose_preview ?? null };
  }
  return {
    kind: "value",
    shape,
    bytes: call.result_bytes ?? null,
    budgetBytes: call.result_budget_bytes,
    overflow: call.result_overflow,
    // Bounded means "what you see is less than what is stored" — a size fact,
    // with the row-shape heuristics as the fallback when no size was reported.
    bounded:
      (bytes > 0 && previewBytes(call.result_preview) < bytes) ||
      (shape.kind === "rows" && (shape.truncated || shape.rows.some(row => row.summary))),
  };
}

/**
 * How much of a payload we actually hold, in the units the daemon counts.
 *
 * `result_bytes` and `prompt_bytes` are Go byte lengths. A JavaScript string's
 * `.length` counts UTF-16 code units, so comparing the two calls a fully inlined
 * `"Revisão"` bounded (7 vs 8) and `"レビュー"` badly bounded (4 vs 12) — a
 * "there is more" notice on text that is already complete.
 */
function previewBytes(preview: unknown): number {
  if (preview === undefined || preview === null) return 0;
  const text = typeof preview === "string" ? preview : JSON.stringify(preview);
  return new TextEncoder().encode(text ?? "").length;
}

/**
 * A child is addressable unless the runtime says it is gone.
 *
 * `expired` is the one state that means the target itself left; every other
 * terminal leaves a parked child that a call or a message revives. There is no
 * Revive control anywhere precisely because these two affordances *are* it.
 */
function childAddressable(call: CallPayload, state: CallState | null): boolean {
  if (!call.child_session_id) return false;
  return state !== "expired";
}

function buildControls(
  call: CallPayload,
  state: CallState | null,
  counterpartExists: boolean
): CallDetailControls {
  const terminal = state === null ? false : isTerminalCallState(state);
  return {
    cancel: state !== null && !terminal,
    callAgain: terminal && Boolean(call.agent),
    messageChild: childAddressable(call, state),
    openChildSession: Boolean(call.child_session_id) && counterpartExists,
  };
}

function buildIdleTtl(call: CallPayload, state: CallState | null): CallIdleTtl {
  // A call in flight suspends the clock outright, so there is nothing to count.
  if (state !== null && !isTerminalCallState(state)) return { kind: "suspended" };
  if (call.idle_expires_at === null) return { kind: "none" };
  return { kind: "counting", expiresAt: call.idle_expires_at };
}

/**
 * The ask, and whether what we hold is all of it.
 *
 * A prompt with no preview but a non-zero size is not "no prompt" — it is a
 * prompt the daemon chose not to inline. The block still renders, offering the
 * fetch, rather than pretending the call carried no instruction.
 */
function buildPromptView(call: CallPayload): CallDetailView["prompt"] {
  if (call.prompt_bytes === 0 && !call.prompt_preview) return null;
  const preview = call.prompt_preview ?? null;
  return {
    preview,
    bytes: call.prompt_bytes,
    bounded: preview === null || previewBytes(preview) < call.prompt_bytes,
  };
}

/** Late evidence exists only when the daemon actually preserved some. */
function buildSupersededView(call: CallPayload): CallDetailView["superseded"] {
  if (call.superseded_bytes === 0 && call.superseded_preview === undefined) return null;
  return { preview: call.superseded_preview, bytes: call.superseded_bytes };
}

export function buildCallDetailView(input: CallDetailInput): CallDetailView {
  const { call } = input;
  const state = toCallState(call.state);
  const counterpartExists = input.counterpartExists ?? true;
  return {
    callId: call.call_id,
    agentName: call.agent ?? null,
    state,
    stateLabel: call.state,
    verdict: toCallVerdict(call.verdict),
    terminal: state === null ? false : isTerminalCallState(state),
    callerId: call.caller.id,
    callerKind: call.caller.kind,
    childSessionId: call.child_session_id ?? null,
    depth: call.depth,
    expectDigest: call.expect_digest ?? null,
    resultBudgetBytes: call.result_budget_bytes,
    resultOverflow: call.result_overflow,
    idleTtl: buildIdleTtl(call, state),
    deadlineAt: call.deadline_at ?? null,
    settledAt: call.settled_at ?? null,
    createdAt: call.created_at,
    timeline: buildCallTimeline(call),
    result: buildResultView(call, state),
    controls: buildControls(call, state, counterpartExists),
    prompt: buildPromptView(call),
    superseded: buildSupersededView(call),
    profileName: call.profile_name,
    workspaceId: call.workspace_id ?? null,
  };
}
