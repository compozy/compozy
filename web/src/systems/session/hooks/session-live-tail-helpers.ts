import type { SessionEventPayload, SessionMessage } from "../types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * True when any applied transcript entry carries a `clarify` `data-agh-event` part. Used as the wake
 * signal to invalidate the live clarifications projection — the transcript event is evidence only;
 * pending truth is always reread from the exact-authority GET.
 */
export function entriesContainClarifyEvent(entries: readonly unknown[] | undefined): boolean {
  if (!entries) {
    return false;
  }
  for (const entry of entries) {
    if (!isRecord(entry) || !isRecord(entry.message) || !Array.isArray(entry.message.parts)) {
      continue;
    }
    for (const part of entry.message.parts) {
      if (
        isRecord(part) &&
        part.type === "data-agh-event" &&
        isRecord(part.data) &&
        part.data.type === "clarify"
      ) {
        return true;
      }
    }
  }
  return false;
}

export function numberFromEventID(value: unknown): number | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (trimmed.length === 0) {
    return null;
  }
  const parsed = Number.parseInt(trimmed, 10);
  return Number.isFinite(parsed) ? parsed : null;
}

export function parseSessionStreamPayload<T>(event: MessageEvent): T | null {
  if (typeof event.data !== "string" || event.data.trim().length === 0) {
    return null;
  }
  try {
    return JSON.parse(event.data) as T;
  } catch {
    return null;
  }
}

function terminalFailureText(payload: SessionEventPayload): string | null {
  const failureSummary = payload.failure?.summary?.trim();
  if (failureSummary) {
    return failureSummary;
  }
  const stopDetail = payload.stop_detail?.trim();
  return stopDetail && stopDetail.length > 0 ? stopDetail : null;
}

export function terminalFailureMessage(
  payload: SessionEventPayload,
  fallbackSessionId: string
): SessionMessage | null {
  const detail = terminalFailureText(payload);
  if (!detail) {
    return null;
  }
  const sessionID = payload.session_id || fallbackSessionId;
  return {
    id: `session-stopped-${sessionID}`,
    role: "assistant",
    parts: [
      {
        type: "data-agh-event",
        data: {
          type: "error",
          session_id: sessionID,
          timestamp: payload.timestamp,
          stop_reason: payload.stop_reason,
          error: detail,
          failure: payload.failure ?? undefined,
        },
      },
    ],
  } as SessionMessage;
}
