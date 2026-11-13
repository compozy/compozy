/**
 * The session's linked work in its menu. One item keeps the `Pull request #N ▸` submenu; two or
 * more become `Linked work ▸`, one submenu per item, grouped like the Linked work list from five
 * items and capped so the menu never scrolls. The rules are the list's own (COPY.md "Linked
 * work"): agent prompts only on the session's own branch, a rolled-up item opens its child session
 * and is never unlinked here. The catalog row carries only the summary, so the full list loads
 * when the submenu opens (`useSessionLinkedWork`).
 */
import {
  ArrowUpRight,
  ExternalLinkIcon,
  FolderGit2,
  GitBranch,
  GitPullRequest,
  Link2,
  ListTree,
  MessageSquareTextIcon,
  RefreshCwIcon,
  Unlink,
  WrenchIcon,
  type LucideIcon,
} from "@compozy/ui/icons";

import {
  pullRequestMenuModel,
  type PullRequestMenuItem,
} from "@/systems/session/lib/pull-request-actions";
import {
  LINK_GROUP_LABEL,
  LIST_GROUPS_MIN,
  groupLinks,
  isOwnBranch,
  isRolledUp,
  linkKey,
  linkNumber,
  linkShortName,
  linkStatus,
  linkTitle,
  type SessionLinkedWorkItem,
} from "@/systems/session/lib/session-linked-work";
import { isUserControllableSession } from "@/systems/session/lib/session-running";
import type { SessionPayload } from "@/systems/session/types";

import type { SessionMenuNode } from "./session-menu-model";

/** Past this many items the submenu stops and offers the session's full list. */
export const MENU_LINK_LIMIT = 8;

const PR_ICON: Record<Exclude<PullRequestMenuItem["id"], "exit">, LucideIcon> = {
  fix: WrenchIcon,
  address: MessageSquareTextIcon,
  open: ExternalLinkIcon,
  refresh: RefreshCwIcon,
};

const separator = (id: string): SessionMenuNode => ({ kind: "separator", id });

function prNode(entry: PullRequestMenuItem, key: string): SessionMenuNode {
  return {
    kind: "item",
    id: `${key}-pr-${entry.id}`,
    label: entry.label,
    icon: entry.id === "exit" ? GitPullRequest : PR_ICON[entry.id],
  };
}

/** One item's verbs: the PR chip menu, then copy and unlink. */
export function linkActionNodes(
  session: SessionPayload,
  item: SessionLinkedWorkItem,
  key: string
): SessionMenuNode[] {
  const rolledUp = isRolledUp(item);
  const model = item.pull_request
    ? pullRequestMenuModel({
        pullRequest: item.pull_request,
        controllable: isUserControllableSession(session) && isOwnBranch(item, session),
      })
    : null;
  const nodes: SessionMenuNode[] = [];
  if (rolledUp) {
    nodes.push({
      kind: "item",
      id: `${key}-open-session`,
      label: "Open session",
      icon: ArrowUpRight,
      hint: item.source_session_title ?? undefined,
    });
  }
  if (model) {
    if (model.primary) nodes.push(prNode(model.primary, key));
    nodes.push(...model.rest.map(entry => prNode(entry, key)));
    if (model.prompts.length > 0) {
      nodes.push(separator(`${key}-prompts`), ...model.prompts.map(entry => prNode(entry, key)));
    }
  }
  nodes.push(separator(`${key}-copy`));
  nodes.push(
    linkNumber(item) !== null
      ? { kind: "item", id: `${key}-copy-link`, label: "Copy pull request link", icon: Link2 }
      : { kind: "item", id: `${key}-copy-branch`, label: "Copy branch name", icon: GitBranch }
  );
  if (!rolledUp) {
    nodes.push({
      kind: "item",
      id: `${key}-unlink`,
      label: "Unlink from this session",
      icon: Unlink,
    });
  }
  return nodes;
}

function itemIcon(item: SessionLinkedWorkItem): LucideIcon {
  if (linkNumber(item) !== null) return GitPullRequest;
  return item.worktree_name ? FolderGit2 : GitBranch;
}

function itemNode(session: SessionPayload, item: SessionLinkedWorkItem): SessionMenuNode {
  const key = `link-${linkKey(item)}`;
  return {
    kind: "sub",
    id: key,
    label: `${linkShortName(item)} ${linkTitle(item)}`,
    icon: itemIcon(item),
    hint: linkStatus(item).label,
    children: linkActionNodes(session, item, key),
  };
}

const LINK_ANOTHER: SessionMenuNode = {
  kind: "item",
  id: "link-another",
  label: "Link another…",
  icon: Link2,
};

/** `Pull request #N ▸` for one item, `Linked work ▸` for several, nothing for none. */
export function linkedWorkNode(
  session: SessionPayload,
  items: readonly SessionLinkedWorkItem[]
): SessionMenuNode | null {
  if (items.length === 0) return null;
  if (items.length === 1) {
    const item = items[0]!;
    const number = linkNumber(item);
    return {
      kind: "sub",
      id: "pull-request",
      label: number === null ? `Branch ${item.branch}` : `Pull request #${number}`,
      icon: itemIcon(item),
      children: [...linkActionNodes(session, item, "lead"), separator("lead-link"), LINK_ANOTHER],
    };
  }
  const shown = new Set(
    groupLinks(items)
      .flatMap(group => group.items)
      .slice(0, MENU_LINK_LIMIT)
  );
  const body: SessionMenuNode[] =
    items.length < LIST_GROUPS_MIN
      ? items.map(item => itemNode(session, item))
      : groupLinks(items).flatMap(group => {
          const members = group.items.filter(item => shown.has(item));
          return members.length === 0
            ? []
            : [
                {
                  kind: "group",
                  id: `links-${group.id}`,
                  label: LINK_GROUP_LABEL[group.id],
                  children: members.map(item => itemNode(session, item)),
                } as const,
              ];
        });
  const hidden = items.length - shown.size;
  return {
    kind: "sub",
    id: "linked-work",
    label: "Linked work",
    icon: GitPullRequest,
    hint: String(items.length),
    children: [
      ...body,
      separator("links-footer"),
      ...(hidden > 0
        ? [
            {
              kind: "item",
              id: "links-show-all",
              label: `Show all ${items.length}…`,
              icon: ListTree,
            } as const,
          ]
        : []),
      LINK_ANOTHER,
    ],
  };
}

/** The pull request links Copy ▸ can hand out at once. */
export function linkedPullRequestCount(items: readonly SessionLinkedWorkItem[]): number {
  return items.filter(item => linkNumber(item) !== null).length;
}
