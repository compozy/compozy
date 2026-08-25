// Public API for the terminal system. Cross-system imports come through here
// and never reach into internals.

export {
  TerminalApiError,
  answerTerminalInputRequest,
  closeTerminal,
  controlTerminalRecording,
  createTerminal,
  rejectTerminalInputRequest,
  signalTerminal,
} from "./adapters/terminal-api";
export type { TerminalSocket, TerminalSocketFactory } from "./adapters/terminal-socket";

export { TerminalStoreProvider } from "./contexts/terminal-store-context";
export { useTerminalAttachment } from "./hooks/use-terminal-attachment";
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
export {
  terminalAttachModeFor,
  terminalLeaseView,
  type TerminalControlRead,
  type TerminalLeaseView,
} from "./lib/terminal-lease";
export {
  buildTerminalQuote,
  terminalSelectionLines,
  type TerminalQuote,
} from "./lib/terminal-quote";
export { TERMINAL_ALL_PROFILES_KEY, terminalKeys } from "./lib/query-keys";
export {
  TERMINAL_JOURNAL_PAGE_SIZE,
  terminalCatalogQuery,
  terminalDetailQuery,
  terminalInputRequestsQuery,
  terminalJournalQuery,
  terminalRecordingQuery,
  terminalScope,
} from "./lib/query-options";
export {
  TerminalProtocolClient,
  type TerminalStreamHandlers,
  type TerminalStreamSink,
  type TerminalStreamStatus,
} from "./lib/terminal-protocol-client";
export { terminalStoreLogic, type TerminalPaneState } from "./stores/terminal-store";

export { SessionTerminalBlock } from "./components/session-terminal-block";
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
  terminalGrantFromToolGrant,
  type TerminalGrant,
  type TerminalGrantKind,
} from "./lib/terminal-grant";
export {
  isTerminalPermission,
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
export { TerminalJournalDetail } from "./components/terminal-journal-detail";
export {
  TerminalJournalFilterDialog,
  type TerminalJournalFilterDialogProps,
} from "./components/terminal-journal-filter-dialog";
export { TerminalJournalPanel } from "./components/terminal-journal-panel";
export { TerminalLeaseBadge } from "./components/terminal-lease-badge";
export {
  TerminalAuditBlockedBar,
  TerminalGapSeam,
  TerminalStreamNotice,
} from "./components/terminal-notices";
export { TerminalPane } from "./components/terminal-pane";
export { TerminalPipeLogPane } from "./components/terminal-pipe-log-pane";
export { TerminalQuoteBlock, TerminalSelectionActions } from "./components/terminal-quote-block";
export { TerminalRecordingPlayer } from "./components/terminal-recording-player";
export { TERMINAL_JOURNAL_TAB, TerminalTabs, type TerminalTabId } from "./components/terminal-tabs";
export {
  useTerminalCatalogStream,
  type TerminalCatalogEventSourceFactory,
  type UseTerminalCatalogStreamOptions,
} from "./hooks/use-terminal-catalog-stream";
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
  TerminalInputRequest,
  TerminalJournalEntry,
  TerminalJournalFilters,
  TerminalJournalPage,
  TerminalLeaseState,
  TerminalMode,
  TerminalScopeKey,
  TerminalScopeParams,
} from "./types";
