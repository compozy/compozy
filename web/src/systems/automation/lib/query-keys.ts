import type {
  AutomationJobStableFilter,
  AutomationRunHistoryFilter,
  AutomationRunListFilter,
  AutomationTriggerStableFilter,
} from "../types";
import { PROFILE_AGGREGATE } from "@/systems/profiles";
import type { ProfileScopeParams } from "@/systems/profiles";
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

function profileLens(profile?: string, allProfiles?: boolean): string {
  return allProfiles === true ? PROFILE_AGGREGATE : normalizeText(profile);
}

function readScopeLens(scope: ProfileScopeParams): string {
  return "all_profiles" in scope ? PROFILE_AGGREGATE : normalizeText(scope.profile);
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
      profileLens(normalized.profile, normalized.all_profiles),
    ] as const;
  },
  jobDetails: () => [...automationKeys.jobs(), "detail"] as const,
  jobDetail: (id: string, scope?: ProfileScopeParams) =>
    scope === undefined
      ? ([...automationKeys.jobDetails(), id] as const)
      : ([...automationKeys.jobDetails(), id, readScopeLens(scope)] as const),
  jobRunsRoot: () => [...automationKeys.jobs(), "runs"] as const,
  jobRunsFor: (id: string) => [...automationKeys.jobRunsRoot(), id] as const,
  jobRuns: (id: string, filters: AutomationRunHistoryFilter = {}) =>
    [
      ...automationKeys.jobRunsFor(id),
      filters.status ?? "",
      normalizeText(filters.since),
      normalizeText(filters.until),
      normalizeNumber(filters.limit),
      profileLens(filters.profile, filters.all_profiles),
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
      profileLens(normalized.profile, normalized.all_profiles),
    ] as const;
  },
  triggerDetails: () => [...automationKeys.triggers(), "detail"] as const,
  triggerDetail: (id: string, scope?: ProfileScopeParams) =>
    scope === undefined
      ? ([...automationKeys.triggerDetails(), id] as const)
      : ([...automationKeys.triggerDetails(), id, readScopeLens(scope)] as const),
  triggerRunsRoot: () => [...automationKeys.triggers(), "runs"] as const,
  triggerRunsFor: (id: string) => [...automationKeys.triggerRunsRoot(), id] as const,
  triggerRuns: (id: string, filters: AutomationRunHistoryFilter = {}) =>
    [
      ...automationKeys.triggerRunsFor(id),
      filters.status ?? "",
      normalizeText(filters.since),
      normalizeText(filters.until),
      normalizeNumber(filters.limit),
      profileLens(filters.profile, filters.all_profiles),
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
      profileLens(filters.profile, filters.all_profiles),
    ] as const,

  suggestions: () => [...automationKeys.all, "suggestions"] as const,
  suggestionLists: () => [...automationKeys.suggestions(), "list"] as const,
  suggestionList: (workspaceID: string, status: string) =>
    [...automationKeys.suggestionLists(), workspaceID, status] as const,
};
