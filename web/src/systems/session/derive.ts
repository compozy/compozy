/**
 * The session system's derive surface: continuing a session with another
 * agent (and, later, forking it), plus how a derived session shows where it
 * came from. Grouped so the public barrel reads as one contract.
 */
export { SessionContinueHost } from "./components/session-continue-host";
export { SessionContinueDialog } from "./components/session-continue-dialog";
export {
  SessionContinueDivider,
  SessionContinueEmptyChild,
} from "./components/session-continue-divider";
export { SessionOriginPill } from "./components/session-origin-pill";
export {
  SessionContinueContext,
  type SessionContinueRequest,
} from "./contexts/session-continue-context-value";
export {
  SessionOriginContext,
  type SessionOriginContextValue,
} from "./contexts/session-origin-context-value";
export {
  useSessionContinueHost,
  type SessionContinueHostState,
} from "./hooks/use-session-continue-host";
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
  type ContinueSessionRequest,
  type SessionDerivePreview,
  type SessionDeriveResult,
} from "./adapters/session-derive-api";
