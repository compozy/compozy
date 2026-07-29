// Pure per-turn aggregation of the files an assistant turn modified via Edit/Write
// tool calls, feeding the settled-turn "Changed files" roll-up. The per-call diff
// math lives in `@/systems/session/lib/tool-diff-stat`; this layer only folds
// calls into distinct first-touch-ordered entries: same file edited twice within
// a turn collapses to one entry with summed stats. Only settled, non-error
// Edit/Write calls count — a failed edit changed nothing.

import { fileDiffStatForTool } from "@/systems/session/lib/tool-diff-stat";
import type { ChangedFileEntry, SessionTimelineToolPart } from "./session-timeline.logic";

export const CHANGED_FILES_VISIBLE_CAP = 8;

export interface ChangedFilesSummary {
  files: ChangedFileEntry[];
  additions: number;
  deletions: number;
}

/**
 * Aggregate a turn's tool parts into the changed-files summary, or `null` when the
 * turn modified no files.
 */
export function aggregateChangedFiles(
  tools: readonly SessionTimelineToolPart[]
): ChangedFilesSummary | null {
  const byPath = new Map<string, ChangedFileEntry>();
  for (const tool of tools) {
    if (tool.isError || tool.status !== "settled") continue;
    const stat = fileDiffStatForTool(tool.toolName, tool.args);
    if (!stat) continue;
    const existing = byPath.get(stat.path);
    if (existing) {
      existing.additions += stat.additions;
      existing.deletions += stat.deletions;
    } else {
      byPath.set(stat.path, {
        path: stat.path,
        additions: stat.additions,
        deletions: stat.deletions,
      });
    }
  }
  if (byPath.size === 0) return null;
  const files = [...byPath.values()];
  return {
    files,
    additions: files.reduce((sum, file) => sum + file.additions, 0),
    deletions: files.reduce((sum, file) => sum + file.deletions, 0),
  };
}
