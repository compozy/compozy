import { createStoreLogic } from "@xstate/store";

interface CoordinationInvitationState {
  phase: "idle" | "submitting";
}

type CoordinationInvitationEvents = {
  actionSettled: Record<never, never>;
  actionStarted: Record<never, never>;
};

export const coordinationInvitationLogic = createStoreLogic<
  CoordinationInvitationState,
  CoordinationInvitationEvents
>({
  context: { phase: "idle" },
  on: {
    actionStarted: context =>
      context.phase === "submitting" ? undefined : { phase: "submitting" },
    actionSettled: context => (context.phase === "idle" ? undefined : { phase: "idle" }),
  },
});
