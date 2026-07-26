import type { ThreadMessage } from "@assistant-ui/react";

export function threadProviderIdentity(messages: readonly ThreadMessage[]): string {
  return messages.map(message => `${message.id.length}:${message.id}`).join("|");
}
