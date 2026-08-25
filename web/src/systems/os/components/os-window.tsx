import { OverlayContainerContext, Spinner, type TopbarSlotStore } from "@compozy/ui";
import { shallowEqual } from "@xstate/store";
import { Suspense, useState } from "react";
import { Rnd } from "react-rnd";

import { cn } from "@/lib/utils";

import { WindowScopeContext } from "@/hooks/use-window-scope";

import { WindowLiveDataContext } from "../contexts/window-live-data-context";
import {
  OS_WINDOW_DRAG_CANCEL_SELECTOR,
  OS_WINDOW_DRAG_HANDLE_CLASS,
  useOsWindow,
} from "../hooks/use-os-window";
import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { useWindowManagerGestureDragging } from "../hooks/use-window-manager-store";
import { useWindowLiveDataEnabled } from "../hooks/use-window-live-data-enabled";
import { useWindowMergeTarget } from "../hooks/use-window-merge-target";
import { getOsApp, getOsAppMinimum } from "../lib/app-registry";
import type { OsWindowFrameModel } from "../lib/group-projection";
import { ensureWindowSlotStore } from "../lib/window-slot-registry";
import { shortcutActionLabel } from "../lib/window-manager-shortcuts";
import { windowVisualLayer } from "../lib/window-visual-layer";
import { OsWindowDeck } from "./os-window-deck";
import { OsWindowErrorBoundary } from "./os-window-error-boundary";
import { OsWindowChrome, OsWindowSurface } from "./os-window-frame";
import { OsZoomMenu } from "./os-zoom-menu";

export interface OsWindowProps {
  frame: OsWindowFrameModel;
}

/**
 * One window frame rendered from the current client projection: a solo window
 * keeps today's chrome exactly (D1), a group hosts the deck plus every
 * member's surface — inactive members stay mounted but hidden, so switching
 * tabs never reloads a surface (US-003).
 */
export function OsWindow({ frame }: OsWindowProps) {
  const {
    focused,
    handleDrag,
    handleDragStart,
    handleDragStop,
    handleFocusCapture,
    handlePointerDownCapture,
    handlePointerEnter,
    handleResizeStart,
    handleResizeStop,
    handleTrafficLight,
    keepMounted,
    rect,
    registerRnd,
    resizeMax,
  } = useOsWindow(frame);
  const { presentation, activeApp, closeShortcut, newTabShortcut } = useDesktop(state => {
    const effective = state.windowManagerConfig?.effectiveShortcuts;
    const platform = typeof navigator === "undefined" ? "" : navigator.platform;
    return {
      presentation: state.presentation,
      activeApp: state.windows[frame.activeWindowId]?.app ?? null,
      closeShortcut: shortcutActionLabel(effective, "window.close", platform) ?? undefined,
      newTabShortcut: shortcutActionLabel(effective, "window.tab.new", platform) ?? undefined,
    };
  }, shallowEqual);
  const dragging = useWindowManagerGestureDragging(frame.activeWindowId);
  const compact = presentation === "compact";
  const deckVisible = frame.members.length >= 2 && !compact;
  const { chromeRef, mergeTargeted } = useWindowMergeTarget(frame, !deckVisible && !compact);
  const slotStores = new Map<string, TopbarSlotStore>(
    frame.members.map(member => [member, ensureWindowSlotStore(member)])
  );
  if (!keepMounted || activeApp === null) return null;

  const minimum = getOsAppMinimum(activeApp);
  // Floating frames resize on every edge; tiled frames only on free edges —
  // shared boundaries resize through their seam, matching the daemon contract.
  const edges = frame.resizableEdges;
  const enableResizing =
    frame.kind === "floating"
      ? true
      : {
          left: edges.left,
          right: edges.right,
          top: edges.top,
          bottom: edges.bottom,
          topLeft: edges.top && edges.left,
          topRight: edges.top && edges.right,
          bottomLeft: edges.bottom && edges.left,
          bottomRight: edges.bottom && edges.right,
        };

  return (
    <Rnd
      ref={registerRnd}
      className={cn(compact && "absolute inset-0", focused && "will-change-transform")}
      position={compact ? { x: 0, y: 0 } : { x: rect.x, y: rect.y }}
      size={compact ? { width: "100%", height: "100%" } : { width: rect.w, height: rect.h }}
      minWidth={compact ? undefined : minimum.width}
      minHeight={compact ? undefined : minimum.height}
      maxWidth={resizeMax?.width}
      maxHeight={resizeMax?.height}
      disableDragging={compact}
      enableResizing={!compact && enableResizing}
      dragHandleClassName={OS_WINDOW_DRAG_HANDLE_CLASS}
      cancel={OS_WINDOW_DRAG_CANCEL_SELECTOR}
      onDragStart={handleDragStart}
      onDrag={handleDrag}
      onDragStop={handleDragStop}
      onResizeStart={handleResizeStart}
      onResizeStop={handleResizeStop}
      style={{
        zIndex: windowVisualLayer(frame),
        display: frame.minimized ? "none" : undefined,
      }}
    >
      <OsWindowChrome
        ref={chromeRef}
        focused={focused}
        presentation={presentation}
        kind={frame.kind}
        // Reduced opacity keeps merge and snap affordances readable through
        // the moving frame without rerasterizing the full window during drag.
        className={cn(
          "relative h-full w-full transition-opacity duration-base ease-out",
          dragging && "opacity-70"
        )}
        data-dragging={dragging ? "" : undefined}
        data-testid={`os-window-frame-${frame.id}`}
        data-frame-kind={frame.kind}
        onPointerEnter={handlePointerEnter}
        onPointerDownCapture={handlePointerDownCapture}
        onFocusCapture={handleFocusCapture}
      >
        {deckVisible ? (
          <OsWindowDeck
            frame={frame}
            slotStores={slotStores}
            onTrafficLight={handleTrafficLight}
            zoomMenu={button => <OsZoomMenu windowId={frame.activeWindowId}>{button}</OsZoomMenu>}
            dragHandleClassName={OS_WINDOW_DRAG_HANDLE_CLASS}
            shortcutLabels={{ close: closeShortcut, newTab: newTabShortcut }}
          />
        ) : null}
        {frame.members.map(member => (
          <OsWindowMember
            key={member}
            windowId={member}
            active={member === frame.activeWindowId}
            focused={focused && member === frame.activeWindowId}
            controls={deckVisible ? "deck" : "head"}
            presentation={presentation}
            slotStore={slotStores.get(member)}
            onTrafficLight={handleTrafficLight}
          />
        ))}
        {mergeTargeted ? (
          <div
            aria-hidden="true"
            data-slot="os-window-merge-target"
            className="pointer-events-none absolute inset-x-0 top-0 z-50 flex h-11 items-center justify-center border border-accent bg-accent-tint"
          >
            <span className="rounded-sm bg-elevated px-2 py-1 text-form-hint font-medium text-fg shadow-overlay">
              Group as tabs
            </span>
          </div>
        ) : null}
      </OsWindowChrome>
    </Rnd>
  );
}

