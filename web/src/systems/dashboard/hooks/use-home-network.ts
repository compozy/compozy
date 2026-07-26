import { useQuery } from "@tanstack/react-query";

import { networkStatusOptions, useNetworkUsage } from "@/systems/network";

import type { HomeNetworkRow } from "../lib/home-network";

export interface HomeNetworkModel {
  rows: HomeNetworkRow[];
}

export function useHomeNetwork(messagesToday: number | undefined): HomeNetworkModel {
  const statusQuery = useQuery(networkStatusOptions());
  const usageQuery = useNetworkUsage();

  const status = statusQuery.data;
  const rows: HomeNetworkRow[] = [];

  if (status) {
    const peers = status.local_peers ?? 0;
    rows.push({
      key: "peers",
      label: "Peers online",
      value: String(peers),
      tone: peers > 0 ? "success" : undefined,
    });
    if (status.open_work_items !== undefined) {
      rows.push({ key: "work", label: "Open work", value: String(status.open_work_items) });
    }
    if (messagesToday !== undefined) {
      rows.push({
        key: "messages",
        label: "Messages today",
        value: String(messagesToday),
        mono: true,
      });
    }
    rows.push({
      key: "channels",
      label: "Channels",
      value: String(status.channels ?? 0),
      mono: true,
    });
  }

  const budgetPayload = usageQuery.data?.pages[0]?.budget;
  if (budgetPayload) {
    rows.push({
      key: "wakes",
      label: "Wakes used",
      value: String(budgetPayload.wakes_used),
      mono: true,
    });
  }

  return { rows };
}
