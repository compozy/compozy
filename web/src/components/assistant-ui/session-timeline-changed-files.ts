// Pure per-turn aggregation of the files an assistant turn modified via Edit/Write
// tool calls, feeding the settled-turn "Changed files" roll-up. AGH has no git
// checkpoint to read stats from — the +/- counts are derived directly from the
// tool payloads the reader already sees in the `edit-content` / `write-content`
// renderers:
//   - Write: the whole content is written, so additions = its line count, no deletions.
//   - Edit: a minimal line-level diff (LCS) of `old_string` → `new_string`.
// Same file edited twice within a turn collapses to one entry with summed stats.
// Only settled, non-error Edit/Write calls count; a failed edit changed nothing.

import { resolveRegisteredToolName } from "@/systems/session/lib/tool-labels";
import type { ChangedFileEntry, SessionTimelineToolPart } from "./session-timeline.logic";

// The file-modifying tools AGH surfaces structured diff sources for
// (`tool-renderers/{edit,write}-content.tsx`). Normalized through
// `resolveRegisteredToolName` so ACP-style titles ("Edit routes.go") also match.
const FILE_MODIFYING_TOOLS = new Set(["Edit", "Write"]);

export interface ChangedFilesSummary {
  files: ChangedFileEntry[];
  additions: number;
  deletions: number;
}

interface DiffStat {
  additions: number;
  deletions: number;
}

function toolFilePath(args: Record<string, unknown>): string | undefined {
  const raw = args.file_path ?? args.filePath;
  if (typeof raw !== "string") return undefined;
  const trimmed = raw.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function stringArg(args: Record<string, unknown>, key: string): string {
  const value = args[key];
  return typeof value === "string" ? value : "";
}

// Splits into lines without inventing a trailing empty line for a final newline
// artifact — an empty string is zero lines, "a\nb" is two.
function splitLines(text: string): string[] {
  if (text.length === 0) return [];
  return text.split(/\r?\n/);
}

// Bound the O(m·n) LCS: real Edit snippets are small, so this only guards against
// a pathological giant replacement, where we fall back to counting every line as
// changed (additions = new lines, deletions = old lines).
const LCS_CELL_LIMIT = 250_000;

function longestCommonSubsequenceLength(a: readonly string[], b: readonly string[]): number {
  const m = a.length;
  const n = b.length;
  if (m === 0 || n === 0) return 0;
  if (m * n > LCS_CELL_LIMIT) return 0;
  let previous = Array.from<number>({ length: n + 1 }).fill(0);
  let current = Array.from<number>({ length: n + 1 }).fill(0);
  for (let i = 1; i <= m; i += 1) {
    for (let j = 1; j <= n; j += 1) {
      current[j] =
        a[i - 1] === b[j - 1] ? previous[j - 1]! + 1 : Math.max(previous[j]!, current[j - 1]!);
    }
    [previous, current] = [current, previous];
    current.fill(0);
  }
  return previous[n]!;
}

function editDiffStat(args: Record<string, unknown>): DiffStat {
  const oldLines = splitLines(stringArg(args, "old_string"));
  const newLines = splitLines(stringArg(args, "new_string"));
  const common = longestCommonSubsequenceLength(oldLines, newLines);
  return { additions: newLines.length - common, deletions: oldLines.length - common };
}

function writeDiffStat(args: Record<string, unknown>): DiffStat {
  return { additions: splitLines(stringArg(args, "content")).length, deletions: 0 };
}

function toolDiffStat(canonicalName: string, args: Record<string, unknown>): DiffStat {
  return canonicalName === "Write" ? writeDiffStat(args) : editDiffStat(args);
}

/**
 * Aggregate a turn's tool parts into the changed-files summary, or `null` when the
 * turn modified no files. Only settled, non-error Edit/Write calls contribute;
 * repeated edits to one path collapse to a single first-touch-ordered entry whose
 * stats are summed.
 */
export function aggregateChangedFiles(
  tools: readonly SessionTimelineToolPart[]
): ChangedFilesSummary | null {
  const byPath = new Map<string, ChangedFileEntry>();
  for (const tool of tools) {
    if (tool.isError || tool.status !== "settled") continue;
    const canonicalName = resolveRegisteredToolName(tool.toolName);
    if (!FILE_MODIFYING_TOOLS.has(canonicalName)) continue;
    const path = toolFilePath(tool.args);
    if (!path) continue;
    const stat = toolDiffStat(canonicalName, tool.args);
    const existing = byPath.get(path);
    if (existing) {
      existing.additions += stat.additions;
      existing.deletions += stat.deletions;
    } else {
      byPath.set(path, { path, additions: stat.additions, deletions: stat.deletions });
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
