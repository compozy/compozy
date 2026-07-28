import { createStoreLogic, type EnqueueObject } from "@xstate/store";

type BundleScope = "global" | "workspace";

export interface BundleActivationDraft {
  profile: string;
  scope: BundleScope;
  confirmNetworkRequirement: boolean;
}

interface BundleActivationBase extends BundleActivationDraft {
  requestId: number;
}

export type BundleActivationDialogState =
  | (BundleActivationBase & { phase: "preview-pending" })
  | (BundleActivationBase & { phase: "previewing" })
  | (BundleActivationBase & { phase: "ready" })
  | (BundleActivationBase & { phase: "activating" })
  | (BundleActivationBase & { phase: "activated" })
  | (BundleActivationBase & { phase: "failed"; stage: "preview" | "activate"; error: string });

export interface BundleActivationDialogInput extends BundleActivationDraft {}

type BundleActivationDialogEvents = {
  previewExecutionProvided: { preview: BundlePreviewExecution };
  profileSelected: { preview: BundlePreviewExecution; profile: string };
  scopeSelected: { preview: BundlePreviewExecution; scope: BundleScope };
  networkRequirementConfirmed: { confirmed: boolean };
  previewRetried: { preview: BundlePreviewExecution };
  previewSucceeded: { requestId: number };
  previewFailed: { requestId: number; error: string };
  activationSubmitted: { activate: BundleActivationExecution; notifySuccess: () => void };
  activationSucceeded: { notifySuccess: () => void; requestId: number };
  activationFailed: { requestId: number; error: string };
};

type BundleActivationDialogEmitted = {
  activated: {};
};

type BundlePreviewExecution = (draft: BundleActivationDraft) => Promise<void>;
type BundleActivationExecution = (draft: BundleActivationDraft) => Promise<void>;

type BundleActivationEnqueue = EnqueueObject<
  BundleActivationDialogState,
  { type: "activated" },
  BundleActivationDialogEvents
>;

export function createBundleActivationDialogLogic() {
  return createStoreLogic<
    BundleActivationDialogState,
    BundleActivationDialogEvents,
    BundleActivationDialogEmitted,
    BundleActivationDialogInput
  >({
    context: input => ({ ...input, phase: "preview-pending", requestId: 0 }),
    on: {
      previewExecutionProvided: (context, event, enqueue) => {
        if (context.phase !== "preview-pending") return;
        return preview(enqueue, context, event.preview);
      },
      profileSelected: (context, event, enqueue) => {
        if (context.phase === "activating" || context.profile === event.profile) return;
        return preview(enqueue, { ...context, profile: event.profile }, event.preview);
      },
      scopeSelected: (context, event, enqueue) => {
        if (context.phase === "activating" || context.scope === event.scope) return;
        return preview(enqueue, { ...context, scope: event.scope }, event.preview);
      },
      networkRequirementConfirmed: (context, event) => ({
        ...context,
        confirmNetworkRequirement: event.confirmed,
      }),
      previewRetried: (context, event, enqueue) => {
        if (context.phase === "activating") return;
        return preview(enqueue, context, event.preview);
      },
      previewSucceeded: (context, event) => {
        if (context.phase !== "previewing" || context.requestId !== event.requestId) return;
        return { ...context, phase: "ready" };
      },
      previewFailed: (context, event) => {
        if (context.phase !== "previewing" || context.requestId !== event.requestId) return;
        return { ...context, phase: "failed", stage: "preview", error: event.error };
      },
      activationSubmitted: (context, event, enqueue) => {
        if (
          context.phase !== "ready" &&
          !(context.phase === "failed" && context.stage === "activate")
        )
          return;
        const requestId = context.requestId + 1;
        const draft = bundleActivationDraft(context);
        enqueue.effect(async ({ trigger }) => {
          try {
            await event.activate(draft);
            trigger.activationSucceeded({ notifySuccess: event.notifySuccess, requestId });
          } catch (error) {
            trigger.activationFailed({
              requestId,
              error: errorMessage(error, "Failed to activate bundle"),
            });
          }
        });
        return { ...context, phase: "activating", requestId };
      },
      activationSucceeded: (context, event, enqueue) => {
        if (context.phase !== "activating" || context.requestId !== event.requestId) return;
        enqueue.effect(event.notifySuccess);
        enqueue.emit.activated({});
        return { ...context, phase: "activated" };
      },
      activationFailed: (context, event) => {
        if (context.phase !== "activating" || context.requestId !== event.requestId) return;
        return { ...context, phase: "failed", stage: "activate", error: event.error };
      },
    },
  });
}

function preview(
  enqueue: BundleActivationEnqueue,
  context: BundleActivationDialogState,
  execute: BundlePreviewExecution
): BundleActivationDialogState {
  const requestId = context.requestId + 1;
  const draft = bundleActivationDraft(context);
  enqueue.effect(async ({ trigger }) => {
    try {
      await execute(draft);
      trigger.previewSucceeded({ requestId });
    } catch (error) {
      trigger.previewFailed({
        requestId,
        error: errorMessage(error, "The bundle preview could not be loaded"),
      });
    }
  });
  return { ...context, phase: "previewing", requestId };
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}

function bundleActivationDraft(context: BundleActivationDialogState): BundleActivationDraft {
  return {
    confirmNetworkRequirement: context.confirmNetworkRequirement,
    profile: context.profile,
    scope: context.scope,
  };
}
