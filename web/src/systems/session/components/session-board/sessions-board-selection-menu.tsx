import { use } from "react";

import { SessionsBoardSelectionVerbsContext } from "../../contexts/sessions-board-selection-context-value";
import { SessionSelectionContextItems } from "../session-list/session-list-selection-menu";

/**
 * A selected card's "N sessions" menu: the list's selection verbs and counts,
 * read only while the menu is open so a toggle never re-renders the cards.
 */
export function SessionsBoardSelectionMenu({ testId }: { testId: string }) {
  const verbs = use(SessionsBoardSelectionVerbsContext);
  if (!verbs) return null;
  return <SessionSelectionContextItems testId={testId} verbs={verbs} />;
}
