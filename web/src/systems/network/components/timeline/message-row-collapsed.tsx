import { cn } from "@/lib/utils";

import type { NetworkConversationMessage } from "../../types";
import { formatTimelineClockWithSeconds, formatTimelineIso } from "../../lib/format-timestamp";
import { HoverToolbar, type HoverToolbarHandlers } from "./hover-toolbar";
import { MessageBodyText } from "./message-body";
import type { MessageRowDensity } from "./message-row";

export interface MessageRowCollapsedProps extends HoverToolbarHandlers {
  message: NetworkConversationMessage;
  density?: MessageRowDensity;
  className?: string;
}

const GUTTER_WIDTH: Record<MessageRowDensity, string> = {
  channel: "w-9", // 36px gutter to align with full row avatar
  overlay: "w-8", // 32px gutter
};

export function MessageRowCollapsed({
  message,
  density = "channel",
  className,
  onCopyLink,
  onCopyText,
}: MessageRowCollapsedProps) {
  const clock = formatTimelineClockWithSeconds(message.timestamp);
  const iso = formatTimelineIso(message.timestamp);

  return (
    <article
      aria-label="Message continuation"
      className={cn(
        "group relative flex gap-3 px-5 py-1 transition-colors duration-fast ease-out hover:bg-row-hover",
        density === "overlay" && "px-4",
        className
      )}
      data-density={density}
      data-message-id={message.message_id}
      data-testid="network-message-row-collapsed"
      data-variant="collapsed"
    >
      <div
        className={cn("relative shrink-0", GUTTER_WIDTH[density])}
        data-testid="network-message-collapsed-gutter"
      >
        <time
          aria-hidden="true"
          className="absolute top-1 left-0 right-0 text-center font-mono text-badge tracking-mono text-subtle opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
          data-testid="network-message-collapsed-timestamp"
          dateTime={iso}
          title={iso}
        >
          {clock}
        </time>
      </div>

      <div className="flex min-w-0 flex-1">
        <MessageBodyText message={message} />
      </div>

      <HoverToolbar
        onCopyLink={onCopyLink}
        onCopyText={onCopyText}
        testIdSuffix={message.message_id}
      />
    </article>
  );
}
