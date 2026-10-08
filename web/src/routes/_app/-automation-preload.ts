import type { QueryClient } from "@tanstack/react-query";

import type { AutomationsRouteSearch } from "@/systems/automation";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationListLoopFilter,
  automationRouteHasActiveFilters,
  automationMatchesActiveWorkspace,
  automationSuggestionsListOptions,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationTriggersListOptions,
} from "@/systems/automation";
import { readProfileLens, readProfileScopeParams } from "@/systems/profiles";

/**
 * Both lists with the shared filters minus `start`: a Start view only decides which kind
 * renders, so both totals (the view counts) load. `target=task` never asks for triggers.
 */
export async function preloadAutomationsRoute(
  queryClient: QueryClient,
  search: AutomationsRouteSearch
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const activeWorkspaceID =
    search.scope === "global" ? null : await resolveActiveWorkspaceId(queryClient);
  if (search.scope === "workspace" && !activeWorkspaceID) return;
  const filters = {
    scope: search.scope,
    workspace_id: search.scope === "workspace" ? (activeWorkspaceID ?? undefined) : undefined,
    enabled: search.enabled,
    limit: 50,
    loop: automationListLoopFilter(search),
    q: search.q,
    source: search.source,
    target: search.target,
    ...profileScope,
  };
  const jobsQuery = queryClient.ensureInfiniteQueryData(automationJobsListOptions(filters));
  const triggersQuery =
    search.target !== "task"
      ? queryClient.ensureInfiniteQueryData(automationTriggersListOptions(filters))
      : undefined;
  const suggestionsQuery =
    activeWorkspaceID && !automationRouteHasActiveFilters(search) && triggersQuery
      ? Promise.all([jobsQuery, triggersQuery]).then(([jobs, triggers]) => {
          if (jobs.pages[0]?.page.total !== 0 || triggers.pages[0]?.page.total !== 0) {
            return undefined;
          }
          return queryClient.ensureQueryData(
            automationSuggestionsListOptions(activeWorkspaceID, "pending")
          );
        })
      : undefined;
  await settleRouteQueries(
    [jobsQuery, triggersQuery, suggestionsQuery].filter(query => query !== undefined)
  );
}

export async function preloadAutomationJobDetailRoute(
  queryClient: QueryClient,
  jobId: string
): Promise<void> {
  if (!jobId) return;
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const [job, activeWorkspaceId] = await Promise.all([
    queryClient.ensureQueryData(automationJobDetailOptions(jobId, profileScope)).catch(() => null),
    resolveActiveWorkspaceId(queryClient),
  ]);
  if (!job || !automationMatchesActiveWorkspace(job, activeWorkspaceId)) return;
  await settleRouteQueries([
    queryClient.ensureQueryData(automationJobRunsOptions(jobId, { limit: 10, ...profileScope })),
  ]);
}

export async function preloadAutomationTriggerDetailRoute(
  queryClient: QueryClient,
  triggerId: string
): Promise<void> {
  if (!triggerId) return;
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const [trigger, activeWorkspaceId] = await Promise.all([
    queryClient
      .ensureQueryData(automationTriggerDetailOptions(triggerId, profileScope))
      .catch(() => null),
    resolveActiveWorkspaceId(queryClient),
  ]);
  if (!trigger || !automationMatchesActiveWorkspace(trigger, activeWorkspaceId)) return;
  await settleRouteQueries([
    queryClient.ensureQueryData(
      automationTriggerRunsOptions(triggerId, { limit: 10, ...profileScope })
    ),
  ]);
}
