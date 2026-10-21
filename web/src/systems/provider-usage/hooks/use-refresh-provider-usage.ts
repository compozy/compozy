"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { refreshProviderUsage } from "../adapters/provider-usage-api";
import { providerUsageKeys } from "../lib/query-keys";
import type { ProviderUsageRowAction } from "../lib/status";
import { PROVIDER_USAGE_OPEN_MAX_AGE_SECONDS, targetsNeedingOpenRefresh } from "../lib/view-model";
import type { ProviderUsageEnvelope, ProviderUsageRefreshIntent } from "../types";

export interface ProviderUsageRefreshInput {
  intent: ProviderUsageRefreshIntent;
  providerIds?: string[];
  maxAgeSeconds?: number;
}

/**
 * Sends a refresh for the profile. The daemon answers with the envelope, but
 * the list read stays the single writer: settling re-reads it, as the stream
 * would on the revision bump.
 */
export function useRefreshProviderUsage(profile: string) {
  const queryClient = useQueryClient();
  const scoped = profile.trim();
  return useMutation({
    mutationFn: ({ intent, providerIds, maxAgeSeconds }: ProviderUsageRefreshInput) =>
      refreshProviderUsage({
        profile: scoped,
        intent,
        ...(providerIds && providerIds.length > 0 ? { provider_ids: providerIds } : {}),
        ...(maxAgeSeconds === undefined ? {} : { max_age_seconds: maxAgeSeconds }),
      }),
    onError: (error, { intent }) => {
      if (intent === "explicit") {
        toast.error("Couldn't refresh provider usage", { description: error.message });
      }
    },
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: providerUsageKeys.envelope(scoped), exact: true }),
  });
}

/**
 * Popover open: a background refresh for targets older than 60 s, never an
 * explicit one, so opening can neither prompt nor bypass a cooldown. Rows
 * render from cache meanwhile; nothing is sent when every row is fresh.
 */
export function useProviderUsageOpenRefresh(profile: string): () => void {
  const queryClient = useQueryClient();
  const refresh = useRefreshProviderUsage(profile);
  const scoped = profile.trim();
  return () => {
    const envelope = queryClient.getQueryData<ProviderUsageEnvelope>(
      providerUsageKeys.envelope(scoped)
    );
    if (!envelope || !envelope.enabled) return;
    const providerIds = targetsNeedingOpenRefresh(envelope, Date.now());
    if (providerIds.length === 0) return;
    refresh.mutate({
      intent: "background",
      providerIds,
      maxAgeSeconds: PROVIDER_USAGE_OPEN_MAX_AGE_SECONDS,
    });
  };
}

export interface UseProviderUsageActionsOptions {
  /**
   * Saves `provider_usage.providers.claude.keychain_access = true` through the
   * settings apply lifecycle. The OS layer owns settings, so it supplies this
   * and reports a failed save itself. The daemon owns the consent collection.
   */
  allowKeychain?: (providerId: string) => Promise<void>;
  /**
   * Opens the provider usage settings for a row whose fix is configuration.
   * The OS layer owns windows; a `configure` action never sends a refresh.
   */
  configure?: (providerId: string) => void;
}

export interface ProviderUsageActions {
  /** Header Refresh: every target, explicit intent. */
  refreshAll: () => void;
  /** Status-line actions: one target, explicit intent; `configure` opens settings instead. */
  runAction: (providerId: string, action: ProviderUsageRowAction) => void;
  /** An explicit header refresh is in flight. */
  refreshing: boolean;
  /** Providers with an explicit row action in flight. */
  pendingProviderIds: ReadonlySet<string>;
}

/** Operator actions: the only callers that send `intent: "explicit"`. */
export function useProviderUsageActions(
  profile: string,
  { allowKeychain, configure }: UseProviderUsageActionsOptions = {}
): ProviderUsageActions {
  const queryClient = useQueryClient();
  const all = useRefreshProviderUsage(profile);
  const row = useRefreshProviderUsage(profile);
  const rowPending = row.isPending ? (row.variables?.providerIds ?? []) : [];
  return {
    refreshAll: () => all.mutate({ intent: "explicit" }),
    runAction: (providerId, action) => {
      if (action === "configure") {
        configure?.(providerId);
        return;
      }
      const explicit = () => row.mutate({ intent: "explicit", providerIds: [providerId] });
      if (action === "allow_keychain") {
        void allowKeychain?.(providerId).then(
          () =>
            queryClient.invalidateQueries({
              queryKey: providerUsageKeys.envelope(profile.trim()),
              exact: true,
            }),
          () => undefined
        );
        return;
      }
      explicit();
    },
    refreshing: all.isPending,
    pendingProviderIds: new Set(rowPending),
  };
}
