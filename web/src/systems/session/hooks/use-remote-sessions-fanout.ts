import { QueryClient, QueryClientContext, useQueries } from "@tanstack/react-query";
import { useContext } from "react";

import {
  COMPUTER_SESSION_PAGE_LIMIT,
  iconKindForPlatform,
  mergeSessionPages,
  normalizeComputerPlatform,
  useComputers,
  type ComputerPayload,
  type ComputerSessionPageInput,
  type MergedSessionRow,
  type ScopedComputerFailureItem,
} from "@/systems/computers";

import type { fetchSessionCatalogPage } from "../adapters/session-catalog-api";
import { sessionCatalogFirstPageOptions } from "../lib/session-catalog-options";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import { sessionRowKey } from "../lib/session-row-ref";
import type { SessionListFilters, SessionPayload } from "../types";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

interface RemoteQuerySlice {
  data?: Awaited<ReturnType<typeof fetchSessionCatalogPage>>;
  isLoading: boolean;
  isError: boolean;
  refetch: () => Promise<unknown>;
}

export interface RemoteSessionsFanoutResult {
  hasRemoteComputers: boolean;
  remoteSessions: MergedSessionRow<SessionPayload>[];
  computerBySessionId: ReadonlyMap<string, SessionRowComputerInfo>;
  loadingComputers: readonly { id: string; name: string }[];
  failedComputers: readonly ScopedComputerFailureItem[];
}

function projectRemoteFanout(
  remoteComputers: readonly ComputerPayload[],
  queries: readonly RemoteQuerySlice[]
): Omit<RemoteSessionsFanoutResult, "hasRemoteComputers"> {
  const pageInputs: ComputerSessionPageInput<SessionPayload>[] = [];
  const loadingComputers: { id: string; name: string }[] = [];
  const failedComputers: ScopedComputerFailureItem[] = [];

  for (let index = 0; index < remoteComputers.length; index++) {
    const computer = remoteComputers[index];
    const query = queries[index];
    if (!computer || !query) continue;
    const isOffline = computer.status === "offline";
    if (!isOffline && query.isLoading) {
      loadingComputers.push({ id: computer.id, name: computer.name });
    }
    if (!isOffline && query.isError && !query.data) {
      failedComputers.push({
        id: computer.id,
        name: computer.name,
        retry: () => {
          void query.refetch();
        },
      });
    }
    pageInputs.push({
      computer: { id: computer.id, name: computer.name, status: computer.status },
      sessions: query.data?.sessions ?? [],
      nextCursor: query.data?.page.next_cursor ?? null,
      hasMore: Boolean(query.data?.page.has_more),
    });
  }

  const remoteSessions = mergeSessionPages(pageInputs);
  const computerBySessionId = new Map<string, SessionRowComputerInfo>();
  const computerMetaById = new Map(remoteComputers.map(computer => [computer.id, computer]));
  for (const row of remoteSessions) {
    const platform = normalizeComputerPlatform(computerMetaById.get(row.computerId)?.platform);
    const info: SessionRowComputerInfo = {
      id: row.computerId,
      name: row.computerName,
      platform,
      iconKind: iconKindForPlatform(platform),
      offline: row.offline,
    };
    computerBySessionId.set(
      sessionRowKey({ computerId: row.computerId, sessionId: row.session.id }),
      info
    );
    computerBySessionId.set(row.session.id, info);
  }

  return { remoteSessions, computerBySessionId, loadingComputers, failedComputers };
}

/**
 * First-page fan-out across enabled remote computers (`limit = 50` per computer),
 * retained for the command palette (`use-os-palette-entities.ts`).
 */
export function useRemoteSessionsFanout(options: {
  enabled?: boolean;
  filters?: SessionListFilters;
}): RemoteSessionsFanoutResult {
  const contextClient = useContext(QueryClientContext);
  const enabled = (options.enabled ?? true) && contextClient !== undefined;
  const filters: SessionListFilters = {
    ...options.filters,
    limit: options.filters?.limit ?? COMPUTER_SESSION_PAGE_LIMIT,
  };
  const computers = useComputers({ enabled, passive: true });
  const remoteComputers = computers.enabledComputers;

  const queries = useQueries(
    {
      queries: remoteComputers.map(computer => ({
        ...sessionCatalogFirstPageOptions(filters, computer.id),
        enabled: enabled && computer.status === "connected",
      })),
    },
    contextClient ?? NOOP_QUERY_CLIENT
  );

  const projection = projectRemoteFanout(remoteComputers, queries);
  return {
    hasRemoteComputers: remoteComputers.length > 0,
    ...projection,
  };
}
