import { use, type MouseEvent } from "react";
import { SubagentDrawerContext } from "../../contexts/subagent-drawer-context";
import { Button, cn } from "@compozy/ui";

import { useSessionSubagentBranch } from "../../hooks/use-session-subagent-branch";
import { subagentNavigationTarget } from "../../lib/subagent-navigation";
import { SUBAGENT_BRANCH_PAGE_SIZE } from "../../lib/subagent-query-options";
import type { SessionListTarget } from "../../types";
import { sortSubagentsByUrgency } from "../subagents/subagent-format";
import type { SubagentView } from "../subagents/types";
import { BRANCH_NODE } from "./session-list-branch-rail";
import { SessionSubagentRowsSkeleton } from "./session-list-skeleton";
import { SessionListSubagentRow } from "./session-list-subagent-row";

export interface SessionListSubagentBranchProps {
  workspaceId: string;
  workspace?: string;
  profile?: string;
  sessionId: string;
  /** The parent row's `subagent_summary.total`: sizes the placeholder before rows load. */
  total: number;
  currentSessionId?: string;
  onSelectSession: (target: SessionListTarget) => void;
  testIdPrefix: string;
}

/**
 * A parent row's open subagent list: one compact row per direct subagent on
 * the parent's rail, most urgent first (waiting, failed, working, queued,
 * settled), each opening the subagent's session. The daemon pages in that
 * order; the rows re-sort here so a live status change reorders loaded rows.
 * At most one page mounts until "Show N more" asks for the next.
 */
export function SessionListSubagentBranch({
  workspaceId,
  workspace,
  profile,
  sessionId,
  total,
  currentSessionId,
  onSelectSession,
  testIdPrefix,
}: SessionListSubagentBranchProps) {
  const drawer = use(SubagentDrawerContext);
  const branch = useSessionSubagentBranch(workspaceId, sessionId, total);
  const open = (subagent: SubagentView, event: MouseEvent<HTMLButtonElement>) => {
    if (drawer) {
      drawer.open(subagent, { newWindow: event.metaKey || event.ctrlKey }, workspaceId);
      return;
    }
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
        sortSubagentsByUrgency(branch.rows).map(subagent => (
          <SessionListSubagentRow
            key={subagent.id}
            subagent={subagent}
            workspace={workspace}
            profile={profile}
            current={
              subagent.child_session_id !== null && subagent.child_session_id === currentSessionId
            }
            onOpen={
              drawer || subagentNavigationTarget(subagent, workspaceId)
                ? event => open(subagent, event)
                : undefined
            }
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
