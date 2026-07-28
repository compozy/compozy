import type { QueryClient } from "@tanstack/react-query";

import {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationMatchesActiveWorkspace,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationSuggestionsListOptions,
  automationTriggersListOptions,
  type AutomationJobStableFilter,
  type AutomationTriggerStableFilter,
} from "@/systems/automation";
import type { AutomationRouteSearch } from "@/systems/os/apps/automation/use-automation-page";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

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
  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(automationJobsListOptions(filters)),
    ...(activeWorkspaceID && search.scope !== "global"
      ? [
          queryClient.ensureQueryData(
            automationSuggestionsListOptions(activeWorkspaceID, "pending")
          ),
        ]
      : []),
  ]);
}

export async function preloadAutomationTriggersRoute(
  queryClient: QueryClient,
  search: AutomationRouteSearch
): Promise<void> {
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
    queryClient.ensureInfiniteQueryData(automationTriggersListOptions(filters)),
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
