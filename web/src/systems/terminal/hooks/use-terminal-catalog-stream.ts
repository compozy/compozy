"use client";

import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

import {
  parseTerminalCatalogEvent,
  reconcileTerminalCatalog,
  reconcileTerminalProfileSnapshot,
  terminalCatalogStreamPath,
  TERMINAL_CATALOG_EVENTS,
} from "../lib/terminal-catalog-stream";
import { terminalKeys } from "../lib/query-keys";
import type { TerminalInfo } from "../types";

export type TerminalCatalogEventSourceFactory = (url: string) => StreamEventSource;

export interface UseTerminalCatalogStreamOptions {
  workspaceId: string;
  /** The profile these terminals belong to, as the catalog key spells it. */
  profileKey: string;
  /** Reads the labeled aggregate without treating its cache key as a profile name. */
  allProfiles?: boolean;
  /** Concrete owners subscribed into the aggregate cache. */
  profiles?: readonly string[];
  enabled?: boolean;
  /** Test seam; the ticketed browser source is the default. */
  eventSourceFactory?: TerminalCatalogEventSourceFactory;
}

function defaultEventSourceFactory(url: string): StreamEventSource {
  return createStreamEventSource(url);
}

function normalizeTerminalProfiles(profiles: readonly string[]): string[] {
  const normalized = new Set<string>();
  for (const profile of profiles) {
    const value = profile.trim();
    if (value !== "") normalized.add(value);
  }
  return [...normalized].sort();
}

/**
 * Opens one catalog stream for one scope.
 *
 * Everything the stream writes is addressed to the key captured here, so a
 * frame that arrives after a profile switch — the socket is closing, but the
 * last event is already in the task queue — lands on the scope it was read
 * under and is then thrown away with that cache entry, rather than being merged
 * into the scope now on screen.
 */
function openTerminalCatalogStream(
  queryClient: QueryClient,
  workspaceId: string,
  profileKey: string,
  streamProfile: string,
  aggregate: boolean,
  eventSourceFactory: TerminalCatalogEventSourceFactory
): () => void {
  const queryKey = terminalKeys.catalog({ workspaceId, profileKey });
  let closed = false;

  const handleFrame = (name: string): EventListener => {
    return event => {
      if (closed) return;
      if (!(event instanceof MessageEvent) || typeof event.data !== "string") return;
      let raw: unknown;
      try {
        raw = JSON.parse(event.data);
      } catch {
        // A frame this client cannot read is dropped rather than merged
        // half-understood into a list someone is reading.
        return;
      }
      const parsed = parseTerminalCatalogEvent(name, raw);
      if (!parsed) return;
      queryClient.setQueryData<TerminalInfo[]>(queryKey, current => {
        if (aggregate && parsed.name === "terminal.snapshot") {
          return reconcileTerminalProfileSnapshot(current, streamProfile, parsed.terminals);
        }
        return reconcileTerminalCatalog(current, parsed);
      });
    };
  };

  // A reconnect may have missed frames, so the list rereads from the server
  // rather than trusting whatever the cache still holds.
  const handleOpen: EventListener = () => {
    if (closed) return;
    void queryClient.invalidateQueries({ queryKey, exact: true });
  };

  const listeners = TERMINAL_CATALOG_EVENTS.map(name => ({ name, listener: handleFrame(name) }));
  const source = eventSourceFactory(terminalCatalogStreamPath(workspaceId, streamProfile));
  const detach = () => {
    source.removeEventListener("open", handleOpen);
    for (const entry of listeners) {
      source.removeEventListener(entry.name, entry.listener);
    }
  };
  try {
    source.addEventListener("open", handleOpen);
    for (const entry of listeners) {
      source.addEventListener(entry.name, entry.listener);
    }
  } catch (error) {
    detach();
    source.close();
    throw error;
  }

  return () => {
    closed = true;
    detach();
    source.close();
  };
}

/**
 * Keeps the terminal list live without attaching to any terminal.
 *
 * Only an open pane holds a WebSocket, so the tab strip, the dock badge and the
 * list would otherwise be as old as the last read. Exactly one subscription
 * exists per `(workspace, profile)`: switching either closes the previous
 * source before opening the next, and the closed source's frames are ignored,
 * so a late event can never write into the scope that replaced it.
 */
export function useTerminalCatalogStream({
  workspaceId,
  profileKey,
  allProfiles = false,
  profiles = [],
  enabled = true,
  eventSourceFactory,
}: UseTerminalCatalogStreamOptions): void {
  const queryClient = useQueryClient();
  const streamProfiles = allProfiles ? normalizeTerminalProfiles(profiles) : [profileKey];
  const streamProfileSignature = JSON.stringify(streamProfiles);
  const canConnect =
    enabled &&
    workspaceId.trim() !== "" &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined") &&
    streamProfiles.length > 0;

  useEffect(() => {
    if (!canConnect) return undefined;
    const concreteProfiles = JSON.parse(streamProfileSignature) as string[];
    const cleanups = concreteProfiles.map(streamProfile =>
      openTerminalCatalogStream(
        queryClient,
        workspaceId,
        profileKey,
        streamProfile,
        allProfiles,
        eventSourceFactory ?? defaultEventSourceFactory
      )
    );
    return () => {
      for (const cleanup of cleanups) cleanup();
    };
  }, [
    allProfiles,
    canConnect,
    eventSourceFactory,
    profileKey,
    queryClient,
    streamProfileSignature,
    workspaceId,
  ]);
}
