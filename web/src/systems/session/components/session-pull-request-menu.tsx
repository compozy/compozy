import { DropdownMenuContent, MenuTreeItems } from "@compozy/ui";

import { copySessionText } from "../lib/session-copy";
import { pullRequestMenuTree, type PullRequestMenuTreeInput } from "../lib/pull-request-menu-tree";

export type SessionPullRequestMenuProps = Omit<
  PullRequestMenuTreeInput,
  "onCopy" | "testId" | "key"
>;

/**
 * The pull request menu of the topbar PR chip and of a Linked work row: one tree
 * (`pullRequestMenuTree`), the next action first. Items read
 * `session-pull-request-action-<id>`.
 */
export function SessionPullRequestMenu(props: SessionPullRequestMenuProps) {
  const nodes = pullRequestMenuTree({
    ...props,
    onCopy: (text, kind) => void copySessionText(text, kind),
    testId: id => `session-pull-request-action-${id}`,
  });
  return (
    <DropdownMenuContent
      align="start"
      aria-label="Pull request actions"
      className="min-w-58"
      data-testid="session-pull-request-menu"
    >
      <MenuTreeItems host="dropdown" nodes={nodes} />
    </DropdownMenuContent>
  );
}
