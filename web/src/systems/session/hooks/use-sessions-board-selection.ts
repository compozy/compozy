import { useSelector, useStore } from "@xstate/store-react";

import { sessionListGroupedRows, sessionListGroupRowsLogic } from "../lib/session-list-group-rows";
import type { SessionListPair } from "../lib/session-list-groups";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { useSessionListBulkSelection } from "./use-session-list-bulk-selection";
import {
  useConfirmSessionsBoardAbsence,
  useSessionsBoardAbsence,
} from "./use-sessions-board-absence";

/** A session's own folder pair; User-scope sessions have no project folders. */
function ownerPair(session: SessionPayload): SessionListPair | null {
  return session.workspace_id
    ? { workspaceId: session.workspace_id, profile: session.profile_name }
    : null;
}

/**
 * The board's one bulk selection: the session list's selection, verbs, counts,
 * shortcuts and folder moves, fed by the cards each column and the open Later and
 * Archive bars publish, in on-screen order (lane, then column, then the bars).
 *
 * A selected card missing from every read is held until a re-read confirms it
 * gone (`useSessionsBoardAbsence`), so a card changing column keeps its place.
 * Move to folder resolves each selected session's own (project,
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
  const rows = useStore(sessionListGroupRowsLogic);
  const published = sessionListGroupedRows(useSelector(rows, snapshot => snapshot.context.groups));
  const owners = new Map(published.sessions.map(session => [session.id, ownerPair(session)]));
  const settled = !published.fetching && !paused;
  const { absence, holdMissing } = useSessionsBoardAbsence(settled);
  const bulk = useSessionListBulkSelection({
    selectionKey,
    shelf: "active",
    sessions: published.sessions,
    visibleOrder: published.visibleOrder,
    sessionActions,
    folders: { pairOf: sessionId => owners.get(sessionId) ?? null },
    holdMissing,
  });
  useConfirmSessionsBoardAbsence({
    absence,
    selection: bulk.store,
    presentIds: published.sessions.map(session => session.id),
    settled,
    failed: published.failed,
  });
  return { bulk, rows };
}
