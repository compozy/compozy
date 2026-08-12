import { createStoreLogic } from "@xstate/store";

interface AutomationJobRunState {
  pendingIds: ReadonlySet<string>;
}

type AutomationJobRunEvents = {
  runRequested: {
    execute: (id: string) => Promise<{ id: string }>;
    onFailure: (error: unknown) => void;
    onSuccess: (run: { id: string }) => void;
    id: string;
    permitted: boolean;
  };
  runSettled: { id: string };
};

export const automationJobRunLogic = createStoreLogic<
  AutomationJobRunState,
  AutomationJobRunEvents
>({
  context: { pendingIds: new Set<string>() },
  on: {
    runRequested: (context, event, enqueue) => {
      if (!event.permitted || context.pendingIds.has(event.id)) return;
      enqueue.effect(async ({ trigger }) => {
        try {
          event.onSuccess(await event.execute(event.id));
        } catch (error) {
          event.onFailure(error);
        } finally {
          trigger.runSettled({ id: event.id });
        }
      });
      return { pendingIds: new Set([...context.pendingIds, event.id]) };
    },
    runSettled: (context, event) => {
      if (!context.pendingIds.has(event.id)) return;
      const pendingIds = new Set(context.pendingIds);
      pendingIds.delete(event.id);
      return { pendingIds };
    },
  },
});
