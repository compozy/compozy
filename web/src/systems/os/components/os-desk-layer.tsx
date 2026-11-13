import { MonitorX } from "@compozy/ui/icons";
import type { CSSProperties } from "react";

import { Empty } from "@compozy/ui";
import type { SessionPayload } from "@/systems/session";

import {
  useOsDeskLayer,
  useVisitedTabs,
  type DeskDesktopModel,
  type OsDeskLayerModel,
} from "../hooks/use-os-desk-layer";
import { useTabSessions } from "../hooks/use-os-tab-bar";
import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { useOsSplitSeam } from "../hooks/use-os-split-seam";
import { useOsTabDrag } from "../hooks/use-os-tab-drag";
import type { OsWindow } from "../lib/os-types";
import type { TabProjection } from "../lib/window-manager-types";
import type { TabDragState } from "../stores/os-tab-drag-store";
import { OsDropZone, OsRootDropZone } from "./os-drop-zone";
import type { DesktopTransitionIntent } from "../stores/window-manager-store";
import { OsSplitSeam, OsSplitView, type OsSplitSeamSlot } from "./os-split-view";
import { OsWindowPane } from "./os-window-pane";

/** Maps transition state to a keyframe name; reduced motion still fires an instant fade so end events run. */
function desktopTransitionAnimationName(input: {
  reducedMotion: boolean;
  transitionActive: boolean;
  incoming: boolean;
  outgoing: boolean;
  active: boolean;
  mode: DesktopTransitionIntent["mode"] | undefined;
  direction: DesktopTransitionIntent["direction"] | undefined;
}): string | undefined {
  if (!input.transitionActive) return undefined;
  if (input.reducedMotion) {
    return input.incoming && input.active ? "os-desk-fade-in" : undefined;
  }
  if (input.incoming && input.active) {
    if (input.mode === "crossfade") return "os-desk-fade-in";
    return input.direction === "later" ? "os-desk-in-later" : "os-desk-in-earlier";
  }
  if (input.outgoing && !input.active) {
    if (input.mode === "crossfade") return "os-desk-fade-out";
    return input.direction === "later" ? "os-desk-out-later" : "os-desk-out-earlier";
  }
  return undefined;
}

/** Visibility, interactivity, and the enter/leave animation for one desktop tree. */
function desktopLayerView(input: {
  model: DeskDesktopModel;
  reducedMotion: boolean;
  viewportReady: boolean;
  transition: DesktopTransitionIntent | null;
}) {
  const { model, reducedMotion, viewportReady, transition } = input;
  const incoming = transition?.toDesktopId === model.desktop.id;
  const outgoing = transition?.fromDesktopId === model.desktop.id;
  const transitionActive =
    transition !== null && transition.mode !== "instant" && (incoming || outgoing);
  const visible = viewportReady && (model.active || transitionActive);
  const interactive = viewportReady && model.active;
  // Animations start only once the active flag flips: the entering desktop
  // animates while active, the leaving desktop while inactive.
  const animation = desktopTransitionAnimationName({
    reducedMotion,
    transitionActive,
    incoming,
    outgoing,
    active: model.active,
    mode: transition?.mode,
    direction: transition?.direction,
  });
  const style: CSSProperties = {
    contain: "strict",
    contentVisibility: visible ? "visible" : "hidden",
    opacity: model.active ? 1 : 0,
    pointerEvents: interactive ? "auto" : "none",
    animation:
      animation === undefined
        ? undefined
        : `${animation} ${
            reducedMotion ? "0ms linear" : "var(--duration-shell-base) var(--ease-spring)"
          } both`,
  };
  return { incoming, interactive, style };
}

const renderWindow = (windowId: string) => <OsWindowPane key={windowId} windowId={windowId} />;

/** A laid-out seam: drag to resize, double-click to balance, arrow keys when focused. */
function LiveOsSplitSeam({ slot }: { slot: OsSplitSeamSlot }) {
  const { state, onPointerDown, onKeyDown, onDoubleClick } = useOsSplitSeam(
    slot.seam,
    slot.memberCount
  );
  if (slot.seam === undefined) return <OsSplitSeam axis={slot.axis} gap={slot.gap} />;
  const { leadingWeight, trailingWeight } = slot.seam;
  const pair = leadingWeight + trailingWeight;
  return (
    <OsSplitSeam
      axis={slot.axis}
      gap={slot.gap}
      valueNow={pair > 0 ? Math.round((leadingWeight / pair) * 100) : undefined}
      state={state}
      onPointerDown={onPointerDown}
      onKeyDown={onKeyDown}
      onDoubleClick={onDoubleClick}
    />
  );
}

const renderSeam = (slot: OsSplitSeamSlot) => (
  <LiveOsSplitSeam key={`${slot.splitId}:${slot.boundaryIndex}`} slot={slot} />
);

