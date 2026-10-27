import { Button, StatusDot, cn } from "@compozy/ui";

import { useSessionSubagentBranch } from "../../hooks/use-session-subagent-branch";
import { subagentNavigationTarget } from "../../lib/subagent-navigation";
import { SUBAGENT_BRANCH_PAGE_SIZE } from "../../lib/subagent-query-options";
import type { SessionListTarget } from "../../types";
import { SUBAGENT_STATUS_WORD, subagentDot } from "../subagents/subagent-format";
import { SubagentRowTrail } from "../subagents/subagent-row-trail";
import type { SubagentView } from "../subagents/types";
import { BRANCH_NODE } from "./session-list-branch-rail";
import { SUBAGENT_ROW_FRAME } from "./session-list-row-frame";
import { SessionSubagentRowsSkeleton } from "./session-list-skeleton";

export interface SessionListSubagentBranchProps {
  workspaceId: string;
  sessionId: string;
  /** The parent row's `subagent_summary.total`: sizes the placeholder before rows load. */
  total: number;
  currentSessionId?: string;
  onSelectSession: (target: SessionListTarget) => void;
  testIdPrefix: string;
}

/**
 * A parent row's open subagent list: one compact row per direct subagent on
 * the parent's rail, newest first, each opening the subagent's session. At
 * most one page mounts until "Show N more" asks for the next.
 */
export function SessionListSubagentBranch({
  workspaceId,
  sessionId,
  total,
  currentSessionId,
  onSelectSession,
  testIdPrefix,
}: SessionListSubagentBranchProps) {
  const branch = useSessionSubagentBranch(workspaceId, sessionId, total);
  const open = (subagent: SubagentView) => {
    const target = subagentNavigationTarget(subagent, workspaceId);
    if (!target) return;
    onSelectSession({
      id: target.sessionId,
      agent_name: target.agentName,
      workspace_id: workspaceId,
      ...(target.subagentId ? { subagentId: target.subagentId } : {}),
    });
  };

  return (
    <div data-testid={`${testIdPrefix}-subagents-${sessionId}`}>
      {branch.loading ? (
        // Placeholders are the live rows' own frame, so the list does not jump when they land.
        <SessionSubagentRowsSkeleton count={Math.min(total, SUBAGENT_BRANCH_PAGE_SIZE)} />
      ) : (
        branch.rows.map(subagent => (
          <SubagentBranchRow
            key={subagent.id}
            subagent={subagent}
            current={
              subagent.child_session_id !== null && subagent.child_session_id === currentSessionId
            }
            onOpen={() => open(subagent)}
          />
        ))
      )}
      {branch.failed ? (
        <div className={cn(BRANCH_NODE, "flex items-center gap-2 px-2 py-1.5")}>
          <span className="text-transcript-caption text-subtle">Couldn't load subagents.</span>
          <Button type="button" variant="ghost" size="xs" onClick={branch.retry}>
            Try again
          </Button>
        </div>
      ) : branch.remaining > 0 && !branch.loading ? (
        <div className={cn(BRANCH_NODE, "px-0.5 py-0.5")}>
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={branch.loadingMore}
            onClick={branch.showMore}
            data-testid={`${testIdPrefix}-subagents-more-${sessionId}`}
          >
            Show {Math.min(SUBAGENT_BRANCH_PAGE_SIZE, branch.remaining)} more
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function SubagentBranchRow({
  subagent,
  current,
  onOpen,
}: {
  subagent: SubagentView;
  current: boolean;
  onOpen: () => void;
}) {
  const dot = subagentDot(subagent.status);
  const word = SUBAGENT_STATUS_WORD[subagent.status];
  const body = (
    <>
      <span className="flex justify-center" aria-hidden="true">
        <StatusDot
          tone={dot.tone ?? "faint"}
          variant={dot.tone === null ? "ring" : "solid"}
          className={cn(dot.tone === null && "text-indicator")}
        />
      </span>
      <span className="min-w-0 truncate">{subagent.title}</span>
      <SubagentRowTrail subagent={subagent} />
    </>
  );
  const rowClass = cn(SUBAGENT_ROW_FRAME, current && "bg-selected");

  return (
    <div className={BRANCH_NODE} data-slot="session-subagent-row" data-status={subagent.status}>
      {subagent.child_session_id !== null || subagent.origin === "provider_native" ? (
        <button
          type="button"
          className={cn(
            rowClass,
            "transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring"
          )}
          aria-label={`Open ${subagent.title}`}
          aria-description={word}
          aria-current={current ? "true" : undefined}
          onClick={onOpen}
        >
          {body}
        </button>
      ) : (
        // A delegated child can still be awaiting session admission.
        <div role="group" aria-label={`${subagent.title}, ${word}`} className={rowClass}>
          {body}
        </div>
      )}
    </div>
  );
}
