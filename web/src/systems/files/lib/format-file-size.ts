const UNITS = ["KB", "MB", "GB"] as const;

/** `2.3 MB`, `1 MB`, `512 B`: one decimal place, dropped when it is zero. */
export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 1024) return `${Math.max(0, Math.round(bytes || 0))} B`;
  let value = bytes;
  let unit: (typeof UNITS)[number] = "KB";
  for (const next of UNITS) {
    value /= 1024;
    unit = next;
    if (value < 1024) break;
  }
  const rounded = Math.round(value * 10) / 10;
  return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)} ${unit}`;
}

/** The text read cap (Business Rule 5); the daemon renders the first 1 MiB of larger files. */
export const FILE_TEXT_LIMIT = 1 << 20;
