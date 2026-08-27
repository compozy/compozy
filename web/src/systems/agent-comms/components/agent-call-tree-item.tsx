import { TreeItem, TreeItemLabel } from "@compozy/ui";
import type { ItemInstance } from "@headless-tree/core";

import { AgentCallTreeRootRow } from "./agent-call-tree-root-row";
import { AgentCallTreeRow } from "./agent-call-tree-row";
import type { CallTreeGroupCounts } from "../lib/agent-comms-tree-counts";
import type { CallTreeNode } from "../lib/agent-comms-tree-nodes";
import type { ChildState } from "../types";

export interface AgentCallTreeItemProps {
  item: ItemInstance<CallTreeNode>;
  rootLabels?: ReadonlyMap<string, string>;
  countsByRoot?: ReadonlyMap<string, CallTreeGroupCounts>;
  childStates?: ReadonlyMap<string, ChildState>;
  selectedCallId: string | null;
  onStopSubtree?: (rootSessionId: string, profileName: string) => void;
  pendingStopRootSessionId: string | null;
}

export function AgentCallTreeItem({
  item,
  rootLabels,
  countsByRoot,
  childStates,
  selectedCallId,
  onStopSubtree,
  pendingStopRootSessionId,
}: AgentCallTreeItemProps) {
  const node = item.getItemData();
  if (node.kind === "root") return null;
  if (node.kind === "group") {
    const counts = countsByRoot?.get(node.group.rootSessionId);
    return (
      <TreeItem key={item.getId()} item={item} render={<div />} data-testid="agent-call-tree-group">
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
}
