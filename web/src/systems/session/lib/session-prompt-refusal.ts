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

/** Reads the visible reason and identifies a rebind failure that preceded prompt admission. */
export function parseSessionPromptRefusal(
  status: number,
  body: string
): { reason: string | null; rebindTimeout: boolean } {
  const trimmed = body.trim();
  if (trimmed.length === 0) return { reason: null, rebindTimeout: false };
  try {
    const payload: unknown = JSON.parse(trimmed);
    if (typeof payload !== "object" || payload === null)
      return { reason: null, rebindTimeout: false };
    const code = "code" in payload ? payload.code : null;
    const rebindTimeout =
      status === 504 &&
      code === SESSION_REBIND_TIMEOUT_CODE &&
      "retryable" in payload &&
      payload.retryable === true;
    const reason =
      "error" in payload && typeof payload.error === "string" ? payload.error.trim() : "";
    if (!reason) return { reason: null, rebindTimeout };
    if (code === MCP_RELOAD_FAILED_CODE || code === SESSION_REBIND_TIMEOUT_CODE) {
      return { reason: codedRefusalReason(reason, code), rebindTimeout };
    }
    return { reason: readableReason(reason), rebindTimeout };
  } catch {
    return { reason: readableReason(trimmed), rebindTimeout: false };
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
