import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type MemoryHeader = OperationResponse<"listMemory", 200>["memories"][number];
export type MemoryListPage = OperationResponse<"listMemory", 200>;
export type MemoryReadResponse = OperationResponse<"readMemory", 200>;
export type MemoryWriteResponse = OperationResponse<"writeMemory", 200>;
export type MemoryEditResponse = OperationResponse<"editMemory", 200>;
export type MemoryDeleteResponse = OperationResponse<"deleteMemory", 200>;
export type MemoryDreamTriggerResponse = OperationResponse<"triggerMemoryDream", 200>;
export type MemoryDecisionRevertResponse = OperationResponse<"revertMemoryDecision", 200>;

export type MemorySearchRequest = OperationRequestBody<"searchMemory">;
export type MemorySearchResponse = OperationResponse<"searchMemory", 200>;
export type MemorySearchResult = MemorySearchResponse["results"][number];

export type MemoryDecisionsResponse = OperationResponse<"listMemoryDecisions", 200>;
export type MemoryDecision = MemoryDecisionsResponse["decisions"][number];
export type MemoryDecisionOp = MemoryDecision["op"];
export type MemoryDecisionSource = MemoryDecision["source"];

export type MemoryWriteRequest = OperationRequestBody<"writeMemory">;
export type MemoryEditRequest = OperationRequestBody<"editMemory">;
export type MemoryDecisionRevertRequest = OperationRequestBody<"revertMemoryDecision">;

export type MemoryListQuery = NonNullable<OperationQuery<"listMemory">>;
export type MemoryScope = MemoryListQuery["scope"];
export type MemoryAgentTier = MemoryListQuery["agent_tier"];
export type MemorySort = MemoryListQuery["sort"];
export type MemoryType = MemoryHeader["type"];

export type KnowledgeScope = Exclude<MemoryScope, undefined>;
export type KnowledgeAgentTier = Exclude<MemoryAgentTier, undefined>;

/**
 * Locator that uniquely addresses a Memory v2 file across scope, workspace,
 * and the agent two-tier model. `agent_tier` only applies when scope is
 * `agent`; `workspace_id` applies when scope is `workspace` or when
 * `agent_tier` is `workspace`.
 */
export interface KnowledgeSelector {
  scope: KnowledgeScope;
  workspaceId?: string;
  agentName?: string;
  agentTier?: KnowledgeAgentTier;
}

export interface ListMemoryDecisionsParams extends KnowledgeSelector {
  op?: MemoryDecisionOp;
  filename?: string;
  since?: string;
  limit?: number;
}

export interface KnowledgeMemoryItem extends MemoryHeader {
  key?: string;
}

export interface KnowledgeListFilter extends KnowledgeSelector {
  type?: MemoryType;
  sort?: MemorySort;
  cursor?: string;
  limit?: number;
  includeSystem?: boolean;
}

export interface KnowledgeFilter extends KnowledgeListFilter {
  search?: string;
}
