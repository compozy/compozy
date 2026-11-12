import { useState, type ReactNode } from "react";

import { TreeBranch } from "@compozy/ui";

import { isNeedsYouBadge } from "../../lib/session-badge";
import type { SessionPayload } from "../../types";
import { SessionListBranchMore } from "./session-list-branch-more";

/** The child rows an open branch shows before "Show N more". */
const CHILD_BRANCH_LIMIT = 6;

export interface SessionListChildBranchProps {
  sessionId: string;
  /** The parent's child sessions in list order. */
  childSessions: readonly SessionPayload[];
  currentSessionId: string | undefined;
  /** The list's bulk selection; a selected child stays on screen. */
  selectedIds: ReadonlySet<string> | undefined;
  childRow: (child: SessionPayload) => ReactNode;
  testIdPrefix: string;
}

/**
 * A parent row's open child-session list: the first six children in list
 * order, then "Show N more" for the rest and "Show less" to fold back. A child
 * past the cap that is open, selected, or needs you stays in its place among
 * the shown rows, so the list never hides what the operator is working on.
 * Once shown, such a child stays until "Show less", so a row does not vanish
 * under the pointer or from focus when it is deselected or left. Keyboard
 * cycles and ranges still walk every child: landing on a hidden one opens or
 * selects it, which shows it.
 */
export function SessionListChildBranch({
  sessionId,
  childSessions,
  currentSessionId,
  selectedIds,
  childRow,
  testIdPrefix,
}: SessionListChildBranchProps) {
  const [expanded, setExpanded] = useState(false);
  const [revealed, setRevealed] = useState<ReadonlySet<string>>(() => new Set());
  const pinned = (child: SessionPayload, index: number) =>
    index < CHILD_BRANCH_LIMIT ||
    child.id === currentSessionId ||
    selectedIds?.has(child.id) === true ||
    isNeedsYouBadge(child.badge);
  // Remember each child the pins brought past the cap (state adjusted during render).
  const newlyRevealed = childSessions.filter(
    (child, index) => index >= CHILD_BRANCH_LIMIT && pinned(child, index) && !revealed.has(child.id)
  );
  if (newlyRevealed.length > 0) {
    setRevealed(new Set([...revealed, ...newlyRevealed.map(child => child.id)]));
  }
  const capped = childSessions.filter(pinned);
  const shown = expanded
    ? childSessions
    : childSessions.filter((child, index) => pinned(child, index) || revealed.has(child.id));
  const hidden = childSessions.length - shown.length;
  const foldable = shown.length > capped.length;
  const fold = () => {
    setExpanded(false);
    setRevealed(new Set());
  };

  return (
    <TreeBranch>
      {shown.map(child => childRow(child))}
      {hidden > 0 || foldable ? (
        <SessionListBranchMore
          count={hidden}
          onShowMore={() => setExpanded(true)}
          onShowLess={fold}
          testId={`${testIdPrefix}-children-more-${sessionId}`}
        />
      ) : null}
    </TreeBranch>
  );
}
