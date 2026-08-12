import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useStoreBinding } from "@/hooks/use-store-binding";

import { applyWindowManagerLayout } from "../adapters/window-manager-layouts-api";
import { parseWindowManagerLayoutDocument } from "../lib/window-manager-layout-schema";
import {
  windowManagerLayoutFingerprint,
  windowManagerLayoutReviewOptions,
} from "../lib/window-manager-layout-query";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutPreview,
  WindowManagerLayoutState,
  WindowManagerNormalizedRect,
} from "../lib/window-manager-layout-types";
import { windowManagerKeys } from "@/systems/os";

interface ReviewedLayout {
  fingerprint: string;
  preview: WindowManagerLayoutPreview;
}

export type WindowManagerLayoutEditorPhase =
  | "baseline"
  | "draft"
  | "dirty"
  | "saving"
  | "conflict"
  | "error";

export interface WindowManagerLayoutEditorStoreInput {
  initial: WindowManagerLayoutState;
  previous?: WindowManagerLayoutEditorStoreContext;
}

interface WindowManagerLayoutEditorStoreContext {
  baseline: WindowManagerLayoutDocument;
  baselineFingerprint: string;
  draft: WindowManagerLayoutDocument;
  error: Error | null;
  operation: number;
  phase: WindowManagerLayoutEditorPhase;
  revision: number;
}

type WindowManagerLayoutEditorStoreEvents = {
  applyFailed: { error: Error; fingerprint: string; operation: number; revision: number };
  applyRequested: {
    execute: (revision: number, draft: WindowManagerLayoutDocument) => Promise<void>;
  };
  applySucceeded: { fingerprint: string; operation: number; revision: number };
  draftChanged: { draft: WindowManagerLayoutDocument };
  importFailed: { error: Error };
  resetRequested: {};
};

function baselineContext(initial: WindowManagerLayoutState): WindowManagerLayoutEditorStoreContext {
  const baseline = structuredClone(initial.document);
  return {
    baseline,
    baselineFingerprint: windowManagerLayoutFingerprint(baseline),
    draft: structuredClone(baseline),
    error: null,
    operation: 0,
    phase: "baseline",
    revision: initial.revision,
  };
}

function reconcileLayoutEditorContext(
  previous: WindowManagerLayoutEditorStoreContext,
  initial: WindowManagerLayoutState
): WindowManagerLayoutEditorStoreContext {
  if (initial.revision < previous.revision) return previous;
  const baseline = structuredClone(initial.document);
  const baselineFingerprint = windowManagerLayoutFingerprint(baseline);
  if (
    initial.revision === previous.revision &&
    baselineFingerprint === previous.baselineFingerprint
  ) {
    return previous;
  }
  const clean = windowManagerLayoutFingerprint(previous.draft) === previous.baselineFingerprint;
  const draft = clean ? structuredClone(baseline) : previous.draft;
  return {
    ...previous,
    baseline,
    baselineFingerprint,
    draft,
    error: null,
    phase: windowManagerLayoutFingerprint(draft) === baselineFingerprint ? "baseline" : "dirty",
    revision: initial.revision,
  };
}

function isCurrentDraft(
  context: WindowManagerLayoutEditorStoreContext,
  revision: number,
  fingerprint: string
): boolean {
  return (
    context.revision === revision && windowManagerLayoutFingerprint(context.draft) === fingerprint
  );
}

function isConflictError(error: Error): boolean {
  return Reflect.get(error, "status") === 409;
}

export const windowManagerLayoutEditorLogic = createStoreLogic<
  WindowManagerLayoutEditorStoreContext,
  WindowManagerLayoutEditorStoreEvents,
  never,
  WindowManagerLayoutEditorStoreInput
>({
  context: input =>
    input.previous
      ? reconcileLayoutEditorContext(input.previous, input.initial)
      : baselineContext(input.initial),
  on: {
    applyFailed: (
      context,
      event: { error: Error; fingerprint: string; operation: number; revision: number }
    ) => {
      if (
        context.operation !== event.operation ||
        !isCurrentDraft(context, event.revision, event.fingerprint)
      ) {
        return;
      }
      return {
        ...context,
        error: event.error,
        phase: isConflictError(event.error) ? "conflict" : "error",
      };
    },
    applyRequested: (context, event, enqueue) => {
      if (context.phase === "saving") return;
      const draft = structuredClone(context.draft);
      const fingerprint = windowManagerLayoutFingerprint(draft);
      const operation = context.operation + 1;
      const revision = context.revision;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute(revision, draft);
          trigger.applySucceeded({ fingerprint, operation, revision });
        } catch (cause) {
          const error = cause instanceof Error ? cause : new Error("Unable to apply the layout.");
          trigger.applyFailed({ error, fingerprint, operation, revision });
        }
      });
      return { ...context, error: null, operation, phase: "saving" };
    },
    applySucceeded: (
      context,
      event: { fingerprint: string; operation: number; revision: number }
    ) => {
      if (
        context.operation !== event.operation ||
        !isCurrentDraft(context, event.revision, event.fingerprint)
      ) {
        return;
      }
      return { ...context, error: null, phase: "draft" };
    },
    draftChanged: (context, event: { draft: WindowManagerLayoutDocument }) => ({
      ...context,
      draft: event.draft,
      error: null,
      phase: "dirty",
    }),
    importFailed: (context, event: { error: Error }) => ({
      ...context,
      error: event.error,
      phase: "error",
    }),
    resetRequested: context => ({
      ...context,
      draft: structuredClone(context.baseline),
      error: null,
      phase: "baseline",
    }),
  },
});

