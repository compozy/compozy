import { File, Folder, MessagesSquare, type LucideIcon } from "lucide-react";
import type { MouseEvent } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { cn } from "@/lib/utils";
import type { SessionMentionKind } from "./session-mention-model";

const MENTION_ICONS: Record<SessionMentionKind, LucideIcon> = {
  file: File,
  folder: Folder,
  session: MessagesSquare,
};

const KIND_NAMES: Record<SessionMentionKind, string> = {
  file: "File",
  folder: "Folder",
  session: "Session",
};

export interface SessionMentionChipProps {
  kind: SessionMentionKind;
  /** Relative path (folders end in `/`) or session id: the tooltip and accessible name. */
  target: string;
  /** File name, folder name ending in `/`, or session title (id when untitled). */
  label: string;
  /** History only: a session chip opens that session (⌘/Ctrl opens a new window). */
  onOpen?: (options: { newWindow: boolean }) => void;
}

/**
 * Inline mention chip for the composer and the transcript (S2, S4): the skill
 * chip's text-weight anatomy in neutral ink, so a mention points without ever
 * reading as a runnable command. The icon carries the kind; the tooltip and
 * accessible name carry the full relative path or session id.
 */
export function SessionMentionChip({ kind, target, label, onOpen }: SessionMentionChipProps) {
  const Icon = MENTION_ICONS[kind];
  const accessibleName =
    kind === "session"
      ? `${KIND_NAMES[kind]} ${label} (${target})`
      : `${KIND_NAMES[kind]} ${target}`;
  const content = (
    <>
      <Icon
        className="mr-1 inline-block size-[1em] -translate-y-px align-middle text-muted"
        aria-hidden="true"
      />
      {label}
    </>
  );
  const className = cn(
    "mx-0.5 inline max-w-full align-baseline font-medium text-fg select-none",
    onOpen
      ? cn(
          "cursor-pointer rounded-xs underline-offset-3 decoration-line-focus",
          "hover:underline focus-visible:underline",
          "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-line-focus"
        )
      : null
  );
  const trigger = onOpen ? (
    <button
      type="button"
      data-testid="session-mention-chip"
      data-mention-kind={kind}
      data-mention-target={target}
      aria-label={accessibleName}
      className={className}
      onClick={(event: MouseEvent<HTMLButtonElement>) =>
        onOpen({ newWindow: event.metaKey || event.ctrlKey })
      }
    />
  ) : (
    <span
      data-testid="session-mention-chip"
      data-mention-kind={kind}
      data-mention-target={target}
      aria-label={accessibleName}
      className={className}
    />
  );
  return (
    <Tooltip>
      <TooltipTrigger render={trigger}>{content}</TooltipTrigger>
      <TooltipContent className="font-mono">{target}</TooltipContent>
    </Tooltip>
  );
}
