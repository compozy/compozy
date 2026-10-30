/**
 * Inert row wiring for gallery prototypes and component tests that render a session row or card
 * view without the live hooks: lifecycle verbs that do nothing and an entry preload that never
 * fires. Stories pass Storybook's `fn` to log the verbs instead.
 */
import type { useSessionEntryPreload } from "../hooks/use-session-entry-preload";
import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";

type Handler = () => () => void;

const inert: Handler = () => () => undefined;

export function sessionLifecycleActionsFixture(
  handler: Handler = inert
): SessionLifecycleActionHandlers {
  return {
    pendingAction: null,
    pendingSessionId: null,
    onStop: handler(),
    onRename: handler(),
    onArchive: handler(),
    onUnarchive: handler(),
    onDelete: handler(),
  };
}

const idle = () => undefined;

export const SESSION_ENTRY_PRELOAD_FIXTURE: ReturnType<typeof useSessionEntryPreload> = {
  onPointerEnter: idle,
  onPointerLeave: idle,
  onFocus: idle,
  onBlur: idle,
  onTouchStart: idle,
};
