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
export type AgentGroupedCatalogItem = NonNullable<AgentCatalogResponse["items"]>[number];
export type AgentCatalogFilter = OperationQuery<"listAgentCatalog">;
export type AgentCatalogStableFilter = Omit<AgentCatalogFilter, "cursor" | "workspace">;
export type AgentAggregateFilter = OperationQuery<"listAgentsAggregate">;

/** The authored-file view that owns a mutation, captured when the user acts. */
export interface AgentAuthoredMutationVariables<TParams> {
  name: string;
  cacheWorkspace: string | null;
  profile: string;
  params: TParams;
}

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

/** The stored or resolved identity block of `_dx.md`; open on the wire, closed here. */
export interface AgentAppearancePayload {
  version: number;
  avatar: "generated" | "icon" | "emoji";
  seed?: string;
  generation?: number;
  icon?: string;
  emoji?: string;
  color?: string;
}

export type AgentInstanceState = "starting" | "active" | "paused";
export type AgentInstanceActivity = "idle" | "working" | "waiting" | "failed" | "unknown";
export type AgentInstancePauseReason =
  | ""
  | "manual"
  | "fatigue"
  | "definition_not_stays_on"
  | "definition_missing";

/**
 * `_dx.md` Instances. Hand-typed until the instance routes land in the OpenAPI document; then this
 * becomes an `OperationResponse` alias and the adapter moves onto `apiClient`.
 */
export interface AgentInstancePayload {
  id: string;
  agent_name: string;
  label: string;
  provider?: string;
  resolved_appearance: AgentAppearancePayload;
  /** Empty for a Global instance. */
  workspace_id: string;
  /** A display name; `@user` is Global. */
  scope: string;
  state: AgentInstanceState;
  activity: AgentInstanceActivity;
  now?: string;
  needs_attention: boolean;
  pause_reason?: AgentInstancePauseReason;
  main_session_id?: string | null;
  checks_enabled?: boolean;
  last_wake_at?: string | null;
  next_wake_at?: string | null;
  quiet_streak?: number;
  failure_streak?: number;
}

export interface AgentInstancesResponse {
  instances: AgentInstancePayload[];
  next_cursor: string;
}

export interface CreateAgentInstanceParams {
  agent: string;
  /** A workspace id, or `@user` for Global. */
  workspace: string;
}

export interface AgentInstanceCheckIn {
  result: string;
  reason?: string;
  at?: string;
  created_at?: string;
}

export interface AgentInstanceCheckInsResponse {
  check_ins: AgentInstanceCheckIn[];
  next_cursor: string;
}

export interface DraftAgentInstructionsParams {
  goal: string;
  runtime: { provider: string; model?: string };
}

export interface DraftAgentInstructionsResponse {
  instructions: string;
}
