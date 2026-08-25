/**
 * Typed failures for the calls and mailbox routes.
 *
 * Every calls/messages route answers one envelope — `{error, code, details?,
 * available?, widening?, original_id?}` — and the `code` is deterministic across
 * tool, CLI, HTTP, and UDS (`_dx.md` Errors). Callers branch on the code, never
 * on message text, and the UI renders the code verbatim as a mono suffix after a
 * plain-language line that names the recovery.
 */
import type { CallErrorRosterEntry } from "../types";

/** The `_dx.md` codes this surface can act on. Others pass through untyped. */
export const AGENT_COMMS_ERROR_CODES = [
  "call_agent_unknown",
  "call_prompt_empty",
  "call_expect_invalid",
  "call_deadline_invalid",
  "call_target_expired",
  "call_target_not_found",
  "call_target_denied",
  "call_workspace_denied",
  "call_children_cap",
  "call_depth_exceeded",
  "call_widening_rejected",
  "call_idempotency_conflict",
  "call_batch_empty",
  "call_batch_over_cap",
  "call_not_settled",
  "call_already_settled",
  "call_publish_not_settled",
  "call_publish_no_participation",
  "message_rate_limited",
  "message_duplicate",
  "message_too_large",
  "message_target_blocked",
  "message_target_denied",
  "message_pending_cap",
  "message_not_found",
] as const;

export type AgentCommsErrorCode = (typeof AGENT_COMMS_ERROR_CODES)[number];

const ERROR_CODE_MEMBERSHIP = Object.fromEntries(
  AGENT_COMMS_ERROR_CODES.map(code => [code, true])
) as Record<AgentCommsErrorCode, true>;

export function isAgentCommsErrorCode(value: string): value is AgentCommsErrorCode {
  return Object.hasOwn(ERROR_CODE_MEMBERSHIP, value);
}

export interface AgentCommsErrorDetails {
  /** The daemon's deterministic code, when it sent one. */
  code: string | null;
  /** Free-form key/value context — e.g. `reset_at`, `expired_at`, `suggestion`. */
  details: Readonly<Record<string, string>>;
  /** Roster attached to `call_agent_unknown`, so the refusal can name alternatives. */
  available: readonly CallErrorRosterEntry[];
  /** Widening atoms named by `call_widening_rejected`. */
  widening: readonly string[];
  /** The prior call id on `call_idempotency_conflict`. */
  originalId: string | null;
}

const EMPTY_DETAILS: AgentCommsErrorDetails = Object.freeze({
  code: null,
  details: Object.freeze({}),
  available: Object.freeze([]),
  widening: Object.freeze([]),
  originalId: null,
});

export class AgentCommsApiError extends Error {
  readonly status: number;
  readonly code: string | null;
  readonly details: Readonly<Record<string, string>>;
  readonly available: readonly CallErrorRosterEntry[];
  readonly widening: readonly string[];
  readonly originalId: string | null;

  constructor(message: string, status: number, payload: AgentCommsErrorDetails = EMPTY_DETAILS) {
    super(message);
    this.name = "AgentCommsApiError";
    this.status = status;
    this.code = payload.code;
    this.details = payload.details;
    this.available = payload.available;
    this.widening = payload.widening;
    this.originalId = payload.originalId;
  }

  /** True when the daemon named this exact condition. */
  hasCode(code: AgentCommsErrorCode): boolean {
    return this.code === code;
  }
}

export function isAgentCommsApiError(error: unknown): error is AgentCommsApiError {
  return error instanceof AgentCommsApiError;
}

/** True when the failure is one the operator can fix by acting differently. */
export function agentCommsErrorCode(error: unknown): AgentCommsErrorCode | null {
  if (!isAgentCommsApiError(error) || error.code === null) return null;
  return isAgentCommsErrorCode(error.code) ? error.code : null;
}

function readString(source: Record<string, unknown>, key: string): string | null {
  const value = source[key];
  return typeof value === "string" && value.length > 0 ? value : null;
}

function readStringMap(source: Record<string, unknown>, key: string): Record<string, string> {
  const value = source[key];
  if (typeof value !== "object" || value === null) return {};
  const entries = Object.entries(value).filter(
    (entry): entry is [string, string] => typeof entry[1] === "string"
  );
  return Object.fromEntries(entries);
}

function readStringArray(source: Record<string, unknown>, key: string): string[] {
  const value = source[key];
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string")
    : [];
}

function readRoster(source: Record<string, unknown>): CallErrorRosterEntry[] {
  const value = source.available;
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is CallErrorRosterEntry => {
    if (typeof item !== "object" || item === null) return false;
    return typeof (item as { name?: unknown }).name === "string";
  });
}

/**
 * Pull the typed envelope off an openapi-fetch `error`.
 *
 * The generated types union every documented status, so the envelope arrives as
 * an unknown-shaped object; this reads it defensively and never throws while
 * building an error.
 */
export function readAgentCommsError(error: unknown): AgentCommsErrorDetails {
  if (typeof error !== "object" || error === null) return EMPTY_DETAILS;
  const source = error as Record<string, unknown>;
  return {
    code: readString(source, "code"),
    details: readStringMap(source, "details"),
    available: readRoster(source),
    widening: readStringArray(source, "widening"),
    originalId: readString(source, "original_id"),
  };
}

/**
 * Compose the thrown message: the daemon's own sentence when it sent one, else
 * the caller's fallback with the status. The code travels on the error object,
 * not spliced into prose — surfaces render it as its own mono element.
 */
export function agentCommsErrorMessage(
  fallback: string,
  response: Response,
  error: unknown
): string {
  if (typeof error === "object" && error !== null) {
    const detail = readString(error as Record<string, unknown>, "error");
    if (detail) return detail;
  }
  return `${fallback}: ${response.status}`;
}

/** The one construction site — keeps message, status, and envelope in step. */
export function toAgentCommsApiError(
  fallback: string,
  response: Response,
  error: unknown
): AgentCommsApiError {
  return new AgentCommsApiError(
    agentCommsErrorMessage(fallback, response, error),
    response.status,
    readAgentCommsError(error)
  );
}
