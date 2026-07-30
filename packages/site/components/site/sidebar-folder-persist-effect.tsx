"use client";

import { useFolder } from "fumadocs-ui/components/sidebar/base";
import { useEffect, useRef } from "react";
import { readFolderOpen, writeFolderOpen } from "./sidebar-folder-persist";

/**
 * Reconciles the server-safe active-route fallback with the user's stored choice, then persists
 * later interactions. Reading before the first write prevents hydration from overwriting an
 * explicit closed preference with the server's active-route default.
 */
export function FolderOpenPersistWriter({ folderKey }: { folderKey: string }) {
  const folder = useFolder();
  const open = folder?.open ?? false;
  const setOpen = folder?.setOpen;
  const initializedKey = useRef<string | null>(null);

  useEffect(() => {
    if (!setOpen) return;

    if (initializedKey.current !== folderKey) {
      initializedKey.current = folderKey;
      const persisted = readFolderOpen(folderKey);
      if (persisted !== null && persisted !== open) {
        setOpen(persisted);
        return;
      }
    }

    writeFolderOpen(folderKey, open);
  }, [folderKey, open, setOpen]);

  return null;
}
