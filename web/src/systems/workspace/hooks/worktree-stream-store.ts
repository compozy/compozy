import { createStoreLogic, type EnqueueObject } from "@xstate/store";

export type WorktreeStreamStatus = "disabled" | "connecting" | "live" | "stale";

interface WorktreeStreamState {
  generation: number;
  status: WorktreeStreamStatus;
}

type WorktreeStreamEventPayloadMap = {
  connectionRequested: { connect: WorktreeStreamConnector };
  connectionDisabled: {};
  connectionLive: { generation: number };
  connectionStale: { generation: number };
};

export type WorktreeStreamConnector = (
  onStatusChange: (status: Exclude<WorktreeStreamStatus, "disabled">) => void
) => () => void;

type WorktreeStreamEnqueue = EnqueueObject<
  WorktreeStreamState,
  never,
  WorktreeStreamEventPayloadMap
>;

interface WorktreeStreamTrigger {
  connectionLive: (event: { generation: number }) => void;
  connectionStale: (event: { generation: number }) => void;
}

const activeConnections = new WeakMap<object, () => void>();

/**
 * Generation-fenced single-owner connection for one worktree's event stream.
 * Switching the observed worktree supersedes the previous connection, and a
 * status callback from a superseded stream is dropped instead of resurrecting it.
 */
export const worktreeStreamLogic = createStoreLogic<
  WorktreeStreamState,
  WorktreeStreamEventPayloadMap
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
  connect: WorktreeStreamConnector,
  enqueue: WorktreeStreamEnqueue,
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

function stopConnection(connections: WeakMap<object, () => void>, trigger: WorktreeStreamTrigger) {
  const close = connections.get(trigger);
  if (!close) return;
  connections.delete(trigger);
  close();
}
