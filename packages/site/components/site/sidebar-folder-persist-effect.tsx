"use client";

import { useFolder } from "fumadocs-ui/components/sidebar/base";
import { useEffect, useEffectEvent } from "react";
import { writeFolderOpen } from "./sidebar-folder-persist";

/**
 * Syncs Fumadocs folder open state out to localStorage.
 * Restore is owned by useSyncExternalStore → defaultOpen (no setState-in-effect).
 */
export function FolderOpenPersistWriter({ folderKey }: { folderKey: string }) {
  const folder = useFolder();
  const open = folder?.open ?? false;

  const persist = useEffectEvent((nextOpen: boolean) => {
    writeFolderOpen(folderKey, nextOpen);
  });

  useEffect(() => {
    persist(open);
  }, [open]);

  return null;
}
