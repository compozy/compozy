import type { ProfileLens } from "../types";

/** Stable string for a lens, so a workspace slot and the Global slot never collide. */
export function profileLensKey(lens: ProfileLens): string {
  return lens.scope === "workspace" ? `workspace:${lens.workspaceId}` : "global";
}

/**
 * Profile query identities.
 *
 * Selection reads are lens-qualified and plan reads are target-qualified for the
 * same reason worktree reads are workspace-qualified: an invalidation can only
 * ever reach the slot it names.
 */
export const profileKeys = {
  all: ["profiles"] as const,
  lists: () => [...profileKeys.all, "list"] as const,
  list: () => [...profileKeys.lists()] as const,
  details: () => [...profileKeys.all, "detail"] as const,
  detail: (name: string) => [...profileKeys.details(), name] as const,
  selections: () => [...profileKeys.all, "selection"] as const,
  selectionMap: () => [...profileKeys.selections(), "map"] as const,
  selection: (lens: ProfileLens) => [...profileKeys.selections(), profileLensKey(lens)] as const,
  plans: () => [...profileKeys.all, "plan"] as const,
  renamePlan: (name: string, newName: string) =>
    [...profileKeys.plans(), "rename", name, newName] as const,
  archivePlan: (name: string) => [...profileKeys.plans(), "archive", name] as const,
  deletePlan: (name: string) => [...profileKeys.plans(), "delete", name] as const,
  operations: () => [...profileKeys.all, "operations"] as const,
};
