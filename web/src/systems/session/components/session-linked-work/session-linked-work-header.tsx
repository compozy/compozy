/**
 * The header keeps today's chip, naming the most urgent item, and `+3 PRs` beside it opens the
 * list. Nothing new at rest, nothing stays open.
 */
import { FolderGit2, GitPullRequest, Link2, Unlink } from "@compozy/ui/icons";
import type { ReactElement, ReactNode } from "react";

import {
  Button,
  DropdownMenuItem,
  DropdownMenuLabel,
  Icon,
  Popover,
  PopoverContent,
  PopoverTrigger,
  cn,
} from "@compozy/ui";

import { useSessionTopbarClipped } from "../../hooks/use-session-topbar-clipped";
import {
  isRolledUp,
  linkFacts,
  type SessionLinkedWorkItem,
  type SessionRunsIn,
} from "../../lib/session-linked-work";
import type { SessionPullRequestFacts } from "../../lib/session-pull-request";
import {
  SessionPullRequestChip,
  type SessionPullRequestChipProps,
} from "../session-pull-request-chip";
import { SESSION_CHIP_CLASS } from "../session-pull-request-chip-style";
import type { SessionLinkActions } from "./session-link-row";
import { SessionLinkedWorkPanel } from "./session-linked-work-panel";

export interface SessionLinkedWorkListProps {
  items: readonly SessionLinkedWorkItem[];
  runsIn: SessionRunsIn;
  agentLabel: string;
  actions: SessionLinkActions;
  /** Starts with the list showing (prototypes). */
  listOpen?: boolean;
  openMenuFor?: string;
  openDetailFor?: string;
}

function SessionLinkedWorkPopover({
  trigger,
  children,
  list,
  testId,
}: {
  trigger: ReactElement;
  children: ReactNode;
  list: SessionLinkedWorkListProps;
  testId: string;
}) {
  return (
    <Popover defaultOpen={list.listOpen}>
      <PopoverTrigger data-testid={testId} render={trigger}>
        {children}
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-120 max-w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0"
      >
        <SessionLinkedWorkPanel
          actions={list.actions}
          agentLabel={list.agentLabel}
          items={list.items}
          openDetailFor={list.openDetailFor}
          openMenuFor={list.openMenuFor}
          runsIn={list.runsIn}
        />
      </PopoverContent>
    </Popover>
  );
}

export interface SessionLinkedWorkStatusProps extends SessionLinkedWorkListProps {
  lead: SessionLinkedWorkItem | null;
  more: string | null;
  /** The session's own facts, for a session with nothing linked or a lead on another branch. */
  ownFacts: SessionPullRequestFacts;
  /** Delivery actions: they act on the session's own branch, so only its chip carries them. */
  ownChip: Pick<
    SessionPullRequestChipProps,
    "exitPlan" | "exitRunning" | "onExitAction" | "onRefresh" | "refreshing"
  >;
  leadIsOwn: boolean;
  /** Opens the worktree a linked item lives in, when this client can show it. */
  worktreeAction?: (item: SessionLinkedWorkItem) => (() => void) | undefined;
  /** Opens the chip's own menu (prototypes). */
  chipMenuOpen?: boolean;
  onClippedChange?: (clipped: boolean) => void;
}

/** The chip's link rows, for the case the list has nothing more to say. */
function ChipLinkActions({
  lead,
  runsIn,
  actions,
  worktreeAction,
}: Pick<SessionLinkedWorkStatusProps, "lead" | "runsIn" | "actions" | "worktreeAction">) {
  const openWorktree = lead ? worktreeAction?.(lead) : undefined;
  return (
    <>
      <DropdownMenuLabel className="tracking-normal normal-case">
        This session runs in {runsIn.name}
      </DropdownMenuLabel>
      {lead?.worktree_name && openWorktree ? (
        <DropdownMenuItem onClick={openWorktree}>
          <Icon as={FolderGit2} size="sm" />
          Worktree {lead.worktree_name}
        </DropdownMenuItem>
      ) : null}
      <DropdownMenuItem onClick={actions.onLink}>
        <Icon as={Link2} size="sm" />
        Link another…
      </DropdownMenuItem>
      {lead && !isRolledUp(lead) ? (
        <DropdownMenuItem onClick={() => actions.onUnlink(lead)}>
          <Icon as={Unlink} size="sm" />
          Unlink from this session
        </DropdownMenuItem>
      ) : null}
    </>
  );
}

/** The head's status zone: the lead's chip, then `+3 PRs` when there is more to see. */
export function SessionLinkedWorkStatus(props: SessionLinkedWorkStatusProps) {
  const { lead, more, ownFacts, ownChip, leadIsOwn, actions, onClippedChange } = props;
  const anchor = useSessionTopbarClipped(onClippedChange);
  const own = lead === null || leadIsOwn;
  return (
    <span className="flex min-w-0 shrink-0 items-center gap-1" ref={anchor}>
      <SessionPullRequestChip
        {...(own ? ownChip : { onRefresh: actions.onRefresh, refreshing: actions.refreshing })}
        controllable={actions.controllable}
        defaultMenuOpen={props.chipMenuOpen}
        facts={lead ? linkFacts(lead) : ownFacts}
        menuFooter={
          more ? undefined : (
            <ChipLinkActions
              actions={actions}
              lead={lead}
              runsIn={props.runsIn}
              worktreeAction={props.worktreeAction}
            />
          )
        }
        onPrompt={actions.onPrompt ? (_kind, text) => actions.onPrompt?.(text) : undefined}
      />
      {more ? (
        <SessionLinkedWorkPopover
          list={props}
          testId="session-links-more"
          trigger={
            <Button
              className={cn(SESSION_CHIP_CLASS, "pr-2.5")}
              size="sm"
              type="button"
              variant="secondary"
            />
          }
        >
          {more}
        </SessionLinkedWorkPopover>
      ) : null}
    </span>
  );
}

/** Shown once the head has clipped the status zone: the list stays reachable from the actions. */
export function SessionLinkedWorkNarrowEntry(list: SessionLinkedWorkListProps) {
  const { length } = list.items;
  if (length === 0) return null;
  return (
    <SessionLinkedWorkPopover
      list={list}
      testId="session-links-narrow"
      trigger={
        <Button
          aria-label={`${length} linked to this session`}
          size="sm"
          type="button"
          variant="quiet"
        />
      }
    >
      <Icon as={GitPullRequest} data-icon="inline-start" size="sm" />
      {length}
    </SessionLinkedWorkPopover>
  );
}
