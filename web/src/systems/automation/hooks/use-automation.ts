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
import { type ProfileScopeParams, useProfileReadScope } from "@/systems/profiles";

interface QueryHookOptions {
  enabled?: boolean;
  profileScope?: ProfileScopeParams;
}

export function useAutomationJobs(
  filters: AutomationJobStableFilter = {},
  options: QueryHookOptions = {}
) {
  // Scope applied at the single list hook, so a switch moves every consumer.
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery({
    ...automationJobsListOptions({ ...filters, ...params }),
    enabled: options.enabled ?? true,
  });
  return {
    ...query,
    jobs: flattenAutomationJobPages(query.data),
    total: query.data?.pages[0]?.page.total ?? 0,
  };
}

export function useAutomationJob(id: string, options: QueryHookOptions = {}) {
  const { params } = useProfileReadScope();
  return useQuery(
    automationJobDetailOptions(id, options.profileScope ?? params, options.enabled ?? true)
  );
}

export function useAutomationJobRuns(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  options: QueryHookOptions = {}
) {
  const { params } = useProfileReadScope();
  return useQuery(
    automationJobRunsOptions(
      id,
      { ...filters, ...(options.profileScope ?? params) },
      options.enabled ?? true
    )
  );
}

export function useAutomationTriggers(
  filters: AutomationTriggerStableFilter = {},
  options: QueryHookOptions = {}
) {
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery({
    ...automationTriggersListOptions({ ...filters, ...params }),
    enabled: options.enabled ?? true,
  });
  return {
    ...query,
    triggers: flattenAutomationTriggerPages(query.data),
    total: query.data?.pages[0]?.page.total ?? 0,
  };
}

export function useAutomationTrigger(id: string, options: QueryHookOptions = {}) {
  const { params } = useProfileReadScope();
  return useQuery(
    automationTriggerDetailOptions(id, options.profileScope ?? params, options.enabled ?? true)
  );
}

export function useAutomationTriggerRuns(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  options: QueryHookOptions = {}
) {
  const { params } = useProfileReadScope();
  return useQuery(
    automationTriggerRunsOptions(
      id,
      { ...filters, ...(options.profileScope ?? params) },
      options.enabled ?? true
    )
  );
}

export function useAutomationRuns(
  filters: AutomationRunListFilter = {},
  options: QueryHookOptions = {}
) {
  const { params } = useProfileReadScope();
  return useQuery(automationRunsListOptions({ ...filters, ...params }, options.enabled ?? true));
}
