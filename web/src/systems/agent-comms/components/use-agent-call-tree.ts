import { useLayoutEffect, useRef } from "react";

import { hotkeysCoreFeature, selectionFeature, syncDataLoaderFeature } from "@headless-tree/core";
import { useTree } from "@headless-tree/react";
import type { Virtualizer } from "@tanstack/react-virtual";

import type { CallCommsTree } from "../lib/agent-comms-tree";
import {
  buildCallTreeDataSource,
  callNodeId,
  callTreeNodeName,
  isCallTreeFolder,
  CALL_TREE_ROOT_ID,
  type CallTreeNode,
} from "../lib/agent-comms-tree-nodes";

interface AgentCallTreeScroller {
  virtualizer: Virtualizer<HTMLDivElement, Element> | null;
  indexById: Map<string, number>;
}

/** Owns the headless tree's data synchronization, selection, and scroll seam. */
export function useAgentCallTree(
  tree: CallCommsTree,
  selectedCallId: string | null,
  onSelectCall: (callId: string) => void
) {
  const source = buildCallTreeDataSource(tree);
  const previousFolderIds = useRef(new Set<string>());
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const scroller = useRef<AgentCallTreeScroller>({
    virtualizer: null,
    indexById: new Map(),
  });

  const instance = useTree<CallTreeNode>({
    rootItemId: CALL_TREE_ROOT_ID,
    getItemName: item => callTreeNodeName(item.getItemData()),
    isItemFolder: item => isCallTreeFolder(item.getItemData()),
    initialState: { expandedItems: source.folderIds },
    dataLoader: {
      getItem: source.getItem,
      getChildren: source.getChildren,
    },
    features: [syncDataLoaderFeature, selectionFeature, hotkeysCoreFeature],
    onPrimaryAction: item => {
      const node = item.getItemData();
      if (node.kind === "call") onSelectCall(node.row.call.call_id);
    },
    scrollToItem: item => {
      const index = scroller.current.indexById.get(item.getId());
      if (index === undefined) return;
      scroller.current.virtualizer?.scrollToIndex(index, { align: "auto" });
    },
  });

  useLayoutEffect(() => {
    const folderIds = new Set(buildCallTreeDataSource(tree).folderIds);
    // Rebuild first so newly loaded folders have item instances. Expanding
    // through the item API then updates headless-tree's internal state before
    // its next rebuild; `config.setExpandedItems` is React-state-backed and
    // would rebuild once with the old, collapsed state.
    instance.rebuildTree();
    const expandedFolderIds = new Set(instance.getState().expandedItems);
    for (const folderId of folderIds) {
      if (!previousFolderIds.current.has(folderId) && !expandedFolderIds.has(folderId)) {
        instance.getItemInstance(folderId).expand();
      }
    }
    previousFolderIds.current = folderIds;

    if (selectedCallId && tree.rowsByCallId.has(selectedCallId)) {
      const selected = instance.getItemInstance(callNodeId(selectedCallId));
      selected.setFocused();
      void selected.scrollTo({ block: "nearest" });
    }
  }, [instance, selectedCallId, tree]);

  return { instance, viewportRef, scroller };
}
