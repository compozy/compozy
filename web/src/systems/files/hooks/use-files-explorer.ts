import type { KeyboardEvent } from "react";
import { useEffect, useState } from "react";

import { primaryShortcutModifier } from "@/systems/os";

import type { FilesRootRef } from "../adapters/files-api";
import type { FileViewerTarget } from "../lib/file-viewer-target";
import { isGoToFileChord, isSearchChord } from "../lib/files-shortcuts";
import { useFileTreeBridge } from "./use-file-tree-bridge";
import { filesRootKey, recordRecentFile, useFilesSearchPrefs } from "./use-files-search-prefs";

export type ExplorerPane = "files" | "search";

/**
 * The explorer's interaction state: tree filter and ignored toggle, the Files ↔ Search pane,
 * Go to file, a revealed folder, the recent-files record, and the explorer-scoped shortcuts
 * (Cmd/Ctrl+P, Cmd/Ctrl+Shift+F) that fire only while focus is inside the explorer.
 */
export function useFilesExplorer(root: FilesRootRef, target: FileViewerTarget | null) {
  const [filter, setFilter] = useState("");
  const [showIgnored, setShowIgnored] = useState(true);
  const [pane, setPane] = useState<ExplorerPane>("files");
  const [goToFileOpen, setGoToFileOpen] = useState(false);
  const [searchFocus, setSearchFocus] = useState(0);
  // "Reveal in tree" asks the tree to reveal the selection again, even after a collapse.
  const [revealCount, setRevealCount] = useState(0);
  // A folder chosen in Go to file stays revealed until the next open request.
  const [folder, setFolder] = useState<{ path: string; nonce: number | null } | null>(null);
  const prefs = useFilesSearchPrefs(root);
  const targetPath = target?.kind === "file" ? target.path : null;
  const targetNonce = target?.nonce ?? null;
  const revealedFolder = folder && folder.nonce === targetNonce ? `${folder.path}/` : null;
  const selectedPath = revealedFolder ?? targetPath;
  const bridge = useFileTreeBridge(root, { showIgnored, revealPath: selectedPath });

  const rootKey = filesRootKey(root);
  // Every open request (a new nonce) moves the file to the top of the recent list.
  useEffect(() => {
    if (targetPath) recordRecentFile(rootKey, targetPath);
  }, [rootKey, targetPath, targetNonce]);

  const openSearch = () => {
    setPane("search");
    setSearchFocus(token => token + 1);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (isGoToFileChord(event)) {
      event.preventDefault();
      setGoToFileOpen(true);
    } else if (isSearchChord(event)) {
      event.preventDefault();
      openSearch();
    }
  };
  const platform = typeof navigator === "undefined" ? "" : navigator.platform;

  return {
    bridge,
    filter,
    setFilter,
    showIgnored,
    setShowIgnored,
    pane,
    setPane: (next: ExplorerPane) => (next === "search" ? openSearch() : setPane(next)),
    searchFocus,
    selectedPath,
    // A new open request (nonce) or "Reveal in tree" reveals the selection; nothing else does.
    revealKey: `${targetNonce ?? ""}:${revealCount}`,
    revealSelection: () => setRevealCount(count => count + 1),
    recent: prefs.recent,
    goToFileOpen,
    openGoToFile: () => setGoToFileOpen(true),
    closeGoToFile: () => setGoToFileOpen(false),
    revealFolder: (path: string) => {
      setPane("files");
      setFilter("");
      setFolder({ path, nonce: targetNonce });
    },
    showFiles: () => setPane("files"),
    onKeyDown,
    primaryKey: primaryShortcutModifier(platform) === "meta" ? "⌘" : "Ctrl+",
  };
}
