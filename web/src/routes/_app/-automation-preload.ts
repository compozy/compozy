import type { QueryClient } from "@tanstack/react-query";

import type { AutomationEntity, AutomationsRouteSearch } from "@/systems/automation";

import {
  resolveActiveWorkspaceId,
  resolveActiveWorkspaceSelection,
  settleRouteQueries,
} from "./-route-preload";
import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationListLoopFilter,
  automationRouteHasActiveFilters,
  automationMatchesActiveWorkspace,
  automationSuggestionsSource,
  automationSuggestionsSourceOptions,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationTriggersListOptions,
} from "@/systems/automation";
import { readProfileScopeParams } from "@/systems/profiles";
import { automationBreadthParams, breadthQuery } from "@/systems/workspace";

/**
 * Both lists with the shared filters minus `start`: a Start view only decides which kind
 * renders, so both totals (the view counts) load. `target=task` never asks for triggers.
 */
export async function preloadAutomationsRoute(
  queryClient: QueryClient,
  search: AutomationsRouteSearch
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient);
  // The destination decides breadth: a project lists only its own automations,
  // User only those without a project, All projects every one.
  const dataScope = (await resolveActiveWorkspaceSelection(queryClient))?.dataScope ?? null;
  if (!dataScope) return;
  const filters = {
    enabled: search.enabled,
    limit: 50,
    loop: automationListLoopFilter(search),
    q: search.q,
    source: search.source,
    target: search.target,
    ...automationBreadthParams(breadthQuery(dataScope)),
    ...profileScope,
  };
  const suggestions = automationSuggestionsSource(dataScope, !profileScope.all_profiles);
  const jobsQuery = queryClient.ensureInfiniteQueryData(automationJobsListOptions(filters));
  const triggersQuery =
    search.target !== "task"
      ? queryClient.ensureInfiniteQueryData(automationTriggersListOptions(filters))
      : undefined;
  const suggestionsQuery =
    suggestions && !automationRouteHasActiveFilters(search) && triggersQuery
      ? Promise.all([jobsQuery, triggersQuery]).then(([jobs, triggers]) => {
          if (jobs.pages[0]?.page.total !== 0 || triggers.pages[0]?.page.total !== 0) {
            return undefined;
          }
          return queryClient.ensureQueryData(
            automationSuggestionsSourceOptions(suggestions, "pending")
          );
        })
      : undefined;
  await settleRouteQueries(
    [jobsQuery, triggersQuery, suggestionsQuery].filter(query => query !== undefined)
  );
}

/**
 * Starts the entity read at once and, only for an entity the active workspace owns, warms its
 * runs. Nothing here holds navigation: the detail window owns the loading presentation, and
 * waiting on the entity would leave the desktop blank until it answers.
 */
async function prefetchAutomationDetail(
  queryClient: QueryClient,
  readEntity: () => Promise<AutomationEntity>,
  readRuns: () => Promise<unknown>
): Promise<void> {
  const [entity, activeWorkspaceId] = await Promise.all([
    readEntity().catch(() => null),
    resolveActiveWorkspaceId(queryClient),
  ]);
  if (!entity || !automationMatchesActiveWorkspace(entity, activeWorkspaceId)) return;
  await readRuns();
}

export function preloadAutomationJobDetailRoute(
  queryClient: QueryClient,
  jobId: string
): Promise<void> {
  if (!jobId) return Promise.resolve();
  const profileScope = readProfileScopeParams(queryClient);
  return settleRouteQueries([
    prefetchAutomationDetail(
      queryClient,
      () => queryClient.ensureQueryData(automationJobDetailOptions(jobId, profileScope)),
      () =>
        queryClient.ensureQueryData(automationJobRunsOptions(jobId, { limit: 10, ...profileScope }))
    ),
  ]);
}

export function preloadAutomationTriggerDetailRoute(
  queryClient: QueryClient,
  triggerId: string
): Promise<void> {
  if (!triggerId) return Promise.resolve();
  const profileScope = readProfileScopeParams(queryClient);
  return settleRouteQueries([
    prefetchAutomationDetail(
      queryClient,
      () => queryClient.ensureQueryData(automationTriggerDetailOptions(triggerId, profileScope)),
      () =>
        queryClient.ensureQueryData(
          automationTriggerRunsOptions(triggerId, { limit: 10, ...profileScope })
        )
    ),
  ]);
}
