import { useLayoutEffect, type RefObject } from "react";

import { observeElementRect, useVirtualizer } from "@tanstack/react-virtual";
import type { ItemInstance, TreeInstance } from "@headless-tree/core";

import { Tree } from "@compozy/ui";

import { AgentCallTreeItem, type AgentCallTreeItemProps } from "./agent-call-tree-item";
import type { AgentCallTreeScroller } from "../hooks/use-agent-call-tree";
import type { CallTreeNode } from "../lib/agent-comms-tree-nodes";

/** Fixed comfortable single-line row estimate plus its gap. */
const ROW_ESTIMATE = 34;
const OVERSCAN = 12;

interface AgentCallTreeVirtualWindowProps {
  tree: TreeInstance<CallTreeNode>;
  rows: readonly ItemInstance<CallTreeNode>[];
  viewportRef: RefObject<HTMLDivElement | null>;
  scroller: RefObject<AgentCallTreeScroller>;
  itemProps: Omit<AgentCallTreeItemProps, "item">;
  testId?: string;
}

export function AgentCallTreeVirtualWindow({
  tree,
  rows,
  viewportRef,
  scroller,
  itemProps,
  testId,
}: AgentCallTreeVirtualWindowProps) {
  "use no memo";

  // react-doctor-disable-next-line react-hooks-js/incompatible-library -- `use no memo` is the explicit React Compiler boundary for TanStack Virtual's mutable methods.
  // oxlint-disable-next-line react/incompatible-library -- `use no memo` isolates TanStack Virtual's mutable methods; remove the boundary when TanStack ships a compiler-compatible API.
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => viewportRef.current,
    getItemKey: index => rows[index]?.getId() ?? index,
    estimateSize: () => ROW_ESTIMATE,
    overscan: OVERSCAN,
    enabled: true,
    initialRect: { width: 0, height: ROW_ESTIMATE },
    observeElementRect: (instance, callback) =>
      observeElementRect(instance, rect => {
        if (rect.height > 0) callback(rect);
      }),
    useFlushSync: false,
  });

  const rowIds = rows.map(item => item.getId()).join("\u0000");
  useLayoutEffect(() => {
    scroller.current.virtualizer = virtualizer;
    const indexById = new Map<string, number>();
    rows.forEach((item, index) => indexById.set(item.getId(), index));
    scroller.current.indexById = indexById;
  }, [rowIds, rows, scroller, virtualizer]);

  const window = virtualizer.getVirtualItems();
  const first = window[0];
  const last = window[window.length - 1];
  const paddingTop = first ? first.start : 0;
  const paddingBottom = last ? virtualizer.getTotalSize() - last.end : 0;

  return (
    <div
      ref={viewportRef}
      className="max-h-[60vh] overflow-y-auto"
      data-testid={testId ? `${testId}-viewport` : undefined}
    >
      <Tree
        tree={tree}
        indent={0}
        aria-label="Delegation activity"
        data-testid={testId}
        data-virtualized="true"
        className="gap-0.5"
      >
        {paddingTop > 0 ? <div style={{ height: paddingTop }} aria-hidden="true" /> : null}
        {window.map(virtualRow => {
          const item = rows[virtualRow.index];
          return item ? <AgentCallTreeItem key={item.getId()} item={item} {...itemProps} /> : null;
        })}
        {paddingBottom > 0 ? <div style={{ height: paddingBottom }} aria-hidden="true" /> : null}
      </Tree>
    </div>
  );
}
