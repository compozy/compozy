import type { NetworkConversationMessage } from "../../types";
import { readMessageBody } from "./message-body.logic";

export interface MessageBodyTextProps {
  message: NetworkConversationMessage;
  className?: string;
}

export function MessageBodyText({ message, className }: MessageBodyTextProps) {
  const body = readMessageBody(message);
  if (!body) {
    return null;
  }

  return (
    <p
      className={
        className ?? "whitespace-pre-wrap text-pretty text-item-title leading-relaxed text-fg"
      }
      data-testid="network-message-body"
    >
      {body}
    </p>
  );
}
