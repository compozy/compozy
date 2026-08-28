/**
 * Clock copy that belongs on the call record, not on `<Time>`'s defaults.
 *
 * Timeline hours are UTC `HH:MM:SS` so the rail matches the daemon's ISO
 * instants. Idle TTL states when the session-owned clock is suspended. Settled
 * duration is `settled − created`.
 */
import { formatDuration } from "@compozy/ui";

import type { CallIdleTtl } from "./call-detail-view-model";

export function formatUtcClock(iso: string): string {
  const parsed = Date.parse(iso);
  if (!Number.isFinite(parsed)) return "—";
  return new Date(parsed).toISOString().slice(11, 19);
}

export function formatIdleTtlCopy(ttl: CallIdleTtl): string | null {
  if (ttl.kind === "suspended") return "suspended while running";
  return null;
}

export function formatSettledDuration(createdAt: string, settledAt: string): string | null {
  const created = Date.parse(createdAt);
  const settled = Date.parse(settledAt);
  if (!Number.isFinite(created) || !Number.isFinite(settled) || settled <= created) {
    return null;
  }
  return formatDuration(settled - created);
}
