/** Format the daemon's byte counts without changing their unit semantics. */
export function formatAgentCallBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kib = bytes / 1024;
  return kib < 1024 ? `${kib.toFixed(kib < 10 ? 1 : 0)} KiB` : `${(kib / 1024).toFixed(1)} MiB`;
}
