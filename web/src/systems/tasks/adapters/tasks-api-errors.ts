import type { TaskStreamFilter } from "../types";

export class TasksApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "TasksApiError";
  }
}

export function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

export function buildTaskStreamUrl(taskId: string, filters: TaskStreamFilter = {}): string {
  const trimmedId = taskId.trim();
  if (trimmedId === "") {
    throw new TasksApiError("task id is required to build stream url", 400);
  }
  const path = `/api/tasks/${encodeURIComponent(trimmedId)}/stream`;
  const query = new URLSearchParams();
  if (filters.after_sequence !== undefined)
    query.set("after_sequence", String(filters.after_sequence));
  if (filters.profile) query.set("profile", filters.profile);
  if (filters.all_profiles) query.set("all_profiles", "true");
  const search = query.toString();
  return search ? `${path}?${search}` : path;
}
