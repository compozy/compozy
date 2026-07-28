import type { AgentPayload } from "../types";

export interface AgentCategoryFolderNode {
  kind: "folder";
  id: string;
  label: string;
  segments: string[];
  children: AgentCategoryNode[];
}

export interface AgentCategoryLeafNode {
  kind: "leaf";
  id: string;
  label: string;
  agent: AgentPayload;
}

export type AgentCategoryNode = AgentCategoryFolderNode | AgentCategoryLeafNode;

const FOLDER_ID_PREFIX = "category:";
const LEAF_ID_PREFIX = "agent:";
const CATEGORY_LABEL_SEPARATOR = " / ";

interface MutableFolder {
  kind: "folder";
  segments: string[];
  childrenByKey: Map<string, MutableFolder>;
  agents: AgentPayload[];
}

function createFolder(segments: string[]): MutableFolder {
  return {
    kind: "folder",
    segments,
    childrenByKey: new Map(),
    agents: [],
  };
}

function isCategoryPathRootLevel(path: string[] | null | undefined): boolean {
  return !Array.isArray(path) || path.length === 0;
}

function compareCaseInsensitive(a: string, b: string): number {
  const aLower = a.toLowerCase();
  const bLower = b.toLowerCase();
  if (aLower === bLower) return a.localeCompare(b);
  return aLower < bLower ? -1 : 1;
}

function compareSiblings(a: AgentCategoryNode, b: AgentCategoryNode): number {
  if (a.kind !== b.kind) {
    return a.kind === "folder" ? -1 : 1;
  }
  return compareCaseInsensitive(a.label, b.label);
}

function buildLeaf(agent: AgentPayload): AgentCategoryLeafNode {
  return {
    kind: "leaf",
    id: `${LEAF_ID_PREFIX}${agent.name}`,
    label: agent.name,
    agent,
  };
}

function materializeFolder(folder: MutableFolder): AgentCategoryFolderNode {
  if (folder.segments.length === 0) {
    throw new Error("agent category folder requires at least one segment");
  }
  const label = folder.segments[folder.segments.length - 1];
  if (!label) {
    throw new Error("agent category folder requires a label segment");
  }
  const childFolders = Array.from(folder.childrenByKey.values()).map(materializeFolder);
  const childLeaves = folder.agents.map(buildLeaf);
  const children: AgentCategoryNode[] = [...childFolders, ...childLeaves];
  children.sort(compareSiblings);
  return {
    kind: "folder",
    id: `${FOLDER_ID_PREFIX}${folder.segments.join("/")}`,
    label,
    segments: folder.segments,
    children,
  };
}

export function buildAgentCategoryTree(agents: AgentPayload[]): AgentCategoryNode[] {
  const root = createFolder([]);
  const rootAgents: AgentPayload[] = [];
  for (const agent of agents) {
    const path = agent.category_path;
    if (isCategoryPathRootLevel(path)) {
      rootAgents.push(agent);
      continue;
    }
    let cursor = root;
    const trail: string[] = [];
    for (const segment of path as string[]) {
      trail.push(segment);
      let next = cursor.childrenByKey.get(segment);
      if (!next) {
        next = createFolder([...trail]);
        cursor.childrenByKey.set(segment, next);
      }
      cursor = next;
    }
    cursor.agents.push(agent);
  }
  const folderNodes = Array.from(root.childrenByKey.values()).map(materializeFolder);
  const rootLeaves = rootAgents.map(buildLeaf);
  const result: AgentCategoryNode[] = [...folderNodes, ...rootLeaves];
  result.sort(compareSiblings);
  return result;
}

export function formatCategoryLabel(path: string[] | null | undefined): string {
  if (isCategoryPathRootLevel(path)) return "";
  return (path as string[]).join(CATEGORY_LABEL_SEPARATOR);
}

export function getAgentCategoryFolderId(segments: string[]): string {
  return `${FOLDER_ID_PREFIX}${segments.join("/")}`;
}

export function getAgentLeafId(agentName: string): string {
  return `${LEAF_ID_PREFIX}${agentName}`;
}

export function joinAgentCategorySegments(segments: string[]): string {
  return segments.join("/");
}

export function isAgentRootLevel(agent: AgentPayload): boolean {
  return isCategoryPathRootLevel(agent.category_path);
}

export const AGENT_CATEGORY_FOLDER_ID_PREFIX = FOLDER_ID_PREFIX;
export const AGENT_CATEGORY_LEAF_ID_PREFIX = LEAF_ID_PREFIX;
export const AGENT_CATEGORY_LABEL_SEPARATOR = CATEGORY_LABEL_SEPARATOR;
