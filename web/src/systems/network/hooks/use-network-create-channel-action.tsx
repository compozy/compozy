import { useLayoutEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector } from "@xstate/store-react";
import { Plus } from "lucide-react";

import { Button } from "@compozy/ui";

import { useStoreBinding } from "@/hooks/use-store-binding";

import { NetworkCreateChannelDialog } from "../components/network-create-channel-dialog";
import { sortAgentsForNetwork } from "../lib/network-formatters";
import { networkCreateChannelLogic } from "./network-create-channel-store";
import { useCreateNetworkChannel } from "./use-network-actions";
import { useAgents } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

export function useNetworkCreateChannelAction({
  enabled,
  workspaceId,
}: {
  enabled: boolean;
  workspaceId?: string | null;
}) {
  const navigate = useNavigate();
  const { activeWorkspace, activeWorkspaceId, workspaces } = useActiveWorkspace();
  const requestedWorkspaceId = workspaceId ?? activeWorkspaceId;
  const requestedWorkspace =
    (workspaces ?? []).find(candidate => candidate.id === requestedWorkspaceId) ??
    (activeWorkspace?.id === requestedWorkspaceId ? activeWorkspace : undefined);
  const agentsQuery = useAgents(requestedWorkspaceId);
  const createChannel = useCreateNetworkChannel({ workspaceId: requestedWorkspaceId });
  const { store } = useStoreBinding(requestedWorkspaceId, () =>
    networkCreateChannelLogic.createStore()
  );
  const createDraft = useSelector(store, snapshot => snapshot.context.draft);
  const createOpen = useSelector(store, snapshot => snapshot.context.open);
  const dialogWorkspaceId = useSelector(store, snapshot => snapshot.context.dialogWorkspaceId);
  const pendingRequest = useSelector(store, snapshot => snapshot.context.pendingRequest);

  useLayoutEffect(
    () => () => {
      store.trigger.lifecycleDisposed();
    },
    [store]
  );
  const dialogIsCurrentWorkspace = dialogWorkspaceId === requestedWorkspaceId;
  const sortedAgents = sortAgentsForNetwork(agentsQuery.data ?? []);
  const canCreateChannel =
    enabled &&
    requestedWorkspaceId != null &&
    dialogIsCurrentWorkspace &&
    createDraft.channelName.trim() !== "" &&
    createDraft.purpose.trim() !== "" &&
    createDraft.selectedAgentNames.length > 0 &&
    pendingRequest === null;

  const openNetworkSettings = () => {
    void navigate({ to: "/settings/network" });
  };
  const handleCreateChannel = () => {
    if (!enabled || !requestedWorkspaceId || !canCreateChannel) return;
    store.trigger.channelCreationRequested({
      create: input => createChannel.mutateAsync(input),
      navigate: async target => {
        await navigate({
          params: { workspaceId: target.workspaceId, channel: target.channel },
          to: "/network/$workspaceId/$channel/threads",
        });
      },
      workspaceId: requestedWorkspaceId,
    });
  };

  const action = enabled ? (
    <Button
      data-testid="network-open-create-dialog"
      disabled={agentsQuery.isLoading || requestedWorkspaceId == null}
      onClick={() => {
        if (requestedWorkspaceId) store.trigger.dialogOpened({ workspaceId: requestedWorkspaceId });
      }}
      size="sm"
      type="button"
      variant="outline"
    >
      <Plus aria-hidden="true" className="size-3" />
      Channel
    </Button>
  ) : null;
  const dialog = (
    <NetworkCreateChannelDialog
      agents={sortedAgents}
      canSubmit={canCreateChannel}
      draft={createDraft}
      isSubmitting={
        dialogIsCurrentWorkspace && pendingRequest?.workspaceId === requestedWorkspaceId
      }
      onAgentSelectionChange={selectedAgentNames =>
        store.trigger.agentSelectionChanged({ selectedAgentNames })
      }
      onChannelNameChange={channelName => store.trigger.channelNameEntered({ channelName })}
      onFanoutPolicyChange={fanoutPolicy => store.trigger.fanoutPolicyChanged({ fanoutPolicy })}
      onOpenChange={open => {
        if (open && requestedWorkspaceId) {
          store.trigger.dialogOpened({ workspaceId: requestedWorkspaceId });
          return;
        }
        store.trigger.dialogClosed();
      }}
      onPurposeChange={purpose => store.trigger.purposeEntered({ purpose })}
      onSubmit={handleCreateChannel}
      open={createOpen && dialogIsCurrentWorkspace}
      workspaceName={requestedWorkspace?.name}
    />
  );

  return { action, dialog, openNetworkSettings };
}
