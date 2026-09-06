// Live-follow scroll math + mode machine. The three-mode machine drives whether
// the viewport pins to the live edge (`following-end`), anchors a freshly-sent
// prompt near the top while its answer streams in below (`anchoring-new-turn`),
// or leaves the reader in control after a manual wheel/touch/pointer gesture
// (`free-scrolling`). `getRowBottom` / `getAnchoredTurnMetrics` are the pure
// measurement math; `createVirtualizerMeasurementState` adapts the TanStack
// virtualizer to the `TimelineListMeasurementState` shape the math consumes.

export type TimelineScrollMode = "following-end" | "anchoring-new-turn" | "free-scrolling";

export interface TimelineListMeasurementState {
  readonly data: readonly unknown[];
  readonly scroll: number;
  readonly scrollLength: number;
  readonly positionAtIndex: (index: number) => number | undefined;
  readonly sizeAtIndex: (index: number) => number | undefined;
}

export interface AnchoredTurnMetrics {
  readonly anchorTop: number;
  readonly lastBottom: number;
  readonly turnHeight: number;
  readonly usableViewportHeight: number;
  readonly visibleUsableBottom: number;
  readonly overflowsUsableViewport: boolean;
  readonly targetScrollToRevealEnd: number;
  readonly scrollDeltaToRevealEnd: number;
}

export function getRowBottom(state: TimelineListMeasurementState, index: number): number | null {
  const top = state.positionAtIndex(index);
  const height = state.sizeAtIndex(index);
  if (
    typeof top !== "number" ||
    typeof height !== "number" ||
    !Number.isFinite(top) ||
    !Number.isFinite(height)
  ) {
    return null;
  }

  return top + Math.max(1, height);
}

export function getAnchoredTurnMetrics({
  state,
  anchorIndex,
  composerOverlayHeight,
  anchorOffset,
}: {
  readonly state: TimelineListMeasurementState;
  readonly anchorIndex: number;
  readonly composerOverlayHeight: number;
  readonly anchorOffset: number;
}): AnchoredTurnMetrics | null {
  if (state.data.length === 0) {
    return null;
  }

  const boundedAnchorIndex = Math.max(0, Math.min(anchorIndex, state.data.length - 1));
  const anchorTop = state.positionAtIndex(boundedAnchorIndex);
  const lastBottom = getRowBottom(state, state.data.length - 1);
  if (typeof anchorTop !== "number" || !Number.isFinite(anchorTop) || lastBottom === null) {
    return null;
  }

  const usableViewportHeight = Math.max(
    0,
    state.scrollLength - composerOverlayHeight - anchorOffset
  );
  const turnHeight = Math.max(0, lastBottom - anchorTop);
  const visibleUsableBottom = state.scroll + usableViewportHeight;
  const targetScrollToRevealEnd = Math.max(0, lastBottom - usableViewportHeight);
  const scrollDeltaToRevealEnd = Math.max(0, targetScrollToRevealEnd - state.scroll);

  return {
    anchorTop,
    lastBottom,
    turnHeight,
    usableViewportHeight,
    visibleUsableBottom,
    overflowsUsableViewport: turnHeight > usableViewportHeight,
    targetScrollToRevealEnd,
    scrollDeltaToRevealEnd,
  };
}

// The single manual-navigation event collapses wheel/touch/pointer gestures — the
// hook wires all three DOM listeners to it — while the list lifecycle emits the
// other three. Keeping the transition table pure makes the mode machine unit
// testable without a DOM or virtualizer.
export type ScrollMachineEvent = "manual-navigation" | "new-turn" | "reached-end" | "scroll-to-end";

export function nextScrollMode(
  current: TimelineScrollMode,
  event: ScrollMachineEvent
): TimelineScrollMode {
  switch (event) {
    case "manual-navigation":
      return "free-scrolling";
    case "new-turn":
      return "anchoring-new-turn";
    case "reached-end":
    case "scroll-to-end":
      return "following-end";
    default:
      return current;
  }
}

// --- Scroll ownership (ADR-007) ---------------------------------------------
//
// One owner at a time: following pins to the live edge; the new-turn anchor
// holds the sent bubble near the top until the reply outgrows the reserved
// space; free leaves the reader alone. The reducer below is the whole contract:
// an upward gesture detaches at once, following re-arms only inside the bottom
// band or through the pill, a fold/expand suppresses end-maintenance for two
// frames so disclosure never yanks, and steering an already-streaming turn
// snaps instead of animating.

/** Re-arm band: t3code's lesson — a half-viewport re-arm yanked readers; 40px is strict but reliable. */
export const FOLLOW_REARM_BAND_PX = 40;
/** Frames of end-maintenance suppression after a disclosure toggles (t3code). */
export const FOLD_SETTLE_FRAMES = 2;
/** The sent bubble rests this far under the top of the viewport. */
export const NEW_TURN_ANCHOR_OFFSET_PX = 16;

