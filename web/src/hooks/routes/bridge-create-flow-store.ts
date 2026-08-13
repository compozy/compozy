import { createStoreLogic } from "@xstate/store";

import type { EntityMode } from "@compozy/ui";

import { notifyUser } from "@/lib/user-feedback";

import {
  buildBridgeCreateRequest,
  getBridgeSetupProfile,
  isBridgeProviderSelectable,
  type BridgeCreateDraft,
  type BridgeProvider,
  type BridgeSecretRecoveryState,
  type CreateBridgeRequest,
  type CreateBridgeResponse,
} from "@/systems/bridges";

interface PersistedBridge {
  displayName: string;
  id: string;
}

export interface BridgeSecretBindingOutcome {
  bound: string[];
  clearedSlotNames: string[];
  failures: Record<string, string>;
}

export type BridgeCreateExecution = (request: CreateBridgeRequest) => Promise<CreateBridgeResponse>;
export type BridgeSecretBindingExecution = (
  bridgeId: string,
  provider: BridgeProvider,
  draft: BridgeCreateDraft,
  alreadyBound?: string[],
  expectedSlotNames?: readonly string[]
) => Promise<BridgeSecretBindingOutcome>;
export type BridgeNavigationExecution = (bridge: PersistedBridge) => Promise<void>;

export type BridgeCreateFlow =
  | { phase: "closed"; attempt: number }
  | {
      phase: "editing" | "failed";
      attempt: number;
      draft: BridgeCreateDraft;
      mode: EntityMode;
      error?: string;
    }
  | {
      phase: "creating";
      attempt: number;
      draft: BridgeCreateDraft;
      mode: EntityMode;
      provider: BridgeProvider;
    }
  | {
      phase: "retrying-secrets";
      attempt: number;
      bridge: PersistedBridge;
      draft: BridgeCreateDraft;
      mode: EntityMode;
      recovery: BridgeSecretRecoveryState;
    }
  | {
      phase: "secret-recovery";
      attempt: number;
      bridge: PersistedBridge;
      draft: BridgeCreateDraft;
      mode: EntityMode;
      recovery: BridgeSecretRecoveryState;
    }
  | { phase: "manifest"; attempt: number; bridge: PersistedBridge }
  | {
      phase: "navigating";
      attempt: number;
      bridge: PersistedBridge;
      fallback:
        | { kind: "closed" }
        | { kind: "manifest" }
        | {
            kind: "secret-recovery";
            draft: BridgeCreateDraft;
            mode: EntityMode;
            recovery: BridgeSecretRecoveryState;
          };
      successMessage: string | null;
    };

type BridgeCreateEvents = {
  createOpened: { draft: BridgeCreateDraft };
  draftChanged: { draft: BridgeCreateDraft };
  modeChanged: { mode: EntityMode };
  createSubmitted: {
    activeWorkspaceId?: string | null;
    bindSecrets: BridgeSecretBindingExecution;
    create: BridgeCreateExecution;
    navigate: BridgeNavigationExecution;
    provider?: BridgeProvider;
  };
  createCompleted: {
    attempt: number;
    bindOutcome: BridgeSecretBindingOutcome;
    navigate: BridgeNavigationExecution;
    result: CreateBridgeResponse;
  };
  createFailed: { attempt: number; error: string };
  secretRetrySubmitted: {
    bindSecrets: BridgeSecretBindingExecution;
    navigate: BridgeNavigationExecution;
  };
  secretRetryCompleted: {
    attempt: number;
    bindOutcome: BridgeSecretBindingOutcome;
    navigate: BridgeNavigationExecution;
  };
  secretRetryFailed: { attempt: number; error: string };
  navigationSucceeded: { attempt: number };
  navigationFailed: { attempt: number; error: string };
  dialogDismissed: { navigate: BridgeNavigationExecution };
  manifestOpened: { navigate: BridgeNavigationExecution };
};

