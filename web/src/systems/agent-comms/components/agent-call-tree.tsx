/**
 * The Activity tree.
 *
 * Structure comes from the projection; keyboard, focus, and expansion come from
 * `@headless-tree` through `@compozy/ui`'s `Tree` primitives. Nothing about
 * traversal is hand-rolled here — ↑/↓ move, ←/→ fold and unfold, Enter opens the
 * record, and the roving-tabindex and `aria-expanded` bookkeeping that make that
 * work for a screen reader come from the library rather than from this file.
 *
 * Past a threshold the rows are **windowed**. Windowing and keyboard traversal
 * meet at two seams the library provides: `scrollToItem` brings an unmounted row
 * into view when focus moves to it, and `onPrimaryAction` is Enter.
 *
 * The synthetic root is never rendered: it exists only to give the data-loader a
 * single entry point above the per-tree groups.
 */
import { Tree } from "@compozy/ui";

import { AgentCallTreeItem } from "./agent-call-tree-item";
import { AgentCallTreeVirtualWindow } from "./agent-call-tree-virtual-window";
import { useAgentCallTree } from "../hooks/use-agent-call-tree";
import { CALL_TREE_VIRTUALIZATION_THRESHOLD } from "../lib/agent-call-tree-constants";
import type { CallTreeGroupCounts } from "../lib/agent-comms-tree-counts";
import type { CallCommsTree } from "../lib/agent-comms-tree";

export type { CallTreeGroupCounts };

export interface AgentCallTreeProps {
  tree: CallCommsTree;
  /** Human names for root sessions, when the caller knows them. */
  rootLabels?: ReadonlyMap<string, string>;
  /** Counts per tree — derived from a complete loaded population, or absent. */
  countsByRoot?: ReadonlyMap<string, CallTreeGroupCounts>;
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
  selectedCallId = null,
  onSelectCall,
  onStopSubtree,
  pendingStopRootSessionId = null,
  "data-testid": testId,
}: AgentCallTreeProps) {
  const { instance, viewportRef, scroller } = useAgentCallTree(tree, selectedCallId, onSelectCall);
  const rows = instance.getItems().filter(item => item.getItemData().kind !== "root");
  const virtualized = rows.length > CALL_TREE_VIRTUALIZATION_THRESHOLD;
  const itemProps = {
    ...(rootLabels === undefined ? {} : { rootLabels }),
    ...(countsByRoot === undefined ? {} : { countsByRoot }),
    selectedCallId,
    ...(onStopSubtree === undefined ? {} : { onStopSubtree }),
    pendingStopRootSessionId,
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
        {rows.map(item => (
          <AgentCallTreeItem key={item.getId()} item={item} {...itemProps} />
        ))}
      </Tree>
    );
  }

  return (
    <AgentCallTreeVirtualWindow
      tree={instance}
      rows={rows}
      viewportRef={viewportRef}
      scroller={scroller}
      itemProps={itemProps}
      {...(testId === undefined ? {} : { testId })}
    />
  );
}
