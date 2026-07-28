import type { SearchSchemaInput } from "@tanstack/react-router";

export const TASK_DETAIL_TABS = ["overview", "runs", "activity"] as const;
export type TaskDetailTab = (typeof TASK_DETAIL_TABS)[number];

export const TASK_INSPECT_TARGETS = ["diagnostics", "stream", "bridges", "raw"] as const;
export type TaskInspectTarget = (typeof TASK_INSPECT_TARGETS)[number];

/** Navigation shape; the route validator materializes `tab` before rendering. */
export interface TaskDetailSearch {
  tab?: TaskDetailTab;
  inspect?: TaskInspectTarget;
}

export interface ResolvedTaskDetailSearch extends TaskDetailSearch {
  tab: TaskDetailTab;
}

function parseEnum<T extends string>(value: unknown, allowed: readonly T[], fallback: T): T {
  return typeof value === "string" && (allowed as readonly string[]).includes(value)
    ? (value as T)
    : fallback;
}

export function validateTaskDetailSearch(search: Record<string, unknown>): ResolvedTaskDetailSearch;
export function validateTaskDetailSearch(
  search: TaskDetailSearch & SearchSchemaInput
): ResolvedTaskDetailSearch;
export function validateTaskDetailSearch(
  search: Record<string, unknown> | TaskDetailSearch
): ResolvedTaskDetailSearch {
  const inspect =
    typeof search.inspect === "string" &&
    (TASK_INSPECT_TARGETS as readonly string[]).includes(search.inspect)
      ? (search.inspect as TaskInspectTarget)
      : undefined;
  return {
    tab: parseEnum(search.tab, TASK_DETAIL_TABS, "overview"),
    ...(inspect ? { inspect } : {}),
  };
}