export interface ScrollOwnershipState {
  readonly mode: TimelineScrollMode;
  /** Frames left before end-maintenance may move the viewport again. */
  readonly foldSettleFrames: number;
  /** A steer during streaming asked for an instant (never animated) return to the end. */
  readonly snapToEnd: boolean;
}

export const INITIAL_SCROLL_OWNERSHIP: ScrollOwnershipState = {
  mode: "following-end",
  foldSettleFrames: 0,
  snapToEnd: false,
};

export type ScrollOwnershipEvent =
  /** A wheel / touch / pointer gesture; `up` detaches instantly, `down` only re-arms through the band. */
  | { type: "user-scroll"; direction: "up" | "down"; atEnd: boolean }
  /** The viewport settled inside the bottom band (band test done by the caller). */
  | { type: "reached-end" }
  | { type: "jump-to-latest" }
  | { type: "new-turn" }
  /** The reply outgrew the anchored space: hand off to following. */
  | { type: "anchor-outgrown" }
  /** A steer was sent; while the turn streams the viewport snaps to the end. */
  | { type: "steer"; streaming: boolean }
  | { type: "fold-toggled" }
  | { type: "frame" };

export function scrollOwnershipTransition(
  state: ScrollOwnershipState,
  event: ScrollOwnershipEvent
): ScrollOwnershipState {
  switch (event.type) {
    case "user-scroll":
      if (event.direction === "up") {
        return { ...state, mode: "free-scrolling", snapToEnd: false };
      }
      return event.atEnd && state.mode === "free-scrolling"
        ? { ...state, mode: "following-end" }
        : state;
    case "reached-end":
      return state.mode === "following-end" ? state : { ...state, mode: "following-end" };
    case "jump-to-latest":
    case "anchor-outgrown":
      return { ...state, mode: "following-end" };
    case "new-turn":
      return { ...state, mode: "anchoring-new-turn", snapToEnd: false };
    case "steer":
      return event.streaming
        ? { ...state, mode: "following-end", snapToEnd: true }
        : { ...state, mode: "anchoring-new-turn", snapToEnd: false };
    case "fold-toggled":
      return { ...state, foldSettleFrames: FOLD_SETTLE_FRAMES };
    case "frame":
      return state.foldSettleFrames === 0
        ? state
        : { ...state, foldSettleFrames: state.foldSettleFrames - 1 };
  }
}

/** The snap was consumed by the viewport; clear the request without touching the mode. */
export function consumeSnapToEnd(state: ScrollOwnershipState): ScrollOwnershipState {
  return state.snapToEnd ? { ...state, snapToEnd: false } : state;
}

/** End-maintenance (pin on growth) runs only while following and outside a fold settle window. */
export function shouldMaintainEnd(state: ScrollOwnershipState): boolean {
  return state.mode === "following-end" && state.foldSettleFrames === 0;
}

export interface TimelineEndGeometry {
  readonly contentLength: number;
  readonly scroll: number;
  readonly scrollLength: number;
}

/** Inside the bottom band; the composer inset cancels out because it hides the same amount of viewport. */
export function resolveTimelineIsAtEnd(
  geometry: TimelineEndGeometry,
  bandPx: number = FOLLOW_REARM_BAND_PX
): boolean {
  return geometry.contentLength - geometry.scroll - geometry.scrollLength <= bandPx;
}

export interface PrependAnchor {
  readonly scrollTop: number;
  readonly scrollHeight: number;
}

/** Exact reading position after older history lands above: the same offset plus the height delta. */
export function prependScrollTop(anchor: PrependAnchor, nextScrollHeight: number): number {
  return Math.max(0, anchor.scrollTop + (nextScrollHeight - anchor.scrollHeight));
}

/** Composer growth pushes the viewport only while following; a reader elsewhere keeps their position. */
export function composerGrowthAdjustment(mode: TimelineScrollMode, deltaHeight: number): number {
  return mode === "following-end" ? Math.max(0, deltaHeight) : 0;
}

/** How much of the composer overlays the scroller — measured from rects, never assumed. */
export function measuredComposerOverlay(
  viewport: { readonly bottom: number },
  composer: { readonly top: number } | null
): number {
  if (!composer) return 0;
  return Math.max(0, viewport.bottom - composer.top);
}

// Minimal slice of the TanStack `Virtualizer` the shim reads (its public
// `measurementsCache`, kept current by `getVirtualItems()` during render), so the
// pure module never imports the virtualizer runtime.
export interface VirtualizerMeasurementSource {
  readonly options: { readonly count: number };
  readonly scrollOffset: number | null;
  readonly measurementsCache: ReadonlyArray<{ readonly start: number; readonly size: number }>;
}

export function createVirtualizerMeasurementState(
  virtualizer: VirtualizerMeasurementSource,
  viewportHeight: number
): TimelineListMeasurementState {
  const measurements = virtualizer.measurementsCache;
  const count = virtualizer.options.count;
  return {
    data: { length: count } as readonly unknown[],
    scroll: virtualizer.scrollOffset ?? 0,
    scrollLength: viewportHeight,
    positionAtIndex: index => measurements[index]?.start,
    sizeAtIndex: index => measurements[index]?.size,
  };
}
