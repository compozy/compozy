import { USER_SCOPE_SELECTOR, type DataScope } from "./destination";

/** The User entry of a Project filter; sent as `@user`. */
export const USER_SCOPE = USER_SCOPE_SELECTOR;

/** Project-filter value: a workspace id, or `USER_SCOPE` for items without a project. */
export type ProjectFilterValue = string;

/** Route-agnostic query; each adapter maps it with its route's encoder (see `_dx.md` matrix). */
export interface BreadthQuery {
  scope: DataScope;
  /** Only with `scope.kind == "all"`: narrows the aggregate server-side. Never `global` or `""`. */
  workspaceIds?: readonly string[];
}

/**
 * Build the query a listing sends for a data scope and its Project filter. The
 * filter applies only under All projects; empty means every project plus User.
 * Values are trimmed, deduplicated, and never empty or the reserved `global`.
 */
export function breadthQuery(
  scope: DataScope,
  filter?: readonly ProjectFilterValue[]
): BreadthQuery {
  if (scope.kind !== "all" || !filter) return { scope };
  const workspaceIds = new Set<string>();
  for (const value of filter) {
    const id = value.trim();
    if (id !== "" && id !== "global") workspaceIds.add(id);
  }
  return workspaceIds.size === 0 ? { scope } : { scope, workspaceIds: [...workspaceIds] };
}

/** Session-catalog spelling: `all_workspaces` (+ `workspace_ids`), or one `workspace_id` (`@user` for User). */
export function sessionCatalogBreadthParams(query: BreadthQuery): {
  all_workspaces?: true;
  workspace_ids?: string[];
  workspace_id?: string;
} {
  const { scope } = query;
  if (scope.kind === "all") {
    return query.workspaceIds
      ? { all_workspaces: true, workspace_ids: [...query.workspaceIds] }
      : { all_workspaces: true };
  }
  return { workspace_id: scope.kind === "user" ? USER_SCOPE_SELECTOR : scope.workspaceId };
}

/**
 * Task-list spelling: `all_workspaces` (+ `workspace_ids`) under All projects,
 * `scope=global` for User (no workspace), `scope=workspace` for a project.
 */
export function taskBreadthParams(query: BreadthQuery): {
  all_workspaces?: true;
  workspace_ids?: string[];
  scope?: "global" | "workspace";
  workspace?: string;
} {
  const { scope } = query;
  if (scope.kind === "project") return { scope: "workspace", workspace: scope.workspaceId };
  if (scope.kind === "user") return { scope: "global" };
  return query.workspaceIds
    ? { all_workspaces: true, workspace_ids: [...query.workspaceIds] }
    : { all_workspaces: true };
}
