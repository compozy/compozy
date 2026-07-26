// Types
export type {
  ACPCaps,
  AgentEventPayload,
  AghPermissionData,
  AnswerClarificationBody,
  AnswerClarificationResult,
  ApproveSessionParams,
  ClarificationPending,
  ClarifyEventView,
  ClarifyStatus,
  CreateSessionParams,
  FetchSessionEventsParams,
  PermissionDecision,
  PermissionRequest,
  NormalizedSessionTranscriptResponse,
  SessionEventPayload,
  SessionFailurePayload,
  SessionApprovalResponse,
  SessionEventsResponse,
  SessionHistoryResponse,
  SessionLedgerEvent,
  SessionLedgerMeta,
  SessionLedgerResponse,
  SessionMessage,
  SessionByIDResponse,
  SessionPayload,
  SessionPromptPayload,
  SessionPromptRequest,
  SessionPromptResponse,
  SessionPromptResult,
  SessionGoalCommandResult,
  SessionGoalContext,
  SessionGoalResponse,
  SessionGoalSnapshot,
  SessionGoalStatus,
  GoalPromptMeta,
  SessionAttachResponse,
  SessionBadge,
  SessionCatalogEventPayload,
  SessionRecapPayload,
  SessionRecapResponse,
  SessionUsagePayload,
  SessionUsageResponse,
  SessionRepairPayload,
  SessionRepairQuery,
  SessionRepairResponse,
  SessionResponse,
  SessionState,
  SessionListFilters,
  SessionTranscriptPage,
  SessionsResponse,
  SessionsQuery,
  SessionTranscriptResponse,
  TranscriptMarkerPayload,
  SessionDataParts,
  TokenUsagePayload,
  ToolUseResult,
  TranscriptMessage,
  TranscriptMessageRole,
  TurnHistoryPayload,
  UIMessage,
} from "./types";

// Adapters
export {
  answerSessionClarification,
  approveSession,
  cancelQueuedSessionPrompt,
  cancelSessionPrompt,
  ClarificationNotAnswerableError,
  createSession,
  fetchSessionClarifications,
  deleteSession,
  fetchSession,
  fetchSessionById,
  fetchSessionEvents,
  fetchSessionHistory,
  fetchSessionGoal,
  fetchSessionLedger,
  fetchSessionRecap,
  fetchSessionUsage,
  fetchSessionTranscript,
  fetchSessions,
  interruptSessionPrompt,
  repairSession,
  resumeSession,
  sendSessionPrompt,
  SessionApiError,
  SessionLedgerUnavailableError,
  SessionNotFoundError,
  steerSessionPrompt,
  stopSession,
} from "./adapters/session-api";

// Query infrastructure
export { formatMessageTimestamp, formatMessageTimestampFull } from "./lib/format-timestamp";
export { isClarifyEventData } from "./lib/clarify-event";
export { isAgentEventPayload, resolveToolResult } from "./lib/message-parts";
export { getSessionDisplayTitle, UNTITLED_SESSION_TITLE } from "./lib/session-display-title";
export { sessionKeys } from "./lib/query-keys";
export {
  sessionByIdOptions,
  sessionClarificationsOptions,
  sessionDetailOptions,
  sessionEventsOptions,
  sessionHistoryOptions,
  sessionGoalOptions,
  sessionLedgerOptions,
  sessionRecapOptions,
  sessionUsageOptions,
  sessionTranscriptOptions,
  sessionsListOptions,
} from "./lib/query-options";
export {
  hasRunningSession,
  idleAttachableAgentNames,
  isSessionRunning,
  isUserControllableSession,
  runningAgentNames,
} from "./lib/session-running";

// Stores
export { useSessionStore } from "./hooks/use-session-store";
export type {
  ComposerDraft,
  SessionState as SessionStoreState,
  SessionActions,
  SessionStore,
} from "./stores/session-store";

