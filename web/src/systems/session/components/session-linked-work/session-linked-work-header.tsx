/**
 * The header keeps today's chip for a single item; past one, a single `8 linked PRs` chip opens the
 * list, so no one item is promoted over the rest. Nothing new at rest, nothing stays open.
 */
import { GitPullRequest } from "@compozy/ui/icons";
import { useState, type ReactElement, type ReactNode } from "react";

import { Button, Icon, Pill, Popover, PopoverContent, PopoverTrigger, cn } from "@compozy/ui";

import { useSessionTopbarClipped } from "../../hooks/use-session-topbar-clipped";
import {
  isRolledUp,
  linkFacts,
  linkedCountLabel,
  needsYouLabel,
  type LinkedWorkCount,
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
  /** Past one item the count replaces the lead's chip. */
  count: LinkedWorkCount;
  /** The session's own facts, for a session with nothing linked or a lead on another branch. */
  ownFacts: SessionPullRequestFacts;
  /** Delivery actions: they act on the session's own branch, so only its chip carries them. */
  ownChip: Pick<
    SessionPullRequestChipProps,
    "exitPlan" | "exitRunning" | "onExitAction" | "onRefresh" | "refreshing" | "onReviewChanges"
  >;
  /** Opens the chip's own menu (prototypes). */
  chipMenuOpen?: boolean;
  onClippedChange?: (clipped: boolean) => void;
}

/** Several items as one chip: no one item is promoted, and the list says each one's status. */
function SessionLinkedWorkCount({
  list,
  count,
  onOpenChange,
}: {
  list: SessionLinkedWorkListProps;
  count: LinkedWorkCount;
  onOpenChange: (open: boolean) => void;
}) {
  const label = linkedCountLabel(count);
  const attention = count.needsYou > 0;
  return (
    <SessionLinkedWorkPopover
      list={list}
      onOpenChange={onOpenChange}
      testId="session-links-summary"
      trigger={
        <Button
          aria-label={attention ? `${label}, ${needsYouLabel(count.needsYou)}` : label}
          className={cn(SESSION_CHIP_CLASS, "pr-2.5")}
          size="sm"
          type="button"
          variant="secondary"
        />
      }
    >
      <Icon as={GitPullRequest} size="sm" />
      {label}
      {attention ? <Pill.Dot size="sm" tone="attn" /> : null}
    </SessionLinkedWorkPopover>
  );
}

/** A lone item, or the session's own facts: today's full chip, with the link actions in its menu. */
function SessionLinkedWorkChip(props: SessionLinkedWorkStatusProps) {
  const { lead, ownFacts, ownChip, actions } = props;
  const own = lead === null || actions.isOwn(lead);
  return (
    <SessionPullRequestChip
      {...(own
        ? ownChip
        : { onRefresh: () => actions.onRefresh(lead), refreshing: actions.refreshing })}
      controllable={own && actions.controllable}
      defaultMenuOpen={props.chipMenuOpen}
      facts={lead ? linkFacts(lead) : ownFacts}
      link={lead ? chipLink(lead, actions) : undefined}
      onPrompt={own && actions.onPrompt ? (_kind, text) => actions.onPrompt?.(text) : undefined}
    />
  );
}

/** The lone item's own verbs in the chip menu: its worktree, its child session, link and unlink. */
function chipLink(
  lead: SessionLinkedWorkItem,
  actions: SessionLinkActions
): SessionPullRequestChipProps["link"] {
  const openWorktree = actions.worktreeAction?.(lead);
  const child = lead.source_session_id;
  return {
    ...(lead.worktree_name && openWorktree
      ? { worktree: { name: lead.worktree_name, runsHere: lead.runs_here, onOpen: openWorktree } }
      : {}),
    ...(child ? { onOpenSession: () => actions.onOpenSession(child) } : {}),
    onLink: actions.onLink,
    ...(isRolledUp(lead) ? {} : { onUnlink: () => actions.onUnlink(lead) }),
  };
}

/** The head's status zone: the lone item's chip, or `8 linked PRs` when there are several. */
export function SessionLinkedWorkStatus(props: SessionLinkedWorkStatusProps) {
  const anchor = useSessionTopbarClipped(props.onClippedChange);
  // An unlink can leave one item: the open list keeps its trigger so its Undo keeps its context.
  // A prototype can start with the list open; only the count can be its trigger.
  const [listOpen, setListOpen] = useState(props.count.total > 1 && (props.listOpen ?? false));
  return (
    <span className="flex min-w-0 shrink-0 items-center gap-1" ref={anchor}>
      {props.count.total > 1 || listOpen ? (
        <SessionLinkedWorkCount count={props.count} list={props} onOpenChange={setListOpen} />
      ) : (
        <SessionLinkedWorkChip {...props} />
      )}
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
