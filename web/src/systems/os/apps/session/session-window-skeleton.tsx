import { SkeletonRegion } from "@compozy/ui";

import { SessionThreadSkeletonBody } from "@/components/assistant-ui/session-thread-skeleton";
import { SessionSidebarLayout, SessionSidebarSkeleton } from "@/systems/session";

interface SessionWindowSkeletonProps {
  /**
   * The window that names no session has nothing to stand in for the pane, since the message
   * that fills it arrives with the window's code.
   */
  pane?: "thread" | "empty";
  /** Takes the rail's resize handle out of reach, for a frame shown before the window mounts. */
  inert?: boolean;
}

/**
 * The session window before its session is known: the same rail-and-pane split the loaded window
 * mounts (`SessionSidebarLayout` keeps the rail's open state and width), with the rail's shell
 * over placeholder rows and the pane as a transcript over a composer card. Every frame the loaded
 * window will fill is already in place, so the swap moves nothing. One status region for the
 * whole window, and no queries: a restored desktop of hidden session windows loads cheaply.
 */
export function SessionWindowSkeleton({
  pane = "thread",
  inert = false,
}: SessionWindowSkeletonProps) {
  const split = (
    <SessionSidebarLayout sidebar={<SessionSidebarSkeleton />}>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {pane === "thread" ? <SessionThreadSkeletonBody /> : null}
      </div>
    </SessionSidebarLayout>
  );

  return (
    <SkeletonRegion
      label={pane === "thread" ? "Loading session" : "Loading sessions"}
      data-testid="session-route-loading"
      className="flex min-h-0 min-w-0 flex-1 overflow-hidden"
    >
      {inert ? (
        <div className="contents" inert>
          {split}
        </div>
      ) : (
        split
      )}
    </SkeletonRegion>
  );
}
