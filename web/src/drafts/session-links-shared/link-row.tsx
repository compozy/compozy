/**
 * One linked item: what it is, how it's doing, and where it came from. `Item` in a popover or side
 * panel, `ListingRow` on a full page, the two row primitives the design system assigns to those
 * places. The status words are the header chip's own.
 */
import {
  CircleSlash,
  FolderGit2,
  FolderX,
  GitBranch,
  MoreHorizontal,
  Unlink,
  type LucideIcon,
} from "@compozy/ui/icons";

import {
  Button,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Icon,
  Item,
  ItemActions,
  ItemContent,
  ItemTitle,
  ListingRow,
  MonoId,
  Pill,
  cn,
} from "@compozy/ui";

import { ChipBody } from "@/systems/session/components/session-pull-request-chip";
import { PULL_REQUEST_SHAPE_ICON } from "@/systems/session/components/session-pull-request-chip-style";
import { SessionPullRequestMenu } from "@/systems/session/components/session-pull-request-menu";
import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import { linkChip, linkSourceLabel, linkTitle, tightSignals } from "./link-model";

/** `stacked` for popovers, `tight` for the side panel, `wide` for a full page. */
export type LinkRowLayout = "stacked" | "tight" | "wide";

const STATUS_CLASS =
  "flex min-w-0 items-center gap-1.5 overflow-hidden whitespace-nowrap text-badge font-medium text-muted-foreground [&_svg]:size-3 [&_svg]:shrink-0";

/** The chip's own words; a branch has no status of its own, so the row says why. */
function LinkStatus({ link, tight }: { link: SessionLink; tight: boolean }) {
  const chip = linkChip(link);
  if (chip.kind === "pull-request") {
    const model = tight ? { ...chip, signals: tightSignals(chip.signals) } : chip;
    return (
      <span className={STATUS_CLASS} data-slot="session-link-status">
        <ChipBody model={model} refreshing={false} />
      </span>
    );
  }
  const unknown = chip.marker === "unavailable" || chip.marker === "incomplete";
  return (
    <span className={STATUS_CLASS} data-slot="session-link-status">
      <Icon as={unknown ? CircleSlash : GitBranch} className="text-subtle" />
      {unknown ? `PR status unknown · ${chip.detail}` : "No pull request yet"}
    </span>
  );
}

/** Where the work lives, and whether the agent runs there. */
function LinkPlace({ link }: { link: SessionLink }) {
  const { worktree } = link;
  // With no title the row already leads with the worktree, so the branch says something new.
  if (!worktree || (!link.title && worktree.state === "ready")) {
    return <MonoId className="min-w-0 truncate" preserveCase value={link.facts.branch ?? ""} />;
  }
  const missing = worktree.state === "missing";
  return (
    <span className="flex min-w-0 items-center gap-1">
      <Icon as={missing ? FolderX : FolderGit2} className="size-3 shrink-0" />
      <span className="min-w-0 truncate">{missing ? "Worktree removed" : worktree.name}</span>
      {link.runsHere ? (
        <Pill className="shrink-0" size="sm">
          Runs here
        </Pill>
      ) : null}
    </span>
  );
}

function LinkRowMenu({
  link,
  defaultOpen,
  className,
  onUnlink,
}: {
  link: SessionLink;
  defaultOpen: boolean;
  className: string;
  onUnlink: () => void;
}) {
  return (
    <DropdownMenu defaultOpen={defaultOpen}>
      <DropdownMenuTrigger
        aria-label={`Actions for ${linkTitle(link)}`}
        render={
          <Button
            className={cn("relative z-10 shrink-0 data-popup-open:opacity-100", className)}
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
  layout: LinkRowLayout;
  agentLabel: string;
  /** Pins the row's menu open for review. */
  menuOpen?: boolean;
  onUnlink: (link: SessionLink) => void;
}

export function LinkRow({ link, layout, agentLabel, menuOpen = false, onUnlink }: LinkRowProps) {
  const url = link.facts.pull_request?.url;
  const anchor = { href: url ?? "#", rel: "noreferrer", target: url ? "_blank" : undefined };
  const source = linkSourceLabel(link, agentLabel);
  const unlink = () => onUnlink(link);

  if (layout === "wide") {
    return (
      <ListingRow className="group/link" data-slot="session-link-row" data-source={link.source}>
        <ListingRow.Link {...anchor}>
          <ListingRow.Icon>
            <Icon as={rowIcon(link)} className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>{linkTitle(link)}</ListingRow.Title>
            </ListingRow.Name>
            <ListingRow.Meta className="[&_[data-slot=chip-glyph]]:hidden">
              <LinkStatus link={link} tight={false} />
            </ListingRow.Meta>
          </ListingRow.Main>
        </ListingRow.Link>
        <ListingRow.Trail className="flex items-center gap-4 text-eyebrow text-subtle">
          <span className="flex w-24 justify-end">{source}</span>
          <span className="flex w-44 min-w-0">
            <LinkPlace link={link} />
          </span>
          <LinkRowMenu
            className="opacity-0 group-focus-within/link:opacity-100 group-hover/link:opacity-100"
            defaultOpen={menuOpen}
            link={link}
            onUnlink={unlink}
          />
        </ListingRow.Trail>
      </ListingRow>
    );
  }

  return (
    <Item
      className="flex-nowrap items-start"
      data-slot="session-link-row"
      data-source={link.source}
      selectable
      size="xs"
    >
      <ItemContent className="min-w-0 group-data-[size=xs]/item:gap-1">
        <ItemTitle className="w-full text-small-body">
          <a
            {...anchor}
            className="min-w-0 truncate outline-none after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring"
          >
            {linkTitle(link)}
          </a>
        </ItemTitle>
        <LinkStatus link={link} tight={layout === "tight"} />
        <span className="flex min-w-0 items-center justify-between gap-3 text-eyebrow text-subtle">
          <LinkPlace link={link} />
          {source ? <span className="shrink-0">{source}</span> : null}
        </span>
      </ItemContent>
      <ItemActions>
        <LinkRowMenu
          className="opacity-0 group-focus-within/item:opacity-100 group-hover/item:opacity-100"
          defaultOpen={menuOpen}
          link={link}
          onUnlink={unlink}
        />
      </ItemActions>
    </Item>
  );
}
