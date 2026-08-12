import type { WorktreePayload } from "../types";

const NON_SLUG_CHARACTER = /[^a-z0-9]+/g;
const REPEATED_DASH = /-+/g;
const FALLBACK_SLUG = "worktree";

/**
 * Mirrors the daemon's `worktree.SanitizeName` so the dialog preview matches the
 * name the daemon will actually mint. Any drift here shows the user a path that
 * never gets created.
 */
export function sanitizeWorktreeName(value: string): string {
  const slug = value
    .trim()
    .toLowerCase()
    .replace(NON_SLUG_CHARACTER, "-")
    .replace(REPEATED_DASH, "-")
    .replace(/^-|-$/g, "");
  return slug === "" ? FALLBACK_SLUG : slug;
}

/** Branch and directory both derive from the name — no prefix is ever forced. */
export function deriveWorktreeBranch(name: string): string {
  return sanitizeWorktreeName(name);
}

/**
 * Recovers the placement root from paths the daemon already minted. The root is
 * operator-configurable (`worktrees.root`) and no endpoint exposes it, so it is
 * inferred from real data or reported as unknown — never guessed from a default.
 */
export function deriveWorktreeParentDir(
  worktrees: ReadonlyArray<Pick<WorktreePayload, "path" | "name" | "origin">>
): string | null {
  for (const worktree of worktrees) {
    // Adopted externals keep their foreign path and say nothing about the root.
    if (worktree.origin === "adopted") continue;
    const separator = worktree.path.includes("\\") ? "\\" : "/";
    const index = worktree.path.lastIndexOf(separator);
    if (index <= 0) continue;
    const parent = worktree.path.slice(0, index);
    const basename = worktree.path.slice(index + 1);
    // Only trust the parent when the leaf really is this worktree's directory.
    if (basename === sanitizeWorktreeName(worktree.name)) return parent;
  }
  return null;
}

export interface WorktreeCreatePreview {
  branch: string;
  /** `null` when the placement root is unknown — the row renders branch only. */
  path: string | null;
}

export function buildWorktreeCreatePreview(
  name: string,
  branchOverride: string | undefined,
  parentDir: string | null
): WorktreeCreatePreview | null {
  const effectiveName = name.trim();
  if (effectiveName === "") return null;

  const branch = branchOverride?.trim()
    ? branchOverride.trim()
    : deriveWorktreeBranch(effectiveName);
  const directory = sanitizeWorktreeName(effectiveName);
  const separator = parentDir?.includes("\\") ? "\\" : "/";
  return { branch, path: parentDir ? `${parentDir}${separator}${directory}` : null };
}
