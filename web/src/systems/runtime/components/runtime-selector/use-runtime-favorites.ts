import { useEffect } from "react";

import { useSelector } from "@xstate/store-react";

import {
  FAVORITES_STORAGE_KEY,
  RECENTS_STORAGE_KEY,
  type RuntimeFavoritesStore,
} from "./favorites";
import {
  hydrateRuntimeFavoritesFromStorage,
  runtimeFavoritesStore,
} from "./runtime-favorites-store";

let mountedFavoritesConsumers = 0;

function handleRuntimeFavoritesStorageChange(event: StorageEvent): void {
  if (event.key !== FAVORITES_STORAGE_KEY && event.key !== RECENTS_STORAGE_KEY) return;
  void hydrateRuntimeFavoritesFromStorage();
}

/**
 * Shared browser-local favorites and recents, keyed by the exact compound
 * `(provider, model)` identity. The catalog remains query-owned: it validates
 * new events and filters each selector's view without deleting preferences
 * that belong to another selector surface.
 */
export function useRuntimeFavorites(validKeys: ReadonlySet<string>): RuntimeFavoritesStore {
  const state = useSelector(runtimeFavoritesStore, snapshot => snapshot.context);

  // A module singleton hydrates when it is created. The first live selector
  // owns its browser synchronization so every selector shares one hydration
  // and one cross-tab listener without replacing the shared store identity.
  useEffect(() => {
    const isFirstConsumer = mountedFavoritesConsumers === 0;
    mountedFavoritesConsumers += 1;
    if (isFirstConsumer) {
      window.addEventListener("storage", handleRuntimeFavoritesStorageChange);
      void hydrateRuntimeFavoritesFromStorage();
    }
    return () => {
      mountedFavoritesConsumers -= 1;
      if (mountedFavoritesConsumers === 0) {
        window.removeEventListener("storage", handleRuntimeFavoritesStorageChange);
      }
    };
  }, []);

  const keys = [...validKeys];
  const visibleFavorites = state.favorites.filter(key => validKeys.has(key));
  const visibleRecents = state.recents.filter(key => validKeys.has(key));
  const toggleFavorite = (id: string) => {
    runtimeFavoritesStore.trigger.favoriteToggled({ key: id.trim(), validKeys: keys });
  };
  const pushRecent = (id: string) => {
    runtimeFavoritesStore.trigger.recentPushed({ key: id.trim(), validKeys: keys });
  };

  return {
    favorites: new Set(visibleFavorites),
    recents: visibleRecents,
    isFavorite: id => validKeys.has(id) && state.favorites.includes(id),
    toggleFavorite,
    pushRecent,
  };
}
