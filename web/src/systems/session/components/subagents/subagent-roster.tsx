import { useSubagentDrawerSelected } from "../../hooks/use-subagent-drawer-selected";
import { SubagentRuntimeMeta } from "./subagent-runtime-meta";
import { useState, type MouseEvent } from "react";
import { ChevronRight, Square } from "@compozy/ui/icons";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  StateGlyph,
  cn,
} from "@compozy/ui";

import { SubagentBranch, type SubagentOpenOptions } from "./subagent-card";
import { SubagentElapsed } from "./subagent-elapsed";
import {
  SUBAGENT_ROW_STOP_FAILED,
  isSubagentStoppable,
  subagentDisplay,
  subagentElapsedClock,
  subagentLiveLine,
  subagentRosterGroups,
} from "./subagent-format";
import {
  SUBAGENT_ROSTER_LIST_FRAME,
  SUBAGENT_ROSTER_MAIN_FRAME,
  SUBAGENT_ROSTER_ROW_FRAME,
} from "./subagent-roster-frame";
import { SubagentPullRequestLink } from "./subagent-row-trail";
import { SubagentTile } from "./subagent-tile";
import { subagentPullRequest, subagentWorktree } from "./subagent-worktree-format";
import type { SubagentView } from "./types";
import { useSubagentStop } from "./use-subagent-stop";

const SUBAGENT_ROSTER_FIRST_PAGE = 6;
const SUBAGENT_ROSTER_PAGE = 12;

export interface SubagentRosterProps {
  /** Direct children of this session; grandchildren are reached by opening a child. */
  subagents: readonly SubagentView[];
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
  /** Cancels one live delegated subagent. */
  onStop?: (subagent: SubagentView) => Promise<unknown>;
  defaultPreviousOpen?: boolean;
  stale?: boolean;
  className?: string;
}

interface RosterRowProps {
  subagent: SubagentView;
  onOpen?: SubagentRosterProps["onOpen"];
  onStop?: SubagentRosterProps["onStop"];
  stale: boolean;
}

/** Title, then the model and kind, then what it is doing now; settled titles dim in place. */
function RosterRowLabel({
  subagent,
  stale,
  settled,
}: {
  subagent: SubagentView;
  stale: boolean;
  settled: boolean;
}) {
  const line = subagentLiveLine(subagent, { stale });
  const branch = subagentWorktree(subagent)?.branch;
  return (
    <>
      <SubagentTile stale={stale} subagent={subagent} />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span
          className={cn(
            "truncate text-small-body font-medium",
            settled ? "text-fg-2" : "text-fg-strong"
          )}
        >
          {subagent.title}
        </span>
        <SubagentRuntimeMeta subagent={subagent} providerMark={false} />
        {line ? (
          <span
            className={cn(
              "truncate text-transcript-meta",
              line.tone === "destructive" ? "text-destructive" : "text-muted-foreground"
            )}
            data-slot="subagent-live-line"
          >
            {line.verb ? <span className="text-success">{line.verb} </span> : null}
            {line.text}
          </span>
        ) : null}
        {branch ? <SubagentBranch branch={branch} className="min-w-0 text-micro" /> : null}
      </span>
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
        "text-muted-foreground",
        !pending &&
          "hidden group-hover/roster-row:inline-flex group-has-focus-visible/roster-row:inline-flex"
      )}
    >
      {pending ? <StateGlyph size="sm" state="running" /> : <Square aria-hidden="true" />}
    </Button>
  );
}

function RosterRow({ subagent, onOpen, onStop, stale }: RosterRowProps) {
  const selected = useSubagentDrawerSelected(subagent.id);
  const stoppable = isSubagentStoppable(subagent) && onStop !== undefined;
  const { pending, stop } = useSubagentStop(
    stoppable ? () => onStop(subagent) : undefined,
    SUBAGENT_ROW_STOP_FAILED,
    [subagent]
  );
  const display = subagentDisplay(subagent, { stale });
  const clock = subagentElapsedClock(subagent, { stale });
  const pullRequest = subagentPullRequest(subagent);
  const label = <RosterRowLabel settled={display.settled} stale={stale} subagent={subagent} />;

  const handleOpen = (event: MouseEvent<HTMLButtonElement>) => {
    onOpen?.(subagent, { newWindow: event.metaKey || event.ctrlKey });
  };

  return (
    <li
      data-slot="subagent-roster-row"
      data-status={subagent.status}
      data-state={display.glyph}
      data-stopping={pending ? "true" : undefined}
      data-subagent-id={subagent.id}
      className={cn(
        SUBAGENT_ROSTER_ROW_FRAME,
        selected && "bg-selected ring-1 ring-inset ring-line-strong"
      )}
    >
      {onOpen ? (
        <button
          type="button"
          aria-label={`Open ${subagent.title}`}
          aria-description={display.word}
          aria-current={selected ? "true" : undefined}
          className={SUBAGENT_ROSTER_MAIN_FRAME}
          onClick={handleOpen}
        >
          {label}
        </button>
      ) : (
        <div className={SUBAGENT_ROSTER_MAIN_FRAME}>{label}</div>
      )}
      <span className="inline-flex shrink-0 items-center gap-1 pt-2">
        {pullRequest ? <SubagentPullRequestLink pr={pullRequest} size="row" /> : null}
        <SubagentElapsed
          className="font-sans text-transcript-meta text-muted-foreground"
          clock={clock}
        />
        {onOpen ? (
          <ChevronRight
            aria-hidden="true"
            className={cn(
              "size-3.5 shrink-0 text-subtle",
              stoppable &&
                "group-hover/roster-row:hidden group-has-focus-visible/roster-row:hidden",
              pending && "hidden"
            )}
          />
        ) : null}
        {stoppable ? <RosterRowStop pending={pending} onStop={stop} /> : null}
      </span>
    </li>
  );
}

/**
 * One session's subagent rows (nav VC-01/03): failed pinned on top, then live
 * (waiting first), then the collapsible `Previous subagents (N)`. Six rows
 * first, then `Show 12 more` per page. The inspector section and the composer
 * panel host it.
 */
export function SubagentRoster({
  subagents,
  onOpen,
  onStop,
  defaultPreviousOpen = false,
  stale = false,
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
                  className="group/previous mt-1 flex w-full items-center gap-1.5 rounded-sm p-1.5 text-left text-transcript-caption text-muted-foreground outline-none hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring"
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
