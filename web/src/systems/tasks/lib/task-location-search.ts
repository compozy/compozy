import { z } from "zod";

import { TASK_INBOX_LANE_FILTER_IDS, type InboxLaneFilterId } from "./inbox-grouping";
import {
  TASK_LIST_SORT_OPTIONS,
  TASK_PRIORITY_OPTIONS,
  TASK_STATUS_OPTIONS,
  taskOwnerFilterFromValue,
} from "./tasks-list-filters";
import { TASK_TEMPLATE_IDS, type TaskTemplateId } from "./task-templates";
import type { TaskListSortKey, TaskPriority, TaskStatus, TaskViewMode } from "../types";

const taskRouteModeSchema = z.enum(["kanban", "dashboard", "inbox"]);
const taskListSortSchema = z.enum(TASK_LIST_SORT_OPTIONS);
const taskPrioritySchema = z.enum(TASK_PRIORITY_OPTIONS);
const taskStatusSchema = z.enum(TASK_STATUS_OPTIONS);
const inboxLaneSchema = z.enum(TASK_INBOX_LANE_FILTER_IDS);
const taskTemplateIdSchema = z.enum(TASK_TEMPLATE_IDS);

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
