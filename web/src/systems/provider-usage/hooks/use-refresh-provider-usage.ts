"use client";

import { useEffect, useRef } from "react";
import { hashKey, useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { refreshProviderUsage } from "../adapters/provider-usage-api";
import { providerUsageKeys } from "../lib/query-keys";
import { describeProviderUsageStatus, type ProviderUsageRowAction } from "../lib/status";
import { PROVIDER_USAGE_OPEN_MAX_AGE_SECONDS, targetsNeedingOpenRefresh } from "../lib/view-model";
import type {
  ProviderUsageEnvelope,
  ProviderUsageRefreshIntent,
  ProviderUsageTarget,
} from "../types";

const REFRESH_WARNING_STATUSES = new Set([
  "needs_consent",
  "signed_out",
  "error",
  "stale",
  "rate_limited",
]);

export interface ProviderUsageRefreshInput {
  intent: ProviderUsageRefreshIntent;
  providerIds?: string[];
  maxAgeSeconds?: number;
}

interface RefreshFeedbackScope {
  profile: string;
  subscriptions: Set<() => void>;
}

function notifyRefreshOutcome(profile: string, envelope: ProviderUsageEnvelope) {
  const unresolved = envelope.targets.filter(target => REFRESH_WARNING_STATUSES.has(target.status));
  if (unresolved.length === 0) return;
  const details = unresolved.slice(0, 3).map(target => {
    const status = describeProviderUsageStatus(target, {
      now: Date.now(),
      isMacHost: envelope.keychain_supported,
    });
    const reason = target.error?.message.trim() || status?.text;
    return unresolved.length === 1 ? reason : `${target.provider_name}: ${reason}`;
  });
  if (unresolved.length > 3) details.push(`${unresolved.length - 3} more. Open Usage for details.`);
  toast.warning(
    unresolved.length === 1
      ? `${unresolved[0]?.provider_name} usage still needs attention (${profile})`
      : `${unresolved.length} providers still need attention (${profile})`,
    { description: details.join(" · ") }
  );
}

function isCollecting(target: ProviderUsageTarget): boolean {
  return target.refreshing || target.status === "refreshing";
}

/** Observe the existing SSE-driven reads after the HTTP refresh wait expires. */
function watchRefreshOutcome(
  queryClient: QueryClient,
  scope: RefreshFeedbackScope,
  initial: ProviderUsageEnvelope
) {
  const pending = new Set<string>();
  const outcomes = new Map<string, ProviderUsageTarget>();
  for (const target of initial.targets) {
    outcomes.set(target.provider_id, target);
    if (isCollecting(target)) pending.add(target.provider_id);
  }
  const key = providerUsageKeys.envelope(scope.profile);
  const queryHash = hashKey(key);
  const startedAt = Date.parse(initial.generated_at);
  const check = () => {
    const current = queryClient.getQueryData<ProviderUsageEnvelope>(key);
    if (!current || current.profile_id !== initial.profile_id) return;
    const generatedAt = Date.parse(current.generated_at);
    if (
      generatedAt < startedAt ||
      (generatedAt === startedAt && current.revision < initial.revision)
    )
      return;
    const targets = new Map(current.targets.map(target => [target.provider_id, target]));
    for (const providerId of pending) {
      const target = targets.get(providerId);
      if (target && isCollecting(target)) continue;
      pending.delete(providerId);
      if (target) outcomes.set(providerId, target);
      else outcomes.delete(providerId);
    }
    if (pending.size > 0) return;
    stop();
    notifyRefreshOutcome(scope.profile, { ...current, targets: [...outcomes.values()] });
  };
  const unsubscribe = queryClient.getQueryCache().subscribe(event => {
    if (
      event.type === "updated" &&
      event.action.type === "success" &&
      event.query.queryHash === queryHash
    )
      check();
  });
  const stop = () => {
    unsubscribe();
    scope.subscriptions.delete(stop);
  };
  scope.subscriptions.add(stop);
  // A successful read can arrive before the refresh response's callback.
  check();
}

/**
 * Sends a refresh for the profile. The daemon answers with the envelope, but
 * the list read stays the single writer: settling re-reads it, as the stream
 * would on the revision bump.
 */
export function useRefreshProviderUsage(profile: string) {
  const queryClient = useQueryClient();
  const scoped = profile.trim();
  const feedbackScope = useRef<RefreshFeedbackScope | null>(null);
  useEffect(() => {
    const scope: RefreshFeedbackScope = { profile: scoped, subscriptions: new Set() };
    feedbackScope.current = scope;
    return () => {
      for (const stop of scope.subscriptions) stop();
      feedbackScope.current = null;
    };
  }, [scoped]);
  return useMutation({
    mutationKey: providerUsageKeys.refresh(scoped),
    onMutate: () => feedbackScope.current,
    mutationFn: ({ intent, providerIds, maxAgeSeconds }: ProviderUsageRefreshInput) =>
      refreshProviderUsage({
        profile: scoped,
        intent,
        ...(providerIds && providerIds.length > 0 ? { provider_ids: providerIds } : {}),
        ...(maxAgeSeconds === undefined ? {} : { max_age_seconds: maxAgeSeconds }),
      }),
    onSuccess: (envelope, { intent, providerIds }, scope) => {
      if (intent !== "explicit") return;
      const requested = providerIds?.length ? new Set(providerIds) : null;
      const selected = {
        ...envelope,
        targets: envelope.targets.filter(target => !requested || requested.has(target.provider_id)),
      };
      if (!selected.targets.some(isCollecting)) {
        notifyRefreshOutcome(scoped, selected);
        return;
      }
      if (scope && scope === feedbackScope.current)
        watchRefreshOutcome(queryClient, scope, selected);
    },
    onError: (error, { intent }) => {
      if (intent === "explicit") {
        toast.error(`Couldn't refresh provider usage (${scoped})`, { description: error.message });
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
