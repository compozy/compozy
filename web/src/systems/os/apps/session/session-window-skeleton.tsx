import { SessionSidebarLayout, SessionSidebarSkeleton } from "@/systems/session";

import { SessionWindowSkeletonShape } from "./session-window-skeleton-shape";

/**
 * The session window while its session resolves, in the same rail-and-pane split the loaded
 * window mounts (`SessionSidebarLayout` keeps the rail's open state and width). The frame shown
 * before the window's code has loaded is `SessionWindowFallback`, the same shape in a static split.
 */
export function SessionWindowSkeleton({ pane }: { pane?: "thread" | "empty" }) {
  return (
    <SessionWindowSkeletonShape
      Layout={SessionSidebarLayout}
      pane={pane}
      sidebar={<SessionSidebarSkeleton />}
    />
  );
}
