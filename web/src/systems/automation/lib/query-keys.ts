import type {
  AutomationJobStableFilter,
  AutomationRunHistoryFilter,
  AutomationRunListFilter,
  AutomationTriggerStableFilter,
} from "../types";
import {
  normalizeAutomationJobFilter,
  normalizeAutomationTriggerFilter,
} from "./automation-list-query";

function normalizeText(value?: string): string {
  return value ?? "";
}

function normalizeNumber(value?: number): string {
  return value == null ? "" : String(value);
}

function normalizeBoolean(value?: boolean): string {
  return value === undefined ? "" : String(value);
}

export const automationKeys = {
  all: ["automation"] as const,

  jobs: () => [...automationKeys.all, "jobs"] as const,
  jobLists: () => [...automationKeys.jobs(), "list"] as const,
  jobList: (filters: AutomationJobStableFilter = {}) => {
    const normalized = normalizeAutomationJobFilter(filters);
    return [
      ...automationKeys.jobLists(),
      normalized.scope ?? "",
      normalizeText(normalized.workspace_id),
      normalized.source ?? "",
      normalizeBoolean(normalized.enabled),
      normalizeText(normalized.q),
      normalizeNumber(normalized.limit),
      normalizeText(normalized.loop),
    ] as const;
  },
  jobDetails: () => [...automationKeys.jobs(), "detail"] as const,
  jobDetail: (id: string) => [...automationKeys.jobDetails(), id] as const,
  jobRunsRoot: () => [...automationKeys.jobs(), "runs"] as const,
  jobRuns: (id: string, filters: AutomationRunHistoryFilter = {}) =>
    [
      ...automationKeys.jobRunsRoot(),
      id,
      filters.status ?? "",
      normalizeText(filters.since),
      normalizeText(filters.until),
      normalizeNumber(filters.limit),
    ] as const,

  triggers: () => [...automationKeys.all, "triggers"] as const,
  triggerLists: () => [...automationKeys.triggers(), "list"] as const,
  triggerList: (filters: AutomationTriggerStableFilter = {}) => {
    const normalized = normalizeAutomationTriggerFilter(filters);
    return [
      ...automationKeys.triggerLists(),
      normalized.scope ?? "",
      normalizeText(normalized.workspace_id),
      normalized.source ?? "",
      normalizeBoolean(normalized.enabled),
      normalizeText(normalized.event),
      normalizeText(normalized.q),
      normalizeNumber(normalized.limit),
      normalizeText(normalized.loop),
    ] as const;
  },
  triggerDetails: () => [...automationKeys.triggers(), "detail"] as const,
  triggerDetail: (id: string) => [...automationKeys.triggerDetails(), id] as const,
  triggerRunsRoot: () => [...automationKeys.triggers(), "runs"] as const,
  triggerRuns: (id: string, filters: AutomationRunHistoryFilter = {}) =>
    [
      ...automationKeys.triggerRunsRoot(),
      id,
      filters.status ?? "",
      normalizeText(filters.since),
      normalizeText(filters.until),
      normalizeNumber(filters.limit),
    ] as const,

  runs: () => [...automationKeys.all, "runs"] as const,
  runLists: () => [...automationKeys.runs(), "list"] as const,
  runList: (filters: AutomationRunListFilter = {}) =>
    [
      ...automationKeys.runLists(),
      normalizeText(filters.job_id),
      normalizeText(filters.trigger_id),
      filters.status ?? "",
      normalizeText(filters.since),
      normalizeText(filters.until),
      normalizeNumber(filters.limit),
    ] as const,

  suggestions: () => [...automationKeys.all, "suggestions"] as const,
  suggestionLists: () => [...automationKeys.suggestions(), "list"] as const,
  suggestionList: (workspaceID: string, status: string) =>
    [...automationKeys.suggestionLists(), workspaceID, status] as const,
};
