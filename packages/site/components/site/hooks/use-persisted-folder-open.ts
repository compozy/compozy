"use client";

import { useSyncExternalStore } from "react";
import { getFolderOpenSnapshot, subscribeFolderOpen } from "../sidebar-folder-persist";

export function usePersistedFolderOpen(folderKey: string, fallback: boolean): boolean {
  return useSyncExternalStore(
    onStoreChange => subscribeFolderOpen(folderKey, onStoreChange),
    () => getFolderOpenSnapshot(folderKey, fallback),
    () => fallback
  );
}
