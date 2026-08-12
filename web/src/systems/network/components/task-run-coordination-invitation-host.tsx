import { useSelector, useStore } from "@xstate/store-react";

import { NetworkCoordinationInvitation } from "./coordination-invitation";
import { coordinationInvitationLogic } from "./coordination-invitation-store";
import {
  useAcceptNetworkCoordinationInvitation,
  useDismissNetworkCoordinationInvitation,
  useNetworkCoordination,
} from "../hooks/use-network-coordination";

export interface TaskRunCoordinationInvitationHostProps {
  taskId: string;
  runId: string;
  workspaceId: string;
}

export function TaskRunCoordinationInvitationHost({
  taskId,
  runId,
  workspaceId,
}: TaskRunCoordinationInvitationHostProps) {
  const actionStore = useStore(coordinationInvitationLogic);
  const actionPhase = useSelector(actionStore, snapshot => snapshot.context.phase);
  const ref = { scope: "task" as const, taskId, runId };
  const coordination = useNetworkCoordination(workspaceId, ref);
  const accept = useAcceptNetworkCoordinationInvitation(workspaceId, ref);
  const dismiss = useDismissNetworkCoordinationInvitation(workspaceId, ref);

  const revision = coordination.data?.revision;
  const visible = Boolean(coordination.data?.eligibility.eligible);
  const pending = actionPhase === "submitting" || accept.isPending || dismiss.isPending;
  const unlock = () => {
    actionStore.trigger.actionSettled();
  };

  return (
    <NetworkCoordinationInvitation
      accepting={accept.isPending}
      dismissing={dismiss.isPending}
      errorMessage={accept.error?.message ?? dismiss.error?.message}
      onAccept={() => {
        if (
          actionStore.getSnapshot().context.phase === "submitting" ||
          pending ||
          revision === undefined
        ) {
          return;
        }
        actionStore.trigger.actionStarted();
        accept.mutate(revision, { onSettled: unlock });
      }}
      onDismiss={() => {
        if (
          actionStore.getSnapshot().context.phase === "submitting" ||
          pending ||
          revision === undefined
        ) {
          return;
        }
        actionStore.trigger.actionStarted();
        dismiss.mutate(revision, { onSettled: unlock });
      }}
      visible={visible}
    />
  );
}
