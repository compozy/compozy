/**
 * The Sessions board's loading shape, on its own entry so a caller that shows it before the
 * session code has loaded (the Sessions window's loading fallback) does not pull the hooks, the
 * queries or the live board in with the barrel.
 */
export {
  SessionsBoardSkeleton,
  type SessionsBoardSkeletonProps,
} from "./components/session-board/sessions-board-skeleton";
