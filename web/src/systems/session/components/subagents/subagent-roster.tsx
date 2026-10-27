import { useState, type MouseEvent } from "react";
import { ChevronRight, Square } from "lucide-react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
  StateGlyph,
  cn,
} from "@compozy/ui";

import { SubagentAvatar } from "./subagent-avatar";
import { SubagentBranch, type SubagentOpenOptions } from "./subagent-card";
import {
  SUBAGENT_ROW_STOP_FAILED,
  SUBAGENT_STATUS_WORD,
  isSubagentStoppable,
  subagentRosterGroups,
} from "./subagent-format";
import { SubagentHoverContent } from "./subagent-hover-content";
import {
  SUBAGENT_ROSTER_LIST_FRAME,
  SUBAGENT_ROSTER_MAIN_BRANCH,
  SUBAGENT_ROSTER_MAIN_FRAME,
  SUBAGENT_ROSTER_MAIN_PLAIN,
  SUBAGENT_ROSTER_ROW_FRAME,
} from "./subagent-roster-frame";
import { SubagentPullRequestLink, SubagentRowTrail } from "./subagent-row-trail";
import { subagentPullRequest, subagentWorktree } from "./subagent-worktree-format";
import type { SubagentView } from "./types";
import { useSubagentStop } from "./use-subagent-stop";

const SUBAGENT_ROSTER_FIRST_PAGE = 6;
const SUBAGENT_ROSTER_PAGE = 12;

type SubagentHoverSide = "left" | "right";

export interface SubagentRosterProps {
  /** Direct children of this session; grandchildren are reached by opening a child. */
  subagents: readonly SubagentView[];
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
  /** Cancels one live delegated subagent. */
  onStop?: (subagent: SubagentView) => Promise<unknown>;
  defaultPreviousOpen?: boolean;
  stale?: boolean;
  /** Where a row's hover card opens: away from the host's edge. */
  hoverSide?: SubagentHoverSide;
  className?: string;
}

interface RosterRowProps {
  subagent: SubagentView;
  onOpen?: SubagentRosterProps["onOpen"];
  onStop?: SubagentRosterProps["onStop"];
  stale: boolean;
  hoverSide: SubagentHoverSide;
}

interface RosterRowLabelProps {
  subagent: SubagentView;
  branch: string | undefined;
  stale: boolean;
}

// Isolated rows grow a branch line (S5) under the title.
function RosterRowLabel({ subagent, branch, stale }: RosterRowLabelProps) {
  return (
    <>
      <SubagentAvatar
        provider={subagent.runtime.provider}
        status={subagent.status}
        size="sm"
        surface="rail"
        still={stale}
        className={branch ? "mt-px" : undefined}
      />
      {branch ? (
        <span className="flex min-w-0 flex-col gap-px">
          <span className="truncate text-transcript-meta">{subagent.title}</span>
          <SubagentBranch branch={branch} className="min-w-0 text-micro" />
        </span>
      ) : (
        <span className="truncate text-transcript-meta">{subagent.title}</span>
      )}
    </>
  );
}

function RosterRowStop({ pending, onStop }: { pending: boolean; onStop: () => void }) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      aria-label={pending ? "Stopping subagent" : "Stop subagent"}
      disabled={pending}
      onClick={onStop}
      className={cn(
        "text-muted",
        !pending &&
          "hidden group-hover/roster-row:inline-flex group-has-focus-visible/roster-row:inline-flex"
      )}
    >
      {pending ? <StateGlyph size="sm" state="running" /> : <Square aria-hidden="true" />}
    </Button>
  );
}

