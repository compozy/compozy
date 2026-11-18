import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
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
import { marketplaceErrorCode, marketplaceErrorMessage } from "./marketplace-ui";

const NO_FAILURES: MarketplaceEnableFailures = {};

/** Rows with an enable in flight, keyed by destination + origin; each enable clears its own key. */
const enablingLogic = createStoreLogic<
  { keys: ReadonlySet<string> },
  { enableStarted: { key: string }; enableFinished: { key: string } }
>({
  context: { keys: new Set<string>() },
  on: {
    enableStarted: (context, event) =>
      context.keys.has(event.key) ? undefined : { keys: new Set(context.keys).add(event.key) },
    enableFinished: (context, event) => {
      if (!context.keys.has(event.key)) return undefined;
      const keys = new Set(context.keys);
      keys.delete(event.key);
      return { keys };
    },
  },
});

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
  // This hook reports a failed enable itself, with the daemon's code, on the row's own toast.
  const toggle = useToggleExtension({ reportErrors: false });
  const failures = useQuery(marketplaceEnableFailuresOptions(destination)).data ?? NO_FAILURES;
  const enablingStore = useStore(enablingLogic);
  const enabling = useSelector(enablingStore, snapshot => snapshot.context.keys);

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

  /** Keep a recorded failure on its row, now described by the latest enable answer. */
  const updateCause = (
    scope: MarketplaceDestinationScope,
    entry: MarketplaceCatalogListing,
    cause: MarketplaceEnableFailure["cause"]
  ) =>
    queryClient.setQueryData(marketplaceEnableFailuresOptions(scope).queryKey, current => {
      const failure = current?.[marketplaceOriginKey(entry)];
      if (!failure) return current;
      return { ...current, [marketplaceOriginKey(entry)]: { ...failure, cause } };
    });

  const turnOn = async (entry: MarketplaceCatalogListing, target: MarketplaceEnableTarget) => {
    // Captured now: the answer belongs to this destination even if the window switches meanwhile.
    const scope = { profileName: destination.profileName, workspaceId: destination.workspaceId };
    const key = scopedOriginKey(scope, entry);
    // Read the live snapshot, not the render's: a second click before re-render is still a no-op.
    if (enablingStore.getSnapshot().context.keys.has(key)) return;
    enablingStore.trigger.enableStarted({ key });
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
      const code = marketplaceErrorCode(error);
      // A failed Retry replaces the cause the row describes: the daemon's newest answer wins.
      updateCause(scope, entry, { error: message, code: code ?? "extension_enable_failed" });
      // One toast per row: a later Retry replaces it instead of stacking a copy.
      toast.error(message, { id: key, ...(code ? { description: code } : {}) });
      announce(message);
    } finally {
      enablingStore.trigger.enableFinished({ key });
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
