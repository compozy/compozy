import { useState } from "react";

import { deriveMCPManagementFilter } from "@/systems/settings";

import type { MarketplaceInstalledItem } from "../hooks/use-marketplace-kind-page";
import type { MarketplaceListing } from "../types";

function entryKey(entry: MarketplaceListing): string {
  return `${entry.kind}:${entry.entry_id}`;
}

function installedItemKey(item: MarketplaceInstalledItem): string {
  if (item.activationId) return `bundle:${item.activationId}`;
  if (item.mcpServer) {
    const filter = deriveMCPManagementFilter(item.mcpServer);
    const owner = filter
      ? `${filter.scope}:${filter.target}:${filter.scope === "workspace" ? filter.workspace_id : ""}`
      : `${item.mcpServer.scope}:${item.mcpServer.workspace_id ?? ""}:unknown`;
    return `mcp:${owner}:${item.mcpServer.name}`;
  }
  if (item.skill) return `skill:${item.skill.source}:${item.skill.name}`;
  return `${item.entry.kind}:${item.entry.installed_name ?? item.entry.entry_id}`;
}

function useMarketplacePending() {
  const [pendingEntries, setPendingEntries] = useState<ReadonlyMap<string, number>>(
    () => new Map()
  );
  const [flashIds, setFlashIds] = useState<ReadonlySet<string>>(() => new Set());

  const track = async <T>(key: string, action: () => Promise<T>): Promise<T> => {
    setPendingEntries(current => {
      const next = new Map(current);
      next.set(key, (next.get(key) ?? 0) + 1);
      return next;
    });
    return Promise.resolve()
      .then(action)
      .finally(() => {
        setPendingEntries(current => {
          const next = new Map(current);
          const remaining = (next.get(key) ?? 1) - 1;
          if (remaining > 0) next.set(key, remaining);
          else next.delete(key);
          return next;
        });
      });
  };

  return {
    flash: (entry: MarketplaceListing) => {
      setFlashIds(current => new Set(current).add(entryKey(entry)));
    },
    handleFlashEnd: (entry: MarketplaceListing) => {
      const key = entryKey(entry);
      setFlashIds(current => {
        const next = new Set(current);
        next.delete(key);
        return next;
      });
    },
    isEntryFlashing: (entry: MarketplaceListing) => flashIds.has(entryKey(entry)),
    isEntryPending: (entry: MarketplaceListing) => (pendingEntries.get(entryKey(entry)) ?? 0) > 0,
    isItemPending: (item: MarketplaceInstalledItem) =>
      (pendingEntries.get(installedItemKey(item)) ?? 0) > 0,
    trackEntry: <T>(entry: MarketplaceListing, action: () => Promise<T>) =>
      track(entryKey(entry), action),
    trackItem: <T>(item: MarketplaceInstalledItem, action: () => Promise<T>) =>
      track(installedItemKey(item), action),
  };
}

export { useMarketplacePending };
