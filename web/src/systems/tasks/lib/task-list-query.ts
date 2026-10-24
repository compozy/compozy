import type { InfiniteData } from "@tanstack/react-query";

import type {
  TaskListFacets,
  TaskListFilter,
  TaskListItem,
  TaskListPage,
  TaskListStableFilter,
  TaskStatus,
} from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

/** The Project filter narrows the aggregate only; sorted so equal filters share a key and cursor. */
function normalizeWorkspaceIds(filters: TaskListFilter): string[] | undefined {
  if (!filters.all_workspaces || !filters.workspace_ids?.length) return undefined;
  return [...new Set(filters.workspace_ids)].sort();
}

export function normalizeTaskListFilter(filters: TaskListFilter = {}): TaskListFilter {
  return {
    all_workspaces: filters.all_workspaces || undefined,
    workspace_ids: normalizeWorkspaceIds(filters),
    scope: filters.scope,
    workspace: normalizeOptionalText(filters.workspace),
    // Filters on the derived active-run worktree id. The daemon requires a
    // workspace alongside it, which `taskScopeForActiveWorkspace` guarantees.
    worktree: normalizeOptionalText(filters.worktree),
    status: filters.status,
    priority: filters.priority,
    include_drafts: filters.include_drafts,
    // Absent means excluded: the calm default is a server-side predicate, so the
    // request carries `include_loop` only when the reveal filter is on.
    include_loop: filters.include_loop,
    loop_run_id: normalizeOptionalText(filters.loop_run_id),
    approval_state: filters.approval_state,
    owner_kind: filters.owner_kind,
    owner_ref: normalizeOptionalText(filters.owner_ref),
    parent_task_id: normalizeOptionalText(filters.parent_task_id),
    query: normalizeOptionalText(filters.query),
    sort: filters.sort,
    cursor: normalizeOptionalText(filters.cursor),
    limit: filters.limit,
    profile: normalizeOptionalText(filters.profile),
    all_profiles: filters.all_profiles,
  };
}

export function taskListStableFilter(filters: TaskListFilter = {}): TaskListStableFilter {
  const normalized = normalizeTaskListFilter(filters);
  return {
    // Breadth is part of the stable filter: an aggregate cursor never pages a
    // single-project read, nor a differently narrowed aggregate.
    all_workspaces: normalized.all_workspaces,
    workspace_ids: normalized.workspace_ids,
    scope: normalized.scope,
    workspace: normalized.workspace,
    // Part of the stable filter, so it enters both the query key and the
    // cursor-bound page request — pagination can never straddle a scope change.
    worktree: normalized.worktree,
    status: normalized.status,
    priority: normalized.priority,
    include_drafts: normalized.include_drafts,
    // Stable, so the reveal state enters both the query key and every
    // cursor-bound page request — pagination can never straddle a reveal flip.
    include_loop: normalized.include_loop,
    loop_run_id: normalized.loop_run_id,
    approval_state: normalized.approval_state,
    owner_kind: normalized.owner_kind,
    owner_ref: normalized.owner_ref,
    parent_task_id: normalized.parent_task_id,
    query: normalized.query,
    sort: normalized.sort,
    limit: normalized.limit,
    // Profile scope is part of the request and therefore part of the key: two
    // profiles reading the same workspace are two catalogs, never one entry.
    profile: normalized.profile,
    all_profiles: normalized.all_profiles,
  };
}

/**
 * Names the list a read belongs to before anyone refines it: its breadth, project and worktree
 * scope, drafts, the Loop reveal, parent and profile lens. Status, priority, owner, text, sort
 * and the Project narrowing refine the same list, so they stay out. Equal populations may bridge
 * a refinement with the previous rows; a different one never inherits them.
 */
export function taskListPopulation(filters: TaskListStableFilter): string {
  return JSON.stringify([
    filters.all_workspaces ?? null,
    filters.scope ?? null,
    filters.workspace ?? null,
    filters.worktree ?? null,
    filters.include_drafts ?? null,
    filters.include_loop ?? null,
    filters.loop_run_id ?? null,
    filters.parent_task_id ?? null,
    filters.profile ?? null,
    filters.all_profiles ?? null,
  ]);
}

export function taskListPageRequest(
  filters: TaskListStableFilter,
  cursor?: string
): TaskListFilter {
  return {
    ...filters,
    cursor: normalizeOptionalText(cursor),
  };
}

export interface TaskListReadModel {
  tasks: TaskListItem[];
  total: number;
  facets: TaskListFacets;
}

const EMPTY_FACETS: TaskListFacets = { owners: [], statuses: [] };

export function taskStatusCountsFromFacets(facets: TaskListFacets): Record<TaskStatus, number> {
  const counts: Record<TaskStatus, number> = {
    draft: 0,
    pending: 0,
    blocked: 0,
    needs_attention: 0,
    ready: 0,
    in_progress: 0,
    completed: 0,
    failed: 0,
    canceled: 0,
  };
  for (const facet of facets.statuses) {
    counts[facet.status] = facet.count;
  }
  return counts;
}

export function readTaskListData(
  data: InfiniteData<TaskListPage, unknown> | undefined
): TaskListReadModel {
  const firstPage = data?.pages[0];
  const tasks: TaskListItem[] = [];
  const seen = new Set<string>();

  for (const page of data?.pages ?? []) {
    for (const task of page.tasks) {
      if (seen.has(task.id)) {
        continue;
      }
      seen.add(task.id);
      tasks.push(task);
    }
  }

  return {
    tasks,
    total: firstPage?.page.total ?? 0,
    facets: firstPage?.facets ?? EMPTY_FACETS,
  };
}
