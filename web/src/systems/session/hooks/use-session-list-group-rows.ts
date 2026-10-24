import { use, useEffect } from "react";

import { SessionListGroupSelectionContext } from "../contexts/session-list-group-selection-context-value";
import type { SessionListGroupRows } from "../lib/session-list-group-rows";

/**
 * Publishes what one group has on screen to the grouped list's one selection,
 * and withdraws it when the group collapses or unmounts.
 */
export function usePublishSessionListGroupRows(key: string, rows: SessionListGroupRows) {
  const store = use(SessionListGroupSelectionContext)?.rows;
  const { rank, sessions, visibleOrder, pair } = rows;
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
      },
    });
  }, [store, key, rank, sessions, order, workspaceId, profile]);
  useEffect(() => () => store?.trigger.removed({ key }), [store, key]);
}
