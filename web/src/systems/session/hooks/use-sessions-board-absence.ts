import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { sessionBoardAbsenceKey, sessionBoardAbsenceLogic } from "../lib/session-board-absence";
import { isSessionBoardQueryKey } from "../lib/session-catalog-options";
import type { SessionSelectionStore } from "./use-session-selection";

/** After a confirming re-read succeeds, the columns publish its cards before pruning trusts them. */
const BOARD_ABSENCE_SETTLE_MS = 250;

type SessionBoardAbsenceStore = ReturnType<typeof sessionBoardAbsenceLogic.createStore>;

/**
 * The board's absence state and the selection's `holdMissing` predicate: selected
 * cards missing from the reads are held while any read is in flight or the lanes
 * are off screen, and until `useConfirmSessionsBoardAbsence` confirms them gone.
 */
export function useSessionsBoardAbsence(settled: boolean) {
  const absence = useStore(sessionBoardAbsenceLogic);
  const { key, status } = useSelector(absence, snapshot => snapshot.context);
  return {
    absence,
    holdMissing: (missing: readonly string[]) =>
      !settled || status !== "confirmed" || sessionBoardAbsenceKey(missing) !== key,
  };
}

/**
 * Confirms that selected cards missing from every settled board read are gone.
 * The columns are separate reads, so a card changing column can be missing from
 * all of them for a moment even after each one settles. The board is re-read
 * once per absence; only a successful re-read that still lacks the cards lets
 * them leave the selection, and a failed one keeps them held.
 */
export function useConfirmSessionsBoardAbsence({
  absence,
  selection,
  presentIds,
  settled,
}: {
  absence: SessionBoardAbsenceStore;
  selection: SessionSelectionStore;
  /** Every session the board's reads currently publish. */
  presentIds: readonly string[];
  /** No read is in flight and the lanes are on screen. */
  settled: boolean;
}) {
  const queryClient = useQueryClient();
  const { status, episode } = useSelector(absence, snapshot => snapshot.context);
  const presentKey = presentIds.join("\n");
  const selectedKey = useSelector(selection, snapshot => snapshot.context.selectedIds.join("\n"));
  useEffect(() => {
    if (!settled) return;
    const present = new Set(presentKey === "" ? [] : presentKey.split("\n"));
    const missing = selection.getSnapshot().context.selectedIds.filter(id => !present.has(id));
    absence.trigger.observed({ key: sessionBoardAbsenceKey(missing) });
  }, [settled, presentKey, selectedKey, selection, absence]);
  useEffect(() => {
    if (status !== "rereading") return;
    queryClient
      .invalidateQueries(
        { predicate: query => isSessionBoardQueryKey(query.queryKey) },
        { throwOnError: true }
      )
      .then(
        () => absence.trigger.reread({ episode, ok: true }),
        () => absence.trigger.reread({ episode, ok: false })
      );
  }, [status, episode, absence, queryClient]);
  useEffect(() => {
    if (status !== "settling") return;
    const timer = setTimeout(() => absence.trigger.settled({ episode }), BOARD_ABSENCE_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [status, episode, absence]);
}
