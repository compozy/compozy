import { SESSION_EMPTY_PATH } from "../../lib/app-catalog";
import type { OsWindowRoute } from "../../lib/os-types";
import { SessionWindowSkeleton } from "./session-window-skeleton";

/**
 * The session window's body while its code loads: the rail-and-pane skeleton the window shows
 * while its session resolves, so the two are one continuous frame. A route that names a session
 * gets the transcript; the empty route (`/sessions`) names none and keeps the pane clear.
 */
export function SessionWindowFallback({ route }: { route: OsWindowRoute }) {
  return (
    <SessionWindowSkeleton
      inert
      pane={route.pathname === SESSION_EMPTY_PATH ? "empty" : "thread"}
    />
  );
}
