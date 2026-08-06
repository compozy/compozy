import { PillDot, StatusDot, Time, type PillTone } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionRowActions } from "../session-row-actions";

function sessionStatusTone(badge: string): PillTone {
  if (badge === "running") return "accent";
  if (badge === "idle") return "success";
  if (badge === "waiting-for-auth" || badge === "hung") return "warning";
  if (badge === "failed" || badge === "unhealthy") return "danger";
  return "neutral";
}

export function SessionStatusMark({ badge }: { badge: string }) {
  if (badge === "stopped") {
    return <StatusDot tone="faint" variant="ring" label="stopped" />;
  }
  return <PillDot tone={sessionStatusTone(badge)} pulse={badge === "running"} size="sm" />;
}

export interface SessionListRowProps {
  session: SessionPayload;
  current?: boolean;
  onSelect: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Trailing controls rendered beside the row actions (e.g. a thread toggle). */
  trailing?: React.ReactNode;
}

export function SessionListRow({
  session,
  current = false,
  onSelect,
  sessionActions,
  testIdPrefix,
  trailing,
}: SessionListRowProps) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-1">
      <button
        type="button"
        className={cn(
          "relative grid min-w-0 grid-cols-[8px_minmax(0,1fr)_auto] items-start gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none",
          current &&
            "bg-row-selected before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-accent"
        )}
        data-status={session.badge}
        data-testid={`${testIdPrefix}-session-${session.id}`}
        aria-current={current ? "true" : undefined}
        onClick={onSelect}
      >
        <span className="mt-1.5 grid place-items-center">
          <SessionStatusMark badge={session.badge} />
        </span>
        <span className="min-w-0">
          <span
            className={cn(
              "block truncate text-small-body text-fg-strong",
              current && "font-medium"
            )}
          >
            {getSessionDisplayTitle(session)}
          </span>
          <span className="block truncate text-micro text-subtle">
            <span className="font-medium text-muted">{session.agent_name}</span>
            <span aria-hidden="true"> · </span>
            {session.badge}
            {session.archived_at !== null ? (
              <>
                <span aria-hidden="true"> · </span>
                Archived
              </>
            ) : null}
          </span>
        </span>
        <Time iso={session.updated_at} className="mt-0.5 font-mono text-micro text-subtle" />
      </button>
      <div className="flex items-center gap-0.5 pt-1">
        {trailing}
        <SessionRowActions session={session} actions={sessionActions} />
      </div>
    </div>
  );
}
