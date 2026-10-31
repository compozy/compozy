import { GitFork } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

import { useSessionForkMessageAction } from "../hooks/use-session-fork-message-action";

/**
 * Rewind's twin on a user message: same row, size and gates, opposite
 * consequence — it starts a second session through this turn and leaves this
 * one alone. Absent where rewind is absent; disabled while the thread moves.
 */
export function SessionForkMessageAction() {
  const action = useSessionForkMessageAction();

  if (!action.available) {
    return null;
  }

  return (
    <Button
      aria-label="Fork from here"
      data-testid="user-message-fork"
      disabled={action.busy}
      onClick={action.trigger}
      size="xs"
      type="button"
      variant="quiet"
    >
      <GitFork aria-hidden="true" />
      Fork from here
    </Button>
  );
}
