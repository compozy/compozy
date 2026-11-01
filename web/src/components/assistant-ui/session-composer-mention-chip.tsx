import { File, Folder, MessagesSquare, type LucideIcon } from "@compozy/ui/icons";
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
  /** Relative path (folders end in `/`) or session id: the tooltip and screen-reader text. */
  target: string;
  /** File name, folder name ending in `/`, or session title (id when untitled). */
  label: string;
  /** History only: a session chip opens that session (⌘/Ctrl opens a new window). */
  onOpen?: (options: { newWindow: boolean }) => void;
  /** History only: the raw token a copied selection carries in place of the chip. */
  token?: string;
}

/**
 * Inline mention chip for the composer and the transcript (S2, S4): the skill
 * chip's text-weight anatomy in neutral ink, so a mention points without ever
 * reading as a runnable command. The icon carries the kind; the tooltip and
 * visually hidden text carry the full relative path or session id.
 */
export function SessionMentionChip({
  kind,
  target,
  label,
  onOpen,
  token,
}: SessionMentionChipProps) {
  const Icon = MENTION_ICONS[kind];
  // Screen readers get the kind and the full path or session id as hidden
  // text inside the chip, so the name holds on a plain span too.
  const hiddenDescription =
    kind === "session" ? `Session ${label} (${target})` : `${KIND_NAMES[kind]} ${target}`;
  const content = (
    <>
      <Icon
        className="mr-1 inline-block size-[1em] -translate-y-px align-middle text-muted-foreground"
        aria-hidden="true"
      />
      <span aria-hidden="true">{label}</span>
      <span className="sr-only">{hiddenDescription}</span>
    </>
  );
  const className = cn(
    "mx-0.5 inline max-w-full align-baseline font-medium whitespace-nowrap text-fg select-none",
    onOpen
      ? cn(
          "cursor-pointer rounded-xxs underline-offset-3 decoration-line-focus decoration-1",
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
      data-mention-token={token}
      className={className}
      onClick={(event: MouseEvent<HTMLButtonElement>) =>
        onOpen({ newWindow: event.metaKey || event.ctrlKey })
      }
    >
      {content}
    </button>
  ) : (
    <span
      data-testid="session-mention-chip"
      data-mention-kind={kind}
      data-mention-target={target}
      data-mention-token={token}
      className={className}
    >
      {content}
    </span>
  );
  // The trigger element owns its content, so the button's accessible name is
  // the hidden description it renders, never an empty control.
  return (
    <Tooltip>
      <TooltipTrigger render={trigger} />
      <TooltipContent className="max-w-[360px] font-mono text-micro font-normal [overflow-wrap:anywhere]">
        {target}
      </TooltipContent>
    </Tooltip>
  );
}
