import { createStoreLogic, type EnqueueObject } from "@xstate/store";

export type WorktreeCatalogStreamStatus = "disabled" | "connecting" | "live" | "stale";

interface WorktreeCatalogStreamState {
  generation: number;
  status: WorktreeCatalogStreamStatus;
}

type WorktreeCatalogStreamEventPayloadMap = {
  connectionRequested: { connect: WorktreeCatalogStreamConnector };
  connectionDisabled: {};
  connectionLive: { generation: number };
  connectionStale: { generation: number };
};

export type WorktreeCatalogStreamConnector = (
  onStatusChange: (status: Exclude<WorktreeCatalogStreamStatus, "disabled">) => void
) => () => void;

type WorktreeCatalogStreamEnqueue = EnqueueObject<
  WorktreeCatalogStreamState,
  never,
  WorktreeCatalogStreamEventPayloadMap
>;

interface WorktreeCatalogStreamTrigger {
  connectionLive: (event: { generation: number }) => void;
  connectionStale: (event: { generation: number }) => void;
}

const activeConnections = new WeakMap<object, () => void>();

/**
 * Generation-fenced single-owner connection, mirroring the session catalog
 * stream: a status callback from a superseded connection is dropped instead of
 * resurrecting a closed stream's state.
 */
export const worktreeCatalogStreamLogic = createStoreLogic<
  WorktreeCatalogStreamState,
  WorktreeCatalogStreamEventPayloadMap
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
  connect: WorktreeCatalogStreamConnector,
  enqueue: WorktreeCatalogStreamEnqueue,
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
  trigger: WorktreeCatalogStreamTrigger
) {
  const close = connections.get(trigger);
  if (!close) return;
  connections.delete(trigger);
  close();
}
