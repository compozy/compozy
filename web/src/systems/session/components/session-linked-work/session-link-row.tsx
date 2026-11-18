/**
 * One linked item in two lines: what it is, then how it's doing. Everything else waits on hover:
 * the tooltip carries the chip's detail, and the number gives way to the row's menu.
 */
import { FolderGit2, FolderX, GitBranch, MoreHorizontal, type LucideIcon } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuTrigger,
  Icon,
  Item,
  ItemActions,
  ItemContent,
  ItemMedia,
  ItemTitle,
  Pill,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import {
  isRolledUp,
  linkCaution,
  linkChip,
  linkNumber,
  linkOriginLabel,
  linkSourceLabel,
  linkStatus,
  linkTitle,
  type SessionLinkedWorkItem,
} from "../../lib/session-linked-work";
import {
  PULL_REQUEST_SHAPE_ICON,
  SESSION_PULL_REQUEST_DETAILS_CLASS,
} from "../session-pull-request-chip-style";
import { SessionPullRequestDetails } from "../session-pull-request-details";
import { SessionPullRequestMenu } from "../session-pull-request-menu";

/** What a row can do. Absent handlers leave the matching action out. */
export interface SessionLinkActions {
  /** User-controllable sessions get agent prompts, on their own branch's item only. */
  controllable: boolean;
  /** Agent prompts say "this branch", so they only hold for the session's own branch. */
  isOwn: (item: SessionLinkedWorkItem) => boolean;
  refreshing: boolean;
  onLink: () => void;
  onUnlink: (item: SessionLinkedWorkItem) => void;
  /** Opens the child session a rolled-up item came from. */
  onOpenSession: (sessionId: string) => void;
  onRefresh: (item: SessionLinkedWorkItem) => void;
  onPrompt?: (text: string) => void;
  /** Opens the session's Review panel on Changes; offered on its own branch's item only. */
  onReviewChanges?: () => void;
  /** Opens the worktree a linked item lives in, when this client can show it. */
  worktreeAction?: (item: SessionLinkedWorkItem) => (() => void) | undefined;
}

function Fact({ children, truncate = false }: { children: ReactNode; truncate?: boolean }) {
  return (
    <>
      <span aria-hidden="true" className="text-faint">
        ·
      </span>
      <span className={truncate ? "flex min-w-0 items-center gap-1" : "shrink-0"}>{children}</span>
    </>
  );
}

/** The worktree only when it adds something: the row's title already names a bare one. */
function WorktreeFact({ item }: { item: SessionLinkedWorkItem }) {
  const { worktree_name: name, worktree_removed: removed } = item;
  if (!removed && (!name || linkTitle(item) === name)) return null;
  return (
    <Fact truncate>
      <Icon as={removed ? FolderX : FolderGit2} className="size-3 shrink-0" />
      <span className="truncate">{removed ? "Worktree removed" : name}</span>
    </Fact>
  );
}

/** Status, then where it lives, then only the exceptions. */
function SessionLinkFacts({
  item,
  agentLabel,
}: {
  item: SessionLinkedWorkItem;
  agentLabel: string;
}) {
  const status = linkStatus(item);
  const caution = linkCaution(item);
  const source = linkSourceLabel(item, agentLabel);
  const origin = linkOriginLabel(item);
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-eyebrow text-subtle">
      <Pill className="shrink-0" form="plain" tone={status.tone}>
        {status.tone === "neutral" ? null : <Pill.Dot />}
        {status.label}
      </Pill>
      <WorktreeFact item={item} />
      {caution ? <Fact truncate>{caution}</Fact> : null}
      {origin ? (
        <Fact truncate>
          <span className="truncate" data-slot="session-link-origin">
            {origin}
          </span>
        </Fact>
      ) : null}
      {item.runs_here ? (
        <Pill className="shrink-0" size="sm">
          Runs here
        </Pill>
      ) : null}
      {source ? <Fact>{source}</Fact> : null}
    </span>
  );
}

