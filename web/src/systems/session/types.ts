import type { UIMessage as AIUIMessage } from "ai";

import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type SessionsResponse = OperationResponse<"listSessions", 200>;
export type SessionCatalogEventPayload = OperationResponse<"streamSessionCatalog", 200>;
export type SessionsQuery = OperationQuery<"listSessions">;
export type SessionListFilters = Omit<SessionsQuery, "cursor">;
export type SessionPayload = SessionsResponse["sessions"][number];
export type SessionResponse = OperationResponse<"getSession", 200>;
export type SessionByIDResponse = OperationResponse<"getSessionByID", 200>;
export type ACPCaps = NonNullable<SessionPayload["acp_caps"]>;
export type SessionState = SessionPayload["state"];
export type SessionFailurePayload = NonNullable<SessionPayload["failure"]>;
export type SessionLineagePayload = NonNullable<SessionPayload["lineage"]>;
export type AgentMePayload = OperationResponse<"getAgentMe", 200>["me"];
export type AgentContextPayload = OperationResponse<"getAgentContext", 200>["context"];
export type AgentSpawnPayload = OperationResponse<"spawnAgentSession", 201>["spawn"];
export type CoordinatorConfigPayload = OperationResponse<
  "getAgentCoordinatorConfig",
  200
>["coordinator"];

export type SessionEventsResponse = OperationResponse<"listSessionEvents", 200>;
export type SessionEventPayload = SessionEventsResponse["events"][number];
export type FetchSessionEventsParams = OperationQuery<"listSessionEvents">;

export type SessionHistoryResponse = OperationResponse<"getSessionHistory", 200>;
export type TurnHistoryPayload = SessionHistoryResponse["history"][number];

export type SessionTranscriptResponse = OperationResponse<"getSessionTranscript", 200>;
export type SessionTranscriptEntry = SessionTranscriptResponse["entries"][number];
export type SessionTranscriptQuery = OperationQuery<"getSessionTranscript">;
export type SessionStreamResponse = OperationResponse<"streamSession", 200>;
export type TranscriptSnapshotPayload = NonNullable<SessionStreamResponse["transcript_snapshot"]>;
export type TranscriptDeltaPayload = NonNullable<SessionStreamResponse["transcript_delta"]>;
export type SessionBadge = SessionPayload["badge"];
export type SessionAttachResponse = OperationResponse<"attachSession", 200>;
export type SessionRecapResponse = OperationResponse<"getSessionRecap", 200>;
export type SessionRecapPayload = SessionRecapResponse["recap"];
export type SessionUsageResponse = OperationResponse<"getSessionUsage", 200>;
export type SessionUsagePayload = SessionUsageResponse["usage"];
export type TranscriptMarkerPayload = SessionRecapPayload["recent_markers"][number];
export type SessionRepairResponse = OperationResponse<"repairSession", 200>;
export type SessionRepairPayload = SessionRepairResponse["repair"];
export type SessionRepairQuery = OperationQuery<"repairSession">;
export type SessionPromptRequest = OperationRequestBody<"sendSessionPrompt">;
export type SessionPromptResponse =
  | OperationResponse<"sendSessionPrompt", 200>
  | OperationResponse<"sendSessionPrompt", 202>;
type SessionPromptEnvelope = Extract<SessionPromptResponse, { prompt: unknown }>;
export type SessionPromptPayload = SessionPromptEnvelope["prompt"];
export type SessionGoalCommandResult = Extract<SessionPromptResponse, { outcome: unknown }>;
export type SessionPromptResult = SessionPromptPayload | SessionGoalCommandResult;
export type SessionGoalResponse = OperationResponse<"getSessionGoal", 200>;
export type SessionGoalSnapshot = NonNullable<SessionGoalResponse["goal"]>;
export type SessionGoalContext = SessionGoalSnapshot["context"];
export type SessionGoalStatus = SessionGoalSnapshot["status"];
export type GoalPromptMeta = NonNullable<SessionEventPayload["goal"]>;

export type SessionLedgerResponse = OperationResponse<"getMemorySessionLedger", 200>;
export type SessionLedgerMeta = SessionLedgerResponse["meta"];
export type SessionLedgerEvent = SessionLedgerResponse["events"][number];

export type CreateSessionParams = OperationRequestBody<"createSession">;
export type SessionApprovalResponse = OperationResponse<"approveSession", 200>;
export type ApproveSessionParams = OperationRequestBody<"approveSession">;
export type PermissionDecision = ApproveSessionParams["decision"];

