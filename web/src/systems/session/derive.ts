/**
 * The session system's derive surface: continuing a session with another
 * agent and forking it with the same one, plus how a derived session shows
 * where it came from. Grouped so the public barrel reads as one contract.
 */
export { SessionDeriveHost } from "./components/session-derive-host";
export { SessionContinueDialog } from "./components/session-continue-dialog";
export { SessionForkDialog } from "./components/session-fork-dialog";
export {
  SessionContinueDivider,
  SessionContinueEmptyChild,
} from "./components/session-continue-divider";
export { SessionOriginPill } from "./components/session-origin-pill";
export {
  SessionDeriveContext,
  type SessionContinueRequest,
} from "./contexts/session-derive-context-value";
export {
  SessionForkContext,
  type SessionForkPoint,
  type SessionForkRequest,
} from "./contexts/session-fork-context-value";
export {
  SessionOriginContext,
  type SessionOriginContextValue,
} from "./contexts/session-origin-context-value";
export { useSessionDeriveHost, type SessionDeriveHostState } from "./hooks/use-session-derive-host";
export {
  landDerivedSession,
  type SessionDerivePlacement,
  type SessionDerivePlacementHandlers,
} from "./hooks/use-session-derive";
export { useSessionOrigin } from "./hooks/use-session-origin";
export type { SessionOriginView } from "./lib/session-origin";
export {
  continueSession,
  fetchSessionDerivePreview,
  forkSession,
  type ContinueSessionRequest,
  type ForkSessionRequest,
  type SessionDerivePreview,
  type SessionDeriveResult,
} from "./adapters/session-derive-api";
