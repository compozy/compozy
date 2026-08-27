/**
 * The Activity tree.
 *
 * Structure comes from the projection; keyboard, focus, and expansion come from
 * `@headless-tree` through `@compozy/ui`'s `Tree` primitives. Nothing about
 * traversal is hand-rolled here — ↑/↓ move, ←/→ fold and unfold, Enter opens the
 * record, and the roving-tabindex and `aria-expanded` bookkeeping that make that
 * work for a screen reader come from the library rather than from this file.
 *
 * Past a threshold the rows are **windowed**: only what is near the viewport is
 * mounted, with spacer blocks standing in for the rest of the scroll height. The
 * spacer shape is deliberate — rows stay in document order inside the tree
 * container, which is what the roving tabindex and `aria-*` relationships depend
 * on. Absolutely-positioned rows would render at the right pixel and the wrong
 * place in the accessibility tree.
 *
 * Windowing and keyboard traversal meet at two seams the library provides rather
 * than at anything invented here: `scrollToItem` brings an unmounted row into
 * view when focus moves to it, and `onPrimaryAction` is Enter. Without the
 * first, ↓ would walk into rows that do not exist yet.
 *
 * The synthetic root is never rendered: it exists only to give the data-loader a
 * single entry point above the per-tree groups.
 */
import { useLayoutEffect } from "react";

import { useVirtualizer } from "@tanstack/react-virtual";

import { Tree, TreeItem, TreeItemLabel } from "@compozy/ui";

import { AgentCallTreeRootRow } from "./agent-call-tree-root-row";
import { AgentCallTreeRow } from "./agent-call-tree-row";
import { useAgentCallTree } from "./use-agent-call-tree";
import { CALL_TREE_VIRTUALIZATION_THRESHOLD } from "../lib/agent-call-tree-constants";
import type { CallCommsTree } from "../lib/agent-comms-tree";
import type { ChildState } from "../types";

/** Fixed comfortable single-line row estimate plus its gap. */
const ROW_ESTIMATE = 34;
const OVERSCAN = 12;

/** Per-tree counts, keyed by root session id. */
export interface CallTreeGroupCounts {
  total?: number;
  running?: number;
  needsYou?: number;
}

export interface AgentCallTreeProps {
  tree: CallCommsTree;
  /** Human names for root sessions, when the caller knows them. */
  rootLabels?: ReadonlyMap<string, string>;
  /** Daemon counts per tree — never derived from the rendered rows. */
  countsByRoot?: ReadonlyMap<string, CallTreeGroupCounts>;
  /**
   * What became of each child, keyed by child session id.
   *
   * Sparse on purpose: a child whose tree catalog has not finished loading has
   * no entry, and its row shows no child-state pill rather than a guess.
   */
  childStates?: ReadonlyMap<string, ChildState>;
  selectedCallId?: string | null;
  onSelectCall: (callId: string) => void;
  /** Absent when the operator cannot drain trees. */
  onStopSubtree?: (rootSessionId: string, profileName: string) => void;
  pendingStopRootSessionId?: string | null;
  "data-testid"?: string;
}

export function AgentCallTree({
  tree,
  rootLabels,
  countsByRoot,
  childStates,
  selectedCallId = null,
  onSelectCall,
  onStopSubtree,
  pendingStopRootSessionId = null,
  "data-testid": testId,
}: AgentCallTreeProps) {
  // The tree's `scrollToItem` seam needs the virtualizer, which needs the row
  // list, which needs the tree. The holder returned by the hook breaks that
  // cycle and is filled after the first paint, before a keystroke can arrive.
  const { instance, viewportRef, scroller } = useAgentCallTree(tree, selectedCallId, onSelectCall);

  // The synthetic root is dropped here so every downstream index — virtualizer
  // and `scrollToItem` alike — refers to a row that actually renders.
  const rows = instance.getItems().filter(item => item.getItemData().kind !== "root");
  const virtualized = rows.length > CALL_TREE_VIRTUALIZATION_THRESHOLD;

  // oxlint-disable-next-line react/incompatible-library -- virtualizer state is isolated inside this compiler boundary.
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => viewportRef.current,
    getItemKey: index => rows[index]?.getId() ?? index,
    estimateSize: () => ROW_ESTIMATE,
    overscan: OVERSCAN,
    enabled: virtualized,
    useFlushSync: false,
  });

  useLayoutEffect(() => {
    scroller.current.virtualizer = virtualizer;
    const indexById = new Map<string, number>();
    rows.forEach((item, index) => indexById.set(item.getId(), index));
    scroller.current.indexById = indexById;
  });

  const renderRow = (item: (typeof rows)[number]) => {
    const node = item.getItemData();
    // Filtered out above; narrowing needs it stated.
    if (node.kind === "root") return null;
    if (node.kind === "group") {
      const counts = countsByRoot?.get(node.group.rootSessionId);
      return (
        <TreeItem
          key={item.getId()}
          item={item}
          render={<div />}
          data-testid="agent-call-tree-group"
        >
          <TreeItemLabel item={item} className="p-0">
            <AgentCallTreeRootRow
              rootSessionId={node.group.rootSessionId}
              rootLabel={rootLabels?.get(node.group.rootSessionId) ?? null}
              totalCalls={counts?.total}
              runningCalls={counts?.running}
              needsYouCalls={counts?.needsYou}
              escalation={node.group.escalation}
              stopPending={pendingStopRootSessionId === node.group.rootSessionId}
              {...(onStopSubtree
                ? {
                    onStopSubtree: () =>
                      onStopSubtree(
                        node.group.rootSessionId,
                        node.group.rows[0]?.call.profile_name ?? ""
                      ),
                  }
                : {})}
            />
          </TreeItemLabel>
        </TreeItem>
      );
    }

    const callId = node.row.call.call_id;
    // No `onClick` here: the row's own handler already routes to
    // `onPrimaryAction`, so click and Enter reach the record through one path.
    // Adding a second would open the record twice per click.
    return (
      <TreeItem key={item.getId()} item={item} data-testid="agent-call-tree-row">
        <TreeItemLabel item={item} className="p-0 not-in-data-[folder=true]:ps-0">
          <AgentCallTreeRow
            row={node.row}
            depth={node.row.depth}
            childState={childStates?.get(node.row.call.child_session_id ?? "") ?? null}
            selected={selectedCallId === callId}
          />
        </TreeItemLabel>
      </TreeItem>
    );
  };

  if (!virtualized) {
    return (
      <Tree
        tree={instance}
        indent={0}
        aria-label="Delegation activity"
        data-testid={testId}
        className="gap-0.5"
      >
        {rows.map(renderRow)}
      </Tree>
    );
  }

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
        tree={instance}
        indent={0}
        aria-label="Delegation activity"
        data-testid={testId}
        data-virtualized="true"
        className="gap-0.5"
      >
        {paddingTop > 0 ? <div style={{ height: paddingTop }} aria-hidden="true" /> : null}
        {window.map(virtualRow => {
          const item = rows[virtualRow.index];
          return item ? renderRow(item) : null;
        })}
        {paddingBottom > 0 ? <div style={{ height: paddingBottom }} aria-hidden="true" /> : null}
      </Tree>
    </div>
  );
}
