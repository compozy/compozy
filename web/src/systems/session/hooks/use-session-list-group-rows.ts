import { useEffect } from "react";

import type { SessionListGroupRowsStore } from "../contexts/session-list-group-selection-context-value";
import type { SessionListGroupRows } from "../lib/session-list-group-rows";

/**
 * Publishes what one group has on screen to the one selection that spans the
 * groups (a grouped list, or the board's columns), and withdraws it when the
 * group collapses or unmounts. Without a store there is no selection to join.
 */
export function usePublishSessionListGroupRows(
  store: SessionListGroupRowsStore | undefined,
  key: string,
  rows: SessionListGroupRows
) {
  const { rank, sessions, visibleOrder, pair, fetching = false } = rows;
  const order = visibleOrder.join("\n");
  const workspaceId = pair?.workspaceId;
  const profile = pair?.profile;
  useEffect(() => {
    store?.trigger.published({
      key,
      rows: {
        rank,
        sessions,
        visibleOrder: order === "" ? [] : order.split("\n"),
        pair: workspaceId !== undefined && profile !== undefined ? { workspaceId, profile } : null,
        fetching,
      },
    });
  }, [store, key, rank, sessions, order, workspaceId, profile, fetching]);
  useEffect(() => () => store?.trigger.removed({ key }), [store, key]);
}
