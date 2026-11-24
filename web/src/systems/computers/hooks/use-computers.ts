import { useContext, useEffect } from "react";
import { QueryClient, QueryClientContext, useQuery } from "@tanstack/react-query";

import { accountKeys, type AccountStatusPayload } from "@/systems/account";

import { openComputersStream } from "../adapters/computers-api";
import { computerRegistryKeys } from "../lib/query-keys";
import { computerDetailOptions, computersListOptions } from "../lib/query-options";
import { toComputerView, toLocalComputerView } from "../lib/computer-view";
import type { ComputerPayload } from "../types";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

/**
 * Reads `/api/computers` and stays synchronized with `/api/computers/stream` (`computer.changed`).
 *
 * When `passive` is true, observes the cache populated by the shell-level stream without
 * triggering a standalone HTTP request on mount (avoids redundant fetches in list/palette views).
 */
export function useComputers(
  options: { enabled?: boolean; stream?: boolean; passive?: boolean } = {}
) {
  const contextClient = useContext(QueryClientContext);
  const queryClient = contextClient ?? NOOP_QUERY_CLIENT;
  const cachedAccount = contextClient?.getQueryData<AccountStatusPayload>(accountKeys.status());
  const signedOutInCache = cachedAccount !== undefined && !cachedAccount.signed_in;
  const enabled = (options.enabled ?? true) && contextClient !== undefined && !signedOutInCache;
  const passive = options.passive ?? false;
  const stream = options.stream ?? false;
  const query = useQuery(computersListOptions(enabled && !passive), queryClient);

  useEffect(() => {
    if (!enabled || !stream || !contextClient) return;
    return openComputersStream({
      onComputerChanged: () => {
        void contextClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
      onReset: () => {
        void contextClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
    });
  }, [contextClient, enabled, stream]);

  const computers = query.data?.computers ?? [];
  const enabledComputers = computers.filter(computer => computer.enabled);
  const local = query.data?.local;

  return {
    ...query,
    computers,
    enabledComputers,
    computerViews: computers.map(toComputerView),
    local,
    localView: toLocalComputerView(local),
  };
}

export function useComputer(id: string | null | undefined, enabled = true) {
  const contextClient = useContext(QueryClientContext);
  const normalized = id?.trim() ?? "";
  return useQuery(
    computerDetailOptions(
      normalized,
      contextClient !== undefined && enabled && normalized !== "" && normalized !== "local"
    ),
    contextClient ?? NOOP_QUERY_CLIENT
  );
}

export function findComputerById(
  computers: readonly ComputerPayload[],
  computerId: string | null | undefined
): ComputerPayload | undefined {
  if (!computerId || computerId === "local") return undefined;
  return computers.find(computer => computer.id === computerId);
}
