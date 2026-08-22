import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import type { NetworkParticipationDraft } from "@/lib/network-participation";
import { notifyUser } from "@/lib/user-feedback";
import { createdInProfileToast } from "@/systems/profiles";

import { LoopInputValidationError } from "../adapters/loops-api";
import { initialOverrideDraft, type LoopOverrideDraft } from "../lib/loop-overrides";
import { initialRunInputs, type LoopRunInputs } from "../lib/loop-run-form";
import type {
  LoopDryRunPreview,
  LoopEffectiveConfig,
  LoopInputSchema,
  RunLoopResult,
} from "../types";

interface LoopRunFormPendingRequest {
  attempt: number;
  generation: number;
  kind: "dry-run" | "run";
}

interface LoopRunFormState {
  dryRunGeneration: number;
  fieldErrors: Record<string, string>;
  inputs: LoopRunInputs;
  networkParticipation: NetworkParticipationDraft;
  networkParticipationOverridden: boolean;
  nextAttempt: number;
  overrides: LoopOverrideDraft;
  pendingRequest: LoopRunFormPendingRequest | null;
  plan: LoopDryRunPreview | null;
  submitAttempted: boolean;
}

type LoopRunFormEvents = {
  dryRunFailed: { attempt: number; error: unknown };
  dryRunRequested: { execute: () => Promise<RunLoopResult> };
  dryRunSucceeded: {
    attempt: number;
    generation: number;
    plan: LoopDryRunPreview | null;
  };
  inputChanged: { name: string; value: unknown };
  networkParticipationChanged: { draft: NetworkParticipationDraft };
  overridesChanged: { overrides: LoopOverrideDraft };
  runFailed: { attempt: number; error: unknown; loopName: string };
  runRequested: {
    /**
     * Whether the confirmation should name the run's owner. Only the aggregate
     * needs it: a scoped view already says whose runs these are.
     */
    announceOwner: boolean;
    execute: () => Promise<RunLoopResult>;
    loopName: string;
  };
  runSucceeded: { attempt: number; ownerProfile?: string; runId?: string };
  submissionAttempted: {};
};

type LoopRunFormEmitted = {
  runStarted: { runId?: string };
};

interface LoopRunFormScope {
  loopName: string;
  workspaceId: string;
}

interface LoopRunFormStateInput {
  effectiveConfig: LoopEffectiveConfig;
  networkParticipation: NetworkParticipationDraft;
  schema: LoopInputSchema | undefined;
  scope: LoopRunFormScope;
}

function scopeKey({ loopName, workspaceId }: LoopRunFormScope): string {
  return `${workspaceId}\u0000${loopName}`;
}

function draftChanged(current: LoopRunFormState): LoopRunFormState {
  return {
    ...current,
    dryRunGeneration: current.dryRunGeneration + 1,
    plan: null,
  };
}

export const loopRunFormLogic = createStoreLogic<
  LoopRunFormState,
  LoopRunFormEvents,
  LoopRunFormEmitted,
  LoopRunFormStateInput
