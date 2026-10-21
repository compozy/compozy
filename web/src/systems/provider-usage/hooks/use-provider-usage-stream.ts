"use client";

import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

import { providerUsageStreamPath } from "../adapters/provider-usage-api";
import { providerUsageKeys } from "../lib/query-keys";

export type ProviderUsageEventSourceFactory = (url: string) => StreamEventSource;

export const PROVIDER_USAGE_STREAM_EVENTS = ["ready", "provider_usage_changed"] as const;

export interface UseProviderUsageStreamOptions {
  enabled?: boolean;
  /** Test seam; the ticketed browser source is the default. */
  eventSourceFactory?: ProviderUsageEventSourceFactory;
}

function defaultEventSourceFactory(url: string): StreamEventSource {
  return createStreamEventSource(url);
}

interface StreamLease {
  refs: number;
  cleanup: () => void;
}

const leasesByClient = new WeakMap<QueryClient, Map<string, StreamLease>>();

/** The tray, the popover, and the Usage app share one source per profile. */
function acquireStreamLease(
  queryClient: QueryClient,
  key: string,
  open: () => () => void
): () => void {
  let byKey = leasesByClient.get(queryClient);
  if (!byKey) {
    byKey = new Map();
    leasesByClient.set(queryClient, byKey);
  }
  let lease = byKey.get(key);
  if (!lease) {
    lease = { refs: 0, cleanup: open() };
    byKey.set(key, lease);
  }
  lease.refs += 1;
  const owned = lease;
  const scope = byKey;
  return () => {
    owned.refs -= 1;
    if (owned.refs === 0) {
      owned.cleanup();
      scope.delete(key);
    }
  };
}

/**
 * Frames carry only `{revision, boot_id}`; the envelope is always re-read.
 * `ready` (every connect, reconnects included) and every change frame
 * invalidate, so a daemon restart (new `boot_id`, revisions starting over)
 * needs no revision bookkeeping here. A frame from a closed source is dropped
 * so a late event cannot write into the profile that replaced it.
 */
function openProviderUsageStream(
  queryClient: QueryClient,
  profile: string,
  factory: ProviderUsageEventSourceFactory
): () => void {
  const queryKey = providerUsageKeys.envelope(profile);
  let closed = false;
  const reread: EventListener = () => {
    if (closed) return;
    void queryClient.invalidateQueries({ queryKey, exact: true });
  };
  const source = factory(providerUsageStreamPath(profile));
  for (const name of PROVIDER_USAGE_STREAM_EVENTS) source.addEventListener(name, reread);
  return () => {
    closed = true;
    for (const name of PROVIDER_USAGE_STREAM_EVENTS) source.removeEventListener(name, reread);
    source.close();
  };
}

/** Keeps the profile's usage envelope live from `GET /api/provider-usage/stream`. */
export function useProviderUsageStream(
  profile: string,
  { enabled = true, eventSourceFactory }: UseProviderUsageStreamOptions = {}
): void {
  const queryClient = useQueryClient();
  const scoped = profile.trim();
  const canConnect =
    enabled &&
    scoped !== "" &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");

  useEffect(() => {
    if (!canConnect) return undefined;
    if (eventSourceFactory) return openProviderUsageStream(queryClient, scoped, eventSourceFactory);
    return acquireStreamLease(queryClient, scoped, () =>
      openProviderUsageStream(queryClient, scoped, defaultEventSourceFactory)
    );
  }, [canConnect, eventSourceFactory, queryClient, scoped]);
}
