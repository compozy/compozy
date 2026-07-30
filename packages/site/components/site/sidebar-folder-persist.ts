const STORAGE_PREFIX = "compozy.docs.sidebar.folder:";

function storageKey(folderKey: string): string {
  return STORAGE_PREFIX + folderKey;
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
  } catch {
    // Quota / private mode — persistence is best-effort.
  }
}
