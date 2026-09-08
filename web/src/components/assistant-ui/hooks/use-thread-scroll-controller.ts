import { type VirtualItem, type Virtualizer, useVirtualizer } from "@tanstack/react-virtual";
import { type RefObject, useEffect, useLayoutEffect, useRef } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import {
  createVirtualizerMeasurementState,
  FOLLOW_REARM_BAND_PX,
  getAnchoredTurnMetrics,
  measuredComposerOverlay,
  NEW_TURN_ANCHOR_OFFSET_PX,
  shouldMaintainEnd,
} from "../timeline-scroll-anchoring";
import { estimateMessageSize, VIRTUAL_MESSAGE_ESTIMATE } from "../timeline-row-estimates";
import { useOptionalThreadScrollStore, type ThreadScrollStore } from "./thread-scroll-context";
import { firstVisibleRow, landMessageRow, lastVisibleRow } from "./thread-scroll-landing";
import { threadScrollLogic, threadScrollMode } from "./thread-scroll-store";

const LOAD_OLDER_CONTROL_ESTIMATE = 48;
const VIRTUAL_OVERSCAN = 6;
const INITIAL_VIEWPORT_RECT = { width: 1_024, height: 640 };
const LOAD_OLDER_CONTROL_KEY = "session-load-older-control";
const COMPOSER_SHELL_SELECTOR = '[data-testid="composer-shell"]';

interface ThreadScrollMessage {
  id?: string;
  role?: string;
}

export interface ThreadScrollControllerResult {
  captureHistoryAnchor: () => void;
  /** The first message whose row reaches into the viewport; `null` before any row is measured. */
  readVisibleMessageId: () => string | null;
  /** The first and last messages whose rows reach into the viewport (the trail's window). */
  readVisibleMessageIds: () => { first: string | null; last: string | null };
  /**
   * A find/trail jump: hands the viewport to the reader (free mode) and lands the
   * row — or, when `locate` names one, the matched content inside it — `offsetTop`
   * under the viewport's top edge, waiting for the virtualizer to mount and
   * measure it. Resolves `false` when the row never appeared.
   */
  scrollToMessage: (
    messageId: string,
    offsetTop: number,
    locate?: (row: HTMLElement) => Range | null
  ) => Promise<boolean>;
  leadingItemCount: number;
  measureVirtualElement: (element: HTMLDivElement | null) => void;
  showScrollToBottom: boolean;
  scrollToEnd: () => void;
  store: ThreadScrollStore;
  virtualItems: VirtualItem[];
  virtualTotalSize: number;
}

function findLastUserIndex(messages: readonly ThreadScrollMessage[]): number {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.role === "user") return index;
  }
  return -1;
}

/**
 * A mounted row's real height. A message that renders no row (an operational
 * marker the reader never sees, an assistant shell before its first content)
 * is genuinely 0px and must measure as 0: booking its estimate instead inflates
 * the virtual layout past the real scroll height, and the last rows then sit
 * beyond the reachable bottom — the promoted turn's answer never mounts
 * (BUG-20260906-promoted-turn-missing-live). Only an element no layout engine
 * has sized (jsdom, a detached node) keeps the estimate.
 */
export function measureVirtualRow(
  element: HTMLDivElement,
  entry: ResizeObserverEntry | undefined,
  virtualizer: {
    options: { estimateSize: (index: number) => number };
    indexFromElement: (element: HTMLDivElement) => number;
  }
): number {
  const observedSize = entry?.borderBoxSize?.[0]?.blockSize;
  if (observedSize !== undefined) {
    return Math.round(observedSize);
  }

  if (element.offsetHeight > 0) {
    return element.offsetHeight;
  }

  if (element.isConnected && element.getClientRects().length > 0) {
    return 0;
  }

  return virtualizer.options.estimateSize(virtualizer.indexFromElement(element));
}

function observeViewportRect(
  virtualizer: Virtualizer<HTMLDivElement, HTMLDivElement>,
  publish: (rect: { width: number; height: number }) => void
): (() => void) | undefined {
  const viewport = virtualizer.scrollElement;
  if (!viewport) return undefined;
  const measure = () => {
    publish({
      width: viewport.clientWidth || INITIAL_VIEWPORT_RECT.width,
      height: viewport.clientHeight || INITIAL_VIEWPORT_RECT.height,
    });
  };
  measure();
  const observer = new ResizeObserver(measure);
  observer.observe(viewport);
  return () => observer.disconnect();
}

