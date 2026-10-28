import { useId, useState, type MouseEvent, type ReactNode } from "react";
import { ChevronRight, GitBranch } from "lucide-react";

import { HoverCard, HoverCardContent, HoverCardTrigger, Icon, cn } from "@compozy/ui";

import { SubagentAvatar } from "./subagent-avatar";
import {
  isSubagentLive,
  subagentCardLines,
  subagentDisplay,
  type SubagentLineTwoTone,
} from "./subagent-format";
import { SubagentHoverContent } from "./subagent-hover-content";
import { SubagentPullRequestLink, SubagentRowTrail } from "./subagent-row-trail";
import { middleTruncate, subagentPullRequest, subagentWorktree } from "./subagent-worktree-format";
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
  revealOpen?: boolean;
  onRevealClose?: () => void;
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
  if (hasNested) return "disclosure";
  return subagent.origin === "provider_native" && canOpen ? "drill" : "static";
}

/**
 * The card is a container; its open control (avatar + lines) stretches over
 * the whole frame through `::after`, so the PR link in the trail is a sibling
 * of that control, never nested in it (Gap 2).
 */
function cardClassName(mode: SubagentCardMode, variant: "framed" | "flush", open: boolean): string {
  const interactive = mode !== "static";
  return cn(
    "group/subagent-card relative grid w-full items-center gap-2.5 py-1.75 pr-2.5 pl-2.25 text-left text-fg",
    interactive
      ? "grid-cols-[minmax(0,1fr)_auto_14px] transition-colors duration-fast ease-out hover:bg-surface-2 has-[[data-slot=subagent-card-open]:focus-visible]:shadow-focus-ring"
      : "grid-cols-[minmax(0,1fr)_auto]",
    CARD_FRAME[variant],
    variant === "framed" && interactive && "hover:border-line",
    open && "rounded-b-none"
  );
}

const OPEN_CONTROL = {
  "data-slot": "subagent-card-open",
  className:
    "grid min-w-0 grid-cols-[24px_minmax(0,1fr)] items-center gap-2.5 text-left outline-none after:absolute after:inset-0 after:rounded-md",
} as const;

/** The isolated subagent's branch: mono, middle-truncated, full name in `title` (S5). */
export function SubagentBranch({ branch, className }: { branch: string; className?: string }) {
  return (
    <span
      data-slot="subagent-branch"
      title={branch}
      className={cn(
        "inline-flex shrink-0 items-center gap-1 font-mono text-mono-id whitespace-nowrap text-fg-2",
        className
      )}
    >
      <Icon as={GitBranch} size="xs" aria-hidden="true" className="text-subtle" />
      {middleTruncate(branch)}
    </span>
  );
}

/** Avatar · title / line 2 (branch first when isolated); the status lives in the trail. */
function SubagentCardMain({ subagent, settled }: { subagent: SubagentView; settled: boolean }) {
  const lines = subagentCardLines(subagent);
  const branch = subagentWorktree(subagent)?.branch;
  return (
    <>
      <SubagentAvatar provider={subagent.runtime.provider} />
      <span className="flex min-w-0 flex-col gap-px">
        <span
          className={cn(
            "min-w-0 truncate text-transcript-meta font-medium",
            settled ? "text-muted" : "text-fg-strong"
          )}
        >
          {subagent.title}
        </span>
        {branch || lines.lineTwo ? (
          <span
            className="flex min-w-0 items-baseline text-transcript-caption"
            data-slot="subagent-card-line-two"
          >
            {branch ? <SubagentBranch branch={branch} className="self-center" /> : null}
            {branch && lines.lineTwo ? (
              <span aria-hidden="true" className="mx-1.25 shrink-0 text-faint">
                ·
              </span>
            ) : null}
            {lines.lineTwo ? (
              <span className={cn("min-w-0 truncate", LINE_TWO_TONE[lines.lineTwoTone])}>
                {lines.lineTwo}
              </span>
            ) : null}
          </span>
        ) : null}
      </span>
    </>
  );
}

/**
 * One delegation in the parent transcript (transcript VC-01/02): avatar, title,
 * line 2, the status trail (glyph, word, elapsed), and a chevron when there is
 * a child session to open. Isolated subagents add the branch and the PR link (S5).
 */
export function SubagentCard({
  subagent,
  onOpen,
  nested,
  defaultExpanded = false,
  revealOpen = false,
  onRevealClose,
  stale = false,
  variant = "framed",
  location,
  parentLocation,
  className,
}: SubagentCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const panelId = useId();
  const mode = subagentCardMode(subagent, onOpen !== undefined, nested != null);
  const open = mode === "disclosure" && (expanded || revealOpen);
  const display = subagentDisplay(subagent, { stale });
  const word = display.word;
  const pullRequest = subagentPullRequest(subagent);
  const chevron = mode === "static" ? null : open ? "open" : "closed";

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
        {...OPEN_CONTROL}
      />
    ) : mode === "disclosure" ? (
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        aria-description={word}
        onClick={() => {
          if (revealOpen) onRevealClose?.();
          setExpanded(!open);
        }}
        {...OPEN_CONTROL}
      />
    ) : (
      // No child to open and nothing to expand, still reachable: focus opens the hover card (S3).
      <div role="group" tabIndex={0} aria-label={`${subagent.title}, ${word}`} {...OPEN_CONTROL} />
    );

  return (
    <div className={cn("flex min-w-0 flex-col", variant === "framed" && "max-w-140")}>
      {/* The trigger's element kind follows the mode (a static row becomes a
          drill-in button once the roster confirms a child): remount the card so
          its hover listeners bind to the live element, not the replaced one. */}
      <HoverCard key={mode}>
        <div
          data-slot="subagent-card"
          data-subagent-id={subagent.id}
          data-status={subagent.status}
          data-state={display.glyph}
          data-origin={subagent.origin}
          data-settled={isSubagentLive(subagent.status) ? undefined : "true"}
          data-stale={stale ? "true" : undefined}
          className={cn(cardClassName(mode, variant, open), className)}
        >
          <HoverCardTrigger render={trigger}>
            <SubagentCardMain subagent={subagent} settled={display.settled} />
          </HoverCardTrigger>
          <span className="inline-flex shrink-0 items-center gap-2">
            {pullRequest ? <SubagentPullRequestLink pr={pullRequest} /> : null}
            <SubagentRowTrail subagent={subagent} stale={stale} glyph />
          </span>
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
        </div>
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
          hidden={!open}
          data-slot="subagent-card-nested"
          className="relative flex flex-col gap-px rounded-b-md border border-t-0 border-line-soft bg-canvas py-1.5 pr-2 pl-10.5 before:absolute before:top-1 before:bottom-2.5 before:left-5.25 before:w-px before:bg-line-strong"
        >
          {nested}
        </div>
      ) : null}
    </div>
  );
}
