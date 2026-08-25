// Types
export type {
  ACPCaps,
  AgentEventPayload,
  CompozyPermissionData,
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
  SessionInputPayload,
  SessionInputsResponse,
  SessionLedgerEvent,
  SessionLedgerMeta,
  SessionLedgerResponse,
  SessionMessage,
  SessionByIDResponse,
  SessionOwnerResponse,
  SessionPayload,
  SessionAttachment,
  SessionBusyInputDraft,
  SessionBusyInputHandler,
  SessionPromptAttachment,
  SessionPromptPayload,
  SessionPromptRequest,
  SessionPromptResponse,
  SessionPromptResult,
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

// Adapters
export {
  SessionPromptRuntimeProvider,
  type SessionPromptRuntimeProviderProps,
} from "./contexts/session-prompt-runtime-context";
export {
  getSessionPromptRuntimeSnapshot,
  useSessionPromptRuntime,
} from "./hooks/use-session-prompt-runtime";
export {
  useOptionalSessionPromptRuntimeContext,
  useSessionPromptRuntimeContext,
} from "./hooks/use-session-prompt-runtime-context";
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
  clearSessionRuntime,
  ClarificationNotAnswerableError,
  createSession,
  fetchSessionClarifications,
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
  steerSessionPrompt,
  stopSession,
  unarchiveSession,
} from "./adapters/session-api";
export type { SessionRewindRequest, SessionRewindResult } from "./adapters/session-api";
export {
  useSetSessionRuntime,
  type SetSessionRuntimeVariables,
} from "./hooks/use-session-runtime-selection";

// Query infrastructure
export { sessionPromptCapability } from "./lib/session-prompt-capability";
export type { SessionPromptCapability } from "./lib/session-prompt-capability";
// Attachment surface — byte URLs, prompt parts, item shaping, cards (./attachments).
export {
  attachmentExtensionMark,
  attachmentsFromPromptMessageParts,
  consumeSubmittedComposerAttachment,
  formatAttachmentBytes,
  isImageMediaType,
  retainSubmittedComposerAttachments,
  sessionAttachmentBytesURL,
  sessionAttachmentIdFromURI,
  sessionAttachmentMediaType,
  SessionAttachmentFileCard,
  SessionAttachmentFrame,
  SessionAttachmentGallery,
  userMessageAttachmentItems,
  userMessageHasAttachments,
  userMessageHasText,
  type SessionAttachmentFileCardProps,
  type SessionAttachmentFileItem,
  type SessionAttachmentFrameProps,
  type SessionAttachmentGalleryProps,
  type SessionAttachmentImageItem,
  type SessionAttachmentItem,
} from "./attachments";
export { formatMessageTimestamp, formatMessageTimestampFull } from "./lib/format-timestamp";
export { isClarifyEventData } from "./lib/clarify-event";
export { isAgentEventPayload, resolveToolResult } from "./lib/message-parts";
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
export { queuedPromptAttachmentSummary } from "./lib/queued-prompt";
export type {
  QueuedPrompt,
  QueuedPromptAttachmentPreview,
  QueuedPromptAttachmentSummary,
} from "./lib/queued-prompt";
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
  sessionScopedDetailOptions,
  sessionsListOptions,
  sessionsCompleteListOptions,
} from "./lib/query-options";
export {
  sessionCommandMenuCatalog,
  useSessionCommands,
  type SessionCommandMenuCatalog,
  type SessionCommandMenuItem,
  type SessionCommandMenuSection,
} from "./hooks/use-session-commands";
export {
  useCancelSessionInput,
  usePromoteSessionInput,
  useReplaceSessionInput,
  useSessionInputs,
  type PromoteSessionInputVariables,
  type ReplaceSessionInputVariables,
} from "./hooks/use-session-inputs";
export {
  canPromptSession,
  hasUnrecoverableRuntime,
  hasRunningSession,
  idleAttachableAgentNames,
  isSessionRunning,
  isUserControllableSession,
  runningAgentNames,
} from "./lib/session-running";

