/**
 * The Session app's loading shapes, on their own entry so a caller that shows them before the
 * session code has loaded (the Sessions window's loading fallback) reads only saved presentation
 * state, without pulling queries, the panel library or the live board in with the barrel.
 */
export { SessionSidebarSkeleton } from "./components/session-list/session-list-skeleton";
export { SessionSidebarStaticLayout } from "./components/session-sidebar-static-layout";
export {
  SessionsBoardSkeleton,
  type SessionsBoardSkeletonProps,
} from "./components/session-board/sessions-board-skeleton";

export { useSessionWindowViewState } from "./hooks/use-session-window-view-state";
