import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import {
  DEFAULT_NETWORK_PARTICIPATION_DRAFT,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/lib/network-participation";

import type { FanOutTaskRunsRequest, FanOutTaskRunsResponse } from "../types";

const strategies = ["named", "run"] as const;

interface FanOutDialogState {
  attemptId: number;
  designationsText: string;
  formError: string | null;
  networkParticipation: NetworkParticipationDraft;
  phase: "editing" | "submitting";
  /** One fresh worktree per designated run. Off by default — isolation is opt-in. */
  worktreePerRun: boolean;
  /**
   * The accepted response, kept only for an isolated fan-out: per-run worktree
   * attribution exists nowhere else, so closing straight away would discard the
   * one report that says which run landed where.
   */
  result: FanOutTaskRunsResponse | null;
}

type FanOutDialogEvents = {
  designationsChanged: { value: string };
  worktreePerRunChanged: { value: boolean };
  networkParticipationChanged: { value: NetworkParticipationDraft };
  openChanged: { open: boolean; setOpen: (open: boolean) => void };
  submitFailed: { attemptId: number; error: string };
  submitRequested: {
    execute: (data: FanOutTaskRunsRequest) => Promise<FanOutTaskRunsResponse | void>;
    setOpen: (open: boolean) => void;
  };
  submitSucceeded: {
    attemptId: number;
    result: FanOutTaskRunsResponse | null;
    setOpen: (open: boolean) => void;
  };
};

export function createTaskFanOutDialogLogic() {
  return createStoreLogic<FanOutDialogState, FanOutDialogEvents>({
    context: { ...initialState(), attemptId: 0 },
    on: {
      designationsChanged: (context, event) => ({
        ...context,
        designationsText: event.value,
        formError: null,
      }),
      networkParticipationChanged: (context, event) => ({
        ...context,
        formError: null,
        networkParticipation: event.value,
      }),
      worktreePerRunChanged: (context, event) => ({
        ...context,
        formError: null,
        worktreePerRun: event.value,
      }),
      openChanged: (context, event, enqueue) => {
        if (event.open) {
          enqueue.effect(() => event.setOpen(true));
          return;
        }
        return close(context, enqueue, event.setOpen);
      },
      submitFailed: (context, event) => {
        if (context.phase !== "submitting" || context.attemptId !== event.attemptId) return;
        return { ...context, formError: event.error, phase: "editing" };
      },
      submitRequested: (context, event, enqueue) => {
        if (context.phase === "submitting") return;
        const payload = toPayload(context);
        if (payload instanceof Error) return { ...context, formError: payload.message };
        const attemptId = context.attemptId + 1;
        enqueue.effect(async ({ trigger }) => {
          try {
            const result = (await event.execute(payload)) ?? null;
            trigger.submitSucceeded({ attemptId, result, setOpen: event.setOpen });
          } catch (error) {
            trigger.submitFailed({ attemptId, error: fanOutErrorMessage(error) });
          }
        });
        return { ...context, attemptId, formError: null, phase: "submitting" };
      },
      submitSucceeded: (context, event, enqueue) => {
        if (context.phase !== "submitting" || context.attemptId !== event.attemptId) return;
        if (context.worktreePerRun && event.result && event.result.runs.length > 0) {
          return { ...context, phase: "editing", result: event.result };
        }
        return close(context, enqueue, event.setOpen);
      },
    },
  });
}

function close(
  context: FanOutDialogState,
  enqueue: EnqueueObject<FanOutDialogState, never, FanOutDialogEvents>,
  setOpen: (open: boolean) => void
): FanOutDialogState {
  enqueue.effect(() => setOpen(false));
  return { ...initialState(), attemptId: context.attemptId };
}

function initialState(): Omit<FanOutDialogState, "attemptId"> {
  return {
    designationsText: "",
    formError: null,
    networkParticipation: { ...DEFAULT_NETWORK_PARTICIPATION_DRAFT },
    phase: "editing",
    result: null,
    worktreePerRun: false,
  };
}

function fanOutErrorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== ""
    ? error.message
    : "Failed to fan out task runs.";
}

function toPayload(context: FanOutDialogState): FanOutTaskRunsRequest | Error {
  const designations = context.designationsText
    .split(/\r?\n/u)
    .map(line => line.trim())
    .filter(Boolean)
    .map(brief => ({ brief }));
  if (designations.length === 0) return new Error("Add at least one assignment.");
  const error = networkParticipationValidationMessage(context.networkParticipation, strategies);
  if (error) return new Error(error);
  return {
    designations,
    network_participation: serializeNetworkParticipation(context.networkParticipation),
    // Sent only when requested: the daemon's default is shared-root execution,
    // and an explicit `false` would read as a policy the operator never set.
    ...(context.worktreePerRun ? { worktree_per_run: true } : {}),
  };
}
