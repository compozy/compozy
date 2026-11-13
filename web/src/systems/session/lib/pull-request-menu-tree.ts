/**
 * The one pull request menu: the topbar PR chip, a Linked work row and the session menu's
 * `Pull request #N ▸` submenu render this tree, so the three cannot drift. Each host passes only
 * the handlers it can act on; an absent handler leaves its item out (`pullRequestMenuModel`'s
 * rule: unavailable actions are absent, never disabled). A busy item stays, disabled, and names
 * what it is doing.
 */
import {
  ArrowUpRight,
  ExternalLinkIcon,
  FileDiffIcon,
  FolderGit2,
  GitBranch,
  GitPullRequestCreateIcon,
  Link2,
  MessageSquareTextIcon,
  RefreshCwIcon,
  Unlink,
  UploadIcon,
  WrenchIcon,
  type LucideIcon,
} from "@compozy/ui/icons";

import { formatRelativeTime, tidyMenuTree, type MenuTreeNode } from "@compozy/ui";

import {
  buildAddressReviewPrompt,
  buildFixChecksPrompt,
  pullRequestMenuModel,
  type PullRequestExitPlanView,
  type PullRequestMenuItem,
} from "./pull-request-actions";
import type { SessionPullRequest } from "./session-pull-request";

export type SessionPullRequestPromptKind = "fix" | "address";

const EXIT_ICON: Record<string, LucideIcon> = {
  commit_push: UploadIcon,
  push: UploadIcon,
  open_pr: GitPullRequestCreateIcon,
};

const EXIT_BUSY_LABEL: Record<string, string> = {
  commit_push: "Committing & pushing…",
  push: "Pushing…",
  open_pr: "Creating PR…",
};

const ITEM_ICON: Record<Exclude<PullRequestMenuItem["id"], "exit">, LucideIcon> = {
  fix: WrenchIcon,
  address: MessageSquareTextIcon,
  open: ExternalLinkIcon,
  refresh: RefreshCwIcon,
};

/** Every action id the tree can hold; test ids and keys derive from it. */
export type PullRequestMenuActionId =
  | PullRequestMenuItem["id"]
  | "review-changes"
  | "worktree"
  | "open-session"
  | "copy-link"
  | "copy-branch"
  | "link"
  | "unlink";

export interface PullRequestMenuTreeInput {
  pullRequest: SessionPullRequest | null | undefined;
  /** The item's branch: Copy branch name stands in for the link before a PR exists. */
  branch?: string;
  /** Last successful observation; Refresh's hint. */
  observedAt?: string | null;
  /** User-controllable sessions on their own branch get exit actions and agent prompts. */
  controllable: boolean;
  exitPlan?: PullRequestExitPlanView;
  exitRunning?: boolean;
  refreshing?: boolean;
  onExitAction?: (action: string) => void;
  onRefresh?: () => void;
  onPrompt?: (kind: SessionPullRequestPromptKind, text: string) => void;
  /** Opens the session's Review panel on Changes; only the session's own branch has one. */
  onReviewChanges?: () => void;
  /** The worktree the item lives in, when this client can open it. */
  worktree?: { name: string; runsHere: boolean; onOpen: () => void };
  /** Opens the child session a rolled-up item came from. */
  onOpenSession?: () => void;
  onCopy?: (text: string, kind: "Pull request link" | "Branch name") => void;
  onLink?: () => void;
  /** Absent for a rolled-up item: it is managed in its child session. */
  onUnlink?: () => void;
  /** Per-item test id; a host that renders several items keys them. */
  testId?: (id: PullRequestMenuActionId) => string | undefined;
  /** Node id prefix, unique per item inside one menu. */
  key?: string;
}

const sep = (key: string, id: string): MenuTreeNode => ({ kind: "separator", id: `${key}${id}` });

/**
 * The next action first, then Open PR, Review changes and Refresh, then any remaining
 * "Ask the agent" prompt; then where the work lives and its link; then the link itself.
 */
