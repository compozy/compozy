import { createStoreLogic } from "@xstate/store";

interface CoordinationInvitationState {
  phase: "idle" | "submitting";
}

type CoordinationInvitationEvents = {
  actionSettled: Record<never, never>;
  actionRequested: {
    execute: (onSettled: () => void) => void;
    permitted: boolean;
  };
};

export const coordinationInvitationLogic = createStoreLogic<
  CoordinationInvitationState,
  CoordinationInvitationEvents
>({
  context: { phase: "idle" },
  on: {
    actionRequested: (context, event, enqueue) => {
      if (!event.permitted || context.phase === "submitting") return;
      enqueue.effect(({ trigger }) => event.execute(() => trigger.actionSettled()));
      return { phase: "submitting" };
    },
    actionSettled: context => (context.phase === "idle" ? undefined : { phase: "idle" }),
  },
});
