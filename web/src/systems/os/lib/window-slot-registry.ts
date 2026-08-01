import { createTopbarSlotStore, type TopbarSlotStore, type TopbarSlotValue } from "@compozy/ui";

/**
 * One topbar slot store per live window, shared by the frame that renders the
 * member surface and every shell reader (deck labels, palette "Go to tab").
 * Stores follow the WINDOW, so regrouping keeps a tab's live label without a
 * republish. Readers prune against the live window set — windows close from
 * any client, so no single owner can release deterministically.
 */
const stores = new Map<string, TopbarSlotStore>();
const storeSubscriptions = new Map<string, () => void>();
const listeners = new Set<() => void>();
let version = 0;

function emitRegistryChange(): void {
  version += 1;
  for (const listener of listeners) listener();
}

export function ensureWindowSlotStore(windowId: string): TopbarSlotStore {
  let store = stores.get(windowId);
  if (store === undefined) {
    store = createTopbarSlotStore();
    stores.set(windowId, store);
    storeSubscriptions.set(windowId, store.subscribe(emitRegistryChange));
  }
  return store;
}

export function windowSlotSnapshot(windowId: string): TopbarSlotValue | null {
  return stores.get(windowId)?.getSnapshot() ?? null;
}

export function pruneWindowSlotStores(liveWindowIds: ReadonlySet<string>): void {
  let changed = false;
  for (const windowId of stores.keys()) {
    if (liveWindowIds.has(windowId)) continue;
    storeSubscriptions.get(windowId)?.();
    storeSubscriptions.delete(windowId);
    stores.delete(windowId);
    changed = true;
  }
  if (changed) emitRegistryChange();
}

export function subscribeWindowSlotRegistry(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function windowSlotRegistryVersion(): number {
  return version;
}
