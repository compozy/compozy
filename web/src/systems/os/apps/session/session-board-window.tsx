import { ListTree, Plus, SquareKanban } from "lucide-react";

import { Button, useTopbarSlot } from "@compozy/ui";

import { SessionsBoard, SessionViewToggle } from "@/systems/session";

import { useSessionBoardWindow } from "./hooks/use-session-board-window";

/**
 * The session window in Kanban mode (US-011): the body becomes the board for
 * the workspace / All projects scope and publishes its own head — "Sessions",
 * the view toggle, "Hide child sessions" and "New session". A card switches
 * the window back to List with that session open.
 */
export function SessionBoardWindow({
  windowId,
  sessionId,
}: {
  windowId: string;
  /** The session this window shows in List mode; its card reads as current. */
  sessionId: string | null;
}) {
  const { view, board, openForAgent, onOpen } = useSessionBoardWindow(windowId, sessionId);

  useTopbarSlot({
    glyph: <SquareKanban />,
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={view.setView} value={view.view} />,
    actions: (
      <>
        <Button
          aria-pressed={view.hideChildren}
          data-testid="sessions-board-hide-children"
          onClick={() => view.setHideChildren(!view.hideChildren)}
          size="sm"
          type="button"
          variant="quiet"
        >
          <ListTree aria-hidden="true" data-icon="inline-start" />
          Hide child sessions
        </Button>
        <Button
          data-testid="sessions-board-new-session"
          onClick={() => openForAgent("")}
          size="sm"
          type="button"
          variant="secondary"
        >
          <Plus aria-hidden="true" data-icon="inline-start" />
          New session
        </Button>
      </>
    ),
  });

  return <SessionsBoard board={board} currentSessionId={sessionId} onOpen={onOpen} />;
}
