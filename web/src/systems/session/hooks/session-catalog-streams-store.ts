import { createStoreLogic, type EnqueueObject } from "@xstate/store";

export type SessionCatalogStreamStatus = "disabled" | "connecting" | "live" | "stale";

interface SessionCatalogStreamsState {
  generation: number;
  status: SessionCatalogStreamStatus;
}

type SessionCatalogStreamsEventPayloadMap = {
  connectionRequested: { connect: SessionCatalogStreamConnector };
  connectionDisabled: {};
  connectionLive: { generation: number };
  connectionStale: { generation: number };
};

export type SessionCatalogStreamConnector = (
  onStatusChange: (status: Exclude<SessionCatalogStreamStatus, "disabled">) => void
) => () => void;

type SessionCatalogStreamsEnqueue = EnqueueObject<
  SessionCatalogStreamsState,
  never,
  SessionCatalogStreamsEventPayloadMap
>;

interface SessionCatalogStreamsTrigger {
  connectionLive: (event: { generation: number }) => void;
  connectionStale: (event: { generation: number }) => void;
}

const activeConnections = new WeakMap<object, () => void>();

export const sessionCatalogStreamsLogic = createStoreLogic<
  SessionCatalogStreamsState,
  SessionCatalogStreamsEventPayloadMap
>({
  context: { generation: 0, status: "disabled" },
  on: {
    connectionRequested: (context, event, enqueue) => {
      const generation = context.generation + 1;
      enqueueConnection(event.connect, enqueue, generation);
      return { generation, status: "connecting" };
    },
    connectionDisabled: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => stopConnection(activeConnections, trigger));
      return { generation: context.generation + 1, status: "disabled" };
    },
    connectionLive: (context, event) => {
      if (event.generation !== context.generation) return;
      return { ...context, status: "live" };
    },
    connectionStale: (context, event) => {
      if (event.generation !== context.generation) return;
      return { ...context, status: "stale" };
    },
  },
});

function enqueueConnection(
  connect: SessionCatalogStreamConnector,
  enqueue: SessionCatalogStreamsEnqueue,
  generation: number
) {
  enqueue.effect(({ trigger }) => {
    stopConnection(activeConnections, trigger);
    try {
      const close = connect(status => {
        if (status === "live") trigger.connectionLive({ generation });
        else trigger.connectionStale({ generation });
      });
      activeConnections.set(trigger, close);
    } catch {
      trigger.connectionStale({ generation });
    }
  });
}

function stopConnection(
  connections: WeakMap<object, () => void>,
  trigger: SessionCatalogStreamsTrigger
) {
  const close = connections.get(trigger);
  if (!close) return;
  connections.delete(trigger);
  close();
}