export type ClarificationsResponse = OperationResponse<"listSessionClarifications", 200>;
/** Live pending clarification projection — the exact authority for pending truth. */
export type ClarificationPending = ClarificationsResponse["clarifications"][number];
export type AnswerClarificationBody = OperationRequestBody<"answerSessionClarification">;
export type AnswerClarificationResult = OperationResponse<"answerSessionClarification", 200>;
export type ToolArtifactPage = OperationResponse<"readToolArtifact", 200>;
export type ToolArtifactRef = ToolArtifactPage["artifact"];

export type ClarifyStatus = "pending" | "resolved" | "timed_out" | "canceled";

export interface ClarifyEventAnswer {
  choice: number | null;
  text: string;
  fallback: boolean;
}

export interface ClarifyEventRequest {
  request_id: string;
  workspace_id?: string;
  session_id?: string;
  agent_name?: string;
  question: string;
  choices?: string[];
  asked_at?: string;
  deadline?: string;
}

/**
 * Parsed view of a durable `clarify` transcript event. The pending card renders from the live GET,
 * never from this payload; terminal receipts are truthful historical evidence read from it.
 */
export interface ClarifyEventView {
  status: ClarifyStatus;
  requestId: string;
  request: ClarifyEventRequest;
  answer: ClarifyEventAnswer | null;
  at?: string;
}

export interface ToolUseResult {
  stdout?: string;
  stderr?: string;
  filePath?: string;
  content?: string;
  structuredPatch?: unknown[];
  error?: string;
  preview?: string;
  truncated?: boolean;
  artifacts?: ToolArtifactRef[];
  rawOutput?: unknown;
}

export interface TokenUsagePayload {
  turn_id?: string;
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  thought_tokens?: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
  context_used?: number;
  context_size?: number;
  cost_amount?: number;
  cost_currency?: string;
  timestamp?: string;
}

export interface RuntimeActivityPayload {
  turn_id?: string;
  turn_source?: string;
  turn_started_at?: string | null;
  deadline_at?: string | null;
  last_activity_at?: string | null;
  last_activity_kind?: string;
  last_activity_detail?: string;
  current_tool?: string;
  tool_call_id?: string;
  last_progress_at?: string | null;
  iteration_current?: number;
  iteration_max?: number;
  idle_seconds?: number;
  elapsed_ms: number;
  elapsed_seconds?: number;
}

export interface AgentEventPayload {
  type: string;
  session_id?: string;
  turn_id?: string;
  client_message_id?: string;
  request_id?: string;
  timestamp?: string;
  text?: string;
  title?: string;
  tool_call_id?: string;
  stop_reason?: string;
  action?: string;
  resource?: string;
  decision?: string;
  error?: string;
  failure?: SessionFailurePayload;
  usage?: TokenUsagePayload;
  runtime?: RuntimeActivityPayload;
  marker?: TranscriptMarkerPayload;
  goal?: GoalPromptMeta;
  raw?: unknown;
}

export interface AghPermissionData extends AgentEventPayload {
  request_id: string;
  raw?: Record<string, unknown>;
}

export interface SessionDataParts extends Record<string, unknown> {
  "agh-event": AgentEventPayload;
  "agh-permission": AghPermissionData;
}

export type SessionMessage = AIUIMessage<unknown, SessionDataParts>;
export type TranscriptMessage = SessionMessage;
export type TranscriptMessageRole = TranscriptMessage["role"];

export type NormalizedSessionTranscriptEntry = Omit<SessionTranscriptEntry, "message"> & {
  message: SessionMessage;
};

export type NormalizedSessionTranscriptResponse = Omit<SessionTranscriptResponse, "entries"> & {
  entries: NormalizedSessionTranscriptEntry[];
};

/** Frontend-owned stream cursor added to the bounded REST transcript page. */
export type SessionTranscriptPage = NormalizedSessionTranscriptResponse & {
  cursor: number;
};

export type UIMessageRole = "user" | "assistant" | "tool_call" | "tool_result" | "system" | "diff";

export interface UIMessageDiff {
  language?: string;
  content: string;
  path?: string;
  additions?: number;
  removals?: number;
}

export interface UIMessage {
  id: string;
  role: UIMessageRole;
  content: string;
  toolName?: string;
  toolInput?: Record<string, unknown>;
  toolResult?: ToolUseResult;
  toolError?: boolean;
  thinking?: string;
  thinkingComplete?: boolean;
  isStreaming?: boolean;
  diff?: UIMessageDiff;
  timestamp: number;
}

export interface PermissionRequest {
  requestId: string;
  toolName: string;
  toolInput: Record<string, unknown>;
  action: string;
  resource: string;
  supportedDecisions?: PermissionDecision[];
  turnId?: string;
  toolCallId?: string;
}
