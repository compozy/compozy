/** Compact context counts shared by the composer, inspector, and turn history. */
export function formatContextTokens(value: number): string {
  if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(1))}M`;
  if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}K`;
  return value.toLocaleString();
}

export function formatContextPercent(ratio: number): string {
  const percent = ratio * 100;
  return `${percent < 10 ? Number(percent.toFixed(1)) : Math.round(percent)}%`;
}

export function formatContextBytes(bytes: number): string {
  if (bytes >= 1_048_576) return `${Number((bytes / 1_048_576).toFixed(1))} MiB`;
  if (bytes >= 1_024) return `${Number((bytes / 1_024).toFixed(1))} KiB`;
  return `${bytes.toLocaleString()} B`;
}

/** Only strip the conventional turn prefix; opaque ids keep their identity. */
export function formatContextTurn(id: string): string {
  return id.replace(/^turn[-_]/, "");
}

/** Wall-clock `HH:MM:SS` of a report; empty when the timestamp is unusable. */
export function formatContextClock(iso: string): string {
  const ms = Date.parse(iso);
  if (!Number.isFinite(ms)) return "";
  return new Date(ms).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  });
}
