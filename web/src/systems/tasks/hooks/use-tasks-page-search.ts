import { useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import type {
  TaskListSortKey,
  TaskPriority,
  TaskRecordsFilter,
  TaskStatus,
  TaskViewMode,
} from "../types";
import type { InboxLaneFilterId } from "@/systems/tasks/lib/inbox-grouping";
import {
  taskOwnerFilterFromValue,
  taskOwnerFilterValue,
  type TaskFilterOwnerOption,
} from "@/systems/tasks/lib/tasks-list-filters";

import {
  parseTasksSurfaceMode,
  validateTasksSearch,
  type TasksRouteSearch,
} from "../lib/task-location-search";

type SearchChangeHandler = (update: (current: TasksRouteSearch) => TasksRouteSearch) => void;

const SEARCH_DEBOUNCE_MS = 200;

/** Catalog filters read from the validated route search, with their calm defaults. */
function tasksRouteFilters(routeSearch: TasksRouteSearch) {
  return {
    inboxLaneFilter: routeSearch.inboxLane ?? "all",
    inboxPriorityFilter: routeSearch.inboxPriority ?? null,
    inboxStatusFilter: routeSearch.inboxStatus ?? null,
    inboxUnreadOnly: routeSearch.inboxUnread === true,
    mode: parseTasksSurfaceMode(routeSearch),
    ownerFilter: taskOwnerFilterFromValue(routeSearch.owner),
    priorityFilter: routeSearch.priority ?? null,
    routeInboxSearchQuery: routeSearch.inboxQuery ?? "",
    routeSearchQuery: routeSearch.query ?? "",
    sortBy: routeSearch.sort ?? "recent",
    statusFilter: routeSearch.status ?? null,
  };
}

/**
 * Ephemeral by contract (US-002.AC-3): local state, never route search, so it
 * survives neither a reload nor a shared link. The mode rides along so
 * switching surface resets it.
 */
function useTaskRecordsReveal(mode: TaskViewMode) {
  const [reveal, setReveal] = useState<{ mode: TaskViewMode; records: TaskRecordsFilter }>({
    mode,
    records: "work",
  });
  if (reveal.mode !== mode) {
    setReveal({ mode, records: "work" });
  }
  const recordsFilter: TaskRecordsFilter = reveal.mode === mode ? reveal.records : "work";
  return {
    handleRecordsFilterChange: (records: TaskRecordsFilter) => setReveal({ mode, records }),
    // The reveal is a list-surface affordance; kanban roots are work items only.
    includeLoop: mode === "list" && recordsFilter === "loop",
    recordsFilter,
  };
}

/**
 * Route-owned catalog filters: the validated URL state is the sole owner, so
 * every handler writes a patch back through `onSearchChange`.
 */
function useTasksPageSearch(routeSearch: TasksRouteSearch, onSearchChange?: SearchChangeHandler) {
  const filters = tasksRouteFilters(routeSearch);
  const reveal = useTaskRecordsReveal(filters.mode);
  const updateSearch = (patch: Partial<TasksRouteSearch>) => {
    onSearchChange?.(current =>
      validateTasksSearch({
        ...current,
        ...patch,
      })
    );
  };
  const listSearch = useDebouncedInput({
    delayMs: SEARCH_DEBOUNCE_MS,
    externalValue: filters.routeSearchQuery,
    onCommit: query => updateSearch({ query: query.trim() ? query : undefined }),
  });
  const inboxSearch = useDebouncedInput({
    delayMs: SEARCH_DEBOUNCE_MS,
    externalValue: filters.routeInboxSearchQuery,
    onCommit: query => updateSearch({ inboxQuery: query.trim() ? query : undefined }),
  });

  // The reveal counts as a filter here, or a revealed workspace with no loop
  // records would fall through to the zero-inventory template panel instead of
  // the filter-scoped message that explains what emptied the list.
  const hasListFilters = Boolean(
    filters.statusFilter ||
    filters.ownerFilter ||
    filters.priorityFilter ||
    filters.routeSearchQuery.trim() ||
    reveal.includeLoop
  );

  return {
    ...filters,
    ...reveal,
    handleInboxLaneChange: (lane: InboxLaneFilterId) =>
      updateSearch({ inboxLane: lane === "all" ? undefined : lane }),
    handleInboxPriorityChange: (priority: TaskPriority | null) =>
      updateSearch({ inboxPriority: priority ?? undefined }),
    handleInboxStatusChange: (status: TaskStatus | null) =>
      updateSearch({ inboxStatus: status ?? undefined }),
    handleInboxUnreadToggle: (unreadOnly: boolean) =>
      updateSearch({ inboxUnread: unreadOnly ? true : undefined }),
    handleOwnerChange: (owner: TaskFilterOwnerOption | null) =>
      updateSearch({ owner: owner ? taskOwnerFilterValue(owner) : undefined }),
    handlePriorityChange: (priority: TaskPriority | null) =>
      updateSearch({ priority: priority ?? undefined }),
    handleSortChange: (sort: TaskListSortKey) =>
      updateSearch({ sort: sort === "recent" ? undefined : sort }),
    handleStatusChange: (status: TaskStatus | null) =>
      updateSearch({ status: status ?? undefined }),
    hasListFilters,
    inboxSearchQuery: inboxSearch.draftValue,
    searchQuery: listSearch.draftValue,
    setInboxSearchQuery: inboxSearch.setDraftValue,
    setSearchQuery: listSearch.setDraftValue,
  };
}

export { useTasksPageSearch };
export type { SearchChangeHandler };
