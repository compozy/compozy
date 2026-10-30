import { useEffect, useEffectEvent, useRef, type PointerEvent } from "react";

import { notifyUser } from "@/lib/user-feedback";
import {
  SESSION_BOARD_DRAWER_OPEN,
  SESSION_BOARD_DRAWER_SEARCH_KEY,
  sessionHasUnsentInput,
  type SessionLifecycleActionHandlers,
} from "@/systems/session";

import { useOsShell } from "../../../hooks/use-os-shell";
import { adjacentShortcutItem } from "../../../lib/adjacent-shortcut-item";
import {
  claimSessionBoardDrawerCycle,
  type SessionBoardDrawerCycleDirection,
} from "../../../lib/session-board-drawer-cycle";
import type { SessionWindowSidebarTarget } from "../use-session-window-open-session";

/** A card on screen, read from the board's DOM in visual order (columns, lanes, Archive). */
interface BoardCard extends SessionWindowSidebarTarget {
  id: string;
  element: HTMLElement;
}

const CARD_SELECTOR = "[data-session-id]";

/** A folded column (Done) keeps its cards mounted but out of sight: they are not stepped to. */
const FOLDED_COLUMN_SELECTOR = '[data-collapsed="true"]';

/** Pointer targets that are controls, so a press on them is never "outside" the drawer. */
const INTERACTIVE_SELECTOR = [
  "button, a[href], input, textarea, select, label, summary, [contenteditable]",
  '[role="button"], [role="link"], [role="menuitem"], [role="option"], [role="tab"]',
  '[role="checkbox"], [role="switch"], [role="separator"], [tabindex]:not([tabindex="-1"])',
].join(", ");

/** A press that moves farther than this is a drag (a scroll, a selection), not a click. */
const CLICK_SLOP_PX = 4;

/** The cards on the board in visual order; `visible` drops those of a folded column. */
function boardCards(board: HTMLElement | null, { visible = true } = {}): BoardCard[] {
  if (!board) return [];
  return Array.from(board.querySelectorAll<HTMLElement>(CARD_SELECTOR)).flatMap(element => {
    const id = element.dataset.sessionId ?? "";
    if (!id || (visible && element.closest(FOLDED_COLUMN_SELECTOR))) return [];
    return [
      {
        id,
        sessionId: id,
        agentName: element.dataset.agentName ?? "",
        workspaceId: element.dataset.workspaceId ?? "",
        element,
      },
    ];
  });
}

/**
 * Where focus goes once the drawer's session leaves the board (archived, deleted): the card
 * below it in the same column, else the one above, else the next card on the board.
 */
function successorCard(board: HTMLElement | null, sessionId: string): BoardCard | null {
  const cards = boardCards(board);
  const current = cards.find(card => card.id === sessionId);
  if (!current) return null;
  const column = current.element.closest("section");
  const siblings = column ? cards.filter(card => column.contains(card.element)) : [];
  const index = siblings.indexOf(current);
  const neighbour = siblings[index + 1] ?? siblings[index - 1];
  if (neighbour) return neighbour;
  return cards.length > 1 ? adjacentShortcutItem(cards, sessionId, "next") : null;
}

/**
 * Where keyboard focus can land for a session: its card, or — while the card sits in a folded
 * column — that column's expand toggle, the nearest control on screen.
 */
function cardFocusTarget(board: HTMLElement | null, sessionId: string | null): HTMLElement | null {
  if (sessionId === null) return null;
  const card = boardCards(board, { visible: false }).find(item => item.id === sessionId)?.element;
  const folded = card?.closest(FOLDED_COLUMN_SELECTOR);
  if (!folded) return card ?? null;
  return folded.querySelector<HTMLElement>("button[aria-expanded]");
}

/**
 * The card Previous/Next lands on. A drawer session that is not on screen (a hidden child,
 * a folded column) starts from the board's ends, so no card is skipped.
 */
function steppedCard(
  board: HTMLElement | null,
  sessionId: string | null,
  direction: SessionBoardDrawerCycleDirection
): BoardCard | null {
  const cards = boardCards(board);
  if (cards.some(card => card.id === sessionId)) {
    return adjacentShortcutItem(cards, sessionId, direction);
  }
  return (direction === "next" ? cards[0] : cards.at(-1)) ?? null;
}

/** A press on a scrollbar lands on the scroller itself, past its client box. */
function onScrollbar(event: PointerEvent<HTMLElement>): boolean {
  const target = event.target;
  if (!(target instanceof HTMLElement)) return false;
  const bounds = target.getBoundingClientRect();
  return (
    event.clientX - bounds.left > target.clientWidth ||
    event.clientY - bounds.top > target.clientHeight
  );
}

/**
 * The board's session drawer (#1052) as window state. The drawer shows this window's own
 * session: its route carries the session and `drawer=open`, so a card swap, a close, Back and
 * Forward, a reload and a window restore all go through the window manager like any other
 * navigation. This hook owns the gestures around it — closing, stepping through cards,
 * presses on the empty board, and where focus lands — and reads the board only from the DOM,
 * so it never re-renders the board.
 */
