import type { SchedulerBacklogQuery } from "../types";

function normalizeText(value?: string | null): string {
  return typeof value === "string" ? value : "";
}

function normalizeNumber(value?: number): string {
  return value === undefined ? "" : String(value);
}

function normalizeFlag(value?: boolean): string {
  return value === undefined ? "" : value ? "1" : "0";
}

export const schedulerKeys = {
  all: ["scheduler"] as const,
  status: () => [...schedulerKeys.all, "status"] as const,
  backlog: (query: SchedulerBacklogQuery = {}) =>
    [
      ...schedulerKeys.all,
      "backlog",
      normalizeNumber(query.limit),
      normalizeText(query.scope),
      normalizeText(query.workspace),
      normalizeFlag(query.include_paused),
    ] as const,
};
