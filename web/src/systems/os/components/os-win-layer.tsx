import { MonitorX } from "lucide-react";
import type { CSSProperties } from "react";

import { Empty } from "@compozy/ui";

import type { DesktopLayerModel, OsWinLayerModel } from "../hooks/use-os-win-layer";
import { useWindowManagerGesturePreview } from "../hooks/use-window-manager-store";
import type { LayoutProjection } from "../lib/window-manager-types";
import type { DesktopTransitionIntent } from "../stores/window-manager-store";
import { OsEmptyDesktop } from "./os-empty-desktop";
import { OsSnapOverlay } from "./os-snap-overlay";
import { OsSnapSeamLayer, type SeamGestureHandlers } from "./os-snap-seam";
import { OsWindow } from "./os-window";

/** Keeps frame-rate gesture updates from re-rendering the desktop and its windows. */
function LiveSnapOverlay() {
  const preview = useWindowManagerGesturePreview();
  return <OsSnapOverlay preview={preview} />;
}

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
  model: DesktopLayerModel;
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

function DesktopLayer({
  model,
  compact,
  reducedMotion,
  viewportReady,
  transition,
  onTransitionComplete,
  seamProjection,
  onResize,
  onFrameResize,
  onSeamPreview,
  onFrameSeamPreview,
  onSeamPreviewEnd,
  paletteShortcutLabel,
  hasProject,
  onPickProject,
}: {
  model: DesktopLayerModel;
  compact: boolean;
  reducedMotion: boolean;
  viewportReady: boolean;
  transition: DesktopTransitionIntent | null;
  onTransitionComplete: () => void;
  seamProjection: LayoutProjection | undefined;
  paletteShortcutLabel: string | null;
  hasProject: boolean;
  onPickProject?: () => void;
} & SeamGestureHandlers) {
  const { incoming, interactive, style } = desktopLayerView({
    model,
    reducedMotion,
    viewportReady,
    transition,
  });

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
      {interactive && !model.anyVisible ? (
        <OsEmptyDesktop
          desktopName={model.desktop.name}
          paletteShortcutLabel={paletteShortcutLabel}
          hasProject={hasProject}
          onPickProject={onPickProject}
        />
      ) : null}
      {model.frames.map(frame => (
        <OsWindow key={frame.id} frame={frame} />
      ))}
      {!compact && interactive ? (
        <>
          <OsSnapSeamLayer
            projection={seamProjection}
            onResize={onResize}
            onFrameResize={onFrameResize}
            onSeamPreview={onSeamPreview}
            onFrameSeamPreview={onFrameSeamPreview}
            onSeamPreviewEnd={onSeamPreviewEnd}
          />
          <LiveSnapOverlay />
        </>
      ) : null}
    </section>
  );
}

/** Every desktop tree remains mounted; only the client-active tree is interactive. */
export function OsWinLayer({
  model,
  reducedMotion,
  transition,
  onTransitionComplete,
  onResize,
  onFrameResize,
  onSeamPreview,
  onFrameSeamPreview,
  onSeamPreviewEnd,
  paletteShortcutLabel,
  hasProject,
  onPickProject,
}: {
  model: OsWinLayerModel;
  reducedMotion: boolean;
  transition: DesktopTransitionIntent | null;
  onTransitionComplete: () => void;
  paletteShortcutLabel: string | null;
  /** A project is active (Global has none): gates the empty desktop's composer. */
  hasProject: boolean;
  /** Global scope has no project: the empty desktop offers the workspace picker instead. */
  onPickProject?: () => void;
} & SeamGestureHandlers) {
  const { layerRef, desktops, presentation, viewportState, activeProjection } = model;
  return (
    <div
      ref={layerRef}
      data-slot="os-win-layer"
      // The desk is the whole work area: the rail and the compact tab bar own their own grid tracks.
      className="absolute inset-0"
    >
      {desktops.map(desktop => (
        <DesktopLayer
          key={desktop.desktop.id}
          model={desktop}
          compact={presentation === "compact"}
          reducedMotion={reducedMotion}
          viewportReady={viewportState === "ready"}
          transition={transition}
          onTransitionComplete={onTransitionComplete}
          seamProjection={activeProjection}
          onResize={onResize}
          onFrameResize={onFrameResize}
          onSeamPreview={onSeamPreview}
          onFrameSeamPreview={onFrameSeamPreview}
          onSeamPreviewEnd={onSeamPreviewEnd}
          paletteShortcutLabel={paletteShortcutLabel}
          hasProject={hasProject}
          onPickProject={onPickProject}
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