export function useWindowManagerLayoutEditor(
  workspaceId: string,
  initial: WindowManagerLayoutState
) {
  const initialFingerprint = windowManagerLayoutFingerprint(initial.document);
  const { store } = useStoreBinding(
    workspaceId,
    () => windowManagerLayoutEditorLogic.createStore({ initial }),
    previous =>
      windowManagerLayoutEditorLogic.createStore({
        initial,
        previous: previous.getSnapshot().context,
      }),
    (current, nextWorkspaceId) => {
      if (current.key !== nextWorkspaceId) return true;
      const previousContext = current.store.getSnapshot().context;
      const baselineChanged =
        initial.revision > previousContext.revision ||
        (initial.revision === previousContext.revision &&
          initialFingerprint !== previousContext.baselineFingerprint);
      return previousContext.phase !== "saving" && baselineChanged;
    }
  );
  const context = useSelector(store, snapshot => snapshot.context);
  const queryClient = useQueryClient();

  const fingerprint = windowManagerLayoutFingerprint(context.draft);
  const dirty = fingerprint !== context.baselineFingerprint;
  const review = useQuery(
    windowManagerLayoutReviewOptions(workspaceId, context.revision, context.draft)
  );

  const applyMutation = useMutation({
    mutationFn: async ({
      candidate,
      revision,
    }: {
      candidate: WindowManagerLayoutDocument;
      revision: number;
    }) => {
      await applyWindowManagerLayout(workspaceId, revision, candidate);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: windowManagerKeys.snapshot(workspaceId) });
    },
  });
  const requestApply = () =>
    store.trigger.applyRequested({
      execute: async (revision, candidate) => {
        await applyMutation.mutateAsync({ candidate, revision });
      },
    });

  const updateDraft = (draft: WindowManagerLayoutDocument) => store.trigger.draftChanged({ draft });
  const importDocument = async (file: File) => {
    try {
      const value: unknown = JSON.parse(await file.text());
      const imported = parseWindowManagerLayoutDocument(value);
      updateDraft({ ...imported, workspaceId });
    } catch (cause) {
      store.trigger.importFailed({
        error:
          cause instanceof Error ? cause : new Error("The selected file is not a layout document."),
      });
    }
  };
  const setDesktop = (index: number, next: WindowManagerLayoutDesktop) => {
    updateDraft({
      ...context.draft,
      desktops: context.draft.desktops.map((desktop, position) =>
        position === index ? next : desktop
      ),
    });
  };
  const setDesktopName = (index: number, name: string) => {
    const desktop = context.draft.desktops[index];
    if (!desktop) return;
    setDesktop(index, { ...desktop, name });
  };
  const setFloatingRect = (windowId: string, floatingRect: WindowManagerNormalizedRect) => {
    const window = context.draft.windows[windowId];
    if (!window) return;
    updateDraft({
      ...context.draft,
      windows: { ...context.draft.windows, [windowId]: { ...window, floatingRect } },
    });
  };

  const currentReview = review.data?.fingerprint === fingerprint ? review.data : null;
  const reviewed: ReviewedLayout | null = currentReview?.preview
    ? { fingerprint, preview: currentReview.preview }
    : null;
  return {
    apply: { mutate: requestApply },
    dirty,
    draft: context.draft,
    exportDocument: () => structuredClone(context.draft),
    importDocument,
    importError: context.phase === "error" ? (context.error?.message ?? null) : null,
    mutationError: context.error ?? review.error ?? applyMutation.error,
    phase: context.phase,
    reset: () => store.trigger.resetRequested(),
    review,
    reviewed,
    reviewCurrent: reviewed !== null,
    revision: context.revision,
    setDesktop,
    setDesktopName,
    setFloatingRect,
    updateDraft,
    validation: currentReview?.validation ?? null,
  };
}

export type WindowManagerLayoutEditorModel = ReturnType<typeof useWindowManagerLayoutEditor>;
