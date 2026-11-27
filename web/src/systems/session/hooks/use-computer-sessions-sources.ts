import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";

import {
  isRemoteComputerId,
  LOCAL_COMPUTER_ID,
  type ComputerId,
} from "@/lib/computer-scope";
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
import type { ProfileScopeParams } from "@/systems/profiles";
import {
  computerWorkspacesOptions,
  useActiveWorkspace,
  type DataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

import type { fetchSessionFacets } from "../adapters/session-catalog-api";
import { sessionKeys } from "../lib/query-keys";
import { sessionFacetsOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import {
  buildComputerPopulationFilters,
  matchRemoteWorkspaceIds,
} from "../lib/session-list-computer-filter";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionCatalogPageResponse, SessionListFilters } from "../types";

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

/** True when any catalog page for `computerId` holds at least one session in memory (Rule 23). */
function hasCachedRowsForComputer(queryClient: QueryClient, computerId: ComputerId): boolean {
  const keys = sessionKeys.forComputer(computerId);
  const queries = queryClient.getQueriesData<{ pages?: SessionCatalogPageResponse[] }>({
    queryKey: keys.lists(),
  });
  for (const [, data] of queries) {
    if (!data?.pages) continue;
    for (const page of data.pages) {
      if (page.sessions.length > 0) return true;
    }
  }
  return false;
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
  enabled?: boolean;
}): ComputerSessionsSourcesResult {
  const queryClient = useQueryClient();
  const { scope, lens, shelf, showAutomation, search, worktreeId, enabled = true } = options;
  const { canRead, enabledComputers, computerViews, localView } = useComputers({ passive: true });
  const { registeredWorkspaces } = useActiveWorkspace();

  const enabledRemoteIds = new Set(enabledComputers.map(computer => computer.id));
  const remoteViews = canRead
    ? computerViews.filter(computer => enabledRemoteIds.has(computer.id))
    : [];
  const hasRemoteComputers = canRead && remoteViews.length > 0;

  const focusedProject =
    scope?.kind === "project"
      ? registeredWorkspaces.find(workspace => workspace.id === scope.workspaceId)
      : undefined;
  const focusedRepoKey =
    scope?.kind === "project" ? (focusedProject?.repository_key?.trim() || null) : null;

  const basePopulation = buildComputerPopulationFilters({
    shelf,
    showAutomation,
    search,
    worktreeId,
  });

  // Local facets read (reuses the window's active scope and profile lens).
  const localFacetsFilters = sessionCatalogScopedFilters(scope, basePopulation, lens);
  const localFacetsQuery = useQuery({
    ...sessionFacetsOptions(localFacetsFilters, LOCAL_COMPUTER_ID),
    enabled: enabled && scope !== null && hasRemoteComputers,
  });

  // Remote workspace catalogs for name resolution (P1) and repository_key matching (Q5).
  const remoteWorkspaceQueries = useQueries({
    queries: remoteViews.map(computer =>
      computerWorkspacesOptions(
        computer.id,
        enabled && hasRemoteComputers && computer.status === "connected"
      )
    ),
  });

  // Remote facets reads: remote computers read all of their own profiles (Q4).
  const remoteFacetsQueries = useQueries({
    queries: remoteViews.map((computer, index) => {
      const remoteWorkspaces = remoteWorkspaceQueries[index]?.data ?? [];
      const matched =
        scope?.kind === "project"
          ? matchRemoteWorkspaceIds(focusedRepoKey, remoteWorkspaces)
          : null;
      const remoteFilters: SessionListFilters =
        scope?.kind === "user"
          ? sessionCatalogScopedFilters(scope, basePopulation, { all_profiles: true })
          : scope?.kind === "project"
            ? {
                ...basePopulation,
                all_profiles: true,
                ...(matched && matched.length > 0 ? { workspace_ids: [...matched] } : {}),
              }
            : sessionCatalogScopedFilters({ kind: "all" }, basePopulation, { all_profiles: true });
      const canQuery =
        enabled &&
        hasRemoteComputers &&
        computer.status === "connected" &&
        (scope?.kind !== "project" || (matched !== null && matched.length > 0));
      return {
        ...sessionFacetsOptions(remoteFilters, computer.id),
        enabled: canQuery,
      };
    }),
  });

  if (!hasRemoteComputers) {
    return {
      hasRemoteComputers: false,
      local: localView,
      sources: [],
      remoteComputers: [],
      uncountedNames: [],
    };
  }

  const localRead = mapComputerRead({
    status: "connected",
    queryStatus: localFacetsQuery.status,
    hasCachedRows: localFacetsQuery.data !== undefined,
    isLocal: true,
  });
  const localSource: ComputerSessionsSource = {
    key: LOCAL_COMPUTER_ID,
    isLocal: true,
    name: localView.name,
    computer: localView,
    read: localRead,
    detail: computerReadDetail(null, localRead, true),
    canOperate: true,
    canCreate: localView.localRunAgents,
    facets: localFacetsQuery.data,
    count: isComputerCounted(localRead) ? (localFacetsQuery.data?.facets.all ?? null) : null,
    needsYou: localFacetsQuery.data?.facets.needs_you ?? 0,
    working: localFacetsQuery.data?.facets.working ?? 0,
    laterCount: localFacetsQuery.data?.later ?? 0,
    workspaces: registeredWorkspaces,
    workspacesLoading: false,
    workspacesError: false,
    matchedWorkspaceIds: scope?.kind === "project" ? [scope.workspaceId] : null,
    retry: () => void localFacetsQuery.refetch(),
  };

  const remoteSources: ComputerSessionsSource[] = remoteViews.map((computer, index) => {
    const facetsQuery = remoteFacetsQueries[index];
    const wsQuery = remoteWorkspaceQueries[index];
    const workspaces = wsQuery?.data ?? [];
    const matchedWorkspaceIds =
      scope?.kind === "project" ? matchRemoteWorkspaceIds(focusedRepoKey, workspaces) : null;
    const noRepoMatch =
      scope?.kind === "project" &&
      wsQuery?.isSuccess === true &&
      matchedWorkspaceIds !== null &&
      matchedWorkspaceIds.length === 0;
    const hasCached =
      facetsQuery?.data !== undefined || hasCachedRowsForComputer(queryClient, computer.id);
    const read = noRepoMatch
      ? "ok"
      : mapComputerRead({
          status: computer.status,
          queryStatus: facetsQuery?.status,
          hasCachedRows: hasCached,
          isLocal: false,
        });
    const counted = isComputerCounted(read);
    const count = noRepoMatch
      ? 0
      : counted && facetsQuery?.data !== undefined
        ? facetsQuery.data.facets.all
        : null;
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
      facets: noRepoMatch ? undefined : facetsQuery?.data,
      count,
      needsYou: noRepoMatch ? 0 : (facetsQuery?.data?.facets.needs_you ?? 0),
      working: noRepoMatch ? 0 : (facetsQuery?.data?.facets.working ?? 0),
      laterCount: noRepoMatch ? 0 : (facetsQuery?.data?.later ?? 0),
      workspaces,
      workspacesLoading: wsQuery?.isLoading ?? false,
      workspacesError: wsQuery?.isError ?? false,
      matchedWorkspaceIds,
      retry: () => {
        void wsQuery?.refetch();
        void facetsQuery?.refetch();
      },
    };
  });

  const sources = [localSource, ...remoteSources];
  return {
    hasRemoteComputers: true,
    local: localView,
    sources,
    remoteComputers: remoteViews,
    uncountedNames: uncountedComputerNames(sources),
  };
}
