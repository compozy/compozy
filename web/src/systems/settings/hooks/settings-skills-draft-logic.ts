import { createStoreLogic } from "@xstate/store";

import type { SettingsSkillsSection } from "@/systems/settings";

type SkillsConfig = SettingsSkillsSection["config"];
type SaveKind = "disabled" | "policy" | "sources";

export type SkillsDraftUpdate =
  | SkillsConfig
  | null
  | ((current: SkillsConfig | null) => SkillsConfig | null);

export type SkillsDraftFlow = {
  baseline: SkillsConfig | null;
  draft: SkillsConfig | null;
  key: string;
  labels: Record<SaveKind, string | null>;
  nextAttempt: number;
  pending: Record<SaveKind, number | null>;
};

interface SkillsDraftInput {
  baseline: SkillsConfig | null;
  key: string;
  previous?: SkillsDraftFlow;
}

function sameConfig(left: SkillsConfig | null, right: SkillsConfig | null): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/**
 * A save only advances the baseline for the fields it wrote. Folding the whole
 * response in would silently accept another section's unsaved draft as saved.
 */
function mergeSavedSkillsBaseline(
  current: SkillsConfig | null,
  saved: SkillsConfig,
  kind: SaveKind
): SkillsConfig {
  if (current === null) return saved;
  if (kind === "disabled") {
    return { ...current, disabled_skills: saved.disabled_skills };
  }
  if (kind === "sources") {
    return { ...current, sources: saved.sources, custom_sources: saved.custom_sources };
  }
  return {
    ...current,
    enabled: saved.enabled,
    poll_interval: saved.poll_interval,
    marketplace: saved.marketplace,
    allowed_marketplace_mcp: saved.allowed_marketplace_mcp,
    allowed_marketplace_hooks: saved.allowed_marketplace_hooks,
  };
}

function createSkillsDraftFlow(input: SkillsDraftInput): SkillsDraftFlow {
  const previous = input.previous;
  if (!previous || previous.key !== input.key) {
    return {
      baseline: input.baseline,
      draft: input.baseline,
      key: input.key,
      labels: { disabled: null, policy: null, sources: null },
      nextAttempt: previous?.nextAttempt ?? 0,
      pending: { disabled: null, policy: null, sources: null },
    };
  }
  const preserveDraft = !sameConfig(previous.draft, previous.baseline);
  return {
    ...previous,
    baseline: input.baseline,
    draft: preserveDraft ? previous.draft : input.baseline,
  };
}

export function shouldRebindSkillsDraft(
  context: SkillsDraftFlow,
  baseline: SkillsConfig | null,
  key: string
): boolean {
  if (context.key !== key) return true;
  if (Object.values(context.pending).some(attempt => attempt !== null)) return false;
  return !sameConfig(context.baseline, baseline);
}

/** Scoped skills draft workflow; switching the server scope atomically replaces the baseline. */
export const settingsSkillsDraftLogic = createStoreLogic({
  context: (input: SkillsDraftInput): SkillsDraftFlow => createSkillsDraftFlow(input),
  on: {
    draftChanged: (context, event: { update: SkillsDraftUpdate }) => ({
      ...context,
      draft: typeof event.update === "function" ? event.update(context.draft) : event.update,
    }),
    saveFailed: (context, event: { attempt: number; kind: SaveKind }) =>
      context.pending[event.kind] === event.attempt
        ? { ...context, pending: { ...context.pending, [event.kind]: null } }
        : undefined,
    saveRequested: (
      context,
      event: {
        baseline: SkillsConfig;
        execute: () => Promise<unknown>;
        kind: SaveKind;
        label: string;
      },
      enqueue
    ) => {
      if (!context.draft || context.pending[event.kind] !== null) return;
      const attempt = context.nextAttempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.saveSucceeded({
            attempt,
            baseline: event.baseline,
            kind: event.kind,
            label: event.label,
          });
        } catch {
          trigger.saveFailed({ attempt, kind: event.kind });
        }
      });
      return {
        ...context,
        nextAttempt: attempt,
        pending: { ...context.pending, [event.kind]: attempt },
      };
    },
    saveSucceeded: (
      context,
      event: { attempt: number; baseline: SkillsConfig; kind: SaveKind; label: string }
    ) =>
      context.pending[event.kind] === event.attempt
        ? {
            ...context,
            baseline: mergeSavedSkillsBaseline(context.baseline, event.baseline, event.kind),
            labels: { ...context.labels, [event.kind]: event.label },
            pending: { ...context.pending, [event.kind]: null },
          }
        : undefined,
  },
});
