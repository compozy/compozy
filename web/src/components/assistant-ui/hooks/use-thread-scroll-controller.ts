import { type RefObject, useEffect, useLayoutEffect, useRef, useState } from "react";

import { nextScrollMode, type TimelineScrollMode } from "../timeline-scroll-anchoring";

const AT_END_THRESHOLD = 96;
const ANCHOR_OFFSET = 16;

interface ThreadScrollMessage {
  id?: string;
  role?: string;
}

export interface ThreadScrollControllerResult {
  showScrollToBottom: boolean;
  scrollToEnd: () => void;
}

function findLastUserIndex(messages: readonly ThreadScrollMessage[]): number {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.role === "user") return index;
  }
  return -1;
}

function messageRow(viewport: HTMLElement, index: number): HTMLElement | null {
  return viewport.querySelector<HTMLElement>(`[data-message-index="${index}"]`);
}

export function useThreadScrollController(
  viewportRef: RefObject<HTMLDivElement | null>,
  messages: readonly ThreadScrollMessage[]
): ThreadScrollControllerResult {
  const messageCount = messages.length;
  const modeRef = useRef<TimelineScrollMode>("following-end");
  const anchorIndexRef = useRef<number | null>(null);
  const lastUserIdRef = useRef<string | null>(null);
  const previousCountRef = useRef(0);
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);

  const scrollToEnd = () => {
    modeRef.current = nextScrollMode(modeRef.current, "scroll-to-end");
    anchorIndexRef.current = null;
    setShowScrollToBottom(false);
    const viewport = viewportRef.current;
    if (viewport) viewport.scrollTop = viewport.scrollHeight;
  };

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    const distanceFromBottom = () =>
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    const handleManualNavigation = () => {
      modeRef.current = nextScrollMode(modeRef.current, "manual-navigation");
      anchorIndexRef.current = null;
      setShowScrollToBottom(distanceFromBottom() > AT_END_THRESHOLD);
    };
    const handleScroll = () => {
      if (distanceFromBottom() <= AT_END_THRESHOLD) {
        modeRef.current = nextScrollMode(modeRef.current, "reached-end");
        setShowScrollToBottom(false);
      } else if (modeRef.current === "free-scrolling") {
        setShowScrollToBottom(true);
      }
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
  }, [viewportRef]);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || messageCount === 0) return;
    const lastUserIndex = findLastUserIndex(messages);
    const lastUserId = lastUserIndex >= 0 ? (messages[lastUserIndex]?.id ?? null) : null;
    const grew = messageCount > previousCountRef.current;
    if (
      previousCountRef.current > 0 &&
      grew &&
      lastUserId &&
      lastUserId !== lastUserIdRef.current
    ) {
      modeRef.current = nextScrollMode(modeRef.current, "new-turn");
      anchorIndexRef.current = lastUserIndex;
      setShowScrollToBottom(false);
    }
    lastUserIdRef.current = lastUserId;
    previousCountRef.current = messageCount;

    const anchorIndex = anchorIndexRef.current;
    if (modeRef.current === "anchoring-new-turn" && anchorIndex !== null) {
      const anchor = messageRow(viewport, anchorIndex);
      const last = messageRow(viewport, messageCount - 1);
      if (!anchor || !last) return;
      const anchorTop = anchor.offsetTop;
      const turnBottom = last.offsetTop + last.offsetHeight;
      const turnHeight = turnBottom - anchorTop;
      viewport.scrollTop =
        turnHeight > viewport.clientHeight
          ? Math.max(anchorTop - ANCHOR_OFFSET, turnBottom - viewport.clientHeight)
          : Math.max(0, anchorTop - ANCHOR_OFFSET);
      return;
    }

    if (modeRef.current === "following-end") {
      viewport.scrollTop = viewport.scrollHeight;
    }
  }, [messageCount, messages, viewportRef]);

  return { showScrollToBottom, scrollToEnd };
}
