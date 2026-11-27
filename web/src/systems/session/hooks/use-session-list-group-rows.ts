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
  const { rank, computerId, sessions, visibleOrder, pair, fetching = false, failed = false } = rows;
  const order = visibleOrder.join("\n");
  const workspaceId = pair?.workspaceId;
  const profile = pair?.profile;
  const pairComputerId = pair?.computerId;
  useEffect(() => {
    store?.trigger.published({
      key,
      rows: {
        rank,
        computerId,
        sessions,
        visibleOrder: order === "" ? [] : order.split("\n"),
        pair:
          workspaceId !== undefined && profile !== undefined
            ? {
                workspaceId,
                profile,
                ...(pairComputerId ? { computerId: pairComputerId } : {}),
              }
            : null,
        fetching,
        failed,
      },
    });
  }, [
    store,
    key,
    rank,
    computerId,
    sessions,
    order,
    workspaceId,
    profile,
    pairComputerId,
    fetching,
    failed,
  ]);
  useEffect(() => () => store?.trigger.removed({ key }), [store, key]);
}
