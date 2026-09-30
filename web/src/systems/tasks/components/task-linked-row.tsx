import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { cn, OwnerAvatar, StateGlyph, type StateGlyphState, Time } from "@compozy/ui";

import { ownerAvatarKindFor, taskOwnerLabel } from "../lib/task-formatters";
import type { TaskChildSummary } from "../types";

export type TaskLinkedRowState = "done" | "active" | "todo";

export interface TaskLinkedRowProps {
  taskId: string;
  title: string;
  state: TaskLinkedRowState;
  owner?: TaskChildSummary["owner"] | null;
  lastActivityAt?: string | null;
  testId?: string;
}

const STATE_GLYPH: Record<TaskLinkedRowState, StateGlyphState> = {
  done: "done",
  active: "running",
  todo: "queued",
};

const STATE_LABEL: Record<TaskLinkedRowState, string> = {
  active: "Active",
  done: "Completed",
  todo: "Not started",
};

/**
 * Shared row anatomy for subtasks and dependencies: status dot, title,
 * owner, freshness, chevron. The whole row is the link — no per-row accent
 * button (accent budget stays with the head primary).
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.3
 */
export function TaskLinkedRow({
  taskId,
  title,
  state,
  owner,
  lastActivityAt,
  testId,
}: TaskLinkedRowProps) {
  const ownerName = owner ? taskOwnerLabel(owner) : null;
  return (
    <Link
      className={cn(
        "grid grid-cols-[14px_minmax(0,1fr)_auto_14px] items-center gap-3 px-4 py-2.5",
        "border-t border-line-soft transition-colors duration-fast first:border-t-0 hover:bg-surface-2",
        "focus-visible:outline-none focus-visible:shadow-focus-ring",
        lastActivityAt ? "sm:grid-cols-[14px_minmax(0,1fr)_auto_auto_14px]" : null
      )}
      data-testid={testId}
      aria-label={`${title}, ${STATE_LABEL[state]}`}
      params={{ id: taskId }}
      to="/tasks/$id"
    >
      <StateGlyph
        className="justify-self-center"
        label={STATE_LABEL[state]}
        state={STATE_GLYPH[state]}
      />
      <span className="truncate text-ws-name font-medium text-fg-strong">
        {state === "done" ? <s className="text-muted decoration-faint">{title}</s> : title}
      </span>
      {ownerName && owner ? (
        <span className="inline-flex min-w-0 items-center gap-1.5 text-form-label text-muted">
          <OwnerAvatar
            name={ownerName}
            ownerId={owner.ref ?? ownerName}
            ownerKind={ownerAvatarKindFor(owner.kind)}
            size="sm"
          />
          <span className="hidden truncate sm:inline">{ownerName}</span>
        </span>
      ) : (
        <span aria-hidden="true" />
      )}
      {lastActivityAt ? (
        <span className="hidden text-eyebrow tabular-nums text-subtle sm:inline">
          <Time iso={lastActivityAt} mode="relative" />
        </span>
      ) : null}
      <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
    </Link>
  );
}
