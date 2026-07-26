import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationRunsListOptions,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationTriggersListOptions,
} from "../lib/query-options";
import {
  flattenAutomationJobPages,
  flattenAutomationTriggerPages,
} from "../lib/automation-list-query";
import type {
  AutomationJobStableFilter,
  AutomationRunHistoryFilter,
  AutomationRunListFilter,
  AutomationTriggerStableFilter,
} from "../types";

interface QueryHookOptions {
  enabled?: boolean;
}

export function useAutomationJobs(
  filters: AutomationJobStableFilter = {},
  options: QueryHookOptions = {}
) {
  const query = useInfiniteQuery({
    ...automationJobsListOptions(filters),
    enabled: options.enabled ?? true,
  });
  return {
    ...query,
    jobs: flattenAutomationJobPages(query.data),
    total: query.data?.pages[0]?.page.total ?? 0,
  };
}

export function useAutomationJob(id: string, options: QueryHookOptions = {}) {
  return useQuery(automationJobDetailOptions(id, options.enabled ?? true));
}

export function useAutomationJobRuns(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(automationJobRunsOptions(id, filters, options.enabled ?? true));
}

export function useAutomationTriggers(
  filters: AutomationTriggerStableFilter = {},
  options: QueryHookOptions = {}
) {
  const query = useInfiniteQuery({
    ...automationTriggersListOptions(filters),
    enabled: options.enabled ?? true,
  });
  return {
    ...query,
    triggers: flattenAutomationTriggerPages(query.data),
    total: query.data?.pages[0]?.page.total ?? 0,
  };
}

export function useAutomationTrigger(id: string, options: QueryHookOptions = {}) {
  return useQuery(automationTriggerDetailOptions(id, options.enabled ?? true));
}

export function useAutomationTriggerRuns(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(automationTriggerRunsOptions(id, filters, options.enabled ?? true));
}

export function useAutomationRuns(
  filters: AutomationRunListFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(automationRunsListOptions(filters, options.enabled ?? true));
}
