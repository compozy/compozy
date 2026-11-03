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
/** A catalog item with its row's session metrics; `group=agent` returns them side by side. */
export type AgentTeamItem = AgentGroupedCatalogItem & { sessions?: AgentCatalogItem["sessions"] };
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

export type AgentMemoryDocument = OperationResponse<"getAgentMemory", 200>;
export type AgentMemoryBudget = AgentMemoryDocument["budget"];
export type PutAgentMemoryParams = OperationRequestBody<"putAgentMemory">;
export type AddAgentMemoryParams = OperationRequestBody<"addAgentMemory">;
export type ReplaceAgentMemoryParams = OperationRequestBody<"replaceAgentMemory">;
export type RemoveAgentMemoryParams = OperationRequestBody<"removeAgentMemory">;
export type RemoveAgentMemoryResponse = OperationResponse<"removeAgentMemory", 200>;
export type SearchAgentMemoryParams = OperationRequestBody<"searchAgentMemory">;
export type SearchAgentMemoryResponse = OperationResponse<"searchAgentMemory", 200>;
export type ValidateAgentMemoryParams = OperationRequestBody<"validateAgentMemory">;
export type ValidateAgentMemoryResponse = OperationResponse<"validateAgentMemory", 200>;
export type AgentMemoryHistoryResponse = OperationResponse<"listAgentMemoryHistory", 200>;
export type RollbackAgentMemoryParams = OperationRequestBody<"rollbackAgentMemory">;

/** The stored or resolved identity block of `_dx.md`; open on the wire, closed here. */
export interface AgentAppearancePayload {
  version: number;
  avatar: "generated" | "icon" | "emoji";
  seed?: string;
  generation?: number;
  icon?: string;
  emoji?: string;
  color?: string;
  /** An icon or emoji on its tinted plate (`plate`, the default) or standing alone (`none`). */
  background?: "plate" | "none";
}

export type AgentInstanceState = "starting" | "active" | "paused";
export type AgentInstanceActivity = "idle" | "working" | "waiting" | "failed" | "unknown";
export type AgentInstancePauseReason =
  | ""
  | "manual"
  | "fatigue"
  | "unattended"
  | "definition_not_always_on"
  | "definition_missing";

/**
 * `_dx.md` Instances. Hand-typed until the instance routes land in the OpenAPI document; then this
 * becomes an `OperationResponse` alias and the adapter moves onto `apiClient`.
 */
export interface AgentInstancePayload {
  id: string;
  agent_name: string;
  label: string;
  /** Empty until the main session or the definition names one. */
  provider: string;
  resolved_appearance: AgentAppearancePayload;
  profile_id: string;
  /** The profile selector id routes need. Not on the wire: the read that fetched the row sets it. */
  profile_name: string;
  /** Empty for a Global instance. */
  workspace_id: string;
  /** A display name; `@user` is Global. */
  scope: string;
  state: AgentInstanceState;
  activity: AgentInstanceActivity;
  now: string;
  needs_attention: boolean;
  pause_reason: AgentInstancePauseReason;
  /** Empty until the main session exists. */
  main_session_id: string;
  checks_enabled: boolean;
  /** Empty until the first check. */
  last_wake_at: string;
  /** Empty when no check is scheduled. */
  next_wake_at: string;
  next_wake?: OperationResponse<"listAgentInstances", 200>["instances"][number]["next_wake"];
  quiet_streak: number;
  failure_streak: number;
  /** Failed checks in a row that pause it: `agents.heartbeat.fatigue_limit`. */
  fatigue_limit: number;
  /** Non-human steps since you last engaged; the brake pauses at `unattended_turn_limit`. */
  unattended_turns?: number;
  /** `agents.always_on.unattended_turn_limit`; 0 disables the brake. */
  unattended_turn_limit?: number;
  /** Non-human input held while paused, delivered on resume. */
  held_wakes?: number;
  created_at: string;
  updated_at: string;
}

export type AgentInstanceWirePayload = Omit<AgentInstancePayload, "profile_name">;

export interface AgentInstancesResponse {
  instances: AgentInstanceWirePayload[];
  next_cursor: string;
  /** `agents.always_on.max_active_instances`. */
  max_active_instances: number;
  /** Instances of this profile holding a slot. */
  active_count: number;
}

export interface CreateAgentInstanceParams {
  agent: string;
  /** A workspace id, or `@user` for Global. */
  workspace: string;
}

/** Persisted audit and dry-run projections share the generated transport contract. */
export type AgentInstanceCheckInsResponse = OperationResponse<"listAgentInstanceCheckIns", 200>;
export type AgentInstanceCheckIn = AgentInstanceCheckInsResponse["check_ins"][number];
export type AgentInstanceCheckResponse = OperationResponse<"checkAgentInstance", 200>;
export type AgentInstanceCheckDigestItem = NonNullable<
  AgentInstanceCheckResponse["digest"]
>[number];

export interface AgentInstanceOccurrence {
  state: "running" | "finished";
  result?: string;
  reason?: string;
  report_id?: string;
}

export type DraftAgentInstructionsParams = OperationRequestBody<"draftAgentInstructions">;
export type DraftAgentInstructionsResponse = OperationResponse<"draftAgentInstructions", 200>;

export type AgentActivityPayload = OperationResponse<"getAgentActivity", 200>;

export type AgentInstanceReportKind =
  | "proposal"
  | "failure"
  | "approval_expired"
  | "interrupted"
  | "rotation_failed"
  | "rotation_deferred";

export type AgentInstanceReportState = "pending" | "reserved" | "delivered" | "indeterminate";

export interface AgentInstanceReport {
  id: string;
  kind: AgentInstanceReportKind;
  state: AgentInstanceReportState;
  created_at: string;
  proposals?: { title: string; body: string }[];
  reason?: string;
  detail?: string;
  session_id?: string;
  tool?: string;
  /** What the expired request wanted to do; absent on older reports. */
  action?: string;
  /** The command or path the expired request was for. */
  resource?: string;
  expired_by?: string;
  cause?: string;
  since?: string;
}

export interface AgentInstanceReportsResponse {
  reports: AgentInstanceReport[];
  next_cursor: string;
}

/** `GET /api/agent-instances/{id}/away`, from `_dx.md`; this route ships with the away reads. */
export interface AgentInstanceAway {
  last_seen_at: string | null;
  quiet_check_ins: number;
  reports: AgentInstanceReport[];
}

/** A skill an always-on agent proposed, with its exact contents (S9). */
export type AgentSkillReviewPayload = OperationResponse<"getAgentSkillReview", 200>["review"];
export type AgentSkillReviewState = "pending" | "approved" | "rejected";
