import { GitPullRequest } from "lucide-react";

import { StateGlyph } from "@compozy/ui";

import type { SessionBoardColumnId } from "../../lib/session-board-contract";

/**
 * The mark ahead of a column's name: a pull request for In review, else the session state. Its
 * own leaf so the live column and the board's loading placeholder draw the same mark.
 */
export function SessionsBoardColumnGlyph({ id }: { id: SessionBoardColumnId }) {
  if (id === "in_review") {
    return <GitPullRequest aria-hidden="true" className="size-3.5 shrink-0 text-info" />;
  }
  const state = id === "needs_you" ? "attention" : id === "working" ? "running" : "done";
  return <StateGlyph state={state} />;
}
