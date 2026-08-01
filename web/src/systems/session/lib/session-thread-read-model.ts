import type { ThreadMessage } from "@assistant-ui/react";

function transcriptMessageIDs(transcriptMessages: readonly ThreadMessage[]): ReadonlySet<string> {
  return new Set(transcriptMessages.map(message => message.id));
}

export function hasUnreconciledRuntimeMessages({
  transcriptMessages,
  runtimeMessages,
}: {
  transcriptMessages: readonly ThreadMessage[];
  runtimeMessages: readonly ThreadMessage[];
}): boolean {
  const transcriptIDs = transcriptMessageIDs(transcriptMessages);
  return runtimeMessages.some(message => !transcriptIDs.has(message.id));
}

export function mergeSessionThreadReadModel({
  transcriptMessages,
  runtimeMessages,
  includeRuntimeTail = true,
}: {
  transcriptMessages: readonly ThreadMessage[];
  runtimeMessages: readonly ThreadMessage[];
  includeRuntimeTail?: boolean;
}): readonly ThreadMessage[] {
  if (runtimeMessages.length === 0 || !includeRuntimeTail) {
    return transcriptMessages;
  }

  const transcriptIDs = transcriptMessageIDs(transcriptMessages);
  const optimisticTail: ThreadMessage[] = [];

  for (const message of runtimeMessages) {
    if (transcriptIDs.has(message.id)) continue;
    optimisticTail.push(message);
  }

  return optimisticTail.length === 0
    ? transcriptMessages
    : [...transcriptMessages, ...optimisticTail];
}
