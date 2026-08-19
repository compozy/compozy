import { createStoreLogic } from "@xstate/store";

import type { WindowManagerAttachedClientView } from "../lib/window-manager-types";

export type WindowManagerRegistrationPhase =
  | "idle"
  | "registering"
  | "registered"
  | "waiting-retry"
  | "retry-paused";

interface WindowManagerRegistrationBase {
  clientId: string;
  documentVisible: boolean;
  epoch: number;
  retryCount: number;
}

export type WindowManagerRegistrationContext =
  | (WindowManagerRegistrationBase & {
      client: null;
      error: null;
      phase: "idle";
      retryCount: 0;
      workspaceId: null;
    })
  | (WindowManagerRegistrationBase & {
      client: null;
      error: null;
      phase: "registering";
      workspaceId: string;
    })
  | (WindowManagerRegistrationBase & {
      client: WindowManagerAttachedClientView;
      error: null;
      phase: "registered";
      retryCount: 0;
      workspaceId: string;
    })
  | (WindowManagerRegistrationBase & {
      client: null;
      error: Error;
      phase: "waiting-retry" | "retry-paused";
      workspaceId: string;
    });

type WindowManagerRegistrationEvents = {
  documentVisibilityChanged: { visible: boolean };
  recoveryRequested: Record<never, never>;
  registrationFailed: { error: Error; epoch: number };
  registrationSucceeded: { client: WindowManagerAttachedClientView; epoch: number };
  retryElapsed: Record<never, never>;
};

interface WindowManagerRegistrationInput {
  clientId: string;
  documentVisible: boolean;
  workspaceId: string | null;
}

export const windowManagerClientRegistrationLogic = createStoreLogic<
  WindowManagerRegistrationContext,
  WindowManagerRegistrationEvents,
  never,
  WindowManagerRegistrationInput
>({
  context: input =>
    input.workspaceId === null
      ? {
          client: null,
          clientId: input.clientId,
          documentVisible: input.documentVisible,
          epoch: 0,
          error: null,
          phase: "idle",
          retryCount: 0,
          workspaceId: null,
        }
      : {
          client: null,
          clientId: input.clientId,
          documentVisible: input.documentVisible,
          epoch: 0,
          error: null,
          phase: "registering",
          retryCount: 0,
          workspaceId: input.workspaceId,
        },
  on: {
    documentVisibilityChanged: (context, event) => {
      if (context.documentVisible === event.visible) return context;
      if (!event.visible && context.phase === "waiting-retry") {
        return { ...context, documentVisible: false, phase: "retry-paused" };
      }
      if (event.visible && context.phase === "retry-paused" && context.workspaceId !== null) {
        return {
          ...context,
          documentVisible: true,
          epoch: context.epoch + 1,
          error: null,
          phase: "registering",
          retryCount: context.retryCount + 1,
        };
      }
      return { ...context, documentVisible: event.visible };
    },
    recoveryRequested: context => {
      if (context.workspaceId === null) return;
      return {
        ...context,
        client: null,
        epoch: context.epoch + 1,
        error: null,
        phase: "registering",
        retryCount: 0,
      };
    },
    registrationFailed: (context, event) => {
      if (context.phase !== "registering" || context.epoch !== event.epoch) return;
      return {
        ...context,
        client: null,
        error: event.error,
        phase: context.documentVisible ? "waiting-retry" : "retry-paused",
      };
    },
    registrationSucceeded: (context, event) => {
      if (
        context.phase !== "registering" ||
        context.epoch !== event.epoch ||
        event.client.workspaceId !== context.workspaceId ||
        event.client.clientId !== context.clientId
      ) {
        return;
      }
      return {
        ...context,
        client: event.client,
        error: null,
        phase: "registered",
        retryCount: 0,
      };
    },
    retryElapsed: context => {
      if (context.phase !== "waiting-retry" || context.workspaceId === null) return;
      return {
        ...context,
        epoch: context.epoch + 1,
        error: null,
        phase: "registering",
        retryCount: context.retryCount + 1,
      };
    },
  },
});

export function windowManagerRetryDelay(retryCount: number): number {
  return Math.min(8_000, 500 * 2 ** Math.min(retryCount, 4));
}
