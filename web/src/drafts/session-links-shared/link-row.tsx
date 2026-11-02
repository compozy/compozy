/**
 * One linked item in two lines: what it is, then how it's doing. Everything else waits on hover:
 * the tooltip carries the chip's detail, and the number gives way to the row's menu.
 */
import {
  FolderGit2,
  FolderX,
  GitBranch,
  MoreHorizontal,
  Unlink,
  type LucideIcon,
} from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuItem,
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
  PULL_REQUEST_SHAPE_ICON,
  SESSION_PULL_REQUEST_DETAILS_CLASS,
} from "@/systems/session/components/session-pull-request-chip-style";
import { SessionPullRequestDetails } from "@/systems/session/components/session-pull-request-details";
import { SessionPullRequestMenu } from "@/systems/session/components/session-pull-request-menu";
import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import { linkCaution, linkChip, linkSourceLabel, linkStatus, linkTitle } from "./link-model";

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
function WorktreeFact({ link }: { link: SessionLink }) {
  const { worktree } = link;
  if (!worktree || (!link.title && worktree.state === "ready")) return null;
  const missing = worktree.state === "missing";
  return (
    <Fact truncate>
      <Icon as={missing ? FolderX : FolderGit2} className="size-3 shrink-0" />
      <span className="truncate">{missing ? "Worktree removed" : worktree.name}</span>
    </Fact>
  );
}

/**
 * Status, then where it lives, then only the exceptions. Without an `agentLabel` it leaves out who
 * added the item, as the link dialog does: nothing there is linked yet.
 */
export function LinkFacts({ link, agentLabel }: { link: SessionLink; agentLabel?: string }) {
  const status = linkStatus(link);
  const caution = linkCaution(link);
  const source = agentLabel ? linkSourceLabel(link, agentLabel) : null;
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-eyebrow text-subtle">
      <Pill className="shrink-0" form="plain" tone={status.tone}>
        {status.tone === "neutral" ? null : <Pill.Dot />}
        {status.label}
      </Pill>
      <WorktreeFact link={link} />
      {caution ? <Fact truncate>{caution}</Fact> : null}
      {link.runsHere ? (
        <Pill className="shrink-0" size="sm">
          Runs here
        </Pill>
      ) : null}
      {source ? <Fact>{source}</Fact> : null}
    </span>
  );
}

function LinkRowMenu({
  link,
  defaultOpen,
  onUnlink,
}: {
  link: SessionLink;
  defaultOpen: boolean;
  onUnlink: () => void;
}) {
  return (
    <DropdownMenu defaultOpen={defaultOpen}>
      <DropdownMenuTrigger
        aria-label={`Actions for ${linkTitle(link)}`}
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
        controllable
        exitRunning={false}
        footer={
          <DropdownMenuItem onClick={onUnlink}>
            <Unlink aria-hidden="true" />
            Unlink from this session
          </DropdownMenuItem>
        }
        observedAt={link.facts.pull_request_lookup?.observed_at}
        pullRequest={link.facts.pull_request}
        refreshing={false}
      />
    </DropdownMenu>
  );
}

function rowIcon(link: SessionLink): LucideIcon {
  const chip = linkChip(link);
  if (chip.kind === "pull-request") return PULL_REQUEST_SHAPE_ICON[chip.shape];
  return link.worktree ? FolderGit2 : GitBranch;
}

export interface LinkRowProps {
  link: SessionLink;
  agentLabel: string;
  /** Pins the row's menu open for review. */
  menuOpen?: boolean;
  /** Pins the row's details open for review. */
  detailOpen?: boolean;
  onUnlink: (link: SessionLink) => void;
}

export function LinkRow({
  link,
  agentLabel,
  menuOpen = false,
  detailOpen = false,
  onUnlink,
}: LinkRowProps) {
  const pr = link.facts.pull_request;
  return (
    <Item
      className="flex-nowrap gap-3"
      data-slot="session-link-row"
      data-source={link.source}
      selectable
      size="sm"
    >
      <ItemMedia className="size-8 rounded-md bg-surface-2 text-muted-foreground">
        <Icon as={rowIcon(link)} className="size-4" />
      </ItemMedia>
      <ItemContent className="min-w-0 gap-1">
        <ItemTitle className="w-full">
          <Tooltip defaultOpen={detailOpen}>
            <TooltipTrigger
              render={
                <a
                  aria-label={linkTitle(link)}
                  className="min-w-0 truncate outline-none after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring"
                  href={pr?.url ?? "#"}
                  rel="noreferrer"
                  target={pr ? "_blank" : undefined}
                />
              }
            >
              {linkTitle(link)}
            </TooltipTrigger>
            <TooltipContent
              align="start"
              className={SESSION_PULL_REQUEST_DETAILS_CLASS}
              side="left"
              // The title sits past the icon; the details clear the popover's edge instead.
              sideOffset={72}
            >
              <SessionPullRequestDetails model={linkChip(link)} />
            </TooltipContent>
          </Tooltip>
        </ItemTitle>
        <LinkFacts agentLabel={agentLabel} link={link} />
      </ItemContent>
      <ItemActions className="relative z-10 grid items-center justify-items-end">
        {pr ? (
          <span className="pointer-events-none col-start-1 row-start-1 font-mono text-mono-id tracking-mono-id text-faint tabular-nums group-focus-within/item:opacity-0 group-hover/item:opacity-0 group-has-data-popup-open/item:opacity-0">
            #{pr.number}
          </span>
        ) : null}
        <span className="col-start-1 row-start-1">
          <LinkRowMenu defaultOpen={menuOpen} link={link} onUnlink={() => onUnlink(link)} />
        </span>
      </ItemActions>
    </Item>
  );
}
