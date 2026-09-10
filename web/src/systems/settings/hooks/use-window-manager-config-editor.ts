import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useStoreBinding } from "@/hooks/use-store-binding";

import {
  updateWindowManagerSettings,
  type WindowManagerSettingsSaveResult,
} from "../adapters/window-manager-layouts-api";
import { WINDOW_MANAGER_RANGES } from "../lib/window-manager-snap-geometry";
import { type WindowManagerConfig, windowManagerKeys } from "@/systems/os";

export type WindowManagerConfigEditorPhase =
  | "baseline"
  | "draft"
  | "dirty"
  | "saving"
  | "conflict"
  | "error";

export type WindowManagerConfigProblem =
  | { field: "gaps"; message: string }
  | { field: "snap"; message: string }
  | { field: "repeatRatios"; message: string }
  | { field: "historyLimit"; message: string };

interface WindowManagerConfigEditorStoreContext {
  baseline: WindowManagerConfig;
  baselineRevision: string;
  inputRevision: string;
  draft: WindowManagerConfig;
  draftRevision: number;
  error: Error | null;
  result: WindowManagerSettingsSaveResult | null;
  operation: number;
  phase: WindowManagerConfigEditorPhase;
}

interface WindowManagerConfigEditorStoreInput {
  baseline: WindowManagerConfig;
  previous?: WindowManagerConfigEditorStoreContext;
}

type WindowManagerConfigEditorStoreEvents = {
  draftChanged: { draft: WindowManagerConfig };
  resetRequested: {};
  saveFailed: { error: Error; operation: number; revision: string; draftRevision: number };
  saveRequested: {
    execute: (draft: WindowManagerConfig) => Promise<WindowManagerSettingsSaveResult>;
  };
  saveSucceeded: {
    operation: number;
    revision: string;
    draftRevision: number;
    result: WindowManagerSettingsSaveResult;
  };
};

function configRevision(config: WindowManagerConfig): string {
  const {
    shortcuts: _shortcuts,
    globalShortcuts: _globalShortcuts,
    shortcutDefaults: _defaults,
    effectiveShortcuts: _effective,
    ...behavior
  } = config;
  return JSON.stringify(behavior);
}

function isCurrentConfig(
  context: WindowManagerConfigEditorStoreContext,
  revision: string
): boolean {
  return context.baselineRevision === revision;
}

function isConflictError(error: Error): boolean {
  return Reflect.get(error, "status") === 409;
}

function initialConfigEditorContext(
  baseline: WindowManagerConfig
): WindowManagerConfigEditorStoreContext {
  return {
    baseline: structuredClone(baseline),
    baselineRevision: configRevision(baseline),
    inputRevision: configRevision(baseline),
    draft: structuredClone(baseline),
    draftRevision: 0,
    error: null,
    result: null,
    operation: 0,
    phase: "baseline",
  };
}

function reconcileConfigEditorContext(
  previous: WindowManagerConfigEditorStoreContext,
  baseline: WindowManagerConfig
): WindowManagerConfigEditorStoreContext {
  const revision = configRevision(baseline);
  if (revision === previous.inputRevision) return previous;
  const clean = configRevision(previous.draft) === previous.baselineRevision;
  const draft = clean ? structuredClone(baseline) : previous.draft;
  return {
    ...previous,
    baseline: structuredClone(baseline),
    baselineRevision: revision,
    inputRevision: revision,
    draft,
    draftRevision: previous.draftRevision + 1,
    error: previous.error,
    phase: configRevision(draft) === revision ? "baseline" : "dirty",
  };
}

export const windowManagerConfigEditorLogic = createStoreLogic<
  WindowManagerConfigEditorStoreContext,
  WindowManagerConfigEditorStoreEvents,
  never,
  WindowManagerConfigEditorStoreInput
>({
  context: input =>
    input.previous
      ? reconcileConfigEditorContext(input.previous, input.baseline)
      : initialConfigEditorContext(input.baseline),
  on: {
    draftChanged: (context, event: { draft: WindowManagerConfig }) => ({
      ...context,
      draft: event.draft,
      draftRevision: context.draftRevision + 1,
      error: null,
      result: null,
      phase: context.phase === "saving" ? "saving" : "dirty",
    }),
    resetRequested: context => ({
      ...context,
      draft: structuredClone(context.baseline),
      draftRevision: context.draftRevision + 1,
      error: null,
      result: null,
      phase: "baseline",
    }),
    saveFailed: (
      context,
      event: { error: Error; operation: number; revision: string; draftRevision: number }
    ) => {
      if (context.operation !== event.operation || !isCurrentConfig(context, event.revision))
        return;
      if (context.draftRevision !== event.draftRevision) {
        return { ...context, phase: "dirty" };
      }
      return {
        ...context,
        error: event.error,
        phase: isConflictError(event.error) ? "conflict" : "error",
      };
    },
    saveRequested: (context, event, enqueue) => {
      if (context.phase === "saving") return;
      const draft = structuredClone(context.draft);
      const draftRevision = context.draftRevision;
      const operation = context.operation + 1;
      const revision = context.baselineRevision;
      enqueue.effect(async ({ trigger }) => {
        try {
          const result = await event.execute(draft);
          trigger.saveSucceeded({ draftRevision, operation, revision, result });
        } catch (cause) {
          const error =
            cause instanceof Error ? cause : new Error("Unable to save window-manager settings.");
          trigger.saveFailed({ draftRevision, error, operation, revision });
        }
      });
      return { ...context, error: null, result: null, operation, phase: "saving" };
    },
    saveSucceeded: (
      context,
      event: {
        operation: number;
        revision: string;
        draftRevision: number;
        result: WindowManagerSettingsSaveResult;
      }
    ) => {
      if (context.operation !== event.operation || !isCurrentConfig(context, event.revision))
        return;
      return {
        ...context,
        baseline: structuredClone(event.result.config),
        baselineRevision: configRevision(event.result.config),
        draft:
          context.draftRevision === event.draftRevision
            ? structuredClone(event.result.config)
            : context.draft,
        error: null,
        result: context.draftRevision === event.draftRevision ? event.result : null,
        phase: context.draftRevision === event.draftRevision ? "baseline" : "dirty",
      };
    },
  },
});

