import { useNavigate } from "@tanstack/react-router";

import { useCreateNetworkThread } from "../../hooks/use-network-actions";
import { Composer } from "./composer";

export interface ChannelThreadComposerProps {
  workspaceId: string;
  channel: string;
  /** The channel's fanout policy, named in the delivery hint when known. */
  fanoutPolicy?: string | null;
  /** The session id used by the operator to author messages in this channel. */
  sessionId: string;
  /** The local peer id (used for the optimistic root message). */
  peerFrom: string;
  displayName?: string;
  /** Disabled when no session is available; describes why so the placeholder can explain. */
  disabledReason?: string;
}

export function ChannelThreadComposer({
  workspaceId,
  channel,
  fanoutPolicy,
  sessionId,
  peerFrom,
  displayName,
  disabledReason,
}: ChannelThreadComposerProps) {
  const navigate = useNavigate();
  const { createThread, isCreating } = useCreateNetworkThread({ workspaceId });
  const disabled = disabledReason != null || workspaceId === "";

  const handleSubmit = async ({
    text,
    mentions,
    reset,
  }: {
    text: string;
    mentions: string[];
    reset: () => void;
  }) => {
    try {
      const result = await createThread({
        channel,
        sessionId,
        text,
        mentions,
        peerFrom,
        displayName,
      });
      reset();
      if (workspaceId) {
        void navigate({
          to: "/network/$workspaceId/$channel/threads/$threadId",
          params: { workspaceId, channel, threadId: result.threadId },
        });
      }
    } catch {
      // The hook surfaces a Sonner toast on the second collision. Keep the
      // textarea contents so the user can adjust + retry.
    }
  };

  return (
    <Composer
      disabled={disabled}
      disabledReason={disabledReason}
      hint={
        fanoutPolicy
          ? `Delivered per channel fanout — ${fanoutPolicy}.`
          : "Delivered per channel fanout."
      }
      isSending={isCreating}
      onSubmit={handleSubmit}
      placeholder="Start a new thread..."
      sendLabel={`Send to #${channel}`}
      testIdSuffix="channel-thread"
    />
  );
}
