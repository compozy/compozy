import { agentInstancesOptions } from "@/systems/agent";
import { hasCachedRowsForComputer } from "../lib/session-offline-catalog";
import {
  QueryClient,
  QueryClientContext,
  useQueries,
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useContext } from "react";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import {
  computerCanOperateSessions,
  computerReadDetail,
  isComputerCounted,
  mapComputerRead,
  uncountedComputerNames,
  useComputers,
  type ComputerRead,
} from "@/systems/computers";
import type {
  ComputerFixture,
  LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-types";
import {
  profilesListOptions,
  useProfilePerProfileScope,
  type ProfileOwner,
  type ProfilePayload,
  type ProfileScopeParams,
} from "@/systems/profiles";
import {
  computerWorkspacesOptions,
  workspacesListOptions,
  type DataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

import type { fetchSessionFacets } from "../adapters/session-catalog-api";
import { sessionFacetsOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import {
  buildComputerPopulationFilters,
  matchRemoteWorkspaceIds,
} from "../lib/session-list-computer-filter";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionListFilters } from "../types";

export type SessionCatalogFacetsPayload = Awaited<ReturnType<typeof fetchSessionFacets>>;

export interface ComputerSessionsSource {
  /** `"local"` or the remote computer id. */
  key: ComputerId;
  isLocal: boolean;
  name: string;
  computer: LocalComputerFixture | ComputerFixture;
  read: ComputerRead;
  detail: string;
  /** True when the computer allows folder and session mutations (`F11`). */
  canOperate: boolean;
  /** True when the computer's `+` button may create new sessions (`S12`, `F11`). */
  canCreate: boolean;
  facets: SessionCatalogFacetsPayload | undefined;
  /** Facet session count, or `null` when unknown (`S11`). */
  count: number | null;
  needsYou: number;
  working: number;
  laterCount: number;
  workspaces: readonly WorkspacePayload[];
  workspacesLoading: boolean;
  workspacesError: boolean;
  /** Profiles catalog for this computer when Group by Profile is active. */
  profiles?: readonly ProfileOwner[];
  residentWorkspaceIds?: readonly string[];
  /**
   * When the topbar focuses one project (`Q5`), the workspace ids on this computer
   * whose `repository_key` matches that project; `null` under All projects or User.
   */
  matchedWorkspaceIds: readonly string[] | null;
  retry: () => void;
}

export interface ComputerSessionsSourcesResult {
  /** False for one-computer-only accounts, hosted browsers, and phone clients (`S13`, `N6`). */
  hasRemoteComputers: boolean;
  local: LocalComputerFixture;
  /** Enabled computers in section order: this computer first, then registry order (`O4`). */
  sources: readonly ComputerSessionsSource[];
  /** Enabled remote computer views, for pickers and badges. */
  remoteComputers: readonly ComputerFixture[];
  /** Names of computers that did not answer, for `Not counted: <names>` (`R9`). */
  uncountedNames: readonly string[];
}

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

function toProfileOwners(rows: readonly ProfilePayload[] | undefined): ProfileOwner[] {
  return (rows ?? []).map(row => ({
    id: row.id,
    name: row.name,
    color: row.color,
    icon: row.icon,
    emoji: row.emoji,
    archived: row.archived_at != null,
  }));
}

function buildRemoteFacetsQuerySpec(input: {
  computer: ComputerFixture;
  remoteWorkspaces: readonly WorkspacePayload[];
  focusedRepoKey: string | null;
  scope: DataScope | null;
  basePopulation: SessionListFilters;
  enabled: boolean;
}) {
  const { computer, remoteWorkspaces, focusedRepoKey, scope, basePopulation, enabled } = input;
  const matched =
    scope?.kind === "project" ? matchRemoteWorkspaceIds(focusedRepoKey, remoteWorkspaces) : null;
  const remoteFilters: SessionListFilters =
    scope?.kind === "user"
      ? sessionCatalogScopedFilters(scope, basePopulation, { all_profiles: true })
      : scope?.kind === "project"
        ? {
            ...basePopulation,
            all_profiles: true,
            ...(matched && matched.length > 0 ? { workspace_ids: [...matched] } : {}),
          }
        : sessionCatalogScopedFilters({ kind: "all" }, basePopulation, {
            all_profiles: true,
          });
  const canQuery =
    enabled &&
    scope !== null &&
    computer.status === "connected" &&
    (scope?.kind !== "project" || (matched !== null && matched.length > 0));
  return {
    ...sessionFacetsOptions(remoteFilters, computer.id),
    enabled: canQuery,
  };
}

function buildLocalSource(input: {
  localView: LocalComputerFixture;
  localFacetsQuery: UseQueryResult<SessionCatalogFacetsPayload> | undefined;
  registeredWorkspaces: readonly WorkspacePayload[];
  profiles: readonly ProfileOwner[];
  scope: DataScope | null;
}): ComputerSessionsSource {
  const { localView, localFacetsQuery, registeredWorkspaces, profiles, scope } = input;
  const localRead = mapComputerRead({
    status: "connected",
    queryStatus: localFacetsQuery?.status,
    hasCachedRows: localFacetsQuery?.data !== undefined,
    isLocal: true,
  });
  return {
    key: LOCAL_COMPUTER_ID,
    isLocal: true,
    name: localView.name,
    computer: localView,
    read: localRead,
    detail: computerReadDetail(null, localRead, true),
    canOperate: true,
    canCreate: localView.localRunAgents,
    facets: localFacetsQuery?.data,
    count: isComputerCounted(localRead) ? (localFacetsQuery?.data?.facets.all ?? null) : null,
    needsYou: localFacetsQuery?.data?.facets.needs_you ?? 0,
    working: localFacetsQuery?.data?.facets.working ?? 0,
    laterCount: localFacetsQuery?.data?.later ?? 0,
    workspaces: registeredWorkspaces,
    workspacesLoading: false,
    workspacesError: false,
    profiles,
    matchedWorkspaceIds: scope?.kind === "project" ? [scope.workspaceId] : null,
    retry: () => void localFacetsQuery?.refetch(),
  };
}

function resolveRemoteSourceRead({
  computer,
  facetsQuery,
  hasCached,
  noRepoMatch,
  workspacesPending,
  workspacesFailed,
}: {
  computer: ComputerFixture;
  facetsQuery: UseQueryResult<SessionCatalogFacetsPayload> | undefined;
  hasCached: boolean;
  noRepoMatch: boolean;
  workspacesPending: boolean;
  workspacesFailed: boolean;
}): ComputerRead {
  if (computer.status === "connected" && workspacesPending) {
    return workspacesFailed ? "failed" : "connecting";
  }
  if (noRepoMatch) return "ok";
  return mapComputerRead({
    status: computer.status,
    queryStatus: facetsQuery?.status,
    hasCachedRows: hasCached,
    isLocal: false,
  });
}

function buildRemoteSource(input: {
  computer: ComputerFixture;
  facetsQuery: UseQueryResult<SessionCatalogFacetsPayload> | undefined;
  wsQuery: UseQueryResult<WorkspacePayload[]> | undefined;
  profiles: readonly ProfileOwner[];
  scope: DataScope | null;
  focusedRepoKey: string | null;
  queryClient: QueryClient;
}): ComputerSessionsSource {
  const { computer, facetsQuery, wsQuery, profiles, scope, focusedRepoKey, queryClient } = input;
  const workspaces = wsQuery?.data ?? [];
  const matchedWorkspaceIds =
    scope?.kind === "project" ? matchRemoteWorkspaceIds(focusedRepoKey, workspaces) : null;
  const noRepoMatch =
    computer.status === "connected" &&
    scope?.kind === "project" &&
    wsQuery?.isSuccess === true &&
    matchedWorkspaceIds !== null &&
    matchedWorkspaceIds.length === 0;
  const facets =
    scope === null || matchedWorkspaceIds?.length === 0 ? undefined : facetsQuery?.data;
  const hasCached = facets !== undefined || hasCachedRowsForComputer(queryClient, computer.id);
  const read = resolveRemoteSourceRead({
    computer,
    facetsQuery,
    hasCached,
    noRepoMatch,
    workspacesPending: scope?.kind === "project" && wsQuery?.data === undefined,
    workspacesFailed: wsQuery?.isError === true,
  });
  const counted = isComputerCounted(read);
  const count = noRepoMatch ? 0 : counted && facets !== undefined ? facets.facets.all : null;
  const canOperate =
    read === "ok" && computerCanOperateSessions(computer.scopes, !isRemoteComputerId(computer.id));
  return {
    key: computer.id,
    isLocal: false,
    name: computer.name,
    computer,
    read,
    detail: computerReadDetail(computer, read, false),
    canOperate,
    canCreate: canOperate,
    facets,
    count,
    needsYou: noRepoMatch ? 0 : (facets?.facets.needs_you ?? 0),
    working: noRepoMatch ? 0 : (facets?.facets.working ?? 0),
    laterCount: noRepoMatch ? 0 : (facets?.later ?? 0),
    workspaces,
    workspacesLoading: wsQuery?.isLoading ?? false,
    workspacesError: wsQuery?.isError ?? false,
    profiles,
    matchedWorkspaceIds,
    retry: () => {
      void wsQuery?.refetch();
      if (scope !== null && matchedWorkspaceIds?.length !== 0) void facetsQuery?.refetch();
    },
  };
}

function residentWorkspaceIds(
  source: ComputerSessionsSource,
  instances: readonly { workspace_id: string }[] | undefined
) {
  if (!instances) return undefined;
  const matched = source.matchedWorkspaceIds && new Set(source.matchedWorkspaceIds);
  return instances.flatMap(instance =>
    !matched || matched.has(instance.workspace_id) ? [instance.workspace_id] : []
  );
}

function residentQueryOptions(input: {
  enabled: boolean;
  remoteViews: readonly ComputerFixture[];
  profiles: readonly (readonly ProfilePayload[] | undefined)[];
  localProfiles: ReturnType<typeof useProfilePerProfileScope>;
}) {
  if (!input.enabled) return [];
  const { key, names } = input.localProfiles;
  return [
    agentInstancesOptions(key, names),
    ...input.remoteViews.map((computer, index) => {
      const names = input.profiles[index + 1]?.map(profile => profile.name) ?? [];
      return {
        ...agentInstancesOptions("remote-all", names, computer.id),
        enabled:
          names.length > 0 &&
          computer.status === "connected" &&
          computer.scopes.includes("settings:write"),
      };
    }),
  ];
}

function enabledComputerViews(registry: ReturnType<typeof useComputers>): ComputerFixture[] {
  if (!registry.canRead) return [];
  const ids = new Set(registry.enabledComputers.map(computer => computer.id));
  return registry.computerViews.filter(computer => ids.has(computer.id));
}

function sourceWorkspaces(
  options: { scope: DataScope | null; workspaces?: readonly WorkspacePayload[] },
  queryClient: QueryClient,
  hasRemoteComputers: boolean
) {
  const workspaces =
    options.workspaces ??
    (hasRemoteComputers ? (queryClient.getQueryData(workspacesListOptions().queryKey) ?? []) : []);
  const scope = options.scope;
  const project =
    scope?.kind === "project"
      ? workspaces.find(workspace => workspace.id === scope.workspaceId)
      : undefined;
  return { workspaces, focusedRepoKey: project?.repository_key?.trim() || null };
}

/**
 * One facets + workspace-metadata source per enabled computer: this computer
 * first, then registry order (`O4`). A folded or offline section costs at most
 * one facets read (`C7`); counts and needs-you/working roll-ups always come
 * from facets, never from loaded rows (`C8`, `S11`).
 */
export function useComputerSessionsSources(options: {
  scope: DataScope | null;
  lens: ProfileScopeParams;
  shelf: SessionListShelf;
  showAutomation: boolean;
  search?: string;
  worktreeId?: string;
  population?: Pick<SessionListFilters, "children" | "subagents">;
  includeProfiles?: boolean;
  includeResidents?: boolean;
  workspaces?: readonly WorkspacePayload[];
  enabled?: boolean;
}): ComputerSessionsSourcesResult {
  const contextClient = useContext(QueryClientContext);
  const queryClient = contextClient ?? NOOP_QUERY_CLIENT;
  const { scope, lens, shelf, showAutomation, search, worktreeId, population } = options;
  const includeProfiles = options.includeProfiles === true || options.includeResidents === true;
  const localProfiles = useProfilePerProfileScope();
  const enabled = (options.enabled ?? true) && contextClient !== undefined;
  const registry = useComputers({ enabled, passive: true });
  const { localView } = registry;
  const remoteViews = enabledComputerViews(registry);
  const hasRemoteComputers = remoteViews.length > 0;
  const { workspaces: registeredWorkspaces, focusedRepoKey } = sourceWorkspaces(
    options,
    queryClient,
    hasRemoteComputers
  );

  const basePopulation: SessionListFilters = {
    ...buildComputerPopulationFilters({ shelf, showAutomation, search, worktreeId }),
    ...population,
  };

  // Register local facets ONLY when remote computers exist so single-computer QueryCache stays clean.
  const localFacetsFilters = sessionCatalogScopedFilters(scope, basePopulation, lens);
  const localFacetsQuery = useQuery(
    {
      ...sessionFacetsOptions(localFacetsFilters, LOCAL_COMPUTER_ID),
      enabled: hasRemoteComputers && enabled && scope !== null,
    },
    queryClient
  );

  // Remote workspace catalogs for name resolution (P1) and repository_key matching (Q5).
  const remoteWorkspaceQueries = useQueries(
    {
      queries: remoteViews.map(computer =>
        computerWorkspacesOptions(computer.id, enabled && computer.status === "connected")
      ),
    },
    queryClient
  );

  // Remote facets reads: remote computers read all of their own profiles (Q4).
  const remoteFacetsQueries = useQueries(
    {
      queries: remoteViews.map((computer, index) =>
        buildRemoteFacetsQuerySpec({
          computer,
          remoteWorkspaces: remoteWorkspaceQueries[index]?.data ?? [],
          focusedRepoKey,
          scope,
          basePopulation,
          enabled,
        })
      ),
    },
    queryClient
  );

  // Per-computer profile catalogs when Group by Profile is active across computers.
  const profileOptions: ReturnType<typeof profilesListOptions>[] =
    hasRemoteComputers && includeProfiles
      ? [
          profilesListOptions(enabled),
          ...remoteViews.map(computer =>
            profilesListOptions(enabled && computer.status === "connected", computer.id)
          ),
        ]
      : [];
  const profileQueries = useQueries({ queries: profileOptions }, queryClient);

  const residentQueries = useQueries(
    {
      queries: residentQueryOptions({
        enabled: enabled && hasRemoteComputers && options.includeResidents === true,
        remoteViews,
        profiles: profileQueries.map(query => query.data),
        localProfiles,
      }),
    },
    queryClient
  );

  if (!hasRemoteComputers) {
    return {
      hasRemoteComputers: false,
      local: localView,
      sources: [],
      remoteComputers: [],
      uncountedNames: [],
    };
  }

  const localSource = buildLocalSource({
    localView,
    localFacetsQuery,
    registeredWorkspaces,
    profiles: toProfileOwners(profileQueries[0]?.data),
    scope,
  });

  const remoteSources = remoteViews.map((computer, index) =>
    buildRemoteSource({
      computer,
      facetsQuery: remoteFacetsQueries[index],
      wsQuery: remoteWorkspaceQueries[index],
      profiles: toProfileOwners(profileQueries[index + 1]?.data),
      scope,
      focusedRepoKey,
      queryClient,
    })
  );

  const sources = [localSource, ...remoteSources].map((source, index) => ({
    ...source,
    residentWorkspaceIds: residentWorkspaceIds(source, residentQueries[index]?.data?.instances),
  }));
  return {
    hasRemoteComputers: true,
    local: localView,
    sources,
    remoteComputers: remoteViews,
    uncountedNames: uncountedComputerNames(sources),
  };
}
