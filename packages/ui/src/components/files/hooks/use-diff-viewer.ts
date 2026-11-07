"use client";

import type {
  CodeViewHandle,
  CodeViewItem,
  FileDiffLoadedFiles,
  FileDiffMetadata,
} from "@pierre/diffs/react";
import * as React from "react";

import { useResolvedCodeTheme } from "../../custom/hooks/use-code-block";
import {
  codeViewVersion,
  parseItemPatch,
  placeholderDiff,
  versionKey,
  type DiffViewerItem,
  type DiffViewerItemStatus,
  type DiffViewerPatchEntry,
  withoutContextExpansion,
} from "../diff-viewer-items";
import { PIERRE_CODE_THEMES } from "../pierre-theme";

export interface DiffViewerContents {
  /** `null` when the old side does not exist (pure rename loads only the new side). */
  oldText: string | null;
  newText: string;
}

export interface UseDiffViewerOptions {
  collapsed: boolean;
  items: readonly DiffViewerItem[];
  loadContents?: (item: DiffViewerItem) => Promise<DiffViewerContents>;
  loadPatch: (item: DiffViewerItem) => Promise<string | null>;
  mode: "split" | "unified";
  onVisibleItemsChange?: (ids: string[]) => void;
}

type Handle = CodeViewHandle<undefined, undefined>;
type CodeViewInstance = NonNullable<ReturnType<Handle["getInstance"]>>;

function sameIds(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((id, index) => id === right[index]);
}

function usePatchEntries(
  items: readonly DiffViewerItem[],
  loadPatch: UseDiffViewerOptions["loadPatch"]
) {
  const [entries, setEntries] = React.useState<ReadonlyMap<string, DiffViewerPatchEntry>>(
    () => new Map()
  );
  const itemsRef = React.useRef(items);
  const loadPatchRef = React.useRef(loadPatch);
  const inflightRef = React.useRef(new Map<string, string>());
  React.useLayoutEffect(() => {
    itemsRef.current = items;
    loadPatchRef.current = loadPatch;
  });

  const setEntry = React.useCallback((id: string, entry: DiffViewerPatchEntry) => {
    setEntries(current => new Map(current).set(id, entry));
  }, []);

  const load = React.useCallback(
    (ids: readonly string[], current: ReadonlyMap<string, DiffViewerPatchEntry>) => {
      for (const id of ids) {
        const item = itemsRef.current.find(candidate => candidate.id === id);
        if (!item || item.loadable === false) continue;
        const version = versionKey(item);
        const entry = current.get(id);
        if (entry?.version === version && entry.status !== "pending") continue;
        if (inflightRef.current.get(id) === version) continue;
        inflightRef.current.set(id, version);
        const settle = (next: DiffViewerPatchEntry) => {
          if (inflightRef.current.get(id) === version) inflightRef.current.delete(id);
          const latest = itemsRef.current.find(candidate => candidate.id === id);
          // A newer revision superseded this request; its own load wins.
          if (!latest || versionKey(latest) !== version) return;
          setEntry(id, next);
        };
        loadPatchRef.current(item).then(
          patch => {
            if (patch === null) return settle({ status: "unavailable", version });
            const fileDiff = parseItemPatch(item, patch);
            settle(
              fileDiff ? { fileDiff, status: "ready", version } : { status: "error", version }
            );
          },
          () => settle({ status: "error", version })
        );
      }
    },
    [setEntry]
  );

  const reload = React.useCallback(
    (id: string) => {
      setEntries(current => {
        const next = new Map(current);
        next.delete(id);
        return next;
      });
      load([id], new Map());
    },
    [load]
  );

  return { entries, load, reload };
}

