import {
  ExternalLinkIcon,
  GitPullRequestCreateIcon,
  MessageSquareTextIcon,
  RefreshCwIcon,
  UploadIcon,
  WrenchIcon,
  type LucideIcon,
} from "@compozy/ui/icons";

import {
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  Eyebrow,
  Spinner,
  Time,
} from "@compozy/ui";

import {
  buildAddressReviewPrompt,
  buildFixChecksPrompt,
  pullRequestMenuModel,
  type PullRequestExitPlanView,
  type PullRequestMenuItem,
} from "../lib/pull-request-actions";
import type { SessionPullRequest } from "../lib/session-pull-request";

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

interface SessionPullRequestMenuProps {
  pullRequest: SessionPullRequest | null | undefined;
  /** Last successful observation; the Refresh item's hint. */
  observedAt?: string | null;
  controllable: boolean;
  exitPlan?: PullRequestExitPlanView;
  exitRunning: boolean;
  refreshing: boolean;
  onExitAction?: (action: string) => void;
  onRefresh?: () => void;
  onPrompt?: (kind: SessionPullRequestPromptKind, text: string) => void;
}

/**
 * The chip's menu (`_uiux.md` S2): the next action first, then Open PR and
 * Refresh, then any remaining "Ask the agent" prompt. Order carries priority.
 */
export function SessionPullRequestMenu({
  pullRequest,
  observedAt,
  controllable,
  exitPlan,
  exitRunning,
  refreshing,
  onExitAction,
  onRefresh,
  onPrompt,
}: SessionPullRequestMenuProps) {
  const model = pullRequestMenuModel({
    pullRequest,
    controllable,
    ...(exitPlan ? { exitPlan } : {}),
  });

  const run = (item: PullRequestMenuItem) => {
    switch (item.id) {
      case "exit":
        if (!exitRunning) onExitAction?.(item.action);
        return;
      case "open":
        window.open(item.url, "_blank", "noreferrer");
        return;
      case "refresh":
        if (!refreshing) onRefresh?.();
        return;
      case "fix":
        if (pullRequest) onPrompt?.("fix", buildFixChecksPrompt(pullRequest));
        return;
      case "address":
        if (pullRequest) onPrompt?.("address", buildAddressReviewPrompt(pullRequest));
    }
  };

  const busy = (item: PullRequestMenuItem) =>
    (item.id === "exit" && exitRunning) || (item.id === "refresh" && refreshing);

  const renderItem = (item: PullRequestMenuItem, title?: string) => (
    <MenuItem
      busy={busy(item)}
      hint={item.id === "refresh" ? observedAt : null}
      item={item}
      key={item.id}
      onSelect={() => run(item)}
      {...(title ? { title } : {})}
    />
  );

  return (
    <DropdownMenuContent
      align="start"
      aria-label="Pull request actions"
      className="min-w-58"
      data-testid="session-pull-request-menu"
    >
      {model.primary ? renderItem(model.primary, model.primaryReason) : null}
      {model.primary ? <DropdownMenuSeparator /> : null}
      {model.rest.map(item => renderItem(item))}
      {model.prompts.length > 0 ? (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuLabel className="px-2.5 pt-1.5 pb-1">
              <Eyebrow variant="caps">Ask the agent</Eyebrow>
            </DropdownMenuLabel>
            {model.prompts.map(item => renderItem(item))}
          </DropdownMenuGroup>
        </>
      ) : null}
    </DropdownMenuContent>
  );
}

function MenuItem({
  item,
  busy,
  hint,
  title,
  onSelect,
}: {
  item: PullRequestMenuItem;
  busy: boolean;
  hint?: string | null;
  title?: string;
  onSelect: () => void;
}) {
  const Icon = item.id === "exit" ? (EXIT_ICON[item.action] ?? UploadIcon) : ITEM_ICON[item.id];
  const busyLabel =
    item.id === "exit" ? (EXIT_BUSY_LABEL[item.action] ?? "Working…") : "Refreshing…";
  return (
    <DropdownMenuItem
      aria-busy={busy || undefined}
      closeOnClick={!busy}
      data-action={item.id}
      data-slot="session-pull-request-action"
      onClick={onSelect}
      title={title}
    >
      {busy ? <Spinner aria-hidden="true" role={undefined} /> : <Icon aria-hidden="true" />}
      {busy ? busyLabel : item.label}
      {hint && !busy ? <Time className="ml-auto text-eyebrow text-subtle" iso={hint} /> : null}
    </DropdownMenuItem>
  );
}
