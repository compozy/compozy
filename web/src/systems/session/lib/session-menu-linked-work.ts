/**
 * The session's linked work inside its menu. One item is a `Pull request #N ▸` (or `Branch x ▸`)
 * submenu; two or more are `Linked work N ▸`, one submenu per item, grouped like the Linked work
 * list from five items and capped so the menu never scrolls. Each item's submenu is the shared
 * pull request menu (`pullRequestMenuTree`). A rolled-up item belongs to its child session and is
 * never unlinked here (COPY.md "Linked work").
 */
import {
  FolderGit2,
  GitBranch,
  GitPullRequest,
  Link2,
  ListTree,
  type LucideIcon,
} from "@compozy/ui/icons";

import { tidyMenuTree, type MenuTreeNode } from "@compozy/ui";

import { pullRequestMenuTree, type PullRequestMenuActionId } from "./pull-request-menu-tree";
import type { SessionCopyKind } from "./session-copy";
import {
  LINK_GROUP_LABEL,
  LIST_GROUPS_MIN,
  groupLinks,
  isRolledUp,
  linkKey,
  linkNumber,
  linkShortName,
  linkStatus,
  linkTitle,
  type SessionLinkedWorkItem,
} from "./session-linked-work";

/** Past this many items the submenu stops and offers the session's full list. */
export const MENU_LINK_LIMIT = 8;

/** What the menu knows about the session's linked work and can do with it. */
export interface SessionMenuLinkedWork {
  /** The full list once read; before that, the catalog summary's lead. */
  items: readonly SessionLinkedWorkItem[];
  /** How many items the session has, from the summary while the list loads. */
  total: number;
  refreshing: boolean;
  onRefresh?: (item: SessionLinkedWorkItem) => void;
  onUnlink?: (item: SessionLinkedWorkItem) => void;
  /** Opens the link dialog; absent where no window can host it. */
  onLink?: () => void;
  /** Opens the session's full list. */
  onShowAll?: () => void;
}

interface LinkedWorkMenuOptions {
  disabled: boolean;
  onCopy: (text: string, kind: SessionCopyKind) => void;
  testId: (id: string) => string;
}

function itemIcon(item: SessionLinkedWorkItem): LucideIcon {
  if (linkNumber(item) !== null) return GitPullRequest;
  return item.worktree_name ? FolderGit2 : GitBranch;
}

function itemActions(
  work: SessionMenuLinkedWork,
  item: SessionLinkedWorkItem,
  key: string,
  options: LinkedWorkMenuOptions,
  onLink?: () => void
): MenuTreeNode[] {
  const { onRefresh, onUnlink } = work;
  return pullRequestMenuTree({
    pullRequest: item.pull_request,
    branch: item.branch,
    observedAt: item.pull_request_lookup?.observed_at,
    // Agent prompts and exit actions need the session's window; the list cannot send them.
    controllable: false,
    refreshing: work.refreshing,
    ...(onRefresh ? { onRefresh: () => onRefresh(item) } : {}),
    ...(onUnlink && !isRolledUp(item) ? { onUnlink: () => onUnlink(item) } : {}),
    ...(onLink ? { onLink } : {}),
    onCopy: options.onCopy,
    key: `${key}-`,
    testId: (id: PullRequestMenuActionId) => options.testId(`${key}-${id}`),
  });
}

function itemNode(
  work: SessionMenuLinkedWork,
  item: SessionLinkedWorkItem,
  options: LinkedWorkMenuOptions
): MenuTreeNode {
  const key = `link-${linkKey(item)}`;
  return {
    kind: "sub",
    id: key,
    label: `${linkShortName(item)} ${linkTitle(item)}`,
    icon: itemIcon(item),
    hint: linkStatus(item).label,
    disabled: options.disabled,
    children: itemActions(work, item, key, options),
  };
}

/** `Pull request #N ▸` for one item, `Linked work N ▸` for several, nothing for none. */
export function sessionLinkedWorkNode(
  work: SessionMenuLinkedWork,
  options: LinkedWorkMenuOptions
): MenuTreeNode | null {
  const { items } = work;
  const total = Math.max(work.total, items.length);
  if (items.length === 0) return null;
  if (total === 1) {
    const item = items[0]!;
    const number = linkNumber(item);
    return {
      kind: "sub",
      id: "pull-request",
      label: number === null ? `Branch ${item.branch}` : `Pull request #${number}`,
      icon: itemIcon(item),
      disabled: options.disabled,
      testId: options.testId("pull-request"),
      children: itemActions(work, item, "lead", options, work.onLink),
    };
  }
  const groups = groupLinks(items);
  const shown = new Set(groups.flatMap(group => group.items).slice(0, MENU_LINK_LIMIT));
  const body: MenuTreeNode[] =
    items.length < LIST_GROUPS_MIN
      ? items.map(item => itemNode(work, item, options))
      : groups.map(group => ({
          kind: "group",
          id: `links-${group.id}`,
          label: LINK_GROUP_LABEL[group.id],
          children: group.items.flatMap(item =>
            shown.has(item) ? [itemNode(work, item, options)] : []
          ),
        }));
  const hidden = total - shown.size;
  return {
    kind: "sub",
    id: "linked-work",
    label: "Linked work",
    icon: GitPullRequest,
    hint: String(total),
    disabled: options.disabled,
    testId: options.testId("linked-work"),
    children: tidyMenuTree([
      ...body,
      { kind: "separator", id: "links-footer" },
      ...(hidden > 0 && work.onShowAll
        ? [
            {
              kind: "item",
              id: "links-show-all",
              label: `Show all ${total}…`,
              icon: ListTree,
              testId: options.testId("links-show-all"),
              onSelect: work.onShowAll,
            } as const,
          ]
        : []),
      ...(work.onLink
        ? [
            {
              kind: "item",
              id: "link-another",
              label: "Link another…",
              icon: Link2,
              testId: options.testId("link-another"),
              onSelect: work.onLink,
            } as const,
          ]
        : []),
    ]),
  };
}

/** The pull request links Copy ▸ can hand out at once. */
export function linkedPullRequestUrls(items: readonly SessionLinkedWorkItem[]): string[] {
  return items.flatMap(item => (item.pull_request?.url ? [item.pull_request.url] : []));
}
