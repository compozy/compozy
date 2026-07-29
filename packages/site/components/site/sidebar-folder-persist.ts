const STORAGE_PREFIX = "compozy.docs.sidebar.folder:";

type Listener = () => void;

const listeners = new Map<string, Set<Listener>>();

function storageKey(folderKey: string): string {
  return STORAGE_PREFIX + folderKey;
}

function emit(folderKey: string): void {
  const set = listeners.get(folderKey);
  if (!set) return;
  for (const listener of set) {
    listener();
  }
}

export function folderPersistKey(folder: {
  $id?: string;
  index?: { url?: string };
  name: unknown;
}): string {
  if (folder.$id) return folder.$id;
  if (folder.index?.url) return folder.index.url;
  if (typeof folder.name === "string" && folder.name.length > 0) return folder.name;
  return String(folder.name);
}

export function readFolderOpen(key: string): boolean | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(storageKey(key));
    if (raw === "1") return true;
    if (raw === "0") return false;
    return null;
  } catch {
    return null;
  }
}

export function writeFolderOpen(key: string, open: boolean): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(storageKey(key), open ? "1" : "0");
    emit(key);
  } catch {
    // Quota / private mode — persistence is best-effort.
  }
}

export function subscribeFolderOpen(key: string, onStoreChange: Listener): () => void {
  let set = listeners.get(key);
  if (!set) {
    set = new Set();
    listeners.set(key, set);
  }
  set.add(onStoreChange);

  const onStorage = (event: StorageEvent) => {
    if (event.key === storageKey(key)) onStoreChange();
  };
  if (typeof window !== "undefined") {
    window.addEventListener("storage", onStorage);
  }

  return () => {
    set.delete(onStoreChange);
    if (set.size === 0) listeners.delete(key);
    if (typeof window !== "undefined") {
      window.removeEventListener("storage", onStorage);
    }
  };
}

export function getFolderOpenSnapshot(key: string, fallback: boolean): boolean {
  return readFolderOpen(key) ?? fallback;
}
