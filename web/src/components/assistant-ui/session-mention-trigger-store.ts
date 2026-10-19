import { createStoreLogic } from "@xstate/store";

import type { SessionMentionTrigger } from "./session-mention-model";

interface SessionMentionTriggerState {
  /** The `@` occurrence before the caret, as the editor last reported it. */
  observed: SessionMentionTrigger | null;
  /** Offset of the occurrence the operator closed with Esc; it stays closed. */
  dismissedOffset: number | null;
}

type SessionMentionTriggerEvents = {
  observed: { trigger: SessionMentionTrigger | null };
  dismissed: Record<never, never>;
};

function sameTrigger(a: SessionMentionTrigger | null, b: SessionMentionTrigger | null): boolean {
  return a === b || (a !== null && b !== null && a.offset === b.offset && a.query === b.query);
}

/**
 * Picker interaction state for one composer. A dismissal belongs to one `@`
 * occurrence: it lasts while the caret stays in that token and clears as soon
 * as the token ends (a space, a moved caret, a deleted `@`), so a new `@`
 * opens the picker again (US-004.AC-1/2/4).
 */
export const sessionMentionTriggerLogic = createStoreLogic<
  SessionMentionTriggerState,
  SessionMentionTriggerEvents
>({
  context: { observed: null, dismissedOffset: null },
  on: {
    observed: (context, { trigger }) => {
      const dismissedOffset =
        trigger !== null && trigger.offset === context.dismissedOffset
          ? context.dismissedOffset
          : null;
      if (sameTrigger(context.observed, trigger) && dismissedOffset === context.dismissedOffset) {
        return context;
      }
      return { observed: trigger, dismissedOffset };
    },
    dismissed: context =>
      context.observed === null
        ? context
        : { ...context, dismissedOffset: context.observed.offset },
  },
});

/** The occurrence the picker should serve, or `null` when none or dismissed. */
export function activeMentionTrigger(
  state: SessionMentionTriggerState
): SessionMentionTrigger | null {
  const { observed, dismissedOffset } = state;
  return observed !== null && observed.offset !== dismissedOffset ? observed : null;
}
