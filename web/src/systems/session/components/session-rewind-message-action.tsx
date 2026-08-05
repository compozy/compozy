import { RotateCcw } from "lucide-react";

import { Button, ConfirmDialog } from "@compozy/ui";
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
    <>
      <Button
        aria-label="Rewind to here"
        data-testid="user-message-rewind"
        disabled={action.busy}
        onClick={action.trigger}
        size="xs"
        type="button"
        variant="ghost"
      >
        <RotateCcw aria-hidden="true" className="size-3" />
        Rewind to here
      </Button>
      <ConfirmDialog
        cancelLabel="Cancel"
        confirmButtonProps={{ "data-testid": "session-rewind-confirm" }}
        confirmLabel="Rewind to here"
        contentProps={{ "data-testid": "session-rewind-dialog" }}
        description="Messages from this point onward will be removed from the active conversation. Files, tool effects, network actions, and saved memory will not be undone."
        isPending={action.isPending}
        onConfirm={action.confirm}
        onOpenChange={action.setOpen}
        open={action.open}
        title="Rewind this session?"
        tone="danger"
      />
    </>
  );
}
