"use client";

import type { FileTree as PierreFileTreeModel } from "@pierre/trees";
import { useFileTree } from "@pierre/trees/react";
import * as React from "react";

import {
  ancestorDirectories,
  buildRowDecoration,
  isDirectoryPath,
  parentDirectory,
  planPathSync,
  toGitStatusEntries,
  TREE_SPRITE_SHEET,
  type FileTreeDecorations,
} from "../file-tree-model";

const ITEM_HEIGHT = 26;

export interface UseFileTreeModelOptions {
  decorations?: FileTreeDecorations;
  filter?: string;
  initialExpansion: "closed" | "open";
  loadingPaths?: readonly string[];
  onExpand?: (directoryPath: string) => void;
  onSelect?: (path: string) => void;
  paths: readonly string[];
  resetKey?: string | number;
  selectedPath?: string | null;
}

export interface FileTreeModelState {
  filterMatches: boolean;
  handleClickCapture: (event: React.MouseEvent<HTMLElement>) => void;
  model: PierreFileTreeModel;
}

function selectOnly(model: PierreFileTreeModel, path: string | null | undefined) {
  for (const selected of model.getSelectedPaths()) {
    if (selected !== path) model.getItem(selected)?.deselect();
  }
  if (!path) return;
  const item = model.getItem(path);
  if (item && !item.isSelected()) item.select();
}

function expandAncestors(model: PierreFileTreeModel, path: string) {
  for (const directory of ancestorDirectories(path)) {
    const item = model.getItem(directory);
    if (!item || !("expand" in item)) return;
    if (!item.isExpanded()) item.expand();
  }
}

function rowPathFromEvent(event: Event): string | null {
  for (const node of event.composedPath()) {
    if (!(node instanceof HTMLElement)) continue;
    if (node.dataset.type === "item" && node.dataset.itemPath) return node.dataset.itemPath;
  }
  return null;
}

function useLatest<T>(value: T): React.RefObject<T> {
  const ref = React.useRef(value);
  React.useLayoutEffect(() => {
    ref.current = value;
  });
  return ref;
}

/**
 * Owns the lazy bridge over Pierre's path-first model: it applies `paths`
 * incrementally (`batch`), replaces the model on `resetKey` (`resetPaths`),
 * and reports first expansion of a childless directory through `onExpand`.
 * Pierre consumes its options once, so every changing input is pushed through
 * model methods and callbacks are read through refs.
 */
export function useFileTreeModel({
  decorations,
  filter,
  initialExpansion,
  loadingPaths,
  onExpand,
  onSelect,
  paths,
  resetKey,
  selectedPath,
}: UseFileTreeModelOptions): FileTreeModelState {
  const decorationsRef = useLatest(decorations);
  const loadingRef = useLatest(new Set(loadingPaths));
  const onExpandRef = useLatest(onExpand);
  const onSelectRef = useLatest(onSelect);

  const { model } = useFileTree({
    paths: [],
    initialExpansion,
    flattenEmptyDirectories: false,
    density: "compact",
    itemHeight: ITEM_HEIGHT,
    search: false,
    fileTreeSearchMode: "hide-non-matches",
    icons: { set: "standard", colored: false, spriteSheet: TREE_SPRITE_SHEET },
    composition: { contextMenu: { triggerMode: "right-click" } },
    renderRowDecoration: ({ item }) =>
      buildRowDecoration(decorationsRef.current?.[item.path], loadingRef.current.has(item.path)),
  });

  useLazyExpansion(model, paths, resetKey, onExpandRef);
  useDecorations(model, decorations, loadingPaths, decorationsRef);
  useControlledSelection(model, selectedPath, paths);
  const filterMatches = useFilter(model, filter, paths);

  const handleClickCapture = (event: React.MouseEvent<HTMLElement>) => {
    const path = rowPathFromEvent(event.nativeEvent);
    if (path && !isDirectoryPath(path)) onSelectRef.current?.(path);
  };

  return { filterMatches, handleClickCapture, model };
}

function useLazyExpansion(
  model: PierreFileTreeModel,
  paths: readonly string[],
  resetKey: string | number | undefined,
  onExpandRef: React.RefObject<((directoryPath: string) => void) | undefined>
) {
  // What the model holds, which directories already have children, and which
  // directories were already handed to `onExpand`.
  const knownRef = React.useRef(new Set<string>());
  const parentsRef = React.useRef(new Set<string>());
  const requestedRef = React.useRef(new Set<string>());
  const resetKeyRef = React.useRef(resetKey);

  const notifyExpanded = React.useCallback(() => {
    for (const path of knownRef.current) {
      if (!isDirectoryPath(path) || requestedRef.current.has(path)) continue;
      if (parentsRef.current.has(path)) continue;
      const item = model.getItem(path);
      if (item && "isExpanded" in item && item.isExpanded()) {
        requestedRef.current.add(path);
        onExpandRef.current?.(path);
      }
    }
  }, [model, onExpandRef]);

  React.useLayoutEffect(() => {
    if (resetKeyRef.current !== resetKey) {
      resetKeyRef.current = resetKey;
      model.resetPaths(paths);
      requestedRef.current = new Set();
    } else {
      const operations = planPathSync(knownRef.current, paths);
      if (operations.length > 0) model.batch(operations);
    }
    knownRef.current = new Set(paths);
    parentsRef.current = new Set(paths.map(parentDirectory));
    notifyExpanded();
  }, [model, paths, resetKey, notifyExpanded]);

  React.useEffect(() => model.subscribe(notifyExpanded), [model, notifyExpanded]);
}

function useDecorations(
  model: PierreFileTreeModel,
  decorations: FileTreeDecorations | undefined,
  loadingPaths: readonly string[] | undefined,
  decorationsRef: React.RefObject<FileTreeDecorations | undefined>
) {
  const loadingKey = loadingPaths?.join("\n") ?? "";
  React.useLayoutEffect(() => {
    // The decoration lane reads refs; re-applying the status set is the public
    // way to make the view re-run `renderRowDecoration` for loading changes.
    model.setGitStatus(toGitStatusEntries(decorationsRef.current));
  }, [model, decorations, loadingKey, decorationsRef]);
}

function useControlledSelection(
  model: PierreFileTreeModel,
  selectedPath: string | null | undefined,
  paths: readonly string[]
) {
  React.useLayoutEffect(() => {
    if (selectedPath) expandAncestors(model, selectedPath);
    // A selected directory (`dir/`) is revealed open, so its children show.
    if (selectedPath?.endsWith("/")) {
      const directory = model.getItem(selectedPath);
      if (directory && "expand" in directory && !directory.isExpanded()) directory.expand();
    }
    selectOnly(model, selectedPath);
    if (selectedPath && model.getItem(selectedPath)) {
      model.scrollToPath(selectedPath, { offset: "nearest" });
    }
  }, [model, selectedPath, paths]);
}

function useFilter(
  model: PierreFileTreeModel,
  filter: string | undefined,
  paths: readonly string[]
): boolean {
  const query = filter?.trim() ?? "";
  React.useLayoutEffect(() => {
    model.setSearch(query === "" ? null : query);
  }, [model, query, paths]);
  const subscribe = React.useCallback((listener: () => void) => model.subscribe(listener), [model]);
  return React.useSyncExternalStore(
    subscribe,
    () => query === "" || model.getSearchMatchingPaths().length > 0,
    () => true
  );
}
