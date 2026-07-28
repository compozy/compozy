// Shared locale-aware timestamp formatting for session surfaces (the message
// hover toolbar and the inspector trace both render turn times). Both helpers
// guard against non-finite / non-positive input so a message with no derivable
// timestamp renders nothing rather than "Invalid Date".

/** Compact `HH:MM` label rendered inline next to the copy affordance. */
export function formatMessageTimestamp(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "";
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

/** Fuller `Mon D, YYYY, HH:MM:SS` form surfaced through the hover `title`. */
export function formatMessageTimestampFull(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "";
  return new Date(ms).toLocaleString([], {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
