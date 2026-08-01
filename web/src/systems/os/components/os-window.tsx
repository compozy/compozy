import { OverlayContainerContext, Spinner, type TopbarSlotStore } from "@compozy/ui";
import { shallowEqual } from "@xstate/store";
import { Suspense, useState } from "react";
import { Rnd } from "react-rnd";

import { cn } from "@/lib/utils";

import {
  OS_WINDOW_DRAG_CANCEL_SELECTOR,
  OS_WINDOW_DRAG_HANDLE_CLASS,
  useOsWindow,
} from "../hooks/use-os-window";
import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { useWindowManagerGestureDragging } from "../hooks/use-window-manager-store";
import { useWindowMergeTarget } from "../hooks/use-window-merge-target";
import { getOsApp, getOsAppMinimum } from "../lib/app-registry";
import type { OsWindowFrameModel } from "../lib/group-projection";
import { ensureWindowSlotStore } from "../lib/window-slot-registry";
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
  const model = useOsWindow(frame);
  const { presentation, activeApp, reduceMotion } = useDesktop(
    state => ({
      presentation: state.presentation,
      activeApp: state.windows[frame.activeWindowId]?.app ?? null,
      reduceMotion: state.reduceMotion,
    }),
    shallowEqual
  );
  const dragging = useWindowManagerGestureDragging(frame.activeWindowId);
  const compact = presentation === "compact";
  const deckVisible = frame.members.length >= 2 && !compact;
  const merge = useWindowMergeTarget(frame, !deckVisible && !compact);
  const slotStores = new Map<string, TopbarSlotStore>(
    frame.members.map(member => [member, ensureWindowSlotStore(member)])
  );
  if (!model.keepMounted || activeApp === null) return null;

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
      ref={model.rndRef}
      className={compact ? "absolute inset-0" : undefined}
      position={compact ? { x: 0, y: 0 } : { x: model.rect.x, y: model.rect.y }}
      size={
        compact ? { width: "100%", height: "100%" } : { width: model.rect.w, height: model.rect.h }
      }
      minWidth={compact ? undefined : minimum.width}
      minHeight={compact ? undefined : minimum.height}
      maxWidth={model.resizeMax?.width}
      maxHeight={model.resizeMax?.height}
      disableDragging={compact}
      enableResizing={!compact && enableResizing}
      dragHandleClassName={OS_WINDOW_DRAG_HANDLE_CLASS}
      cancel={OS_WINDOW_DRAG_CANCEL_SELECTOR}
      onDragStart={model.handleDragStart}
      onDrag={model.handleDrag}
      onDragStop={model.handleDragStop}
      onResizeStart={model.handleResizeStart}
      onResizeStop={model.handleResizeStop}
      style={{
        zIndex: frame.layer,
        display: frame.minimized ? "none" : undefined,
      }}
    >
      <OsWindowChrome
        ref={merge.chromeRef}
        focused={model.focused}
        presentation={presentation}
        // Reduced opacity plus a slight shrink while dragging keeps the merge
        // and snap affordances underneath readable through the moving frame.
        className={cn(
          "relative h-full w-full transition-[opacity,transform] duration-base ease-out",
          dragging && "opacity-70",
          dragging && !reduceMotion && "scale-[0.98]"
        )}
        data-dragging={dragging ? "" : undefined}
        data-testid={`os-window-frame-${frame.id}`}
        data-frame-kind={frame.kind}
        onPointerEnter={model.handlePointerEnter}
        onPointerDownCapture={model.handlePointerDownCapture}
        onFocusCapture={model.handleFocusCapture}
      >
        {deckVisible ? (
          <OsWindowDeck
            frame={frame}
            slotStores={slotStores}
            onTrafficLight={model.handleTrafficLight}
            zoomMenu={button => <OsZoomMenu windowId={frame.activeWindowId}>{button}</OsZoomMenu>}
            dragHandleClassName={OS_WINDOW_DRAG_HANDLE_CLASS}
          />
        ) : null}
        {frame.members.map(member => (
          <OsWindowMember
            key={member}
            windowId={member}
            active={member === frame.activeWindowId}
            focused={model.focused && member === frame.activeWindowId}
            controls={deckVisible ? "deck" : "head"}
            presentation={presentation}
            slotStore={slotStores.get(member)}
            onTrafficLight={model.handleTrafficLight}
          />
        ))}
        {merge.mergeTargeted ? (
          <div
            aria-hidden="true"
            data-slot="os-window-merge-target"
            className="pointer-events-none absolute inset-x-0 top-0 z-50 flex h-11 items-center justify-center border border-accent bg-accent-tint"
          >
            <span className="rounded-sm bg-elevated px-2 py-1 text-form-hint font-medium text-fg shadow-elevated">
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
              <Controller windowId={windowId} />
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
