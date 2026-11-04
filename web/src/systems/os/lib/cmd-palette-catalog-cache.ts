import {
  CMD_PALETTE_CATALOG_CONTRACT_VERSION,
  parseCatalogRecord,
  type CmdPaletteCatalogRecord,
} from "./cmd-palette-catalog-record";

/**
 * The last-known structural catalog, so ⌘K renders instantly against a cold or
 * reconnecting daemon (BR-19) instead of blocking on a round trip.
 *
 * One bounded record per workspace and profile lens, keyed by `(canonical
 * workspace id, profile lens, contract version)` — the catalog answers
 * differently per profile, so one lens must never hydrate another's. The record
 * holds structure only — availability is re-evaluated against the *current*
 * client's context on hydration, so a record another tab wrote can never speak
 * for this one.
 */
const DATABASE_NAME = "compozy.cmd_palette";
const DATABASE_VERSION = 1;
const STORE_NAME = "catalog";
/**
 * Both parts are already trimmed and neither a canonical workspace id nor a
 * profile lens may contain it, so this separator cannot let two distinct pairs
 * collide on one record.
 */
const KEY_SEPARATOR = "::";

function recordKey(workspaceId: string, profileKey: string): string {
  const parts = [workspaceId.trim(), profileKey.trim(), `v${CMD_PALETTE_CATALOG_CONTRACT_VERSION}`];
  return parts.join(KEY_SEPARATOR);
}

function indexedDbFactory(): IDBFactory | null {
  if (typeof globalThis === "undefined") return null;
  const factory = Reflect.get(globalThis, "indexedDB");
  return factory instanceof Object ? (factory as IDBFactory) : null;
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error("IndexedDB request failed"));
  });
}

async function openDatabase(): Promise<IDBDatabase | null> {
  const factory = indexedDbFactory();
  if (factory === null) return null;
  const request = factory.open(DATABASE_NAME, DATABASE_VERSION);
  request.onupgradeneeded = () => {
    const database = request.result;
    if (!database.objectStoreNames.contains(STORE_NAME)) database.createObjectStore(STORE_NAME);
  };
  return await requestResult(request);
}

async function withStore<T>(
  mode: IDBTransactionMode,
  run: (store: IDBObjectStore) => Promise<T>
): Promise<T | null> {
  const database = await openDatabase();
  if (database === null) return null;
  try {
    const transaction = database.transaction(STORE_NAME, mode);
    const completed = new Promise<void>((resolve, reject) => {
      transaction.oncomplete = () => resolve();
      transaction.onabort = () =>
        reject(transaction.error ?? new Error("IndexedDB transaction aborted"));
      transaction.onerror = () =>
        reject(transaction.error ?? new Error("IndexedDB transaction failed"));
    });
    const [result] = await Promise.all([run(transaction.objectStore(STORE_NAME)), completed]);
    return result;
  } finally {
    database.close();
  }
}

/**
 * Reads the record for a workspace. A corrupt or version-mismatched entry is
 * dropped here and reported as absent, so the caller performs a full refetch
 * rather than rendering a half-understood catalog.
 */
export async function readCatalogRecord(
  workspaceId: string,
  profileKey: string
): Promise<CmdPaletteCatalogRecord | null> {
  const workspace = workspaceId.trim();
  const profile = profileKey.trim();
  if (workspace === "" || profile === "") return null;
  let stored: unknown;
  try {
    stored = await withStore("readonly", store =>
      requestResult(store.get(recordKey(workspace, profile)))
    );
  } catch {
    // A browser that refuses storage (private mode, quota, disabled) is a cold
    // open, not an error the operator should ever see.
    return null;
  }
  if (stored === undefined || stored === null) return null;
  const record = parseCatalogRecord(stored);
  if (record === null || record.workspaceId !== workspace || record.profileKey !== profile) {
    await dropCatalogRecord(workspace, profile);
    return null;
  }
  return record;
}

export async function writeCatalogRecord(record: CmdPaletteCatalogRecord): Promise<void> {
  const workspace = record.workspaceId.trim();
  const profile = record.profileKey.trim();
  if (workspace === "" || profile === "") return;
  try {
    await withStore("readwrite", async store => {
      const key = recordKey(workspace, profile);
      const stored = await requestResult(store.get(key));
      if (JSON.stringify(stored) === JSON.stringify(record)) return;
      await requestResult(store.put(record, key));
    });
  } catch {
    // Persisting is an optimisation for the next open; failing it must never
    // fail the hydration that already succeeded.
  }
}

export async function dropCatalogRecord(workspaceId: string, profileKey: string): Promise<void> {
  const workspace = workspaceId.trim();
  const profile = profileKey.trim();
  if (workspace === "" || profile === "") return;
  try {
    await withStore("readwrite", store =>
      requestResult(store.delete(recordKey(workspace, profile)))
    );
  } catch {
    // Same reasoning as the write path: cache hygiene never surfaces.
  }
}
