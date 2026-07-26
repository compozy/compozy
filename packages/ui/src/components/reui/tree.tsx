"use client";

import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import type { ItemInstance, TreeInstance } from "@headless-tree/core";
import * as React from "react";

import { cn } from "@agh/ui/lib/utils";
import { ChevronDownIcon, MinusIcon, PlusIcon } from "lucide-react";
import { TreeContext, type ToggleIconType, useTreeContext } from "./hooks/use-tree-context";

interface TreeProps<T> extends Omit<React.HTMLAttributes<HTMLDivElement>, "children"> {
  indent?: number;
  tree: TreeInstance<T>;
  toggleIconType?: ToggleIconType;
  children?: React.ReactNode;
}

function Tree<T>({
  indent = 20,
  tree,
  className,
  toggleIconType = "chevron",
  ...props
}: TreeProps<T>) {
  const containerProps = tree.getContainerProps();
  const { style: mergedContainerStyle, ...otherProps } = mergeProps<"div">(containerProps, props);

  const mergedStyle = {
    ...mergedContainerStyle,
    "--tree-indent": `${indent}px`,
  } as React.CSSProperties;
  const contextValue = { indent, tree, toggleIconType };

  return (
    <TreeContext.Provider value={contextValue}>
      <div
        data-slot="tree"
        style={mergedStyle}
        className={cn("flex flex-col", className)}
        {...otherProps}
      />
    </TreeContext.Provider>
  );
}

interface TreeItemProps<T> extends Omit<useRender.ComponentProps<"button">, "indent"> {
  item: ItemInstance<T>;
  indent?: number;
}

function TreeItem<T>({ item, className, render, children, ...props }: TreeItemProps<T>) {
  const parentContext = useTreeContext<T>();
  const { indent } = parentContext;

  const itemProps = item.getProps();
  const { style: mergedItemStyle, ...otherProps } = mergeProps<"button">(itemProps, {
    ...props,
    children,
  });

  const mergedStyle = {
    ...mergedItemStyle,
    "--tree-padding": `${item.getItemMeta().level * indent}px`,
  } as React.CSSProperties;

  // Feature methods (drag/search/selection) only exist when the matching
  // feature is registered with useTree. Guard the optional features so trees
  // without them (e.g. selection-only) don't crash at render time.
  const isFolder = item.isFolder();
  const focused = item.isFocused?.() ?? false;
  const selected = item.isSelected?.() ?? false;
  const dragTarget = item.isDragTarget?.();
  const searchMatch = item.isMatchingSearch?.();
  const defaultProps = {
    "data-slot": "tree-item",
    type: "button" as const,
    style: mergedStyle,
    className: cn(
      "z-10 ps-(--tree-padding) outline-hidden select-none focus:z-20 data-disabled:pointer-events-none data-disabled:opacity-50",
      className
    ),
    "data-focus": focused,
    "data-folder": isFolder,
    "data-selected": selected,
    "data-drag-target": dragTarget,
    "data-search-match": searchMatch,
    "aria-expanded": isFolder ? item.isExpanded() : undefined,
  };
  const contextValue = { ...parentContext, currentItem: item };

  return (
    <TreeContext.Provider value={contextValue}>
      {useRender({
        defaultTagName: "button",
        render,
        props: mergeProps<"button">(defaultProps, otherProps),
      })}
    </TreeContext.Provider>
  );
}

interface TreeItemLabelProps<T> extends React.HTMLAttributes<HTMLSpanElement> {
  item?: ItemInstance<T>;
}

function TreeItemLabel<T>({
  item: propItem,
  children,
  className,
  ...props
}: TreeItemLabelProps<T>) {
  const { currentItem, toggleIconType } = useTreeContext<T>();
  const item = propItem ?? currentItem;

  if (!item) return null;

  const isFolder = item.isFolder();
  const isExpanded = item.isExpanded();

  return (
    <span
      data-slot="tree-item-label"
      className={cn(
        "flex items-center gap-1 transition-colors not-in-data-[folder=true]:ps-7 bg-transparent text-fg hover:bg-hover in-data-[selected=true]:bg-elevated in-data-[selected=true]:text-fg-strong in-data-[drag-target=true]:bg-accent-tint in-data-[search-match=true]:bg-info-tint in-focus-visible:outline-none in-focus-visible:shadow-focus-ring [&_svg]:pointer-events-none [&_svg]:shrink-0",
        "rounded-sm",
        "py-1.5",
        "px-2",
        "text-small-body",
        className
      )}
      {...props}
    >
      {isFolder &&
        (toggleIconType === "plus-minus" ? (
          isExpanded ? (
            <MinusIcon className="text-muted size-3" stroke="currentColor" strokeWidth="1" />
          ) : (
            <PlusIcon className="text-muted size-3" stroke="currentColor" strokeWidth="1" />
          )
        ) : (
          <ChevronDownIcon className="text-muted size-3 in-aria-[expanded=false]:-rotate-90" />
        ))}
      {children ?? item.getItemName()}
    </span>
  );
}

interface TreeDragLineProps<T> extends React.HTMLAttributes<HTMLDivElement> {
  tree?: TreeInstance<T>;
}

function TreeDragLine<T>({ className, tree: propTree, ...props }: TreeDragLineProps<T>) {
  const context = useTreeContext<T>();
  const tree = propTree ?? context.tree;

  if (!tree?.getDragLineStyle) {
    return null;
  }

  const dragLine = tree.getDragLineStyle();
  return (
    <div
      style={dragLine}
      className={cn(
        "bg-accent before:bg-canvas before:border-accent absolute z-30 -mt-px h-0.5 w-[unset] before:absolute before:-top-0.75 before:left-0 before:size-2 before:border-2",
        "before:rounded-full",
        className
      )}
      {...props}
    />
  );
}

export { Tree, TreeDragLine, TreeItem, TreeItemLabel };
export type { TreeDragLineProps, TreeItemLabelProps, TreeItemProps, TreeProps };
