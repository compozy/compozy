import { createStore, createStoreConfig } from "@xstate/store";
import {
  persist,
  rehydrateStore,
  type PersistStorageValue,
  type StateStorage,
} from "@xstate/store/persist";

import {
  dedupeFavorites,
  FAVORITES_STORAGE_KEY,
  readFavoritesList,
  RECENTS_LIMIT,
  RECENTS_STORAGE_KEY,
  writeFavoritesList,
} from "./favorites";

export interface RuntimeFavoritesState {
  favorites: readonly string[];
  recents: readonly string[];
}

type RuntimeFavoritesEvents = {
  favoriteToggled: { key: string; validKeys: readonly string[] };
  recentPushed: { key: string; validKeys: readonly string[] };
  storageHydrated: { favorites: readonly string[]; recents: readonly string[] };
};

const initialRuntimeFavoritesState: RuntimeFavoritesState = { favorites: [], recents: [] };

function sameList(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function normalizedList(values: readonly string[]): string[] {
  return dedupeFavorites(values);
}

function normalizedContext(
  context: RuntimeFavoritesState,
  favorites: readonly string[],
  recents: readonly string[]
): RuntimeFavoritesState {
  const nextFavorites = normalizedList(favorites);
  const nextRecents = normalizedList(recents).slice(0, RECENTS_LIMIT);
  if (sameList(context.favorites, nextFavorites) && sameList(context.recents, nextRecents)) {
    return context;
  }
  return { favorites: nextFavorites, recents: nextRecents };
}

function acceptsKey(key: string, validKeys: readonly string[]): boolean {
  return key.length > 0 && validKeys.includes(key);
}

interface PersistedRuntimeFavorites {
  context: RuntimeFavoritesState;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every(item => typeof item === "string");
}

function isPersistedRuntimeFavorites(value: unknown): value is PersistedRuntimeFavorites {
  if (!isRecord(value) || !isRecord(value.context)) return false;
  return isStringArray(value.context.favorites) && isStringArray(value.context.recents);
}

function parsePersistedRuntimeFavorites(value: string): RuntimeFavoritesState | null {
  try {
    const parsed: unknown = JSON.parse(value);
    if (isPersistedRuntimeFavorites(parsed)) return parsed.context;
  } catch (error) {
    console.warn("Failed to parse runtime favorites persistence payload", error);
    return null;
  }

  console.warn("Invalid runtime favorites persistence payload");
  return null;
}

/** Pure preference transitions; catalog identities are passed only at event boundaries. */
export const runtimeFavoritesConfig = createStoreConfig<
  RuntimeFavoritesState,
  RuntimeFavoritesEvents
>({
  context: initialRuntimeFavoritesState,
  on: {
    favoriteToggled: (context, event) => {
      if (!acceptsKey(event.key, event.validKeys)) return context;
      const favorites = context.favorites.includes(event.key)
        ? context.favorites.filter(key => key !== event.key)
        : [...context.favorites, event.key];
      return normalizedContext(context, favorites, context.recents);
    },
    recentPushed: (context, event) => {
      if (!acceptsKey(event.key, event.validKeys)) return context;
      return normalizedContext(context, context.favorites, [
        event.key,
        ...context.recents.filter(key => key !== event.key),
      ]);
    },
    storageHydrated: (context, event) => normalizedContext(context, event.favorites, event.recents),
  },
});

const runtimeFavoritesStorage: StateStorage = {
  getItem: () =>
    JSON.stringify({
      context: {
        favorites: readFavoritesList(FAVORITES_STORAGE_KEY),
        recents: readFavoritesList(RECENTS_STORAGE_KEY),
      },
      version: 0,
    } satisfies PersistStorageValue<RuntimeFavoritesState>),
  removeItem: () => {
    writeFavoritesList(FAVORITES_STORAGE_KEY, []);
    writeFavoritesList(RECENTS_STORAGE_KEY, []);
  },
  setItem: (_name, value) => {
    const persisted = parsePersistedRuntimeFavorites(value);
    if (!persisted) return;
    writeFavoritesList(FAVORITES_STORAGE_KEY, persisted.favorites);
    writeFavoritesList(RECENTS_STORAGE_KEY, persisted.recents);
  },
};

/** Shared browser preference store; selector instances intentionally share it. */
export const runtimeFavoritesStore = createStore(runtimeFavoritesConfig).with(
  persist({
    name: "compozy:runtime-selector:preferences",
    storage: runtimeFavoritesStorage,
    merge: (persisted, current) =>
      normalizedContext(current, persisted.favorites ?? [], persisted.recents ?? []),
  })
);

export async function hydrateRuntimeFavoritesFromStorage(): Promise<void> {
  await rehydrateStore(runtimeFavoritesStore);
  const { favorites, recents } = runtimeFavoritesStore.getSnapshot().context;
  runtimeFavoritesStore.trigger.storageHydrated({ favorites, recents });
}
