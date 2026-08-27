// Runtime consumers import from `@/systems/agent-comms`; Storybook and tests may
// use the `/mocks` entry point to stage contract-faithful data.

// Types
export type {
  CallDelivery,
  CallErrorPayload,
  CallErrorRosterEntry,
  CallMessagePayload,
  CallMessagesListQuery,
  CallMessagesListResponse,
  CallPayload,
  CallResultResponse,
  CallState,
  CallVerdict,
  CallsListQuery,
  CallsListResponse,
  ChildState,
  CreateCallRequest,
  CreateCallResponse,
  SendCallMessageRequest,
  SendCallMessageResponse,
  StopSessionDrainResponse,
} from "./types";
export { CALL_DELIVERIES, CALL_STATES, CALL_VERDICTS, CHILD_STATES } from "./types";

// Adapters
export {
  AGENT_COMMS_ERROR_CODES,
  AgentCommsApiError,
  agentCommsErrorCode,
  isAgentCommsApiError,
  isAgentCommsErrorCode,
  type AgentCommsErrorCode,
  type CallCount,
  type CallMessagesFilter,
  type CallsListFilter,
} from "./adapters/agent-comms-api";

// Query infrastructure
export { agentCommsKeys } from "./lib/query-keys";
export {
  CALLS_PANEL_PAGE_SIZE,
  CALLS_TREE_PAGE_SIZE,
  LIVE_CALL_POLL_INTERVAL,
  callCountOptions,
  callDetailOptions,
  callMessagesOptions,
  callPromptOptions,
  callResultOptions,
  callSupersededOptions,
  callsListOptions,
} from "./lib/query-options";
export { isScopeReady, type AgentCommsScope } from "./lib/agent-comms-scope";

// Domain rules
export {
  CALL_DELIVERY_SIGNAL,
  CALL_STATE_SIGNAL,
  CALL_VERDICT_SIGNAL,
  CHILD_STATE_SIGNAL,
  callStateSignal,
  isFinishedCallState,
  isNeedsYouCallState,
  isTerminalCallState,
  toCallDelivery,
  toCallState,
  toCallVerdict,
  toChildState,
  type CallAttentionClass,
  type CallStateSignal,
} from "./lib/call-state";
export {
  buildCallTree,
  childStatesForRoot,
  escalateCallPayloads,
  escalateCallStates,
  type CallCommsTree,
  type CallTreeGroup,
  type CallTreeRow,
  type ChildStateCatalogRow,
} from "./lib/agent-comms-tree";
export { countsForTreeGroups, type CallTreeGroupCounts } from "./lib/agent-comms-tree-counts";
export { callCreateFailureCopy, callMessageFailureCopy } from "./lib/call-failure-copy";
export {
  buildCallTreeDataSource,
  callNodeId,
  groupNodeId,
  type CallTreeDataSource,
  type CallTreeNode,
} from "./lib/agent-comms-tree-nodes";
export {
  buildCallDetailView,
  type CallDetailControls,
  type CallDetailView,
  type CallIdleTtl,
  type CallResultView,
} from "./lib/call-detail-view-model";
export { buildCallTimeline, type CallTimelineEvent } from "./lib/call-detail-timeline";
export {
  buildCallResultShape,
  type CallResultRow,
  type CallResultShape,
} from "./lib/call-result-rows";
export { parseExpectDraft, type ExpectDraftResult } from "./lib/expect-draft";
export {
  resolveCallSurfaceState,
  type CallEmptyReason,
  type CallSurfaceState,
} from "./lib/agent-comms-empty-state";
export {
  deriveCallAttention,
  type CallAttentionCause,
  type CallAttentionModel,
  type CallAttentionRow,
} from "./lib/agent-comms-attention";
export {
  readSyntheticTurn,
  type SyntheticTurn,
  type SyntheticTurnKind,
} from "./lib/synthetic-turn";
export {
  AGENT_CALL_TOOL_NAME,
  callIdsFromToolResult,
  type AgentCallToolInvocation,
} from "./lib/agent-call-tool-parts";

// Hooks
export { useAgentCommsScope } from "./hooks/use-agent-comms-scope";
export {
  useAgentCommsActivity,
  type AgentCommsActivityModel,
} from "./hooks/use-agent-comms-activity";
export { useCallDetail, type CallDetailModel } from "./hooks/use-call-detail";
export { useCallMutations } from "./hooks/use-call-mutations";
export { useCallCount, useCallCounts, type CallCountFilter } from "./hooks/use-call-counts";
export { useAgentCallCompose, type AgentCallComposeModel } from "./hooks/use-agent-call-compose";
export { useCallsById, type CallsByIdModel } from "./hooks/use-calls-by-id";

// Components
export {
  AgentCallLiveness,
  AgentCallStatePill,
  AgentCallVerdictChip,
  AgentChildStatePill,
  AgentMessageDeliveryPill,
} from "./components/agent-call-state-pill";
export { AgentUntrustedFrame } from "./components/agent-untrusted-frame";
export { AgentCallCost } from "./components/agent-call-cost";
export { AgentCallTree } from "./components/agent-call-tree";
export { CALL_TREE_VIRTUALIZATION_THRESHOLD } from "./lib/agent-call-tree-constants";
export { AgentCallTreeRow } from "./components/agent-call-tree-row";
export { AgentCallTreeRootRow } from "./components/agent-call-tree-root-row";
export { AgentCallDetail } from "./components/agent-call-detail";
export { AgentCallDetailHeader } from "./components/agent-call-detail-header";
export { AgentCallDetailTimeline } from "./components/agent-call-detail-timeline";
export { AgentCallAttempts } from "./components/agent-call-attempts";
export { AgentCallResultView } from "./components/agent-call-result-view";
export { AgentComposeMessage } from "./components/agent-compose-message";
export { AgentCallCompose, type AgentCallTarget } from "./components/agent-call-compose";
export { AgentCallTurnCard } from "./components/agent-call-turn-card";
export { AgentCallTurnFanout } from "./components/agent-call-turn-fanout";
export { AgentSyntheticTurn } from "./components/agent-synthetic-turn";
export { AgentCallInvocationCard } from "./components/agent-call-invocation-card";
export {
  AgentCallsInspectorPanel,
  type CallDirectionSection,
} from "./components/agent-calls-inspector-panel";
