import { use, useState, type MouseEvent } from "react";
import { useSelector } from "@xstate/store-react";

import type { SessionListGroupRowsStore } from "../contexts/session-list-group-selection-context-value";
import {
  SessionsBoardRowsStoreContext,
  SessionsBoardSelectionStoreContext,
} from "../contexts/sessions-board-selection-context-value";
import { sessionListGroupRank } from "../lib/session-list-group-rows";
import { sessionCheckboxGesture, sessionSelectionGesture } from "../lib/session-selection-gesture";
import type { SessionPayload } from "../types";
import { usePublishSessionListGroupRows } from "./use-session-list-group-rows";
import { sessionSelectionLogic, type SessionSelectionStore } from "./use-session-selection";

/*
 * The board selection as its views read it: a card, a column's select-all and a bottom bar
 * join the selection `useSessionsBoardSelection` owns through context, without reaching its
 * data layer, so the board views render from fixtures in prototypes and stories.
 */

/** Stands in for the store on a board rendered without a selection (prototypes, menus-off tests). */
const DETACHED_SELECTION = sessionSelectionLogic.createStore();

/**
 * Shift-click ranges inside the card's own column, in its on-screen order. An
 * anchor in another column starts a new range at the clicked card, as an anchor
 * outside the order does in the list.
 */
function toggleBoardRange(
  store: SessionSelectionStore,
  rows: SessionListGroupRowsStore | null,
  sessionId: string
) {
  const groups = rows ? Object.values(rows.getSnapshot().context.groups) : [];
  const order = groups.find(group => group.visibleOrder.includes(sessionId))?.visibleOrder;
  store.trigger.toggleRange({ toId: sessionId, visibleOrder: order ?? [sessionId] });
}

/**
 * One card's place in the board selection. The card reads only booleans about
 * itself, so a toggle re-renders the cards it changes and nothing else; `null`
 * on a board without a selection.
 */
export function useSessionBoardCardSelection(sessionId: string) {
  const store = use(SessionsBoardSelectionStoreContext);
  const rows = use(SessionsBoardRowsStoreContext);
  const source = store ?? DETACHED_SELECTION;
  const selected = useSelector(source, snapshot =>
    snapshot.context.selectedIds.includes(sessionId)
  );
  const mode = useSelector(source, snapshot => snapshot.context.selectedIds.length > 0);
  // Only a card inside a selection of two or more trades its menu for the selection's.
  const inMultiSelection = useSelector(
    source,
    snapshot =>
      snapshot.context.selectedIds.length > 1 && snapshot.context.selectedIds.includes(sessionId)
  );
  if (!store) return null;
  return {
    selected,
    mode,
    inMultiSelection,
    /** Applies a selection gesture; `false` when the click opens the session instead. */
    activate: (event: MouseEvent<HTMLElement>): boolean => {
      const gesture = sessionSelectionGesture(event, mode);
      if (gesture === "open") return false;
      if (gesture === "range") toggleBoardRange(store, rows, sessionId);
      else store.trigger.toggle({ id: sessionId });
      return true;
    },
    check: (event: Event) => {
      if (sessionCheckboxGesture(event) === "range") toggleBoardRange(store, rows, sessionId);
      else store.trigger.toggle({ id: sessionId });
    },
    /** Finder's rule: a context click outside the selection makes that card the selection. */
    retarget: () => {
      if (mode && !selected) store.trigger.selectOnly({ id: sessionId });
    },
  };
}

export type SessionsBoardGroupSelectionState = "all" | "some" | "none";

function groupSelectionState(
  selectedIds: readonly string[],
  ids: readonly string[]
): SessionsBoardGroupSelectionState {
  const selected = new Set(selectedIds);
  const count = ids.filter(id => selected.has(id)).length;
  if (count === 0) return "none";
  return count === ids.length ? "all" : "some";
}

/**
 * A column's or lane's select-all over the cards it shows: checking it adds
 * them to the selection, clearing it removes only them. `null` without a
 * selection or without cards.
 */
export function useSessionsBoardGroupSelection(ids: readonly string[]) {
  const store = use(SessionsBoardSelectionStoreContext);
  const source = store ?? DETACHED_SELECTION;
  const state = useSelector(source, snapshot =>
    groupSelectionState(snapshot.context.selectedIds, ids)
  );
  const mode = useSelector(source, snapshot => snapshot.context.selectedIds.length > 0);
  if (!store || ids.length === 0) return null;
  return {
    state,
    mode,
    toggle: () => {
      if (state === "all") store.trigger.deselect({ ids });
      else store.trigger.selectAll({ ids });
    },
  };
}

/** The board's bottom bars, in on-screen order after every column. */
export type SessionsBoardShelfId = "later" | "archive";

const SHELF_RANK: Record<SessionsBoardShelfId, number> = { later: 9998, archive: 9999 };

/**
 * A bottom bar's disclosure (Later, Archive) and its place in the board
 * selection. Its cards join the selection only while the bar shows them;
 * closing the bar drops them at once, as archiving a session drops its row from
 * the List's selection.
 */
export function useSessionsBoardShelfSelection(
  shelf: SessionsBoardShelfId,
  sessions: readonly SessionPayload[],
  fetching: boolean,
  failed: boolean
) {
  const [open, setOpen] = useState(false);
  const store = use(SessionsBoardSelectionStoreContext);
  const shown = open ? sessions : [];
  usePublishSessionListGroupRows(use(SessionsBoardRowsStoreContext) ?? undefined, shelf, {
    rank: sessionListGroupRank([SHELF_RANK[shelf]]),
    sessions: shown,
    visibleOrder: shown.map(session => session.id),
    // Folder moves resolve each selected session's own pair (`useSessionsBoardSelection`).
    pair: null,
    fetching: open && fetching,
    failed: open && failed,
  });
  return {
    open,
    toggle: () => {
      if (open) store?.trigger.deselect({ ids: sessions.map(session => session.id) });
      setOpen(!open);
    },
  };
}
