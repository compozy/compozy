import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import { QueryClient, QueryClientContext, useQuery } from "@tanstack/react-query";
import { useContext } from "react";

import { isComputerCounted } from "@/systems/computers";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import { sessionCatalogFirstPageOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import { buildComputerPopulationFilters } from "../lib/session-list-computer-filter";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionCatalogPageResponse, SessionListFilters } from "../types";
import { type ComputerSessionsSource } from "./use-computer-sessions-sources";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

const DEFAULT_LOCAL_LENS: ProfileScopeParams = { all_profiles: true };

function searchProbeFilters(input: {
  source: ComputerSessionsSource;
  scope: DataScope | null;
  search: string;
  shelf: SessionListShelf;
  showAutomation: boolean;
  worktreeId?: string;
  localLens: ProfileScopeParams;
}): { filters: SessionListFilters; skip: boolean } {
  const { source, scope, search, shelf, showAutomation, worktreeId, localLens } = input;
  const population = buildComputerPopulationFilters({
    shelf,
    showAutomation,
    search,
    worktreeId,
    excludePinned: true,
  });
  const base: SessionListFilters = { ...population, q: search.trim(), skip_total: false };
  if (source.isLocal) {
    return {
      filters: sessionCatalogScopedFilters(scope ?? { kind: "all" }, base, localLens),
      skip: false,
    };
  }
  if (scope?.kind === "project") {
    const matched = source.matchedWorkspaceIds ?? [];
    if (matched.length === 0) {
      return { filters: { ...base, all_workspaces: true, all_profiles: true }, skip: true };
    }
    return {
      filters: { ...base, all_workspaces: true, workspace_ids: [...matched], all_profiles: true },
      skip: false,
    };
  }
  return {
    filters: sessionCatalogScopedFilters(scope?.kind === "user" ? scope : { kind: "all" }, base, {
      all_profiles: true,
    }),
    skip: false,
  };
}

/**
 * Resolves search visibility and filtered header count for one Separate computer section:
 * - online sections with 0 matches hide (`Opus 6`, `Astra 15`),
 * - offline sections filter cached rows from the unfiltered cache and hide at 0 matches.
 */
export function useComputerSectionSearch(input: {
  source: ComputerSessionsSource;
  scope: DataScope | null;
  localLens?: ProfileScopeParams;
  search: string;
  shelf: SessionListShelf;
  showAutomation: boolean;
  worktreeId?: string;
}): { hidden: boolean; headerCount: number | null } {
  const contextClient = useContext(QueryClientContext);
  const filtering = input.search.trim() !== "";
  const probe = searchProbeFilters({
    ...input,
    localLens: input.localLens ?? DEFAULT_LOCAL_LENS,
  });
  const onlineSearchEnabled =
    filtering &&
    input.scope !== null &&
    input.source.read === "ok" &&
    !probe.skip &&
    contextClient !== undefined;
  const searchQuery = useQuery(
    {
      ...sessionCatalogFirstPageOptions(probe.filters, input.source.key),
      enabled: onlineSearchEnabled,
    },
    contextClient ?? NOOP_QUERY_CLIENT
  );

  return computerSectionSearchResult({
    source: input.source,
    scope: input.scope,
    filtering,
    probe,
    contextClient,
    page: onlineSearchEnabled ? searchQuery.data : undefined,
  });
}

function computerSectionSearchResult({
  source,
  scope,
  filtering,
  probe,
  contextClient,
  page,
}: {
  source: ComputerSessionsSource;
  scope: DataScope | null;
  filtering: boolean;
  probe: ReturnType<typeof searchProbeFilters>;
  contextClient: QueryClient | undefined;
  page: SessionCatalogPageResponse | undefined;
}): { hidden: boolean; headerCount: number | null } {
  if (!filtering) {
    return {
      hidden: false,
      headerCount: isComputerCounted(source.read) ? source.count : null,
    };
  }
  if (scope === null) return { hidden: false, headerCount: null };
  if (source.read === "offline") {
    const matches = probe.skip
      ? []
      : readCachedOfflineRowsForComputer(contextClient, source.key, probe.filters);
    return { hidden: matches.length === 0, headerCount: matches.length };
  }
  if (source.read === "ok") {
    if (probe.skip) return { hidden: true, headerCount: 0 };
    if (page !== undefined) {
      const matched = page.page.total ?? page.sessions.length;
      return { hidden: matched === 0, headerCount: matched };
    }
    return { hidden: source.count === 0, headerCount: source.count };
  }
  return { hidden: false, headerCount: null };
}
