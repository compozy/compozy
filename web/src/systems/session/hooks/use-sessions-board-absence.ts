import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { sessionBoardAbsenceKey, sessionBoardAbsenceLogic } from "../lib/session-board-absence";
import { isSessionBoardQueryKey } from "../lib/session-catalog-options";
import type { SessionSelectionStore } from "./use-session-selection";

/** After a confirming re-read succeeds, the columns publish its cards before pruning trusts them. */
const BOARD_ABSENCE_SETTLE_MS = 250;

/** A re-read that a failed board read left unconfirmed is tried again after this pause. */
const BOARD_ABSENCE_RETRY_MS = 15_000;

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
 * once per absence; only a re-read after which this board's reads all succeed
 * and still lack the cards lets them leave the selection. A failed read keeps
 * them held and retries after a pause.
 */
export function useConfirmSessionsBoardAbsence({
  absence,
  selection,
  presentIds,
  settled,
  failed,
}: {
  absence: SessionBoardAbsenceStore;
  selection: SessionSelectionStore;
  /** Every session the board's reads currently publish. */
  presentIds: readonly string[];
  /** No read is in flight and the lanes are on screen. */
  settled: boolean;
  /** One of this board's column or Archive reads has failed. */
  failed: boolean;
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
    void queryClient
      .invalidateQueries({ predicate: query => isSessionBoardQueryKey(query.queryKey) })
      .then(() => absence.trigger.reread({ episode }));
  }, [status, episode, absence, queryClient]);
  useEffect(() => {
    if (status !== "settling") return;
    // This board's own reads decide: a failed column or Archive read confirms nothing.
    const timer = setTimeout(
      () => absence.trigger.settled({ episode, ok: !failed }),
      BOARD_ABSENCE_SETTLE_MS
    );
    return () => clearTimeout(timer);
  }, [status, episode, failed, absence]);
  useEffect(() => {
    if (status !== "unconfirmed") return;
    const timer = setTimeout(() => absence.trigger.retry({ episode }), BOARD_ABSENCE_RETRY_MS);
    return () => clearTimeout(timer);
  }, [status, episode, absence]);
}
