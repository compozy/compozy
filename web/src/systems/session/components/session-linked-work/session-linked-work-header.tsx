/**
 * The header keeps today's chip, naming the most urgent item, and `+3 PRs` beside it opens the
 * list. Nothing new at rest, nothing stays open.
 */
import { ArrowUpRight, FolderGit2, GitPullRequest, Link2, Unlink } from "@compozy/ui/icons";
import { useState, type ReactElement, type ReactNode } from "react";

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

function reachesToast(event: Event) {
  const related = "relatedTarget" in event ? event.relatedTarget : null;
  return [event.target, related].some(
    node => node instanceof Element && node.closest("[data-sonner-toast]") !== null
  );
}

function SessionLinkedWorkPopover({
  trigger,
  children,
  list,
  testId,
  onOpenChange,
}: {
  trigger: ReactElement;
  children: ReactNode;
  list: SessionLinkedWorkListProps;
  testId: string;
  onOpenChange?: (open: boolean) => void;
}) {
  const [open, setOpenState] = useState(list.listOpen ?? false);
  const setOpen = (next: boolean) => {
    setOpenState(next);
    onOpenChange?.(next);
  };
  const handleOpenChange = (next: boolean, details: { reason: string; event: Event }) => {
    // Undo lives in the toast, outside this popover: reaching it must leave the list showing.
    const leaving = details.reason === "outside-press" || details.reason === "focus-out";
    if (!next && leaving && reachesToast(details.event)) return;
    setOpen(next);
  };
  const actions = {
    ...list.actions,
    // The link dialog takes over: left open, this popover would sit on top of it.
    onLink: () => {
      setOpen(false);
      list.actions.onLink();
    },
  };
  return (
    <Popover onOpenChange={handleOpenChange} open={open}>
      <PopoverTrigger data-testid={testId} render={trigger}>
        {children}
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-120 max-w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0"
      >
        <SessionLinkedWorkPanel
          actions={actions}
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
}: Pick<SessionLinkedWorkStatusProps, "runsIn" | "actions" | "worktreeAction"> & {
  lead: SessionLinkedWorkItem;
}) {
  const openWorktree = worktreeAction?.(lead);
  const child = lead.source_session_id;
  return (
    <>
      <DropdownMenuLabel className="tracking-normal normal-case">
        This session runs in {runsIn.name}
      </DropdownMenuLabel>
      {lead.worktree_name && openWorktree ? (
        <DropdownMenuItem onClick={openWorktree}>
          <Icon as={FolderGit2} size="sm" />
          Worktree {lead.worktree_name}
        </DropdownMenuItem>
      ) : null}
      {child ? (
        <DropdownMenuItem onClick={() => actions.onOpenSession(child)}>
          <Icon as={ArrowUpRight} size="sm" />
          Open session
        </DropdownMenuItem>
      ) : null}
      <DropdownMenuItem onClick={actions.onLink}>
        <Icon as={Link2} size="sm" />
        Link another…
      </DropdownMenuItem>
      {isRolledUp(lead) ? null : (
        <DropdownMenuItem onClick={() => actions.onUnlink(lead)}>
          <Icon as={Unlink} size="sm" />
          Unlink from this session
        </DropdownMenuItem>
      )}
    </>
  );
}

/** The head's status zone: the lead's chip, then `+3 PRs` when there is more to see. */
export function SessionLinkedWorkStatus(props: SessionLinkedWorkStatusProps) {
  const { lead, more, ownFacts, ownChip, actions, onClippedChange } = props;
  const anchor = useSessionTopbarClipped(onClippedChange);
  const own = lead === null || actions.isOwn(lead);
  // An unlink can drop the last overflow item: the open list stays so its Undo keeps its context.
  const [listOpen, setListOpen] = useState(false);
  return (
    <span className="flex min-w-0 shrink-0 items-center gap-1" ref={anchor}>
      <SessionPullRequestChip
        {...(own
          ? ownChip
          : { onRefresh: () => actions.onRefresh(lead), refreshing: actions.refreshing })}
        controllable={own && actions.controllable}
        defaultMenuOpen={props.chipMenuOpen}
        facts={lead ? linkFacts(lead) : ownFacts}
        menuFooter={
          lead && !more ? (
            <ChipLinkActions
              actions={actions}
              lead={lead}
              runsIn={props.runsIn}
              worktreeAction={props.worktreeAction}
            />
          ) : undefined
        }
        onPrompt={own && actions.onPrompt ? (_kind, text) => actions.onPrompt?.(text) : undefined}
      />
      {more || listOpen ? (
        <SessionLinkedWorkPopover
          list={props}
          onOpenChange={setListOpen}
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
          {more ?? "Linked work"}
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
