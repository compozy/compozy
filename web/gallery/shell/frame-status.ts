/**
 * Ready/error handshake between a standalone prototype (`/<slug>`, `main.tsx`) and the gallery
 * frame that embeds it. The prototype posts to its parent with its own origin as `targetOrigin`;
 * the host trusts a message only from the same origin and from its own iframe's window.
 */
export const frameStatusType = "compozy:prototype-status";

export type FrameStatus =
  | { type: typeof frameStatusType; status: "ready"; slug: string; state: string }
  | { type: typeof frameStatusType; status: "error"; slug: string; message: string };

/** A preview that reports nothing in this time counts as failed ("Prototype did not load"). */
export const previewTimeoutMs = 10_000;

export type PreviewStatus = "loading" | "ready" | "error";

/** The status a message carries, or `null` when the message is not from this frame. */
export function acceptFrameStatus(
  event: Pick<MessageEvent, "data" | "origin" | "source">,
  frame: Window | null,
  origin: string
): FrameStatus["status"] | null {
  if (!frame || event.source !== frame || event.origin !== origin) return null;
  const data: unknown = event.data;
  if (!data || typeof data !== "object") return null;
  const { type, status } = data as { type?: unknown; status?: unknown };
  if (type !== frameStatusType) return null;
  return status === "ready" || status === "error" ? status : null;
}

/**
 * Preview status after an accepted message or the timeout. An error is final until the preview
 * remounts, even after "ready" (a later render error must not stay hidden); the timeout only
 * fails a preview that never reported "ready".
 */
export function nextPreviewStatus(
  current: PreviewStatus,
  event: FrameStatus["status"] | "timeout"
): PreviewStatus {
  if (current === "error") return current;
  if (event === "timeout") return current === "loading" ? "error" : current;
  return event;
}

/** Posts the status to the gallery frame embedding this page (no-op when top level). */
export function postFrameStatus(status: FrameStatus) {
  if (window.parent === window) return;
  window.parent.postMessage(status, window.location.origin);
}
