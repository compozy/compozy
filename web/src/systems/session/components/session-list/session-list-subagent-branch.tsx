import { use, type MouseEvent } from "react";
import { SubagentDrawerContext } from "../../contexts/subagent-drawer-context";
import { useSubagentDrawerSelected } from "../../hooks/use-subagent-drawer-selected";
import { SubagentRuntimeMeta } from "../subagents/subagent-runtime-meta";
import { Button, StateGlyph, cn } from "@compozy/ui";

import { useSessionSubagentBranch } from "../../hooks/use-session-subagent-branch";
import { subagentNavigationTarget } from "../../lib/subagent-navigation";
import { SUBAGENT_BRANCH_PAGE_SIZE } from "../../lib/subagent-query-options";
import type { SessionListTarget } from "../../types";
import { sortSubagentsByUrgency, subagentDisplay } from "../subagents/subagent-format";
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
 * the parent's rail, most urgent first (waiting, failed, working, queued,
 * settled), each opening the subagent's session. The daemon pages in that
 * order; the rows re-sort here so a live status change reorders loaded rows.
 * At most one page mounts until "Show N more" asks for the next.
 */
export function SessionListSubagentBranch({
  workspaceId,
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
          <SubagentBranchRow
            key={subagent.id}
            subagent={subagent}
            current={
              subagent.child_session_id !== null && subagent.child_session_id === currentSessionId
            }
            onOpen={event => open(subagent, event)}
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
  onOpen: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const selected = useSubagentDrawerSelected(subagent.id);
  const display = subagentDisplay(subagent);
  const word = display.word;
  const body = (
    <>
      <span className="flex justify-center">
        <StateGlyph size="sm" state={display.glyph} />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className={cn("truncate", display.settled && "text-muted")}>{subagent.title}</span>
        <SubagentRuntimeMeta subagent={subagent} />
      </span>
      <SubagentRowTrail subagent={subagent} />
    </>
  );
  const rowClass = cn(SUBAGENT_ROW_FRAME, (current || selected) && "bg-selected");

  return (
    <div
      className={BRANCH_NODE}
      data-slot="session-subagent-row"
      data-status={subagent.status}
      data-state={display.glyph}
    >
      {
        <button
          type="button"
          className={cn(
            rowClass,
            "transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring"
          )}
          aria-label={`Open ${subagent.title}`}
          aria-description={word}
          aria-current={current || selected ? "true" : undefined}
          onClick={onOpen}
        >
          {body}
        </button>
      }
    </div>
  );
}
