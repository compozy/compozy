import type { QueryClient } from "@tanstack/react-query";

import type { AutomationRouteSearch } from "@/systems/automation";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationRouteHasActiveFilters,
  type AutomationJobStableFilter,
  automationMatchesActiveWorkspace,
  automationSuggestionsListOptions,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationTriggersListOptions,
  type AutomationTriggerStableFilter,
} from "@/systems/automation";
import { readProfileLens, readProfileScopeParams } from "@/systems/profiles";

async function resolveListScope(
  queryClient: QueryClient,
  search: AutomationRouteSearch
): Promise<Pick<AutomationJobStableFilter, "scope" | "workspace_id">> {
  const scope = search.scope === "all" ? undefined : search.scope;
  if (scope !== "workspace") return { scope };
  return { scope, workspace_id: (await resolveActiveWorkspaceId(queryClient)) ?? undefined };
}

export async function preloadAutomationJobsRoute(
  queryClient: QueryClient,
  search: AutomationRouteSearch
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const activeWorkspaceID =
    search.scope === "global" ? null : await resolveActiveWorkspaceId(queryClient);
  const scope =
    search.scope === "workspace"
      ? { scope: "workspace" as const, workspace_id: activeWorkspaceID ?? undefined }
      : { scope: search.scope === "all" ? undefined : search.scope };
  const filters: AutomationJobStableFilter = {
    ...scope,
    enabled: search.enabled,
    limit: 50,
    loop: search.loop,
    q: search.q,
    source: search.source,
  };
  if (scope.scope === "workspace" && !scope.workspace_id) return;
  const jobsQuery = queryClient.ensureInfiniteQueryData(
    automationJobsListOptions({ ...filters, ...profileScope })
  );
  const suggestionsQuery =
    activeWorkspaceID && !automationRouteHasActiveFilters(search)
      ? jobsQuery.then(result => {
          if (result.pages[0]?.page.total !== 0) return undefined;
          return queryClient.ensureQueryData(
            automationSuggestionsListOptions(activeWorkspaceID, "pending")
          );
        })
      : undefined;
  await settleRouteQueries([jobsQuery, ...(suggestionsQuery ? [suggestionsQuery] : [])]);
}

export async function preloadAutomationTriggersRoute(
  queryClient: QueryClient,
  search: AutomationRouteSearch
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const scope = await resolveListScope(queryClient, search);
  const filters: AutomationTriggerStableFilter = {
    ...scope,
    enabled: search.enabled,
    event: search.event,
    limit: 50,
    loop: search.loop,
    q: search.q,
    source: search.source,
  };
  if (scope.scope === "workspace" && !scope.workspace_id) return;
  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(
      automationTriggersListOptions({ ...filters, ...profileScope })
    ),
  ]);
}

export async function preloadAutomationJobDetailRoute(
  queryClient: QueryClient,
  jobId: string
): Promise<void> {
  if (!jobId) return;
  const [job, activeWorkspaceId] = await Promise.all([
    queryClient.ensureQueryData(automationJobDetailOptions(jobId)).catch(() => null),
    resolveActiveWorkspaceId(queryClient),
  ]);
  if (!job || !automationMatchesActiveWorkspace(job, activeWorkspaceId)) return;
  await settleRouteQueries([
    queryClient.ensureQueryData(automationJobRunsOptions(jobId, { limit: 10 })),
  ]);
}

export async function preloadAutomationTriggerDetailRoute(
  queryClient: QueryClient,
  triggerId: string
): Promise<void> {
  if (!triggerId) return;
  const [trigger, activeWorkspaceId] = await Promise.all([
    queryClient.ensureQueryData(automationTriggerDetailOptions(triggerId)).catch(() => null),
    resolveActiveWorkspaceId(queryClient),
  ]);
  if (!trigger || !automationMatchesActiveWorkspace(trigger, activeWorkspaceId)) return;
  await settleRouteQueries([
    queryClient.ensureQueryData(automationTriggerRunsOptions(triggerId, { limit: 10 })),
  ]);
}
