import { createStoreLogic } from "@xstate/store";

import {
  consumeSnapToEnd,
  INITIAL_SCROLL_OWNERSHIP,
  type ScrollOwnershipState,
  scrollOwnershipTransition,
  type TimelineScrollMode,
} from "../timeline-scroll-anchoring";

/** A reading position captured before the list changes above it (older history, released pages). */
export interface ThreadHistoryAnchor {
  messageId: string;
  offsetTop: number;
  previousCount: number;
}

interface ThreadScrollState {
  anchorIndex: number | null;
  /** Measured overlap of the composer over the scroller (0 in the stacked layout), never assumed. */
  composerOverlayHeight: number;
  historyAnchor: ThreadHistoryAnchor | null;
  lastUserId: string | null;
  ownership: ScrollOwnershipState;
  previousCount: number;
  programmaticScrollTop: number | null;
  showScrollToBottom: boolean;
  /** Increments on every snap request so the controller applies each one exactly once. */
  snapGeneration: number;
}

type ThreadScrollEvents = {
  /** The anchored reply outgrew its reserved space: following takes over. */
  anchorOutgrown: Record<never, never>;
  composerHeightObserved: { overlayHeight: number };
  /** A fold/group/changed-files disclosure toggled: hold end-maintenance for two frames. */
  disclosureToggled: Record<never, never>;
  frameElapsed: Record<never, never>;
  historyAnchorApplied: Record<never, never>;
  historyPageRequested: ThreadHistoryAnchor;
  manualNavigationObserved: { awayFromEnd: boolean; direction: "up" | "down" };
  messagesCommitted: { count: number; lastUserId: string | null; lastUserIndex: number };
  /** Find/trail jump: the reader takes the viewport before the landing scroll (free mode). */
  navigationJumpRequested: Record<never, never>;
  programmaticScrollApplied: { scrollTop: number };
  scrollObserved: { atEnd: boolean; scrollTop: number };
  scrollToEndRequested: Record<never, never>;
  snapApplied: Record<never, never>;
  /** A steer was sent; while the turn streams the viewport snaps to the end (US-024.EC-3). */
  steerRequested: { streaming: boolean };
};

/** The mode as the viewport reads it; the reducer owns the transitions (ADR-007). */
export function threadScrollMode(state: Pick<ThreadScrollState, "ownership">): TimelineScrollMode {
  return state.ownership.mode;
}

export const threadScrollLogic = createStoreLogic<ThreadScrollState, ThreadScrollEvents>({
  context: {
    anchorIndex: null,
    composerOverlayHeight: 0,
    historyAnchor: null,
    lastUserId: null,
    ownership: INITIAL_SCROLL_OWNERSHIP,
    previousCount: 0,
    programmaticScrollTop: null,
    showScrollToBottom: false,
    snapGeneration: 0,
  },
  on: {
    anchorOutgrown: context => ({
      ...context,
      anchorIndex: null,
      ownership: scrollOwnershipTransition(context.ownership, { type: "anchor-outgrown" }),
    }),
    composerHeightObserved: (context, event) =>
      context.composerOverlayHeight === event.overlayHeight
        ? context
        : { ...context, composerOverlayHeight: event.overlayHeight },
    disclosureToggled: context => ({
      ...context,
      ownership: scrollOwnershipTransition(context.ownership, { type: "fold-toggled" }),
    }),
    frameElapsed: context => {
      const ownership = scrollOwnershipTransition(context.ownership, { type: "frame" });
      return ownership === context.ownership ? context : { ...context, ownership };
    },
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
      ownership: scrollOwnershipTransition(context.ownership, { type: "jump-to-latest" }),
      showScrollToBottom: false,
    }),
    manualNavigationObserved: (context, event) => {
      const ownership = scrollOwnershipTransition(context.ownership, {
        type: "user-scroll",
        direction: event.direction,
        atEnd: !event.awayFromEnd,
      });
      return {
        ...context,
        anchorIndex: ownership.mode === "anchoring-new-turn" ? context.anchorIndex : null,
        historyAnchor: null,
        ownership,
        programmaticScrollTop: null,
        showScrollToBottom: event.awayFromEnd,
      };
    },
    scrollObserved: (context, event) => {
      if (
        context.programmaticScrollTop !== null &&
        Math.abs(event.scrollTop - context.programmaticScrollTop) <= 1
      ) {
        return { ...context, programmaticScrollTop: null };
      }
      if (event.atEnd) {
        return {
          ...context,
          anchorIndex: null,
          ownership: scrollOwnershipTransition(context.ownership, { type: "reached-end" }),
          programmaticScrollTop: null,
          showScrollToBottom: false,
        };
      }
      // Away from the band with no gesture of ours: the reader owns the viewport.
      return {
        ...context,
        anchorIndex: null,
        ownership: scrollOwnershipTransition(context.ownership, {
          type: "user-scroll",
          direction: "up",
          atEnd: false,
        }),
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
        ownership: startsNewTurn
          ? scrollOwnershipTransition(context.ownership, { type: "new-turn" })
          : context.ownership,
        previousCount: event.count,
        showScrollToBottom: startsNewTurn ? false : context.showScrollToBottom,
      };
    },
    navigationJumpRequested: context => ({
      ...context,
      anchorIndex: null,
      historyAnchor: null,
      ownership: scrollOwnershipTransition(context.ownership, {
        type: "user-scroll",
        direction: "up",
        atEnd: false,
      }),
      programmaticScrollTop: null,
      showScrollToBottom: true,
    }),
    programmaticScrollApplied: (context, event) => ({
      ...context,
      programmaticScrollTop: event.scrollTop,
    }),
    snapApplied: context => ({
      ...context,
      ownership: consumeSnapToEnd(context.ownership),
    }),
    steerRequested: (context, event) => {
      const ownership = scrollOwnershipTransition(context.ownership, {
        type: "steer",
        streaming: event.streaming,
      });
      return {
        ...context,
        anchorIndex: ownership.mode === "anchoring-new-turn" ? context.anchorIndex : null,
        ownership,
        showScrollToBottom: ownership.mode === "following-end" ? false : context.showScrollToBottom,
        snapGeneration: ownership.snapToEnd ? context.snapGeneration + 1 : context.snapGeneration,
      };
    },
  },
});
