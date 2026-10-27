import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceSelection, settleRouteQueries } from "./-route-preload";
import {
  loopAnnotationsOptions,
  type LoopCatalogStableFilter,
  loopConfigOptions,
  loopDefinitionProfile,
  loopDetailOptions,
  loopRunDetailOptions,
  loopRunDiffOptions,
  loopRunDiffQuery,
  type LoopRunDiffRouteSearch,
  type LoopRunsFilter,
  loopRunsListOptions,
  loopRunsOptions,
  loopsAggregateOptions,
  loopsCatalogOptions,
} from "@/systems/loops";
import {
  profileSelectionOptions,
  readActingProfile,
  readProfileScopeParams,
  readProfileView,
} from "@/systems/profiles";
import {
  dataScopeFromSelector,
  focusedWorkspaceSelector,
  loopBreadthTarget,
  type DataScope,
} from "@/systems/workspace";

/**
 * The data scope a Loops route reads, by the window binding rule: a serialized
 * `workspace` binding wins; otherwise the destination (a project, User, or the
 * All projects aggregate). `null` while unsettled.
 */
async function resolveRouteScope(
  queryClient: QueryClient,
  routeWorkspaceId?: string
): Promise<DataScope | null> {
  const bound = routeWorkspaceId ? dataScopeFromSelector(routeWorkspaceId) : null;
  if (bound) return bound;
  return (await resolveActiveWorkspaceSelection(queryClient)).dataScope;
}

/** Item routes read one workspace path; an unbound All projects item has none to warm. */
async function withRouteWorkspace(
  queryClient: QueryClient,
  routeWorkspaceId: string | undefined,
  preload: (workspaceId: string) => readonly Promise<unknown>[]
): Promise<void> {
  const workspaceId = focusedWorkspaceSelector(
    await resolveRouteScope(queryClient, routeWorkspaceId)
  );
  if (!workspaceId) return;
  await settleRouteQueries(preload(workspaceId));
}

/** Definitions resolve for one concrete profile: under All profiles, the acting (remembered) one. */
async function definitionProfile(queryClient: QueryClient) {
  const view = readProfileView(queryClient);
  if (view.kind === "aggregate") {
    await queryClient.ensureQueryData(profileSelectionOptions()).catch(() => undefined);
  }
  return loopDefinitionProfile({
    aggregate: view.kind === "aggregate",
    destination: readActingProfile(queryClient),
    params: readProfileScopeParams(queryClient),
  });
}

export async function preloadLoopsRoute(
  queryClient: QueryClient,
  filters: LoopCatalogStableFilter,
  routeWorkspaceId?: string
): Promise<void> {
  const scope = await resolveRouteScope(queryClient, routeWorkspaceId);
  if (scope === null) return;
  const profile = await definitionProfile(queryClient);
  if (scope.kind === "all") {
    const { limit: _pageSize, ...batchFilters } = filters;
    await settleRouteQueries([
      queryClient.ensureQueryData(
        loopsAggregateOptions({ all_workspaces: true }, { ...batchFilters, ...profile })
      ),
    ]);
    return;
  }
  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(
      loopsCatalogOptions(focusedWorkspaceSelector(scope), { ...filters, ...profile })
    ),
  ]);
}

export async function preloadLoopDetailRoute(
  queryClient: QueryClient,
  name: string,
  routeWorkspaceId?: string
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient);
  const definitions = await definitionProfile(queryClient);
  return withRouteWorkspace(queryClient, routeWorkspaceId, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopConfigOptions(workspaceId, name)),
    queryClient.ensureInfiniteQueryData(
      loopsCatalogOptions(workspaceId, {
        limit: 50,
        q: name,
        sort: "name",
        ...definitions,
      })
    ),
    queryClient.ensureQueryData(
      loopRunsOptions(workspaceId, { loop: name, limit: 5, ...profileScope })
    ),
  ]);
}

export function preloadLoopRunFormRoute(
  queryClient: QueryClient,
  name: string,
  routeWorkspaceId?: string
): Promise<void> {
  return withRouteWorkspace(queryClient, routeWorkspaceId, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopConfigOptions(workspaceId, name)),
  ]);
}

export function preloadLoopEditorRoute(
  queryClient: QueryClient,
  name: string,
  routeWorkspaceId?: string
): Promise<void> {
  return withRouteWorkspace(queryClient, routeWorkspaceId, workspaceId => [
    import("@/systems/os/apps/loops/loop-editor-location"),
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopAnnotationsOptions(workspaceId, name)),
  ]);
}

export function preloadLoopConfigureRoute(
  queryClient: QueryClient,
  name: string,
  routeWorkspaceId?: string
): Promise<void> {
  return withRouteWorkspace(queryClient, routeWorkspaceId, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopConfigOptions(workspaceId, name)),
  ]);
}

export async function preloadLoopRunsRoute(
  queryClient: QueryClient,
  filters: LoopRunsFilter,
  routeWorkspaceId?: string
): Promise<void> {
  const scope = await resolveRouteScope(queryClient, routeWorkspaceId);
  if (scope === null) return;
  const profileScope = readProfileScopeParams(queryClient);
  await settleRouteQueries([
    queryClient.ensureQueryData(
      loopRunsListOptions(loopBreadthTarget({ scope }), { ...filters, ...profileScope })
    ),
  ]);
}

export async function preloadLoopRunDetailRoute(
  queryClient: QueryClient,
  runId: string,
  routeWorkspaceId?: string
): Promise<void> {
  const workspaceId = focusedWorkspaceSelector(
    await resolveRouteScope(queryClient, routeWorkspaceId)
  );
  if (!workspaceId) {
    return;
  }
  const profileScope = readProfileScopeParams(queryClient);

  const run = queryClient.ensureQueryData(
    loopRunDetailOptions(workspaceId, runId, true, profileScope)
  );
  // A run that does not pin its executed definition reads the Loop's current one. That read
  // needs the run, so it chains off it; neither delays the route, which shows its own skeleton.
  const definition = run.then(({ executed_definition: executedDefinition, run: record }) =>
    !executedDefinition && record.loop_name
      ? queryClient.ensureQueryData(loopDetailOptions(workspaceId, record.loop_name))
      : undefined
  );
  await settleRouteQueries([run, definition]);
}

export async function preloadLoopRunDiffRoute(
  queryClient: QueryClient,
  runId: string,
  search: LoopRunDiffRouteSearch
): Promise<void> {
  const workspaceId = focusedWorkspaceSelector(
    await resolveRouteScope(queryClient, search.workspace)
  );
  if (!workspaceId) {
    return;
  }
  const profileScope = readProfileScopeParams(queryClient);
  await settleRouteQueries([
    queryClient.ensureQueryData(loopRunDetailOptions(workspaceId, runId, true, profileScope)),
  ]);
  const query = loopRunDiffQuery(search);
  if (!query) {
    return;
  }
  await settleRouteQueries([
    queryClient.ensureQueryData(loopRunDiffOptions(workspaceId, runId, query, true, profileScope)),
  ]);
}
