import { createStoreLogic } from "@xstate/store";
import {
  persist,
  rehydrateStore,
  type PersistStorageValue,
  type StateStorage,
} from "@xstate/store/persist";

export const PINNED_CHANNELS_STORAGE_KEY = "network:pinned-channels";
type PinnedChannelsState = Record<string, string[]>;

export interface PinnedChannelsStoreInput {
  workspaceId: string;
  pinnedIds: string[];
}

interface PinnedChannelsStoreContext {
  workspaceId: string;
  pinnedIds: string[];
}

function sameIds(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

export const pinnedChannelsLogic = createStoreLogic({
  context: (input: PinnedChannelsStoreInput): PinnedChannelsStoreContext => ({
    workspaceId: input.workspaceId,
    pinnedIds: input.pinnedIds,
  }),
  on: {
    pinToggled: (context, event: { workspaceId: string; channel: string }) => {
      if (event.workspaceId !== context.workspaceId) return;

      const pinnedIds = context.pinnedIds.includes(event.channel)
        ? context.pinnedIds.filter(channel => channel !== event.channel)
        : [event.channel, ...context.pinnedIds];
      return { ...context, pinnedIds };
    },
  },
});

export type PinnedChannelsStore = ReturnType<typeof pinnedChannelsLogic.createStore>;

export function createPinnedChannelsStore(workspaceId: string): PinnedChannelsStore {
  return pinnedChannelsLogic.createStore({ workspaceId, pinnedIds: [] }).with(
    persist({
      name: PINNED_CHANNELS_STORAGE_KEY,
      storage: createPinnedChannelsStorage(workspaceId),
      skipHydration: true,
      merge: (persisted, current) => ({
        workspaceId: current.workspaceId,
        pinnedIds: sameIds(current.pinnedIds, persisted.pinnedIds ?? [])
          ? current.pinnedIds
          : (persisted.pinnedIds ?? []),
      }),
      pick: context => ({ pinnedIds: context.pinnedIds }),
    })
  );
}

export function rehydratePinnedChannelsStore(store: PinnedChannelsStore): Promise<void> {
  return rehydrateStore(store);
}

function createPinnedChannelsStorage(workspaceId: string): StateStorage {
  return {
    getItem: () =>
      JSON.stringify({
        context: { pinnedIds: readPinnedChannels(workspaceId) },
        version: 0,
      } satisfies PersistStorageValue<Partial<PinnedChannelsStoreContext>>),
    removeItem: () => writePinnedChannels(workspaceId, []),
    setItem: (_name, value) => {
      const persisted = JSON.parse(value) as PersistStorageValue<PinnedChannelsStoreContext>;
      writePinnedChannels(workspaceId, persisted.context.pinnedIds ?? []);
    },
  };
}

function readPinnedChannelsState(): PinnedChannelsState {
  if (typeof window === "undefined") return {};
  try {
    const parsed = JSON.parse(window.localStorage.getItem(PINNED_CHANNELS_STORAGE_KEY) ?? "{}");
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return {};
    const state: PinnedChannelsState = {};
    for (const [workspaceId, channels] of Object.entries(parsed)) {
      if (!Array.isArray(channels)) continue;
      const clean = channels.filter(item => typeof item === "string" && item.trim() !== "");
      if (clean.length > 0) state[workspaceId] = clean;
    }
    return state;
  } catch (error) {
    console.warn("Failed to read pinned network channels", error);
    return {};
  }
}

function readPinnedChannels(workspaceId: string): string[] {
  return workspaceId ? (readPinnedChannelsState()[workspaceId] ?? []) : [];
}

function writePinnedChannels(workspaceId: string, values: string[]): void {
  if (typeof window === "undefined") return;
  const next = { ...readPinnedChannelsState() };
  if (values.length === 0) delete next[workspaceId];
  else next[workspaceId] = values;
  window.localStorage.setItem(PINNED_CHANNELS_STORAGE_KEY, JSON.stringify(next));
}
