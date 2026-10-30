import { use, useEffect, useRef, useState, type MouseEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import type { SessionListGroupRowsStore } from "../contexts/session-list-group-selection-context-value";
import {
  SessionsBoardRowsStoreContext,
  SessionsBoardSelectionStoreContext,
} from "../contexts/sessions-board-selection-context-value";
import { isSessionBoardQueryKey } from "../lib/session-catalog-options";
import { sessionListGroupedRows, sessionListGroupRowsLogic } from "../lib/session-list-group-rows";
import type { SessionListPair } from "../lib/session-list-groups";
import { sessionCheckboxGesture, sessionSelectionGesture } from "../lib/session-selection-gesture";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { useSessionListBulkSelection } from "./use-session-list-bulk-selection";
import { sessionSelectionLogic, type SessionSelectionStore } from "./use-session-selection";

/** After a confirming re-read resolves, the columns publish its cards before pruning trusts them. */
const BOARD_ABSENCE_SETTLE_MS = 250;

/** A session's own folder pair; User-scope sessions have no project folders. */
function ownerPair(session: SessionPayload): SessionListPair | null {
  return session.workspace_id
    ? { workspaceId: session.workspace_id, profile: session.profile_name }
    : null;
}

/**
 * The board's one bulk selection: the session list's selection, verbs, counts,
 * shortcuts and folder moves, fed by the cards each column and the open Archive
 * bar publish, in on-screen order (lane, then column, then the Archive bar).
 *
 * The columns are separate reads, so a card changing column can be missing from
 * all of them for a moment even after every read settles. A selected card that
 * goes missing is kept while reads are in flight or the lanes are not on screen,
 * then the board is re-read once; only a card still missing after that leaves the
 * selection. Move to folder resolves each selected session's own (project,
 * profile) pair, so matching owners move and mixed owners get the list's reason.
 */
export function useSessionsBoardSelection({
  selectionKey,
  sessionActions,
  paused = false,
}: {
  /** The board's identity: a new breadth, lens or grouping is a new selection. */
  selectionKey: string;
  sessionActions: SessionLifecycleActionHandlers;
  /** The lanes are loading or failed: their cards are off screen, not gone. */
  paused?: boolean;
}) {
  const queryClient = useQueryClient();
  const rows = useStore(sessionListGroupRowsLogic);
  const published = sessionListGroupedRows(useSelector(rows, snapshot => snapshot.context.groups));
  const owners = new Map(published.sessions.map(session => [session.id, ownerPair(session)]));
  // The absences a completed re-read confirmed; any other absence is held.
  const [confirmed, setConfirmed] = useState<string | null>(null);
  const settled = !published.fetching && !paused;
  const bulk = useSessionListBulkSelection({
    selectionKey,
    archived: false,
    sessions: published.sessions,
    visibleOrder: published.visibleOrder,
    sessionActions,
    folders: { pairOf: sessionId => owners.get(sessionId) ?? null },
    holdMissing: missing => !settled || absenceKey(missing) !== confirmed,
  });
  const { store } = bulk;
  const presentKey = published.sessions.map(session => session.id).join("\n");
  const requested = useRef<string | null>(null);
  // The absence whose re-read has resolved; it is confirmed once the columns publish that read.
  const [reread, setReread] = useState<string | null>(null);
  useEffect(() => {
    if (!settled) return;
    const present = new Set(presentKey === "" ? [] : presentKey.split("\n"));
    const missing = store.getSnapshot().context.selectedIds.filter(id => !present.has(id));
    if (missing.length === 0) {
      requested.current = null;
      return;
    }
    const key = absenceKey(missing);
    // One re-read per absence: the re-read itself unsettles the board, which must not ask again.
    if (key === requested.current) return;
    requested.current = key;
    void queryClient
      .invalidateQueries({ predicate: query => isSessionBoardQueryKey(query.queryKey) })
      .then(() => setReread(key));
  }, [settled, presentKey, store, queryClient]);
  useEffect(() => {
    if (reread === null) return;
    const timer = setTimeout(() => setConfirmed(reread), BOARD_ABSENCE_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [reread]);
  return { bulk, rows };
}

function absenceKey(ids: readonly string[]): string {
  return [...ids].sort().join("\n");
}

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
