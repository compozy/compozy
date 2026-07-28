import { createStoreLogic } from "@xstate/store";

import type { SettingsRolesConfig } from "../types";

type RolesDraftFlow = {
  baseline: SettingsRolesConfig | null;
  draft: SettingsRolesConfig | null;
  lastAppliedLabel: string | null;
  nextAttempt: number;
  pendingSaveAttempt: number | null;
};

interface RolesDraftInput {
  baseline: SettingsRolesConfig | null;
  lastAppliedLabel: string | null;
}

/** Per-settings-page draft workflow seeded from the current Query baseline. */
export const settingsRolesDraftLogic = createStoreLogic<
  RolesDraftFlow,
  {
    draftChanged: { draft: SettingsRolesConfig | null };
    draftReset: { baseline: SettingsRolesConfig | null };
    saveFailed: { attempt: number };
    saveRequested: { execute: () => Promise<unknown> };
    saveSucceeded: { attempt: number };
  },
  never,
  RolesDraftInput
>({
  context: input => ({
    baseline: input.baseline,
    draft: input.baseline,
    lastAppliedLabel: input.lastAppliedLabel,
    nextAttempt: 0,
    pendingSaveAttempt: null,
  }),
  on: {
    draftChanged: (context, event: { draft: SettingsRolesConfig | null }) => ({
      ...context,
      draft: event.draft,
    }),
    draftReset: (context, event) => ({
      ...context,
      baseline: event.baseline,
      draft: event.baseline,
      pendingSaveAttempt: null,
    }),
    saveFailed: (context, event: { attempt: number }) =>
      context.pendingSaveAttempt === event.attempt
        ? { ...context, pendingSaveAttempt: null }
        : undefined,
    saveRequested: (context, event, enqueue) => {
      if (!context.draft || context.pendingSaveAttempt !== null) return;
      const attempt = context.nextAttempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.saveSucceeded({ attempt });
        } catch {
          trigger.saveFailed({ attempt });
        }
      });
      return { ...context, nextAttempt: attempt, pendingSaveAttempt: attempt };
    },
    saveSucceeded: (context, event: { attempt: number }) =>
      context.pendingSaveAttempt === event.attempt
        ? {
            ...context,
            baseline: context.draft,
            lastAppliedLabel: "Saved · applied immediately",
            pendingSaveAttempt: null,
          }
        : undefined,
  },
});

export function shouldAdoptRolesBaseline(
  context: RolesDraftFlow,
  baseline: SettingsRolesConfig | null
): boolean {
  if (sameConfig(context.baseline, baseline) || context.pendingSaveAttempt !== null) return false;
  return sameConfig(context.draft, context.baseline);
}

function sameConfig(left: SettingsRolesConfig | null, right: SettingsRolesConfig | null): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}
