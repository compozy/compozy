// Per-call line-diff stats for the file-modifying tools (Edit/Write), derived
// directly from the tool args the renderers already see — CompozyOS has no git
// checkpoint to read stats from:
//   - Write: the whole content is written, so additions = its line count, no deletions.
//   - Edit: a minimal line-level diff (LCS) of `old_string` → `new_string`.
// Shared by the per-row diff stat and the per-turn changed-files aggregation.

import { resolveRegisteredToolName } from "./tool-labels";

/** The file-modifying tools CompozyOS surfaces structured diff sources for. */
export const FILE_MODIFYING_TOOLS = new Set(["Edit", "Write"]);

export interface ToolDiffStat {
  additions: number;
  deletions: number;
}

export interface ToolFileDiffStat extends ToolDiffStat {
  path: string;
}

export function toolFilePath(args: Record<string, unknown>): string | undefined {
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

function editDiffStat(args: Record<string, unknown>): ToolDiffStat {
  const oldLines = splitLines(stringArg(args, "old_string"));
  const newLines = splitLines(stringArg(args, "new_string"));
  const common = longestCommonSubsequenceLength(oldLines, newLines);
  return { additions: newLines.length - common, deletions: oldLines.length - common };
}

function writeDiffStat(args: Record<string, unknown>): ToolDiffStat {
  return { additions: splitLines(stringArg(args, "content")).length, deletions: 0 };
}

export function toolDiffStat(canonicalName: string, args: Record<string, unknown>): ToolDiffStat {
  return canonicalName === "Write" ? writeDiffStat(args) : editDiffStat(args);
}

/**
 * The per-file diff stat for one tool call, or `null` when the call is not a
 * file-modifying tool or names no file. Callers gate on settled/non-error —
 * a failed edit changed nothing.
 */
export function fileDiffStatForTool(
  toolName: string,
  args: Record<string, unknown>
): ToolFileDiffStat | null {
  const canonicalName = resolveRegisteredToolName(toolName);
  if (!FILE_MODIFYING_TOOLS.has(canonicalName)) return null;
  const path = toolFilePath(args);
  if (!path) return null;
  return { path, ...toolDiffStat(canonicalName, args) };
}
