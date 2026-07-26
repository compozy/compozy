import { Button } from "@agh/ui";

import { cn } from "@/lib/utils";

import type { NetworkConversationMessage } from "../../types";
import { formatTimelineClock, formatTimelineIso } from "../../lib/format-timestamp";
import { WorkChip } from "../work/work-chip";
import { HoverToolbar, type HoverToolbarHandlers } from "./hover-toolbar";
import { MessageAvatar, type MessageAvatarOwnerRole } from "./message-avatar";
import { MessageBodyText } from "./message-body";

export type MessageRowDensity = "channel" | "overlay";

export interface MessageRowOptimisticHandlers {
  /** Retry handler invoked when the optimistic message is in the `failed` state. */
  onRetry?: (message: NetworkConversationMessage) => void;
  /** Discard handler invoked from the inline retry/discard cluster. */
  onDiscard?: (message: NetworkConversationMessage) => void;
  /** Click handler when the work chip is clicked (opens the Work Inspector). */
  onWorkChipClick?: (message: NetworkConversationMessage) => void;
}

export interface MessageRowProps extends HoverToolbarHandlers, MessageRowOptimisticHandlers {
  message: NetworkConversationMessage;
  density?: MessageRowDensity;
  className?: string;
}

function pickRoleLabel(message: NetworkConversationMessage): MessageAvatarOwnerRole {
  if (message.session_id != null && message.session_id !== "") {
    return "agent";
  }
  return message.local ? "human" : "system";
}

function authorSeed(message: NetworkConversationMessage): string {
  return message.peer_from ?? message.display_name ?? message.message_id;
}

function readOptimisticState(message: NetworkConversationMessage): "pending" | "failed" | null {
  const candidate = (message as Partial<{ optimistic: "pending" | "failed" }>).optimistic;
  if (candidate === "pending" || candidate === "failed") {
    return candidate;
  }
  return null;
}

function readWorkState(message: NetworkConversationMessage): string | null {
  const body = (message.body ?? null) as { state?: unknown } | null;
  if (!body || typeof body.state !== "string") {
    return null;
  }
  const trimmed = body.state.trim();
  return trimmed.length > 0 ? trimmed : null;
}

const DENSITY_AVATAR: Record<MessageRowDensity, 26 | 32> = {
  channel: 26,
  overlay: 32,
};

export function MessageRow({
  message,
  density = "channel",
  className,
  onCopyLink,
  onCopyText,
  onRetry,
  onDiscard,
  onWorkChipClick,
}: MessageRowProps) {
  const role = pickRoleLabel(message);
  const clock = formatTimelineClock(message.timestamp);
  const iso = formatTimelineIso(message.timestamp);
  const displayName = message.display_name?.trim() || message.peer_from || "Unknown peer";
  const avatarSize = DENSITY_AVATAR[density];
  const optimisticState = readOptimisticState(message);
  const workState = readWorkState(message);
  const workId = message.work_id ?? null;

  return (
    <article
      aria-label={`${displayName} message`}
      className={cn(
        "group relative flex gap-3 px-5 py-2 transition-colors duration-fast ease-out hover:bg-row-hover",
        density === "overlay" && "px-4",
        optimisticState === "pending" && "opacity-70",
        optimisticState === "failed" && "rounded-chip bg-danger-tint",
        className
      )}
      data-density={density}
      data-message-id={message.message_id}
      data-optimistic={optimisticState ?? undefined}
      data-testid="network-message-row-full"
      data-variant="full"
    >
      <MessageAvatar
        initialFrom={displayName}
        name={displayName}
        ownerRole={role}
        seed={authorSeed(message)}
        sizePx={avatarSize}
      />

      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex items-baseline gap-2 pr-14">
          <span className="truncate text-item-title font-semibold text-fg-strong">
            {displayName}
          </span>
          <time
            className="shrink-0 font-mono text-mono-id tracking-mono-id text-subtle tabular-nums"
            data-testid="network-message-timestamp"
            dateTime={iso}
            title={iso}
          >
            {clock}
          </time>
          {workId && workState ? (
            <WorkChip
              ariaLabel={`Work ${workId} · ${workState}`}
              onClick={onWorkChipClick ? () => onWorkChipClick(message) : undefined}
              startedAt={message.timestamp}
              state={workState}
            />
          ) : null}
        </div>

        <MessageBodyText message={message} />

        {optimisticState === "failed" ? (
          <div
            className="flex items-center gap-2 pt-1 text-xs text-danger"
            data-testid="network-message-failed-cluster"
          >
            <span>Couldn&apos;t send.</span>
            <Button
              data-testid="network-message-retry"
              onClick={onRetry ? () => onRetry(message) : undefined}
              size="sm"
              type="button"
              variant="ghost"
            >
              Retry
            </Button>
            <Button
              data-testid="network-message-discard"
              onClick={onDiscard ? () => onDiscard(message) : undefined}
              size="sm"
              type="button"
              variant="ghost"
            >
              Discard
            </Button>
          </div>
        ) : null}
      </div>

      <HoverToolbar
        onCopyLink={onCopyLink}
        onCopyText={onCopyText}
        testIdSuffix={message.message_id}
      />
    </article>
  );
}
