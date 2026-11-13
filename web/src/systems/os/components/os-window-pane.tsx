import { OverlayContainerContext, SkeletonRows, Spinner } from "@compozy/ui";
import { memo, Suspense, useDeferredValue, useState, type MouseEvent, type ReactNode } from "react";

import { WindowScopeContext } from "@/hooks/use-window-scope";
import {
  WindowDataBindingContext,
  WorkspaceBadge,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import { WindowLiveDataContext } from "../contexts/window-live-data-context";
import { useDesktop } from "../hooks/use-desktop";
import { useOsTabDragStart, useOsTabPreviewed, useOsWindowDragged } from "../hooks/use-os-tab-drag";
import { useOsWindowMember } from "../hooks/use-os-window-member";
import { useDeskBoundToActingProfile } from "../hooks/use-window-manager-store";
import { osAppId } from "../lib/app-catalog";
import { getOsApp } from "../lib/app-registry";
import { ensureWindowSlotStore } from "../lib/window-slot-registry";
import { OsWindowErrorBoundary } from "./os-window-error-boundary";
import { OsWindowLoadingFallback } from "./os-window-loading-fallback";
import { OsWindowSurface } from "./os-window-surface";

/**
 * One window in its tab's split: the app's surface with its head, toolbar and body. A window
 * hidden behind a collapsed split's switcher, or in a tab that is not showing, stays mounted so
 * returning to it never reloads its surface.
 */
export const OsWindowPane = memo(function OsWindowPane({ windowId }: { windowId: string }) {
  const member = useOsWindowMember(windowId);
  const { coordinator, win, focused, liveDataEnabled, windowBinding } = member;
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);
  // Mount layout-heavy bodies, including app skeletons, after the pane commits.
  const contentHost = useDeferredValue(overlayHost);
  const deskBound = useDeskBoundToActingProfile();
  const { startHeadDrag } = useOsTabDragStart();
  const dragged = useOsWindowDragged(windowId);
  // A drag previews another tab's content without activating it: its windows show too.
  const previewed = useOsTabPreviewed(win?.tabId ?? null);
  const projectedShown = useDesktop(
    state =>
      state.projections[win?.tabId ?? ""]?.windows.find(entry => entry.windowId === windowId)
        ?.shown ?? true
  );
  if (!win) return null;
  const visible = win.shown || (previewed && projectedShown);
  const appId = osAppId(win.app);
  if (appId === null) return null;
  const app = getOsApp(appId);
  const Controller = app.Controller;
  const AppIcon = app.icon;

  // Navigation intent is classified at the click, not inferred from paths
  // (ADR-011): a link inside the body drills in (`push`); a breadcrumb selects
  // a parent, and the coordinator checks whether it matches the tab's stack.
  const classifyNavigation = (event: MouseEvent<HTMLElement>) => {
    const element = event.target instanceof Element ? event.target : null;
    if (!element) return;
    if (element.closest('[data-slot="topbar-back"], [data-slot="topbar-crumb"]')) {
      coordinator.noteNavigateMode("pop");
      return;
    }
    if (element.closest("a[href]") && element.closest('[data-slot="os-window-body"]')) {
      coordinator.noteNavigateMode("push");
    }
  };

  return (
    <OsWindowSurface
      onClickCapture={classifyNavigation}
      onFocusCapture={member.handleFocusCapture}
      onPointerDownCapture={member.handlePointerDownCapture}
      onPointerEnter={member.handlePointerEnter}
      title={app.title}
      // Root heads show the app in the identity well until the surface
      // publishes its own glyph (or a drill-in trail replaces it).
      glyph={<AppIcon />}
      leading={projectBadgeLead(app, windowBinding.badge, windowId)}
      focused={focused}
      onClose={member.close}
      slotStore={ensureWindowSlotStore(windowId)}
      // The runtime animates in-window drills against this one named body.
      bodyTransitionName={focused && win.shown ? "os-window-body" : undefined}
      onHeadPointerDown={event => startHeadDrag(windowId, event)}
      className={dragged ? "relative opacity-45" : "relative"}
      hidden={!visible}
      // Named region landmark per window: assistive tech can jump between
      // windows the way sighted users scan the desktop (WCAG landmarks).
      aria-label={`${app.title} window`}
      data-testid={`os-window-${windowId}`}
      data-window-id={windowId}
      data-app={win.app}
      data-instance-key={win.instanceKey}
      data-focused={focused ? "" : undefined}
      data-shown={visible ? "" : undefined}
    >
      <OverlayContainerContext.Provider value={contentHost}>
        {contentHost && deskBound ? (
          <OsWindowErrorBoundary title={app.title}>
            <Suspense
              fallback={
                <OsWindowLoadingFallback
                  app={app}
                  generic={
                    <SkeletonRows role="status" aria-label="Loading" className="gap-4 p-4" />
                  }
                  windowId={windowId}
                />
              }
            >
              <WindowLiveDataContext value={liveDataEnabled}>
                {/* Scopes per-window selection (e.g. active worktree) without
                    threading windowId through every descendant hook. */}
                <WindowScopeContext value={windowId}>
                  <WindowDataBindingContext value={windowBinding.data}>
                    <Controller windowId={windowId} />
                  </WindowDataBindingContext>
                </WindowScopeContext>
              </WindowLiveDataContext>
            </Suspense>
          </OsWindowErrorBoundary>
        ) : (
          <div className="flex min-h-32 flex-1 items-center justify-center">
            <Spinner className="size-4 text-subtle" />
          </div>
        )}
      </OverlayContainerContext.Provider>
      <div
        ref={setOverlayHost}
        data-slot="os-window-overlays"
        // contain-paint makes portaled fixed-position scrims resolve against
        // this box, so a dialog dims and travels with its window only.
        className="contain-paint pointer-events-none absolute inset-0 z-40 [&:not(:empty)]:pointer-events-auto"
      />
    </OsWindowSurface>
  );
});

/**
 * The bound item's project in the head on the All projects desktop, unless the app names its
 * project in its own head. Absent (not an empty node) so the head keeps no leading zone.
 */
function projectBadgeLead(
  app: { hidesProjectBadge?: true },
  identity: WorkspaceBadgeIdentity | null,
  windowId: string
): ReactNode {
  if (identity === null || app.hidesProjectBadge) return undefined;
  return (
    <WorkspaceBadge
      identity={identity}
      variant="chrome"
      data-testid={`os-window-project-${windowId}`}
    />
  );
}
