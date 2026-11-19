/**
 * The session system's side chat surface (`_spec.md` side chat): the side panel's pure views, the
 * list/open/close hooks, the head toggle, and the window-scoped shortcut request. Grouped so the
 * public barrel reads as one contract.
 */
export {
  fetchSessionSides,
  openSessionSide,
  type OpenSessionSideRequest,
  type OpenSessionSideResult,
  type SessionSideItem,
  type SessionSidesResponse,
} from "./adapters/session-side-api";
export { SessionSideCloseDialog } from "./components/side-chat/session-side-close-dialog";
export {
  SessionSideHeader,
  type SessionSideHeaderProps,
} from "./components/side-chat/session-side-header";
export { SessionSideMainStatus } from "./components/side-chat/session-side-main-status";
export {
  SessionSidePanelBody,
  type SessionSidePanelBodyState,
} from "./components/side-chat/session-side-panel-body";
export { SessionSideThreadBoundary } from "./components/side-chat/session-side-thread-boundary";
export { SessionSideToggle } from "./components/side-chat/session-side-toggle";
export { useCloseSideChat } from "./hooks/use-close-side-chat";
export {
  newSideChatKey,
  type SideChatOpenOutcome,
  useOpenSideChat,
} from "./hooks/use-open-side-chat";
export {
  type SessionSidesModel,
  useSessionHasSideChats,
  useSessionSides,
} from "./hooks/use-session-sides";
export {
  canHostSideChats,
  defaultSideChatId,
  isSideChatSession,
  nextSideChatId,
  openSideChats,
  parseSideCommand,
  SIDE_CHAT_COPY,
  sideChatHostRefusal,
  type SideMainStatus,
  type SideMainStatusKind,
  sideChatNeedsYou,
  sideChatTitle,
  sideMainStatus,
  sideMainStatusOf,
} from "./lib/session-side";
export { isSessionSidesQuery, sessionSidesOptions } from "./lib/session-side-query-options";
// The panel's focus handoff to the main session's composer or pending permission.
export {
  findSessionFocusTarget,
  SESSION_FOCUS_TARGET,
  type SessionFocusTarget,
} from "./lib/session-focus-target";
export {
  requestSessionSideToggle,
  subscribeSessionSideToggle,
} from "./lib/session-side-toggle-request";
