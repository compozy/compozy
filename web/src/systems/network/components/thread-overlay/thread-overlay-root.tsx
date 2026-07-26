import { Eyebrow } from "@agh/ui";

import type { NetworkConversationMessage } from "../../types";
import type { HoverToolbarHandlers } from "../timeline/hover-toolbar";
import { MessageRow } from "../timeline/message-row";

export interface ThreadOverlayRootProps {
  rootMessage: NetworkConversationMessage | null;
  isLoading: boolean;
  toolbarHandlers?: (message: NetworkConversationMessage) => HoverToolbarHandlers;
}

export function ThreadOverlayRoot({
  rootMessage,
  isLoading,
  toolbarHandlers,
}: ThreadOverlayRootProps) {
  if (isLoading && !rootMessage) {
    return (
      <div
        aria-label="Loading thread root"
        className="flex flex-col gap-2 border-b border-line px-4 pt-4 pb-3"
        data-testid="network-thread-overlay-root-loading"
        role="status"
      >
        <Eyebrow className="text-subtle">Loading root</Eyebrow>
      </div>
    );
  }

  if (!rootMessage) {
    return null;
  }

  const handlers = toolbarHandlers?.(rootMessage);

  return (
    <div className="flex flex-col gap-1.5 border-b border-line pt-4 pb-3">
      <Eyebrow className="px-4 text-subtle" data-testid="network-thread-overlay-root-badge">
        ROOT
      </Eyebrow>
      <MessageRow
        density="overlay"
        message={rootMessage}
        onCopyLink={handlers?.onCopyLink}
        onCopyText={handlers?.onCopyText}
      />
    </div>
  );
}