function OsWindowMember({
  windowId,
  active,
  focused,
  controls,
  presentation,
  slotStore,
  onTrafficLight,
}: {
  windowId: string;
  active: boolean;
  focused: boolean;
  controls: "head" | "deck";
  presentation: "floating" | "compact";
  slotStore: TopbarSlotStore | undefined;
  onTrafficLight: Parameters<typeof OsWindowSurface>[0]["onTrafficLight"];
}) {
  const { coordinator } = useOsShell();
  const win = useDesktop(state => state.windows[windowId]);
  const liveDataEnabled = useWindowLiveDataEnabled(windowId);
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);
  if (!win) return null;
  const app = getOsApp(win.app);
  const Controller = app.Controller;
  const compact = presentation === "compact";

  // Navigation intent is classified at the click, not inferred from paths
  // (ADR-011): a link inside the body drills in (`push`); the breadcrumb back
  // pops the tab's own stack.
  const classifyNavigation = (event: React.MouseEvent<HTMLElement>) => {
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
      title={app.title}
      focused={focused}
      controls={controls}
      onTrafficLight={controls === "head" ? onTrafficLight : undefined}
      zoomMenu={
        compact || controls === "deck"
          ? undefined
          : button => <OsZoomMenu windowId={windowId}>{button}</OsZoomMenu>
      }
      presentation={presentation}
      slotStore={slotStore}
      headClassName={cn(
        !compact &&
          controls === "head" &&
          `${OS_WINDOW_DRAG_HANDLE_CLASS} cursor-grab active:cursor-grabbing`
      )}
      className={cn("relative", !active && "hidden")}
      hidden={!active}
      // Named region landmark per window: assistive tech can jump between
      // windows the way sighted users scan the desktop (WCAG landmarks).
      aria-label={`${app.title} window`}
      data-testid={`os-window-${windowId}`}
      data-app={win.app}
      data-instance-key={win.instanceKey}
      data-minimized={win.minimized ? "" : undefined}
      data-window-placement={win.placement}
      data-stack-active={active ? "" : undefined}
    >
      <OverlayContainerContext.Provider value={overlayHost}>
        {overlayHost ? (
          <OsWindowErrorBoundary title={app.title}>
            <Suspense
              fallback={
                <div className="flex min-h-32 flex-1 items-center justify-center">
                  <Spinner className="size-4 text-subtle" />
                </div>
              }
            >
              <WindowLiveDataContext value={liveDataEnabled}>
                {/* Scopes per-window selection (e.g. active worktree) without
                    threading windowId through every descendant hook. */}
                <WindowScopeContext value={windowId}>
                  <Controller windowId={windowId} />
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
}
