import type {
  KnowledgeAgentTier,
  KnowledgeMemoryItem,
  KnowledgeScope,
  MemoryDecisionOp,
  MemoryDecisionSource,
  MemoryType,
} from "@/systems/knowledge/types";

const SCOPE_ORDER: Record<KnowledgeScope, number> = {
  global: 0,
  workspace: 1,
  agent: 2,
};

export function knowledgeMemoryKey(
  memory: Pick<KnowledgeMemoryItem, "filename" | "scope" | "key">
) {
  return memory.key ?? `${memory.scope}:${memory.filename}`;
}

export function compareKnowledgeScope(left: KnowledgeScope, right: KnowledgeScope): number {
  return (SCOPE_ORDER[left] ?? 99) - (SCOPE_ORDER[right] ?? 99);
}

export function knowledgeScopeLabel(scope: KnowledgeScope): string {
  if (scope === "workspace") return "Workspace";
  if (scope === "agent") return "Agent";
  return "Global";
}

export function knowledgeScopeShortLabel(scope: KnowledgeScope): string {
  if (scope === "workspace") return "ws";
  if (scope === "agent") return "agent";
  return "global";
}

export function knowledgeAgentTierLabel(tier: KnowledgeAgentTier): string {
  return tier === "global" ? "Agent · global" : "Agent · workspace";
}

export function knowledgeAgentTierShortLabel(tier: KnowledgeAgentTier): string {
  return tier === "global" ? "ag-global" : "ag-ws";
}

export type KnowledgeTone = MemoryType | KnowledgeScope;

export function memoryTypeTone(type: MemoryType): KnowledgeTone {
  return type;
}

export function memoryScopeTone(scope: KnowledgeScope): KnowledgeTone {
  return scope;
}

const DECISION_OP_LABEL: Record<MemoryDecisionOp, string> = {
  noop: "noop",
  add: "add",
  update: "update",
  delete: "delete",
  reject: "reject",
};

export function decisionOpLabel(op: MemoryDecisionOp): string {
  return DECISION_OP_LABEL[op] ?? op.toLowerCase();
}

export function decisionSourceLabel(source: MemoryDecisionSource): string {
  return source === "rule" ? "rule" : "llm";
}
