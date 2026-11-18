"use client";

import { CodeView } from "@pierre/diffs/react";
import * as React from "react";

import { cn } from "../../lib/utils";
import type { DiffViewerItem, DiffViewerItemStatus } from "./diff-viewer-items";
import { useDiffViewer, type DiffViewerContents } from "./hooks/use-diff-viewer";

export type {
  DiffViewerChangeType,
  DiffViewerItem,
  DiffViewerItemStatus,
} from "./diff-viewer-items";
export type { DiffViewerContents } from "./hooks/use-diff-viewer";

export interface DiffViewerHandle {
  /** Scrolls the item's header to the top of the viewer. */
  scrollToItem: (id: string) => void;
  /** Drops a failed or stale patch and fetches it again. */
  reloadItem: (id: string) => void;
}

export interface DiffViewerFileHeaderState {
  collapsed: boolean;
  status: DiffViewerItemStatus;
}

export interface DiffViewerProps extends Omit<React.ComponentProps<"div">, "children" | "ref"> {
  ref?: React.Ref<DiffViewerHandle>;
  /** Ordered files; `id` is stable and `version` bumps when content changes. */
  items: readonly DiffViewerItem[];
  /**
   * Fetches one file's unified patch when it scrolls near the viewport.
   * Resolve `null` when there is no patch to show. Callers batch requests.
   */
  loadPatch: (item: DiffViewerItem) => Promise<string | null>;
  /** Full old/new contents for expanding unchanged context. Omit to disable expansion. */
  loadContents?: (item: DiffViewerItem) => Promise<DiffViewerContents>;
  mode?: "split" | "unified";
  /** Default collapse state; an item's own `collapsed` wins. */
  collapsed?: boolean;
  /** Ids currently rendered (visible plus overscan), in order. */
  onVisibleItemsChange?: (ids: string[]) => void;
  /** Replaces the per-file header (path, counts, actions, state notes). */
  fileHeader?: (item: DiffViewerItem, state: DiffViewerFileHeaderState) => React.ReactNode;
}

/**
 * Virtualized multi-file diff with split/unified layouts, lazy per-file
 * patches, and expandable context. Patches load as files come into view.
 */
function DiffViewer({
  ref,
  items,
  loadPatch,
  loadContents,
  mode = "unified",
  collapsed = false,
  onVisibleItemsChange,
  fileHeader,
  className,
  ...props
}: DiffViewerProps) {
  const { codeViewItems, handleRef, options, reload, scrollToItem, statusOf, syncVisible } =
    useDiffViewer({ collapsed, items, loadContents, loadPatch, mode, onVisibleItemsChange });

  React.useImperativeHandle(ref, () => ({ scrollToItem, reloadItem: reload }), [
    scrollToItem,
    reload,
  ]);

  const itemsById = new Map(items.map(item => [item.id, item]));

  return (
    <div
      data-slot="diff-viewer"
      data-mode={mode}
      className={cn("flex h-full min-h-0 flex-col bg-canvas", className)}
      {...props}
    >
      <CodeView
        ref={handleRef}
        items={codeViewItems}
        options={options}
        className="min-h-0 flex-1 overflow-auto"
        onScroll={(_, viewer) => syncVisible(viewer)}
        renderCustomHeader={
          fileHeader
            ? codeViewItem => {
                const item = itemsById.get(codeViewItem.id);
                if (!item) return null;
                return fileHeader(item, {
                  collapsed: codeViewItem.collapsed ?? false,
                  status: statusOf(item),
                });
              }
            : undefined
        }
      />
    </div>
  );
}

export { DiffViewer };
