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

export function normalizeTaskListFilter(filters: TaskListFilter = {}): TaskListFilter {
  return {
    scope: filters.scope,
    workspace: normalizeOptionalText(filters.workspace),
    status: filters.status,
    priority: filters.priority,
    include_drafts: filters.include_drafts,
    approval_state: filters.approval_state,
    owner_kind: filters.owner_kind,
    owner_ref: normalizeOptionalText(filters.owner_ref),
    parent_task_id: normalizeOptionalText(filters.parent_task_id),
    participation_channel: normalizeOptionalText(filters.participation_channel),
    query: normalizeOptionalText(filters.query),
    sort: filters.sort,
    cursor: normalizeOptionalText(filters.cursor),
    limit: filters.limit,
  };
}

export function taskListStableFilter(filters: TaskListFilter = {}): TaskListStableFilter {
  const normalized = normalizeTaskListFilter(filters);
  return {
    scope: normalized.scope,
    workspace: normalized.workspace,
    status: normalized.status,
    priority: normalized.priority,
    include_drafts: normalized.include_drafts,
    approval_state: normalized.approval_state,
    owner_kind: normalized.owner_kind,
    owner_ref: normalized.owner_ref,
    parent_task_id: normalized.parent_task_id,
    participation_channel: normalized.participation_channel,
    query: normalized.query,
    sort: normalized.sort,
    limit: normalized.limit,
  };
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
