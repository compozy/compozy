import type { SyncResult } from "../types";

export function formatWorkflowSyncResult(result: SyncResult): string {
  const scanned = result.workflows_scanned ?? 0;
  const pruned = result.workflows_pruned ?? 0;
  const scannedLabel = `${scanned} workflow${scanned === 1 ? "" : "s"} scanned`;
  if (pruned <= 0) {
    return `Sync completed - ${scannedLabel}.`;
  }
  return `Sync completed - ${scannedLabel}, ${pruned} stale workflow${
    pruned === 1 ? "" : "s"
  } removed.`;
}
