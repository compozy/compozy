import {
  parseTasksSurfaceMode,
  validateTaskCreateSearch,
  validateTaskDetailSearch,
  type ResolvedTaskDetailSearch,
  type TaskCreateSearch,
  type TaskViewMode,
} from "@/systems/tasks";
import type { OsWindowRoute } from "../../lib/os-types";

/**
 * `create` and `edit` are dialog locations: they render the catalog or the task
 * detail underneath and layer the task editor over it, so each carries the
 * background surface's own state.
 */
export type TaskWindowLocation =
  | { kind: "catalog"; mode: TaskViewMode }
  | { kind: "create"; mode: TaskViewMode; search: TaskCreateSearch }
  | { kind: "detail"; taskId: string; search: ResolvedTaskDetailSearch }
  | { kind: "edit"; taskId: string; search: ResolvedTaskDetailSearch }
  | { kind: "run"; taskId: string; runId: string };

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function parseTaskWindowLocation(location: OsWindowRoute): TaskWindowLocation {
  if (location.pathname === "/tasks/new") {
    return {
      kind: "create",
      mode: parseTasksSurfaceMode(location.search),
      search: validateTaskCreateSearch(location.search),
    };
  }

  const runMatch = /^\/tasks\/([^/]+)\/runs\/([^/]+)$/.exec(location.pathname);
  if (runMatch) {
    return {
      kind: "run",
      taskId: decodePathSegment(runMatch[1]),
      runId: decodePathSegment(runMatch[2]),
    };
  }

  const editMatch = /^\/tasks\/([^/]+)\/edit$/.exec(location.pathname);
  if (editMatch) {
    return {
      kind: "edit",
      taskId: decodePathSegment(editMatch[1]),
      search: validateTaskDetailSearch(location.search),
    };
  }

  const detailMatch = /^\/tasks\/([^/]+)$/.exec(location.pathname);
  if (detailMatch) {
    return {
      kind: "detail",
      taskId: decodePathSegment(detailMatch[1]),
      search: validateTaskDetailSearch(location.search),
    };
  }

  return { kind: "catalog", mode: parseTasksSurfaceMode(location.search) };
}