// Stores
export { useSessionComposerDraft, useSessionGoalFeedback } from "./hooks/use-session-store";
export type {
  SessionGoalFeedback,
  SessionStoreContext,
  SessionStore,
} from "./stores/session-store";
export { sessionStore } from "./stores/session-store";

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
  useForeignProfileSession,
  type ForeignProfileSessionState,
} from "./hooks/use-foreign-profile-session";
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
export {
  toggleSessionSidebar,
  useSessionSidebarState,
  type UseSessionSidebarStateResult,
} from "./hooks/use-session-sidebar-state";
export type {
  SessionTranscriptThreadState,
  SessionTranscriptThreadStatus,
} from "./lib/session-transcript-thread-context-value";
export {
  useClearSessionConversation,
  useArchiveSession,
  useCancelQueuedSessionPrompt,
  useCreateSession,
  useDeleteSession,
  useInterruptSessionPrompt,
  useQueueSessionPrompt,
  useRepairSession,
  useRenameSession,
  useResumeSession,
  useSendSessionPrompt,
  useSteerSessionPrompt,
  useStopSession,
  useUnarchiveSession,
  type CancelQueuedSessionPromptParams,
  type RepairSessionParams,
  type RenameSessionParams,
  type SendSessionPromptParams,
  type SessionPromptActionParams,
} from "./hooks/use-session-actions";
export { useSessionRewind, type SessionRewindVariables } from "./hooks/use-session-rewind";
export {
  useSessionLifecycleActions,
  type SessionDeleteConfirmation,
  type SessionLifecycleAction,
  type SessionLifecycleActionHandlers,
  type SessionRenameConfirmation,
  type UseSessionLifecycleActionsOptions,
  type UseSessionLifecycleActionsResult,
} from "./hooks/use-session-lifecycle-actions";
export {
  useSessionCreateDialogController,
  useSessionCreateDialogViewModel,
  type SessionCreateDialogApi,
  type SessionCreateDialogController,
  type SessionCreateDialogDraft,
  type SessionCreateDialogState,
} from "./hooks/use-session-create-dialog";
export { SessionCreateProvider } from "./contexts/session-create-context";
export {
  useSessionCreateActions,
  useSessionCreateHasActiveWorkspace,
  useSessionCreateIsCreating,
  useSessionCreatePendingAgentName,
  useSessionCreateStore,
} from "./hooks/use-session-create";
export { useSessionPromptFallback } from "./hooks/use-session-prompt-fallback";
export { createSessionCreateStore, type SessionCreateStore } from "./stores/session-create-store";

// Components
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
  PermissionReceipt,
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
export { useSessionGoalHeader } from "./hooks/use-session-goal-header";
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
export { deriveFileReads, type InspectorFileEntry } from "./components/session-inspector.logic";

// Environment surface — worktree binding, fork, target selection (./environment).
export {
  environmentTargetLabel,
  forkSessionToWorktree,
  isEnvironmentTargetMissing,
  NEW_WORKTREE_LABEL,
  ROOT_ENVIRONMENT_TARGET,
  selectableWorktrees,
  SessionEnvironmentChip,
  SessionEnvironmentControl,
  SessionEnvironmentField,
  SessionWorktreeBindingChip,
  SessionWorktreeForkDialog,
  useForkSessionToWorktree,
  useSessionEnvironment,
  useSessionWorktreeBinding,
  WORKSPACE_ROOT_LABEL,
  WORKTREE_COMMAND_TOKEN,
  type SessionEnvironmentChipState,
  type SessionEnvironmentControlHandle,
  type SessionEnvironmentModel,
  type SessionEnvironmentTarget,
  type SessionWorktreeBinding,
} from "./environment";
export { findSessionCommand } from "./hooks/use-session-commands";
export { useSessionFirstPrompt } from "./hooks/use-session-first-prompt";
export { sendFirstPrompt, FIRST_PROMPT_SEND_FAILED } from "./lib/session-first-prompt";
export { useSessionPromptStaging } from "./hooks/use-session-prompt-staging";
export type { SessionPromptStaging } from "./hooks/use-session-prompt-staging";
