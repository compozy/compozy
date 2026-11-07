import type { DiffViewerChangeType, DiffViewerItem, FileTreeDecorations } from "@compozy/ui";

import type {
  SessionChangeFile,
  SessionChangesScope,
  SessionChangesSummary,
} from "../adapters/session-changes-api";

/** Past either bound the Changes tab shows one file at a time (US-009.AC-1). */
export const LARGE_DIFF_MAX_FILES = 200;
export const LARGE_DIFF_MAX_LINES = 20_000;

/** True when a summary has more than 200 files or more than 20,000 changed lines. */
export function isLargeDiff(totals: SessionChangesSummary["totals"]): boolean {
  return (
    totals.files > LARGE_DIFF_MAX_FILES ||
    totals.additions + totals.deletions > LARGE_DIFF_MAX_LINES
  );
}

/** Why a file's unchanged context can't expand: the daemon refused its full contents. */
export type SessionChangeExpandBlock = "too_large" | "sensitive";

/** A refused expansion, kept for the item version it happened on; a newer version retries. */
export interface SessionChangeExpandRefusal {
  version: string;
  reason: SessionChangeExpandBlock;
}

/** A diff list item that also carries the summary row it came from. */
export interface SessionChangeItem extends DiffViewerItem {
  file: SessionChangeFile;
  /** The fingerprint the summary listed; a patch produced under another one is stale. */
  fingerprint: string;
  /** Set when expanding context was refused; the header says why (US-007.EC-5). */
  expandBlock?: SessionChangeExpandBlock;
}

function changeType(file: SessionChangeFile): DiffViewerChangeType {
  switch (file.status) {
    case "added":
    case "untracked":
      return "new";
    case "deleted":
      return "deleted";
    case "renamed":
      return file.additions + file.deletions === 0 ? "rename-pure" : "rename-changed";
    default:
      return "change";
  }
}

export interface SessionChangeItemsOptions {
  /** Sensitive paths the operator chose **Show contents** for; never persisted. */
  revealed: ReadonlySet<string>;
  /** Explicit collapse choices; they survive refreshes for files still present (US-010.AC-4). */
  collapsed: Readonly<Record<string, boolean>>;
  /** Refused context expansions by path; the matching version shows no expand control. */
  expandRefusals?: ReadonlyMap<string, SessionChangeExpandRefusal>;
}

/**
 * The diff list in summary order. An item's version changes with the revision or the file's
 * fingerprint, so a refresh reloads exactly the files that moved.
 */
export function sessionChangeItems(
  summary: SessionChangesSummary,
  options: SessionChangeItemsOptions
): SessionChangeItem[] {
  return summary.files.map(file => {
    const revealed = options.revealed.has(file.path);
    const gated = file.sensitive === true && !revealed;
    const version = `${summary.revision}|${file.fingerprint}${revealed ? "|revealed" : ""}`;
    const refusal = options.expandRefusals?.get(file.path);
    const expandBlock = refusal?.version === version ? refusal.reason : undefined;
    return {
      id: file.path,
      version,
      path: file.path,
      ...(file.old_path ? { prevPath: file.old_path } : {}),
      changeType: changeType(file),
      loadable: !(file.binary === true || gated),
      ...(expandBlock ? { expandable: false, expandBlock } : {}),
      // Lockfiles and generated files start collapsed, one click expands (US-007.EC-3).
      collapsed: options.collapsed[file.path] ?? file.generated === true,
      file,
      fingerprint: file.fingerprint,
    };
  });
}

const TREE_STATUS: Record<string, NonNullable<FileTreeDecorations[string]["status"]>> = {
  added: "added",
  modified: "modified",
  deleted: "deleted",
  renamed: "renamed",
  untracked: "untracked",
  conflicted: "conflicted",
  type_changed: "type_changed",
};

/** Per-file `+N −M`, status glyph, and lock for the changed-files tree. */
export function sessionChangeDecorations(files: readonly SessionChangeFile[]): FileTreeDecorations {
  const decorations: Record<string, FileTreeDecorations[string]> = {};
  for (const file of files) {
    const status = TREE_STATUS[file.status];
    decorations[file.path] = {
      ...(status ? { status } : {}),
      additions: file.additions,
      deletions: file.deletions,
      ...(file.sensitive ? { sensitive: true } : {}),
    };
  }
  return decorations;
}

/** The single-letter status shown in diff headers (`C` for a conflict, US-007.EC-7). */
export function changeStatusLetter(status: string): string {
  switch (status) {
    case "added":
      return "A";
    case "deleted":
      return "D";
    case "renamed":
      return "R";
    case "untracked":
      return "U";
    case "conflicted":
      return "C";
    case "type_changed":
      return "T";
    default:
      return "M";
  }
}

/** The first new-side line a patch changes; Open in viewer lands there (US-007.AC-4). */
export function firstChangedLine(patch: string): number | undefined {
  let line: number | null = null;
  for (const text of patch.split("\n")) {
    const hunk = /^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(text);
    if (hunk) {
      line = Number(hunk[1]);
      continue;
    }
    if (line === null) continue;
    if (text.startsWith("+")) return Math.max(1, line);
    if (text.startsWith("-") || text.startsWith("\\")) continue;
    line += 1;
  }
  return undefined;
}

/**
 * Where a changed-file click lands (US-005.AC-6): Changes at that file when the summary lists
 * it, else Files at that path.
 */
export function focusedChangeRoute(
  files: readonly SessionChangeFile[],
  path: string
): "changes" | "files" {
  return files.some(file => file.path === path) ? "changes" : "files";
}

/** The header's `head → base` label; a detached HEAD shows its short SHA (US-008.EC-3). */
export function changesHeadLabel(head: SessionChangesSummary["head"]): string {
  return head.branch || head.sha.slice(0, 7);
}

/** The "No changes" sentence for the active scope (US-007.EC-1). */
export function noChangesText(scope: SessionChangesScope, base: string): string {
  return scope === "uncommitted" || !base ? "No uncommitted changes" : `No changes against ${base}`;
}

const NUMBER = new Intl.NumberFormat("en-US");

/** `+1,204` — a lower bound adds a trailing `+` (US-009.EC-1). */
export function formatChangeCount(value: number, partial = false): string {
  return `${NUMBER.format(value)}${partial ? "+" : ""}`;
}
