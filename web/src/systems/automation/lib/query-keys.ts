import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
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

/** All projects breadth, with the Project filter: a cache entry per selection. */
function breadthLens(allWorkspaces?: boolean, workspaceIds?: readonly string[]): string {
  return allWorkspaces === true ? `all:${(workspaceIds ?? []).join(",")}` : "";
}

function readScopeLens(scope: ProfileScopeParams): string {
  return "all_profiles" in scope ? PROFILE_AGGREGATE : normalizeText(scope.profile);
}

export function automationKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "automation");
  const jobs = () => [...all, "jobs"] as const;
  const jobLists = () => [...jobs(), "list"] as const;
  const jobDetails = () => [...jobs(), "detail"] as const;
  const jobRunsRoot = () => [...jobs(), "runs"] as const;
  const jobRunsFor = (id: string) => [...jobRunsRoot(), id] as const;
  const triggers = () => [...all, "triggers"] as const;
  const triggerLists = () => [...triggers(), "list"] as const;
  const triggerDetails = () => [...triggers(), "detail"] as const;
  const triggerRunsRoot = () => [...triggers(), "runs"] as const;
  const triggerRunsFor = (id: string) => [...triggerRunsRoot(), id] as const;
  const runs = () => [...all, "runs"] as const;
  const runLists = () => [...runs(), "list"] as const;
  const suggestions = () => [...all, "suggestions"] as const;
  const suggestionLists = () => [...suggestions(), "list"] as const;
  const suggestionAggregates = () => [...suggestionLists(), "all"] as const;

  return {
    all,
    jobs,
    jobLists,
    jobList: (filters: AutomationJobStableFilter = {}) => {
      const normalized = normalizeAutomationJobFilter(filters);
      return [
        ...jobLists(),
        normalized.scope ?? "",
        normalizeText(normalized.workspace_id),
        breadthLens(normalized.all_workspaces, normalized.workspace_ids),
        normalized.source ?? "",
        normalized.target ?? "",
        normalizeBoolean(normalized.enabled),
        normalizeText(normalized.q),
        normalizeNumber(normalized.limit),
        normalizeText(normalized.loop),
        profileLens(normalized.profile, normalized.all_profiles),
      ] as const;
    },
    jobDetails,
    jobDetail: (id: string, scope?: ProfileScopeParams) =>
      scope === undefined
        ? ([...jobDetails(), id] as const)
        : ([...jobDetails(), id, readScopeLens(scope)] as const),
    jobRunsRoot,
    jobRunsFor,
    jobRuns: (id: string, filters: AutomationRunHistoryFilter = {}) =>
      [
        ...jobRunsFor(id),
        filters.status ?? "",
        normalizeText(filters.since),
        normalizeText(filters.until),
        normalizeNumber(filters.limit),
        profileLens(filters.profile, filters.all_profiles),
      ] as const,

    triggers,
    triggerLists,
    triggerList: (filters: AutomationTriggerStableFilter = {}) => {
      const normalized = normalizeAutomationTriggerFilter(filters);
      return [
        ...triggerLists(),
        normalized.scope ?? "",
        normalizeText(normalized.workspace_id),
        breadthLens(normalized.all_workspaces, normalized.workspace_ids),
        normalized.source ?? "",
        normalized.target ?? "",
        normalizeBoolean(normalized.enabled),
        normalizeText(normalized.event),
        normalizeText(normalized.q),
        normalizeNumber(normalized.limit),
        normalizeText(normalized.loop),
        profileLens(normalized.profile, normalized.all_profiles),
      ] as const;
    },
    triggerDetails,
    triggerDetail: (id: string, scope?: ProfileScopeParams) =>
      scope === undefined
        ? ([...triggerDetails(), id] as const)
        : ([...triggerDetails(), id, readScopeLens(scope)] as const),
    triggerRunsRoot,
    triggerRunsFor,
    triggerRuns: (id: string, filters: AutomationRunHistoryFilter = {}) =>
      [
        ...triggerRunsFor(id),
        filters.status ?? "",
        normalizeText(filters.since),
        normalizeText(filters.until),
        normalizeNumber(filters.limit),
        profileLens(filters.profile, filters.all_profiles),
      ] as const,

    runs,
    runLists,
    runList: (filters: AutomationRunListFilter = {}) =>
      [
        ...runLists(),
        normalizeText(filters.job_id),
        normalizeText(filters.trigger_id),
        filters.status ?? "",
        normalizeText(filters.since),
        normalizeText(filters.until),
        normalizeNumber(filters.limit),
        profileLens(filters.profile, filters.all_profiles),
      ] as const,

    suggestions,
    suggestionLists,
    suggestionList: (workspaceID: string, status: string) =>
      [...suggestionLists(), workspaceID, status] as const,
    suggestionAggregates,
    suggestionAggregate: (workspaceIDs: readonly string[], status: string) =>
      [...suggestionAggregates(), [...workspaceIDs].sort(), status] as const,
  };
}

export const automationKeys = {
  ...automationKeysFor(LOCAL_COMPUTER_ID),
  forComputer: automationKeysFor,
};
