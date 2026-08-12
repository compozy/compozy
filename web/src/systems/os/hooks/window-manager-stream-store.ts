import { createStoreLogic } from "@xstate/store";

import type { LayoutRevision } from "../lib/window-manager-types";

interface WindowManagerStreamState {
  bindingKey: string | null;
  presentationRevision: number;
  reconnectAttempt: number;
  reconnectEpoch: number;
  topologyRevision: LayoutRevision;
}

type WindowManagerStreamEvents = {
  bindingObserved: {
    bindingKey: string | null;
    presentationRevision: number;
    topologyRevision: LayoutRevision;
  };
  disabled: Record<never, never>;
  presentationObserved: { revision: number };
  reconnectElapsed: { attempt: number };
  snapshotObserved: { revision: LayoutRevision };
  topologyObserved: { revision: LayoutRevision };
};

export const windowManagerStreamLogic = createStoreLogic<
  WindowManagerStreamState,
  WindowManagerStreamEvents,
  never,
  { topologyRevision: LayoutRevision }
>({
  context: input => ({
    bindingKey: null,
    presentationRevision: 0,
    reconnectAttempt: 0,
    reconnectEpoch: 0,
    topologyRevision: input.topologyRevision,
  }),
  on: {
    bindingObserved: (context, event) =>
      context.bindingKey === event.bindingKey
        ? {
            ...context,
            presentationRevision: Math.max(
              context.presentationRevision,
              event.presentationRevision
            ),
            topologyRevision: Math.max(context.topologyRevision, event.topologyRevision),
          }
        : {
            ...context,
            bindingKey: event.bindingKey,
            presentationRevision: event.presentationRevision,
            reconnectAttempt: 0,
            topologyRevision: event.topologyRevision,
          },
    disabled: context =>
      context.reconnectAttempt === 0 ? undefined : { ...context, reconnectAttempt: 0 },
    presentationObserved: (context, event) =>
      event.revision <= context.presentationRevision
        ? undefined
        : { ...context, presentationRevision: event.revision },
    reconnectElapsed: (context, event) => {
      if (event.attempt !== context.reconnectAttempt) return;
      return {
        ...context,
        reconnectAttempt: event.attempt + 1,
        reconnectEpoch: context.reconnectEpoch + 1,
      };
    },
    snapshotObserved: (context, event) => ({
      ...context,
      reconnectAttempt: 0,
      topologyRevision: Math.max(context.topologyRevision, event.revision),
    }),
    topologyObserved: (context, event) =>
      event.revision <= context.topologyRevision
        ? undefined
        : { ...context, topologyRevision: event.revision },
  },
});
