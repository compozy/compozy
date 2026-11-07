"use client";

import { FileTree as PierreFileTree } from "@pierre/trees/react";
import type * as React from "react";

import { cn } from "../../lib/utils";
import type { FileTreeDecorations } from "./file-tree-model";
import { FileTreeRowMenu } from "./file-tree-row-menu";
import { useFileTreeModel } from "./hooks/use-file-tree-model";

export type { FileTreeDecoration, FileTreeDecorations, FileTreeStatus } from "./file-tree-model";

export interface FileTreeRowMenuItem {
  kind: "directory" | "file";
  name: string;
  path: string;
}

export interface FileTreeRowMenuControls {
  close: () => void;
}

export interface FileTreeProps extends Omit<
  React.ComponentProps<"div">,
  "children" | "onSelect" | "aria-label"
> {
  /** Accessible name for the tree. */
  "aria-label": string;
  /**
   * Root-relative tree paths; directories end in `/`. Pass the loaded set as
   * it grows — the tree applies additions and removals incrementally.
   */
  paths: readonly string[];
  /** Directory expansion when paths first appear. Lazy explorers keep `closed`. */
  initialExpansion?: "closed" | "open";
  /**
   * Fires when a directory with no loaded children is expanded. Once its children load,
   * collapsing and re-expanding does not fire again until `resetKey` changes; a directory that
   * is still childless (its listing failed) fires again on its next expansion.
   */
  onExpand?: (directoryPath: string) => void;
  /** Directories whose children are being fetched; they show a spinner. */
  loadingPaths?: readonly string[];
  /** Fires for every file activation, including re-clicking the selected row. */
  onSelect?: (path: string) => void;
  /**
   * Controlled selection. The tree selects it, expands its loaded ancestors
   * (which fires `onExpand` for unloaded ones), and scrolls it into view. A
   * directory (`dir/`) is expanded too.
   */
  selectedPath?: string | null;
  decorations?: FileTreeDecorations;
  /** Hides rows that do not match; matching runs over loaded paths only. */
  filter?: string;
  /** Shown over the tree when `filter` matches nothing. */
  filterEmpty?: React.ReactNode;
  /** Shown instead of the tree when `paths` is empty. */
  empty?: React.ReactNode;
  /**
   * Row context menu items (`DropdownMenuItem`, `DropdownMenuSeparator`, …), opened by
   * right-click, Shift+F10, or the ContextMenu key. The tree renders them in a menu at the row;
   * return `null` for a row without a menu.
   */
  rowMenu?: (item: FileTreeRowMenuItem, controls: FileTreeRowMenuControls) => React.ReactNode;
  /** Changing it replaces the whole model (root switch or full refresh). */
  resetKey?: string | number;
}

function FileTree({
  "aria-label": ariaLabel,
  paths,
  initialExpansion = "closed",
  onExpand,
  loadingPaths,
  onSelect,
  selectedPath,
  decorations,
  filter,
  filterEmpty,
  empty,
  rowMenu,
  resetKey,
  className,
  ...props
}: FileTreeProps) {
  const { filterMatches, handleClickCapture, model } = useFileTreeModel({
    decorations,
    filter,
    initialExpansion,
    loadingPaths,
    onExpand,
    onSelect,
    paths,
    resetKey,
    selectedPath,
  });
  const isEmpty = paths.length === 0;

  return (
    <div
      data-slot="file-tree"
      className={cn("relative flex h-full min-h-0 flex-col", className)}
      onClickCapture={handleClickCapture}
      {...props}
    >
      <PierreFileTree
        model={model}
        aria-label={ariaLabel}
        className={cn("block min-h-0 flex-1", isEmpty && "hidden")}
        renderContextMenu={
          rowMenu
            ? (item, context) => {
                const items = rowMenu(
                  { kind: item.kind, name: item.name, path: item.path },
                  { close: () => context.close() }
                );
                if (items === null || items === undefined) return null;
                return (
                  <FileTreeRowMenu
                    anchor={{ getBoundingClientRect: () => DOMRect.fromRect(context.anchorRect) }}
                    label={`Actions for ${item.name}`}
                    onClose={() => context.close()}
                  >
                    {items}
                  </FileTreeRowMenu>
                );
              }
            : undefined
        }
      />
      {isEmpty ? empty : null}
      {!isEmpty && !filterMatches ? (
        <div data-slot="file-tree-filter-empty" className="absolute inset-x-0 top-0">
          {filterEmpty}
        </div>
      ) : null}
    </div>
  );
}

export { FileTree };
