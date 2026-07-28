import { useActiveNetworkSession, type ActiveNetworkSession } from "../../hooks/use-active-session";
import {
  useSendNetworkMessage,
  type SendNetworkMessageThreadInput,
} from "../../hooks/use-network-actions";
import { useThreadOverlay, type UseThreadOverlayResult } from "../../hooks/use-thread-overlay";
import { useOpenWork, type UseOpenWorkResult } from "../../hooks/use-work";
import type { NetworkConversationMessage } from "../../types";

export interface UseThreadOverlayViewArgs {
  workspaceId: string;
  channel: string;
  threadId: string;
  fullPage: boolean;
  onClose: () => void;
}

export interface UseThreadOverlayViewResult {
  overlay: UseThreadOverlayResult;
  session: ActiveNetworkSession | null;
  disabledReason: string | null;
  openWork: UseOpenWorkResult;
  handleRetry: (message: NetworkConversationMessage) => void;
  handleDiscard: (message: NetworkConversationMessage) => void;
}

export function useThreadOverlayView({
  workspaceId,
  channel,
  threadId,
  fullPage,
  onClose,
}: UseThreadOverlayViewArgs): UseThreadOverlayViewResult {
  const overlay = useThreadOverlay({ workspaceId, channel, fullPage, threadId, onClose });
  const session = useActiveNetworkSession(channel, { workspaceId });
  const openWork = useOpenWork({
    workspaceId,
    channel,
    surface: "thread",
    containerId: threadId,
    exactOpenCount: overlay.detail?.open_work_count,
  });
  const { retry, discard } = useSendNetworkMessage({ workspaceId });

  const buildSendInput = (
    message: NetworkConversationMessage
  ): SendNetworkMessageThreadInput | null => {
    if (!session.session) return null;
    return {
      surface: "thread",
      channel,
      threadId,
      sessionId: session.session.sessionId,
      peerFrom: session.session.peerId,
      text: message.text ?? "",
      mentions: message.mentions ?? [],
      displayName: session.session.displayName,
    };
  };

  const handleRetry = (message: NetworkConversationMessage) => {
    const input = buildSendInput(message);
    if (input == null) return;
    void retry(input, message.message_id);
  };

  const handleDiscard = (message: NetworkConversationMessage) => {
    const input = buildSendInput(message);
    if (input == null) return;
    discard(input, message.message_id);
  };

  return {
    overlay,
    session: session.session,
    disabledReason: session.disabledReason,
    openWork,
    handleRetry,
    handleDiscard,
  };
}
