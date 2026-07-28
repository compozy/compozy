import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import {
  createBridgeTestDeliveryDraft,
  type BridgeSummary,
  type BridgeTestDeliveryDraft,
  type SendBridgeTestResponse,
  type TestBridgeDeliveryResponse,
} from "@/systems/bridges";

type DeliveryOperation<TResult> =
  | { phase: "idle"; requestId: number }
  | { phase: "submitting"; requestId: number }
  | { phase: "resolved"; requestId: number; result: TResult }
  | { phase: "failed"; requestId: number; error: string };

export interface BridgeDeliveryTestsState {
  bridge: BridgeSummary | undefined;
  draft: BridgeTestDeliveryDraft;
  dryRun: DeliveryOperation<TestBridgeDeliveryResponse>;
  sendTest: DeliveryOperation<SendBridgeTestResponse>;
}

type BridgeDeliveryEventPayloadMap = {
  reset: { bridge?: BridgeSummary };
  draftChanged: { draft: BridgeTestDeliveryDraft };
  dryRunSubmitted: {
    execute: (
      bridge: BridgeSummary,
      draft: BridgeTestDeliveryDraft
    ) => Promise<TestBridgeDeliveryResponse>;
  };
  sendTestSubmitted: {
    bridge: BridgeSummary;
    execute: (
      bridge: BridgeSummary,
      draft: BridgeTestDeliveryDraft
    ) => Promise<SendBridgeTestResponse>;
  };
  dryRunResolved: { requestId: number; result: TestBridgeDeliveryResponse };
  sendTestResolved: { requestId: number; result: SendBridgeTestResponse };
  dryRunFailed: { requestId: number; error: string };
  sendTestFailed: { requestId: number; error: string };
};

export const bridgeDeliveryTestsLogic = createStoreLogic<
  BridgeDeliveryTestsState,
  BridgeDeliveryEventPayloadMap
>({
  context: {
    bridge: undefined,
    draft: createBridgeTestDeliveryDraft(),
    dryRun: idleOperation<TestBridgeDeliveryResponse>(),
    sendTest: idleOperation<SendBridgeTestResponse>(),
  },
  on: {
    reset: (context, event) => ({
      bridge: event.bridge,
      draft: createBridgeTestDeliveryDraft(event.bridge),
      dryRun: idleOperation<TestBridgeDeliveryResponse>(context.dryRun.requestId + 1),
      sendTest: idleOperation<SendBridgeTestResponse>(context.sendTest.requestId + 1),
    }),
    draftChanged: (context, event) => ({
      ...context,
      draft: event.draft,
    }),
    dryRunSubmitted: (context, event, enqueue) => {
      if (!context.bridge || context.dryRun.phase === "submitting") return;
      const requestId = context.dryRun.requestId + 1;
      const { bridge, draft } = context;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.dryRunResolved({ requestId, result: await event.execute(bridge, draft) });
        } catch (error) {
          trigger.dryRunFailed({
            requestId,
            error: errorMessage(error, "Failed to resolve bridge target"),
          });
        }
      });
      return { ...context, dryRun: { phase: "submitting", requestId } };
    },
    sendTestSubmitted: (context, event, enqueue) => {
      if (
        !event.bridge.enabled ||
        context.draft.message.trim() === "" ||
        context.sendTest.phase === "submitting"
      ) {
        return;
      }
      const requestId = context.sendTest.requestId + 1;
      const { draft } = context;
      const bridge = event.bridge;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.sendTestResolved({ requestId, result: await event.execute(bridge, draft) });
        } catch (error) {
          trigger.sendTestFailed({
            requestId,
            error: errorMessage(error, "Failed to send bridge test message"),
          });
        }
      });
      return { ...context, bridge, sendTest: { phase: "submitting", requestId } };
    },
    dryRunResolved: (context, event, enqueue) => {
      if (context.dryRun.phase !== "submitting" || context.dryRun.requestId !== event.requestId) {
        return;
      }
      enqueue.effect(() =>
        notifyUser({
          message: `Resolved delivery target for ${context.bridge?.display_name ?? "bridge"}.`,
          tone: "success",
        })
      );
      return {
        ...context,
        dryRun: { phase: "resolved", requestId: event.requestId, result: event.result },
      };
    },
    sendTestResolved: (context, event, enqueue) => {
      if (
        context.sendTest.phase !== "submitting" ||
        context.sendTest.requestId !== event.requestId
      ) {
        return;
      }
      enqueue.effect(() =>
        notifyUser({
          message: `Sent a test message through ${context.bridge?.display_name ?? "bridge"}.`,
          tone: "success",
        })
      );
      return {
        ...context,
        sendTest: { phase: "resolved", requestId: event.requestId, result: event.result },
      };
    },
    dryRunFailed: (context, event, enqueue) => {
      if (context.dryRun.phase !== "submitting" || context.dryRun.requestId !== event.requestId) {
        return;
      }
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
      return {
        ...context,
        dryRun: { phase: "failed", requestId: event.requestId, error: event.error },
      };
    },
    sendTestFailed: (context, event, enqueue) => {
      if (
        context.sendTest.phase !== "submitting" ||
        context.sendTest.requestId !== event.requestId
      ) {
        return;
      }
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
      return {
        ...context,
        sendTest: { phase: "failed", requestId: event.requestId, error: event.error },
      };
    },
  },
});

function idleOperation<TResult>(requestId = 0): DeliveryOperation<TResult> {
  return { phase: "idle", requestId };
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
