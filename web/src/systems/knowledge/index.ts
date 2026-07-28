// Types
export type {
  KnowledgeAgentTier,
  KnowledgeFilter,
  KnowledgeListFilter,
  KnowledgeMemoryItem,
  KnowledgeScope,
  KnowledgeSelector,
  ListMemoryDecisionsParams,
  MemoryAgentTier,
  MemoryDecision,
  MemoryDecisionOp,
  MemoryDecisionRevertRequest,
  MemoryDecisionRevertResponse,
  MemoryDecisionsResponse,
  MemoryDecisionSource,
  MemoryDeleteResponse,
  MemoryDreamTriggerResponse,
  MemoryEditRequest,
  MemoryEditResponse,
  MemoryHeader,
  MemoryListPage,
  MemoryListQuery,
  MemoryReadResponse,
  MemoryScope,
  MemorySearchRequest,
  MemorySearchResponse,
  MemorySearchResult,
  MemorySort,
  MemoryType,
  MemoryWriteRequest,
  MemoryWriteResponse,
} from "./types";

// Adapters
export {
  deleteMemory,
  editMemory,
  KnowledgeApiError,
  listMemories,
  listMemoryDecisions,
  readMemory,
  revertMemoryDecision,
  searchMemory,
  triggerMemoryDream,
  writeMemory,
} from "./adapters/knowledge-api";

// Query infrastructure
export { knowledgeKeys } from "./lib/query-keys";
export {
  memoriesListOptions,
  memoryDecisionsOptions,
  memoryDetailOptions,
  memorySearchOptions,
} from "./lib/query-options";

// Hooks
export {
  useMemories,
  useMemory,
  useMemoryDecisions,
  useMemorySearch,
  type UseMemorySearchOptions,
} from "./hooks/use-knowledge";
export {
  useDeleteMemory,
  useEditMemory,
  useRevertMemoryDecision,
  useTriggerMemoryDream,
  useWriteMemory,
  type EditMemoryParams,
} from "./hooks/use-knowledge-actions";

// Components
export { KnowledgeListPanel } from "./components/knowledge-list-panel";
export { KnowledgeDetailPanel } from "./components/knowledge-detail-panel";
export { KnowledgeCreateDialog } from "./components/knowledge-create-dialog";
export { KnowledgeDeleteDialog } from "./components/knowledge-delete-dialog";
export { KnowledgeEditDialog } from "./components/knowledge-edit-dialog";
export { KnowledgeDecisionsSection } from "./components/knowledge-decisions-section";

// Lib
export {
  compareKnowledgeScope,
  decisionOpLabel,
  decisionSourceLabel,
  knowledgeAgentTierLabel,
  knowledgeAgentTierShortLabel,
  knowledgeMemoryKey,
  knowledgeScopeLabel,
  knowledgeScopeShortLabel,
  memoryScopeTone,
  memoryTypeTone,
} from "./lib/knowledge-formatters";
export { groupKnowledgeMemoriesByScope } from "./lib/knowledge-list";
export { DEFAULT_MEMORY_LIST_LIMIT } from "./lib/memory-list-query";
export {
  KNOWLEDGE_TYPE_TONE,
  knowledgeTypeFor,
  type KnowledgeType,
  type KnowledgeTypeTone,
} from "./lib/knowledge-type-tone";
