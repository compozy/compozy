import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { cn, OwnerAvatar, Time } from "@agh/ui";

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

const DOT_CLASS: Record<TaskLinkedRowState, string> = {
  done: "bg-success",
  active: "border border-accent bg-transparent",
  todo: "border border-faint bg-transparent",
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
        "border-t border-line-soft transition-colors duration-fast first:border-t-0 hover:bg-row-hover",
        "focus-visible:outline-none focus-visible:shadow-focus-ring",
        lastActivityAt ? "sm:grid-cols-[14px_minmax(0,1fr)_auto_auto_14px]" : null
      )}
      data-testid={testId}
      aria-label={`${title}, ${STATE_LABEL[state]}`}
      params={{ id: taskId }}
      to="/tasks/$id"
    >
      <span
        aria-label={STATE_LABEL[state]}
        className={cn("size-2 justify-self-center rounded-full", DOT_CLASS[state])}
        data-state={state}
        role="img"
      >
        {state === "active" ? (
          <span aria-hidden="true" className="m-auto block size-1 rounded-full bg-accent" />
        ) : null}
      </span>
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