>({
  context: (input: LoopRunFormStateInput): LoopRunFormState => ({
    dryRunGeneration: 0,
    fieldErrors: {},
    inputs: initialRunInputs(input.schema),
    networkParticipation: input.networkParticipation,
    networkParticipationOverridden: false,
    nextAttempt: 0,
    overrides: initialOverrideDraft(input.effectiveConfig),
    pendingRequest: null,
    plan: null,
    submitAttempted: false,
  }),
  on: {
    inputChanged: (current, event: { name: string; value: unknown }) => ({
      ...draftChanged(current),
      fieldErrors: Object.fromEntries(
        Object.entries(current.fieldErrors).filter(([name]) => name !== event.name)
      ),
      inputs: { ...current.inputs, [event.name]: event.value },
    }),
    overridesChanged: (current, event: { overrides: LoopOverrideDraft }) => ({
      ...draftChanged(current),
      overrides: event.overrides,
    }),
    networkParticipationChanged: (current, event: { draft: NetworkParticipationDraft }) => ({
      ...draftChanged(current),
      networkParticipation: event.draft,
      networkParticipationOverridden: true,
    }),
    submissionAttempted: current => ({ ...current, submitAttempted: true }),
    dryRunFailed: (current, event, enqueue) => {
      if (
        current.pendingRequest?.kind !== "dry-run" ||
        current.pendingRequest.attempt !== event.attempt
      ) {
        return;
      }
      enqueue.effect(() =>
        notifyUser({ message: errorMessage(event.error, "Dry run failed"), tone: "error" })
      );
      return {
        ...current,
        fieldErrors: inputFieldErrors(event.error),
        pendingRequest: null,
      };
    },
    dryRunRequested: (current, event, enqueue) => {
      if (current.pendingRequest) return;
      const attempt = current.nextAttempt + 1;
      const generation = current.dryRunGeneration + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          const result = await event.execute();
          trigger.dryRunSucceeded({ attempt, generation, plan: result.dry_run ?? null });
        } catch (error) {
          trigger.dryRunFailed({ attempt, error });
        }
      });
      return {
        ...current,
        dryRunGeneration: generation,
        nextAttempt: attempt,
        pendingRequest: { attempt, generation, kind: "dry-run" },
        submitAttempted: true,
      };
    },
    dryRunSucceeded: (current, event, enqueue) => {
      if (
        current.pendingRequest?.kind !== "dry-run" ||
        current.pendingRequest.attempt !== event.attempt
      ) {
        return;
      }
      if (event.generation !== current.dryRunGeneration) {
        return { ...current, pendingRequest: null };
      }
      enqueue.effect(() =>
        notifyUser({
          message: "Dry run passed — inputs valid, plan rendered",
          tone: "success",
        })
      );
      return { ...current, pendingRequest: null, plan: event.plan };
    },
    runFailed: (current, event, enqueue) => {
      if (
        current.pendingRequest?.kind !== "run" ||
        current.pendingRequest.attempt !== event.attempt
      ) {
        return;
      }
      enqueue.effect(() =>
        notifyUser({
          message: errorMessage(event.error, `Failed to run ${event.loopName}`),
          tone: "error",
        })
      );
      return {
        ...current,
        fieldErrors: inputFieldErrors(event.error),
        pendingRequest: null,
      };
    },
    runRequested: (current, event, enqueue) => {
      if (current.pendingRequest) return;
      const attempt = current.nextAttempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          const result = await event.execute();
          trigger.runSucceeded({
            attempt,
            // The owner the daemon returned, never the one we asked for: an
            // echoed request would keep saying "default" even if the run was
            // filed elsewhere, which is the misfile this exists to surface.
            ownerProfile: event.announceOwner ? result.run?.profile_name : undefined,
            runId: result.run?.id,
          });
        } catch (error) {
          trigger.runFailed({ attempt, error, loopName: event.loopName });
        }
      });
      return {
        ...current,
        nextAttempt: attempt,
        pendingRequest: { attempt, generation: current.dryRunGeneration, kind: "run" },
        submitAttempted: true,
      };
    },
    runSucceeded: (current, event, enqueue) => {
      if (
        current.pendingRequest?.kind !== "run" ||
        current.pendingRequest.attempt !== event.attempt
      ) {
        return;
      }
      const started = event.runId ? `Run started · ${event.runId}` : "Run started";
      enqueue.effect(() =>
        notifyUser({
          message: event.ownerProfile
            ? `${started} ${createdInProfileToast(event.ownerProfile)}`
            : started,
          tone: "success",
        })
      );
      enqueue.emit.runStarted({ runId: event.runId });
      return { ...current, pendingRequest: null };
    },
  },
});

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}

function inputFieldErrors(error: unknown): Record<string, string> {
  return error instanceof LoopInputValidationError ? { ...error.fieldErrors } : {};
}

export function useLoopRunFormState(input: LoopRunFormStateInput) {
  const { store } = useStoreBinding(scopeKey(input.scope), () =>
    loopRunFormLogic.createStore(input)
  );
  const inputs = useSelector(store, state => state.context.inputs);
  const fieldErrors = useSelector(store, state => state.context.fieldErrors);
  const networkParticipation = useSelector(store, state => state.context.networkParticipation);
  const networkParticipationOverridden = useSelector(
    store,
    state => state.context.networkParticipationOverridden
  );
  const overrides = useSelector(store, state => state.context.overrides);
  const pendingRequest = useSelector(store, state => state.context.pendingRequest);
  const plan = useSelector(store, state => state.context.plan);
  const submitAttempted = useSelector(store, state => state.context.submitAttempted);

  return {
    inputs,
    fieldErrors,
    networkParticipation,
    networkParticipationOverridden,
    overrides,
    plan,
    pendingRequest,
    submitAttempted,
    setInput: (name: string, value: unknown) => store.trigger.inputChanged({ name, value }),
    setOverrides: (next: LoopOverrideDraft) => store.trigger.overridesChanged({ overrides: next }),
    setNetworkParticipation: (draft: NetworkParticipationDraft) =>
      store.trigger.networkParticipationChanged({ draft }),
    markSubmissionAttempted: () => store.trigger.submissionAttempted(),
    requestDryRun: (execute: () => Promise<RunLoopResult>) =>
      store.trigger.dryRunRequested({ execute }),
    requestRun: (loopName: string, execute: () => Promise<RunLoopResult>, announceOwner = false) =>
      store.trigger.runRequested({ announceOwner, execute, loopName }),
    store,
  };
}
