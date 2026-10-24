import type { QueryClient } from "@tanstack/react-query";

import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

type EventSourceFactory = (url: string) => StreamEventSource;

/** The ticketed browser source every terminal catalog stream opens by default. */
export function defaultEventSourceFactory(url: string): StreamEventSource {
  return createStreamEventSource(url);
}

const factoryIds = new WeakMap<EventSourceFactory, number>();
let nextFactoryId = 1;

/** Lease identity of a source factory, so test doubles never share a real lease. */
export function factoryIdentity(factory: EventSourceFactory): string {
  if (factory === defaultEventSourceFactory) return "default";
  const existing = factoryIds.get(factory);
  if (existing !== undefined) return String(existing);
  const id = nextFactoryId;
  nextFactoryId += 1;
  factoryIds.set(factory, id);
  return String(id);
}

interface CatalogStreamLease {
  refs: number;
  cleanup: () => void;
}

const leasesByClient = new WeakMap<QueryClient, Map<string, CatalogStreamLease>>();

/**
 * One EventSource per query client and scope, shared across subscribers.
 *
 * Session blocks and the Terminal app read the same cache. Opening a socket
 * per hook instance would fan the same frames out N times and drop the last
 * subscriber's cache on the first unmount.
 */
export function acquireCatalogStreamLease(
  queryClient: QueryClient,
  key: string,
  open: () => () => void
): () => void {
  let byScope = leasesByClient.get(queryClient);
  if (!byScope) {
    byScope = new Map();
    leasesByClient.set(queryClient, byScope);
  }
  let lease = byScope.get(key);
  if (!lease) {
    lease = { refs: 0, cleanup: open() };
    byScope.set(key, lease);
  }
  lease.refs += 1;
  return () => {
    lease.refs -= 1;
    if (lease.refs === 0) {
      lease.cleanup();
      byScope.delete(key);
    }
  };
}
