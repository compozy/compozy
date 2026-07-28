import { z } from "zod";

import type { InboxLaneFilterId } from "./inbox-grouping";
import { taskOwnerFilterFromValue } from "./tasks-list-filters";
import type { TaskTemplateId } from "./task-templates";
import type { TaskListSortKey, TaskPriority, TaskStatus, TaskViewMode } from "../types";

const taskRouteModeSchema = z.enum(["kanban", "dashboard", "inbox"]);
const taskListSortSchema = z.enum(["recent", "priority"]);
const taskPrioritySchema = z.enum(["urgent", "high", "medium", "low"]);
const taskStatusSchema = z.enum([
  "in_progress",
  "ready",
  "blocked",
  "needs_attention",
  "pending",
  "draft",
  "completed",
  "failed",
  "canceled",
]);
const inboxLaneSchema = z.enum([
  "all",
  "my_work",
  "approvals",
  "failed_runs",
  "blocked",
  "archived",
]);
const taskTemplateIdSchema = z.enum([
  "one_shot",
  "recurring",
  "epic",
  "remote_peer",
  "human_in_loop",
  "blank",
]);

export interface TasksRouteSearch {
  mode?: "kanban" | "dashboard" | "inbox";
  query?: string;
  status?: TaskStatus;
  priority?: TaskPriority;
  owner?: string;
  sort?: TaskListSortKey;
  inboxLane?: InboxLaneFilterId;
  inboxStatus?: TaskStatus;
  inboxPriority?: TaskPriority;
  inboxUnread?: true;
  inboxQuery?: string;
}

/** Create is an overlay, so its location carries the complete catalog state underneath it. */
export interface TaskCreateSearch extends TasksRouteSearch {
  template?: TaskTemplateId;
}

type TasksSearchInput = Partial<Record<keyof TasksRouteSearch, unknown>>;

function optionalParsed<T>(schema: z.ZodType<T>, value: unknown): T | undefined {
  const result = schema.safeParse(value);
  return result.success ? result.data : undefined;
}

function optionalSearchText(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

export function validateTasksSearch(search: TasksSearchInput): TasksRouteSearch {
  const mode = optionalParsed(taskRouteModeSchema, search.mode);
  const query = optionalSearchText(search.query);
  const status = optionalParsed(taskStatusSchema, search.status);
  const priority = optionalParsed(taskPrioritySchema, search.priority);
  const owner = optionalSearchText(search.owner);
  const validOwner = owner && taskOwnerFilterFromValue(owner) ? owner : undefined;
  const sort = optionalParsed(taskListSortSchema, search.sort);
  const inboxLane = optionalParsed(inboxLaneSchema, search.inboxLane);
  const inboxStatus = optionalParsed(taskStatusSchema, search.inboxStatus);
  const inboxPriority = optionalParsed(taskPrioritySchema, search.inboxPriority);
  const inboxUnread = search.inboxUnread === true ? true : undefined;
  const inboxQuery = optionalSearchText(search.inboxQuery);
  return {
    ...(mode ? { mode } : {}),
    ...(query ? { query } : {}),
    ...(status ? { status } : {}),
    ...(priority ? { priority } : {}),
    ...(validOwner ? { owner: validOwner } : {}),
    ...(sort ? { sort } : {}),
    ...(inboxLane ? { inboxLane } : {}),
    ...(inboxStatus ? { inboxStatus } : {}),
    ...(inboxPriority ? { inboxPriority } : {}),
    ...(inboxUnread ? { inboxUnread } : {}),
    ...(inboxQuery ? { inboxQuery } : {}),
  };
}

export function parseTasksSurfaceMode(search: TasksSearchInput): TaskViewMode {
  return validateTasksSearch(search).mode ?? "list";
}

export function validateTaskCreateSearch(search: Record<string, unknown>): TaskCreateSearch {
  const template = taskTemplateIdSchema.safeParse(search.template);
  return {
    ...validateTasksSearch(search),
    ...(template.success ? { template: template.data } : {}),
  };
}

/** Builds the catalog location while preserving the filters behind the create overlay. */
export function taskCatalogSearchFor(
  mode: TaskViewMode,
  current: TasksRouteSearch = {}
): TasksRouteSearch {
  return validateTasksSearch({
    ...current,
    mode: mode === "list" ? undefined : mode,
  });
}

/** Builds a mode-tab location without carrying an invisible list search into another surface. */
export function taskModeSearchFor(
  mode: TaskViewMode,
  current: TasksRouteSearch = {}
): TasksRouteSearch {
  return validateTasksSearch({
    ...current,
    mode: mode === "list" ? undefined : mode,
    query: undefined,
  });
}
