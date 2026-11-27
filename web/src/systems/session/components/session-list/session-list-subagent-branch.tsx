import { use, useMemo, type MouseEvent } from "react";
import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { computerCanOperateSessions, useComputers } from "@/systems/computers";
import { sessionRowKeyFor } from "../../lib/session-row-ref";
import { SubagentDrawerContext } from "../../contexts/subagent-drawer-context";
import { SessionSubagentActionsContext } from "../../contexts/session-subagent-actions-context-value";
import { Button, TreeBranchNode } from "@compozy/ui";

import { useSessionSubagentBranch } from "../../hooks/use-session-subagent-branch";
import { subagentNavigationTarget } from "../../lib/subagent-navigation";
import { SUBAGENT_BRANCH_PAGE_SIZE } from "../../lib/subagent-query-options";
import type { SessionListTarget } from "../../types";
import type { SubagentView } from "../subagents/types";
import { SessionListBranchMore } from "./session-list-branch-more";
import { SessionSubagentRowsSkeleton } from "./session-list-skeleton";
import { SessionListSubagentRow } from "./session-list-subagent-row";

export interface SessionListSubagentBranchProps {
  workspaceId: string;
  workspace?: string;
  profile?: string;
  sessionId: string;
  computerId?: ComputerId;
  /** The parent row's `subagent_summary.total`: sizes the placeholder before rows load. */
  total: number;
  currentSessionId?: string;
  onSelectSession: (target: SessionListTarget) => void;
  testIdPrefix: string;
}

/**
 * A parent row's open subagent list: one compact row per direct subagent on
 * the parent's rail, active work first (waiting, working), then failed,
 * queued and settled, each opening the subagent's session. The daemon pages in
 * that order and the branch re-sorts loaded and live rows the same way, so a
 * status change reorders them. One page mounts until "Show N more" asks for
 * the next, except that every active row shows (#1732).
 */
export function SessionListSubagentBranch({
  workspaceId,
  workspace,
  profile,
  sessionId,
  computerId,
  total,
  currentSessionId,
  onSelectSession,
  testIdPrefix,
}: SessionListSubagentBranchProps) {
  const localDrawer = use(SubagentDrawerContext);
  const remote = isRemoteComputerId(computerId);
  const drawer = remote ? null : localDrawer;
  const { computers } = useComputers({ enabled: remote, passive: true });
  const computer = computers.find(row => row.id === computerId);
  const canOperate =
    !remote ||
    (computer?.status === "connected" && computerCanOperateSessions(computer.scopes, false));
  const actions = use(SessionSubagentActionsContext);
  const branch = useSessionSubagentBranch(workspaceId, sessionId, total, computerId);
  // The branch's rows carry this parent's scope; the actions owner lives above
  // the threads, so a confirmation outlives this branch.
  const verbs = useMemo(() => {
    if (!actions || !canOperate) return {};
    const scope = {
      workspaceId,
      sessionId,
      profile: profile ?? "",
      ...(remote ? { computerId } : {}),
    };
    return {
      onDelete: (subagent: SubagentView) => actions.openDelete(subagent, scope),
      onStop: (subagent: SubagentView) => actions.stop(subagent, scope),
    };
  }, [actions, canOperate, workspaceId, sessionId, profile, computerId, remote]);
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
      ...(remote ? { computerId } : {}),
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
          <SessionListSubagentRow
            key={subagent.id}
            subagent={subagent}
            workspace={workspace}
            profile={profile}
            menuTestId={`${testIdPrefix}-subagent-menu-${subagent.id}`}
            onDelete={verbs.onDelete}
            onStop={verbs.onStop}
            current={
              subagent.child_session_id !== null &&
              sessionRowKeyFor({ id: subagent.child_session_id, computerId }) === currentSessionId
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
        <TreeBranchNode className="flex items-center gap-2 px-2 py-1.5">
          <span className="text-transcript-caption text-subtle">Couldn't load subagents.</span>
          <Button type="button" variant="ghost" size="xs" onClick={branch.retry}>
            Try again
          </Button>
        </TreeBranchNode>
      ) : branch.loading ? null : (
        <SessionListBranchMore
          count={Math.min(SUBAGENT_BRANCH_PAGE_SIZE, branch.remaining)}
          onShowMore={branch.showMore}
          disabled={branch.loadingMore}
          testId={`${testIdPrefix}-subagents-more-${sessionId}`}
        />
      )}
    </div>
  );
}
