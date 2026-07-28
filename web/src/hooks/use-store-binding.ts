import { useState } from "react";

interface StoreBinding<TKey, TStore> {
  key: TKey;
  store: TStore;
}

type ShouldReplaceStore<TKey, TStore> = (
  current: Readonly<StoreBinding<TKey, TStore>>,
  nextKey: TKey
) => boolean;

interface StoreBindingResult<TStore> {
  replace: () => void;
  store: TStore;
}

function keyChanged<TKey, TStore>(
  current: Readonly<StoreBinding<TKey, TStore>>,
  nextKey: TKey
): boolean {
  return !Object.is(current.key, nextKey);
}

/**
 * Replaces a component-local external store before commit when its creation-time
 * identity changes. Store construction must stay pure; ordinary component stores
 * should use XState's `useStore` directly, and callback freshness belongs in events.
 * React may replay render, so both creation callbacks MUST be deterministic: no I/O,
 * subscriptions, timers, mutation calls, or cleanup. The owning hook handles lifecycle.
 */
export function useStoreBinding<TKey, TStore>(
  key: TKey,
  createInitialStore: () => TStore
): StoreBindingResult<TStore>;
export function useStoreBinding<TKey, TStore>(
  key: TKey,
  createInitialStore: () => TStore,
  createReplacementStore: (previous: TStore) => TStore,
  shouldReplace: ShouldReplaceStore<TKey, TStore>
): StoreBindingResult<TStore>;
export function useStoreBinding<TKey, TStore>(
  key: TKey,
  createInitialStore: () => TStore,
  createReplacementStore?: (previous: TStore) => TStore,
  shouldReplace?: ShouldReplaceStore<TKey, TStore>
): StoreBindingResult<TStore> {
  const replaceStore = createReplacementStore ?? (() => createInitialStore());
  const shouldReplaceStore = shouldReplace ?? keyChanged;
  const [binding, setBinding] = useState<StoreBinding<TKey, TStore>>(() => ({
    key,
    store: createInitialStore(),
  }));
  const currentBinding = shouldReplaceStore(binding, key)
    ? { key, store: replaceStore(binding.store) }
    : binding;
  if (currentBinding !== binding) setBinding(currentBinding);

  return {
    replace: () =>
      setBinding(current => ({
        key,
        store: replaceStore(current.store),
      })),
    store: currentBinding.store,
  };
}
