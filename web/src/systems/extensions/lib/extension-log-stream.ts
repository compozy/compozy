import type { ExtensionInstanceScope, ExtensionLogEntry } from "../types";

/** Bounded client-side mirror of the daemon's per-instance ring buffer. */
export const EXTENSION_LOG_BUFFER_LIMIT = 500;

export function buildExtensionLogsStreamUrl(
  name: string,
  options: ExtensionInstanceScope & { after?: number } = {}
): string {
  const params = new URLSearchParams({ follow: "1" });
  const workspace = options.workspaceId?.trim() ?? "";
  if (workspace !== "") params.set("workspace", workspace);
  if (options.after !== undefined && options.after > 0) params.set("after", String(options.after));
  return `/api/extensions/${encodeURIComponent(name)}/logs?${params.toString()}`;
}

/**
 * Sequence is the daemon's monotonic cursor. Appending only strictly-newer records makes replay
 * after a reconnect idempotent, so a reconnect never duplicates or reorders rendered lines.
 */
export function appendExtensionLogEntries(
  current: readonly ExtensionLogEntry[],
  incoming: readonly ExtensionLogEntry[],
  limit: number = EXTENSION_LOG_BUFFER_LIMIT
): ExtensionLogEntry[] {
  const cursor = extensionLogCursor(current);
  const fresh = incoming.filter(entry => entry.sequence > cursor);
  if (fresh.length === 0) return current as ExtensionLogEntry[];
  const merged = [...current, ...[...fresh].sort((left, right) => left.sequence - right.sequence)];
  return merged.length > limit ? merged.slice(merged.length - limit) : merged;
}

export function extensionLogCursor(entries: readonly ExtensionLogEntry[]): number {
  return entries.length === 0 ? 0 : (entries[entries.length - 1]?.sequence ?? 0);
}

export function parseExtensionLogEvent(data: unknown): ExtensionLogEntry | null {
  if (typeof data !== "string") return null;
  try {
    const parsed = JSON.parse(data) as Partial<ExtensionLogEntry>;
    if (typeof parsed?.sequence !== "number" || typeof parsed?.message !== "string") return null;
    return {
      generation_hash: parsed.generation_hash,
      message: parsed.message,
      sequence: parsed.sequence,
      timestamp: typeof parsed.timestamp === "string" ? parsed.timestamp : "",
    };
  } catch {
    return null;
  }
}
