import { useState } from "react";
import { toast } from "@compozy/ui";

import { useToggleExtension } from "@/systems/extensions";

import { marketplaceOriginKey } from "../lib/marketplace-installed-view";
import type { ExtensionEnableError, MarketplaceCatalogListing } from "../types";
import { marketplaceErrorMessage } from "./marketplace-ui";

/** The installed instance an Add produced, kept with the cause its enable reported. */
interface MarketplaceEnableFailure {
  name: string;
  profileName: string;
  workspaceId: string | null;
  cause: ExtensionEnableError;
}

/** The installed instance a Turn on / Retry addresses. */
interface MarketplaceEnableTarget {
  name: string;
  profileName: string;
  workspaceId: string | null;
}

/**
 * Add installs and enables in one daemon operation; when only the enable fails the extension stays
 * installed (ADR-003). This keeps that cause on the row until a Retry or Turn on succeeds, keyed by
 * the catalog origin like every other row state.
 */
export function useMarketplaceEnableRecovery() {
  const toggle = useToggleExtension();
  const [failures, setFailures] = useState<ReadonlyMap<string, MarketplaceEnableFailure>>(
    () => new Map()
  );
  const [enabling, setEnabling] = useState<ReadonlySet<string>>(() => new Set());

  const record = (entry: MarketplaceCatalogListing, failure: MarketplaceEnableFailure | null) =>
    setFailures(current => {
      const next = new Map(current);
      if (failure) next.set(marketplaceOriginKey(entry), failure);
      else next.delete(marketplaceOriginKey(entry));
      return next;
    });

  const turnOn = async (entry: MarketplaceCatalogListing, target: MarketplaceEnableTarget) => {
    const key = marketplaceOriginKey(entry);
    if (enabling.has(key)) return;
    setEnabling(current => new Set(current).add(key));
    try {
      await toggle.mutateAsync({
        enabled: true,
        name: target.name,
        profileName: target.profileName,
        workspaceId: target.workspaceId,
      });
      record(entry, null);
    } catch (error) {
      toast.error(marketplaceErrorMessage(error, `Couldn't turn on ${entry.name}`));
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
    failureFor: (entry: MarketplaceCatalogListing) =>
      failures.get(marketplaceOriginKey(entry)) ?? null,
    isEnabling: (entry: MarketplaceCatalogListing) => enabling.has(marketplaceOriginKey(entry)),
    turnOn,
    /** Retry the enable an Add could not finish, on the instance that Add installed. */
    retry: (entry: MarketplaceCatalogListing) => {
      const failure = failures.get(marketplaceOriginKey(entry));
      if (failure) void turnOn(entry, failure);
    },
  };
}

export type { MarketplaceEnableFailure, MarketplaceEnableTarget };
