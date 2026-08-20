import type { ReactNode } from "react";

export type HostNodeType =
  | "view-list"
  | "view-list-section"
  | "view-list-item"
  | "view-list-item-detail"
  | "view-list-empty"
  | "view-detail"
  | "view-grid"
  | "view-grid-section"
  | "view-grid-item"
  | "view-form"
  | "view-form-field"
  | "view-form-option"
  | "view-action-panel"
  | "view-action-section"
  | "view-action";

export type HostProps = Record<string, unknown> & { children?: ReactNode };

export interface HostText {
  type: "text";
  value: string;
}

export interface HostNode {
  type: HostNodeType;
  props: HostProps;
  children: HostChild[];
  handlerIDs: Map<string, string>;
  hidden: boolean;
}

export type HostChild = HostNode | HostText;

export interface HostContainer {
  children: HostChild[];
  onCommit: () => void;
}

export type HostChildSet = HostChild[];

export function isHostNode(child: HostChild): child is HostNode {
  return child.type !== "text";
}

export function childNodes(node: HostNode, type?: HostNodeType): HostNode[] {
  const result: HostNode[] = [];
  for (const child of node.children) {
    if (isHostNode(child) && (type === undefined || child.type === type)) {
      result.push(child);
    }
  }
  return result;
}

export function firstChildNode(node: HostNode, type: HostNodeType): HostNode | undefined {
  return childNodes(node, type)[0];
}
