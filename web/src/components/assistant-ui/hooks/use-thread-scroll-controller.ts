import { type VirtualItem, type Virtualizer, useVirtualizer } from "@tanstack/react-virtual";
import { type RefObject, useEffect, useLayoutEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import {
  createVirtualizerMeasurementState,
  getAnchoredTurnMetrics,
} from "../timeline-scroll-anchoring";
import { estimateMessageSize, VIRTUAL_MESSAGE_ESTIMATE } from "../timeline-row-estimates";
import { threadScrollLogic } from "./thread-scroll-store";

const AT_END_THRESHOLD = 96;
const ANCHOR_OFFSET = 16;
const LOAD_OLDER_CONTROL_ESTIMATE = 48;
const VIRTUAL_OVERSCAN = 6;
const INITIAL_VIEWPORT_RECT = { width: 1_024, height: 640 };
const LOAD_OLDER_CONTROL_KEY = "session-load-older-control";

interface ThreadScrollMessage {
  id?: string;
  role?: string;
}

export interface ThreadScrollControllerResult {
  captureHistoryAnchor: () => void;
  leadingItemCount: number;
  measureVirtualElement: (element: HTMLDivElement | null) => void;
  showScrollToBottom: boolean;
  scrollToEnd: () => void;
  virtualItems: VirtualItem[];
  virtualTotalSize: number;
}

function findLastUserIndex(messages: readonly ThreadScrollMessage[]): number {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.role === "user") return index;
  }
  return -1;
}

function measureVirtualRow(
  element: HTMLDivElement,
  entry: ResizeObserverEntry | undefined,
  virtualizer: Virtualizer<HTMLDivElement, HTMLDivElement>
): number {
  const observedSize = entry?.borderBoxSize?.[0]?.blockSize;
  if (observedSize !== undefined && observedSize > 0) {
    return Math.round(observedSize);
  }

  if (element.offsetHeight > 0) {
    return element.offsetHeight;
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

export function useThreadScrollController(
  viewportRef: RefObject<HTMLDivElement | null>,
  contentRef: RefObject<HTMLDivElement | null>,
  messages: readonly ThreadScrollMessage[],
  includeLoadOlderControl: boolean
): ThreadScrollControllerResult {
  "use no memo";

  const messageCount = messages.length;
  const leadingItemCount = includeLoadOlderControl ? 1 : 0;
  const store = useStore(threadScrollLogic);
  const historyAnchor = useSelector(store, snapshot => snapshot.context.historyAnchor);
  const showScrollToBottom = useSelector(store, snapshot => snapshot.context.showScrollToBottom);
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
    scrollEndThreshold: AT_END_THRESHOLD,
    scrollToFn: (offset, options, instance) => {
      const viewport = instance.scrollElement;
      if (!viewport) return;
      const scrollState = store.getSnapshot().context;
      if (
        scrollState.mode === "free-scrolling" &&
        scrollState.historyAnchor === null &&
        options.adjustments == null
      ) {
        return;
      }
      const target = offset + (options.adjustments ?? 0);
      const maxScroll = viewport.scrollHeight - viewport.clientHeight;
      const pinnedAtEnd = maxScroll - viewport.scrollTop <= AT_END_THRESHOLD;
      if (
        store.getSnapshot().context.mode === "following-end" &&
        pinnedAtEnd &&
        target < maxScroll
      ) {
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

  const captureHistoryAnchor = () => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const viewportTop = viewport.getBoundingClientRect().top;
    const rows = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")];
    const anchor = rows.find(row => row.getBoundingClientRect().bottom >= viewportTop) ?? rows[0];
    const messageId = anchor?.dataset.messageId;
    if (!anchor || !messageId) return;
    store.trigger.historyPageRequested({
      messageId,
      offsetTop: anchor.getBoundingClientRect().top - viewportTop,
      previousCount: messageCount,
    });
  };

  useLayoutEffect(() => {
    if (!historyAnchor || messageCount <= historyAnchor.previousCount) return undefined;
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
    const handleManualNavigation = () => {
      store.trigger.manualNavigationObserved({
        awayFromEnd: distanceFromBottom() > AT_END_THRESHOLD,
      });
    };
    const handleScroll = () => {
      store.trigger.scrollObserved({
        atEnd: distanceFromBottom() <= AT_END_THRESHOLD,
        scrollTop: viewport.scrollTop,
      });
    };
    viewport.addEventListener("wheel", handleManualNavigation, { passive: true });
    viewport.addEventListener("touchmove", handleManualNavigation, { passive: true });
    viewport.addEventListener("pointerdown", handleManualNavigation, { passive: true });
    viewport.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      viewport.removeEventListener("wheel", handleManualNavigation);
      viewport.removeEventListener("touchmove", handleManualNavigation);
      viewport.removeEventListener("pointerdown", handleManualNavigation);
      viewport.removeEventListener("scroll", handleScroll);
    };
  }, [store, viewportRef]);

  useEffect(() => {
    const viewport = viewportRef.current;
    const content = contentRef.current;
    if (!viewport || !content) return undefined;
    const observer = new ResizeObserver(() => {
      if (store.getSnapshot().context.mode !== "following-end") return;
      viewport.scrollTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
    });
    observer.observe(content);
    return () => observer.disconnect();
  }, [contentRef, store, viewportRef]);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || messageCount === 0) return;
    const lastUserIndex = findLastUserIndex(messages);
    const lastUserId = lastUserIndex >= 0 ? (messages[lastUserIndex]?.id ?? null) : null;
    store.trigger.messagesCommitted({ count: messageCount, lastUserId, lastUserIndex });
    const scrollState = store.getSnapshot().context;

    const anchorIndex = scrollState.anchorIndex;
    if (scrollState.mode === "anchoring-new-turn" && anchorIndex !== null) {
      const viewportHeight = Math.max(
        viewport.clientHeight,
        virtualizer.scrollRect?.height ?? INITIAL_VIEWPORT_RECT.height
      );
      const metrics = getAnchoredTurnMetrics({
        state: createVirtualizerMeasurementState(virtualizer, viewportHeight),
        anchorIndex: anchorIndex + leadingItemCount,
        composerOverlayHeight: 0,
        anchorOffset: ANCHOR_OFFSET,
      });
      if (!metrics) return;
      const target = metrics.overflowsUsableViewport
        ? Math.max(metrics.anchorTop - ANCHOR_OFFSET, metrics.targetScrollToRevealEnd)
        : Math.max(0, metrics.anchorTop - ANCHOR_OFFSET);
      virtualizer.scrollToOffset(target);
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
      return;
    }

    if (scrollState.mode === "following-end") {
      virtualizer.scrollToEnd();
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
    }
  }, [leadingItemCount, messageCount, messages, store, viewportRef, virtualizer]);

  return {
    captureHistoryAnchor,
    leadingItemCount,
    measureVirtualElement: virtualizer.measureElement,
    showScrollToBottom,
    scrollToEnd,
    virtualItems: virtualizer.getVirtualItems(),
    virtualTotalSize: virtualizer.getTotalSize(),
  };
}
