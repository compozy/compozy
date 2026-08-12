import { createStoreLogic } from "@xstate/store";

interface SessionCommandOpenState {
  open: boolean;
}

type SessionCommandOpenEvents = {
  observed: { onOpen?: () => void; open: boolean };
};

export const sessionCommandOpenLogic = createStoreLogic<
  SessionCommandOpenState,
  SessionCommandOpenEvents
>({
  context: { open: false },
  on: {
    observed: (context, event, enqueue) => {
      if (event.open && !context.open && event.onOpen) {
        enqueue.effect(() => {
          try {
            event.onOpen?.();
          } catch (error) {
            console.error("Failed to report an opened session command catalog", error);
          }
        });
      }
      return { open: event.open };
    },
  },
});
