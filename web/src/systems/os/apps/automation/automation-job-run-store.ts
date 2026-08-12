import { createStoreLogic } from "@xstate/store";

interface AutomationJobRunState {
  pendingIds: ReadonlySet<string>;
}

type AutomationJobRunEvents = {
  runRequested: { id: string };
  runSettled: { id: string };
};

export const automationJobRunLogic = createStoreLogic<
  AutomationJobRunState,
  AutomationJobRunEvents
>({
  context: { pendingIds: new Set<string>() },
  on: {
    runRequested: (context, event) =>
      context.pendingIds.has(event.id)
        ? undefined
        : { pendingIds: new Set([...context.pendingIds, event.id]) },
    runSettled: (context, event) => {
      if (!context.pendingIds.has(event.id)) return;
      const pendingIds = new Set(context.pendingIds);
      pendingIds.delete(event.id);
      return { pendingIds };
    },
  },
});