export function useDiffViewer({
  collapsed,
  items,
  loadContents,
  loadPatch,
  mode,
  onVisibleItemsChange,
}: UseDiffViewerOptions) {
  const handleRef = React.useRef<Handle>(null);
  const themeType = useResolvedCodeTheme("auto");
  const { entries, load, reload } = usePatchEntries(items, loadPatch);

  // Placeholders must keep their identity across renders: Pierre's
  // virtualized renderers compare file objects by reference.
  // The caches are memoization only, so they live in state rather than refs.
  const [placeholderCache] = React.useState(() => new Map<string, FileDiffMetadata>());
  const [itemByDiff] = React.useState(() => new WeakMap<FileDiffMetadata, DiffViewerItem>());
  const [lockedDiffs] = React.useState(() => new WeakMap<FileDiffMetadata, FileDiffMetadata>());
  const lockedDiff = React.useCallback(
    (fileDiff: FileDiffMetadata) => {
      let locked = lockedDiffs.get(fileDiff);
      if (!locked) {
        locked = withoutContextExpansion(fileDiff);
        lockedDiffs.set(fileDiff, locked);
      }
      return locked;
    },
    [lockedDiffs]
  );

  const codeViewItems = React.useMemo<CodeViewItem<undefined>[]>(() => {
    const placeholders = new Map<string, FileDiffMetadata>();
    const result = items.map(item => {
      const entry = entries.get(item.id);
      const current = entry?.version === versionKey(item) ? entry : undefined;
      const key = `${item.id}:${versionKey(item)}`;
      const loaded =
        current?.fileDiff && item.expandable === false
          ? lockedDiff(current.fileDiff)
          : current?.fileDiff;
      const fileDiff = loaded ?? placeholderCache.get(key) ?? placeholderDiff(item);
      if (!loaded) placeholders.set(key, fileDiff);
      itemByDiff.set(fileDiff, item);
      const itemCollapsed = item.collapsed ?? collapsed;
      return {
        id: item.id,
        type: "diff" as const,
        fileDiff,
        collapsed: itemCollapsed,
        version: codeViewVersion(item, current, itemCollapsed),
      };
    });
    for (const key of placeholderCache.keys()) {
      if (!placeholders.has(key)) placeholderCache.delete(key);
    }
    for (const [key, fileDiff] of placeholders) placeholderCache.set(key, fileDiff);
    return result;
  }, [items, entries, collapsed, placeholderCache, itemByDiff, lockedDiff]);

  const statusOf = React.useCallback(
    (item: DiffViewerItem): DiffViewerItemStatus => {
      if (item.loadable === false) return "unavailable";
      const entry = entries.get(item.id);
      return entry?.version === versionKey(item) ? entry.status : "pending";
    },
    [entries]
  );

  const visibleRef = React.useRef<string[]>([]);
  const onVisibleRef = React.useRef(onVisibleItemsChange);
  const entriesRef = React.useRef(entries);
  React.useLayoutEffect(() => {
    onVisibleRef.current = onVisibleItemsChange;
    entriesRef.current = entries;
  });

  const syncVisible = React.useCallback(
    (viewer: CodeViewInstance | undefined) => {
      if (!viewer) return;
      const ids = viewer.getRenderedItems().map(rendered => rendered.id);
      load(ids, entriesRef.current);
      if (sameIds(ids, visibleRef.current)) return;
      visibleRef.current = ids;
      onVisibleRef.current?.(ids);
    },
    [load]
  );

  // CodeView renders its window asynchronously, so visibility is read after
  // each item render (coalesced to one frame) as well as on scroll.
  const frameRef = React.useRef<number | null>(null);
  const scheduleSync = React.useCallback(() => {
    if (frameRef.current !== null) return;
    frameRef.current = requestAnimationFrame(() => {
      frameRef.current = null;
      syncVisible(handleRef.current?.getInstance());
    });
  }, [syncVisible]);
  React.useEffect(() => {
    scheduleSync();
  }, [codeViewItems, scheduleSync]);
  React.useEffect(
    () => () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
    },
    []
  );

  const loadContentsRef = React.useRef(loadContents);
  React.useLayoutEffect(() => {
    loadContentsRef.current = loadContents;
  });
  const hasContents = loadContents !== undefined;
  const options = React.useMemo(
    () => ({
      theme: PIERRE_CODE_THEMES,
      themeType,
      diffStyle: mode,
      stickyHeaders: true,
      hunkSeparators: "line-info" as const,
      lineDiffType: "word-alt" as const,
      onPostRender: scheduleSync,
      ...(hasContents
        ? {
            loadDiffFiles: async (fileDiff: FileDiffMetadata): Promise<FileDiffLoadedFiles> => {
              const item = itemByDiff.get(fileDiff);
              const loader = loadContentsRef.current;
              if (!item || !loader || item.expandable === false) {
                throw new Error("diff contents are unavailable");
              }
              const { oldText, newText } = await loader(item);
              const cacheKey = `${item.id}:${versionKey(item)}`;
              const newFile = { name: item.path, contents: newText, cacheKey: `${cacheKey}:new` };
              if (oldText === null) return { oldFile: null, newFile };
              const oldName = item.prevPath ?? item.path;
              return {
                oldFile: { name: oldName, contents: oldText, cacheKey: `${cacheKey}:old` },
                newFile,
              };
            },
          }
        : {}),
    }),
    [themeType, mode, hasContents, itemByDiff, scheduleSync]
  );

  const scrollToItem = React.useCallback((id: string) => {
    handleRef.current?.scrollTo({ type: "item", id, align: "start" });
  }, []);

  return { codeViewItems, handleRef, options, reload, scrollToItem, statusOf, syncVisible };
}
