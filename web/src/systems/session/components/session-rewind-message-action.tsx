import { Pencil } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";
import { useSessionRewindMessageAction } from "../hooks/use-session-rewind-message-action";

/**
 * A session-only action. It is rendered only for a user message that is
 * present in the daemon-owned transcript cache, never for an optimistic tail.
 */
export function SessionRewindMessageAction() {
  const action = useSessionRewindMessageAction();

  if (!action.available) {
    return null;
  }

  return (
    <Button
      aria-label="Edit message"
      data-testid="user-message-rewind"
      disabled={action.busy}
      onClick={action.trigger}
      size="xs"
      type="button"
      variant="quiet"
    >
      <Pencil aria-hidden="true" />
      Edit message
    </Button>
  );
}
