export interface TaskGroupSearch {
  task_group_id?: string;
}

export function taskGroupSearchSchema(search: Record<string, unknown>): TaskGroupSearch {
  const taskGroupId = typeof search.task_group_id === "string" ? search.task_group_id.trim() : "";
  return taskGroupId ? { task_group_id: taskGroupId } : {};
}
