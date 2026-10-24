import type { SearchSchemaInput } from "@tanstack/react-router";

import { ownerDataScope, windowBindingValue } from "@/systems/workspace";

export const TASK_DETAIL_TABS = ["overview", "runs", "activity"] as const;
export type TaskDetailTab = (typeof TASK_DETAIL_TABS)[number];

export const TASK_INSPECT_TARGETS = ["diagnostics", "stream", "raw"] as const;
export type TaskInspectTarget = (typeof TASK_INSPECT_TARGETS)[number];

/** Navigation shape; the route validator materializes `tab` before rendering. */
export interface TaskDetailSearch {
  tab?: TaskDetailTab;
  inspect?: TaskInspectTarget;
  /**
   * The window's data binding (`"<id>"` or `"@user"`), set by open-in-place
   * under All projects. It only says which scope the window reads; the task
   * itself is still read by id.
   */
  workspace?: string;
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
  const workspace = typeof search.workspace === "string" ? search.workspace.trim() : "";
  return {
    tab: parseEnum(search.tab, TASK_DETAIL_TABS, "overview"),
    ...(inspect ? { inspect } : {}),
    ...(workspace !== "" ? { workspace } : {}),
  };
}

/** The detail search that binds a task's window to its own scope (`"<id>"` or `"@user"`). */
export function taskWindowBinding(workspaceId: string | null | undefined): { workspace: string } {
  return { workspace: windowBindingValue(ownerDataScope(workspaceId)) };
}
