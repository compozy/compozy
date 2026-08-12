import { createStoreLogic } from "@xstate/store";

import { nextScrollMode, type TimelineScrollMode } from "../timeline-scroll-anchoring";

interface ThreadScrollState {
  anchorIndex: number | null;
  historyAnchor: {
    messageId: string;
    offsetTop: number;
    previousCount: number;
  } | null;
  lastUserId: string | null;
  mode: TimelineScrollMode;
  previousCount: number;
  programmaticScrollTop: number | null;
  showScrollToBottom: boolean;
}

type ThreadScrollEvents = {
  historyAnchorApplied: Record<never, never>;
  historyPageRequested: {
    messageId: string;
    offsetTop: number;
    previousCount: number;
  };
  manualNavigationObserved: { awayFromEnd: boolean };
  messagesCommitted: { count: number; lastUserId: string | null; lastUserIndex: number };
  programmaticScrollApplied: { scrollTop: number };
  scrollObserved: { atEnd: boolean; scrollTop: number };
  scrollToEndRequested: Record<never, never>;
};

export const threadScrollLogic = createStoreLogic<ThreadScrollState, ThreadScrollEvents>({
  context: {
    anchorIndex: null,
    historyAnchor: null,
    lastUserId: null,
    mode: "following-end",
    previousCount: 0,
    programmaticScrollTop: null,
    showScrollToBottom: false,
  },
  on: {
    historyPageRequested: (context, event) => ({
      ...context,
      historyAnchor: event,
    }),
    historyAnchorApplied: context => ({
      ...context,
      historyAnchor: null,
    }),
    scrollToEndRequested: context => ({
      ...context,
      anchorIndex: null,
      mode: nextScrollMode(context.mode, "scroll-to-end"),
      showScrollToBottom: false,
    }),
    manualNavigationObserved: (context, event) => ({
      ...context,
      anchorIndex: null,
      historyAnchor: null,
      mode: nextScrollMode(context.mode, "manual-navigation"),
      programmaticScrollTop: null,
      showScrollToBottom: event.awayFromEnd,
    }),
    scrollObserved: (context, event) => {
      if (
        context.programmaticScrollTop !== null &&
        Math.abs(event.scrollTop - context.programmaticScrollTop) <= 1
      ) {
        return { ...context, programmaticScrollTop: null };
      }
      return event.atEnd
        ? {
            ...context,
            anchorIndex: null,
            mode: nextScrollMode(context.mode, "reached-end"),
            programmaticScrollTop: null,
            showScrollToBottom: false,
          }
        : {
            ...context,
            anchorIndex: null,
            mode: nextScrollMode(context.mode, "manual-navigation"),
            programmaticScrollTop: null,
            showScrollToBottom: true,
          };
    },
    messagesCommitted: (context, event) => {
      const grew = event.count > context.previousCount;
      const startsNewTurn =
        context.previousCount > 0 &&
        grew &&
        event.lastUserId !== null &&
        event.lastUserId !== context.lastUserId;
      return {
        ...context,
        anchorIndex: startsNewTurn ? event.lastUserIndex : context.anchorIndex,
        lastUserId: event.lastUserId,
        mode: startsNewTurn ? nextScrollMode(context.mode, "new-turn") : context.mode,
        previousCount: event.count,
        showScrollToBottom: startsNewTurn ? false : context.showScrollToBottom,
      };
    },
    programmaticScrollApplied: (context, event) => ({
      ...context,
      programmaticScrollTop: event.scrollTop,
    }),
  },
});
