import { Button, StateGlyph } from "@compozy/ui";

import type { SideMainStatus } from "../../lib/session-side";

/**
 * The main session's state in the side panel header (US-005). Absent while that state is unknown.
 * Activating it hands focus back to the main session: its pending prompt, else its composer.
 */
export function SessionSideMainStatus({
  status,
  onActivate,
}: {
  status: SideMainStatus | null;
  onActivate?: () => void;
}) {
  return (
    // A polite live region that is always mounted, so a state change is announced (Accessibility).
    <span aria-live="polite" className="flex min-w-0 shrink-0" data-testid="side-chat-main-status">
      {status ? (
        <Button
          data-status={status.kind}
          onClick={onActivate}
          size="xs"
          type="button"
          variant="quiet"
        >
          <StateGlyph aria-hidden="true" size="sm" state={status.glyph} />
          {status.label}
        </Button>
      ) : null}
    </span>
  );
}