export function createBridgeCreateFlowLogic() {
  return createStoreLogic<BridgeCreateFlow, BridgeCreateEvents>({
    context: { phase: "closed", attempt: 0 },
    on: {
      createOpened: (context, event) => ({
        attempt: context.attempt + 1,
        draft: event.draft,
        mode: "simple",
        phase: "editing",
      }),
      draftChanged: (context, event) => {
        if (
          context.phase !== "editing" &&
          context.phase !== "failed" &&
          context.phase !== "secret-recovery"
        ) {
          return;
        }
        return { ...context, draft: event.draft };
      },
      modeChanged: (context, event) => {
        if (
          context.phase !== "editing" &&
          context.phase !== "failed" &&
          context.phase !== "secret-recovery"
        ) {
          return;
        }
        return { ...context, mode: event.mode };
      },
      createSubmitted: (context, event, enqueue) => {
        if (context.phase !== "editing" && context.phase !== "failed") return;
        if (!event.provider || !isBridgeProviderSelectable(event.provider)) {
          return fail(
            context,
            "Select an available bridge provider before creating the bridge.",
            enqueue
          );
        }
        const request = buildBridgeCreateRequest(
          context.draft,
          event.provider,
          event.activeWorkspaceId
        );
        if (!request.ok) return fail(context, request.error, enqueue);

        const attempt = context.attempt + 1;
        const { draft } = context;
        const provider = event.provider;
        enqueue.effect(async ({ trigger }) => {
          try {
            const result = await event.create(request.data);
            const bindOutcome = await event.bindSecrets(result.bridge.id, provider, draft);
            trigger.createCompleted({ attempt, bindOutcome, navigate: event.navigate, result });
          } catch (error) {
            trigger.createFailed({
              attempt,
              error: errorMessage(error, "Failed to create bridge"),
            });
          }
        });
        return { attempt, draft, mode: context.mode, phase: "creating", provider };
      },
      createCompleted: (context, event, enqueue) => {
        if (context.phase !== "creating" || context.attempt !== event.attempt) return;
        const bridge = {
          displayName: event.result.bridge.display_name,
          id: event.result.bridge.id,
        };
        const draft = clearSecretSlots(context.draft, event.bindOutcome.clearedSlotNames);
        if (Object.keys(event.bindOutcome.failures).length > 0) {
          const recovery: BridgeSecretRecoveryState = {
            bridgeId: bridge.id,
            bound: event.bindOutcome.bound,
            failures: event.bindOutcome.failures,
            provider: context.provider,
          };
          enqueue.effect(() =>
            notifyUser({
              message: `Created bridge ${bridge.displayName}, but some credentials failed to bind.`,
              tone: "error",
            })
          );
          return {
            attempt: context.attempt,
            bridge,
            draft,
            mode: context.mode,
            phase: "secret-recovery",
            recovery,
          };
        }
        if (getBridgeSetupProfile(context.provider.platform)?.manifest === "slack") {
          enqueue.effect(() =>
            notifyUser({
              message: `Created bridge ${bridge.displayName}. Continue with its Slack manifest.`,
              tone: "success",
            })
          );
          return { attempt: context.attempt, bridge, phase: "manifest" };
        }
        enqueueNavigation(event.navigate, enqueue, context.attempt, bridge);
        return {
          attempt: context.attempt,
          bridge,
          fallback: { kind: "closed" },
          phase: "navigating",
          successMessage: `Created bridge ${bridge.displayName}.`,
        };
      },
      createFailed: (context, event, enqueue) => {
        if (context.phase !== "creating" || context.attempt !== event.attempt) return;
        return fail(context, event.error, enqueue);
      },
      secretRetrySubmitted: (context, event, enqueue) => {
        if (context.phase !== "secret-recovery") return;
        const attempt = context.attempt + 1;
        enqueue.effect(async ({ trigger }) => {
          try {
            trigger.secretRetryCompleted({
              attempt,
              bindOutcome: await event.bindSecrets(
                context.bridge.id,
                context.recovery.provider,
                context.draft,
                context.recovery.bound,
                Object.keys(context.recovery.failures)
              ),
              navigate: event.navigate,
            });
          } catch (error) {
            trigger.secretRetryFailed({
              attempt,
              error: errorMessage(error, "Failed to bind bridge credentials"),
            });
          }
        });
        return { ...context, attempt, phase: "retrying-secrets" };
      },
      secretRetryCompleted: (context, event, enqueue) => {
        if (context.phase !== "retrying-secrets" || context.attempt !== event.attempt) return;
        const draft = clearSecretSlots(context.draft, event.bindOutcome.clearedSlotNames);
        if (Object.keys(event.bindOutcome.failures).length > 0) {
          return {
            ...context,
            draft,
            phase: "secret-recovery",
            recovery: {
              ...context.recovery,
              bound: event.bindOutcome.bound,
              failures: event.bindOutcome.failures,
            },
          };
        }
        enqueue.effect(() =>
          notifyUser({ message: "Bound the remaining bridge credentials.", tone: "success" })
        );
        enqueueNavigation(event.navigate, enqueue, context.attempt, context.bridge);
        return {
          attempt: context.attempt,
          bridge: context.bridge,
          fallback: { kind: "closed" },
          phase: "navigating",
          successMessage: null,
        };
      },
      secretRetryFailed: (context, event, enqueue) => {
        if (context.phase !== "retrying-secrets" || context.attempt !== event.attempt) return;
        enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
        return { ...context, phase: "secret-recovery" };
      },
      dialogDismissed: (context, event, enqueue) => {
        if (
          context.phase === "creating" ||
          context.phase === "retrying-secrets" ||
          context.phase === "navigating"
        ) {
          return;
        }
        if (context.phase === "manifest") {
          return startNavigation(context, event.navigate, enqueue, { kind: "manifest" });
        }
        if (context.phase === "secret-recovery") {
          return startNavigation(context, event.navigate, enqueue, {
            kind: "secret-recovery",
            draft: context.draft,
            mode: context.mode,
            recovery: context.recovery,
          });
        }
        return { attempt: context.attempt + 1, phase: "closed" };
      },
      manifestOpened: (context, event, enqueue) => {
        if (context.phase !== "manifest") return;
        return startNavigation(context, event.navigate, enqueue, { kind: "manifest" });
      },
      navigationSucceeded: (context, event, enqueue) => {
        if (context.phase !== "navigating" || context.attempt !== event.attempt) return;
        if (context.successMessage) {
          enqueue.effect(() =>
            notifyUser({ message: context.successMessage ?? "", tone: "success" })
          );
        }
        return { attempt: context.attempt, phase: "closed" };
      },
      navigationFailed: (context, event, enqueue) => {
        if (context.phase !== "navigating" || context.attempt !== event.attempt) return;
        if (context.fallback.kind === "closed") {
          enqueue.effect(() =>
            notifyUser({
              message: `Created bridge ${context.bridge.displayName}, but it could not be opened.`,
              tone: "error",
            })
          );
          return { attempt: context.attempt, phase: "closed" };
        }
        enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
        if (context.fallback.kind === "manifest") {
          return { attempt: context.attempt, bridge: context.bridge, phase: "manifest" };
        }
        if (context.fallback.kind === "secret-recovery") {
          return {
            attempt: context.attempt,
            bridge: context.bridge,
            draft: context.fallback.draft,
            mode: context.fallback.mode,
            phase: "secret-recovery",
            recovery: context.fallback.recovery,
          };
        }
      },
    },
  });
}