// Hooks
export {
  useSession,
  useSessionById,
  useSessionLedger,
  useSessionGoal,
  useSessionRecap,
  useSessionUsage,
  useSessions,
} from "./hooks/use-sessions";
export {
  useAnswerSessionClarification,
  useSessionClarifications,
  type AnswerClarificationVariables,
} from "./hooks/use-session-clarifications";
export { useSessionRuntimeRenderContext } from "./hooks/use-session-runtime-render-context";
export {
  useWorkspaceSessionActivity,
  workspaceSessionActivityFromResults,
  type WorkspaceSessionActivity,
  type WorkspaceSessionActivityMap,
  type WorkspaceSessionReturnTarget,
} from "./hooks/use-workspace-session-activity";
export {
  sessionCatalogStreamURL,
  useSessionCatalogStreams,
  type SessionCatalogEventSource,
  type SessionCatalogEventSourceFactory,
  type SessionCatalogStreamStatus,
} from "./hooks/use-session-catalog-streams";
export {
  useSessionTranscriptThreadMessages,
  useSessionTranscriptThreadState,
} from "./hooks/use-session-transcript-thread-messages";
export { useSessionTopbarSlot } from "./hooks/use-session-topbar-slot";
export {
  useSessionInspectorState,
  type UseSessionInspectorStateResult,
} from "./hooks/use-session-inspector-state";
export type {
  SessionTranscriptThreadState,
  SessionTranscriptThreadStatus,
} from "./lib/session-transcript-thread-context-value";
export {
  useClearSessionConversation,
  useCancelQueuedSessionPrompt,
  useCreateSession,
  useDeleteSession,
  useInterruptSessionPrompt,
  useQueueSessionPrompt,
  useRepairSession,
  useResumeSession,
  useSendSessionPrompt,
  useSteerSessionPrompt,
  useStopSession,
  type CancelQueuedSessionPromptParams,
  type RepairSessionParams,
  type SendSessionPromptParams,
  type SessionPromptActionParams,
} from "./hooks/use-session-actions";
export {
  useSessionCreateDialog,
  type SessionCreateDialogApi,
  type SessionCreateDialogState,
} from "./hooks/use-session-create-dialog";
export {
  SessionCreateProvider,
  type SessionCreateContextValue,
} from "./contexts/session-create-context";
export { useSessionCreate } from "./hooks/use-session-create";

// Components
export {
  SessionCreateDialog,
  type SessionCreateDialogProps,
} from "./components/session-create-dialog";
export {
  SessionResumeFailure,
  type SessionResumeFailureProps,
} from "./components/session-resume-failure";
export { SessionStatusLine, type SessionStatusLineProps } from "./components/session-status-line";
export { SessionToolCallRow, type SessionToolCallRowProps } from "./components/tool-call-card";
export {
  SessionChatRuntimeProvider,
  type SessionChatRuntimeProviderProps,
} from "./components/session-chat-runtime-provider";
export { ThinkingBlock, type ThinkingBlockProps } from "./components/thinking-block";
export {
  PermissionDataPart,
  PermissionPrompt,
  type PermissionPromptProps,
} from "./components/permission-prompt";
export { ClarificationCard, type ClarificationCardProps } from "./components/clarification-card";
export { ClarificationReceipt } from "./components/clarification-receipt";
export {
  ClarificationDataPart,
  type ClarificationDataPartProps,
} from "./components/clarification-data-part";
export { RuntimeActivityNotice } from "./components/runtime-activity-notice";
export {
  SessionInspector,
  type InspectorMemoryState,
  type InspectorSessionLedger,
  type InspectorUsage,
  type SessionInspectorProps,
} from "./components/session-inspector";
export {
  deriveFileReads,
  deriveTraceEvents,
  type InspectorFileEntry,
  type InspectorTraceEvent,
  type InspectorTraceKind,
  type InspectorTraceStatus,
} from "./components/session-inspector.logic";
