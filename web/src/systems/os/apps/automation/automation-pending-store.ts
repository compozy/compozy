import { createStoreLogic } from "@xstate/store";

interface AutomationPendingState {
  pendingIds: ReadonlySet<string>;
}

type AutomationPendingEvents = {
  /** Runs `run` once per id; a second request while that id is pending is ignored. */
  actionRequested: {
    id: string;
    permitted: boolean;
    run: () => Promise<void>;
  };
  actionSettled: { id: string };
};

/** Per-row in-flight daemon actions (Run now, On/Off); never flips state optimistically. */
export const automationPendingLogic = createStoreLogic<
  AutomationPendingState,
  AutomationPendingEvents
>({
  context: { pendingIds: new Set<string>() },
  on: {
    actionRequested: (context, event, enqueue) => {
      if (!event.permitted || context.pendingIds.has(event.id)) return;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.run();
        } finally {
          trigger.actionSettled({ id: event.id });
        }
      });
      return { pendingIds: new Set([...context.pendingIds, event.id]) };
    },
    actionSettled: (context, event) => {
      if (!context.pendingIds.has(event.id)) return;
      const pendingIds = new Set(context.pendingIds);
      pendingIds.delete(event.id);
      return { pendingIds };
    },
  },
});
