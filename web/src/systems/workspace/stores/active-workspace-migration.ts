import type { BreadthMode, Focus } from "../lib/destination";

/** Persisted destination envelope: schema v2 of `compozy:active-workspace:v4`. */
export interface PersistedActiveWorkspace {
  breadthMode: BreadthMode;
  focus: Focus;
  selectedWorkspaceId: string | null;
  worktreeByScope: Record<string, string | null>;
  worktreePartitionByScope: Record<string, string>;
}

export const ACTIVE_WORKSPACE_SCHEMA_VERSION = 2;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function nonEmptyString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

function stringRecord<T>(
  value: unknown,
  accept: (entry: unknown) => entry is T
): Record<string, T> {
  if (!isRecord(value)) return {};
  return Object.fromEntries(
    Object.entries(value).filter((entry): entry is [string, T] => accept(entry[1]))
  );
}

const isWorktreeEntry = (entry: unknown): entry is string | null =>
  entry === null || typeof entry === "string";

/**
 * v1 → v2 on the same key, lossless: v1 stored a binary `scope`. `global`
 * becomes All projects focused on the remembered project (User without one);
 * `workspace` becomes the focused project. Worktree and selection fields are
 * kept; no scope has a partition claim yet, so nothing is pruned before a
 * fresh snapshot claims it. The retained desktop is dropped: the desktop now
 * follows the destination.
 */
export function migrateActiveWorkspace(persisted: unknown): PersistedActiveWorkspace {
  const context = isRecord(persisted) ? persisted : {};
  const selectedWorkspaceId = nonEmptyString(context.selectedWorkspaceId);
  const focus: Focus = selectedWorkspaceId
    ? { kind: "project", workspaceId: selectedWorkspaceId }
    : { kind: "user" };
  return {
    breadthMode: context.scope === "workspace" && selectedWorkspaceId ? "focused" : "all",
    focus,
    selectedWorkspaceId,
    worktreeByScope: stringRecord(context.worktreeByScope, isWorktreeEntry),
    worktreePartitionByScope: {},
  };
}
