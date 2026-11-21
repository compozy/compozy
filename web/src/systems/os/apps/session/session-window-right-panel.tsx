import { lazy, Suspense, type ReactNode } from "react";

import { ResizableHandle, ResizablePanel, ResizablePanelGroup, type Layout } from "@compozy/ui";

import {
  SessionReviewPanel,
  useSessionContextActivity,
  type SessionContextActivitySource,
  type SessionInspectorProps,
  type UseSessionInspectorStateResult,
  type UseSessionReviewPanelStateResult,
} from "@/systems/session";

import { loadSessionInspector } from "./session-window-module-loader";

const SessionInspector = lazy(() =>
  loadSessionInspector().then(module => ({ default: module.SessionInspector }))
);

const MAIN_PANEL_ID = "session-main";
const REVIEW_PANEL_ID = "session-review";

function SessionWindowInspector({
  activitySource: source,
  ...props
}: SessionInspectorProps & {
  activitySource: SessionContextActivitySource;
}) {
  const activity = useSessionContextActivity(
    source.session,
    source.running,
    source.live,
    source.queued,
    source.goal,
    props.usage?.workTime
  );
  return <SessionInspector {...props} activity={activity} />;
}

export type SessionWindowInspectorInput = Omit<
  SessionInspectorProps,
  "drawerOpen" | "onDrawerOpenChange" | "activity"
> & { activitySource: SessionContextActivitySource };

/**
 * The session window's right side (Business Rule 9): the resizable Review panel beside the
 * session pane, or the Inspector. The two never show together; their stores close each other.
 */
export function SessionWindowRightRegion({
  sessionId,
  workspaceId,
  review,
  inspector,
  inspectorProps,
  children,
}: {
  sessionId: string;
  workspaceId: string;
  review: UseSessionReviewPanelStateResult;
  inspector: UseSessionInspectorStateResult;
  inspectorProps: SessionWindowInspectorInput;
  /** The session pane (rail and transcript). */
  children: ReactNode;
}) {
  // A session without a file root has no toggle and no viewer links, so it never opens.
  const reviewOpen = review.open;
  const onLayoutChanged = (layout: Layout, meta: { isUserInteraction: boolean }) => {
    const percent = layout[REVIEW_PANEL_ID];
    if (!meta.isUserInteraction || percent === undefined) return;
    review.setSize(`${Math.round(percent * 10) / 10}%`);
  };
  return (
    <>
      <ResizablePanelGroup
        orientation="horizontal"
        className="min-h-0 min-w-0 flex-1"
        onLayoutChanged={onLayoutChanged}
        data-testid="session-window-split"
      >
        <ResizablePanel id={MAIN_PANEL_ID} minSize="25%" style={{ overflow: "hidden" }}>
          <div className="flex h-full min-h-0 min-w-0 overflow-hidden">{children}</div>
        </ResizablePanel>
        {reviewOpen ? (
          <>
            <ResizableHandle aria-label="Resize review panel" />
            <ResizablePanel
              id={REVIEW_PANEL_ID}
              defaultSize={review.size}
              minSize={360}
              maxSize="75%"
              style={{ overflow: "hidden" }}
            >
              <SessionReviewPanel sessionId={sessionId} workspaceId={workspaceId} review={review} />
            </ResizablePanel>
          </>
        ) : null}
      </ResizablePanelGroup>
      {inspector.open ? (
        <Suspense fallback={null}>
          <SessionWindowInspector
            {...inspectorProps}
            drawerOpen
            onDrawerOpenChange={open => {
              if (!open) inspector.close();
            }}
          />
        </Suspense>
      ) : null}
    </>
  );
}
