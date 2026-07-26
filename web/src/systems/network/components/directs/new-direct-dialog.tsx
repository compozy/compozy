import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import {
  Button,
  Command,
  CommandEmpty,
  CommandItem,
  CommandList,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
} from "@agh/ui";

import { useResolveNetworkDirectRoom } from "../../hooks/use-network-actions";
import { networkPeersOptions } from "../../lib/query-options";
import type { NetworkPeerSummary } from "../../types";
import { getPeerDisplayName } from "../../lib/network-formatters";

export interface NewDirectDialogProps {
  workspaceId?: string;
  open: boolean;
  onOpenChange: (next: boolean) => void;
  channel: string;
  /** Local peer id used as the requesting side of the resolve request. */
  selfPeerId?: string;
  /** Local session id used for the resolve request. */
  sessionId: string;
}

interface PickerProps {
  workspaceId: string;
  channel: string;
  selfPeerId?: string;
  onSelect: (peer: NetworkPeerSummary) => void;
  selectedPeerId: string | null;
  disabled: boolean;
}

function PeerPickerList({
  workspaceId,
  channel,
  selfPeerId,
  onSelect,
  selectedPeerId,
  disabled,
}: PickerProps) {
  const { data, isLoading } = useQuery(
    networkPeersOptions(workspaceId, channel, Boolean(workspaceId))
  );
  const candidates = (data ?? []).filter(peer => peer.peer_id !== selfPeerId);

  if (isLoading) {
    return <p className="px-2 py-3 text-xs text-subtle">Loading peers…</p>;
  }

  if (candidates.length === 0) {
    return (
      <Command aria-label="Channel peers" className="rounded-none bg-transparent p-0">
        <CommandList>
          <CommandEmpty data-testid="network-new-direct-no-peers">
            No other peers in this channel yet.
          </CommandEmpty>
        </CommandList>
      </Command>
    );
  }

  return (
    <Command aria-label="Channel peers" className="rounded-none bg-transparent p-0">
      <CommandList className="max-h-64">
        {candidates.map(peer => (
          <CommandItem
            aria-selected={peer.peer_id === selectedPeerId ? "true" : "false"}
            className="justify-between rounded-chip p-2"
            data-testid={`network-new-direct-peer-${peer.peer_id}`}
            disabled={disabled}
            key={peer.peer_id}
            onSelect={() => onSelect(peer)}
            value={peer.peer_id}
          >
            <span className="truncate text-small-body text-fg">@{getPeerDisplayName(peer)}</span>
            <Eyebrow>{peer.peer_id}</Eyebrow>
          </CommandItem>
        ))}
      </CommandList>
    </Command>
  );
}

export function NewDirectDialog({
  workspaceId = "",
  open,
  onOpenChange,
  channel,
  selfPeerId,
  sessionId,
}: NewDirectDialogProps) {
  const navigate = useNavigate();
  const { resolveRoom, isResolving, error } = useResolveNetworkDirectRoom({ workspaceId });
  const [selectedPeerId, setSelectedPeerId] = useState<string | null>(null);

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setSelectedPeerId(null);
    }
    onOpenChange(nextOpen);
  };

  const handleSelect = async (peer: NetworkPeerSummary) => {
    setSelectedPeerId(peer.peer_id);
    try {
      const direct = await resolveRoom({
        channel,
        body: {
          peer_id: peer.peer_id,
          session_id: sessionId,
        },
      });
      handleOpenChange(false);
      if (workspaceId) {
        void navigate({
          to: "/network/$workspaceId/$channel/directs/$directId",
          params: { workspaceId, channel, directId: direct.direct_id },
        });
      }
    } catch {
      // Error message is rendered inline below; the dialog stays open.
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="network-new-direct-dialog">
        <DialogHeader>
          <DialogTitle>New direct room</DialogTitle>
          <DialogDescription>
            Pick a peer in #{channel} to open a restricted bilateral conversation.
          </DialogDescription>
        </DialogHeader>

        <PeerPickerList
          workspaceId={workspaceId}
          channel={channel}
          disabled={isResolving}
          onSelect={handleSelect}
          selectedPeerId={selectedPeerId}
          selfPeerId={selfPeerId}
        />

        {error ? (
          <p className="text-xs text-danger" data-testid="network-new-direct-error" role="alert">
            {error.message}
          </p>
        ) : null}

        <DialogFooter>
          <Button onClick={() => handleOpenChange(false)} type="button" variant="outline">
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
