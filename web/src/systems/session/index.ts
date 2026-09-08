export type {
  ACPCaps,
  AgentEventPayload,
  CompozyPermissionData,
  AnswerClarificationBody,
  AnswerClarificationResult,
  ApproveSessionParams,
  ClarificationPending,
  SessionInteractionRecord,
  SessionInteractionStatus,
  SessionInteractionsResponse,
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
  SessionInputPayload,
  SessionInputsResponse,
  SessionLedgerEvent,
  SessionLedgerMeta,
  SessionLedgerResponse,
  SessionMessage,
  SessionByIDResponse,
  SessionOwnerResponse,
  SessionPayload,
  SessionSupervisionPayload,
  SessionWorkSignalPayload,
  SessionWorkSignalKind,
  SessionQuietWarningPayload,
  SessionAttachment,
  SessionBusyInputDraft,
  SessionBusyInputHandler,
  SessionPromptAttachment,
  SessionPromptPayload,
  SessionPromptRequest,
  SessionPromptResponse,
  SessionStopResult,
  SessionPromptDirectTurn,
  SessionPromptResult,
  SessionPromptSendResult,
  PromoteSessionInputRequest,
  ReplaceSessionInputRequest,
  SessionGoalCommandResult,
  SessionGoalContext,
  SessionGoalResponse,
  SessionGoalSnapshot,
  SessionGoalStatus,
  GoalPromptMeta,
  SessionAttachResponse,
  SessionBadge,
  SessionPendingInteraction,
  SessionAttentionEventPayload,
  OperatorNotificationEventPayload,
  SessionCatalogEventPayload,
  SessionCommandPayload,
  SessionCommandsResponse,
  SessionRecapPayload,
  SessionRecapResponse,
  SessionUsagePayload,
  SessionUsageResponse,
  SessionRepairPayload,
  SessionRepairQuery,
  SessionRepairResponse,
  RenameSessionRequest,
  SessionRuntimeSelection,
  SessionResponse,
  SessionState,
  SessionListFilters,
  SessionTranscriptPage,
  SessionsResponse,
  SessionsQuery,
  SetSessionRuntimeRequest,
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
export type { SessionPromptRuntimeSnapshot } from "./contexts/session-prompt-runtime-context-value";
export { isEditableTarget } from "./lib/editable-target";

export {
  SessionPromptRuntimeProvider,
  type SessionPromptRuntimeProviderProps,
} from "./contexts/session-prompt-runtime-context";
export {
  SessionPromptRuntimeSelector,
  type SessionPromptRuntimeSelectorProps,
} from "./components/session-prompt-runtime-selector";
export {
  answerSessionClarification,
  archiveSession,
  approveSession,
  cancelQueuedSessionPrompt,
  cancelSessionPrompt,
  clearSessionInputs,
  clearSessionRuntime,
  ClarificationNotAnswerableError,
  createSession,
  fetchSessionClarifications,
  fetchSessionInteractions,
  deleteSession,
  fetchSession,
  fetchSessionCommands,
  fetchSessionEvents,
  fetchSessionHistory,
  fetchSessionInputs,
  fetchSessionGoal,
  mutateSessionGoal,
  fetchSessionLedger,
  fetchSessionRecap,
  fetchSessionUsage,
  fetchSessionTranscript,
  fetchSessions,
  promoteSessionInputToSteer,
  repairSession,
  renameSession,
  replaceSessionInput,
  rewindSession,
  resumeSession,
  sendSessionPrompt,
  setSessionRuntime,
  SessionApiError,
  SessionLedgerUnavailableError,
  SessionNotFoundError,
  stopSession,
  unarchiveSession,
} from "./adapters/session-api";
export type { SessionRewindRequest, SessionRewindResult } from "./adapters/session-api";

export { sessionPromptCapability } from "./lib/session-prompt-capability";
export type { SessionPromptCapability } from "./lib/session-prompt-capability";
// Attachment surface — byte URLs, prompt parts, item shaping, cards; ./attachments is the public list.
export * from "./attachments";
export { formatMessageTimestamp, formatMessageTimestampFull } from "./lib/format-timestamp";
export { derivePendingClarifyRequestIds, isClarifyEventData } from "./lib/clarify-event";
export { isAgentEventPayload, resolveToolResult } from "./lib/message-parts";
export { isProviderErrorEvent } from "./lib/provider-error";
export { getSessionDisplayTitle, UNTITLED_SESSION_TITLE } from "./lib/session-display-title";
// Attention surface — badge dictionary, pending-interaction reads, list
// preferences, presence lease. Grouped in ./attention; re-exported here.
export {
  acquireSessionPresence,
  DEFAULT_SESSION_LIST_PREFERENCES,
  isFinishedBadge,
  isNeedsYouBadge,
  maskedAttentionNote,
  pendingClarifyCount,
  pendingInteractionReason,
  pendingInteractions,
  pendingPermissionCount,
  releaseSessionPresence,
  renewSessionPresence,
  SESSION_BADGE_SIGNAL,
  SESSION_BADGES,
  SESSION_LIST_SCOPES,
  SESSION_LIST_SORTS,
  SessionBadgeGlyph,
  SessionBadgeMark,
  SessionArchivedToggle,
  SessionScopeToggle,
  sessionAttentionClass,
  sessionBadgeSignal,
  sessionBadgeWordClass,
  sessionListSortParam,
  toSessionBadge,
  toSessionListScope,
  toSessionListSort,
  useSessionListPreferences,
  useSessionListView,
  useSessionPresence,
  useWorkspaceSessionGroups,
  type SessionAttentionClass,
  type SessionBadgeGlyphProps,
  type SessionBadgeMarkProps,
  type SessionBadgeShape,
  type SessionBadgeSignal,
  type SessionBadgeToken,
  type SessionListPreferences,
  type SessionListPreferencesModel,
  type SessionListScope,
  type SessionListSort,
  type SessionListViewModel,
  type WorkspaceSessionGroup,
} from "./attention";
export {
  isQueuedPromptMutable,
  queueCapFromInputs,
  queuedPromptAttachmentSummary,
  queuedPromptOwner,
  queuedPromptsFromInputs,
  withCachedInputs,
} from "./lib/queued-prompt";
export type {
  QueuedPrompt,
  QueuedPromptAttachmentPreview,
  QueuedPromptAttachmentSummary,
  QueuedPromptEditOutcome,
  QueuedPromptOwner,
  QueuedPromptStatus,
} from "./lib/queued-prompt";
export { queuedPromptPreview, type QueuedPromptPreview } from "./lib/queued-prompt-preview";
export {
  beginUnconfirmedRetry,
  findUnconfirmedSend,
  isSendAcknowledgmentLost,
  markSendUnconfirmed,
  resolveUnconfirmedSend,
  type SessionSendEnvelope,
  type SessionSendIdentity,
  type UnconfirmedSend,
} from "./lib/session-unconfirmed-send";
export { SessionQueueStrip, type SessionQueueStripProps } from "./components/session-queue-strip";
export {
  isSessionTransportDisconnected,
  SESSION_TRANSPORT_GRACE_MS,
  SESSION_TRANSPORT_LIVE,
  sessionTransportChip,
  transportGraceElapsed,
  type SessionTransportChipModel,
  type SessionTransportFailure,
  type SessionTransportHistoryReset,
  type SessionTransportPhase,
  type SessionTransportSnapshot,
} from "./lib/session-transport";
export type { SessionTransportState } from "./lib/session-transcript-thread-context-value";
export { SessionTransportChip } from "./components/session-transport-chip";
export {
  SessionTransportFailureNotice,
  SessionTransportHistoryResetNotice,
} from "./components/session-transport-notices";
export { sessionKeys } from "./lib/query-keys";
export {
  cachedForeignSessionOwner,
  resolveForeignSessionOwner,
  resolveSessionOwner,
  sessionOwnerDialogState,
} from "./lib/session-owner-resolution";
export {
  sessionOwnerKeys,
  sessionOwnerOptions,
  type SessionOwnerDialogState,
} from "./lib/query-options";
export { fetchSessionOwner } from "./adapters/session-owner-api";
export {
  SESSION_WORKSPACE_SWITCH_STATES,
  validateSessionDeepLinkSearch,
  type SessionDeepLinkSearch,
  type SessionWorkspaceSwitchState,
} from "./lib/session-deeplink-search";
export {
  sessionClarificationsOptions,
  sessionCommandsOptions,
  sessionExpiredInteractionsOptions,
  sessionResolvedInteractionsOptions,
  sessionInputsOptions,
  sessionAttentionSummaryOptions,
  sessionDetailOptions,
  sessionEventsOptions,
  sessionHistoryOptions,
  sessionGoalOptions,
  sessionLedgerOptions,
  sessionRecapOptions,
  sessionUsageOptions,
  sessionTranscriptOptions,
  sessionAcrossProfilesOptions,
  sessionScopedDetailOptions,
  sessionsListOptions,
  sessionsCompleteListOptions,
} from "./lib/query-options";
export {
  canPromptSession,
  hasUnrecoverableRuntime,
  hasRunningSession,
  idleAttachableAgentNames,
  isSessionRunning,
  isUserControllableSession,
  runningAgentNames,
} from "./lib/session-running";
export { invalidateSessionMutationQueries } from "./lib/session-query-invalidation";
// Busy-input surface — default mode, steer delivery, refusals, send outcome.
// Grouped in ./busy-input; re-exported here.
export * from "./busy-input";

export type {
  SessionGoalFeedback,
  SessionStoreContext,
  SessionStore,
} from "./stores/session-store";
export { sessionStore } from "./stores/session-store";

export type {
  SessionTranscriptThreadState,
  SessionTranscriptThreadStatus,
} from "./lib/session-transcript-thread-context-value";
export { SessionCreateProvider } from "./contexts/session-create-context";
export { createSessionCreateStore, type SessionCreateStore } from "./stores/session-create-store";

export {
  SessionCreateDialog,
  type SessionCreateDialogProps,
} from "./components/session-create-dialog";
export { SessionCreateDialogHost } from "./components/session-create-dialog-host";
export { SessionLoadOlderButton } from "./components/session-load-older-button";
export { SessionRewindMessageAction } from "./components/session-rewind-message-action";
export {
  SessionResumeFailure,
  type SessionResumeFailureProps,
} from "./components/session-resume-failure";
export {
  SessionRuntimeRecoveryNotice,
  type SessionRuntimeRecoveryNoticeProps,
} from "./components/session-runtime-recovery-notice";
export {
  SessionStopAttentionNotice,
  type SessionStopAttentionNoticeProps,
} from "./components/session-stop-attention-notice";
export {
  STOP_VERIFICATION_FAILED_ATTENTION,
  sessionStopAttention,
  type SessionStopAttention,
} from "./lib/session-stop-attention";
export {
  SessionQuietWarningNotice,
  type SessionQuietWarningNoticeProps,
} from "./components/session-quiet-warning-notice";
export {
  SessionQuietStatusRow,
  type SessionQuietStatusRowProps,
} from "./components/session-quiet-status-row";
export {
  SessionThinkingRow,
  type SessionThinkingRowProps,
} from "./components/session-thinking-row";
export {
  agentCountLabel,
  deriveWorkingStatus,
  formatFrozenDuration,
  formatWorkingElapsed,
  type SessionLastTurn,
  type SessionLastTurnCause,
  type SessionWorkingStatus,
  type SessionWorkingStatusInput,
} from "./lib/session-working-status";
export {
  assistantMessageHasContent,
  deriveThinkingState,
  THINKING_FLICKER_GUARD_MS,
  thinkingGuardRemainingMs,
  type SessionThinkingState,
} from "./lib/session-thinking-state";
export {
  liveToolLabel,
  parallelToolLabel,
  toolVisualKind,
  toolVisualState,
  toolVisualStatus,
  type SessionToolKind,
  type SessionToolVisualState,
  type SessionToolVisualStatus,
} from "./lib/session-tool-visual-state";
export {
  formatPayloadSize,
  PAYLOAD_PREVIEW_MAX_LINES,
  payloadTruncationNote,
  truncatePayload,
  type PayloadTruncation,
} from "./lib/session-payload-truncation";
export {
  releaseFarTranscriptPages,
  TRANSCRIPT_KEEP_PAGES,
  transcriptPageIndexOf,
} from "./lib/session-transcript-window";
export { steerMarkerView, type SteerMarkerKind, type SteerMarkerView } from "./lib/steer-marker";
export {
  deriveSteerProvenance,
  emptySteerProvenance,
  type SteerMarkerRender,
  type SteerProvenance,
  type SteerProvenanceIndex,
} from "./lib/steer-provenance";
export { SteerProvenanceProvider } from "./lib/steer-provenance-context";
export {
  deriveTurnOutcomes,
  emptyTurnOutcomes,
  type SessionTurnOutcome,
  type SessionTurnOutcomes,
} from "./lib/session-turn-outcomes";
export { SessionTurnOutcomesProvider } from "./lib/session-turn-outcomes-context";
export {
  formatQuietDuration,
  formatQuietDurationWords,
  sessionQuietWarning,
  sessionQuietWarningClock,
  sessionQuietWarningFacts,
  type SessionQuietWarning,
  type SessionQuietWarningClock,
  type SessionQuietWarningFacts,
} from "./lib/session-quiet-warning";
export { SessionStatusLine, type SessionStatusLineProps } from "./components/session-status-line";
export {
  SessionDeleteDialog,
  type SessionDeleteDialogProps,
} from "./components/session-delete-dialog";
export {
  SessionRenameDialog,
  type SessionRenameDialogProps,
} from "./components/session-rename-dialog";
export { SessionRowActions, type SessionRowActionsProps } from "./components/session-row-actions";
export { SessionList, type SessionListProps } from "./components/session-list/session-list";
export { SessionSidebar, type SessionSidebarProps } from "./components/session-sidebar";
export {
  buildSessionTree,
  childSessionSignalTone,
  collectThreadSessions,
  visibleSessionOrder,
  type VisibleSessionOrderOptions,
} from "./lib/session-hierarchy";
export {
  SessionWorkspaceSwitchDialog,
  type SessionWorkspaceSwitchDialogProps,
} from "./components/session-workspace-switch-dialog";
export { SessionToolCallRow, type SessionToolCallRowProps } from "./components/tool-call-card";
export {
  SessionChatRuntimeProvider,
  type SessionChatRuntimeProviderProps,
} from "./components/session-chat-runtime-provider";
export { ThinkingBlock, type ThinkingBlockProps } from "./components/thinking-block";
export {
  PermissionDataPart,
  PermissionExpiredReceipt,
  PermissionReceipt,
  type PermissionExpiredReceiptProps,
  type PermissionReceiptProps,
} from "./components/permission-data-part";
export { PermissionDock, type PermissionDockProps } from "./components/permission-dock";
export { ClarificationDock, type ClarificationDockProps } from "./components/clarification-dock";
export {
  SessionDecisionDock,
  type SessionDecisionDockProps,
} from "./components/session-decision-dock";
export {
  SessionGoalHeadAction,
  type SessionGoalHeadActionProps,
} from "./components/goal/goal-head-action";
export { SessionGoalStrip, type SessionGoalStripProps } from "./components/goal/session-goal-strip";
export {
  ClarificationReceipt,
  type ClarificationReceiptProps,
} from "./components/clarification-receipt";
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
export { deriveFileReads, type InspectorFileEntry } from "./components/session-inspector.logic";

// Environment surface — worktree binding, fork, target selection (./environment).
export * from "./environment";
export { sendFirstPrompt, FIRST_PROMPT_SEND_FAILED } from "./lib/session-first-prompt";
export * from "./quote";
export type { SessionSendAction } from "./lib/session-busy-input";
export * from "./hooks";