export function pullRequestMenuTree(input: PullRequestMenuTreeInput): MenuTreeNode[] {
  const key = input.key ?? "";
  const testId = (id: PullRequestMenuActionId) => input.testId?.(id);
  const pr = input.pullRequest;
  const model = pullRequestMenuModel({
    pullRequest: pr,
    controllable: input.controllable,
    ...(input.exitPlan ? { exitPlan: input.exitPlan } : {}),
  });
  const run = (item: PullRequestMenuItem): (() => void) | undefined => {
    switch (item.id) {
      case "exit":
        return input.onExitAction ? () => input.onExitAction?.(item.action) : undefined;
      case "open":
        return () => void window.open(item.url, "_blank", "noreferrer");
      case "refresh":
        return input.onRefresh;
      case "fix":
        return pr && input.onPrompt
          ? () => input.onPrompt?.("fix", buildFixChecksPrompt(pr))
          : undefined;
      case "address":
        return pr && input.onPrompt
          ? () => input.onPrompt?.("address", buildAddressReviewPrompt(pr))
          : undefined;
    }
  };
  const node = (item: PullRequestMenuItem, title?: string): MenuTreeNode[] => {
    const onSelect = run(item);
    if (!onSelect) return [];
    const busy =
      (item.id === "exit" && input.exitRunning === true) ||
      (item.id === "refresh" && input.refreshing === true);
    const label = !busy
      ? item.label
      : item.id === "exit"
        ? (EXIT_BUSY_LABEL[item.action] ?? "Working…")
        : "Refreshing…";
    const hint =
      item.id === "refresh" && !busy && input.observedAt
        ? formatRelativeTime(input.observedAt)
        : undefined;
    return [
      {
        kind: "item",
        id: `${key}${item.id}`,
        label,
        icon: item.id === "exit" ? (EXIT_ICON[item.action] ?? UploadIcon) : ITEM_ICON[item.id],
        ...(hint ? { hint } : {}),
        // Why the next step is the one offered (what blocks Commit & push or Open PR).
        ...(title ? { title } : {}),
        disabled: busy,
        testId: testId(item.id),
        onSelect,
      },
    ];
  };
  const action = (
    id: PullRequestMenuActionId,
    label: string,
    icon: LucideIcon,
    onSelect: (() => void) | undefined,
    hint?: string
  ): MenuTreeNode[] =>
    onSelect
      ? [
          {
            kind: "item",
            id: `${key}${id}`,
            label,
            icon,
            ...(hint ? { hint } : {}),
            testId: testId(id),
            onSelect,
          },
        ]
      : [];
  const open = model.rest.find(item => item.id === "open");
  const refresh = model.rest.find(item => item.id === "refresh");
  const { onCopy, worktree } = input;
  const url = pr?.url;
  const branch = input.branch;
  return tidyMenuTree([
    ...(model.primary ? node(model.primary, model.primaryReason) : []),
    ...(open ? node(open) : []),
    ...action("review-changes", "Review changes", FileDiffIcon, input.onReviewChanges),
    ...(refresh ? node(refresh) : []),
    {
      kind: "group",
      id: `${key}ask`,
      label: "Ask the agent",
      children: model.prompts.flatMap(prompt => node(prompt)),
    },
    sep(key, "where"),
    ...(worktree
      ? action(
          "worktree",
          `Worktree ${worktree.name}`,
          FolderGit2,
          worktree.onOpen,
          worktree.runsHere ? "Runs here" : undefined
        )
      : []),
    ...action("open-session", "Open session", ArrowUpRight, input.onOpenSession),
    ...(url
      ? action(
          "copy-link",
          "Copy pull request link",
          Link2,
          onCopy ? () => onCopy(url, "Pull request link") : undefined
        )
      : branch
        ? action(
            "copy-branch",
            "Copy branch name",
            GitBranch,
            onCopy ? () => onCopy(branch, "Branch name") : undefined
          )
        : []),
    sep(key, "link"),
    ...action("link", "Link another…", Link2, input.onLink),
    ...action("unlink", "Unlink from this session", Unlink, input.onUnlink),
  ]);
}