export function useSessionBoardDrawer({
  windowId,
  sessionId,
  open,
  openSession,
}: {
  windowId: string;
  sessionId: string | null;
  open: boolean;
  openSession: (target: SessionWindowSidebarTarget) => void;
}) {
  const { coordinator, manager } = useOsShell();
  const boardRef = useRef<HTMLDivElement | null>(null);
  /** The card to land on after a lifecycle close; `null` lands on the drawer's own card. */
  const landingRef = useRef<string | null>(null);
  const pressRef = useRef<{ target: EventTarget; x: number; y: number } | null>(null);
  /**
   * The landing chosen when an archive or delete was requested: by the time the daemon
   * confirms it, a delete has already taken the card off the board.
   */
  const requestedLandingRef = useRef<{ sessionId: string; landing: string | null } | null>(null);
  /** The session the drawer shows, kept after it closes: a delete closes it before it confirms. */
  const shownRef = useRef<string | null>(null);
  /** Whether the drawer is open now; a removal confirms asynchronously, after renders moved on. */
  const openRef = useRef(open);

  const cardElement = (id: string | null) => cardFocusTarget(boardRef.current, id);

  const setOpen = (next: boolean) => {
    const win = manager.getState().windows[windowId];
    if (!win?.instanceKey) return;
    const search = Object.fromEntries(
      Object.entries(win.route.search).filter(([key]) => key !== SESSION_BOARD_DRAWER_SEARCH_KEY)
    );
    void coordinator.userRetarget(windowId, {
      app: "session",
      instanceKey: win.instanceKey,
      route: {
        pathname: win.route.pathname,
        search: next
          ? { ...search, [SESSION_BOARD_DRAWER_SEARCH_KEY]: SESSION_BOARD_DRAWER_OPEN }
          : search,
      },
    });
  };

  const close = () => setOpen(false);

  const step = (direction: SessionBoardDrawerCycleDirection) => {
    const target = steppedCard(boardRef.current, sessionId, direction);
    if (!target) {
      notifyUser({ message: "No other visible session.", tone: "info" });
      return;
    }
    target.element.scrollIntoView({ block: "nearest", inline: "nearest" });
    openSession(target);
  };

  // While the drawer is open, the shell's Previous/Next session shortcut steps the board.
  const cycle = useEffectEvent(step);
  useEffect(() => {
    if (!open) return undefined;
    return claimSessionBoardDrawerCycle(windowId, direction => cycle(direction));
  }, [open, windowId]);

  // A lifecycle close (archive, delete) can land after the session's own menus and dialogs
  // settle, so focus moves explicitly once the drawer is gone.
  const landFocus = () => {
    const landing = landingRef.current;
    if (landing === null) return;
    landingRef.current = null;
    requestAnimationFrame(() => cardElement(landing)?.focus());
  };
  const landFocusOnClose = useEffectEvent(landFocus);
  useEffect(() => {
    openRef.current = open;
    if (open) shownRef.current = sessionId;
    else landFocusOnClose();
  }, [open, sessionId]);

  return {
    boardRef,
    close,
    step,
    /** Focus on close: the card a lifecycle close chose, else the card of the drawer's session. */
    returnFocus: () => cardElement(landingRef.current ?? sessionId),
    /** Archive or delete of the drawer's session, from the drawer or a card: close and land below. */
    onSessionRemoved: (session: { id: string }, action: "archive" | "delete") => {
      if (session.id !== shownRef.current) return;
      const requested = requestedLandingRef.current;
      requestedLandingRef.current = null;
      landingRef.current =
        requested?.sessionId === session.id
          ? requested.landing
          : (successorCard(boardRef.current, session.id)?.id ?? null);
      // Archive closes the drawer here; a delete retires this window's session, which closes
      // the drawer with it — possibly already, in which case focus moves now.
      if (!openRef.current) landFocus();
      else if (action === "archive") close();
    },
    /** The card verbs, remembering where focus lands should they remove the drawer's session. */
    withLanding: (actions: SessionLifecycleActionHandlers): SessionLifecycleActionHandlers => {
      const remember = (session: { id: string }) => {
        if (session.id !== sessionId) return;
        requestedLandingRef.current = {
          sessionId: session.id,
          landing: successorCard(boardRef.current, session.id)?.id ?? null,
        };
      };
      return {
        ...actions,
        onArchive: session => {
          remember(session);
          actions.onArchive(session);
        },
        onDelete: session => {
          remember(session);
          actions.onDelete(session);
        },
      };
    },
    /**
     * A click on the empty board closes the drawer — not a drag that ends there, not a press
     * on a card or control, and never while the composer holds unsent input.
     */
    boardPointerHandlers: {
      onPointerDown: (event: PointerEvent<HTMLElement>) => {
        pressRef.current = null;
        if (!open || event.button !== 0 || onScrollbar(event)) return;
        const control =
          event.target instanceof Element ? event.target.closest(INTERACTIVE_SELECTOR) : null;
        if (control && event.currentTarget.contains(control)) return;
        pressRef.current = { target: event.target, x: event.clientX, y: event.clientY };
      },
      onPointerUp: (event: PointerEvent<HTMLElement>) => {
        const press = pressRef.current;
        pressRef.current = null;
        if (!press || press.target !== event.target || sessionId === null) return;
        if (Math.hypot(event.clientX - press.x, event.clientY - press.y) > CLICK_SLOP_PX) return;
        if (sessionHasUnsentInput(sessionId)) return;
        close();
      },
    },
  };
}
