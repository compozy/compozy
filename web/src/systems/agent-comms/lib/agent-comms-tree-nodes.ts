/**
 * Adapts the projected delegation tree into the flat id-addressed shape a
 * headless tree data-loader wants.
 *
 * Two node kinds share one id space, so the ids are prefixed rather than bare:
 * a group's id is its root *session*, a call's id is a *call* id, and those come
 * from different sequences. Prefixing makes a collision impossible and makes any
 * id readable in a DOM inspector.
 *
 * Kept pure and separate from the hook so the mapping can be tested without
 * mounting a tree.
 */
import type { CallCommsTree, CallTreeGroup, CallTreeRow } from "./agent-comms-tree";

/** The synthetic parent every real node hangs from. Never rendered. */
export const CALL_TREE_ROOT_ID = "__calls__";

export type CallTreeNode =
  | { kind: "root" }
  | { kind: "group"; group: CallTreeGroup }
  | { kind: "call"; row: CallTreeRow };

export function groupNodeId(rootSessionId: string): string {
  return `group:${rootSessionId}`;
}

export function callNodeId(callId: string): string {
  return `call:${callId}`;
}

export interface CallTreeDataSource {
  getItem: (id: string) => CallTreeNode;
  getChildren: (id: string) => string[];
  /** Group ids in daemon order. */
  groupIds: string[];
  /**
   * Every node that has children — groups plus the calls that delegated further.
   *
   * This is the initial expansion set. Expanding only the groups would leave a
   * grandchild call hidden behind a caret nobody asked for: the operator opened
   * Activity to see who is helping whom, and a depth-2 row is exactly that.
   * Folding is theirs to do afterwards.
   */
  folderIds: string[];
}

export function buildCallTreeDataSource(tree: CallCommsTree): CallTreeDataSource {
  const nodes = new Map<string, CallTreeNode>([[CALL_TREE_ROOT_ID, { kind: "root" }]]);
  const children = new Map<string, string[]>();
  const groupIds: string[] = [];

  for (const group of tree.groups) {
    const id = groupNodeId(group.rootSessionId);
    groupIds.push(id);
    nodes.set(id, { kind: "group", group });
    children.set(id, group.topLevelCallIds.map(callNodeId));
  }

  const folderIds = [...groupIds];
  for (const [callId, row] of tree.rowsByCallId) {
    const id = callNodeId(callId);
    nodes.set(id, { kind: "call", row });
    children.set(id, row.childCallIds.map(callNodeId));
    if (row.childCallIds.length > 0) folderIds.push(id);
  }

  children.set(CALL_TREE_ROOT_ID, groupIds);

  return {
    // A lookup miss must not throw inside a render pass, so an unknown id
    // resolves to the synthetic root rather than crashing the pane.
    getItem: id => nodes.get(id) ?? { kind: "root" },
    getChildren: id => children.get(id) ?? [],
    groupIds,
    folderIds,
  };
}

/** A group is always a folder; a call is one only when it delegated further. */
export function isCallTreeFolder(node: CallTreeNode): boolean {
  if (node.kind === "root") return true;
  if (node.kind === "group") return true;
  return node.row.childCallIds.length > 0;
}

/** The accessible name a row announces. */
export function callTreeNodeName(node: CallTreeNode): string {
  if (node.kind === "root") return "";
  if (node.kind === "group") return node.group.rootSessionId;
  return node.row.call.agent ?? node.row.call.call_id;
}
