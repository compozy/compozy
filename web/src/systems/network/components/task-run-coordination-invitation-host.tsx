import { useRef } from "react";

import { NetworkCoordinationInvitation } from "./coordination-invitation";
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
  const actionLocked = useRef(false);
  const ref = { scope: "task" as const, taskId, runId };
  const coordination = useNetworkCoordination(workspaceId, ref);
  const accept = useAcceptNetworkCoordinationInvitation(workspaceId, ref);
  const dismiss = useDismissNetworkCoordinationInvitation(workspaceId, ref);

  const revision = coordination.data?.revision;
  const visible = Boolean(coordination.data?.eligibility.eligible);
  const pending = accept.isPending || dismiss.isPending;
  const unlock = () => {
    actionLocked.current = false;
  };

  return (
    <NetworkCoordinationInvitation
      accepting={accept.isPending}
      dismissing={dismiss.isPending}
      errorMessage={accept.error?.message ?? dismiss.error?.message}
      onAccept={() => {
        if (actionLocked.current || pending || revision === undefined) return;
        actionLocked.current = true;
        accept.mutate(revision, { onSettled: unlock });
      }}
      onDismiss={() => {
        if (actionLocked.current || pending || revision === undefined) return;
        actionLocked.current = true;
        dismiss.mutate(revision, { onSettled: unlock });
      }}
      visible={visible}
    />
  );
}
