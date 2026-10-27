import type { InfiniteData } from "@tanstack/react-query";

import type {
  TaskInboxFilter,
  TaskInboxGroup,
  TaskInboxPage,
  TaskInboxStableFilter,
} from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

export function normalizeTaskInboxFilter(filters: TaskInboxFilter = {}): TaskInboxFilter {
  return {
    scope: filters.scope,
    workspace: normalizeOptionalText(filters.workspace),
    all_workspaces: filters.all_workspaces || undefined,
    workspace_ids: filters.all_workspaces ? filters.workspace_ids : undefined,
    worktree: normalizeOptionalText(filters.worktree),
    owner_kind: filters.owner_kind,
    owner_ref: normalizeOptionalText(filters.owner_ref),
    lane: filters.lane,
    status: filters.status,
    priority: filters.priority,
    unread: filters.unread,
    query: normalizeOptionalText(filters.query),
    cursor: normalizeOptionalText(filters.cursor),
    limit: filters.limit,
    profile: normalizeOptionalText(filters.profile),
    all_profiles: filters.all_profiles,
  };
}

export function taskInboxStableFilter(filters: TaskInboxFilter = {}): TaskInboxStableFilter {
  const normalized = normalizeTaskInboxFilter(filters);
  return {
    scope: normalized.scope,
    workspace: normalized.workspace,
    all_workspaces: normalized.all_workspaces,
    workspace_ids: normalized.workspace_ids,
    worktree: normalized.worktree,
    owner_kind: normalized.owner_kind,
    owner_ref: normalized.owner_ref,
    lane: normalized.lane,
    status: normalized.status,
    priority: normalized.priority,
    unread: normalized.unread,
    query: normalized.query,
    limit: normalized.limit,
    // Profile scope rides the stable filter, so it lands in both the request and
    // the key — pagination can never straddle a profile switch.
    profile: normalized.profile,
    all_profiles: normalized.all_profiles,
  };
}

/**
 * Names the inbox a read belongs to before anyone refines it: its breadth, project and worktree
 * scope, owner and profile lens. Lane, status, priority, unread, text and the Project narrowing
 * refine the same inbox, so they stay out. Equal populations may bridge a refinement with the
 * previous rows; a different one never inherits them.
 */
export function taskInboxPopulation(filters: TaskInboxStableFilter): string {
  return JSON.stringify([
    filters.all_workspaces ?? null,
    filters.scope ?? null,
    filters.workspace ?? null,
    filters.worktree ?? null,
    filters.owner_kind ?? null,
    filters.owner_ref ?? null,
    filters.profile ?? null,
    filters.all_profiles ?? null,
  ]);
}

export function taskInboxPageRequest(
  filters: TaskInboxStableFilter,
  cursor?: string
): TaskInboxFilter {
  return {
    ...filters,
    cursor: normalizeOptionalText(cursor),
  };
}

export function readTaskInboxData(
  data: InfiniteData<TaskInboxPage, unknown> | undefined
): TaskInboxPage | undefined {
  const firstPage = data?.pages[0];
  if (!firstPage) {
    return undefined;
  }

  const groups = new Map<TaskInboxGroup["lane"], TaskInboxGroup>();
  const seenTasks = new Set<string>();

  for (const page of data.pages) {
    for (const group of page.groups ?? []) {
      const existing = groups.get(group.lane);
      const items = existing?.items ? [...existing.items] : [];
      for (const item of group.items ?? []) {
        if (seenTasks.has(item.task.id)) {
          continue;
        }
        seenTasks.add(item.task.id);
        items.push(item);
      }
      groups.set(group.lane, {
        ...(existing ?? group),
        items,
      });
    }
  }

  const lastPage = data.pages.at(-1) ?? firstPage;
  return {
    ...firstPage,
    groups: Array.from(groups.values()),
    page: {
      ...firstPage.page,
      has_more: lastPage.page.has_more,
      next_cursor: lastPage.page.next_cursor,
    },
  };
}
