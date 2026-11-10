import { useState, type ReactNode } from "react";

import { isNeedsYouBadge } from "../../lib/session-badge";
import type { SessionPayload } from "../../types";
import { BRANCH_RAIL } from "./session-list-branch-rail";
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
 * Keyboard cycles and ranges still walk every child: landing on a hidden one
 * opens or selects it, which shows it.
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
  const capped = childSessions.filter(
    (child, index) =>
      index < CHILD_BRANCH_LIMIT ||
      child.id === currentSessionId ||
      selectedIds?.has(child.id) === true ||
      isNeedsYouBadge(child.badge)
  );
  const shown = expanded ? childSessions : capped;
  const foldable = capped.length < childSessions.length;

  return (
    <div className={BRANCH_RAIL}>
      {shown.map(child => childRow(child))}
      {foldable ? (
        <SessionListBranchMore
          count={childSessions.length - shown.length}
          onShowMore={() => setExpanded(true)}
          onShowLess={() => setExpanded(false)}
          testId={`${testIdPrefix}-children-more-${sessionId}`}
        />
      ) : null}
    </div>
  );
}
