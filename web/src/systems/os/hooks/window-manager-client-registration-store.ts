import { createStoreLogic } from "@xstate/store";

import type { WindowManagerRegisteredClientView } from "../lib/window-manager-types";

export type WindowManagerRegistrationPhase =
  | "idle"
  | "registering"
  | "registered"
  | "waiting-retry"
  | "retry-paused";

/**
 * What this page showed when its registration was lost (a daemon restart forgets client views):
 * the re-registration asks for the desktop again and then refocuses the window (#1655). Kept in
 * memory only, for one (workspace, profile) binding; the first registration has none.
 */
export interface WindowManagerClientRestore {
  /** The desktop hint the next registration sends; `null` once the daemon refused it. */
  activeDesktopId: string | null;
  focusedWindowId: string | null;
}

interface WindowManagerRegistrationBase {
  clientId: string;
  documentVisible: boolean;
  epoch: number;
  retryCount: number;
  restore: WindowManagerClientRestore | null;
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
      error: Error | null;
      phase: "registering";
      workspaceId: string;
    })
  | (WindowManagerRegistrationBase & {
      client: WindowManagerRegisteredClientView;
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
  recoveryRequested: { restore?: WindowManagerClientRestore | null };
  registrationFailed: { error: Error; epoch: number };
  /** The daemon refused the desktop hint (that desktop is gone): register again without it. */
  desktopHintRefused: { epoch: number };
  /** The restored focus of `windowId` was applied or is moot; a newer restore stays. */
  restoreSettled: { windowId: string };
  registrationSucceeded: { client: WindowManagerRegisteredClientView; epoch: number };
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
          restore: null,
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
          restore: null,
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
          phase: "registering",
          retryCount: context.retryCount + 1,
        };
      }
      return { ...context, documentVisible: event.visible };
    },
    recoveryRequested: (context, event) => {
      if (context.workspaceId === null) return;
      return {
        ...context,
        client: null,
        epoch: context.epoch + 1,
        phase: "registering",
        // A restore still pending (a second loss before the first settled) outlives the view the
        // fallback registration showed: that view is the daemon's default, not this page's.
        restore: context.restore ?? event.restore ?? null,
        retryCount: 0,
      };
    },
    desktopHintRefused: (context, event) => {
      if (context.phase !== "registering" || context.epoch !== event.epoch) return;
      if (context.restore === null) return;
      return {
        ...context,
        epoch: context.epoch + 1,
        restore: { ...context.restore, activeDesktopId: null },
      };
    },
    restoreSettled: (context, event) => {
      if (context.restore?.focusedWindowId !== event.windowId) return;
      return { ...context, restore: null };
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
        phase: "registering",
        retryCount: context.retryCount + 1,
      };
    },
  },
});

export function windowManagerRetryDelay(retryCount: number): number {
  return Math.min(8_000, 500 * 2 ** Math.min(retryCount, 4));
}