function inRange(value: number, range: { min: number; max: number }): boolean {
  return Number.isInteger(value) && value >= range.min && value <= range.max;
}

function collectProblems(config: WindowManagerConfig): WindowManagerConfigProblem[] {
  const problems: WindowManagerConfigProblem[] = [];
  const ranges = WINDOW_MANAGER_RANGES;
  if (!Object.values(config.gaps).every(gap => inRange(gap, ranges.gap))) {
    problems.push({
      field: "gaps",
      message: `Gaps run from ${ranges.gap.min} to ${ranges.gap.max} pixels.`,
    });
  }
  if (
    !inRange(config.snap.edgeBand, ranges.edgeBand) ||
    !inRange(config.snap.cornerReach, ranges.cornerReach) ||
    !inRange(config.snap.exitSlack, ranges.exitSlack)
  ) {
    problems.push({ field: "snap", message: "A snap threshold is outside its range." });
  }
  const ratios = config.snap.repeatRatios;
  const canonical = ratios.map(ratio => Math.round(ratio * 1_000_000));
  if (ratios.length < ranges.repeatStops.min || ratios.length > ranges.repeatStops.max) {
    problems.push({
      field: "repeatRatios",
      message: `Keep between ${ranges.repeatStops.min} and ${ranges.repeatStops.max} repeat widths.`,
    });
  } else if (new Set(canonical).size !== ratios.length) {
    problems.push({ field: "repeatRatios", message: "Two repeat widths sit at the same stop." });
  } else if (
    !ratios.every(ratio => ratio >= ranges.repeatRatio.min && ratio <= ranges.repeatRatio.max)
  ) {
    problems.push({
      field: "repeatRatios",
      message: "Repeat widths run from a tenth of the screen to nine tenths.",
    });
  }
  if (!inRange(config.historyLimit, ranges.historyLimit)) {
    problems.push({
      field: "historyLimit",
      message: `Layout history runs from ${ranges.historyLimit.min} to ${ranges.historyLimit.max} steps.`,
    });
  }
  return problems;
}

export function useWindowManagerConfigEditor(baseline: WindowManagerConfig) {
  const baselineRevision = configRevision(baseline);
  const { store } = useStoreBinding(
    baselineRevision,
    () => windowManagerConfigEditorLogic.createStore({ baseline }),
    previous =>
      windowManagerConfigEditorLogic.createStore({
        baseline,
        previous: previous.getSnapshot().context,
      }),
    current => {
      const previousContext = current.store.getSnapshot().context;
      return (
        previousContext.phase !== "saving" && previousContext.inputRevision !== baselineRevision
      );
    }
  );
  const context = useSelector(store, snapshot => snapshot.context);
  const queryClient = useQueryClient();

  const saveMutation = useMutation({
    mutationFn: async (next: WindowManagerConfig) => {
      return updateWindowManagerSettings(next);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: windowManagerKeys.configs() });
    },
  });
  const dirty = configRevision(context.draft) !== context.baselineRevision;
  const problems = collectProblems(context.draft);
  const setDraft = (
    next: WindowManagerConfig | ((current: WindowManagerConfig) => WindowManagerConfig)
  ) => {
    const draft = typeof next === "function" ? next(context.draft) : next;
    store.trigger.draftChanged({ draft });
  };

  return {
    canSave:
      (dirty || context.error !== null) && problems.length === 0 && context.phase !== "saving",
    dirty,
    draft: context.draft,
    error: context.error,
    result: context.result,
    phase: context.phase,
    problems,
    reset: () => store.trigger.resetRequested(),
    save: () =>
      store.trigger.saveRequested({
        execute: async draft => {
          return saveMutation.mutateAsync(draft);
        },
      }),
    setDraft,
  };
}

export type WindowManagerConfigEditorModel = ReturnType<typeof useWindowManagerConfigEditor>;
