import {
  focusedWorkspaceSelector,
  useWindowBindingSearch,
  useWindowDataWorkspace,
  type DataScope,
} from "@/systems/workspace";

export interface LoopWindowScope {
  /** The window's data scope; `null` while unsettled. */
  scope: DataScope | null;
  /** The workspace path an item read uses: a project id, `@user`, or `""` (aggregate, unsettled). */
  workspaceId: string;
  /** Keeps in-window item navigations on this window's serialized binding. */
  search: { workspace?: string };
}

/**
 * The mounted Loops window's data scope: its binding wins in every partition,
 * an unbound window reads its partition (`global` → the All projects aggregate).
 */
export function useLoopWindowScope(): LoopWindowScope {
  const scope = useWindowDataWorkspace();
  const search = useWindowBindingSearch();
  return { scope, workspaceId: focusedWorkspaceSelector(scope), search };
}
