import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import { useActiveWorkspace } from "@/systems/workspace";

import type { NetworkSurface } from "../types";

export const LAST_READ_STORAGE_KEY = "compozy:network:last-read:v2";
const KEY_SEPARATOR = ":";
const EMPTY_LAST_READS: Readonly<Record<string, string>> = {};

export interface NetworkLastReadKey {
  workspaceId: string;
  channel: string;
  surface: NetworkSurface;
  containerId: string;
}

export type NetworkLastReadLookupKey = Omit<NetworkLastReadKey, "workspaceId">;

interface NetworkLastReadContext {
  byWorkspace: Record<string, Record<string, string>>;
}

function latestTimestamp(current: string | undefined, incoming: string): string {
  if (!current) {
    return incoming;
  }

  const currentMs = Date.parse(current);
  const incomingMs = Date.parse(incoming);
  if (Number.isNaN(currentMs) || Number.isNaN(incomingMs)) {
    return incoming;
  }
  return incomingMs > currentMs ? incoming : current;
}

function isValidTimestamp(value: string): boolean {
  return Number.isFinite(Date.parse(value));
}

function normalizedLastReads(value: unknown): Record<string, Record<string, string>> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }

  const byWorkspace: Record<string, Record<string, string>> = {};
  for (const [workspaceId, records] of Object.entries(value)) {
    if (typeof records !== "object" || records === null || Array.isArray(records)) {
      continue;
    }

    const normalizedRecords: Record<string, string> = {};
    for (const [key, timestamp] of Object.entries(records)) {
      if (typeof timestamp === "string" && isValidTimestamp(timestamp)) {
        normalizedRecords[key] = timestamp;
      }
    }
    byWorkspace[workspaceId] = normalizedRecords;
  }
  return byWorkspace;
}

function mergeLastReads(
  persisted: Partial<NetworkLastReadContext>,
  current: NetworkLastReadContext
): NetworkLastReadContext {
  const persistedByWorkspace = normalizedLastReads(persisted.byWorkspace);
  const workspaceIds = new Set([
    ...Object.keys(current.byWorkspace),
    ...Object.keys(persistedByWorkspace),
  ]);
  const byWorkspace: Record<string, Record<string, string>> = {};

  for (const workspaceId of workspaceIds) {
    const currentRecords = current.byWorkspace[workspaceId] ?? {};
    const persistedRecords = persistedByWorkspace[workspaceId] ?? {};
    const keys = new Set([...Object.keys(currentRecords), ...Object.keys(persistedRecords)]);
    const records: Record<string, string> = {};

    for (const key of keys) {
      const incoming = persistedRecords[key];
      const existing = currentRecords[key];
      if (incoming) {
        records[key] = latestTimestamp(existing, incoming);
      } else if (existing) {
        records[key] = existing;
      }
    }
    byWorkspace[workspaceId] = records;
  }

  return { byWorkspace };
}

export function buildLastReadStorageKey(key: NetworkLastReadKey): string {
  return [key.workspaceId, key.channel, key.surface, key.containerId].join(KEY_SEPARATOR);
}

export const networkLastReadLogic = createStoreLogic({
  context: (): NetworkLastReadContext => ({ byWorkspace: {} }),
  on: {
    lastReadMarked: (context, event: { workspaceId: string; key: string; timestamp: string }) => {
      if (!isValidTimestamp(event.timestamp)) return context;
      const currentWorkspace = context.byWorkspace[event.workspaceId] ?? {};
      const timestamp = latestTimestamp(currentWorkspace[event.key], event.timestamp);
      if (timestamp === currentWorkspace[event.key]) {
        return context;
      }

      return {
        ...context,
        byWorkspace: {
          ...context.byWorkspace,
          [event.workspaceId]: { ...currentWorkspace, [event.key]: timestamp },
        },
      };
    },
  },
});

export const networkLastReadStore = networkLastReadLogic.createStore().with(
  persist({
    name: LAST_READ_STORAGE_KEY,
    merge: mergeLastReads,
  })
);

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === LAST_READ_STORAGE_KEY) {
      void rehydrateStore(networkLastReadStore);
    }
  });
}

export interface UseLastReadResult {
  lastReadAt(key: NetworkLastReadLookupKey): string | null;
  markRead(key: NetworkLastReadLookupKey, timestamp: string | null | undefined): void;
}

export function useLastRead(options: { workspaceId?: string | null } = {}): UseLastReadResult {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = options.workspaceId ?? activeWorkspaceId;
  const workspaceReads = useSelector(networkLastReadStore, snapshot =>
    workspaceId ? (snapshot.context.byWorkspace[workspaceId] ?? EMPTY_LAST_READS) : EMPTY_LAST_READS
  );

  const lastReadAt = (key: NetworkLastReadLookupKey) => {
    if (!workspaceId) {
      return null;
    }
    return workspaceReads[buildLastReadStorageKey({ ...key, workspaceId })] ?? null;
  };

  const markRead = (key: NetworkLastReadLookupKey, timestamp: string | null | undefined) => {
    if (!timestamp || !workspaceId) {
      return;
    }
    networkLastReadStore.trigger.lastReadMarked({
      key: buildLastReadStorageKey({ ...key, workspaceId }),
      timestamp,
      workspaceId,
    });
  };

  return { lastReadAt, markRead };
}
