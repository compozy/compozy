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
