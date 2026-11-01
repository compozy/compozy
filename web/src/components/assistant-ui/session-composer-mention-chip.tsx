import { File, Folder, MessagesSquare, type LucideIcon } from "@compozy/ui/icons";
import type { MouseEvent, ReactNode } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { AgentMark } from "@/systems/agent/components/agent-mark";
import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import type { SessionMentionKind } from "./session-mention-model";

type PathOrSessionKind = Exclude<SessionMentionKind, "agent">;

const MENTION_ICONS: Record<PathOrSessionKind, LucideIcon> = {
  file: File,
  folder: Folder,
  session: MessagesSquare,
};

const KIND_NAMES: Record<PathOrSessionKind, string> = {
  file: "File",
  folder: "Folder",
  session: "Session",
};

export interface SessionMentionChipProps {
  kind: SessionMentionKind;
  /** Relative path (folders end in `/`), session id or agent slug: tooltip and screen-reader text. */
  target: string;
  /** File name, folder name ending in `/`, session title (id when untitled), or agent label. */
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
export function SessionMentionChip({ kind, ...props }: SessionMentionChipProps) {
  if (kind === "agent") return <AgentMentionChip {...props} />;
  const Icon = MENTION_ICONS[kind];
  // Screen readers get the kind and the full path or session id as hidden
  // text inside the chip, so the name holds on a plain span too.
  const { label, target } = props;
  return (
    <MentionChipFrame
      {...props}
      kind={kind}
      hiddenDescription={
        kind === "session" ? `Session ${label} (${target})` : `${KIND_NAMES[kind]} ${target}`
      }
      mark={
        <Icon
          className="mr-1 inline-block size-[1em] -translate-y-px align-middle text-muted-foreground"
          aria-hidden="true"
        />
      }
    />
  );
}

/** An agent gone from the catalog keeps its chip, struck through; the sent text doesn't change. */
function AgentMentionChip(props: Omit<SessionMentionChipProps, "kind">) {
  const { appearance, known, label: current, missing } = useAgentIdentityView(props.target);
  const label = known ? current : props.label;
  return (
    <MentionChipFrame
      {...props}
      kind="agent"
      label={label}
      hiddenDescription={`Agent ${label} (${props.target})${missing ? ", no longer available" : ""}`}
      mark={
        <AgentMark
          aria-hidden="true"
          appearance={appearance}
          className="mr-1 inline-grid -translate-y-px align-middle"
          label={label}
          quietIdle
          size={14}
          state="unknown"
        />
      }
      unresolved={missing}
    />
  );
}

function MentionChipFrame({
  kind,
  target,
  label,
  onOpen,
  token,
  mark,
  hiddenDescription,
  unresolved = false,
}: SessionMentionChipProps & {
  mark: ReactNode;
  hiddenDescription: string;
  unresolved?: boolean;
}) {
  const content = (
    <>
      {mark}
      <span aria-hidden="true">{label}</span>
      <span className="sr-only">{hiddenDescription}</span>
    </>
  );
  const className = cn(
    "mx-0.5 inline max-w-full align-baseline font-medium whitespace-nowrap select-none",
    unresolved ? "text-subtle line-through" : "text-fg",
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
      data-mention-unresolved={unresolved || undefined}
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
      data-mention-unresolved={unresolved || undefined}
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
