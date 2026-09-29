import type {
  KnowledgeAgentTier,
  KnowledgeMemoryItem,
  KnowledgeScope,
  MemoryDecisionOp,
  MemoryDecisionSource,
  MemoryType,
} from "@/systems/knowledge/types";

const SCOPE_ORDER: Record<KnowledgeScope, number> = {
  profile: 0,
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
  if (scope === "workspace") return "Project";
  if (scope === "agent") return "Agent";
  return "Profile";
}

export function knowledgeAgentTierLabel(tier: KnowledgeAgentTier): string {
  return tier === "global" ? "Agent · all projects" : "Agent · this project";
}

const MEMORY_TYPE_LABEL: Record<MemoryType, string> = {
  user: "About you",
  feedback: "Feedback",
  project: "Project decision",
  reference: "Reference",
};

/** Display label for the wire `MemoryType`; the enum itself never renders. */
export function knowledgeTypeLabel(type: MemoryType): string {
  return MEMORY_TYPE_LABEL[type] ?? type;
}

const DECISION_OP_LABEL: Record<MemoryDecisionOp, string> = {
  noop: "No change",
  add: "Added",
  update: "Updated",
  delete: "Deleted",
  reject: "Rejected",
};

export function decisionOpLabel(op: MemoryDecisionOp): string {
  return DECISION_OP_LABEL[op] ?? op;
}

export function decisionSourceLabel(source: MemoryDecisionSource): string {
  return source === "rule" ? "Rule" : "Automatic";
}
