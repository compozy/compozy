"use client";

import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import type { StreamEventSource } from "@/lib/ticketed-event-source";

import { terminalAggregateStreamPath, type TerminalAggregateScope } from "../lib/aggregate-query";
import { terminalKeys } from "../lib/query-keys";
import {
  parseTerminalCatalogEvent,
  reconcileTerminalCatalog,
  TerminalCatalogProtocolError,
  TERMINAL_CATALOG_EVENTS,
} from "../lib/terminal-catalog-stream";
import {
  acquireCatalogStreamLease,
  defaultEventSourceFactory,
  factoryIdentity,
} from "../lib/terminal-stream-lease";
import type { TerminalInfo } from "../types";

export type TerminalAggregateEventSourceFactory = (url: string) => StreamEventSource;

export interface UseTerminalAggregateStreamOptions {
  scope: TerminalAggregateScope;
  enabled?: boolean;
  /** Test seam; the ticketed browser source is the default. */
  eventSourceFactory?: TerminalAggregateEventSourceFactory;
}

/**
 * Opens the aggregate stream for one selection. Every write targets the key
 * captured here, so a frame that lands after the selection changed is thrown
 * away with that selection's cache instead of merging into the new one.
 */
function openTerminalAggregateStream(
  queryClient: QueryClient,
  scope: TerminalAggregateScope,
  eventSourceFactory: TerminalAggregateEventSourceFactory
): () => void {
  const queryKey = terminalKeys.aggregate(scope.selection, scope.profileKey);
  let closed = false;
  const refresh = () => void queryClient.invalidateQueries({ queryKey, exact: true });
  const listeners = TERMINAL_CATALOG_EVENTS.map(name => ({
    name,
    listener: ((event: Event) => {
      if (closed) return;
      if (!(event instanceof MessageEvent) || typeof event.data !== "string") return;
      let parsed: ReturnType<typeof parseTerminalCatalogEvent>;
      try {
        parsed = parseTerminalCatalogEvent(name, JSON.parse(event.data));
      } catch (error) {
        if (!(error instanceof TerminalCatalogProtocolError) && !(error instanceof SyntaxError)) {
          throw error;
        }
        refresh();
        return;
      }
      if (!parsed) return;
      // A REST read started before this frame may hold an older list.
      void queryClient.cancelQueries({ queryKey, exact: true });
      queryClient.setQueryData<TerminalInfo[]>(queryKey, current =>
        reconcileTerminalCatalog(current, parsed)
      );
    }) as EventListener,
  }));
  // A reconnect may have missed frames past retention; the daemon then sends a
  // reset snapshot, and the list rereads from REST meanwhile.
  const handleOpen: EventListener = () => {
    if (!closed) refresh();
  };
  const source = eventSourceFactory(terminalAggregateStreamPath(scope));
  source.addEventListener("open", handleOpen);
  for (const entry of listeners) source.addEventListener(entry.name, entry.listener);
  return () => {
    closed = true;
    source.removeEventListener("open", handleOpen);
    for (const entry of listeners) source.removeEventListener(entry.name, entry.listener);
    source.close();
  };
}

/**
 * Keeps the All projects terminal list live. The daemon binds the stream's
 * cursor to the selection (Project filter + profile lens); this hook keys its
 * subscription and cache the same way, so a selection change replaces both.
 */
export function useTerminalAggregateStream({
  scope,
  enabled = true,
  eventSourceFactory,
}: UseTerminalAggregateStreamOptions): void {
  const queryClient = useQueryClient();
  const scopeSignature = JSON.stringify(scope);
  const canConnect =
    enabled &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");

  useEffect(() => {
    if (!canConnect) return undefined;
    const factory = eventSourceFactory ?? defaultEventSourceFactory;
    const current = JSON.parse(scopeSignature) as TerminalAggregateScope;
    const leaseKey = `aggregate:${factoryIdentity(factory)}:${scopeSignature}`;
    return acquireCatalogStreamLease(queryClient, leaseKey, () =>
      openTerminalAggregateStream(queryClient, current, factory)
    );
  }, [canConnect, eventSourceFactory, queryClient, scopeSignature]);
}
