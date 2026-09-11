import { Checkbox, Time } from "@compozy/ui";
import type { SessionRowSelection } from "../../hooks/use-session-selection";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { sessionBadgeSignal } from "../../lib/session-badge";
import { sessionBadgeWordClass } from "../../lib/session-badge-classes";
import { maskedAttentionNote } from "../../lib/session-pending-interactions";
import { SessionBadgeMark } from "../session-badge-mark";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionRowActions } from "../session-row-actions";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface SessionListRowProps {
  session: SessionPayload;
  /**
   * The row's profile, supplied only while the aggregate is on. A scoped list
   * already answers "whose work is this", so it stays tag-free (US-011.AC-1).
   */
  owner?: ProfileOwner;
  current?: boolean;
  onSelect: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  showActions?: boolean;
  testIdPrefix: string;
  /** Trailing controls rendered beside the row actions (e.g. a thread toggle). */
  trailing?: React.ReactNode;
  selection?: SessionRowSelection;
}

export function SessionListRow({
  session,
  owner,
  current = false,
  onSelect,
  sessionActions,
  showActions = true,
  testIdPrefix,
  trailing,
  selection,
}: SessionListRowProps) {
  const signal = sessionBadgeSignal(session.badge);
  const maskedNote = maskedAttentionNote(session, signal.label);
  const selected = selection?.selectedIds.has(session.id) ?? false;
  const title = getSessionDisplayTitle(session);
  return (
    <div className="group/session-row relative grid grid-cols-[minmax(0,1fr)_auto] items-start gap-1">
      <button
        type="button"
        className={cn(
          "relative grid min-w-0 grid-cols-[8px_minmax(0,1fr)_auto] items-start gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none",
          selected && "bg-row-selected",
          current &&
            "bg-row-selected before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-accent"
        )}
        data-status={session.badge}
        data-testid={`${testIdPrefix}-session-${session.id}`}
        aria-current={current ? "true" : undefined}
        aria-selected={selection ? selected : undefined}
        data-selected={selected || undefined}
        onClick={event => {
          if (selection && event.shiftKey) selection.toggleRange(session.id);
          else if (selection && (selection.mode || event.metaKey || event.ctrlKey))
            selection.toggle(session.id);
          else onSelect();
        }}
      >
        <SessionBadgeMark
          badge={session.badge}
          className={cn(
            "mt-1.5",
            selection &&
              "transition-opacity duration-fast motion-reduce:transition-none group-hover/session-row:opacity-0 group-focus-within/session-row:opacity-0",
            selection?.mode && "opacity-0"
          )}
        />
        <span className="min-w-0">
          <span
            className={cn(
              "block truncate text-small-body",
              // A needs-you row keeps its pull even when the window is not focused.
              signal.attention === "needs-you" ? "font-medium text-fg-strong" : "text-fg-strong",
              current && "font-medium"
            )}
          >
            {getSessionDisplayTitle(session)}
          </span>
          <span className="block truncate text-micro text-subtle">
            <span className="font-medium text-muted">{session.agent_name}</span>
            <span aria-hidden="true"> · </span>
            <span className={sessionBadgeWordClass(session.badge)}>{signal.label}</span>
            {maskedNote !== null ? (
              <>
                <span aria-hidden="true"> · </span>
                {maskedNote}
              </>
            ) : null}
            {session.archived_at !== null ? (
              <>
                <span aria-hidden="true"> · </span>
                Archived
              </>
            ) : null}
          </span>
        </span>
        <span className="mt-0.5 flex items-center gap-1.5">
          {owner ? <ProfileOwnerTag compact owner={owner} /> : null}
          <Time iso={session.updated_at} className="font-mono text-micro text-subtle" />
        </span>
      </button>
      {selection ? (
        <Checkbox
          aria-label={`Select ${title}`}
          checked={selected}
          className={cn(
            "absolute top-2 left-1 z-10 transition-opacity duration-fast motion-reduce:transition-none after:inset-0",
            selection.mode
              ? "opacity-100"
              : "pointer-events-none opacity-0 group-hover/session-row:pointer-events-auto group-hover/session-row:opacity-100 group-focus-within/session-row:pointer-events-auto group-focus-within/session-row:opacity-100"
          )}
          onCheckedChange={(_checked, details) => {
            if ("shiftKey" in details.event && details.event.shiftKey)
              selection.toggleRange(session.id);
            else selection.toggle(session.id);
          }}
        />
      ) : null}
      <div className="flex items-center gap-0.5 pt-1">
        {trailing}
        {showActions && !selection?.mode ? (
          <SessionRowActions session={session} actions={sessionActions} />
        ) : null}
      </div>
    </div>
  );
}
