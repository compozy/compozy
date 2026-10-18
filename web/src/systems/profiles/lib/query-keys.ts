/**
 * Profile query identities.
 *
 * The selection read is the single Global slot. Plan reads are target-qualified
 * for the same reason worktree reads are workspace-qualified: an invalidation
 * can only ever reach the slot it names.
 */
export const profileKeys = {
  all: ["profiles"] as const,
  lists: () => [...profileKeys.all, "list"] as const,
  list: () => [...profileKeys.lists()] as const,
  details: () => [...profileKeys.all, "detail"] as const,
  detail: (name: string) => [...profileKeys.details(), name] as const,
  selections: () => [...profileKeys.all, "selection"] as const,
  selection: () => [...profileKeys.selections(), "global"] as const,
  plans: () => [...profileKeys.all, "plan"] as const,
  renamePlan: (name: string, newName: string) =>
    [...profileKeys.plans(), "rename", name, newName] as const,
  archivePlan: (name: string) => [...profileKeys.plans(), "archive", name] as const,
  deletePlan: (name: string) => [...profileKeys.plans(), "delete", name] as const,
  operations: () => [...profileKeys.all, "operations"] as const,
  iconCatalog: () => [...profileKeys.all, "icon-catalog"] as const,
};
