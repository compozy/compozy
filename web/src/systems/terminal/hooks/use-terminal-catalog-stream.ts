"use client";

import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

import {
  parseTerminalCatalogEvent,
  reconcileTerminalCatalog,
  reconcileTerminalProfileSnapshot,
  TerminalCatalogProtocolError,
  terminalCatalogStreamPath,
  TERMINAL_STREAM_EVENTS,
} from "../lib/terminal-catalog-stream";
import { terminalKeys } from "../lib/query-keys";
import {
  applyTerminalRecordingEvent,
  clearTerminalRecordingsForProfile,
  dropTerminalRecording,
  parseTerminalRecordingEvent,
  type TerminalRecordingMap,
} from "../lib/terminal-recording-state";
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

const factoryIds = new WeakMap<TerminalCatalogEventSourceFactory, number>();
let nextFactoryId = 1;

function factoryIdentity(factory: TerminalCatalogEventSourceFactory): string {
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
function acquireCatalogStreamLease(
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
  const inputQueryKey = terminalKeys.inputRequests({ workspaceId, profileKey });
  const recordingsKey = terminalKeys.recordings({ workspaceId, profileKey });
  let closed = false;
  const refreshCatalog = () => {
    void queryClient.invalidateQueries({ queryKey, exact: true });
  };
  const refreshFromREST = () => {
    refreshCatalog();
    void queryClient.invalidateQueries({ queryKey: inputQueryKey, exact: true });
  };
  const dropRecordingsForStream = () => {
    queryClient.setQueryData<TerminalRecordingMap>(recordingsKey, current =>
      clearTerminalRecordingsForProfile(current ?? {}, streamProfile, aggregate)
    );
  };

  const handleFrame = (name: string): EventListener => {
    return event => {
      if (closed) return;
      if (!(event instanceof MessageEvent) || typeof event.data !== "string") return;
      let raw: unknown;
      try {
        raw = JSON.parse(event.data);
      } catch {
        refreshCatalog();
        return;
      }
      const recording = parseTerminalRecordingEvent(name, raw);
      if (recording) {
        queryClient.setQueryData<TerminalRecordingMap>(recordingsKey, current =>
          applyTerminalRecordingEvent(current ?? {}, recording, {
            workspaceId,
            streamProfile,
            aggregate,
          })
        );
        return;
      }
      let parsed: ReturnType<typeof parseTerminalCatalogEvent>;
      try {
        parsed = parseTerminalCatalogEvent(name, raw);
      } catch (error) {
        if (!(error instanceof TerminalCatalogProtocolError)) throw error;
        refreshCatalog();
        return;
      }
      if (!parsed) return;
      if (parsed.name === "terminal.snapshot") {
        dropRecordingsForStream();
      } else if (parsed.name === "terminal.closed") {
        queryClient.setQueryData<TerminalRecordingMap>(recordingsKey, current =>
          dropTerminalRecording(current ?? {}, parsed.terminalId)
        );
      }
      queryClient.setQueryData<TerminalInfo[]>(queryKey, current => {
        if (aggregate && parsed.name === "terminal.snapshot") {
          return reconcileTerminalProfileSnapshot(current, streamProfile, parsed.terminals);
        }
        return reconcileTerminalCatalog(current, parsed);
      });
    };
  };

  // A reconnect may have missed a stop, so this stream's live map is dropped.
  // The daemon then emits `terminal.recording_started` for each still-active
  // recording after the snapshot or replay. The terminal list rereads from REST.
  const handleOpen: EventListener = () => {
    if (closed) return;
    dropRecordingsForStream();
    refreshFromREST();
  };

  const listeners = TERMINAL_STREAM_EVENTS.map(name => ({ name, listener: handleFrame(name) }));
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
 * exists per `(queryClient, workspace, profile, factory)`: extra hook
 * instances share that source, switching scope closes it only after the last
 * subscriber leaves, and a closed source's frames are ignored so a late event
 * can never write into the scope that replaced it.
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
    const factory = eventSourceFactory ?? defaultEventSourceFactory;
    const leaseKey = JSON.stringify({
      workspaceId,
      profileKey,
      allProfiles,
      streamProfileSignature,
      factoryId: factoryIdentity(factory),
    });
    return acquireCatalogStreamLease(queryClient, leaseKey, () => {
      const concreteProfiles = JSON.parse(streamProfileSignature) as string[];
      const cleanups = concreteProfiles.map(streamProfile =>
        openTerminalCatalogStream(
          queryClient,
          workspaceId,
          profileKey,
          streamProfile,
          allProfiles,
          factory
        )
      );
      return () => {
        for (const cleanup of cleanups) cleanup();
      };
    });
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
