/**
 * Browser-local storage layer for the runtime selector's favorites and recents.
 * These preferences never reach the daemon (`_spec.md` §16); they persist in
 * localStorage under these keys, with recents deduped and capped. Stored values
 * are the compound `(provider, model)` identity (see `model-key.ts`) so a
 * favorite/recent is unambiguous when two providers publish the same model id.
 * The `useRuntimeFavorites` hook (in `use-runtime-favorites.ts`) wraps this.
 */
export const FAVORITES_STORAGE_KEY = "agh:runtime-selector:fav";
export const RECENTS_STORAGE_KEY = "agh:runtime-selector:recent";
export const RECENTS_LIMIT = 6;

export function readFavoritesList(key: string): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((value): value is string => typeof value === "string");
  } catch {
    return [];
  }
}

export function writeFavoritesList(key: string, value: string[]): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage is a best-effort convenience; ignore quota/private-mode errors.
  }
}

export function dedupeFavorites(ids: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const id of ids) {
    if (id.length === 0 || seen.has(id)) continue;
    seen.add(id);
    result.push(id);
  }
  return result;
}

export interface RuntimeFavoritesStore {
  favorites: Set<string>;
  recents: string[];
  isFavorite: (id: string) => boolean;
  toggleFavorite: (id: string) => void;
  pushRecent: (id: string) => void;
}
