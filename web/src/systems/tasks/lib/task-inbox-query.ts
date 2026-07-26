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
    owner_kind: filters.owner_kind,
    owner_ref: normalizeOptionalText(filters.owner_ref),
    lane: filters.lane,
    status: filters.status,
    priority: filters.priority,
    unread: filters.unread,
    query: normalizeOptionalText(filters.query),
    cursor: normalizeOptionalText(filters.cursor),
    limit: filters.limit,
  };
}

export function taskInboxStableFilter(filters: TaskInboxFilter = {}): TaskInboxStableFilter {
  const normalized = normalizeTaskInboxFilter(filters);
  return {
    scope: normalized.scope,
    workspace: normalized.workspace,
    owner_kind: normalized.owner_kind,
    owner_ref: normalized.owner_ref,
    lane: normalized.lane,
    status: normalized.status,
    priority: normalized.priority,
    unread: normalized.unread,
    query: normalized.query,
    limit: normalized.limit,
  };
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