function SessionLinkRowMenu({
  item,
  defaultOpen,
  actions,
}: {
  item: SessionLinkedWorkItem;
  defaultOpen: boolean;
  actions: SessionLinkActions;
}) {
  const own = actions.isOwn(item);
  const { onPrompt } = actions;
  const openWorktree = actions.worktreeAction?.(item);
  const child = item.source_session_id;
  return (
    <DropdownMenu defaultOpen={defaultOpen}>
      <DropdownMenuTrigger
        aria-label={`Actions for ${linkTitle(item)}`}
        render={
          <Button
            className="opacity-0 group-focus-within/item:opacity-100 group-hover/item:opacity-100 data-popup-open:opacity-100"
            size="icon-xs"
            type="button"
            variant="quiet"
          />
        }
      >
        <Icon as={MoreHorizontal} size="sm" />
      </DropdownMenuTrigger>
      <SessionPullRequestMenu
        branch={item.branch}
        controllable={actions.controllable && own}
        observedAt={item.pull_request_lookup?.observed_at}
        onLink={actions.onLink}
        onPrompt={onPrompt ? (_kind, text) => onPrompt(text) : undefined}
        onRefresh={() => actions.onRefresh(item)}
        pullRequest={item.pull_request}
        refreshing={actions.refreshing}
        {...(own && actions.onReviewChanges ? { onReviewChanges: actions.onReviewChanges } : {})}
        {...(item.worktree_name && openWorktree
          ? {
              worktree: {
                name: item.worktree_name,
                runsHere: item.runs_here,
                onOpen: openWorktree,
              },
            }
          : {})}
        {...(child ? { onOpenSession: () => actions.onOpenSession(child) } : {})}
        {...(isRolledUp(item) ? {} : { onUnlink: () => actions.onUnlink(item) })}
      />
    </DropdownMenu>
  );
}

function rowIcon(item: SessionLinkedWorkItem): LucideIcon {
  const chip = linkChip(item);
  if (chip.kind === "pull-request") return PULL_REQUEST_SHAPE_ICON[chip.shape];
  return item.worktree_name ? FolderGit2 : GitBranch;
}

const ROW_TARGET =
  "min-w-0 truncate text-left outline-none after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring";

/** The row's target: the child for a rolled-up item, else the pull request, else plain text. */
function rowTarget(item: SessionLinkedWorkItem, actions: SessionLinkActions) {
  const title = linkTitle(item);
  const child = item.source_session_id;
  if (child) {
    return (
      <button
        aria-label={`Open ${title} in its session`}
        className={ROW_TARGET}
        onClick={() => actions.onOpenSession(child)}
        type="button"
      />
    );
  }
  const url = item.pull_request?.url;
  if (url) {
    return (
      <a aria-label={title} className={ROW_TARGET} href={url} rel="noreferrer" target="_blank" />
    );
  }
  return <span className="min-w-0 truncate" />;
}

export interface SessionLinkRowProps {
  item: SessionLinkedWorkItem;
  agentLabel: string;
  actions: SessionLinkActions;
  /** Pins the row's menu open for review. */
  menuOpen?: boolean;
  /** Pins the row's details open for review. */
  detailOpen?: boolean;
}

export function SessionLinkRow({
  item,
  agentLabel,
  actions,
  menuOpen = false,
  detailOpen = false,
}: SessionLinkRowProps) {
  const number = linkNumber(item);
  return (
    <Item
      className="flex-nowrap gap-3"
      data-provenance={item.provenance}
      data-rolled-up={isRolledUp(item) ? "true" : undefined}
      data-slot="session-link-row"
      selectable
      size="sm"
    >
      <ItemMedia className="size-8 rounded-md bg-surface-2 text-muted-foreground">
        <Icon as={rowIcon(item)} className="size-4" />
      </ItemMedia>
      <ItemContent className="min-w-0 gap-1">
        <ItemTitle className="w-full">
          <Tooltip defaultOpen={detailOpen}>
            <TooltipTrigger render={rowTarget(item, actions)}>{linkTitle(item)}</TooltipTrigger>
            <TooltipContent
              align="start"
              className={SESSION_PULL_REQUEST_DETAILS_CLASS}
              side="left"
              // The title sits past the icon; the details clear the popover's edge instead.
              sideOffset={72}
            >
              <SessionPullRequestDetails model={linkChip(item)} />
            </TooltipContent>
          </Tooltip>
        </ItemTitle>
        <SessionLinkFacts agentLabel={agentLabel} item={item} />
      </ItemContent>
      <ItemActions className="relative z-10 grid items-center justify-items-end">
        {number === null ? null : (
          <span className="pointer-events-none col-start-1 row-start-1 font-mono text-mono-id tracking-mono-id text-faint tabular-nums group-focus-within/item:opacity-0 group-hover/item:opacity-0 group-has-data-popup-open/item:opacity-0">
            #{number}
          </span>
        )}
        <span className="col-start-1 row-start-1">
          <SessionLinkRowMenu actions={actions} defaultOpen={menuOpen} item={item} />
        </span>
      </ItemActions>
    </Item>
  );
}