function pinToEnd(viewport: HTMLDivElement, store: ThreadScrollStore): void {
  viewport.scrollTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
  store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
}

/**
 * The transcript's one scroll owner (ADR-007), wired onto the TanStack
 * virtualizer: following pins to the live edge, the new-turn anchor holds the
 * sent bubble near the top against the measured composer overlap, and any
 * upward gesture hands the viewport to the reader until the 40px band or the
 * pill re-arms it. Older history and released pages restore the exact reading
 * position; a disclosure holds end-maintenance for two frames; a steer during
 * streaming snaps. The store may come from the thread (shared with the rows)
 * or be created here for a standalone viewport.
 */
export function useThreadScrollController(
  viewportRef: RefObject<HTMLDivElement | null>,
  contentRef: RefObject<HTMLDivElement | null>,
  messages: readonly ThreadScrollMessage[],
  includeLoadOlderControl: boolean
): ThreadScrollControllerResult {
  "use no memo";

  const messageCount = messages.length;
  const leadingItemCount = includeLoadOlderControl ? 1 : 0;
  const ownStore = useStore(threadScrollLogic);
  const store = useOptionalThreadScrollStore() ?? ownStore;
  // A navigation landing outlives the render that started it (rows for a just-
  // loaded page commit a frame later), so it reads the row list through a ref.
  const messagesRef = useRef(messages);
  useEffect(() => {
    messagesRef.current = messages;
  });
  const historyAnchor = useSelector(store, snapshot => snapshot.context.historyAnchor);
  const showScrollToBottom = useSelector(store, snapshot => snapshot.context.showScrollToBottom);
  const snapGeneration = useSelector(store, snapshot => snapshot.context.snapGeneration);
  const foldSettleFrames = useSelector(
    store,
    snapshot => snapshot.context.ownership.foldSettleFrames
  );
  // react-doctor-disable-next-line react-hooks-js/incompatible-library -- TanStack Virtual owns imperative callbacks; this hook is an explicit React Compiler boundary.
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: messageCount + leadingItemCount,
    getScrollElement: () => viewportRef.current,
    observeElementRect: observeViewportRect,
    estimateSize: virtualIndex => {
      if (includeLoadOlderControl && virtualIndex === 0) {
        return LOAD_OLDER_CONTROL_ESTIMATE;
      }
      return (
        estimateMessageSize(messages[virtualIndex - leadingItemCount]) ?? VIRTUAL_MESSAGE_ESTIMATE
      );
    },
    getItemKey: virtualIndex => {
      if (includeLoadOlderControl && virtualIndex === 0) {
        return LOAD_OLDER_CONTROL_KEY;
      }
      return messages[virtualIndex - leadingItemCount]?.id ?? virtualIndex;
    },
    measureElement: measureVirtualRow,
    initialRect: INITIAL_VIEWPORT_RECT,
    overscan: VIRTUAL_OVERSCAN,
    // React 19 rejects TanStack Virtual's default flushSync notification while
    // the virtualizer is already measuring from its layout effect.
    useFlushSync: false,
    anchorTo: "end",
    followOnAppend: false,
    scrollEndThreshold: FOLLOW_REARM_BAND_PX,
    scrollToFn: (offset, options, instance) => {
      const viewport = instance.scrollElement;
      if (!viewport) return;
      const scrollState = store.getSnapshot().context;
      const mode = threadScrollMode(scrollState);
      if (
        mode === "free-scrolling" &&
        scrollState.historyAnchor === null &&
        options.adjustments == null
      ) {
        return;
      }
      const target = offset + (options.adjustments ?? 0);
      const maxScroll = viewport.scrollHeight - viewport.clientHeight;
      const pinnedAtEnd = maxScroll - viewport.scrollTop <= FOLLOW_REARM_BAND_PX;
      if (mode === "following-end" && pinnedAtEnd && target < maxScroll) {
        return;
      }
      viewport.scrollTo({
        top: target,
        behavior: options.behavior === "smooth" ? "smooth" : "auto",
      });
    },
  });

  const scrollToEnd = () => {
    store.trigger.scrollToEndRequested();
    const viewport = viewportRef.current;
    if (viewport) {
      virtualizer.scrollToEnd();
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
    }
  };

  const readVisibleMessageId = () => {
    const viewport = viewportRef.current;
    if (!viewport) return null;
    return firstVisibleRow(viewport)?.dataset.messageId ?? null;
  };

  const readVisibleMessageIds = () => {
    const viewport = viewportRef.current;
    if (!viewport) return { first: null, last: null };
    return {
      first: firstVisibleRow(viewport)?.dataset.messageId ?? null,
      last: lastVisibleRow(viewport)?.dataset.messageId ?? null,
    };
  };

  // A navigation landing (find/trail): the reader owns the viewport from here
  // on; `landMessageRow` applies the offset directly until the row rests.
  const scrollToMessage = (
    messageId: string,
    offsetTop: number,
    locate?: (row: HTMLElement) => Range | null
  ) => {
    const viewport = viewportRef.current;
    if (!viewport) return Promise.resolve(false);
    store.trigger.navigationJumpRequested();
    return landMessageRow({
      estimateOffset: () => {
        const messageIndex = messagesRef.current.findIndex(message => message.id === messageId);
        if (messageIndex < 0) return null;
        return virtualizer.getOffsetForIndex(messageIndex + leadingItemCount, "start")?.[0] ?? null;
      },
      locate,
      messageId,
      offsetTop,
      onScrolled: scrollTop => store.trigger.programmaticScrollApplied({ scrollTop }),
      viewport,
    });
  };

  const captureHistoryAnchor = () => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const anchor = firstVisibleRow(viewport);
    const messageId = anchor?.dataset.messageId;
    if (!anchor || !messageId) return;
    store.trigger.historyPageRequested({
      messageId,
      offsetTop: anchor.getBoundingClientRect().top - viewport.getBoundingClientRect().top,
      previousCount: messageCount,
    });
  };

  // Exact reading position across a list change above the reader (older history
  // landing, far pages released): the anchored row returns to the same offset.
  useLayoutEffect(() => {
    if (!historyAnchor || messageCount === historyAnchor.previousCount) return undefined;
    const viewport = viewportRef.current;
    const messageIndex = messages.findIndex(message => message.id === historyAnchor.messageId);
    if (!viewport || messageIndex < 0) {
      store.trigger.historyAnchorApplied();
      return undefined;
    }

    let frame = 0;
    let stableFrames = 0;
    let animationFrame = 0;
    const alignAnchor = () => {
      frame += 1;
      const anchorRow = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")].find(
        row => row.dataset.messageId === historyAnchor.messageId
      );
      if (anchorRow) {
        const currentOffset =
          anchorRow.getBoundingClientRect().top - viewport.getBoundingClientRect().top;
        const correction = currentOffset - historyAnchor.offsetTop;
        if (Math.abs(correction) > 0.5) {
          viewport.scrollTop += correction;
          store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
          stableFrames = 0;
        } else {
          stableFrames += 1;
        }
      } else {
        virtualizer.scrollToIndex(messageIndex + leadingItemCount, { align: "start" });
        stableFrames = 0;
      }

      if (stableFrames >= 4 || frame >= 30) {
        store.trigger.historyAnchorApplied();
        return;
      }
      animationFrame = requestAnimationFrame(alignAnchor);
    };
    alignAnchor();
    return () => cancelAnimationFrame(animationFrame);
  }, [historyAnchor, leadingItemCount, messageCount, messages, store, viewportRef, virtualizer]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    const distanceFromBottom = () =>
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    const observeNavigation = (direction: "up" | "down") => {
      store.trigger.manualNavigationObserved({
        awayFromEnd: distanceFromBottom() > FOLLOW_REARM_BAND_PX,
        direction,
      });
    };
    let lastTouchY: number | null = null;
    const handleWheel = (event: WheelEvent) => {
      observeNavigation(event.deltaY < 0 ? "up" : "down");
    };
    const handleTouchStart = (event: TouchEvent) => {
      lastTouchY = event.touches[0]?.clientY ?? null;
    };
    const handleTouchMove = (event: TouchEvent) => {
      const y = event.touches[0]?.clientY ?? null;
      // A finger moving down drags the content down: the reader is heading up.
      const direction = lastTouchY !== null && y !== null && y > lastTouchY ? "up" : "down";
      lastTouchY = y;
      observeNavigation(direction);
    };
    // A pointer on the scrollbar is the reader taking the viewport.
    const handlePointerDown = () => observeNavigation("up");
    const handleScroll = () => {
      store.trigger.scrollObserved({
        atEnd: distanceFromBottom() <= FOLLOW_REARM_BAND_PX,
        scrollTop: viewport.scrollTop,
      });
    };
    viewport.addEventListener("wheel", handleWheel, { passive: true });
    viewport.addEventListener("touchstart", handleTouchStart, { passive: true });
    viewport.addEventListener("touchmove", handleTouchMove, { passive: true });
    viewport.addEventListener("pointerdown", handlePointerDown, { passive: true });
    viewport.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      viewport.removeEventListener("wheel", handleWheel);
      viewport.removeEventListener("touchstart", handleTouchStart);
      viewport.removeEventListener("touchmove", handleTouchMove);
      viewport.removeEventListener("pointerdown", handlePointerDown);
      viewport.removeEventListener("scroll", handleScroll);
    };
  }, [store, viewportRef]);

  // End-maintenance: streaming growth (content) and composer growth (viewport
  // shrink) keep the pin while following, and only then; a disclosure holds it
  // for two frames so opening a fold never yanks (ADR-007).
  useEffect(() => {
    const viewport = viewportRef.current;
    const content = contentRef.current;
    if (!viewport || !content) return undefined;
    const maintain = () => {
      if (!shouldMaintainEnd(store.getSnapshot().context.ownership)) return;
      pinToEnd(viewport, store);
    };
    const observer = new ResizeObserver(maintain);
    observer.observe(content);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, [contentRef, store, viewportRef]);

  // The composer's overlap with the scroller, measured from rects every time
  // either box changes; the new-turn anchor subtracts it.
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    const composer =
      viewport.parentElement?.querySelector<HTMLElement>(COMPOSER_SHELL_SELECTOR) ??
      viewport.closest("[data-thread-root]")?.querySelector<HTMLElement>(COMPOSER_SHELL_SELECTOR) ??
      null;
    const measure = () => {
      store.trigger.composerHeightObserved({
        overlayHeight: measuredComposerOverlay(
          viewport.getBoundingClientRect(),
          composer ? composer.getBoundingClientRect() : null
        ),
      });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(viewport);
    if (composer) observer.observe(composer);
    return () => observer.disconnect();
  }, [store, viewportRef]);

  // Two frames after a disclosure toggles, end-maintenance may resume.
  useEffect(() => {
    if (foldSettleFrames === 0) return undefined;
    const frame = requestAnimationFrame(() => store.trigger.frameElapsed());
    return () => cancelAnimationFrame(frame);
  }, [foldSettleFrames, store]);

  // A steer during streaming: instant return to the end, never animated.
  useLayoutEffect(() => {
    if (snapGeneration === 0) return;
    const viewport = viewportRef.current;
    if (viewport) pinToEnd(viewport, store);
    store.trigger.snapApplied();
  }, [snapGeneration, store, viewportRef]);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || messageCount === 0) return;
    const lastUserIndex = findLastUserIndex(messages);
    const lastUserId = lastUserIndex >= 0 ? (messages[lastUserIndex]?.id ?? null) : null;
    store.trigger.messagesCommitted({ count: messageCount, lastUserId, lastUserIndex });
    const scrollState = store.getSnapshot().context;
    const mode = threadScrollMode(scrollState);

    const anchorIndex = scrollState.anchorIndex;
    if (mode === "anchoring-new-turn" && anchorIndex !== null) {
      const viewportHeight = Math.max(
        viewport.clientHeight,
        virtualizer.scrollRect?.height ?? INITIAL_VIEWPORT_RECT.height
      );
      const metrics = getAnchoredTurnMetrics({
        state: createVirtualizerMeasurementState(virtualizer, viewportHeight),
        anchorIndex: anchorIndex + leadingItemCount,
        composerOverlayHeight: scrollState.composerOverlayHeight,
        anchorOffset: NEW_TURN_ANCHOR_OFFSET_PX,
      });
      if (!metrics) return;
      if (metrics.overflowsUsableViewport) {
        // The reply outgrew the reserved space: following takes over.
        store.trigger.anchorOutgrown();
        virtualizer.scrollToEnd();
        store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
        return;
      }
      virtualizer.scrollToOffset(Math.max(0, metrics.anchorTop - NEW_TURN_ANCHOR_OFFSET_PX));
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
      return;
    }

    if (mode === "following-end") {
      virtualizer.scrollToEnd();
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
    }
  }, [leadingItemCount, messageCount, messages, store, viewportRef, virtualizer]);

  return {
    captureHistoryAnchor,
    readVisibleMessageId,
    readVisibleMessageIds,
    scrollToMessage,
    leadingItemCount,
    measureVirtualElement: virtualizer.measureElement,
    showScrollToBottom,
    scrollToEnd,
    store,
    virtualItems: virtualizer.getVirtualItems(),
    virtualTotalSize: virtualizer.getTotalSize(),
  };
}
