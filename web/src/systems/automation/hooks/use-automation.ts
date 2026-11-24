import { useInfiniteQuery, useQueries, useQuery } from "@tanstack/react-query";

import {
  mergeAutomationJobPages,
  useComputers,
  type ComputerAutomationJobsInput,
} from "@/systems/computers";
import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationJobsPageOptions,
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
  AutomationJob,
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

export interface AutomationJobComputerMeta {
  id: string;
  name: string;
  offline: boolean;
}

export function useAutomationJobs(
  filters: AutomationJobStableFilter = {},
  options: QueryHookOptions = {}
) {
  const enabled = options.enabled ?? true;
  // Scope applied at the single list hook, so a switch moves every consumer.
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery({
    ...automationJobsListOptions({ ...filters, ...params }),
    enabled,
  });
  const computers = useComputers({ enabled, passive: true });
  const remoteComputers = computers.enabledComputers;
  const remoteQueries = useQueries({
    queries: remoteComputers.map(computer => ({
      ...automationJobsPageOptions({ ...filters, ...params }, computer.id),
      enabled: enabled && computer.status === "connected",
    })),
  });

  const localJobs = flattenAutomationJobPages(query.data);
  const computerByJobId = new Map<string, AutomationJobComputerMeta>();
  let jobs: AutomationJob[] = localJobs;
  let remoteTotal = 0;

  if (remoteComputers.length > 0) {
    const inputs: ComputerAutomationJobsInput<AutomationJob>[] = [
      {
        computer: { id: "local", name: "This computer", status: "connected" },
        jobs: localJobs,
      },
    ];
    for (let index = 0; index < remoteComputers.length; index++) {
      const computer = remoteComputers[index];
      const remoteQuery = remoteQueries[index];
      if (!computer || !remoteQuery) continue;
      const remoteJobs = remoteQuery.data?.jobs ?? [];
      remoteTotal += remoteQuery.data?.page.total ?? remoteJobs.length;
      inputs.push({
        computer: { id: computer.id, name: computer.name, status: computer.status },
        jobs: remoteJobs,
      });
    }
    const merged = mergeAutomationJobPages(inputs);
    jobs = merged.map(row => row.job);
    for (const row of merged) {
      if (row.computerId !== "local") {
        computerByJobId.set(row.job.id, {
          id: row.computerId,
          name: row.computerName,
          offline: row.offline,
        });
      }
    }
  }

  return {
    ...query,
    jobs,
    computerByJobId,
    total: (query.data?.pages[0]?.page.total ?? 0) + remoteTotal,
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
