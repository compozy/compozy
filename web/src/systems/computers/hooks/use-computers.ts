import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { openComputersStream } from "../adapters/computers-api";
import { computerRegistryKeys } from "../lib/query-keys";
import { computerDetailOptions, computersListOptions } from "../lib/query-options";
import { toComputerView, toLocalComputerView } from "../lib/computer-view";
import type { ComputerPayload } from "../types";

/**
 * Reads `/api/computers` and stays synchronized with `/api/computers/stream` (`computer.changed`).
 */
export function useComputers(options: { enabled?: boolean; stream?: boolean } = {}) {
  const enabled = options.enabled ?? true;
  const stream = options.stream ?? false;
  const queryClient = useQueryClient();
  const query = useQuery(computersListOptions(enabled));

  useEffect(() => {
    if (!enabled || !stream) return;
    return openComputersStream({
      onComputerChanged: () => {
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
      onReset: () => {
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
    });
  }, [enabled, queryClient, stream]);

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
  const normalized = id?.trim() ?? "";
  return useQuery(computerDetailOptions(normalized, enabled && normalized !== "" && normalized !== "local"));
}

export function findComputerById(
  computers: readonly ComputerPayload[],
  computerId: string | null | undefined
): ComputerPayload | undefined {
  if (!computerId || computerId === "local") return undefined;
  return computers.find(computer => computer.id === computerId);
}
