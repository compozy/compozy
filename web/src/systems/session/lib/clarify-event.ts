import { z } from "zod";

import type {
  AgentEventPayload,
  ClarifyEventAnswer,
  ClarifyEventView,
  ClarifyStatus,
} from "../types";

/** The durable `clarify` transcript event type discriminator. */
export const CLARIFY_EVENT_TYPE = "clarify";

const clarifyStatusSchema = z.enum(["pending", "resolved", "timed_out", "canceled"]);

const clarifyAnswerSchema = z.object({
  choice: z.number().nullable().optional(),
  text: z.string().optional(),
  fallback: z.boolean().optional(),
});

const clarifyRequestSchema = z.object({
  request_id: z.string().min(1),
  workspace_id: z.string().optional(),
  session_id: z.string().optional(),
  agent_name: z.string().optional(),
  question: z.string(),
  choices: z.array(z.string()).optional(),
  asked_at: z.string().optional(),
  deadline: z.string().optional(),
});

/** Canonical `ClarifyEvent` wire shape carried by the `data-compozy-event` clarify payload. */
const clarifyEventSchema = z.object({
  status: clarifyStatusSchema,
  request: clarifyRequestSchema,
  answer: clarifyAnswerSchema.nullish(),
  at: z.string().optional(),
});

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** A `data-compozy-event` payload is a clarification lifecycle event. */
export function isClarifyEventData(value: unknown): value is AgentEventPayload {
  return isRecord(value) && value.type === CLARIFY_EVENT_TYPE;
}

function normalizeAnswer(
  status: ClarifyStatus,
  answer: z.infer<typeof clarifyAnswerSchema> | null | undefined
): ClarifyEventAnswer | null {
  if (status === "pending" || status === "canceled" || !answer) {
    return null;
  }
  return {
    choice: answer.choice ?? null,
    text: answer.text ?? "",
    fallback: answer.fallback ?? false,
  };
}

/**
 * Parse the canonical clarify payload from a `data-compozy-event` part. The backend preserves the
 * `ClarifyEvent` under `data.raw`; we tolerate a top-level shape too. Returns `null` when the payload
 * is not (yet) a parseable clarify event so an unrenderable event degrades to nothing, never a crash.
 * Pending answer controls still read question/choices from the live GET; the durable question is
 * retained only for read-failure recovery, alongside the status, request id, and terminal evidence.
 */
export function parseClarifyEvent(data: AgentEventPayload): ClarifyEventView | null {
  for (const candidate of [data.raw, data]) {
    const parsed = clarifyEventSchema.safeParse(candidate);
    if (!parsed.success) {
      continue;
    }
    const { status, request, answer, at } = parsed.data;
    return {
      status,
      requestId: request.request_id,
      request: {
        request_id: request.request_id,
        workspace_id: request.workspace_id,
        session_id: request.session_id,
        agent_name: request.agent_name,
        question: request.question,
        choices: request.choices,
        asked_at: request.asked_at,
        deadline: request.deadline,
      },
      answer: normalizeAnswer(status, answer),
      at,
    };
  }
  return null;
}

/**
 * Static, non-ticking deadline hint (absolute local time). The broker enforces the timeout server
 * side and emits a `timed_out` event; the client never runs a countdown.
 */
export function formatClarifyDeadline(deadline: string | undefined): string | null {
  if (!deadline) {
    return null;
  }
  const time = new Date(deadline);
  if (Number.isNaN(time.getTime())) {
    return null;
  }
  return time.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

/**
 * Truthful terminal-answer label: the chosen option text for a resolved choice, the trimmed free
 * text for a resolved answer, or `null` when there is no real answer (timed out / canceled) — never
 * an invented answer.
 */
export function clarifyAnswerLabel(view: ClarifyEventView): string | null {
  const answer = view.answer;
  if (!answer) {
    return null;
  }
  if (answer.choice !== null) {
    const label = view.request.choices?.[answer.choice];
    if (label !== undefined) {
      return label;
    }
  }
  const text = answer.text.trim();
  return text === "" ? null : text;
}

interface MessageLike {
  content?: unknown;
}

function isClarifyPart(
  part: unknown
): part is Record<string, unknown> & { data: AgentEventPayload } {
  if (!isRecord(part)) return false;
  const isEventPart =
    part.type === "data-compozy-event" || (part.type === "data" && part.name === "compozy-event");
  return isEventPart && isClarifyEventData(part.data);
}

/**
 * Request ids of clarify asks the transcript still shows as pending, in first-seen order. A
 * later lifecycle event for the same request supersedes the earlier one, so a settled question
 * drops out. Pure derivation over the same message shape the decision dock reads.
 */
export function derivePendingClarifyRequestIds(
  messages: readonly MessageLike[]
): ReadonlySet<string> {
  const statusByRequest = new Map<string, ClarifyStatus>();
  for (const message of messages) {
    const content = Array.isArray(message.content) ? message.content : [];
    for (const part of content) {
      if (!isClarifyPart(part)) continue;
      const view = parseClarifyEvent(part.data);
      if (!view) continue;
      statusByRequest.set(view.requestId, view.status);
    }
  }
  const pending = new Set<string>();
  for (const [requestId, status] of statusByRequest) {
    if (status === "pending") pending.add(requestId);
  }
  return pending;
}
