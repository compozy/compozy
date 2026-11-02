import { toast } from "@compozy/ui";

const MAX_REFUSAL_REASON_LENGTH = 400;

/**
 * The daemon's reason for refusing a prompt, read from the body of its error
 * response (the AI SDK transport surfaces that body as the error message).
 */
export function sessionPromptRefusalReason(body: string): string | null {
  const trimmed = body.trim();
  if (trimmed.length === 0) return null;
  try {
    const payload: unknown = JSON.parse(trimmed);
    if (typeof payload === "object" && payload !== null && "error" in payload) {
      const reason = payload.error;
      if (typeof reason === "string" && reason.trim().length > 0)
        return readableReason(reason.trim());
    }
    return null;
  } catch {
    return readableReason(trimmed);
  }
}

/** Tells the operator a prompt never became a turn; the draft itself is restored separately. */
export function notifySessionPromptNotSent(reason: string | null, sessionId: string): void {
  toast.error(
    reason
      ? `Message not sent: ${reason}. Your draft was kept.`
      : "Couldn't confirm the message was sent. Your draft was kept.",
    { id: `session-prompt-refusal-${sessionId}` }
  );
}

/** Clears only this session's obsolete refusal after a prompt is accepted. */
export function dismissSessionPromptNotSent(sessionId: string): void {
  toast.dismiss(`session-prompt-refusal-${sessionId}`);
}

/** Caps daemon refusal text before presenting it in a notification or chat error. */
function bounded(reason: string): string {
  if (reason.length <= MAX_REFUSAL_REASON_LENGTH) return reason;
  return `${reason.slice(0, MAX_REFUSAL_REASON_LENGTH - 1)}…`;
}

/** Converts missing attachment diagnostics into recovery guidance for a retained draft. */
function readableReason(reason: string): string {
  if (reason.includes("open prompt attachment") && reason.includes("attachment not found")) {
    return "An attachment is no longer available. Send again to upload the retained file";
  }
  return bounded(reason);
}
