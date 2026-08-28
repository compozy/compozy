// Public API for the terminal system. Cross-system imports come through here
// and never reach into internals.

export {
  TerminalApiError,
  answerTerminalInputRequest,
  closeTerminal,
  controlTerminalRecording,
  createTerminal,
  fetchTerminalInputRequestProjection,
  rejectTerminalInputRequest,
  signalTerminal,
  waitTerminal,
} from "./adapters/terminal-api";
export { TerminalStoreProvider } from "./contexts/terminal-store-context";
export {
  useTerminalAttachment,
  type TerminalAttachment,
  type TerminalAttachmentSocketFactory,
  type UseTerminalAttachmentOptions,
} from "./hooks/use-terminal-attachment";
export { useTerminalReplay } from "./hooks/use-terminal-replay";
export { useTerminalStore } from "./hooks/use-terminal-store";

export {
  AsciicastPlayback,
  formatPlaybackClock,
  parseAsciicast,
  AsciicastParseError,
  type Asciicast,
  type AsciicastFrame,
} from "./lib/asciicast";
export {
  projectTerminalBadge,
  terminalsRunning,
  type TerminalBadgeInput,
  type TerminalBadgeProjection,
  type TerminalPendingApproval,
} from "./lib/terminal-badge";
export {
  parseTerminalCatalogEvent,
  reconcileTerminalCatalog,
  TERMINAL_CATALOG_EVENTS,
  type TerminalCatalogEvent,
} from "./lib/terminal-catalog-stream";
export {
  terminalApprovalCopy,
  terminalConfidenceCopy,
  terminalErrorCopy,
  terminalExitCopy,
  terminalGapCopy,
  terminalInputOutcomeCopy,
} from "./lib/terminal-copy";
export { terminalJournalActorLabel } from "./lib/terminal-journal-actor";
export {
  copyTerminalJournalCommand,
  terminalJournalOutputSummary,
} from "./lib/terminal-journal-copy";
export { formatTerminalJournalClock } from "./lib/terminal-journal-clock";
export {
  shouldKeepTerminalJournalHost,
  type TerminalJournalHostRetention,
} from "./lib/terminal-journal-host";
export {
  terminalAttachModeFor,
  terminalLeaseView,
  type TerminalControlRead,
  type TerminalLeaseView,
} from "./lib/terminal-lease";
export {
  buildTerminalQuote,
  copySourcedTerminalQuote,
  parseTerminalQuote,
  sourcedTerminalQuoteText,
  terminalQuoteFromSelection,
  terminalSelectionLines,
  type TerminalQuote,
} from "./lib/terminal-quote";
export {
  clearChooseSessionTerminalQuote,
  holdChooseSessionTerminalQuote,
  peekChooseSessionTerminalQuote,
  takeChooseSessionTerminalQuote,
} from "./lib/terminal-quote-choose";
export {
  clearPendingTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takePendingTerminalQuote,
} from "./lib/terminal-quote-pending";
export {
  TERMINAL_ALL_PROFILES_KEY,
  TERMINAL_JOURNAL_PAGE_SIZE,
  terminalJournalFiltersWithDefaults,
  terminalKeys,
} from "./lib/query-keys";
export {
  terminalCatalogQuery,
  terminalInputRequestsQuery,
  terminalJournalQuery,
  terminalRecordingQuery,
  terminalScope,
  type TerminalQueryScope,
} from "./lib/query-options";
export { terminalAttentionLocation } from "./lib/terminal-attention-location";
export {
  terminalInputAttentionTitle,
  terminalInputRequestTitle,
  terminalInputStackNeedsOrigin,
} from "./lib/terminal-input-identity";
export { terminalRedactedInputCopy } from "./lib/terminal-redacted-marker";
export { useTerminalInputAnswer } from "./hooks/use-terminal-input-answer";
export {
  TerminalProtocolClient,
  type TerminalStreamHandlers,
  type TerminalStreamSink,
  type TerminalStreamStatus,
} from "./lib/terminal-protocol-client";
export { terminalStoreLogic, type TerminalPaneState } from "./stores/terminal-store";

