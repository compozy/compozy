import { Kbd } from "@agh/ui";

import type { DesktopLayerModel, OsWinLayerModel } from "../hooks/use-os-win-layer";
import type { SnapTarget } from "../lib/snap-targets";
import type { LayoutProjection } from "../lib/window-manager-types";
import type { DesktopTransitionIntent } from "../stores/window-manager-store";
import { OsSnapOverlay } from "./os-snap-overlay";
import { OsSnapSeamLayer, type SeamGestureHandlers } from "./os-snap-seam";
import { OsWindow } from "./os-window";

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

function DesktopLayer({
  model,
  compact,
  reducedMotion,
  viewportReady,
  transition,
  onTransitionComplete,
  seamProjection,
  preview,
  onResize,
  onSeamPreview,
  onSeamPreviewEnd,
}: {
  model: DesktopLayerModel;
  compact: boolean;
  reducedMotion: boolean;
  viewportReady: boolean;
  transition: DesktopTransitionIntent | null;
  onTransitionComplete: () => void;
  seamProjection: LayoutProjection | undefined;
  preview: SnapTarget | null;
} & SeamGestureHandlers) {
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
      style={{
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
      }}
    >
      {interactive && !model.anyVisible ? (
        <p
          data-testid="os-desk-hint"
          className="pointer-events-none absolute top-1/2 left-1/2 flex -translate-x-1/2 -translate-y-1/2 items-center gap-2 text-small-body text-subtle select-none"
        >
          <Kbd>⌘K</Kbd> to open anything — or pick a surface from the dock
        </p>
      ) : null}
      {model.windowIds.map(id => (
        <OsWindow key={id} windowId={id} />
      ))}
      {!compact && interactive ? (
        <>
          <OsSnapSeamLayer
            projection={seamProjection}
            onResize={onResize}
            onSeamPreview={onSeamPreview}
            onSeamPreviewEnd={onSeamPreviewEnd}
          />
          <OsSnapOverlay preview={preview} />
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
  preview,
  onTransitionComplete,
  onResize,
  onSeamPreview,
  onSeamPreviewEnd,
}: {
  model: OsWinLayerModel;
  reducedMotion: boolean;
  transition: DesktopTransitionIntent | null;
  preview: SnapTarget | null;
  onTransitionComplete: () => void;
} & SeamGestureHandlers) {
  const { layerRef, desktops, presentation, viewportState, activeProjection } = model;
  return (
    <div
      ref={layerRef}
      data-slot="os-win-layer"
      // The measured work area stops above the Dock band so snaps and floating clamps never resolve beneath it.
      className="absolute inset-x-0 top-0 bottom-[calc(var(--size-dock-band)+env(safe-area-inset-bottom,0px))]"
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
          preview={preview}
          onResize={onResize}
          onSeamPreview={onSeamPreview}
          onSeamPreviewEnd={onSeamPreviewEnd}
        />
      ))}
      {viewportState === "rejected" ? (
        <div
          role="status"
          data-testid="os-viewport-rejected"
          className="absolute inset-0 grid place-items-center px-6"
        >
          <div className="max-w-sm border border-line bg-surface px-5 py-4 text-center shadow-overlay">
            <p className="text-body font-semibold text-foreground">
              This window is too narrow for the configured layout.
            </p>
            <p className="mt-1 text-small-body text-muted">
              Widen it or change the small viewport policy in Settings › Layouts.
            </p>
          </div>
        </div>
      ) : null}
    </div>
  );
}