function RosterRow({ subagent, onOpen, onStop, stale, hoverSide }: RosterRowProps) {
  const stoppable = isSubagentStoppable(subagent) && onStop !== undefined;
  const { pending, stop } = useSubagentStop(
    stoppable ? () => onStop(subagent) : undefined,
    SUBAGENT_ROW_STOP_FAILED,
    [subagent]
  );
  const drillable =
    (subagent.child_session_id !== null || subagent.origin === "provider_native") &&
    onOpen !== undefined;
  const word = SUBAGENT_STATUS_WORD[subagent.status];
  // Isolated rows grow a branch line (S5); the PR link stays a sibling of the open control.
  const branch = subagentWorktree(subagent)?.branch;
  const pullRequest = subagentPullRequest(subagent);
  const mainClass = cn(
    SUBAGENT_ROSTER_MAIN_FRAME,
    branch ? SUBAGENT_ROSTER_MAIN_BRANCH : SUBAGENT_ROSTER_MAIN_PLAIN
  );

  const handleOpen = (event: MouseEvent<HTMLButtonElement>) => {
    onOpen?.(subagent, { newWindow: event.metaKey || event.ctrlKey });
  };

  return (
    <li
      data-slot="subagent-roster-row"
      data-status={subagent.status}
      data-stopping={pending ? "true" : undefined}
      className={SUBAGENT_ROSTER_ROW_FRAME}
    >
      {/* Remount when the trigger switches between button and group row, so the
          hover listeners bind to the live element. */}
      <HoverCard key={drillable ? "drill" : "static"}>
        <HoverCardTrigger
          render={
            drillable ? (
              <button
                type="button"
                aria-label={`Open ${subagent.title}`}
                aria-description={word}
                className={mainClass}
                onClick={handleOpen}
              />
            ) : (
              // Not drillable, still reachable: focus opens the hover card (S3).
              <div
                role="group"
                tabIndex={0}
                aria-label={`${subagent.title}, ${word}`}
                className={mainClass}
              />
            )
          }
        >
          <RosterRowLabel subagent={subagent} branch={branch} stale={stale} />
        </HoverCardTrigger>
        <HoverCardContent side={hoverSide} align="start">
          <SubagentHoverContent subagent={subagent} stale={stale} />
        </HoverCardContent>
      </HoverCard>
      <span
        className={cn("inline-flex shrink-0 items-center gap-1", branch && "self-start pt-1.5")}
      >
        {pullRequest ? <SubagentPullRequestLink pr={pullRequest} size="row" /> : null}
        <SubagentRowTrail
          subagent={subagent}
          stale={stale}
          className={cn(
            stoppable && "group-hover/roster-row:hidden group-has-focus-visible/roster-row:hidden",
            pending && "hidden"
          )}
        />
        {stoppable ? <RosterRowStop pending={pending} onStop={stop} /> : null}
      </span>
    </li>
  );
}

/**
 * One session's subagent rows (nav VC-01/03): failed pinned on top, then live
 * (waiting first), then the collapsible `Previous subagents (N)`. Six rows
 * first, then `Show 12 more` per page. The inspector section and the composer
 * popover host it.
 */
export function SubagentRoster({
  subagents,
  onOpen,
  onStop,
  defaultPreviousOpen = false,
  stale = false,
  hoverSide = "left",
  className,
}: SubagentRosterProps) {
  const [limit, setLimit] = useState(SUBAGENT_ROSTER_FIRST_PAGE);
  const [previousOpen, setPreviousOpen] = useState(defaultPreviousOpen);

  const groups = subagentRosterGroups(subagents);
  const pinned = [...groups.failed, ...groups.live];
  const visible = previousOpen ? [...pinned, ...groups.previous] : pinned;
  const remaining = Math.max(0, visible.length - limit);
  const shownPinned = pinned.slice(0, limit);
  const shownPrevious = groups.previous.slice(0, Math.max(0, limit - pinned.length));
  const row = (subagent: SubagentView) => (
    <RosterRow
      key={subagent.id}
      subagent={subagent}
      onOpen={onOpen}
      onStop={onStop}
      stale={stale}
      hoverSide={hoverSide}
    />
  );

  return (
    <>
      <div className={cn(SUBAGENT_ROSTER_LIST_FRAME, className)}>
        {shownPinned.length > 0 ? (
          <ul className="flex flex-col gap-px">{shownPinned.map(row)}</ul>
        ) : null}
        {groups.previous.length > 0 ? (
          <Collapsible open={previousOpen} onOpenChange={setPreviousOpen}>
            <CollapsibleTrigger
              render={
                <button
                  type="button"
                  className="group/previous mt-1 flex w-full items-center gap-1.5 rounded-sm p-1.5 text-left text-transcript-caption text-muted outline-none hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring"
                />
              }
            >
              <ChevronRight
                aria-hidden="true"
                className="size-3 shrink-0 text-subtle transition-transform duration-base ease-out group-aria-expanded/previous:rotate-90 motion-reduce:transition-none"
              />
              Previous subagents ({groups.previous.length})
            </CollapsibleTrigger>
            <CollapsibleContent>
              <ul className="flex flex-col gap-px">{shownPrevious.map(row)}</ul>
            </CollapsibleContent>
          </Collapsible>
        ) : null}
      </div>
      {remaining > 0 ? (
        <Button
          type="button"
          variant="ghost"
          size="xs"
          className="self-start"
          onClick={() => setLimit(current => current + SUBAGENT_ROSTER_PAGE)}
        >
          Show {Math.min(SUBAGENT_ROSTER_PAGE, remaining)} more
        </Button>
      ) : null}
    </>
  );
}
