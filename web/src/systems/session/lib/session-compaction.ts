import { SessionApiError } from "../adapters/session-api-errors";
import type { SessionPayload } from "../types";
import { isSessionRunning } from "./session-running";

export type SessionCompactionCommand = "compact" | "compress";

/** Daemon refusals the rail explains in the daemon's own words. */
const COMPACTION_REFUSAL_CODES: ReadonlySet<string> = new Set([
  "session_busy",
  "compaction_unsupported",
]);

export const COMPACTION_FAILURE_MESSAGE = "Couldn't request compaction. Try again.";

/**
 * Selects the agent's compaction command the way the daemon does
 * (`session.ResolveCompactionCommand`): a name is the exact advertised name
 * after trimming, and `compact` wins over `compress`. The request is refused
 * as `compaction_unsupported` for any other spelling, so the action is not
 * offered for one.
 */
export function resolveCompactionCommand(
  commands: readonly { name: string }[]
): SessionCompactionCommand | null {
  let compress = false;
  for (const command of commands) {
    const name = command.name.trim();
    if (name === "compact") return "compact";
    if (name === "compress") compress = true;
  }
  return compress ? "compress" : null;
}

/**
 * Whether a compaction request can be admitted now: the agent advertises a
 * compaction command, the session is active, and no turn is running. The
 * daemon still owns the verdict (`session_busy`); this keeps the control
 * honest about requests it would certainly refuse.
 */
export function canCompactNow(session: SessionPayload): boolean {
  return (
    resolveCompactionCommand(session.available_commands) !== null &&
    session.state === "active" &&
    !isSessionRunning(session)
  );
}

/** The inline sentence for a failed request: the daemon's message for a refusal, otherwise a generic line. */
export function describeCompactionFailure(error: unknown): string {
  if (
    error instanceof SessionApiError &&
    error.code !== null &&
    COMPACTION_REFUSAL_CODES.has(error.code)
  ) {
    return error.message;
  }
  return COMPACTION_FAILURE_MESSAGE;
}
