import { useState } from "react";

import { Eyebrow } from "@agh/ui";

import { cn } from "@/lib/utils";

import type { NetworkConversationMessage } from "../../types";
import { formatTimelineClock, formatTimelineIso } from "../../lib/format-timestamp";
import { readMessageBody } from "./message-body.logic";

export interface MessageRowSystemProps {
  message: NetworkConversationMessage;
  className?: string;
}

function buildSystemSummary(message: NetworkConversationMessage): string {
  const summary = readMessageBody(message);
  if (summary) {
    return summary;
  }
  const author = message.display_name?.trim() ?? message.peer_from ?? "";
  return author ? `${author} · ${message.kind}` : message.kind;
}

export function MessageRowSystem({ message, className }: MessageRowSystemProps) {
  const [expanded, setExpanded] = useState(false);
  const clock = formatTimelineClock(message.timestamp);
  const iso = formatTimelineIso(message.timestamp);
  const summary = buildSystemSummary(message);
  const body = readMessageBody(message);
  const canExpand = body.length > 80;

  return (
    <button
      aria-expanded={canExpand ? expanded : undefined}
      aria-label={`System event: ${message.kind}`}
      className={cn("group flex w-full items-center gap-3 px-5 py-1 text-left", className)}
      data-message-id={message.message_id}
      data-testid="network-message-row-system"
      data-variant="system"
      onClick={() => {
        if (canExpand) {
          setExpanded(value => !value);
        }
      }}
      type="button"
    >
      <span aria-hidden="true" className="block h-px w-9 shrink-0 bg-line" />
      <span className="flex min-w-0 flex-1 flex-wrap items-baseline gap-2 font-mono text-xs text-muted">
        <Eyebrow>{message.kind}</Eyebrow>
        <span className={cn("min-w-0", expanded ? "" : "truncate")}>{summary}</span>
      </span>
      <time className="shrink-0 font-mono text-badge text-subtle" dateTime={iso} title={iso}>
        {clock}
      </time>
    </button>
  );
}
