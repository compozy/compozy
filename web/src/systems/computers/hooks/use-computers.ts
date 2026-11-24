import { useContext, useEffect, useMemo } from "react";
import { QueryClient, QueryClientContext, useQuery } from "@tanstack/react-query";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { accountKeys, type AccountStatusPayload } from "@/systems/account";

import { openComputersStream } from "../adapters/computers-api";
import { computerRegistryKeys } from "../lib/query-keys";
import { computerDetailOptions, computersListOptions } from "../lib/query-options";
import { toComputerView, toLocalComputerView } from "../lib/computer-view";
import type { ComputerPayload } from "../types";
import { useComputersHostResources } from "./use-host-resources";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

const EMPTY_COMPUTERS: ComputerPayload[] = [];

/**
 * Reads `/api/computers` and stays synchronized with `/api/computers/stream` (`computer.changed`).
 *
 * When `passive` is true, observes the cache populated by the shell-level stream without
 * triggering a standalone HTTP request on mount (avoids redundant fetches in list/palette views).
 */
export function useComputers(
  options: {
    enabled?: boolean;
    stream?: boolean;
    passive?: boolean;
    withHostResources?: boolean;
  } = {}
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

  const computers = query.data?.computers ?? EMPTY_COMPUTERS;
  const enabledComputers = useMemo(
    () => computers.filter(computer => computer.enabled),
    [computers]
  );
  const local = query.data?.local;
  const loadBalancingEnabled = Boolean(local?.load_balancing);
  const shouldPollResources =
    Boolean(options.withHostResources) && enabled && (computers.length > 0 || loadBalancingEnabled);
  const hostResources = useComputersHostResources(computers, {
    enabled: shouldPollResources,
    includeLocal: local?.run_agents ?? true,
  });

  const computerViews = useMemo(
    () =>
      computers.map(computer => toComputerView(computer, { load: hostResources.get(computer.id) })),
    [computers, hostResources]
  );
  const localView = useMemo(
    () => toLocalComputerView(local, { load: hostResources.get(LOCAL_COMPUTER_ID) }),
    [local, hostResources]
  );

  return {
    ...query,
    computers,
    enabledComputers,
    computerViews,
    local,
    localView,
    loadBalancingEnabled,
    localLoadBalancingWeight: local?.load_balancing_weight ?? 50,
    hostResources,
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
