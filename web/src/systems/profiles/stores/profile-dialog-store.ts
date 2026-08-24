import { createStore } from "@xstate/store";

import type { ProfileDialogIntent } from "../types";

/**
 * The one lifecycle-dialog intent.
 *
 * The palette, the switcher, and the Settings page all raise the same dialogs
 * through this store, so no surface owns a second mutation path (ADR-016). The
 * dialogs themselves hold the plan and the revision; this only records which one
 * should be open and against which profile.
 */
export const profileDialogStore = createStore({
  context: {
    intent: null as ProfileDialogIntent | null,
  },
  on: {
    opened: (context, event: { intent: ProfileDialogIntent }) => ({
      ...context,
      intent: event.intent,
    }),
    closed: context => (context.intent === null ? undefined : { ...context, intent: null }),
  },
});

export const profileDialogSelectors = {
  intent: profileDialogStore.select(context => context.intent),
};

export function openProfileDialog(intent: ProfileDialogIntent): void {
  profileDialogStore.trigger.opened({ intent });
}

export function closeProfileDialog(): void {
  profileDialogStore.trigger.closed();
}
