import { useState, type MouseEvent } from "react";
import { Square } from "lucide-react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
  StateGlyph,
  cn,
} from "@compozy/ui";

import {
  SessionInspectorDisclosureHead,
  SessionInspectorSection,
  SessionInspectorSectionHead,
} from "../session-inspector-section";
import { SubagentAvatar } from "./subagent-avatar";
import type { SubagentOpenOptions } from "./subagent-card";
import {
  SUBAGENT_ROW_STOP_FAILED,
  SUBAGENT_STATUS_WORD,
  isSubagentStoppable,
  subagentRosterGroups,
  subagentRosterTitle,
} from "./subagent-format";
import { SubagentHoverContent } from "./subagent-hover-content";
import { SubagentRowTrail } from "./subagent-row-trail";
import type { SubagentView } from "./types";
import { useSubagentStop } from "./use-subagent-stop";

const SUBAGENT_ROSTER_FIRST_PAGE = 6;
const SUBAGENT_ROSTER_PAGE = 12;

export interface SessionInspectorSubagentsSectionProps {
  /** Direct children of this session; grandchildren are reached by opening a child. */
  subagents: readonly SubagentView[];
  onOpen?: (subagent: SubagentView, options: SubagentOpenOptions) => void;
  /** Cancels one live delegated subagent. */
  onStop?: (subagent: SubagentView) => Promise<unknown>;
  defaultPreviousOpen?: boolean;
  stale?: boolean;
}

interface RosterRowProps {
  subagent: SubagentView;
  onOpen?: SessionInspectorSubagentsSectionProps["onOpen"];
  onStop?: SessionInspectorSubagentsSectionProps["onStop"];
  stale: boolean;
}

function RosterRow({ subagent, onOpen, onStop, stale }: RosterRowProps) {
  const stoppable = isSubagentStoppable(subagent) && onStop !== undefined;
  const { pending, stop } = useSubagentStop(
    stoppable ? () => onStop(subagent) : undefined,
    SUBAGENT_ROW_STOP_FAILED
  );
  const drillable = subagent.child_session_id !== null && onOpen !== undefined;
  const word = SUBAGENT_STATUS_WORD[subagent.status];
  const mainClass =
    "grid min-h-7.5 min-w-0 grid-cols-[20px_minmax(0,1fr)] items-center gap-2 rounded-sm py-1 pl-1.5 text-left outline-none focus-visible:shadow-focus-inset";

  const handleOpen = (event: MouseEvent<HTMLButtonElement>) => {
    onOpen?.(subagent, { newWindow: event.metaKey || event.ctrlKey });
  };

  return (
    <li
      data-slot="subagent-roster-row"
      data-status={subagent.status}
      data-stopping={pending ? "true" : undefined}
      className="group/roster-row grid grid-cols-[minmax(0,1fr)_auto] items-center gap-1 rounded-sm pr-1.5 text-fg hover:bg-surface-2 has-focus-visible:bg-surface-2"
    >
      <HoverCard>
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
              <div aria-description={word} className={mainClass} />
            )
          }
        >
          <SubagentAvatar
            provider={subagent.runtime.provider}
            status={subagent.status}
            size="sm"
            surface="rail"
            still={stale}
          />
          <span className="truncate text-transcript-meta">{subagent.title}</span>
        </HoverCardTrigger>
        <HoverCardContent side="left" align="start">
          <SubagentHoverContent subagent={subagent} stale={stale} />
        </HoverCardContent>
      </HoverCard>
      <span className="inline-flex shrink-0 items-center gap-1">
        <SubagentRowTrail
          subagent={subagent}
          stale={stale}
          className={cn(
            stoppable && "group-hover/roster-row:hidden group-has-focus-visible/roster-row:hidden",
            pending && "hidden"
          )}
        />
        {stoppable ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={pending ? "Stopping subagent" : "Stop subagent"}
            disabled={pending}
            onClick={stop}
            className={cn(
              "text-muted",
              !pending &&
                "hidden group-hover/roster-row:inline-flex group-has-focus-visible/roster-row:inline-flex"
            )}
          >
            {pending ? <StateGlyph size="sm" state="running" /> : <Square aria-hidden="true" />}
          </Button>
        ) : null}
      </span>
    </li>
  );
}

/**
 * Inspector roster of this session's subagents (nav VC-01/03): failed pinned on
 * top, then live (waiting first), then the collapsed `Previous subagents (N)`.
 * Six rows first, then `Show 12 more` per page. Absent when there are none.
 */
export function SessionInspectorSubagentsSection({
  subagents,
  onOpen,
  onStop,
  defaultPreviousOpen = false,
  stale = false,
}: SessionInspectorSubagentsSectionProps) {
  const [limit, setLimit] = useState(SUBAGENT_ROSTER_FIRST_PAGE);
  const [previousOpen, setPreviousOpen] = useState(defaultPreviousOpen);
  if (subagents.length === 0) return null;

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
    <SessionInspectorSection data-testid="session-inspector-subagents">
      <SessionInspectorSectionHead meta={subagents.length}>
        {subagentRosterTitle(groups.live.length)}
      </SessionInspectorSectionHead>
      <div className="-mx-1.5 flex max-h-79 flex-col gap-px overflow-auto pb-0.5">
        {shownPinned.length > 0 ? (
          <ul className="flex flex-col gap-px">{shownPinned.map(row)}</ul>
        ) : null}
        {groups.previous.length > 0 ? (
          <Collapsible open={previousOpen} onOpenChange={setPreviousOpen}>
            <SessionInspectorDisclosureHead className="mt-1 px-1.5 py-1.5">
              Previous subagents ({groups.previous.length})
            </SessionInspectorDisclosureHead>
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
    </SessionInspectorSection>
  );
}
