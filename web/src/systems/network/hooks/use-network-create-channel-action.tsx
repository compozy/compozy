import { useNavigate } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { useState } from "react";

import { useAgents } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";
import { Button, toast } from "@agh/ui";

import { NetworkCreateChannelDialog } from "../components/network-create-channel-dialog";
import { createNetworkChannelDraft, sortAgentsForNetwork } from "../lib/network-formatters";
import { useCreateNetworkChannel } from "./use-network-actions";

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
  const [createOpen, setCreateOpen] = useState(false);
  const [createDraft, setCreateDraft] = useState(createNetworkChannelDraft);
  const sortedAgents = sortAgentsForNetwork(agentsQuery.data ?? []);
  const canCreateChannel =
    enabled &&
    requestedWorkspaceId != null &&
    createDraft.channelName.trim() !== "" &&
    createDraft.purpose.trim() !== "" &&
    createDraft.selectedAgentNames.length > 0;

  const openNetworkSettings = () => {
    void navigate({ to: "/settings/network" });
  };

  const handleCreateChannel = async () => {
    if (!enabled || !requestedWorkspaceId || !canCreateChannel) {
      return;
    }

    try {
      // `coordinator_peer_id` is deliberately absent: the daemon rejects it under
      // any policy but `coordinator`, and coordinator is not writable at create
      // time because member peers do not exist until this call provisions them.
      const response = await createChannel.mutateAsync({
        agent_names: createDraft.selectedAgentNames,
        channel: createDraft.channelName.trim(),
        fanout_policy: createDraft.fanoutPolicy,
        purpose: createDraft.purpose.trim(),
        workspace_id: requestedWorkspaceId,
      });
      const channel = response.channel.channel;
      setCreateDraft(createNetworkChannelDraft());
      setCreateOpen(false);
      void navigate({
        params: { workspaceId: requestedWorkspaceId, channel },
        to: "/network/$workspaceId/$channel/threads",
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to create network channel";
      toast.error(message);
    }
  };

  const action = enabled ? (
    <Button
      data-testid="network-open-create-dialog"
      disabled={agentsQuery.isLoading || requestedWorkspaceId == null}
      onClick={() => setCreateOpen(true)}
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
      isSubmitting={createChannel.isPending}
      onAgentSelectionChange={selectedAgentNames =>
        setCreateDraft(current => ({ ...current, selectedAgentNames }))
      }
      onChannelNameChange={channelName => setCreateDraft(current => ({ ...current, channelName }))}
      onFanoutPolicyChange={fanoutPolicy =>
        setCreateDraft(current => ({ ...current, fanoutPolicy }))
      }
      onOpenChange={setCreateOpen}
      onPurposeChange={purpose => setCreateDraft(current => ({ ...current, purpose }))}
      onSubmit={handleCreateChannel}
      open={createOpen}
      workspaceName={requestedWorkspace?.name}
    />
  );

  return { action, dialog, openNetworkSettings };
}
