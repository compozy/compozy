import { SessionApiError } from "../adapters/session-api-errors";

/**
 * Closed set of reasons a busy send does not go out. Daemon codes keep their
 * `_dx.md` spelling; the client-only reasons (`send_in_flight`, `not_delivered`)
 * name gates the browser applies before or instead of a daemon answer.
 */
export type SessionBusyInputRefusalCode =
  | "active_turn_mismatch"
  | "turn_ended"
  | "session_not_promptable"
  | "steer_attachments_unsupported"
  | "queue_full"
  | "send_conflict"
  | "send_in_flight"
  | "not_delivered";

export interface SessionBusyInputRefusal {
  code: SessionBusyInputRefusalCode;
  /** The daemon's own sentence when it sent one; used by `not_delivered`. */
  message: string | null;
  /** The turn the daemon reports as active after a strict-fence refusal. */
  currentTurnId: string | null;
  /** How many files the refused draft carried (steer refusals name them). */
  attachmentCount: number;
}

/** Thrown by busy-input gates and mapped from daemon refusals; the draft is never consumed. */
export class SessionBusyInputRefusalError extends Error {
  readonly refusal: SessionBusyInputRefusal;

  constructor(refusal: Partial<SessionBusyInputRefusal> & Pick<SessionBusyInputRefusal, "code">) {
    const resolved: SessionBusyInputRefusal = {
      attachmentCount: refusal.attachmentCount ?? 0,
      code: refusal.code,
      currentTurnId: refusal.currentTurnId ?? null,
      message: refusal.message ?? null,
    };
    super(describeSessionBusyInputRefusal(resolved));
    this.name = "SessionBusyInputRefusalError";
    this.refusal = resolved;
  }
}

const DAEMON_REFUSAL_CODES: ReadonlySet<string> = new Set([
  "active_turn_mismatch",
  "session_not_promptable",
  "steer_attachments_unsupported",
  "queue_full",
  "send_conflict",
]);

function isDaemonRefusalCode(code: string): code is SessionBusyInputRefusalCode {
  return DAEMON_REFUSAL_CODES.has(code);
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && "name" in error && error.name === "AbortError"
  );
}

/**
 * Classifies a failed busy send. Aborts (owner replaced) yield `null` — nothing
 * to report; every other failure becomes a refusal so the composer states the
 * reason instead of a silent no-op (US-004.AC-3).
 */
export function sessionBusyInputRefusalFromError(
  error: unknown,
  context: { attachmentCount?: number } = {}
): SessionBusyInputRefusal | null {
  if (isAbortError(error)) return null;
  if (error instanceof SessionBusyInputRefusalError) {
    return {
      ...error.refusal,
      attachmentCount: context.attachmentCount ?? error.refusal.attachmentCount,
    };
  }
  const attachmentCount = context.attachmentCount ?? 0;
  if (error instanceof SessionApiError && error.code !== null && isDaemonRefusalCode(error.code)) {
    const currentTurnId = error.currentTurnId;
    // A fence refusal with no live turn means the turn settled: send normally.
    const code: SessionBusyInputRefusalCode =
      error.code === "active_turn_mismatch" && currentTurnId === null ? "turn_ended" : error.code;
    return { attachmentCount, code, currentTurnId, message: error.message };
  }
  const message = error instanceof Error && error.message.trim().length > 0 ? error.message : null;
  return { attachmentCount, code: "not_delivered", currentTurnId: null, message };
}

/** The plain sentence the composer shows; the code rides beside it as mono. */
export function describeSessionBusyInputRefusal(refusal: SessionBusyInputRefusal): string {
  switch (refusal.code) {
    case "active_turn_mismatch":
      return "Not sent — the turn changed before this went out. Your draft is back.";
    case "turn_ended":
      return "Not sent — the turn ended. Send it normally.";
    case "session_not_promptable":
      return "Not sent — this session is stopped. Resume it or pick another.";
    case "steer_attachments_unsupported": {
      const files =
        refusal.attachmentCount === 1 ? "the file" : `the ${refusal.attachmentCount} files`;
      return `Not sent — steer can't carry files on this agent. Queue it, or remove ${files}.`;
    }
    case "queue_full":
      return "Not sent — the queue is full. Steer, interrupt, or clear the queue.";
    case "send_conflict":
      return "Not sent — this send identity was already used with different text.";
    case "send_in_flight":
      return "Not sent — another send is still in flight. Your draft is back.";
    case "not_delivered":
      return refusal.message
        ? `Not sent — ${refusal.message}`
        : "Not sent — CompozyOS didn't answer. Your draft is back.";
  }
}
