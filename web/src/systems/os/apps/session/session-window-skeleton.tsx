import { SkeletonRegion } from "@compozy/ui";

import { SessionThreadSkeletonBody } from "@/components/assistant-ui/session-thread-skeleton";
import { SessionSidebarLayout, SessionSidebarSkeleton } from "@/systems/session";

/**
 * The session window before its session is known: the same rail-and-pane split the loaded window
 * mounts (`SessionSidebarLayout` keeps the rail's open state and width), with the rail's shell
 * over placeholder rows and the pane as a transcript over a composer card. Every frame the loaded
 * window will fill is already in place, so the swap moves nothing. One status region for the
 * whole window, and no queries: a restored desktop of hidden session windows loads cheaply.
 */
export function SessionWindowSkeleton() {
  return (
    <SkeletonRegion
      label="Loading session"
      data-testid="session-route-loading"
      className="flex min-h-0 min-w-0 flex-1 overflow-hidden"
    >
      <SessionSidebarLayout sidebar={<SessionSidebarSkeleton />}>
        <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <SessionThreadSkeletonBody />
        </div>
      </SessionSidebarLayout>
    </SkeletonRegion>
  );
}
