/**
 * The call's state timeline: what happened, when, in the runtime's own terms.
 *
 * Every event here is anchored to a durable timestamp or a durable counter on
 * the record. Nothing is inferred from elapsed time and nothing is invented to
 * fill a gap — a call with no `started_at` simply has no Running event, because
 * the daemon never recorded one.
 *
 * The `queued` moment shares `created_at`: admission and enqueue are one act, so
 * showing two timestamps would imply a delay the record does not describe.
 */
import {
  Check,
  Circle,
  Clock,
  FileX,
  Plus,
  RotateCcw,
  Sparkles,
  type LucideIcon,
} from "lucide-react";

import { CALL_VERDICT_SIGNAL, toCallState, toCallVerdict } from "./call-state";
import type { CallPayload } from "../types";

export type CallTimelineTone = "neutral" | "success" | "warning" | "danger";

export interface CallTimelineEvent {
  id: string;
  /** Plain-language headline. */
  title: string;
  /** One clause of supporting fact, or null when the title says it all. */
  detail: string | null;
  /** ISO timestamp straight off the record. Formatting belongs to the view. */
  at: string;
  tone: CallTimelineTone;
  glyph: LucideIcon;
}

const SETTLED_TONE: Record<string, CallTimelineTone> = {
  completed: "success",
  "invalid-result": "danger",
  "completed-without-result": "danger",
  failed: "danger",
  expired: "danger",
  canceled: "warning",
  timeout: "warning",
};

const SETTLED_GLYPH: Record<string, LucideIcon> = {
  completed: Check,
  "invalid-result": FileX,
};

/**
 * Why the call ended, in plain words.
 *
 * For a completed call the verdict dictionary already says how the answer
 * arrived; for everything else the daemon's own `failure_detail` is quoted
 * rather than paraphrased. Validator output arrives already sanitized — secret
 * shaped values are hash-redacted before validation runs — so quoting it is safe.
 *
 * `invalid-result` is the exception: the attempts panel owns that text, verbatim
 * and per try. Repeating it here would print the same sentence twice on one
 * screen and split ownership of the evidence between two panes.
 */
function settledDetail(call: CallPayload): string | null {
  const verdict = toCallVerdict(call.verdict);
  if (verdict !== null) return CALL_VERDICT_SIGNAL[verdict].description;
  if (toCallState(call.state) === "invalid-result") {
    const tries = call.repair_attempts + 1;
    return `the answer did not match after ${tries} ${tries === 1 ? "try" : "tries"}`;
  }
  return call.failure_detail ?? null;
}

export function buildCallTimeline(call: CallPayload): CallTimelineEvent[] {
  const events: CallTimelineEvent[] = [
    {
      id: "created",
      title: "Created",
      detail: call.caller.id ? `by ${call.caller.id}` : null,
      at: call.created_at,
      tone: "neutral",
      glyph: Plus,
    },
    {
      id: "queued",
      title: "Queued",
      detail: null,
      at: call.created_at,
      tone: "neutral",
      glyph: Clock,
    },
  ];

  if (call.started_at) {
    events.push({
      id: "running",
      title: "Running",
      detail: call.child_session_id ? `child ${call.child_session_id}` : null,
      at: call.started_at,
      tone: "neutral",
      glyph: Circle,
    });
  }

  // One repair round is the whole budget: the first answer was handed back with
  // its errors, and the second is final either way.
  if (call.repair_attempts > 0) {
    events.push({
      id: "repair",
      title: `Retry ${call.repair_attempts} of ${call.repair_attempts}`,
      detail: "first answer rejected — the errors were handed back to the helper",
      at: call.started_at ?? call.created_at,
      tone: "warning",
      glyph: RotateCcw,
    });
  }

  // Extraction is admission truth, so it earns its own moment rather than
  // hiding inside the settled line.
  if (toCallVerdict(call.verdict) === "extracted") {
    events.push({
      id: "extracted",
      title: "The helper finished without sending its answer",
      detail: "the answer was recovered from its last message and checked",
      at: call.settled_at ?? call.updated_at,
      tone: "neutral",
      glyph: Sparkles,
    });
  }

  if (call.settled_at) {
    const state = toCallState(call.state);
    events.push({
      id: "settled",
      title: state === null ? "Settled" : `Settled — ${state}`,
      detail: settledDetail(call),
      at: call.settled_at,
      tone: (state && SETTLED_TONE[state]) ?? "neutral",
      glyph: (state && SETTLED_GLYPH[state]) ?? Check,
    });
  }

  return events;
}
