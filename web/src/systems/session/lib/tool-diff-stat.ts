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
export function splitLines(text: string): string[] {
  if (text.length === 0) return [];
  return text.split(/\r?\n/);
}

// Bound the O(m·n) LCS: real Edit snippets are small, so this only guards against
// a pathological giant replacement, where we fall back to counting every line as
// changed (additions = new lines, deletions = old lines).
const LCS_CELL_LIMIT = 250_000;

/** One line of an aligned old → new diff, with its 0-based position on each side it exists. */
export interface AlignedDiffLine {
  kind: "add" | "del" | "context";
  text: string;
  oldIndex?: number;
  newIndex?: number;
}

function replaceAll(a: readonly string[], b: readonly string[]): AlignedDiffLine[] {
  return [
    ...a.map((text, oldIndex) => ({ kind: "del" as const, text, oldIndex })),
    ...b.map((text, newIndex) => ({ kind: "add" as const, text, newIndex })),
  ];
}

/**
 * A minimal line diff (LCS) of `a` → `b`: unchanged lines as context, removed
 * lines before the lines that replace them. Past the cell bound every line
 * reads as changed.
 */
export function alignLines(a: readonly string[], b: readonly string[]): AlignedDiffLine[] {
  const m = a.length;
  const n = b.length;
  if (m === 0 || n === 0 || m * n > LCS_CELL_LIMIT) return replaceAll(a, b);
  const width = n + 1;
  const table = new Uint32Array((m + 1) * width);
  for (let i = m - 1; i >= 0; i -= 1) {
    for (let j = n - 1; j >= 0; j -= 1) {
      table[i * width + j] =
        a[i] === b[j]
          ? table[(i + 1) * width + j + 1]! + 1
          : Math.max(table[(i + 1) * width + j]!, table[i * width + j + 1]!);
    }
  }
  const lines: AlignedDiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) {
      lines.push({ kind: "context", text: a[i]!, oldIndex: i, newIndex: j });
      i += 1;
      j += 1;
    } else if (table[(i + 1) * width + j]! >= table[i * width + j + 1]!) {
      lines.push({ kind: "del", text: a[i]!, oldIndex: i });
      i += 1;
    } else {
      lines.push({ kind: "add", text: b[j]!, newIndex: j });
      j += 1;
    }
  }
  for (; i < m; i += 1) lines.push({ kind: "del", text: a[i]!, oldIndex: i });
  for (; j < n; j += 1) lines.push({ kind: "add", text: b[j]!, newIndex: j });
  return lines;
}

function editDiffStat(args: Record<string, unknown>): ToolDiffStat {
  const lines = alignLines(
    splitLines(stringArg(args, "old_string")),
    splitLines(stringArg(args, "new_string"))
  );
  return {
    additions: lines.filter(line => line.kind === "add").length,
    deletions: lines.filter(line => line.kind === "del").length,
  };
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
