import { Plus } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

export interface OsNewSessionButtonProps {
  /** The destination has settled: a project, User, or All projects asking inline. */
  canStart: boolean;
  /** Creation is already in flight. */
  busy?: boolean;
  onNewSession: () => void;
}

/**
 * The window-head New session action (secondary, never the inverted primary:
 * heads carry no page CTA). It rests only while the destination settles or a
 * create is in flight.
 */
export function OsNewSessionButton({
  canStart,
  busy = false,
  onNewSession,
}: OsNewSessionButtonProps) {
  return (
    <Button disabled={!canStart || busy} onClick={onNewSession} variant="secondary" size="sm">
      <Plus aria-hidden="true" data-icon="inline-start" />
      New session
    </Button>
  );
}
