"use client";

import type { FileTree as PierreFileTreeModel } from "@pierre/trees";
import { useFileTree } from "@pierre/trees/react";
import { useReducedMotionConfig } from "motion/react";
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
  onExpandedChange?: (directoryPaths: readonly string[]) => void;
  onSelect?: (path: string) => void;
  paths: readonly string[];
  resetKey?: string | number;
  selectedPath?: string | null;
  revealKey?: string | number;
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
  onExpandedChange,
  onSelect,
  paths,
  resetKey,
  selectedPath,
  revealKey,
}: UseFileTreeModelOptions): FileTreeModelState {
  const decorationsRef = useLatest(decorations);
  const loadingRef = useLatest(new Set(loadingPaths));
  const onExpandRef = useLatest(onExpand);
  const onExpandedChangeRef = useLatest(onExpandedChange);
  // Under UIProvider's motion config: reduced motion shows a still loading arc.
  const reducedMotionRef = useLatest(useReducedMotionConfig() === true);
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
      buildRowDecoration(
        decorationsRef.current?.[item.path],
        loadingRef.current.has(item.path),
        reducedMotionRef.current
      ),
  });

  useLazyExpansion(model, paths, resetKey, onExpandRef, onExpandedChangeRef);
  useDecorations(model, decorations, loadingPaths, decorationsRef);
  useControlledSelection(model, selectedPath, revealKey, paths);
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
  onExpandRef: React.RefObject<((directoryPath: string) => void) | undefined>,
  onExpandedChangeRef: React.RefObject<((directoryPaths: readonly string[]) => void) | undefined>
) {
  // What the model holds, which directories already have children, and which
  // expanded, still childless directories were already handed to `onExpand`.
  const knownRef = React.useRef(new Set<string>());
  const parentsRef = React.useRef(new Set<string>());
  const requestedRef = React.useRef(new Set<string>());
  const resetKeyRef = React.useRef(resetKey);
  const expandedKeyRef = React.useRef("");

  const notifyExpanded = React.useCallback(() => {
    const open: string[] = [];
    for (const path of knownRef.current) {
      if (!isDirectoryPath(path)) continue;
      const item = model.getItem(path);
      const expanded = Boolean(item && "isExpanded" in item && item.isExpanded());
      if (expanded) open.push(path);
      if (parentsRef.current.has(path)) continue;
      // A folder that got no children (its listing failed, or is still on the way) may ask
      // again: collapsing it re-arms `onExpand` for the next expansion.
      if (!expanded) {
        requestedRef.current.delete(path);
        continue;
      }
      if (requestedRef.current.has(path)) continue;
      requestedRef.current.add(path);
      onExpandRef.current?.(path);
    }
    // The expanded set, reported only when it changes: the bridge keeps collapsed folders idle.
    const key = open.sort().join("\n");
    if (key !== expandedKeyRef.current) {
      expandedKeyRef.current = key;
      onExpandedChangeRef.current?.(open);
    }
  }, [model, onExpandRef, onExpandedChangeRef]);

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

/**
 * Selects `selectedPath` and reveals it when the reveal request changes (a new path or a new
 * `revealKey`). A lazy reveal keeps expanding ancestors as their listings arrive, then stops
 * once the row exists, so later path changes never undo a user's collapse.
 */
function useControlledSelection(
  model: PierreFileTreeModel,
  selectedPath: string | null | undefined,
  revealKey: string | number | undefined,
  paths: readonly string[]
) {
  const request = `${selectedPath ?? ""}\u0000${revealKey ?? ""}`;
  const pendingRef = React.useRef<string | null>(null);
  const seenRequestRef = React.useRef<string | null>(null);
  React.useLayoutEffect(() => {
    if (seenRequestRef.current !== request) {
      seenRequestRef.current = request;
      pendingRef.current = selectedPath ? request : null;
    }
    if (selectedPath && pendingRef.current === request) {
      expandAncestors(model, selectedPath);
      // A selected directory (`dir/`) is revealed open, so its children show.
      if (selectedPath.endsWith("/")) {
        const directory = model.getItem(selectedPath);
        if (directory && "expand" in directory && !directory.isExpanded()) directory.expand();
      }
      if (model.getItem(selectedPath)) {
        model.scrollToPath(selectedPath, { offset: "nearest" });
        pendingRef.current = null;
      }
    }
    selectOnly(model, selectedPath);
  }, [model, selectedPath, request, paths]);
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
