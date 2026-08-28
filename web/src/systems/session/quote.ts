export { SessionTerminalQuoteSlot } from "./components/session-terminal-quote-slot";
export { useSessionTerminalQuote } from "./hooks/use-session-terminal-quote";
export {
  claimPendingTerminalQuoteForCreate,
  clearPendingTerminalQuote,
  clearSessionTerminalQuote,
  discardSessionTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  peekSessionTerminalQuote,
  restorePendingTerminalQuoteAfterFailedCreate,
  stageChosenSessionTerminalQuote,
  stagePendingTerminalQuoteForSession,
  stageSessionTerminalQuote,
  takePendingTerminalQuote,
} from "./lib/session-terminal-quote";
export {
  createTerminalQuoteHostActions,
  type TerminalQuoteHost,
  type TerminalQuoteSelection,
} from "./lib/session-terminal-quote-actions";
export {
  applyTerminalQuoteToPromptMessage,
  composeQuotedPrompt,
  composeSessionPromptWithTerminalQuote,
  splitQuotedPrompt,
} from "./lib/session-terminal-quote-prompt";
