import type { ExtensionInstanceScope, ExtensionLogEntry, ExtensionLogsSnapshot } from "../types";

/** Bounded client-side mirror of the daemon's per-instance ring buffer. */
export const EXTENSION_LOG_BUFFER_LIMIT = 500;

export function buildExtensionLogsStreamUrl(
  name: string,
  options: ExtensionInstanceScope & { after?: number; streamEpoch?: string } = {}
): string {
  const params = new URLSearchParams({ follow: "1" });
  const workspace = options.workspaceId?.trim() ?? "";
  if (workspace !== "") params.set("workspace", workspace);
  if (options.after !== undefined && options.after > 0) params.set("after", String(options.after));
  const streamEpoch = options.streamEpoch?.trim() ?? "";
  if (streamEpoch !== "") params.set("stream_epoch", streamEpoch);
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
  if (limit <= 0) return [];
  const cursor = extensionLogCursor(current);
  const fresh: ExtensionLogEntry[] = [];
  let lastSequence = cursor;
  for (const entry of [...incoming].sort((left, right) => left.sequence - right.sequence)) {
    if (entry.sequence <= lastSequence) continue;
    fresh.push(entry);
    lastSequence = entry.sequence;
  }
  if (fresh.length === 0) {
    return current.length > limit
      ? current.slice(current.length - limit)
      : (current as ExtensionLogEntry[]);
  }
  const merged = [...current, ...fresh];
  return merged.length > limit ? merged.slice(merged.length - limit) : merged;
}

/** Normalize one complete daemon snapshot before it becomes canonical cache data. */
export function normalizeExtensionLogEntries(
  entries: readonly ExtensionLogEntry[],
  limit: number = EXTENSION_LOG_BUFFER_LIMIT
): ExtensionLogEntry[] {
  return appendExtensionLogEntries([], entries, limit);
}

export function normalizeExtensionLogSnapshot(
  snapshot: ExtensionLogsSnapshot
): ExtensionLogsSnapshot {
  return {
    logs: normalizeExtensionLogEntries(snapshot.logs),
    stream_epoch: snapshot.stream_epoch.trim(),
  };
}

export function appendExtensionLogEntry(
  snapshot: ExtensionLogsSnapshot,
  entry: ExtensionLogEntry
): ExtensionLogsSnapshot {
  if (entry.stream_epoch !== snapshot.stream_epoch) return snapshot;
  return {
    ...snapshot,
    logs: appendExtensionLogEntries(snapshot.logs, [entry]),
  };
}

export function extensionLogCursor(entries: readonly ExtensionLogEntry[]): number {
  return entries.length === 0 ? 0 : (entries[entries.length - 1]?.sequence ?? 0);
}

export function parseExtensionLogEvent(data: unknown): ExtensionLogEntry | null {
  if (typeof data !== "string") return null;
  try {
    const parsed = JSON.parse(data) as Partial<ExtensionLogEntry>;
    if (
      typeof parsed?.sequence !== "number" ||
      typeof parsed?.message !== "string" ||
      typeof parsed?.stream_epoch !== "string" ||
      parsed.stream_epoch.trim() === ""
    ) {
      return null;
    }
    return {
      generation_hash: parsed.generation_hash,
      message: parsed.message,
      sequence: parsed.sequence,
      stream_epoch: parsed.stream_epoch,
      timestamp: typeof parsed.timestamp === "string" ? parsed.timestamp : "",
    };
  } catch {
    return null;
  }
}

export function parseExtensionLogResetEvent(data: unknown): ExtensionLogsSnapshot | null {
  if (typeof data !== "string") return null;
  try {
    const parsed = JSON.parse(data) as Partial<ExtensionLogsSnapshot>;
    if (
      typeof parsed.stream_epoch !== "string" ||
      parsed.stream_epoch.trim() === "" ||
      !Array.isArray(parsed.logs)
    ) {
      return null;
    }
    const logs: ExtensionLogEntry[] = [];
    for (const entry of parsed.logs) {
      const normalized = parseExtensionLogEvent(JSON.stringify(entry));
      if (!normalized || normalized.stream_epoch !== parsed.stream_epoch) return null;
      logs.push(normalized);
    }
    return normalizeExtensionLogSnapshot({ logs, stream_epoch: parsed.stream_epoch });
  } catch {
    return null;
  }
}