function clearSecretSlots(
  draft: BridgeCreateDraft,
  slotNames: readonly string[]
): BridgeCreateDraft {
  if (slotNames.length === 0) return draft;
  const secretSlotValues = { ...draft.secretSlotValues };
  for (const name of slotNames) secretSlotValues[name] = "";
  return { ...draft, secretSlotValues };
}

function fail(
  flow: Extract<BridgeCreateFlow, { phase: "editing" | "failed" | "creating" }>,
  error: string,
  enqueue: { effect: (effect: () => void) => void }
): BridgeCreateFlow {
  enqueue.effect(() => notifyUser({ message: error, tone: "error" }));
  return {
    attempt: flow.attempt,
    draft: flow.draft,
    error,
    mode: flow.mode,
    phase: "failed",
  };
}

function startNavigation(
  context: Extract<BridgeCreateFlow, { phase: "manifest" | "secret-recovery" }>,
  navigate: BridgeNavigationExecution,
  enqueue: BridgeCreateFlowEnqueue,
  fallback: Extract<BridgeCreateFlow, { phase: "navigating" }>["fallback"]
): Extract<BridgeCreateFlow, { phase: "navigating" }> {
  const attempt = context.attempt + 1;
  enqueueNavigation(navigate, enqueue, attempt, context.bridge);
  return { attempt, bridge: context.bridge, fallback, phase: "navigating", successMessage: null };
}

interface BridgeCreateFlowTrigger {
  navigationSucceeded: (event: { attempt: number }) => void;
  navigationFailed: (event: { attempt: number; error: string }) => void;
}

interface BridgeCreateFlowEnqueue {
  effect: (effect: (scope: { trigger: BridgeCreateFlowTrigger }) => void | Promise<void>) => void;
}

function enqueueNavigation(
  navigate: BridgeNavigationExecution,
  enqueue: BridgeCreateFlowEnqueue,
  attempt: number,
  bridge: PersistedBridge
): void {
  enqueue.effect(async ({ trigger }) => {
    try {
      await navigate(bridge);
      trigger.navigationSucceeded({ attempt });
    } catch (error) {
      trigger.navigationFailed({ attempt, error: errorMessage(error, "Failed to open bridge") });
    }
  });
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
