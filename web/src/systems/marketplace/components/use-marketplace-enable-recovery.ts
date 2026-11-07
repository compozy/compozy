import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "@compozy/ui";

import { useToggleExtension } from "@/systems/extensions";

import { marketplaceOriginKey } from "../lib/marketplace-installed-view";
import { marketplaceEnableFailuresOptions } from "../lib/query-options";
import type {
  MarketplaceCatalogListing,
  MarketplaceDestinationScope,
  MarketplaceEnableFailure,
  MarketplaceEnableFailures,
  MarketplaceEnableTarget,
} from "../types";
import { marketplaceErrorMessage } from "./marketplace-ui";

const NO_FAILURES: MarketplaceEnableFailures = {};

/** Identity of one row in one destination: the same origin elsewhere is another row. */
function scopedOriginKey(scope: MarketplaceDestinationScope, entry: MarketplaceCatalogListing) {
  return JSON.stringify([
    marketplaceEnableFailuresOptions(scope).queryKey,
    marketplaceOriginKey(entry),
  ]);
}

/**
 * Add installs and enables in one daemon operation; when only the enable fails the extension stays
 * installed (ADR-003). This keeps that cause on the row until a Retry or Turn on succeeds. Failures
 * live in the query cache under the destination (workspace + profile) and the catalog origin, so
 * they survive navigation between Browse and a category page, and every write lands in the
 * destination its action started in — a late answer never paints another workspace's row.
 */
export function useMarketplaceEnableRecovery(
  destination: MarketplaceDestinationScope,
  announce: (message: string) => void
) {
  const queryClient = useQueryClient();
  const toggle = useToggleExtension();
  const failures = useQuery(marketplaceEnableFailuresOptions(destination)).data ?? NO_FAILURES;
  const [enabling, setEnabling] = useState<ReadonlySet<string>>(() => new Set());

  const record = (
    scope: MarketplaceDestinationScope,
    entry: MarketplaceCatalogListing,
    failure: MarketplaceEnableFailure | null
  ) =>
    queryClient.setQueryData(marketplaceEnableFailuresOptions(scope).queryKey, current => {
      const next = { ...current };
      if (failure) next[marketplaceOriginKey(entry)] = failure;
      else delete next[marketplaceOriginKey(entry)];
      return next;
    });

  const turnOn = async (entry: MarketplaceCatalogListing, target: MarketplaceEnableTarget) => {
    // Captured now: the answer belongs to this destination even if the window switches meanwhile.
    const scope = { profileName: destination.profileName, workspaceId: destination.workspaceId };
    const key = scopedOriginKey(scope, entry);
    if (enabling.has(key)) return;
    setEnabling(current => new Set(current).add(key));
    try {
      await toggle.mutateAsync({
        enabled: true,
        name: target.name,
        profileName: target.profileName,
        workspaceId: target.workspaceId,
      });
      record(scope, entry, null);
      announce(`${entry.name} turned on`);
    } catch (error) {
      const message = marketplaceErrorMessage(error, `Couldn't turn on ${entry.name}`);
      toast.error(message);
      announce(message);
    } finally {
      setEnabling(current => {
        const next = new Set(current);
        next.delete(key);
        return next;
      });
    }
  };

  return {
    record,
    failureFor: (entry: MarketplaceCatalogListing) => failures[marketplaceOriginKey(entry)] ?? null,
    isEnabling: (entry: MarketplaceCatalogListing) =>
      enabling.has(scopedOriginKey(destination, entry)),
    turnOn,
    /** Retry the enable an Add could not finish, on the instance that Add installed. */
    retry: (entry: MarketplaceCatalogListing) => {
      const failure = failures[marketplaceOriginKey(entry)];
      if (failure) void turnOn(entry, failure);
    },
  };
}
