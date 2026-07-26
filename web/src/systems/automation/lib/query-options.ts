import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  getAutomationJob,
  getAutomationTrigger,
  listAutomationJobRuns,
  listAutomationJobs,
  listAutomationRuns,
  listAutomationTriggerRuns,
  listAutomationTriggers,
} from "../adapters/automation-api";
import { listAutomationSuggestions } from "../adapters/automation-suggestions-api";
import { automationKeys } from "./query-keys";
import {
  automationJobRequest,
  automationTriggerRequest,
  normalizeAutomationJobFilter,
  normalizeAutomationTriggerFilter,
} from "./automation-list-query";
import type {
  AutomationJobStableFilter,
  AutomationRunHistoryFilter,
  AutomationRunListFilter,
  AutomationSuggestionStatus,
  AutomationTriggerStableFilter,
} from "../types";

const DEFAULT_STALE_TIME = 15_000;
const DEFAULT_REFETCH_INTERVAL = 30_000;
const RUNS_REFETCH_INTERVAL = 15_000;

export function automationSuggestionsListOptions(
  workspaceID: string,
  status: AutomationSuggestionStatus = "pending"
) {
  return queryOptions({
    queryKey: automationKeys.suggestionList(workspaceID, status),
    queryFn: ({ signal }) => listAutomationSuggestions(workspaceID, status, signal),
    staleTime: DEFAULT_STALE_TIME,
    enabled: Boolean(workspaceID),
  });
}

export function automationJobsListOptions(filters: AutomationJobStableFilter = {}) {
  const normalizedFilters = normalizeAutomationJobFilter(filters);
  return infiniteQueryOptions({
    queryKey: automationKeys.jobList(normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listAutomationJobs(automationJobRequest(normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: DEFAULT_STALE_TIME,
  });
}

export function automationJobDetailOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: automationKeys.jobDetail(id),
    queryFn: ({ signal }) => getAutomationJob(id, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function automationJobRunsOptions(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  enabled = true
) {
  return queryOptions({
    queryKey: automationKeys.jobRuns(id, filters),
    queryFn: ({ signal }) => listAutomationJobRuns(id, filters, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: RUNS_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function automationTriggersListOptions(filters: AutomationTriggerStableFilter = {}) {
  const normalizedFilters = normalizeAutomationTriggerFilter(filters);
  return infiniteQueryOptions({
    queryKey: automationKeys.triggerList(normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listAutomationTriggers(automationTriggerRequest(normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: DEFAULT_STALE_TIME,
  });
}

export function automationTriggerDetailOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: automationKeys.triggerDetail(id),
    queryFn: ({ signal }) => getAutomationTrigger(id, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function automationTriggerRunsOptions(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  enabled = true
) {
  return queryOptions({
    queryKey: automationKeys.triggerRuns(id, filters),
    queryFn: ({ signal }) => listAutomationTriggerRuns(id, filters, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: RUNS_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function automationRunsListOptions(filters: AutomationRunListFilter = {}, enabled = true) {
  return queryOptions({
    queryKey: automationKeys.runList(filters),
    queryFn: ({ signal }) => listAutomationRuns(filters, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: RUNS_REFETCH_INTERVAL,
    enabled,
  });
}
