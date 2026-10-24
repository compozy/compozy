import { SessionSidebarSkeleton, SessionSidebarStaticLayout } from "@/systems/session/skeleton";

import { SESSION_EMPTY_PATH } from "../../lib/app-catalog";
import type { OsWindowRoute } from "../../lib/os-types";
import { SessionWindowSkeletonShape } from "./session-window-skeleton-shape";

/**
 * The session window's body while its code loads: the shape the window shows while its session
 * resolves, so the two are one continuous frame, in the split the loaded window will mount, at
 * this window's saved rail width. A route that names a session gets the transcript; the empty
 * route (`/sessions`) names none and keeps the pane clear.
 */
export function SessionWindowFallback({ route }: { route: OsWindowRoute }) {
  return (
    <SessionWindowSkeletonShape
      inert
      Layout={SessionSidebarStaticLayout}
      pane={route.pathname === SESSION_EMPTY_PATH ? "empty" : "thread"}
      sidebar={<SessionSidebarSkeleton />}
    />
  );
}
