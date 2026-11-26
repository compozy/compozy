import { sessionStore } from "../stores/session-store";
import { toast } from "@compozy/ui";

const MAX_REFUSAL_REASON_LENGTH = 400;

/**
 * The daemon refused the prompt before accepting it because the coding agent could not start with
 * the session's new MCP servers. Nothing was recorded, so the draft comes back and sending it
 * again retries the reload; there is no separate retry action.
 */
export const MCP_RELOAD_FAILED_CODE = "session_mcp_reload_failed";
export const SESSION_REBIND_TIMEOUT_CODE = "session_rebind_timeout";

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
      if (typeof reason === "string" && reason.trim().length > 0) {
        if ("code" in payload && payload.code === MCP_RELOAD_FAILED_CODE) {
          return codedRefusalReason(reason.trim(), MCP_RELOAD_FAILED_CODE);
        }
        if ("code" in payload && payload.code === SESSION_REBIND_TIMEOUT_CODE) {
          return codedRefusalReason(reason.trim(), SESSION_REBIND_TIMEOUT_CODE);
        }
        return readableReason(reason.trim());
      }
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
  sessionStore.trigger.composerPromptRefusalDismissed({ sessionId });
}

/** Caps daemon refusal text before presenting it in a notification or chat error. */
function bounded(reason: string): string {
  if (reason.length <= MAX_REFUSAL_REASON_LENGTH) return reason;
  return `${reason.slice(0, MAX_REFUSAL_REASON_LENGTH - 1)}…`;
}

// The daemon's sentence ends by saying the message wasn't sent, then names its error code in
// parentheses. The notice says "not sent" once and never shows the code (it is in `code`).
function codedRefusalReason(reason: string, code: string): string {
  return bounded(
    reason
      .replace(new RegExp(`\\s*\\(${code}\\)\\.?$`), "")
      .replace(/\.?\s*Your message wasn't sent\.?$/i, "")
  );
}

/** Converts missing attachment diagnostics into recovery guidance for a retained draft. */
function readableReason(reason: string): string {
  if (reason.includes("open prompt attachment") && reason.includes("attachment not found")) {
    return "An attachment is no longer available. Send again to upload the retained file";
  }
  return bounded(reason);
}
