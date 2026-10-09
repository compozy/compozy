import { useId, useState, type MouseEvent, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { HoverCard, HoverCardContent, HoverCardTrigger, Icon, cn } from "@compozy/ui";

import { SubagentAvatar } from "./subagent-avatar";
import { SubagentElapsed } from "./subagent-elapsed";
import {
  SUBAGENT_STATUS_WORD,
  isSubagentLive,
  subagentCardLines,
  subagentElapsedClock,
  type SubagentLineTwoTone,
} from "./subagent-format";
import { SubagentHoverContent } from "./subagent-hover-content";
import type { SubagentLocationView, SubagentView } from "./types";

export interface SubagentOpenOptions {
  /** ⌘/Ctrl held: open the child in a new window instead of this one. */
  newWindow: boolean;
}

export interface SubagentCardProps {
  subagent: SubagentView;
  /** Drill-in; the card is a button only when it has a child session and this handler. */
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
  /**
   * Provider-native inner work (S8). When set on a card with no child session,
   * the card becomes a disclosure that expands these rows inline.
   */
  nested?: ReactNode;
  defaultExpanded?: boolean;
  /** The stream has not confirmed this row since a reconnect: no ticking, no pulse. */
  stale?: boolean;
  /** `flush` drops the frame inside a group panel. */
  variant?: "framed" | "flush";
  location?: SubagentLocationView;
  parentLocation?: SubagentLocationView;
  className?: string;
}

const LINE_TWO_TONE: Record<SubagentLineTwoTone, string> = {
  default: "text-muted",
  word: "text-subtle",
  danger: "text-danger",
};

const CARD_FRAME = {
  framed: "min-h-11.5 max-w-140 rounded-md border border-line-soft bg-canvas-soft",
  flush: "min-h-10.5 rounded-md border border-transparent",
} as const;

type SubagentCardMode = "drill" | "disclosure" | "static";

/** Drill into a child session; expand provider-native rows inline; otherwise a static row. */
function subagentCardMode(
  subagent: SubagentView,
  canOpen: boolean,
  hasNested: boolean
): SubagentCardMode {
  if (subagent.child_session_id !== null) return canOpen ? "drill" : "static";
  return hasNested ? "disclosure" : "static";
}

function cardClassName(mode: SubagentCardMode, variant: "framed" | "flush", open: boolean): string {
  const interactive = mode !== "static";
  return cn(
    "group/subagent-card grid w-full items-center gap-2.5 py-1.75 pr-2.5 pl-2.25 text-left text-fg",
    interactive
      ? "grid-cols-[24px_minmax(0,1fr)_auto_14px] transition-colors duration-fast ease-out outline-none hover:bg-surface-2 focus-visible:shadow-focus-ring"
      : "grid-cols-[24px_minmax(0,1fr)_auto]",
    CARD_FRAME[variant],
    variant === "framed" && interactive && "hover:border-line",
    open && "rounded-b-none"
  );
}

interface SubagentCardBodyProps {
  subagent: SubagentView;
  stale: boolean;
  /** `null` = no chevron (not interactive); `open` rotates it for an expanded disclosure. */
  chevron: "open" | "closed" | null;
}

/** Card anatomy: avatar · title + status word / line 2 · elapsed · chevron. */
function SubagentCardBody({ subagent, stale, chevron }: SubagentCardBodyProps) {
  const lines = subagentCardLines(subagent);
  const lineTwoTone =
    subagent.status === "waiting" && lines.lineTwoTone === "word"
      ? "text-fg"
      : LINE_TWO_TONE[lines.lineTwoTone];
  return (
    <>
      <SubagentAvatar provider={subagent.runtime.provider} status={subagent.status} still={stale} />
      <span className="flex min-w-0 flex-col gap-px">
        <span className="flex min-w-0 items-baseline gap-2">
          <span className="min-w-0 truncate text-transcript-meta font-medium text-fg-strong">
            {subagent.title}
          </span>
          {lines.titleWord ? (
            <span
              className={cn(
                "shrink-0 text-transcript-caption",
                subagent.status === "failed" ? "text-danger" : "text-subtle"
              )}
            >
              {lines.titleWord}
            </span>
          ) : null}
        </span>
        <span
          className={cn("min-w-0 truncate text-transcript-caption", lineTwoTone)}
          data-slot="subagent-card-line-two"
        >
          {lines.lineTwo}
        </span>
      </span>
      <SubagentElapsed clock={subagentElapsedClock(subagent, { stale })} />
      {chevron ? (
        <Icon
          as={ChevronRight}
          aria-hidden="true"
          className={cn(
            "text-faint transition-[color,rotate] duration-base ease-out group-hover/subagent-card:text-muted motion-reduce:transition-none",
            chevron === "open" && "rotate-90"
          )}
        />
      ) : null}
    </>
  );
}

/**
 * One delegation in the parent transcript (transcript VC-01/02): avatar, title
 * with the status word, line 2, elapsed, and a chevron when there is a child
 * session to open.
 */
export function SubagentCard({
  subagent,
  onOpen,
  nested,
  defaultExpanded = false,
  stale = false,
  variant = "framed",
  location,
  parentLocation,
  className,
}: SubagentCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const panelId = useId();
  const mode = subagentCardMode(subagent, onOpen !== undefined, nested != null);
  const open = mode === "disclosure" && expanded;
  const word = SUBAGENT_STATUS_WORD[subagent.status];
  const props = {
    "data-slot": "subagent-card",
    "data-status": subagent.status,
    "data-origin": subagent.origin,
    "data-settled": isSubagentLive(subagent.status) ? undefined : "true",
    "data-stale": stale ? "true" : undefined,
    className: cn(cardClassName(mode, variant, open), className),
  };

  const handleOpen = (event: MouseEvent<HTMLButtonElement>) => {
    onOpen?.(subagent, { newWindow: event.metaKey || event.ctrlKey });
  };

  const trigger =
    mode === "drill" ? (
      <button
        type="button"
        aria-label={`Open ${subagent.title}`}
        aria-description={word}
        onClick={handleOpen}
        {...props}
      />
    ) : mode === "disclosure" ? (
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls={panelId}
        aria-description={word}
        onClick={() => setExpanded(current => !current)}
        {...props}
      />
    ) : (
      // No child to open and nothing to expand, still reachable: focus opens the hover card (S3).
      <div role="group" tabIndex={0} aria-label={`${subagent.title}, ${word}`} {...props} />
    );
  const body = (
    <SubagentCardBody
      subagent={subagent}
      stale={stale}
      chevron={mode === "static" ? null : open ? "open" : "closed"}
    />
  );

  return (
    <div className={cn("flex min-w-0 flex-col", variant === "framed" && "max-w-140")}>
      {/* The trigger's element kind follows the mode (a static row becomes a
          drill-in button once the roster confirms a child): remount the card so
          its hover listeners bind to the live element, not the replaced one. */}
      <HoverCard key={mode}>
        <HoverCardTrigger render={trigger}>{body}</HoverCardTrigger>
        <HoverCardContent>
          <SubagentHoverContent
            subagent={subagent}
            location={location}
            parentLocation={parentLocation}
            stale={stale}
          />
        </HoverCardContent>
      </HoverCard>
      {mode === "disclosure" ? (
        <div
          id={panelId}
          hidden={!expanded}
          data-slot="subagent-card-nested"
          className="relative flex flex-col gap-px rounded-b-md border border-t-0 border-line-soft bg-canvas py-1.5 pr-2 pl-10.5 before:absolute before:top-1 before:bottom-2.5 before:left-5.25 before:w-px before:bg-line-strong"
        >
          {nested}
        </div>
      ) : null}
    </div>
  );
}
