// Narrow public entry for the terminal system.
//
// Everything exported here is free of the emulator, so a surface that merely
// *mentions* terminals — the session decision dock, the remembered-decisions
// section, the composer's quote slot — can import it without pulling the
// terminal chunk into a bundle its users may never open. The full barrel
// (`@/systems/terminal`) stays behind a lazy boundary.

export {
  TerminalApprovalDetail,
  type TerminalApprovalDetailProps,
} from "./components/terminal-approval-detail";
export { TerminalGrantRow } from "./components/terminal-grant-row";
export { useKnownTerminalTitle } from "./hooks/use-known-terminal-title";
export {
  isTerminalBroaderDecisionForbidden,
  terminalGrantFromToolGrant,
  type TerminalGrant,
  type TerminalGrantKind,
  type ToolApprovalGrantLike,
} from "./lib/terminal-grant";
export {
  terminalAlwaysAllowLabel,
  terminalAskTitle,
  terminalAttentionReason,
  terminalGrantLabel,
  terminalIdFromDetail,
  terminalRejectOnceLabel,
} from "./lib/terminal-permission-copy";
export { TerminalQuoteBlock, TerminalSelectionActions } from "./components/terminal-quote-block";
export {
  isTerminalPermission,
  terminalBlockedRememberedDecisions,
  terminalPermissionDetail,
  type TerminalPermissionDetail,
  type TerminalPermissionRisk,
} from "./lib/terminal-permission";
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
  clearPendingTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takePendingTerminalQuote,
} from "./lib/terminal-quote-pending";
export {
  clearChooseSessionTerminalQuote,
  holdChooseSessionTerminalQuote,
  peekChooseSessionTerminalQuote,
  takeChooseSessionTerminalQuote,
} from "./lib/terminal-quote-choose";
export {
  terminalApprovalCopy,
  terminalConfidenceCopy,
  terminalExitCopy,
  terminalInputOutcomeCopy,
  terminalReplayFailedCopy,
} from "./lib/terminal-copy";
// The dock reads the badge without ever opening a terminal, so the projection
// belongs on the emulator-free entry alongside the other mention-only surfaces.
export {
  projectTerminalBadge,
  terminalsRunning,
  type TerminalBadgeInput,
  type TerminalBadgeProjection,
  type TerminalPendingApproval,
} from "./lib/terminal-badge";
// Session transcript blocks need replay, lease projection, and catalog types
// without opening the Terminal app.
export { useTerminalReplay } from "./hooks/use-terminal-replay";
export { useTerminalCatalogStream } from "./hooks/use-terminal-catalog-stream";
export { terminalLeaseView, type TerminalLeaseView } from "./lib/terminal-lease";
export { terminalCatalogQuery, terminalScope } from "./lib/catalog-query";
export type { TerminalExit, TerminalInfo, TerminalRunState, TerminalSignal } from "./types";
