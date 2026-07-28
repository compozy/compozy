import { toast } from "sonner";

import type { NetworkConversationMessage } from "../../types";
import type { HoverToolbarHandlers } from "./hover-toolbar";
import { readMessageBody } from "./message-body.logic";

export interface MessageCopyTarget {
  surface: "thread" | "direct";
  workspaceId: string;
  channel: string;
  /** Thread id when `surface === "thread"`, direct room id otherwise. */
  conversationId: string;
}

async function writeClipboard(value: string): Promise<boolean> {
  if (typeof navigator === "undefined" || !navigator.clipboard) {
    return false;
  }
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}

function buildMessageLink(target: MessageCopyTarget, messageId: string): string {
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  const segment = target.surface === "thread" ? "threads" : "directs";
  const path = [
    "network",
    encodeURIComponent(target.workspaceId),
    encodeURIComponent(target.channel),
    segment,
    encodeURIComponent(target.conversationId),
  ].join("/");
  return `${origin}/${path}#msg-${encodeURIComponent(messageId)}`;
}

/**
 * Builds per-message toolbar handlers for the network timeline. Returns a stable
 * factory so callers can resolve handlers per message inline. Both actions are
 * real and client-side: Copy link (a shareable deep link) and Copy text (the
 * message body). Copy text is omitted when the message has no readable body.
 */
export function useMessageCopyActions(
  target: MessageCopyTarget
): (message: NetworkConversationMessage) => HoverToolbarHandlers {
  const { surface, workspaceId, channel, conversationId } = target;

  return (message: NetworkConversationMessage): HoverToolbarHandlers => {
    const text = readMessageBody(message);
    return {
      onCopyLink: async () => {
        const link = buildMessageLink(
          { surface, workspaceId, channel, conversationId },
          message.message_id
        );
        const ok = await writeClipboard(link);
        if (ok) {
          toast.success("Link copied");
        } else {
          toast.error("Couldn't copy link");
        }
      },
      onCopyText: text
        ? async () => {
            const ok = await writeClipboard(text);
            if (ok) {
              toast.success("Message copied");
            } else {
              toast.error("Couldn't copy message");
            }
          }
        : undefined,
    };
  };
}