/** The drop zone a drag shows over this tab's content, if its target is here. */
function DropZones({
  tabId,
  drag,
  projection,
}: {
  tabId: string;
  drag: TabDragState;
  projection: TabProjection | undefined;
}) {
  const target = drag.target;
  if (drag.phase !== "dragging" || drag.contentTabId !== tabId || target === null) return null;
  if (target.kind === "root")
    return target.tabId === tabId ? <OsRootDropZone edge={target.edge} /> : null;
  if (target.kind !== "window" && target.kind !== "swap") return null;
  const rect = projection?.windows.find(entry => entry.windowId === target.windowId)?.rect;
  if (rect === undefined) return null;
  return target.kind === "swap" ? (
    <OsDropZone rect={rect} edge="left" mode="swap" />
  ) : (
    <OsDropZone rect={rect} edge={target.edge} />
  );
}

function DeskLayer({
  model,
  windows,
  sessionOf,
  reducedMotion,
  viewportReady,
  transition,
  onTransitionComplete,
}: {
  model: DeskDesktopModel;
  windows: Readonly<Record<string, OsWindow>>;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  reducedMotion: boolean;
  viewportReady: boolean;
  transition: DesktopTransitionIntent | null;
  onTransitionComplete: () => void;
}) {
  const { coordinator } = useOsShell();
  const projections = useDesktop(state => state.projections);
  const gaps = useDesktop(state => state.windowManagerConfig?.gaps);
  const { incoming, interactive, style } = desktopLayerView({
    model,
    reducedMotion,
    viewportReady,
    transition,
  });
  const drag = useOsTabDrag();
  const tabs = model.desktop.tabs;
  // A drag shows another tab's content as its drop target; nothing activates until the drop.
  const previewing =
    drag.phase === "dragging" &&
    drag.session?.desktopId === model.desktop.id &&
    drag.contentTabId !== null &&
    tabs.some(tab => tab.id === drag.contentTabId);
  const shownTabId = previewing ? drag.contentTabId : model.activeTabId;
  const visited = useVisitedTabs(
    tabs.map(tab => tab.id),
    shownTabId
  );

  return (
    <section
      data-screen-label={`Desktop ${model.desktop.order + 1}: ${model.desktop.name}`}
      data-desktop-id={model.desktop.id}
      data-active={model.active ? "true" : "false"}
      aria-hidden={!interactive}
      inert={interactive ? undefined : true}
      className="absolute inset-0"
      onAnimationEnd={event => {
        if (event.target === event.currentTarget && model.active && incoming) {
          onTransitionComplete();
        }
      }}
      style={style}
    >
      {visited.map(tabId => {
        const tab = tabs.find(entry => entry.id === tabId);
        if (tab === undefined) return null;
        const active = tabId === shownTabId;
        const projection = projections[tabId];
        return (
          <div
            key={tabId}
            role="tabpanel"
            hidden={!active}
            className="absolute inset-0 flex"
            data-slot="os-tab-content"
            data-tab-id={tabId}
          >
            <OsSplitView
              root={tab.root}
              windows={windows}
              collapsed={projection?.collapsed ?? []}
              seams={projection?.seams}
              sessionOf={sessionOf}
              renderWindow={renderWindow}
              renderSeam={renderSeam}
              onShowMember={windowId => void coordinator.userFocus(windowId)}
              gaps={gaps}
            >
              <DropZones tabId={tabId} drag={drag} projection={projection} />
            </OsSplitView>
          </div>
        );
      })}
    </section>
  );
}

/** Live wiring for the desk: the measured content area and every desktop's tab content. */
export function LiveOsDeskLayer(props: Omit<OsDeskLayerProps, "model">) {
  const model = useOsDeskLayer();
  return <OsDeskLayer {...props} model={model} />;
}

export interface OsDeskLayerProps {
  model: OsDeskLayerModel;
  reducedMotion: boolean;
  transition: DesktopTransitionIntent | null;
  onTransitionComplete: () => void;
}

/**
 * Every desktop tree remains mounted with the tabs it has shown; only the client-active desktop
 * is interactive, and only its active tab is visible.
 */
export function OsDeskLayer({
  model,
  reducedMotion,
  transition,
  onTransitionComplete,
}: OsDeskLayerProps) {
  const { layerRef, desktops, viewportState } = model;
  const windows = useDesktop(state => state.windows);
  const hasSession = Object.values(windows).some(win => win.app === "session");
  const sessionById = useTabSessions(hasSession);
  const sessionOf = (win: OsWindow) =>
    win.app === "session" ? sessionById.get(win.instanceKey ?? "") : undefined;
  return (
    <div ref={layerRef} data-slot="os-desk-layer" className="absolute inset-0">
      {desktops.map(desktop => (
        <DeskLayer
          key={desktop.desktop.id}
          model={desktop}
          windows={windows}
          sessionOf={sessionOf}
          reducedMotion={reducedMotion}
          viewportReady={viewportState === "ready"}
          transition={transition}
          onTransitionComplete={onTransitionComplete}
        />
      ))}
      {viewportState === "rejected" ? (
        <div
          role="status"
          data-testid="os-viewport-rejected"
          className="absolute inset-0 grid place-items-center px-6"
        >
          <Empty
            framed
            icon={MonitorX}
            className="max-w-sm bg-card shadow-overlay"
            title="Make the window wider to see your desktop"
            description="You can also change this in Settings › Layouts."
          />
        </div>
      ) : null}
    </div>
  );
}
