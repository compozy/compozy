import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type AgentsResponse = OperationResponse<"listAgents", 200>;
export type AgentPayload = AgentsResponse["agents"][number];
export type AgentResponse = OperationResponse<"getAgent", 200>;
export type CreateAgentParams = OperationRequestBody<"createAgent">;
export type UpdateAgentParams = OperationRequestBody<"updateAgent">;
export type DuplicateAgentParams = OperationRequestBody<"duplicateAgent">;
export type DeleteAgentResponse = OperationResponse<"deleteAgent", 200>;
export type AgentMCPServer = NonNullable<AgentPayload["mcp_servers"]>[number];
export type AgentCatalogResponse = OperationResponse<"listAgentCatalog", 200>;
export type AgentCatalogItem = AgentCatalogResponse["agents"][number];
export type AgentCatalogFilter = OperationQuery<"listAgentCatalog">;
export type AgentCatalogStableFilter = Omit<AgentCatalogFilter, "cursor" | "workspace">;

export type AgentSoulPayload = OperationResponse<"getAgentDefinitionSoul", 200>;
export type PutAgentSoulParams = OperationRequestBody<"putAgentSoul">;
export type DeleteAgentSoulParams = OperationRequestBody<"deleteAgentSoul">;
export type ValidateAgentSoulParams = OperationRequestBody<"validateAgentDefinitionSoul">;
export type ValidateAgentSoulResponse = OperationResponse<"validateAgentDefinitionSoul", 200>;
export type AgentSoulHistoryResponse = OperationResponse<"listAgentSoulHistory", 200>;
export type RollbackAgentSoulParams = OperationRequestBody<"rollbackAgentSoul">;
export type PutAgentSoulResponse = OperationResponse<"putAgentSoul", 200>;
export type RollbackAgentSoulResponse = OperationResponse<"rollbackAgentSoul", 200>;
export type DeleteAgentSoulResponse = OperationResponse<"deleteAgentSoul", 200>;

export type AgentHeartbeatPayload = OperationResponse<"getAgentHeartbeat", 200>;
export type PutAgentHeartbeatParams = OperationRequestBody<"putAgentHeartbeat">;
export type DeleteAgentHeartbeatParams = OperationRequestBody<"deleteAgentHeartbeat">;
export type ValidateAgentHeartbeatParams = OperationRequestBody<"validateAgentHeartbeat">;
export type ValidateAgentHeartbeatResponse = OperationResponse<"validateAgentHeartbeat", 200>;
export type AgentHeartbeatHistoryResponse = OperationResponse<"listAgentHeartbeatHistory", 200>;
export type RollbackAgentHeartbeatParams = OperationRequestBody<"rollbackAgentHeartbeat">;
export type PutAgentHeartbeatResponse = OperationResponse<"putAgentHeartbeat", 200>;
export type RollbackAgentHeartbeatResponse = OperationResponse<"rollbackAgentHeartbeat", 200>;
export type DeleteAgentHeartbeatResponse = OperationResponse<"deleteAgentHeartbeat", 200>;
export type AgentHeartbeatStatusPayload = OperationResponse<"getAgentHeartbeatStatus", 200>;
export type WakeAgentHeartbeatParams = OperationRequestBody<"wakeAgentHeartbeat">;
export type WakeAgentHeartbeatResponse = OperationResponse<"wakeAgentHeartbeat", 200>;