export {
  SessionTerminalBlock,
  type SessionTerminalBlockProps,
} from "./components/session-terminal-block";
export {
  TerminalApprovalDetail,
  type TerminalApprovalDetailProps,
} from "./components/terminal-approval-detail";
export {
  TerminalEmptyState,
  TerminalExecuteOnlyState,
  TerminalExpiredState,
  TerminalNotFoundState,
} from "./components/terminal-empty-states";
export { TerminalGrantRow } from "./components/terminal-grant-row";
export {
  isTerminalBroaderDecisionForbidden,
  terminalGrantFromToolGrant,
  type TerminalGrant,
  type TerminalGrantKind,
} from "./lib/terminal-grant";
export {
  terminalAlwaysAllowLabel,
  terminalAskTitle,
  terminalAttentionReason,
  terminalGrantLabel,
  terminalIdFromDetail,
  terminalRejectOnceLabel,
} from "./lib/terminal-permission-copy";
export { useKnownTerminalTitle } from "./hooks/use-known-terminal-title";
export {
  isTerminalPermission,
  terminalBlockedRememberedDecisions,
  terminalPermissionDetail,
  type TerminalPermissionDetail,
  type TerminalPermissionRisk,
} from "./lib/terminal-permission";
export {
  terminalInstanceKey,
  terminalInstanceKeyInScope,
  terminalScopeKey,
} from "./lib/terminal-scope-key";
export { TerminalHeader } from "./components/terminal-header";
export {
  TerminalInputRequestCard,
  TerminalInputResolvedRow,
} from "./components/terminal-input-request";
export {
  TerminalInputRequestStack,
  type TerminalInputRequestStackProps,
} from "./components/terminal-input-request-stack";
export { TerminalJournalDetail } from "./components/terminal-journal-detail";
export {
  terminalJournalChipsFromFilters,
  terminalJournalFilterFields,
  terminalJournalFiltersFromChips,
} from "./lib/terminal-journal-filter-fields";
export {
  TerminalJournalHead,
  TerminalJournalPanel,
  type TerminalJournalPanelProps,
} from "./components/terminal-journal-panel";
export { TerminalLeaseBadge } from "./components/terminal-lease-badge";
export { TerminalGapSeam, TerminalStreamNotice } from "./components/terminal-notices";
export { TerminalPane } from "./components/terminal-pane";
export { TerminalPipeLogPane } from "./components/terminal-pipe-log-pane";
export { TerminalQuoteBlock, TerminalSelectionActions } from "./components/terminal-quote-block";
export { TerminalRecordingPlayer } from "./components/terminal-recording-player";
export { TERMINAL_JOURNAL_TAB, type TerminalTabId } from "./components/terminal-tab-id";
export { TerminalTabs } from "./components/terminal-tabs";
export {
  useTerminalCatalogStream,
  type TerminalCatalogEventSourceFactory,
  type UseTerminalCatalogStreamOptions,
} from "./hooks/use-terminal-catalog-stream";
export { useTerminalRecordings } from "./hooks/use-terminal-recordings";
export {
  applyRecordingStopSuccess,
  type TerminalRecordingMap,
} from "./lib/terminal-recording-state";
export {
  TerminalLimitDialog,
  type TerminalLimitDialogProps,
} from "./components/terminal-limit-dialog";
export { TerminalTakeoverDialog } from "./components/terminal-takeover-dialog";
export {
  TerminalWindowApp,
  type TerminalWindowActions,
  type TerminalWindowAppProps,
} from "./components/terminal-window-app";

export type {
  TerminalActor,
  TerminalApproval,
  TerminalDetectedBy,
  TerminalExit,
  TerminalInfo,
  TerminalInputOutcome,
  TerminalInputActorProjection,
  TerminalInputRequestProjection,
  TerminalPendingInputRequest,
  TerminalInputRequest,
  TerminalResolvedInputRequest,
  TerminalJournalEntry,
  TerminalJournalFilters,
  TerminalJournalPage,
  TerminalLeaseState,
  TerminalMode,
  TerminalScopeKey,
  TerminalScopeParams,
  TerminalSignal,
  TerminalViewerIdentity,
  TerminalWaitResult,
  TerminalWaitUntil,
} from "./types";
